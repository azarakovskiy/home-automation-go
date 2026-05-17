package noise

import "testing"

func TestNewGenerator_unknownType(t *testing.T) {
	_, err := NewGenerator("ogg")
	if err == nil {
		t.Fatal("expected error for unknown type, got nil")
	}
}

func TestWhiteGenerator_fill(t *testing.T) {
	gen, err := NewGenerator("white")
	if err != nil {
		t.Fatalf("NewGenerator: %v", err)
	}
	buf := make([]int16, 4096)
	gen.Fill(buf)

	allZero := true
	for _, s := range buf {
		if s != 0 {
			allZero = false
		}
	}
	if allZero {
		t.Fatal("all samples are zero — generator appears broken")
	}
}

func TestPinkGenerator_fill(t *testing.T) {
	gen, err := NewGenerator("pink")
	if err != nil {
		t.Fatalf("NewGenerator: %v", err)
	}
	buf := make([]int16, 4096)
	gen.Fill(buf)

	allZero := true
	for _, s := range buf {
		if s != 0 {
			allZero = false
		}
	}
	if allZero {
		t.Fatal("all samples are zero — generator appears broken")
	}
}

func TestBrownGenerator_fill(t *testing.T) {
	gen, err := NewGenerator("brown")
	if err != nil {
		t.Fatalf("NewGenerator: %v", err)
	}
	buf := make([]int16, 4096)
	gen.Fill(buf)

	allZero := true
	for _, s := range buf {
		if s != 0 {
			allZero = false
		}
	}
	if allZero {
		t.Fatal("all samples are zero — generator appears broken")
	}
}
