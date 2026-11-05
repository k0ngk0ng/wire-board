package pcmplayer

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestInitializedSingleProfileRejectsUnprovenBankWindow(t *testing.T) {
	data := make([]byte, 7789)
	h := sha256.Sum256(data)
	if p, err := New(Config{Mode: Single, Samples: [2]*Sample{{ID: hex.EncodeToString(h[:]), Encoded: data, Rate: 3}, nil}}); err == nil || p != nil {
		t.Fatal("unproven single dummy bank window accepted")
	}
}
