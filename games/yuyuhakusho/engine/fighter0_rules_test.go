package engine

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

// These contract tests precede the source0 semantic-bundle extension.
func fighter0RuleJSON(t *testing.T) map[string]any {
	t.Helper()
	packed, err := os.ReadFile("../data/chu-world-rules.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(packed))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	return data
}

func TestFighter0WorldRuleLibrary(t *testing.T) {
	library := sessionLibrary(t)
	raw, err := os.ReadFile("../data/fighter0-projectile-rules.json")
	if err != nil {
		t.Fatal(err)
	}
	var source struct {
		Animations []struct {
			Key  AnimationFrameKey
			Data AnimationRecordData
		}
		Sequences []struct {
			Identity uint32
			Frames   []ChuAttachmentFrame
		}
	}
	if err = json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	if len(source.Animations) != 25 || len(source.Sequences) != 8 {
		t.Fatal("source0 original semantic data incomplete")
	}
	for _, row := range source.Animations {
		if got, ok := library.Animations[row.Key]; !ok || !reflect.DeepEqual(got, row.Data) {
			t.Fatal("source0 frame omitted or changed", row.Key)
		}
		if got, ok := library.Fighter0Projectiles.Animations[row.Key]; !ok || !reflect.DeepEqual(got, row.Data) {
			t.Fatal("child does not use the verified world frame", row.Key)
		}
	}
	for i, sequence := range source.Sequences {
		if sequence.Identity != 0x135ee+uint32(i)*6 || !reflect.DeepEqual(library.Fighter0Projectiles.Sequences[sequence.Identity], sequence.Frames) {
			t.Fatal("source0 release sequence omitted or changed", sequence.Identity)
		}
	}
}

func TestFighter0RuleVersionAndInvalidSequences(t *testing.T) {
	data := fighter0RuleJSON(t)
	if data["schema"] != float64(2) && data["schema"] != float64(3) {
		t.Fatal("new rule bundle did not declare source0 extension")
	}
	for _, kind := range []string{"missing", "duplicate", "wrong-identity"} {
		copy := fighter0RuleJSON(t)
		switch kind {
		case "missing":
			delete(copy, "fighter0_projectiles")
		case "duplicate":
			child := copy["fighter0_projectiles"].(map[string]any)
			rows := child["sequences"].([]any)
			rows[1] = rows[0]
		case "wrong-identity":
			child := copy["fighter0_projectiles"].(map[string]any)
			rows := child["sequences"].([]any)
			rows[0].(map[string]any)["identity"] = float64(0x135ed)
		}
		raw, _ := json.Marshal(copy)
		if _, err := DecodeChuWorldRules(raw); err == nil {
			t.Fatal("malformed source0 bundle accepted", kind)
		}
	}
	data["schema"] = float64(1)
	delete(data, "fighter0_projectiles")
	raw, _ := json.Marshal(data)
	if _, err := DecodeChuWorldRules(raw); err != nil {
		t.Fatal("legacy schema1 no longer readable", err)
	}
}
