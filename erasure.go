// Package erasure can encode a payload into 3 sub packets, where
// any 2 of the 3 packets are sufficient to reproduce the original payload.
//
// This is useful when you want to augment durability of data without
// having to completely duplicate the payload.
//
// For instance, assume you want to persist a 1GB file to storage, with
// the ability to recover the file if a copy is corrupted.  You could use
// this package to break your 1GB file into three ~500MB blocks. You
// would then be using only 1.5GB of storage, and be able to recreate
// your original payload using any two 500mb packets.
//
// The playload is encoded by xor'ing the two half of the data and
// appending a checksum to the payload, so that errors can be detected
// and recovered automatically.
package erasure

import (
	"fmt"
	"hash/adler32"
	"unsafe"
)

func chcksum(data []byte) uint32 {
	return adler32.Checksum(data)
}

// Encode breaks data into 3 packets, needing only 2 of them
// to reconstruct the original content. The packets can be decoded
// in any order.
func Encode(data []byte) (block1, block2, block3 []byte, err error) {

	alen := uint64(len(data) / 2)

	blen := alen
	blocklen := alen
	if (len(data)/2)%2 != 0 {
		blen++
	}
	// 1 byte for order, 8 bytes for alen/blen, 4 bytes for crc32
	blocklen = blen + (1 + 8 + 4)

	// A block looks like...
	// 1            : the order of the block
	// 1 to 9       : the length of the block
	// 9 to len     : the data of the block
	// 9+len to end : the checksum of the len+data

	a := make([]byte, blocklen)
	a[0] = byte(1)                 // write the order
	uint64b(a[1:9], alen)          // write the length
	copy(a[9:9+alen], data[:alen]) // write the data from 0 to alen
	asum := chcksum(a[:blocklen-4])
	uint32b(a[blocklen-4:], asum) // write the chsksum of alen+a

	b := make([]byte, blocklen)
	b[0] = byte(2)                 // write the order
	uint64b(b[1:9], blen)          // write the length
	copy(b[9:9+blen], data[alen:]) // write the data from alen to blen
	bsum := chcksum(b[:9+blen])
	uint32b(b[blocklen-4:], bsum) // write the chsksum of blen+b

	x := make([]byte, blocklen)
	// don't need to write length or order (order == 3 because 1^2)
	xor(x[:9+blen], a[:9+blen], b[:9+blen]) // xor a with b
	xsum := chcksum(x[:blocklen-4])
	uint32b(x[blocklen-4:], xsum) // write the chsksum of the xlen+xor

	return a, b, x, nil
}

// Decode the original data from the 3 packets it was encoded with. The blocks
// can come in any order.
//
// The current implementation does not repair blocks that are detected
// broken. If a block was broken, it will be returned along with the payload.
// A user can Encode again the payload to repair the broken block and
// refresh it.
func Decode(block1, block2, block3 []byte) (result, broken []byte, err error) {

	// TODO(antoine): repair broken blocks so they can be refreshed.
	// right now the proper answer is given, but the encoded block
	// is not repaired, only the data necessary to make the payload is

	if len(block1) != len(block2) && len(block2) != len(block3) {
		return nil, nil, fmt.Errorf("blocks are of different sizes")
	}
	blocklen := len(block1)

	pos1, len1, good1 := validate(block1)
	pos2, len2, good2 := validate(block2)
	pos3, len3, good3 := validate(block3)

	switch {
	case good1 && good2:
		broken = block3
	case good1 && good3:
		broken = block2
	case good2 && good3:
		broken = block1
	}

	var (
		blockA, blockB, blockX []byte
		agood, bgood, xgood    bool
		alen, blen             uint64
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
		return nil, nil, fmt.Errorf("block A and B are bad, can't reconstruct")
	}
	if !agood && !xgood {
		return nil, nil, fmt.Errorf("block A and X are bad, can't reconstruct")
	}
	if !bgood && !xgood {
		return nil, nil, fmt.Errorf("block B and X are bad, can't reconstruct")
	}

	if agood && bgood && xgood {
		// don't need to reconstruct
		a := blockA[9 : 9+alen]
		b := blockB[9 : 9+blen]
		return append(a, b...), nil, nil
	}

	if agood && bgood && !xgood {
		// TODO(antoine): repair blockC
		a := blockA[9 : 9+alen]
		b := blockB[9 : 9+blen]

		return append(a, b...), broken, nil
	}

	if bgood && xgood {
		// TODO(antoine): repair blockA

		// reconstruct A from B and X
		a := make([]byte, blocklen-4)
		b := blockB[:blocklen-4]
		x := blockX[:blocklen-4]

		xor(a, b, x)
		// read A's len
		alen := buint64(a[1:9])

		return append(
			a[9:9+alen],
			b[9:9+blen]...,
		), broken, nil
	}

	// TODO(antoine): repair blockB

	// last case possible, B is broken
	// reconstruct B from A and X
	b := make([]byte, blocklen-4)
	a := blockA[:blocklen-4]
	x := blockX[:blocklen-4]

	xor(b, a, x)
	// read B's len
	blen = buint64(b[1:9])
	return append(
		a[9:9+alen],
		b[9:9+blen]...,
	), broken, nil
}

func validate(block []byte) (uint8, uint64, bool) {
	l := len(block)
	expected := buint32(block[l-4 : l])
	actual := chcksum(block[:l-4])
	if expected != actual {
		return 0, 0, false
	}
	return block[0], buint64(block[1:9]), true
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

func uint64b(dst []byte, u uint64) {
	dst[7] = byte((u & 0x00000000000000FF) >> 0)
	dst[6] = byte((u & 0x000000000000FF00) >> 8)
	dst[5] = byte((u & 0x0000000000FF0000) >> 16)
	dst[4] = byte((u & 0x00000000FF000000) >> 24)
	dst[3] = byte((u & 0x000000FF00000000) >> 32)
	dst[2] = byte((u & 0x0000FF0000000000) >> 40)
	dst[1] = byte((u & 0x00FF000000000000) >> 48)
	dst[0] = byte((u & 0xFF00000000000000) >> 56)
}

func buint64(b []byte) uint64 {
	var u uint64
	u |= (uint64(b[7]) << 0)
	u |= (uint64(b[6]) << 8)
	u |= (uint64(b[5]) << 16)
	u |= (uint64(b[4]) << 24)
	u |= (uint64(b[3]) << 32)
	u |= (uint64(b[2]) << 40)
	u |= (uint64(b[1]) << 48)
	u |= (uint64(b[0]) << 56)
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

	n := len(dst) / 8
	for i := 0; i < n; i++ {
		dst64[i] = blockA64[i] ^ blockB64[i]
	}
}
