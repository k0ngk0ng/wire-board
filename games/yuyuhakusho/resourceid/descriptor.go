// Package resourceid validates immutable resource identity metadata, without
// loading a filesystem, client pixels, sound, runtime code or game rules.
package resourceid

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
)

type Resource struct {
	ID            string `json:"id"`
	LogicalPath   string `json:"logical_path"`
	RawJSONSHA256 string `json:"raw_json_sha256"`
	RawJSONBytes  uint64 `json:"raw_json_bytes"`
	GzipSHA256    string `json:"gzip_sha256"`
	GzipBytes     uint64 `json:"gzip_bytes"`
}
type contents struct {
	digest    string
	resources map[string]Resource
}

// Descriptor owns immutable identity metadata. Lookup returns a scalar copy.
// The digest must itself be authenticated using the caller's trusted manifest.
type Descriptor struct{ data *contents }

func (d Descriptor) Digest() string {
	if d.data == nil {
		return ""
	}
	return d.data.digest
}
func (d Descriptor) Lookup(id string) (Resource, bool) {
	if d.data == nil {
		return Resource{}, false
	}
	r, ok := d.data.resources[id]
	return r, ok
}
func hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func lowerSHA(v string) bool {
	if len(v) != 64 {
		return false
	}
	b, e := hex.DecodeString(v)
	return e == nil && hex.EncodeToString(b) == v
}
func object(raw []byte, names ...string) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	token, e := d.Token()
	if e != nil || token != json.Delim('{') {
		return nil, errors.New("resource identity must be an object")
	}
	allowed := make(map[string]bool, len(names))
	for _, n := range names {
		allowed[n] = true
	}
	out := make(map[string]json.RawMessage, len(names))
	for d.More() {
		token, e = d.Token()
		if e != nil {
			return nil, e
		}
		name, ok := token.(string)
		if !ok || !allowed[name] || out[name] != nil {
			return nil, errors.New("unknown or duplicate resource identity field")
		}
		var value json.RawMessage
		if e = d.Decode(&value); e != nil {
			return nil, e
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, errors.New("null resource identity field")
		}
		out[name] = value
	}
	if token, e = d.Token(); e != nil || token != json.Delim('}') {
		return nil, errors.New("invalid resource identity object")
	}
	if _, e = d.Token(); e != io.EOF {
		return nil, errors.New("trailing resource identity JSON")
	}
	if len(out) != len(names) {
		return nil, errors.New("missing resource identity field")
	}
	return out, nil
}

var paths = map[string]string{
	"rules": "/yuyuhakusho/data/chu-world-rules.json.gz", "sprites": "/yuyuhakusho/data/world-sprites.json.gz", "hud": "/yuyuhakusho/data/hud-assets.json.gz", "render_meta": "/yuyuhakusho/data/renderstate-metadata.json.gz",
	"arena-default": "/yuyuhakusho/data/arena-assets-default.json.gz", "arena-2": "/yuyuhakusho/data/arena-assets-2.json.gz", "arena-4": "/yuyuhakusho/data/arena-assets-4.json.gz", "arena-6": "/yuyuhakusho/data/arena-assets-6.json.gz", "arena-8": "/yuyuhakusho/data/arena-assets-8.json.gz", "arena-a": "/yuyuhakusho/data/arena-assets-a.json.gz", "arena-c": "/yuyuhakusho/data/arena-assets-c.json.gz", "arena-e": "/yuyuhakusho/data/arena-assets-e.json.gz", "arena-a-aux": "/yuyuhakusho/data/arena-a-aux-assets.json.gz",
}

func Decode(raw []byte) (Descriptor, error) {
	if len(raw) == 0 || len(raw) > 65536 {
		return Descriptor{}, errors.New("invalid resource descriptor size")
	}
	top, e := object(raw, "schema", "resources")
	if e != nil {
		return Descriptor{}, e
	}
	var version int
	if e = json.Unmarshal(top["schema"], &version); e != nil || version != 1 {
		return Descriptor{}, errors.New("unsupported resource descriptor schema")
	}
	var rows []json.RawMessage
	if e = json.Unmarshal(top["resources"], &rows); e != nil || len(rows) != len(paths) {
		return Descriptor{}, errors.New("incomplete resource descriptor registry")
	}
	data := &contents{digest: hash(raw), resources: make(map[string]Resource, len(paths))}
	for _, row := range rows {
		if _, e = object(row, "id", "logical_path", "raw_json_sha256", "raw_json_bytes", "gzip_sha256", "gzip_bytes"); e != nil {
			return Descriptor{}, e
		}
		var r Resource
		if e = json.Unmarshal(row, &r); e != nil {
			return Descriptor{}, e
		}
		path, known := paths[r.ID]
		_, duplicate := data.resources[r.ID]
		if !known || duplicate || r.LogicalPath != path || !lowerSHA(r.RawJSONSHA256) || !lowerSHA(r.GzipSHA256) || r.RawJSONBytes == 0 || r.GzipBytes == 0 || r.RawJSONBytes > 64<<20 || r.GzipBytes > 64<<20 {
			return Descriptor{}, errors.New("invalid resource identity, size or path")
		}
		data.resources[r.ID] = r
	}
	return Descriptor{data: data}, nil
}
func (d Descriptor) VerifyJSON(id string, raw []byte) error {
	r, ok := d.Lookup(id)
	if !ok || uint64(len(raw)) != r.RawJSONBytes || hash(raw) != r.RawJSONSHA256 {
		return errors.New("exact JSON resource identity differs")
	}
	return nil
}
func (d Descriptor) VerifyGzip(id string, packed []byte) error {
	r, ok := d.Lookup(id)
	if !ok || uint64(len(packed)) != r.GzipBytes || hash(packed) != r.GzipSHA256 {
		return errors.New("stored gzip resource identity differs")
	}
	return nil
}
