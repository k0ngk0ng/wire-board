package resourceid_test

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/resourceid"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func source(t *testing.T) []byte {
	t.Helper()
	b, e := os.ReadFile("../data/runtime-resource-descriptor.json")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func digest(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func mutable(t *testing.T) map[string]any {
	t.Helper()
	var v map[string]any
	if e := json.Unmarshal(source(t), &v); e != nil {
		t.Fatal(e)
	}
	return v
}
func encoded(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestDescriptorCompletePayloadIdentity(t *testing.T) {
	raw := source(t)
	d, e := resourceid.Decode(raw)
	if e != nil {
		t.Fatal(e)
	}
	if d.Digest() != digest(raw) {
		t.Fatal("descriptor identity normalized original bytes")
	}
	var input struct{ Resources []resourceid.Resource }
	if e = json.Unmarshal(raw, &input); e != nil {
		t.Fatal(e)
	}
	if len(input.Resources) != 13 {
		t.Fatal("missing producer resources")
	}
	for _, r := range input.Resources {
		got, ok := d.Lookup(r.ID)
		if !ok || got != r {
			t.Fatal("resource identity differs", r.ID)
		}
		packed, e := os.ReadFile(filepath.Join("../data", filepath.Base(r.LogicalPath)))
		if e != nil {
			t.Fatal(e)
		}
		if e = d.VerifyGzip(r.ID, packed); e != nil {
			t.Fatal(r.ID, e)
		}
		z, e := gzip.NewReader(bytes.NewReader(packed))
		if e != nil {
			t.Fatal(e)
		}
		data, e := io.ReadAll(z)
		z.Close()
		if e != nil {
			t.Fatal(e)
		}
		if e = d.VerifyJSON(r.ID, data); e != nil {
			t.Fatal(r.ID, e)
		}
		if r.RawJSONSHA256 != digest(data) || r.GzipSHA256 != digest(packed) || r.RawJSONBytes != uint64(len(data)) || r.GzipBytes != uint64(len(packed)) {
			t.Fatal("fields do not bind actual payload", r.ID)
		}
		for _, bad := range [][]byte{nil, data[:len(data)-1], append(append([]byte(nil), data...), 0)} {
			if d.VerifyJSON(r.ID, bad) == nil {
				t.Fatal("damaged JSON accepted", r.ID)
			}
		}
		if d.VerifyJSON(r.ID, packed) == nil || d.VerifyGzip(r.ID, data) == nil {
			t.Fatal("JSON/gzip identity domains mixed")
		}
		got.LogicalPath = "changed"
		again, _ := d.Lookup(r.ID)
		if again != r {
			t.Fatal("lookup aliases catalog")
		}
	}
	if _, ok := d.Lookup("unknown"); ok || d.VerifyJSON("unknown", nil) == nil || d.VerifyGzip("unknown", nil) == nil {
		t.Fatal("unknown id accepted")
	}
	var zero resourceid.Descriptor
	if zero.Digest() != "" || zero.VerifyJSON("rules", nil) == nil {
		t.Fatal("zero descriptor usable")
	}
}
func TestDescriptorInvalidStructureAtomic(t *testing.T) {
	for _, kind := range []string{"schema", "missing-schema", "null-root", "null-resource", "unknown-root", "count", "duplicate-id", "unknown-id", "path", "raw-sha", "gzip-sha", "uppercase-sha", "raw-size", "gzip-size", "fraction-size", "overflow-size", "unknown-resource", "missing-resource-field", "null-field"} {
		t.Run(kind, func(t *testing.T) {
			v := mutable(t)
			rows := v["resources"].([]any)
			r := rows[0].(map[string]any)
			switch kind {
			case "schema":
				v["schema"] = 2
			case "missing-schema":
				delete(v, "schema")
			case "null-root":
				v = nil
			case "null-resource":
				rows[0] = nil
			case "unknown-root":
				v["payload"] = "body"
			case "count":
				v["resources"] = rows[:12]
			case "duplicate-id":
				rows[1] = rows[0]
			case "unknown-id":
				r["id"] = "rom"
			case "path":
				r["logical_path"] = "https://bad/../secret"
			case "raw-sha":
				r["raw_json_sha256"] = "invalid"
			case "gzip-sha":
				r["gzip_sha256"] = "00"
			case "uppercase-sha":
				r["raw_json_sha256"] = strings.ToUpper(r["raw_json_sha256"].(string))
			case "raw-size":
				r["raw_json_bytes"] = 0
			case "gzip-size":
				r["gzip_bytes"] = -1
			case "fraction-size":
				r["gzip_bytes"] = 1.5
			case "overflow-size":
				r["raw_json_bytes"] = json.Number("18446744073709551616")
			case "unknown-resource":
				r["body"] = "x"
			case "missing-resource-field":
				delete(r, "gzip_bytes")
			case "null-field":
				r["gzip_bytes"] = nil
			}
			got, e := resourceid.Decode(encoded(t, v))
			if e == nil || !reflect.DeepEqual(got, resourceid.Descriptor{}) {
				t.Fatal("invalid descriptor returned partial identity", kind, e)
			}
		})
	}
	for _, b := range [][]byte{nil, []byte("[]"), append(source(t), []byte(" {}")...), bytes.Replace(source(t), []byte(`"schema":1`), []byte(`"schema":1,"schema":1`), 1), bytes.Replace(source(t), []byte(`"id":"rules"`), []byte(`"id":"rules","id":"rules"`), 1)} {
		if _, e := resourceid.Decode(b); e == nil {
			t.Fatal("duplicate/trailing/nonobject accepted")
		}
	}
}
func TestDescriptorExactBytesAndTrustBoundary(t *testing.T) {
	d, e := resourceid.Decode(source(t))
	if e != nil {
		t.Fatal(e)
	}
	packed, e := os.ReadFile("../data/chu-world-rules.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	z, _ := gzip.NewReader(bytes.NewReader(packed))
	raw, _ := io.ReadAll(z)
	z.Close()
	var object any
	if e = json.Unmarshal(raw, &object); e != nil {
		t.Fatal(e)
	}
	reserialized := encoded(t, object)
	if bytes.Equal(raw, reserialized) {
		t.Fatal("test failed to alter exact source")
	}
	if d.VerifyJSON("rules", reserialized) == nil {
		t.Fatal("JS style parse/stringify retained false identity")
	}
	altered := mutable(t)
	r := altered["resources"].([]any)[0].(map[string]any)
	r["raw_json_sha256"] = digest(reserialized)
	r["raw_json_bytes"] = len(reserialized)
	other, e := resourceid.Decode(encoded(t, altered))
	if e != nil || other.Digest() == d.Digest() || other.VerifyJSON("rules", reserialized) != nil {
		t.Fatal("structural validation confused with external authentication", e)
	}
	t.Log("Caller must authenticate Descriptor.Digest using immutable trusted metadata; self-declared hashes are not a signature")
}
