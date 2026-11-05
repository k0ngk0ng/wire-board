// Package pcmassets owns immutable, content-identified DPCM resources. It does
// not interpret source descriptors, execute CPUs, schedule playback or emit PCM.
package pcmassets

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

type Protocol string

const (
	OrdinaryPCM     Protocol = "ordinary"
	TrackPanPCM     Protocol = "track-pan"
	maxCatalogBytes          = 2 << 20
	maxScoreBytes            = 16 << 20
)

// Descriptor is a value reference. Ordinary RoutingOrPan is the original
// producer's seventh-byte routing meaning. Music uses the separate track pan
// input; its RoutingOrPan remains zero and must not replace that input.
type Descriptor struct {
	ID           string   `json:"id"`
	SampleID     string   `json:"sample_id"`
	NativeRate   uint8    `json:"native_rate"`
	Flags        uint8    `json:"flags"`
	Protocol     Protocol `json:"protocol"`
	RoutingOrPan uint8    `json:"routing_or_pan"`
}

// Payload is a read-only view of its Catalog's sole encoded-byte owner. Bytes
// returns an explicit copy; out-of-range ByteAt is unknown, never ROM padding.
type Payload struct {
	id   string
	data []byte
}

func (p Payload) ID() string { return p.id }
func (p Payload) Len() int   { return len(p.data) }
func (p Payload) ByteAt(offset int) (byte, bool) {
	if offset < 0 || offset >= len(p.data) {
		return 0, false
	}
	return p.data[offset], true
}
func (p Payload) Bytes() []byte { return append([]byte(nil), p.data...) }

// Catalog owns payloads once. All maps and slices are private and immutable
// after Decode. Code/descriptor lookups return detached values or read-only views.
type Catalog struct {
	ordinary    map[uint8]string
	diagnostic  map[uint8]string
	descriptors map[string]Descriptor
	music       map[string]bool
	payloads    map[string][]byte
}

func (c *Catalog) Ordinary(code uint8) (Descriptor, bool) {
	if c == nil {
		return Descriptor{}, false
	}
	id, ok := c.ordinary[code]
	if !ok {
		return Descriptor{}, false
	}
	return c.Descriptor(id)
}
func (c *Catalog) Diagnostic(code uint8) (Descriptor, bool) {
	if c == nil {
		return Descriptor{}, false
	}
	id, ok := c.diagnostic[code]
	if !ok {
		return Descriptor{}, false
	}
	return c.Descriptor(id)
}
func (c *Catalog) Music(id string) (Descriptor, bool) {
	if c == nil || !c.music[id] {
		return Descriptor{}, false
	}
	return c.Descriptor(id)
}
func (c *Catalog) Descriptor(id string) (Descriptor, bool) {
	if c == nil {
		return Descriptor{}, false
	}
	d, ok := c.descriptors[id]
	return d, ok
}
func (c *Catalog) Payload(id string) (Payload, bool) {
	if c == nil {
		return Payload{}, false
	}
	data, ok := c.payloads[id]
	if !ok {
		return Payload{}, false
	}
	return Payload{id, data}, true
}
func (c *Catalog) PayloadLength(id string) (int, bool) { p, ok := c.Payload(id); return p.Len(), ok }
func (c *Catalog) PayloadByte(id string, offset int) (byte, bool) {
	p, ok := c.Payload(id)
	if !ok {
		return 0, false
	}
	return p.ByteAt(offset)
}
func (c *Catalog) Counts() (ordinary, music, diagnostic, payloads int) {
	if c == nil {
		return
	}
	return len(c.ordinary), len(c.music), len(c.diagnostic), len(c.payloads)
}

type command struct {
	Code       uint8      `json:"code"`
	Route      string     `json:"route"`
	Descriptor Descriptor `json:"descriptor"`
}
type encodedPayload struct {
	ID    string `json:"id"`
	Codec string `json:"codec"`
	Data  string `json:"data_base64"`
}
type musicDescriptor struct {
	ID         string `json:"id"`
	SampleID   string `json:"sample_id"`
	NativeRate uint8  `json:"native_rate"`
	Flags      uint8  `json:"flags"`
	Routing    string `json:"routing"`
}
type musicImport struct {
	Format      string            `json:"format"`
	LogicalPath string            `json:"logical_path"`
	RawSHA      string            `json:"raw_json_sha256"`
	RawBytes    int               `json:"raw_json_bytes"`
	Descriptors []musicDescriptor `json:"descriptors"`
}
type wireCatalog struct {
	Schema      int              `json:"schema"`
	Format      string           `json:"format"`
	Ordinary    []command        `json:"ordinary"`
	Diagnostics []command        `json:"diagnostics"`
	Payloads    []encodedPayload `json:"payloads"`
	MusicImport musicImport      `json:"music_import"`
}

