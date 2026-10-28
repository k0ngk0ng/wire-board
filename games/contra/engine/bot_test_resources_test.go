package contra

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

// Portable normalized artwork, not a ROM or saved running World. The fixture
// generator verifies the same RGB/CHR bytes served to human browsers.
func botBuiltinTestCatalog(t *testing.T) BotVisualCatalog {
	t.Helper()
	file, err := os.Open("testdata/bot-visual-catalog.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	z, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var fixture struct {
		Schema    int
		RomSHA256 string
		Catalog   BotVisualCatalogManifest
		RGB       []byte
		Pages     map[uint8][]byte
	}
	if err = json.NewDecoder(z).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Schema != 1 || fixture.RomSHA256 != "6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe" {
		t.Fatal("wrong visual fixture source")
	}
	catalog, err := NewVerifiedBotVisualCatalog(fixture.Catalog, fixture.RGB, func(id uint8) ([]uint8, error) { return fixture.Pages[id], nil })
	if err != nil {
		t.Fatal(err)
	}
	for id := range fixture.Catalog.Pages {
		if _, err = catalog.CHRPage(id); err != nil {
			t.Fatal(err)
		}
	}
	return catalog
}
