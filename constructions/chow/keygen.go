package chow

import (
	"github.com/OpenWhiteBox/AES/primitives/encoding"
	"github.com/OpenWhiteBox/AES/primitives/matrix"
	"github.com/OpenWhiteBox/AES/primitives/table"

	"github.com/OpenWhiteBox/AES/constructions/saes"
)

type KeyGenerationOpts interface{}

type IndependentMasks struct {
	Input, Output MaskType
}

type SameMasks MaskType

type MatchingMasks struct{}

func GenerateKeys(key, seed []byte, opts KeyGenerationOpts) (out Construction, inputMask, outputMask matrix.Matrix) {
	constr := saes.Construction{key}
	roundKeys := constr.StretchedKey()

	// Apply ShiftRows to round keys 0 to 9.
	for k := 0; k < 10; k++ {
		constr.ShiftRows(roundKeys[k])
	}

	// Generate input and output encodings.
	switch opts.(type) {
	case IndependentMasks:
		inputMask = GenerateMask(opts.(IndependentMasks).Input, seed, Inside)
		outputMask = GenerateMask(opts.(IndependentMasks).Output, seed, Outside)
	case SameMasks:
		mask := GenerateMask(MaskType(opts.(SameMasks)), seed, Inside)
		inputMask, outputMask = mask, mask
	case MatchingMasks:
		mask := GenerateMask(RandomMask, seed, Inside)

		inputMask = mask
		outputMask, _ = mask.Invert()
	default:
		panic("Unrecognized key generation options!")
	}

	// Generate the Input Mask table and the 10th T-Box/Output Mask table
	for pos := 0; pos < 16; pos++ {
		out.InputMask[pos] = encoding.BlockTable{
			encoding.IdentityByte{},
			BlockMaskEncoding(seed, pos, Inside),
			MaskTable{inputMask, pos},
		}

		out.TBoxOutputMask[pos] = encoding.BlockTable{
			encoding.ComposedBytes{
				encoding.ByteLinear(MixingBijection(seed, 8, 8, pos)),
				ByteRoundEncoding(seed, 8, pos, Outside),
			},
			BlockMaskEncoding(seed, pos, Outside),
			table.ComposedToBlock{
				TBox{constr, roundKeys[9][pos], roundKeys[10][pos]},
				MaskTable{outputMask, pos},
			},
		}
	}

	// Generate the XOR Tables for the Input and Output Masks.
	out.InputXORTable = blockXORTables(seed, Inside)
	out.OutputXORTable = blockXORTables(seed, Outside)

	// Generate round material.
	for round := 0; round < 9; round++ {
		for pos := 0; pos < 16; pos++ {
			// Generate a word-sized mixing bijection and stick it on the end of the T-Box/Tyi Table.
			mb := MixingBijection(seed, 32, round, pos/4)

			// Build the T-Box and Tyi Table for this round and position in the state matrix.
			out.TBoxTyiTable[round][pos] = encoding.WordTable{
				encoding.ComposedBytes{
					encoding.ByteLinear(MixingBijection(seed, 8, round-1, pos)),
					encoding.ConcatenatedByte{
						RoundEncoding(seed, round-1, 2*pos+0, Outside),
						RoundEncoding(seed, round-1, 2*pos+1, Outside),
					},
				},
				encoding.ComposedWords{
					encoding.ConcatenatedWord{
						encoding.ByteLinear(MixingBijection(seed, 8, round, shiftRows(pos/4*4+0))),
						encoding.ByteLinear(MixingBijection(seed, 8, round, shiftRows(pos/4*4+1))),
						encoding.ByteLinear(MixingBijection(seed, 8, round, shiftRows(pos/4*4+2))),
						encoding.ByteLinear(MixingBijection(seed, 8, round, shiftRows(pos/4*4+3))),
					},
					encoding.WordLinear(mb),
					WordStepEncoding(seed, round, pos, Inside),
				},
				table.ComposedToWord{
					TBox{constr, roundKeys[round][pos], 0},
					TyiTable(pos % 4),
				},
			}

			// Encode the inverse of the mixing bijection from above in the MB^(-1) table for this round and position.
			mbInv, _ := mb.Invert()

			out.MBInverseTable[round][pos] = encoding.WordTable{
				encoding.ConcatenatedByte{
					RoundEncoding(seed, round, 2*pos+0, Inside),
					RoundEncoding(seed, round, 2*pos+1, Inside),
				},
				WordStepEncoding(seed, round, pos, Outside),
				MBInverseTable{mbInv, uint(pos) % 4},
			}
		}
	}

	// Generate the High and Low XOR Tables for reach round.
	out.HighXORTable = xorTables(seed, Inside)
	out.LowXORTable = xorTables(seed, Outside)

	return
}