// Decode validates the ordinary resource and borrows the six payloads from the
// exact score JSON resource, consuming both byte inputs without retaining them.
// The caller separately authenticates these resources against its trusted build
// manifest and passes the same score JSON to score.DecodeCatalog for score rules.
func Decode(r io.Reader, scoreJSON []byte) (*Catalog, error) {
	if r == nil {
		return nil, errors.New("missing PCM resource reader")
	}
	raw, err := io.ReadAll(io.LimitReader(r, maxCatalogBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxCatalogBytes || len(scoreJSON) > maxScoreBytes {
		return nil, errors.New("PCM resource limit exceeded")
	}
	var wire wireCatalog
	if err = decodeStrict(raw, &wire); err != nil {
		return nil, fmt.Errorf("PCM catalog: %w", err)
	}
	if wire.Schema != 1 || wire.Format != "typed-pcm-payloads-v1" || len(wire.Ordinary) != 122 || len(wire.Diagnostics) != 1 || len(wire.Payloads) != 123 {
		return nil, errors.New("unsupported PCM catalog profile")
	}
	imported := wire.MusicImport
	if imported.Format != "typed-audio-score-v2" || imported.LogicalPath != "/yuyuhakusho/data/audio-score.json.gz" || !validSHA(imported.RawSHA) || imported.RawBytes != len(scoreJSON) || len(imported.Descriptors) != 6 {
		return nil, errors.New("invalid music dependency")
	}
	sum := sha256.Sum256(scoreJSON)
	if hex.EncodeToString(sum[:]) != imported.RawSHA {
		return nil, errors.New("exact music JSON identity differs")
	}
	c := &Catalog{ordinary: map[uint8]string{}, diagnostic: map[uint8]string{}, descriptors: map[string]Descriptor{}, music: map[string]bool{}, payloads: map[string][]byte{}}
	for _, p := range wire.Payloads {
		if err = c.addPayload(p); err != nil {
			return nil, err
		}
	}
	referenced := map[string]bool{}
	for _, entry := range wire.Ordinary {
		validCode := entry.Code >= 6 && entry.Code <= 126 || entry.Code == 240
		if !validCode || entry.Route != map[bool]string{true: "remapped-ordinary", false: "direct"}[entry.Code == 240] {
			return nil, errors.New("unsupported ordinary command route")
		}
		if _, ok := c.ordinary[entry.Code]; ok {
			return nil, errors.New("duplicate ordinary command")
		}
		if err = c.addOrdinary(entry.Descriptor, false); err != nil {
			return nil, err
		}
		c.ordinary[entry.Code] = entry.Descriptor.ID
		referenced[entry.Descriptor.SampleID] = true
	}
	for code := uint8(6); code <= 126; code++ {
		if c.ordinary[code] == "" {
			return nil, errors.New("missing ordinary command")
		}
	}
	if c.ordinary[240] == "" {
		return nil, errors.New("missing remapped ordinary command")
	}
	entry := wire.Diagnostics[0]
	if entry.Code != 127 || entry.Route != "internal-silent" {
		return nil, errors.New("unsupported internal diagnostic")
	}
	if err = c.addOrdinary(entry.Descriptor, true); err != nil {
		return nil, err
	}
	c.diagnostic[entry.Code] = entry.Descriptor.ID
	referenced[entry.Descriptor.SampleID] = true
	if len(referenced) != 123 {
		return nil, errors.New("unreferenced or duplicate ordinary payload")
	}
	var music struct {
		Schema      int
		Format      string
		Descriptors []musicDescriptor
		Payloads    []encodedPayload
	}
	if err = readMusicProjection(scoreJSON, &music); err != nil {
		return nil, err
	}
	if music.Schema != 2 || music.Format != "typed-audio-score-v2" || len(music.Descriptors) != 6 || len(music.Payloads) != 6 {
		return nil, errors.New("unsupported music resource profile")
	}
	for _, p := range music.Payloads {
		if err = c.addPayload(p); err != nil {
			return nil, err
		}
	}
	want := map[string]musicDescriptor{}
	for _, d := range imported.Descriptors {
		if _, duplicate := want[d.ID]; duplicate || d.Routing != "track-pan" || d.ID != musicID(d) {
			return nil, errors.New("invalid imported music descriptor")
		}
		want[d.ID] = d
	}
	for _, d := range music.Descriptors {
		if expected, ok := want[d.ID]; !ok || expected != d || c.music[d.ID] {
			return nil, errors.New("music descriptor identity differs")
		}
		if _, ok := c.payloads[d.SampleID]; !ok || referenced[d.SampleID] {
			return nil, errors.New("invalid music payload dependency")
		}
		value := Descriptor{ID: d.ID, SampleID: d.SampleID, NativeRate: d.NativeRate, Flags: d.Flags, Protocol: TrackPanPCM}
		if _, duplicate := c.descriptors[d.ID]; duplicate {
			return nil, errors.New("duplicate descriptor ID")
		}
		c.descriptors[d.ID] = value
		c.music[d.ID] = true
		referenced[d.SampleID] = true
	}
	if len(c.payloads) != 129 || len(referenced) != 129 || len(c.descriptors) != 129 {
		return nil, errors.New("incomplete PCM dependency registry")
	}
	return c, nil
}
func (c *Catalog) addPayload(p encodedPayload) error {
	if !validSHA(p.ID) || p.Codec != "dpcm4" {
		return errors.New("unsupported PCM payload identity or codec")
	}
	if _, duplicate := c.payloads[p.ID]; duplicate {
		return errors.New("duplicate PCM payload")
	}
	decoded, err := base64.StdEncoding.DecodeString(p.Data)
	if err != nil || len(decoded) == 0 || len(decoded) > 32768 || base64.StdEncoding.EncodeToString(decoded) != p.Data {
		return errors.New("invalid or noncanonical DPCM payload")
	}
	sum := sha256.Sum256(decoded)
	if hex.EncodeToString(sum[:]) != p.ID {
		return errors.New("DPCM content digest differs")
	}
	c.payloads[p.ID] = decoded
	return nil
}
func (c *Catalog) addOrdinary(d Descriptor, diagnostic bool) error {
	data, ok := c.payloads[d.SampleID]
	if !ok || d.Protocol != OrdinaryPCM || d.RoutingOrPan&^uint8(0xc0) != 0 || d.RoutingOrPan == 0 || d.ID != ordinaryID(d) {
		return errors.New("invalid ordinary descriptor")
	}
	if _, duplicate := c.descriptors[d.ID]; duplicate {
		return errors.New("duplicate descriptor ID")
	}
	if diagnostic {
		if len(data) != 1 || d.NativeRate != 20 || d.Flags != 0xe0 || d.RoutingOrPan != 0xc0 {
			return errors.New("internal silent profile differs")
		}
	} else {
		if len(data) < 69 || len(data) > 7788 {
			return errors.New("ordinary payload outside recovered profile")
		}
		switch d.Flags {
		case 1, 65, 128, 129, 193:
		default:
			return errors.New("ordinary flags outside recovered profile")
		}
	}
	c.descriptors[d.ID] = d
	return nil
}
func validSHA(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32
}
func contentID(v any) string {
	raw, _ := json.Marshal(v)
	sum := sha256.Sum256(raw)
	return "descriptor-" + hex.EncodeToString(sum[:])[:20]
}
func ordinaryID(d Descriptor) string {
	return contentID(map[string]any{"sample_id": d.SampleID, "native_rate": d.NativeRate, "flags": d.Flags, "protocol": d.Protocol, "routing_or_pan": d.RoutingOrPan})
}
func musicID(d musicDescriptor) string {
	return contentID(map[string]any{"sample_id": d.SampleID, "native_rate": d.NativeRate, "flags": d.Flags, "routing": d.Routing})
}

// The score decoder owns its non-PCM fields. This projection validates the exact
// full resource hash above, top-level shape and the two shared PCM fragments.
func readMusicProjection(raw []byte, dst *struct {
	Schema      int
	Format      string
	Descriptors []musicDescriptor
	Payloads    []encodedPayload
}) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return errors.New("invalid music JSON object")
	}
	seen := map[string]bool{}
	for d.More() {
		token, err = d.Token()
		if err != nil {
			return err
		}
		key, ok := token.(string)
		if !ok || seen[key] || len(seen) >= 64 {
			return errors.New("duplicate or oversized music object")
		}
		seen[key] = true
		var fragment json.RawMessage
		if err = d.Decode(&fragment); err != nil {
			return err
		}
		switch key {
		case "schema":
			err = json.Unmarshal(fragment, &dst.Schema)
		case "format":
			err = json.Unmarshal(fragment, &dst.Format)
		case "dac_descriptors":
			err = decodeStrict(fragment, &dst.Descriptors)
		case "dac_payloads":
			err = decodeStrict(fragment, &dst.Payloads)
		}
		if err != nil {
			return err
		}
	}
	if token, err = d.Token(); err != nil || token != json.Delim('}') {
		return errors.New("invalid music object end")
	}
	if _, err = d.Token(); err != io.EOF {
		return errors.New("trailing music JSON")
	}
	if !seen["schema"] || !seen["format"] || !seen["dac_descriptors"] || !seen["dac_payloads"] {
		return errors.New("missing shared music fields")
	}
	return nil
}
func decodeStrict(raw []byte, dst any) error {
	if err := checkKeys(raw); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	return d.Decode(dst)
}
func checkKeys(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > 64 {
			return errors.New("JSON nesting limit exceeded")
		}
		token, err := d.Token()
		if err != nil {
			return err
		}
		delimiter, structured := token.(json.Delim)
		if !structured {
			return nil
		}
		switch delimiter {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				token, err = d.Token()
				if err != nil {
					return err
				}
				key, ok := token.(string)
				if !ok || seen[key] {
					return errors.New("duplicate JSON field")
				}
				seen[key] = true
				if err = value(depth + 1); err != nil {
					return err
				}
			}
			if token, err = d.Token(); err != nil || token != json.Delim('}') {
				return errors.New("invalid JSON object")
			}
		case '[':
			for d.More() {
				if err = value(depth + 1); err != nil {
					return err
				}
			}
			if token, err = d.Token(); err != nil || token != json.Delim(']') {
				return errors.New("invalid JSON array")
			}
		default:
			return errors.New("unexpected JSON delimiter")
		}
		return nil
	}
	if err := value(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}
