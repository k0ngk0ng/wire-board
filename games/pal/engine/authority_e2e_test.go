package engine_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

func TestOriginalAuthorityArchiveEquivalentAndSavedContinuation(t *testing.T) {
	if os.Getenv("PAL_AUTHORITY_E2E") != "1" {
		t.Skip("explicit original data and prepared authority archive equivalence")
	}
	ctx := context.Background()
	archive, err := os.ReadFile("../../../.local/pal/authority-prepared/authority.zip")
	if err != nil {
		t.Fatal(err)
	}
	open := func(profile string, prepared bool) *engine.Runtime {
		t.Helper()
		program, err := os.ReadFile("../../../.local/pal/runtime-candidates/audio/" + profile + "/rules.wasm")
		if err != nil {
			t.Fatal(err)
		}
		var r *engine.Runtime
		if prepared {
			r, err = engine.NewAuthorityRuntime(ctx, program, archive)
		} else {
			r, err = engine.NewRuntime(ctx, program, os.DirFS("../../../.local/pal/original-98"))
		}
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { r.Close(ctx) })
		return r
	}
	raw, prepared := open("original", false), open("original", true)
	left, err := raw.StartCampaignForPlayers(ctx, 12345, 1)
	if err != nil {
		t.Fatal(err)
	}
	right, err := prepared.StartCampaignForPlayers(ctx, 12345, 1)
	if err != nil {
		t.Fatal(err)
	}
	pages, saves := 0, 0
	for step := 0; step < 4096; step++ {
		if !reflect.DeepEqual(left, right) {
			t.Fatal("prepared authority changed native memory, text, drawing, audio or resource identity", step)
		}
		if left.Event.Kind == engine.Dialogue {
			pages++
			if pages == 1 || pages == 8 || pages == 23 {
				encoded, err := raw.Encode(left)
				if err != nil {
					t.Fatal(err)
				}
				fresh := open("original", true)
				right, err = fresh.Decode(encoded)
				if err != nil || !reflect.DeepEqual(right, left) {
					t.Fatal("raw-resource save did not continue unchanged through the authority archive", err)
				}
				prepared = fresh
				saves++
			}
		}
		if pages > 0 && left.Event.Kind == engine.Frame && left.View.Mode == engine.FieldMode && !left.View.Entering {
			break
		}
		response := uint32(0)
		if left.Event.Kind == engine.Dialogue {
			response = 1
		}
		left, err = raw.Step(ctx, left, response)
		if err != nil {
			t.Fatal(err)
		}
		right, err = prepared.Step(ctx, right, response)
		if err != nil {
			t.Fatal(err)
		}
	}
	if pages != 23 || saves != 3 {
		t.Fatal("prepared authority opening did not reach the original first field", pages, saves)
	}
	data, err := os.ReadFile("../../../.local/pal/verification/runtime-candidates/audio/development-health/hp-9999/campaign-shu-tower-state-1.json")
	if err != nil {
		t.Fatal(err)
	}
	var checkpoint struct {
		Native []byte `json:"native"`
	}
	if err = json.Unmarshal(data, &checkpoint); err != nil {
		t.Fatal(err)
	}
	developmentRaw, developmentPrepared := open("hp-9999", false), open("hp-9999", true)
	a, err := developmentRaw.Decode(checkpoint.Native)
	if err != nil {
		t.Fatal(err)
	}
	b, err := developmentPrepared.Decode(checkpoint.Native)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("prepared authority altered the genuine stage15 save", err)
	}
	a, err = developmentRaw.Step(ctx, a, 0)
	if err != nil {
		t.Fatal(err)
	}
	b, err = developmentPrepared.Step(ctx, b, 0)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("prepared authority diverged from the genuine stage15 continuation", err)
	}
	for i := range archive {
		archive[i] ^= 0xff
	}
	next, err := developmentPrepared.Step(ctx, b, 0)
	expected, referenceErr := developmentRaw.Step(ctx, a, 0)
	if err != nil || referenceErr != nil || !reflect.DeepEqual(next, expected) {
		t.Fatal("caller-owned archive bytes affected a running authority", err, referenceErr)
	}
	if !bytes.Equal(data, mustReadAuthoritySource(t)) {
		t.Fatal("prepared authority changed the genuine stored source")
	}
	hash := sha256.Sum256(data)
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "all native memory/projection equality through original23-page new game, three fresh prepared-runtime raw-save continuations, genuine stage15 development save and subsequent native steps, owned bytes and unchanged archive source; no relabelled native identity or production registration claim", "pages": pages, "saves": saves, "resourceID": left.ResourceID, "originalBuildID": left.BuildID, "developmentBuildID": b.BuildID, "stage15SourceSHA256": hex.EncodeToString(hash[:])}, "", "  ")
	if err = os.WriteFile("../../../.local/pal/verification/authority-archive-equivalence.json", append(report, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}

func mustReadAuthoritySource(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../../.local/pal/verification/runtime-candidates/audio/development-health/hp-9999/campaign-shu-tower-state-1.json")
	if err != nil {
		t.Fatal(err)
	}
	return data
}
