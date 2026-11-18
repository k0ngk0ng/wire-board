package engine_test

// E2E before a runtime catalog: loading a real prior save runs current native
// code, rewrites build IDs, loses a suspended original page/menu/RNG/resources,
// accepts a missing resource version or corrupt stream, mixes development HP
// with original rules, starts new games with the retired core, shares mutable
// transactions, or accepts an unclassified verification core. Candidate cores
// have a compiled provenance ABI and genuine distinct binary identities.
import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

// A manifest alone cannot relabel development/fixture code as original rules:
// the embedded stamp must also match the core's compiled read-only provenance.
func TestOriginalRuntimeCompiledProfileRejectsForgedStamp(t *testing.T) {
	if os.Getenv("PAL_VERSION_CATALOG") != "1" {
		t.Skip("explicit native build provenance acceptance")
	}
	program, err := os.ReadFile("../../../.local/pal/runtime-candidates/original/rules.wasm")
	if err != nil {
		t.Fatal(err)
	}
	files := os.DirFS("../../../.local/pal/original-98")
	for _, change := range []func(map[string]any){
		func(m map[string]any) { m["profile"] = "development-health:9999" },
		func(m map[string]any) { m["purpose"] = "battle-fixture" },
		func(m map[string]any) { m["format"] = 2 },
		func(m map[string]any) { m["sourceCommit"] = "bad" },
		func(m map[string]any) { m["unknown"] = true },
	} {
		bad := replaceCatalogStamp(t, program, change)
		if r, e := engine.NewRuntime(context.Background(), bad, files); e == nil {
			r.Close(context.Background())
			t.Fatal("forged build stamp accepted")
		}
	}
	development, err := os.ReadFile("../../../.local/pal/runtime-candidates/hp-9999/rules.wasm")
	if err != nil {
		t.Fatal(err)
	}
	masked := replaceCatalogStamp(t, development, func(m map[string]any) { m["profile"] = "original-98" })
	if r, e := engine.NewRuntime(context.Background(), masked, files); e == nil {
		r.Close(context.Background())
		t.Fatal("development native body was masked as original rules")
	}
}

func replaceCatalogStamp(t *testing.T, program []byte, change func(map[string]any)) []byte {
	t.Helper()
	read := func(data []byte, pos *int) uint32 {
		var value uint32
		for shift := uint(0); shift < 35; shift += 7 {
			if *pos >= len(data) {
				t.Fatal("truncated test wasm")
			}
			b := data[*pos]
			*pos++
			value |= uint32(b&127) << shift
			if b&128 == 0 {
				return value
			}
		}
		t.Fatal("invalid test wasm length")
		return 0
	}
	write := func(value uint32) []byte {
		out := []byte{}
		for {
			b := byte(value & 127)
			value >>= 7
			if value != 0 {
				b |= 128
			}
			out = append(out, b)
			if value == 0 {
				return out
			}
		}
	}
	for pos := 8; pos < len(program); {
		start := pos
		kind := program[pos]
		pos++
		size := int(read(program, &pos))
		end := pos + size
		if end > len(program) {
			t.Fatal("truncated test wasm section")
		}
		if kind == 0 {
			n := int(read(program, &pos))
			if pos+n > end {
				t.Fatal("invalid test custom name")
			}
			if string(program[pos:pos+n]) == "wire-board.pal.build" {
				var meta map[string]any
				if err := json.Unmarshal(program[pos+n:end], &meta); err != nil {
					t.Fatal(err)
				}
				change(meta)
				raw, err := json.Marshal(meta)
				if err != nil {
					t.Fatal(err)
				}
				name := []byte("wire-board.pal.build")
				body := append(write(uint32(len(name))), name...)
				body = append(body, raw...)
				result := bytes.Clone(program[:start])
				result = append(result, 0)
				result = append(result, write(uint32(len(body)))...)
				result = append(result, body...)
				return append(result, program[end:]...)
			}
		}
		pos = end
	}
	t.Fatal("candidate has no PAL build stamp")
	return nil
}

