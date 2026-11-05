package pcmassets

// A music flag of zero is valid. Missing/null must not silently become that
// valid value. This review regression precedes the strict-field implementation.
import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestRequiredZeroValuedMusicFields(t *testing.T) {
	_, raw, music, _ := load(t)
	for _, domain := range []string{"import", "score"} {
		for _, kind := range []string{"missing", "null"} {
			t.Run(domain+"/"+kind, func(t *testing.T) {
				ordinary, source := object(t, raw), object(t, music)
				var row map[string]any
				if domain == "import" {
					row = ordinary["music_import"].(map[string]any)["descriptors"].([]any)[0].(map[string]any)
				} else {
					row = source["dac_descriptors"].([]any)[0].(map[string]any)
				}
				if kind == "missing" {
					delete(row, "flags")
				} else {
					row["flags"] = nil
				}
				changed := music
				if domain == "score" {
					changed = encode(t, source)
					sum := sha256.Sum256(changed)
					ref := ordinary["music_import"].(map[string]any)
					ref["raw_json_sha256"] = hex.EncodeToString(sum[:])
					ref["raw_json_bytes"] = len(changed)
				}
				expectRejected(t, encode(t, ordinary), changed)
			})
		}
	}
}
