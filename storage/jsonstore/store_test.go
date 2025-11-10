package jsonstore

import "testing"

func TestChunkString(t *testing.T) {
	value := "abcdefghijklmnopqrstuvwxyz"
	chunks := chunkString(value, 5)
	if len(chunks) != 6 {
		t.Fatalf("expected 6 chunks, got %d", len(chunks))
	}
	joined := ""
	for _, c := range chunks {
		if len(c) > 5 {
			t.Fatalf("chunk too large: %s", c)
		}
		joined += c
	}
	if joined != value {
		t.Fatalf("expected %s, got %s", value, joined)
	}
}

func TestEncodeDecodePayload(t *testing.T) {
	original := []byte("reminder payload with lots of repeating text: 1234567890--" +
		"repeat repeat repeat repeat repeat repeat repeat")

	encoded, err := encodePayload(original)
		if err != nil {
			t.Fatalf("encode failed: %v", err)
		}

	decoded, err := decodePayload(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if string(decoded) != string(original) {
		t.Fatalf("expected %s, got %s", original, decoded)
	}
}
