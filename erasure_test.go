package erasure

import (
	"github.com/dustin/randbo"
	"math/rand"
	"testing"
)

func TestBinEncoding(t *testing.T) {
	tests := []uint32{
		0,
		1,
		2,
		3,
		0x01234567,
		1 << 31,
	}

	b := make([]byte, 32/4)
	for _, want := range tests {
		uint32b(b, want)
		got := buint32(b)
		t.Logf("%x", b)
		if want != got {
			t.Errorf("want %x got %x", want, got)
		}
	}
}

func TestXOR(t *testing.T) {
	tests := []struct {
		A string
		B string
	}{
		{
			"hello  ",
			"bonjour",
		},
	}

	for _, tt := range tests {
		a := []byte(tt.A)
		b := []byte(tt.B)
		if len(a) != len(b) {
			t.Fatalf("len(a)=%d != len(b)=%d", len(a), (b))
		}
		x := make([]byte, len(a))
		recA := make([]byte, len(a))
		recB := make([]byte, len(a))
		xor(x, a, b)
		xor(recA, x, b)
		xor(recB, x, a)

		wantA, gotA := tt.A, string(recA)
		wantB, gotB := tt.B, string(recB)

		t.Logf("\na=%x\nb=%x\nx=%x", a, b, x)
		t.Logf("   x=%q", string(x))
		t.Logf("gotA=%q", gotA)
		t.Logf("gotB=%q", gotB)
		if wantA != gotA {
			t.Errorf("want A %q, got %q", wantA, gotA)
		}
		if wantB != gotB {
			t.Errorf("want B %q, got %q", wantB, gotB)
		}
	}
}

func TestCantDecodeBadBlocks(t *testing.T) {
	block1, block2, block3 := make([]byte, 1), make([]byte, 2), make([]byte, 3)
	_, _, err := Decode(block1, block2, block3)
	if err == nil {
		t.Errorf("should have gotten an error when decoding blocks of wrong size")
	}
}

func TestEncodeDecode(t *testing.T) {

	tests := []string{
		"hello there",
		"hello, there",
	}

	if !testing.Short() {
		tests = append(tests, string(nBytes(1<<10)))
	}

	for _, tt := range tests {
		data := []byte(tt)
		blockA, blockB, blockX, err := Encode(data)
		if err != nil {
			t.Fatal(err)
		}
		// can decode in any order
		maxFlipBits := len(data) * 4
		testEncodeDecode(t, data, blockA, blockB, blockX, maxFlipBits)
		testEncodeDecode(t, data, blockA, blockX, blockB, maxFlipBits)
		testEncodeDecode(t, data, blockB, blockA, blockX, maxFlipBits)
		testEncodeDecode(t, data, blockB, blockX, blockA, maxFlipBits)
		testEncodeDecode(t, data, blockX, blockA, blockB, maxFlipBits)
		testEncodeDecode(t, data, blockX, blockB, blockA, maxFlipBits)
	}
}

func testEncodeDecode(t *testing.T, data, blockA, blockB, blockX []byte, maxFlipBits int) {

	// normal case
	gotData, broken, err := Decode(blockA, blockB, blockX)
	if err != nil {
		t.Fatalf("couldn't decode: %v", err)
	} else if broken != nil {
		t.Fatalf("should not have found a broken block: %x", broken)
	} else {
		want, got := string(data), string(gotData)
		if want != got {
			t.Logf("want=%x", want)
			t.Logf(" got=%x", got)
			t.Errorf("want %q got %q", want, got)
			return
		}
	}

	emptyBlock := make([]byte, len(blockA))

	missingBlock := []struct {
		Name string
		A    []byte
		B    []byte
		X    []byte
	}{
		{Name: "A has no data", A: emptyBlock, B: blockB, X: blockX},
		{Name: "B has no data", A: blockA, B: emptyBlock, X: blockX},
		{Name: "X has no data", A: blockA, B: blockB, X: emptyBlock},
	}

	for _, ett := range missingBlock {
		t.Logf("===> %s", ett.Name)
		gotData, broken, err = Decode(ett.A, ett.B, ett.X)
		if err != nil {
			t.Errorf("couldn't decode: %v", err)
		}
		if broken == nil {
			t.Errorf("should have found a broken block")
		}
		want, got := string(data), string(gotData)
		if want != got {
			t.Logf("want=%x", want)
			t.Logf(" got=%x", got)
			t.Errorf("want %q got %q", want, got)
		}
	}

	nop := func(b []byte) func(int) []byte {
		return func(int) []byte { return b }
	}
	flipper := func(str string, b []byte) func(int) []byte {
		return func(n int) []byte {
			return flipbits(b, n)
		}
	}

	// should recover if 1 block is broken
	errorBlock := []struct {
		Name string
		A    func(int) []byte
		B    func(int) []byte
		X    func(int) []byte
	}{
		{Name: "A has flipped bits", A: flipper("A", blockA), B: nop(blockB), X: nop(blockX)},
		{Name: "B has flipped bits", A: nop(blockA), B: flipper("B", blockB), X: nop(blockX)},
		{Name: "C has flipped bits", A: nop(blockA), B: nop(blockB), X: flipper("X", blockX)},
	}
	for _, ett := range errorBlock {
		t.Logf("===> %s", ett.Name)
		for i := 1; i < maxFlipBits; i++ {
			blockA := ett.A(i)
			blockB := ett.B(i)
			blockX := ett.X(i)

			gotData, broken, err = Decode(blockA, blockB, blockX)
			if err != nil {
				t.Errorf("couldn't decode: %v", err)
			}
			if broken == nil {
				t.Errorf("should have found a broken payload")
			}
			want, got := string(data), string(gotData)
			if want != got {
				t.Logf("want=%x", want)
				t.Logf(" got=%x", got)
				t.Errorf("want %q got %q", want, got)
			}

		}
	}

	// should error if 2 block are broken
	tooManyErrors := []struct {
		A   func(int) []byte
		B   func(int) []byte
		X   func(int) []byte
		Err error
	}{
		{A: flipper("A", blockA), B: flipper("B", blockB), X: nop(blockX)},
		{A: nop(blockA), B: flipper("B", blockB), X: flipper("X", blockX)},
		{A: flipper("A", blockA), B: nop(blockB), X: flipper("X", blockX)},
	}
	for _, ett := range tooManyErrors {
		for i := 1; i < maxFlipBits; i++ {
			blockA := ett.A(i)
			blockB := ett.B(i)
			blockX := ett.X(i)

			gotData, broken, err = Decode(blockA, blockB, blockX)
			if err == nil {
				t.Errorf("should have had an error")
			} else if broken != nil {
				t.Errorf("should not have found a broken block, since 2 are broken")
			}
			if want, got := string(data), string(gotData); want == got {
				t.Logf("want=%x", want)
				t.Logf(" got=%x", got)
				t.Errorf("should not have reconstructed string")
			}
		}
	}
}

func flipbits(b []byte, n int) []byte {
	bits := rand.Perm(len(b) * 8)[:n]
	cp := make([]byte, len(b))
	copy(cp, b)

	for _, toflip := range bits {
		Bidx := (toflip / 8)
		bit := (toflip % 8)
		cp[Bidx] ^= (1 << uint(bit))
	}

	return cp
}

func nBytes(n int) []byte {
	p := make([]byte, n)
	_, err := randbo.New().Read(p)
	if err != nil {
		panic(err)
	}
	return p
}
