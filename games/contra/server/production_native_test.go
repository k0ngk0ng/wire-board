//go:build !js

package contra

// Failure inventory before production setup: missing or mismatched compiled
// identities; corrupt/trailing/oversized compressed data; wrong ROM/catalog/page
// identity; a runtime dependency on .local/testdata/ROMs; lazy IO in room/bot
// callbacks; mutable payloads after freezing; browser pixels differing from the
// embedded bot catalogue; registration or external service configuration added
// before gameplay acceptance. Native/Wasm artifact binding has its own E2E.
import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	runtime "github.com/k0ngk0ng/wire-board/games/contra/runtime"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestProductionRequiresBothCompiledIdentities(t *testing.T) {
	oldRules, oldRuntime := runtime.CompiledRulesBuildID, runtime.CompiledRuntimeBuildID
	t.Cleanup(func() { runtime.CompiledRulesBuildID, runtime.CompiledRuntimeBuildID = oldRules, oldRuntime })
	for _, pair := range [][2]string{{"", ""}, {strings.Repeat("a", 64), ""}, {"invalid", strings.Repeat("b", 64)}} {
		runtime.CompiledRulesBuildID, runtime.CompiledRuntimeBuildID = pair[0], pair[1]
		if _, err := NewProductionModule(); err == nil {
			t.Fatal("unbound production module accepted", pair)
		}
	}
}
func TestProductionEmbeddedVisualsAndOfflineCallbacks(t *testing.T) {
	oldRules, oldRuntime := runtime.CompiledRulesBuildID, runtime.CompiledRuntimeBuildID
	t.Cleanup(func() { runtime.CompiledRulesBuildID, runtime.CompiledRuntimeBuildID = oldRules, oldRuntime })
	// Explicit constructor boundary IDs. The separate native artifact E2E uses
	// actual source/manifests and linker injection, not these synthetic strings.
	runtime.CompiledRulesBuildID = strings.Repeat("a", 64)
	runtime.CompiledRuntimeBuildID = strings.Repeat("b", 64)
	m, err := NewProductionModule()
	if err != nil {
		t.Fatal(err)
	}
	if m.BotAction != nil || m.Reclaim != nil || m.BotSearch != nil || m.MinPlayers != 1 || m.MaxPlayers != 2 {
		t.Fatal("human-only delivery must retain 1/2 players and omit computer/autoplay capabilities")
	}
	resources, err := decodeProductionVisuals(productionVisualArchive, productionVisualArchiveSHA256)
	if err != nil {
		t.Fatal(err)
	}
	if resources.ID() == "" {
		t.Fatal("missing immutable pixels")
	}
	rgb, _ := resources.Catalog().RGB()
	rgb[0] ^= 255
	again, _ := resources.Catalog().RGB()
	if bytes.Equal(rgb, again) {
		t.Fatal("mutable catalog")
	}
	ctx := gameapi.Context{Actor: 0, Players: 1, Automated: []bool{false}, Now: time.UnixMilli(100000)}
	match, err := m.Start(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	ctx.Actor = -1
	for i := 0; i < 4; i++ {
		ctx.Now = ctx.Now.Add(50 * time.Millisecond)
		match, err = m.OnTimer(ctx, match)
		if err != nil {
			t.Fatal("prebuilt actual observer", err)
		}
	}
	raw, err := m.View(match, 0)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(raw)
	var view View
	if err = json.Unmarshal(encoded, &view); err != nil {
		t.Fatal(err)
	}
	if view.RuntimeBuildID != runtime.CompiledRuntimeBuildID || view.VisualsID != resources.ID() {
		t.Fatal("production asset identity was lost", view.RuntimeBuildID, view.VisualsID)
	}
}
func TestProductionArchiveRejectsMalformedData(t *testing.T) {
	for _, mode := range []string{"sha", "corrupt", "trailing", "rom", "catalog", "page", "missing", "oversized"} {
		t.Run(mode, func(t *testing.T) {
			packed := append([]byte(nil), productionVisualArchive...)
			expected := productionVisualArchiveSHA256
			if mode == "sha" {
				expected = strings.Repeat("0", 64)
			} else if mode == "corrupt" {
				packed[len(packed)/2] ^= 1
				expected = hashProductionBytes(packed)
			} else if mode == "trailing" {
				packed = append(packed, []byte("tail")...)
				expected = hashProductionBytes(packed)
			} else {
				zr, e := gzip.NewReader(bytes.NewReader(packed))
				if e != nil {
					t.Fatal(e)
				}
				var value map[string]any
				if e = json.NewDecoder(zr).Decode(&value); e != nil {
					t.Fatal(e)
				}
				zr.Close()
				switch mode {
				case "rom":
					value["romSHA256"] = strings.Repeat("a", 64)
				case "catalog":
					value["catalogID"] = strings.Repeat("b", 64)
				case "page":
					value["pages"].(map[string]any)["0"] = "AA=="
				case "missing":
					delete(value["pages"].(map[string]any), "0")
				case "oversized":
					value["unexpected"] = strings.Repeat("x", 2<<20)
				}
				raw, _ := json.Marshal(value)
				var output bytes.Buffer
				zw := gzip.NewWriter(&output)
				zw.Write(raw)
				zw.Close()
				packed = output.Bytes()
				expected = hashProductionBytes(packed)
			}
			if _, e := decodeProductionVisuals(packed, expected); e == nil {
				t.Fatal("invalid embedded visual bundle accepted", mode)
			}
		})
	}
}
func TestProductionAndBrowserVisualCatalogueAgree(t *testing.T) {
	raw, e := os.ReadFile(filepath.Join("..", "engine", "testdata", "bot-visual-catalog.json.gz"))
	if e != nil {
		t.Fatal(e)
	}
	zr, e := gzip.NewReader(bytes.NewReader(raw))
	if e != nil {
		t.Fatal(e)
	}
	defer zr.Close()
	var source struct {
		Catalog engine.BotVisualCatalogManifest
	}
	if e = json.NewDecoder(zr).Decode(&source); e != nil {
		t.Fatal(e)
	}
	expected, e := engine.BotVisualCatalogIdentity(source.Catalog)
	if e != nil {
		t.Fatal(e)
	}
	actual, e := decodeProductionVisuals(productionVisualArchive, productionVisualArchiveSHA256)
	if e != nil {
		t.Fatal(e)
	}
	if expected != actual.ID() {
		t.Fatal("bot and player pixels differ")
	}
}
