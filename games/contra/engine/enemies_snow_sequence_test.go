package contra

// Integration gaps defined before continuous snow verification: correct
// isolated calls can still lose scrolling/joint linkage, same-tick palette
// state, real queue contention, carried entity state, or delayed Boss exit.

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

func TestSnowOriginalContinuations(t *testing.T) {
	f, err := os.Open("testdata/original-enemies-snow-sequences-v2.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var data struct {
		Original  bool `json:"source_is_original_execution"`
		Sequences []struct {
			Mode, Slot  int
			TargetCalls int `json:"target_calls"`
			Records     []snowOracleRecord
		}
	}
	if err = json.NewDecoder(z).Decode(&data); err != nil {
		t.Fatal(err)
	}
	if !data.Original || len(data.Sequences) != 4 {
		t.Fatal("invalid snow continuation source")
	}
	raw, err := os.ReadFile("leveldata/levels.json")
	if err != nil {
		t.Fatal(err)
	}
	resources, err := DecodeLevelResources(raw)
	if err != nil {
		t.Fatal(err)
	}
	tiles, err := resources.Graphics(4, RegularMap)
	if err != nil {
		t.Fatal(err)
	}
	counts := [4]int{9801, 4337, 95, 70}
	focused := [4]int{1217, 499, 95, 70}
	for _, sequence := range data.Sequences {
		if sequence.Mode < 0 || sequence.Mode > 3 || len(sequence.Records) != counts[sequence.Mode] || sequence.TargetCalls != focused[sequence.Mode] {
			t.Fatal("snow continuation coverage changed", sequence.Mode, len(sequence.Records))
		}
		var previous *Enemy
		targetCount, refused := 0, 0
		kind := uint8(18)
		if sequence.Mode >= 2 {
			kind = 20
		}
		for _, r := range sequence.Records {
			before := enemyFromOracle(r.Before.Enemies[r.Slot])
			if r.Slot != sequence.Slot || before.Kind != kind {
				continue
			}
			if previous != nil && before != *previous {
				t.Fatalf("sequence%d target changed between original AI calls", sequence.Mode)
			}
			after := enemyFromOracle(r.After.Enemies[r.Slot])
			previous = &after
			targetCount++
			for _, event := range r.Events {
				if event.Kind == "terrain" && event.Carry != 0 {
					refused++
				}
			}
		}
		if targetCount != focused[sequence.Mode] || previous == nil || previous.Routine != 0 {
			t.Fatal("snow continuation did not finish", sequence.Mode, targetCount, previous)
		}
		if sequence.Mode == 3 && refused == 0 {
			t.Fatal("carrier continuation did not exercise real graphics contention")
		}
		verifySnowComplexRecords(t, sequence.Records, resources, tiles)
		t.Logf("snow sequence%d: %d native entity calls, %d target calls, %d real queue refusals", sequence.Mode, len(sequence.Records), targetCount, refused)
	}
}
