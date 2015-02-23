package erasure

import (
	"github.com/dustin/randbo"
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

func nBytes(n int) []byte {
	p := make([]byte, n)
	_, err := randbo.New().Read(p)
	if err != nil {
		panic(err)
	}
	return p
}
