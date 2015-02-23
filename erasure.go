package erasure

import (
	"fmt"
	"hash/crc32"
)

// Encode breaks data into 3 packets, needing only 2 of them
// to reconstruct the original content.
func Encode(data []byte) (blockA, blockB, coding []byte) {

	alen := uint32(len(data) / 2)

	blen := alen
	blocklen := alen
	if (len(data)/2)%2 != 0 {
		blen++
		blocklen = blen
	}

	// 4 bytes for crc32, 4 bytes for alen/blen
	blocklen += (4 + 4)

	// a block looks like
	// 0 to 4       : the length of the block
	// 4 to len     : the data of the block
	// 4+len to end : the checksum of the len+data

	a := make([]byte, blocklen)
	uint32b(a[:4], alen)           // write the length
	copy(a[4:4+alen], data[:alen]) // write the data from 0 to alen
	asum := crc32.ChecksumIEEE(a[:blocklen-4])
	uint32b(a[blocklen-4:], asum) // write the chsksum of alen+a

	b := make([]byte, blocklen)
	uint32b(b[:4], blen)           // write the length
	copy(b[4:4+blen], data[alen:]) // write the data from alen to blen
	bsum := crc32.ChecksumIEEE(b[:4+blen])
	uint32b(b[blocklen-4:], bsum) // write the chsksum of blen+b

	x := make([]byte, blocklen)
	uint32b(x[:4], blen)                    // write the length
	xor(x[:4+blen], a[:4+blen], b[:4+blen]) // xor a with b
	xsum := crc32.ChecksumIEEE(x[:blocklen-4])
	uint32b(x[blocklen-4:], xsum) // write the chsksum of the xlen+xor

	return a, b, x
}

// Decode the original data from the 3 packets it was encoded with.
func Decode(blockA, blockB, blockX []byte) ([]byte, error) {

	if len(blockA) != len(blockB) && len(blockB) != len(blockX) {
		return nil, fmt.Errorf("blocks are of different sizes")
	}
	blocklen := len(blockA)

	alen, agood := validate(blockA)
	blen, bgood := validate(blockB)

	if agood && bgood {
		// don't need to reconstruct
		a := blockA[4 : 4+alen]
		b := blockB[4 : 4+blen]
		return append(a, b...), nil
	}

	if !agood && !bgood {
		// can't possibly reconstruct
		return nil, fmt.Errorf("block A and B are bad, can't reconstruct")
	}

	_, xgood := validate(blockX)

	// bad cases first
	if !agood && !xgood {
		// can't reconstruct A without X block
		return nil, fmt.Errorf("block A and X are bad, can't reconstruct")
	}
	if !bgood && !xgood {
		// can't reconstruct B without X block
		return nil, fmt.Errorf("block B and X are bad, can't reconstruct")
	}

	// one of A or B is good
	a := blockA[:blocklen-4]
	b := blockB[:blocklen-4]
	x := blockX[:blocklen-4]

	if bgood {
		// reconstruct A from B and X
		xor(a, b, x)
		// read A's len
		alen := buint32(a[:4])
		return append(a[4:4+alen], b[4:4+blen]...), nil
	}
	// a was good

	// reconstruct B from A and X
	xor(b, a, x)
	// read B's len
	blen = buint32(b[:4])
	return append(a[4:4+alen], b[4:4+blen]...), nil
}

func validate(block []byte) (uint32, bool) {
	l := len(block)
	expected := buint32(block[l-4 : l])
	actual := crc32.ChecksumIEEE(block[:l-4])
	if expected != actual {
		return 0, false
	}
	return buint32(block[0:4]), true
}

func uint32b(dst []byte, u uint32) {
	dst[3] = byte((u & 0x000000FF) >> 0)
	dst[2] = byte((u & 0x0000FF00) >> 8)
	dst[1] = byte((u & 0x00FF0000) >> 16)
	dst[0] = byte((u & 0xFF000000) >> 24)
}

func buint32(b []byte) uint32 {
	var u uint32
	u |= (uint32(b[3]) << 0)
	u |= (uint32(b[2]) << 8)
	u |= (uint32(b[1]) << 16)
	u |= (uint32(b[0]) << 24)
	return u
}

func xor(dst, blockA, blockB []byte) {
	// could be made faster with unrolling
	// and working on uint64
	for i, a := range blockA {
		b := blockB[i]
		dst[i] = a ^ b
	}
}
