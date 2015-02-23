package erasure

import (
	"testing"
)

func BenchmarkEncode_1k(b *testing.B)   { benchEncode(b, 1<<10) }
func BenchmarkEncode_512k(b *testing.B) { benchEncode(b, 1<<19) }
func BenchmarkEncode_1M(b *testing.B)   { benchEncode(b, 1<<20) }
func BenchmarkEncode_4M(b *testing.B)   { benchEncode(b, 1<<22) }

func benchEncode(b *testing.B, n int) {
	data := nBytes(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.SetBytes(int64(n))
		_, _, _, err := Encode(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecode_1k(b *testing.B)   { benchDecode(b, 1<<10) }
func BenchmarkDecode_512k(b *testing.B) { benchDecode(b, 1<<19) }
func BenchmarkDecode_1M(b *testing.B)   { benchDecode(b, 1<<20) }
func BenchmarkDecode_4M(b *testing.B)   { benchDecode(b, 1<<22) }

func benchDecode(b *testing.B, n int) {
	data := nBytes(n)
	blockA, blockB, blockC, err := Encode(data)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.SetBytes(int64(n))
		_, err := Decode(blockA, blockB, blockC)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeRepairData_1k(b *testing.B)   { benchDecodeRepairData(b, 1<<10) }
func BenchmarkDecodeRepairData_512k(b *testing.B) { benchDecodeRepairData(b, 1<<19) }
func BenchmarkDecodeRepairData_1M(b *testing.B)   { benchDecodeRepairData(b, 1<<20) }
func BenchmarkDecodeRepairData_4M(b *testing.B)   { benchDecodeRepairData(b, 1<<22) }

func benchDecodeRepairData(b *testing.B, n int) {
	data := nBytes(n)
	blockA, blockB, blockC, err := Encode(data)
	if err != nil {
		b.Fatal(err)
	}
	// block B is not the xor part
	blockB = flipbits(blockB, n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.SetBytes(int64(n))
		_, err := Decode(blockA, blockB, blockC)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeRepairXOR_1k(b *testing.B)   { benchDecodeRepairXOR(b, 1<<10) }
func BenchmarkDecodeRepairXOR_512k(b *testing.B) { benchDecodeRepairXOR(b, 1<<19) }
func BenchmarkDecodeRepairXOR_1M(b *testing.B)   { benchDecodeRepairXOR(b, 1<<20) }
func BenchmarkDecodeRepairXOR_4M(b *testing.B)   { benchDecodeRepairXOR(b, 1<<22) }

func benchDecodeRepairXOR(b *testing.B, n int) {
	data := nBytes(n)
	blockA, blockB, blockC, err := Encode(data)
	if err != nil {
		b.Fatal(err)
	}
	// block C contains the xored data
	blockC = flipbits(blockC, n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.SetBytes(int64(n))
		_, err := Decode(blockA, blockB, blockC)
		if err != nil {
			b.Fatal(err)
		}
	}
}
