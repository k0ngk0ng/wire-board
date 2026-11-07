package contra

// Failure inventory before fixture loader: wrong engine build; a missing,
// corrupted, oversized or non-indexed asset; a URL escaping the mounted asset
// root; undeclared file reads; mutable filesystem contents retained by callbacks.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"
)

func fixtureVisualData(t *testing.T) (fstest.MapFS, map[string]any, string) {
	t.Helper()
	build := strings.Repeat("a", 64)
	rgb, page := make([]byte, 192), make([]byte, 4096)
	asset := func(url string, data []byte) map[string]any {
		sum := sha256.Sum256(data)
		return map[string]any{"url": url, "bytes": len(data), "sha256": hex.EncodeToString(sum[:])}
	}
	m := map[string]any{"rulesBuildID": build, "rgb": asset("/graphics/colors.rgb", rgb), "pages": map[string]any{"0": asset("/graphics/page-000.idx", page)}}
	files := fstest.MapFS{"graphics/colors.rgb": &fstest.MapFile{Data: rgb}, "graphics/page-000.idx": &fstest.MapFile{Data: page}}
	return files, m, build
}

func TestContraFixtureVisualAssetsFreezeVerifiedPayloads(t *testing.T) {
	files, manifest, build := fixtureVisualData(t)
	raw, _ := json.Marshal(manifest)
	v, err := LoadVisualResources(files, raw, build)
	if err != nil {
		t.Fatal(err)
	}
	files["graphics/page-000.idx"].Data[0] = 3
	p, err := v.Catalog().CHRPage(0)
	if err != nil || p[0] != 0 {
		t.Fatal("filesystem data leaked into frozen catalog", err)
	}
	if _, err = v.Catalog().CHRPage(1); err == nil {
		t.Fatal("undeclared page available")
	}
}

func TestContraFixtureVisualAssetsRejectInvalidBundle(t *testing.T) {
	for _, mode := range []string{"build", "visuals", "missing", "hash", "size", "pixels", "parent", "remote", "query", "double-slash", "empty"} {
		t.Run(mode, func(t *testing.T) {
			files, manifest, build := fixtureVisualData(t)
			page := manifest["pages"].(map[string]any)["0"].(map[string]any)
			switch mode {
			case "build":
				manifest["rulesBuildID"] = strings.Repeat("b", 64)
			case "visuals":
				manifest["visualsID"] = strings.Repeat("c", 64)
			case "missing":
				delete(files, "graphics/page-000.idx")
			case "hash":
				files["graphics/page-000.idx"].Data[0] = 1
			case "size":
				page["bytes"] = 8192
			case "pixels":
				files["graphics/page-000.idx"].Data[0] = 4
				sum := sha256.Sum256(files["graphics/page-000.idx"].Data)
				page["sha256"] = hex.EncodeToString(sum[:])
			case "parent":
				page["url"] = "/../graphics/page-000.idx"
			case "remote":
				page["url"] = "https://example.invalid/page.idx"
			case "query":
				page["url"] = "/graphics/page-000.idx?version=1"
			case "double-slash":
				page["url"] = "//graphics/page-000.idx"
			case "empty":
				manifest["pages"] = map[string]any{}
			}
			raw, _ := json.Marshal(manifest)
			if _, err := readFixtureVisualResources(files, raw, build); err == nil {
				t.Fatal("invalid visual bundle accepted")
			}
		})
	}
}
