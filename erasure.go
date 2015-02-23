package erasure

import (
	"fmt"
	"hash/adler32"
	"unsafe"
)

func chcksum(data []byte) uint32 {
	return adler32.Checksum(data)
	// return crc32.ChecksumIEEE(data)
}

// Encode breaks data into 3 packets, needing only 2 of them
// to reconstruct the original content. The order of the encoded blocks
// doesn't matter and can be decoded in any order.
func Encode(data []byte) (blockA, blockB, coding []byte, err error) {

	alen := uint32(len(data) / 2)

	blen := alen
	blocklen := alen
	if (len(data)/2)%2 != 0 {
		blen++
	}
	// 1 byte for order, 4 bytes for alen/blen, 4 bytes for crc32
	blocklen = blen + (1 + 4 + 4)

	// A block looks like...
	// 1            : the order of the block
	// 1 to 5       : the length of the block
	// 5 to len     : the data of the block
	// 5+len to end : the checksum of the len+data

	a := make([]byte, blocklen)
	a[0] = byte(1)                 // write the order
	uint32b(a[1:5], alen)          // write the length
	copy(a[5:5+alen], data[:alen]) // write the data from 0 to alen
	asum := chcksum(a[:blocklen-4])
	uint32b(a[blocklen-4:], asum) // write the chsksum of alen+a

	b := make([]byte, blocklen)
	b[0] = byte(2)                 // write the order
	uint32b(b[1:5], blen)          // write the length
	copy(b[5:5+blen], data[alen:]) // write the data from alen to blen
	bsum := chcksum(b[:5+blen])
	uint32b(b[blocklen-4:], bsum) // write the chsksum of blen+b

	x := make([]byte, blocklen)
	// don't need to write length or order (order == 3 because 1^2)
	xor(x[:5+blen], a[:5+blen], b[:5+blen]) // xor a with b
	xsum := chcksum(x[:blocklen-4])
	uint32b(x[blocklen-4:], xsum) // write the chsksum of the xlen+xor

	return a, b, x, nil
}

// Decode the original data from the 3 packets it was encoded with. The blocks
// can come in any order.
//
// TODO(antoine): repair broken blocks so they can be refreshed
func Decode(block1, block2, block3 []byte) ([]byte, error) {

	if len(block1) != len(block2) && len(block2) != len(block3) {
		return nil, fmt.Errorf("blocks are of different sizes")
	}
	blocklen := len(block1)

	pos1, len1, good1 := validate(block1)
	pos2, len2, good2 := validate(block2)
	pos3, len3, good3 := validate(block3)

	var (
		blockA, blockB, blockX []byte
		agood, bgood, xgood    bool
		alen, blen             uint32
	)

	switch pos1 {
	case 0:
	case 1:
		blockA, agood, alen = block1, good1, len1
	case 2:
		blockB, bgood, blen = block1, good1, len1
	case 3:
		blockX, xgood = block1, good1
	}

	switch pos2 {
	case 0:
	case 1:
		blockA, agood, alen = block2, good2, len2
	case 2:
		blockB, bgood, blen = block2, good2, len2
	case 3:
		blockX, xgood = block2, good2
	}

	switch pos3 {
	case 0:
	case 1:
		blockA, agood, alen = block3, good3, len3
	case 2:
		blockB, bgood, blen = block3, good3, len3
	case 3:
		blockX, xgood = block3, good3
	}

	// bad cases first
	if !agood && !bgood {
		return nil, fmt.Errorf("block A and B are bad, can't reconstruct")
	}
	if !agood && !xgood {
		return nil, fmt.Errorf("block A and X are bad, can't reconstruct")
	}
	if !bgood && !xgood {
		return nil, fmt.Errorf("block B and X are bad, can't reconstruct")
	}

	if agood && bgood {
		// don't need to reconstruct
		a := blockA[5 : 5+alen]
		b := blockB[5 : 5+blen]
		return append(a, b...), nil
	}

	if bgood && xgood {
		// reconstruct A from B and X
		a := make([]byte, blocklen-4)
		b := blockB[:blocklen-4]
		x := blockX[:blocklen-4]

		xor(a, b, x)
		// read A's len
		alen := buint32(a[1:5])
		return append(
			a[5:5+alen],
			b[5:5+blen]...,
		), nil
	}

	if agood && xgood {
		// reconstruct B from A and X
		b := make([]byte, blocklen-4)
		a := blockA[:blocklen-4]
		x := blockX[:blocklen-4]

		xor(b, a, x)
		// read B's len
		blen = buint32(b[1:5])
		return append(
			a[5:5+alen],
			b[5:5+blen]...,
		), nil
	}

	panic("unreachable")
}

func validate(block []byte) (uint8, uint32, bool) {
	l := len(block)
	expected := buint32(block[l-4 : l])
	actual := chcksum(block[:l-4])
	if expected != actual {
		return 0, 0, false
	}
	return block[0], buint32(block[1:5]), true
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

	extra := len(dst) % 8
	for i, a := range blockA[:extra] {
		dst[i] = a ^ blockB[i]
	}
	dst = dst[extra:]
	blockA = blockA[extra:]
	blockB = blockB[extra:]

	fast64bitsXor(dst, blockA, blockB)
}

func fast64bitsXor(dst, blockA, blockB []byte) {
	dst64 := *(*[]uint64)(unsafe.Pointer(&dst))
	blockA64 := *(*[]uint64)(unsafe.Pointer(&blockA))
	blockB64 := *(*[]uint64)(unsafe.Pointer(&blockB))

	// for i, a := range blockA64 {
	// 	b := blockB64[i]
	// 	dst64[i] = a ^ b
	// }

	n := len(dst) / 8
	for i := 0; i < n; i++ {
		dst64[i] = blockA64[i] ^ blockB64[i]
	}
}
