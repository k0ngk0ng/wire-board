package contra

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
)

type BotVisualAsset struct {
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}
type BotVisualCatalogManifest struct {
	RGB   BotVisualAsset           `json:"rgb"`
	Pages map[uint8]BotVisualAsset `json:"pages"`
}

func validBotVisualAsset(a BotVisualAsset, size int) bool {
	b, e := hex.DecodeString(a.SHA256)
	return e == nil && len(b) == 32 && a.SHA256 == strings.ToLower(a.SHA256) && a.Bytes == size
}

// BotVisualCatalogIdentity binds asset contents rather than deployment paths.
// The host binds this digest to its published rule/resource identity.
func BotVisualCatalogIdentity(m BotVisualCatalogManifest) (string, error) {
	if !validBotVisualAsset(m.RGB, 192) || len(m.Pages) == 0 || len(m.Pages) > 128 {
		return "", errors.New("invalid visual catalog manifest")
	}
	for id, a := range m.Pages {
		if id >= 128 || !validBotVisualAsset(a, 4096) {
			return "", errors.New("invalid visual catalog page declaration")
		}
	}
	raw, e := json.Marshal(m)
	if e != nil {
		return "", e
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

type verifiedBotVisualCatalog struct {
	mu     sync.Mutex
	rgb    [192]uint8
	pages  map[uint8]BotVisualAsset
	cached map[uint8][4096]uint8
	load   func(uint8) ([]uint8, error)
}

// NewVerifiedBotVisualCatalog performs no file/network access itself. The host
// supplies source-verified RGB and a fixed resource reader. CHR loads are lazy;
// a server with IO-free action callbacks may preload declared pages outside
// those callbacks, then inject an immutable memory reader instead.
func NewVerifiedBotVisualCatalog(m BotVisualCatalogManifest, rgb []uint8, load func(uint8) ([]uint8, error)) (BotVisualCatalog, error) {
	if _, e := BotVisualCatalogIdentity(m); e != nil {
		return nil, e
	}
	if load == nil {
		return nil, errors.New("missing visual page loader")
	}
	copyRGB := append([]uint8(nil), rgb...)
	sum := sha256.Sum256(copyRGB)
	if len(copyRGB) != 192 || hex.EncodeToString(sum[:]) != m.RGB.SHA256 {
		return nil, errors.New("visual RGB identity mismatch")
	}
	c := &verifiedBotVisualCatalog{pages: make(map[uint8]BotVisualAsset, len(m.Pages)), cached: map[uint8][4096]uint8{}, load: load}
	copy(c.rgb[:], copyRGB)
	for id, a := range m.Pages {
		c.pages[id] = a
	}
	return c, nil
}
func (c *verifiedBotVisualCatalog) RGB() ([]uint8, error) {
	if c == nil {
		return nil, errors.New("missing visual catalog")
	}
	return append([]uint8(nil), c.rgb[:]...), nil
}
func (c *verifiedBotVisualCatalog) CHRPage(id uint8) ([]uint8, error) {
	if c == nil {
		return nil, errors.New("missing visual catalog")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	a, ok := c.pages[id]
	if !ok {
		return nil, fmt.Errorf("visual page %d is not declared", id)
	}
	if page, ok := c.cached[id]; ok {
		return append([]uint8(nil), page[:]...), nil
	}
	source, e := c.load(id)
	if e != nil {
		return nil, e
	}
	data := append([]uint8(nil), source...)
	sum := sha256.Sum256(data)
	if len(data) != a.Bytes || hex.EncodeToString(sum[:]) != a.SHA256 {
		return nil, fmt.Errorf("visual page %d identity mismatch", id)
	}
	var page [4096]uint8
	for i, v := range data {
		if v > 3 {
			return nil, fmt.Errorf("visual page %d has invalid indexed pixels", id)
		}
		page[i] = v
	}
	c.cached[id] = page
	return data, nil
}
