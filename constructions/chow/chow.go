package chow

import (
	"github.com/OpenWhiteBox/AES/primitives/table"
)

type Construction struct {
	InputMask     [16]table.Block      // [round]
	InputXORTable [32][15]table.Nibble // [nibble-wise position][gate number]

	TBoxTyiTable [9][16]table.Word      // [round][position]
	HighXORTable [9][32][3]table.Nibble // [round][nibble-wise position][gate number]

	MBInverseTable [9][16]table.Word      // [round][position]
	LowXORTable    [9][32][3]table.Nibble // [round][nibble-wise position][gate number]

	TBoxOutputMask [16]table.Block      // [position]
	OutputXORTable [32][15]table.Nibble // [nibble-wise position][gate number]
}

func (constr Construction) BlockSize() int { return 16 }

func (constr Construction) Encrypt(dst, src []byte) {
	constr.crypt(dst, src, constr.ShiftRows)
}

func (constr Construction) Decrypt(dst, src []byte) {
	constr.crypt(dst, src, constr.UnShiftRows)
}

func (constr Construction) crypt(dst, src []byte, shift func([]byte)) {
	copy(dst, src)

	// Remove input encoding.
	stretched := constr.ExpandBlock(constr.InputMask, dst)
	constr.SquashBlocks(constr.InputXORTable, stretched, dst)

	for round := 0; round < 9; round++ {
		shift(dst)

		// Apply the T-Boxes and Tyi Tables to each column of the state matrix.
		for pos := 0; pos < 16; pos += 4 {
			stretched := constr.ExpandWord(constr.TBoxTyiTable[round][pos:pos+4], dst[pos:pos+4])
			constr.SquashWords(constr.HighXORTable[round][2*pos:2*pos+8], stretched, dst[pos:pos+4])

			stretched = constr.ExpandWord(constr.MBInverseTable[round][pos:pos+4], dst[pos:pos+4])
			constr.SquashWords(constr.LowXORTable[round][2*pos:2*pos+8], stretched, dst[pos:pos+4])
		}
	}

	shift(dst)

	// Apply the final T-Box transformation and add the output encoding.
	stretched = constr.ExpandBlock(constr.TBoxOutputMask, dst)
	constr.SquashBlocks(constr.OutputXORTable, stretched, dst)
}

func (constr *Construction) ShiftRows(block []byte) {
	copy(block, []byte{
		block[0], block[5], block[10], block[15], block[4], block[9], block[14], block[3], block[8], block[13], block[2],
		block[7], block[12], block[1], block[6], block[11],
	})
}

func (constr *Construction) UnShiftRows(block []byte) {
	copy(block, []byte{
		block[0], block[13], block[10], block[7], block[4], block[1], block[14], block[11], block[8], block[5], block[2],
		block[15], block[12], block[9], block[6], block[3],
	})
}

// Expand one word of the state matrix with the T-Boxes composed with Tyi Tables.
func (constr *Construction) ExpandWord(tboxtyi []table.Word, word []byte) [4][4]byte {
	return [4][4]byte{tboxtyi[0].Get(word[0]), tboxtyi[1].Get(word[1]), tboxtyi[2].Get(word[2]), tboxtyi[3].Get(word[3])}
}

// Squash an expanded word back into one word with 3 pairwise XORs (calc'd one nibble at a time) -- (((a ^ b) ^ c) ^ d)
func (constr *Construction) SquashWords(xorTable [][3]table.Nibble, words [4][4]byte, dst []byte) {
	copy(dst, words[0][:])

	for i := 1; i < 4; i++ {
		for pos := 0; pos < 4; pos++ {
			aPartial := dst[pos]&0xf0 | (words[i][pos]&0xf0)>>4
			bPartial := (dst[pos]&0x0f)<<4 | words[i][pos]&0x0f

			dst[pos] = xorTable[2*pos+0][i-1].Get(aPartial)<<4 | xorTable[2*pos+1][i-1].Get(bPartial)
		}
	}
}

func (constr *Construction) ExpandBlock(mask [16]table.Block, block []byte) (out [16][16]byte) {
	for i := 0; i < 16; i++ {
		out[i] = mask[i].Get(block[i])
	}

	return
}

func (constr *Construction) SquashBlocks(xorTable [32][15]table.Nibble, blocks [16][16]byte, dst []byte) {
	copy(dst, blocks[0][:])

	for i := 1; i < 16; i++ {
		for pos := 0; pos < 16; pos++ {
			aPartial := dst[pos]&0xf0 | (blocks[i][pos]&0xf0)>>4
			bPartial := (dst[pos]&0x0f)<<4 | blocks[i][pos]&0x0f

			dst[pos] = xorTable[2*pos+0][i-1].Get(aPartial)<<4 | xorTable[2*pos+1][i-1].Get(bPartial)
		}
	}
}