type catalogOverlay struct {
	fs.FS
	word []byte
}

func (f catalogOverlay) Open(name string) (fs.File, error) {
	if name == "word.dat" {
		return fstest.MapFS{"word.dat": &fstest.MapFile{Data: f.word}}.Open(name)
	}
	return f.FS.Open(name)
}

func catalogRuntime(t *testing.T, path string, files fs.FS) *engine.Runtime {
	t.Helper()
	program, err := os.ReadFile(filepath.Join("../../..", path))
	if err != nil {
		t.Fatal("build the explicit versioned candidate first", err)
	}
	r, err := engine.NewRuntime(context.Background(), program, files)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(context.Background()) })
	return r
}

func TestOriginalRetainedRuntimeCatalogContinuations(t *testing.T) {
	if os.Getenv("PAL_VERSION_CATALOG") != "1" {
		t.Skip("explicit retained native version acceptance")
	}
	ctx := context.Background()
	files := os.DirFS("../../../.local/pal/original-98")
	old := catalogRuntime(t, ".local/pal/runtime-versions/7ca83b5bcf13e71c1020ef287d9817e5675adf2cbdf7c627644f4cb3566dd03a/rules.wasm", files)
	current := catalogRuntime(t, ".local/pal/runtime-candidates/original/rules.wasm", files)
	oldDev := catalogRuntime(t, ".local/pal/runtime-versions/68d893938bc623a69a33d9ca402741b04a7be2191c7349f4081b5e94b755bec7/rules.wasm", files)
	newDev := catalogRuntime(t, ".local/pal/runtime-candidates/hp-9999/rules.wasm", files)
	if old.Identity().BuildID == current.Identity().BuildID {
		t.Fatal("candidate reused original native identity")
	}
	catalog, err := engine.NewRuntimeCatalog(current, old)
	if err != nil {
		t.Fatal(err)
	}
	if catalog.CurrentIdentity() != current.Identity() {
		t.Fatal("catalog lost current identity")
	}
	inn := originalInn(t, old)
	menu := menuUntil(t, old, menuStep(t, old, inn, 1), 1)
	checks := []map[string]any{}
	for _, source := range []engine.Snapshot{inn, menu} {
		data, err := old.Encode(source)
		if err != nil {
			t.Fatal(err)
		}
		before := bytes.Clone(data)
		transaction, err := catalog.Continue(data)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, before) {
			t.Fatal("catalog mutated supplied old save")
		}
		want, err := old.Step(ctx, source, 0)
		if err != nil {
			t.Fatal(err)
		}
		if err = transaction.Step(ctx, 0); err != nil {
			t.Fatal(err)
		}
		encoded, err := transaction.Encode()
		if err != nil {
			t.Fatal(err)
		}
		actual, err := old.Decode(encoded)
		if err != nil || !reflect.DeepEqual(actual, want) {
			t.Fatal("retained core changed native continuation", err)
		}
		if actual.BuildID != source.BuildID || actual.ResourceID != source.ResourceID {
			t.Fatal("catalog relabeled a saved native identity")
		}
		if _, err = current.Decode(encoded); err == nil {
			t.Fatal("old save was converted to current native ABI")
		}
		broken := bytes.Clone(data)
		broken[len(broken)-1] ^= 0x80
		if _, err = catalog.Continue(broken); err == nil {
			t.Fatal("catalog bypassed checksum validation")
		}
		checks = append(checks, map[string]any{"mode": source.View.Mode, "buildPreserved": true, "exactNativeContinuation": true})
	}
	newGame, err := catalog.StartCampaignContinuationForPlayers(ctx, 12345, 1)
	if err != nil {
		t.Fatal(err)
	}
	newData, err := newGame.Encode()
	if err != nil {
		t.Fatal(err)
	}
	newSnapshot, err := current.Decode(newData)
	if err != nil || newSnapshot.BuildID != current.Identity().BuildID {
		t.Fatal("new game selected retired core", err)
	}
	missing, err := engine.NewRuntimeCatalog(old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = missing.Continue(newData); err == nil {
		t.Fatal("missing core silently fell back")
	}
	word, err := fs.ReadFile(files, "word.dat")
	if err != nil {
		t.Fatal(err)
	}
	word = bytes.Clone(word)
	word[0] = 'X'
	changed := catalogRuntime(t, ".local/pal/runtime-candidates/original/rules.wasm", catalogOverlay{files, word})
	if changed.Identity().ResourceID == current.Identity().ResourceID {
		t.Fatal("resource fixture did not change identity")
	}
	resourceGame, err := changed.StartCampaign(ctx, 12345)
	if err != nil {
		t.Fatal(err)
	}
	resourceData, err := changed.Encode(resourceGame)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = catalog.Continue(resourceData); err == nil {
		t.Fatal("missing resource identity silently fell back")
	}
	resourcesCatalog, err := engine.NewRuntimeCatalog(current, old, changed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = resourcesCatalog.Continue(resourceData); err != nil {
		t.Fatal("same core with retained resource version not selectable", err)
	}
	if _, err = engine.NewRuntimeCatalog(current, oldDev); err == nil {
		t.Fatal("original catalog enrolled development HP")
	}
	devCatalog, err := engine.NewRuntimeCatalog(newDev, oldDev)
	if err != nil {
		t.Fatal(err)
	}
	var saved struct {
		Native []byte `json:"native"`
	}
	data, err := os.ReadFile("../../../.local/pal/verification/development-health-stage-9/campaign/campaign-earth-orb-escaped-state-1.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	oldCampaign, err := oldDev.Continue(saved.Native)
	if err != nil {
		t.Fatal(err)
	}
	retainedCampaign, err := devCatalog.Continue(saved.Native)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(oldCampaign.View(), retainedCampaign.View()) {
		t.Fatal("actual chapter-9 save changed during version routing")
	}
	if _, err = catalog.Continue(saved.Native); err == nil {
		t.Fatal("original catalog resumed development campaign")
	}
	if _, err = devCatalog.Continue(newData); err == nil {
		t.Fatal("development catalog resumed original game")
	}
	for _, versions := range [][]*engine.Runtime{{nil}, {current, current}, {current, nil}} {
		if _, err = engine.NewRuntimeCatalog(versions[0], versions[1:]...); err == nil {
			t.Fatal("invalid runtime registry accepted")
		}
	}
	unclassified := catalogRuntime(t, ".local/pal/native-reference/rules.wasm", files)
	if _, err = engine.NewRuntimeCatalog(current, unclassified); err == nil {
		t.Fatal("unclassified old font reference core enrolled as campaign")
	}
	var group sync.WaitGroup
	for i := 0; i < 4; i++ {
		group.Go(func() {
			transaction, e := catalog.Continue(newData)
			if e != nil {
				t.Error(e)
				return
			}
			before := transaction.View()
			copy := transaction.View()
			copy.Party[0] = 5
			if !reflect.DeepEqual(before, transaction.View()) {
				t.Error("catalog shares mutable projection")
			}
			if e = transaction.Step(ctx, 0); e != nil {
				t.Error(e)
			}
		})
	}
	group.Wait()
	if t.Failed() {
		return
	}
	hash := sha256.Sum256(saved.Native)
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "genuine different native ABI builds, old original page/menu and actual chapter-9 development save, exact old-core continuation without ID relabeling, new games on current core, retained resource versions, corruption/missing-version rejection, profile separation and independent concurrent transactions", "originalOld": old.Identity(), "originalCurrent": current.Identity(), "developmentOld": oldDev.Identity(), "developmentCurrent": newDev.Identity(), "chapter9NativeSHA256": hex.EncodeToString(hash[:]), "checks": checks}, "", "  ")
	if err = os.WriteFile("../../../.local/pal/verification/runtime-catalog-e2e.json", report, 0600); err != nil {
		t.Fatal(err)
	}
}
