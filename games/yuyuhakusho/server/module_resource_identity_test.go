package yuyuhakusho

import (
	"encoding/json"
	"testing"
)

// Resource binding extension failure inventory, before its field/validator:
// absent/empty/wrong persisted descriptor is accepted using a current library;
// caller data changes on rejection; checkpoint exports a different descriptor.
func TestModuleSavedResourceIdentity(t *testing.T) {
	mod, ctx, m := moduleStartTest(t, 2, Options{Characters: []uint8{3, 9}}, []bool{true, true})
	var saved map[string]json.RawMessage
	if err := json.Unmarshal(m.Data, &saved); err != nil {
		t.Fatal(err)
	}
	var hash string
	if err := json.Unmarshal(saved["resourceDescriptorSHA256"], &hash); err != nil || hash != resources.descriptorSHA {
		t.Fatal("persisted match does not bind exact descriptor", err, hash)
	}
	before := string(m.Data)
	for _, mutation := range []string{"missing", "empty", "wrong"} {
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(m.Data, &fields)
		switch mutation {
		case "missing":
			delete(fields, "resourceDescriptorSHA256")
		case "empty":
			fields["resourceDescriptorSHA256"] = json.RawMessage(`""`)
		case "wrong":
			fields["resourceDescriptorSHA256"] = json.RawMessage(`"different"`)
		}
		raw, _ := json.Marshal(fields)
		bad := m
		bad.Data = raw
		if _, err := mod.Restore(ctx, bad); err == nil {
			t.Fatal("accepted cross-resource saved state", mutation)
		}
		if _, err := mod.View(bad, 0); err == nil {
			t.Fatal("projected cross-resource saved state", mutation)
		}
		if _, err := mod.SyncSnapshot(bad, 0); err == nil {
			t.Fatal("exported cross-resource saved state", mutation)
		}
		if string(m.Data) != before {
			t.Fatal("resource rejection changed caller")
		}
	}
	checkpoint := moduleCheckpointTest(t, mod, m, 0)
	if checkpoint.ResourceDescriptorSHA256 != hash {
		t.Fatal("checkpoint identity differs from persisted identity")
	}
}
