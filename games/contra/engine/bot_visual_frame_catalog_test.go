package contra

// Before catalog implementation: no eager whole-cartridge load; corrupt RGB,
// wrong page length/content/hash and unknown IDs fail; manifest/caller/returned
// buffers cannot mutate verified assets; retry and concurrent reads stay safe.
import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"testing"
)

func visualAsset(b []byte) BotVisualAsset {
	h := sha256.Sum256(b)
	return BotVisualAsset{Bytes: len(b), SHA256: hex.EncodeToString(h[:])}
}
func TestVerifiedBotVisualCatalogLazyIdentityAndOwnership(t *testing.T) {
	rgb := make([]byte, 192)
	page := make([]byte, 4096)
	page[8] = 3
	calls := 0
	m := BotVisualCatalogManifest{RGB: visualAsset(rgb), Pages: map[uint8]BotVisualAsset{7: visualAsset(page)}}
	id, e := BotVisualCatalogIdentity(m)
	if e != nil || len(id) != 64 {
		t.Fatal(id, e)
	}
	c, e := NewVerifiedBotVisualCatalog(m, rgb, func(id uint8) ([]uint8, error) {
		calls++
		if id != 7 {
			t.Fatal("unexpected page")
		}
		return page, nil
	})
	if e != nil || calls != 0 {
		t.Fatal("eager load", e, calls)
	}
	m.Pages[7] = BotVisualAsset{}
	rgb[0] = 5
	got, e := c.CHRPage(7)
	if e != nil || calls != 1 || got[8] != 3 {
		t.Fatal(e, calls)
	}
	got[8] = 1
	page[8] = 2
	again, e := c.CHRPage(7)
	if e != nil || again[8] != 3 || calls != 1 {
		t.Fatal("mutable page alias", e)
	}
	colors, e := c.RGB()
	if e != nil || colors[0] != 0 {
		t.Fatal("mutable RGB input")
	}
	colors[0] = 99
	againRGB, _ := c.RGB()
	if !bytes.Equal(againRGB, make([]byte, 192)) {
		t.Fatal("mutable RGB output")
	}
	if _, e := c.CHRPage(8); e == nil || calls != 1 {
		t.Fatal("undeclared page loaded")
	}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b, e := c.CHRPage(7)
			if e != nil || b[8] != 3 {
				t.Error("concurrent catalog changed")
			}
		}()
	}
	wg.Wait()
}
func TestVerifiedBotVisualCatalogRejectsBadAssetsAndAllowsRetry(t *testing.T) {
	rgb := make([]byte, 192)
	page := make([]byte, 4096)
	m := BotVisualCatalogManifest{RGB: visualAsset(rgb), Pages: map[uint8]BotVisualAsset{0: visualAsset(page)}}
	bad := append([]byte(nil), rgb...)
	bad[0] = 1
	if _, e := NewVerifiedBotVisualCatalog(m, bad, func(uint8) ([]uint8, error) { return page, nil }); e == nil {
		t.Fatal("wrong RGB digest")
	}
	loads := 0
	c, e := NewVerifiedBotVisualCatalog(m, rgb, func(uint8) ([]uint8, error) {
		loads++
		p := append([]byte(nil), page...)
		if loads == 1 {
			p[0] = 1
		}
		return p, nil
	})
	if e != nil {
		t.Fatal(e)
	}
	if _, e := c.CHRPage(0); e == nil {
		t.Fatal("wrong page digest")
	}
	if _, e := c.CHRPage(0); e != nil || loads != 2 {
		t.Fatal("valid retry failed", e)
	}
	invalid := make([]byte, 4096)
	invalid[0] = 4
	m.Pages[0] = visualAsset(invalid)
	c, e = NewVerifiedBotVisualCatalog(m, rgb, func(uint8) ([]uint8, error) { return invalid, nil })
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.CHRPage(0); e == nil {
		t.Fatal("invalid indexed value")
	}
	m.Pages[0] = BotVisualAsset{Bytes: 1024, SHA256: visualAsset(page).SHA256}
	if _, e = BotVisualCatalogIdentity(m); e == nil {
		t.Fatal("raw bitplane bytes accepted as indexed page")
	}
}
