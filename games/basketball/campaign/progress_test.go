package campaign

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func fixture(t *testing.T, name string, v any) {
	t.Helper()
	f, e := os.Open("testdata/" + name)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	if name[len(name)-3:] == ".gz" {
		z, e := gzip.NewReader(f)
		if e != nil {
			t.Fatal(e)
		}
		defer z.Close()
		e = json.NewDecoder(z).Decode(v)
		if e != nil {
			t.Fatal(e)
		}
	} else if e = json.NewDecoder(f).Decode(v); e != nil {
		t.Fatal(e)
	}
}
func TestOriginalPasswordDomain(t *testing.T) {
	var rows []struct {
		Code     string
		Accepted bool
		After    Progress
	}
	fixture(t, "password-decode.json.gz", &rows)
	if len(rows) != 51472 {
		t.Fatal(len(rows))
	}
	for _, r := range rows {
		p, e := DecodePassword(r.Code)
		if e != nil || p != r.After {
			t.Fatalf("decode %s: got%+v source%+v err%v", r.Code, p, r.After, e)
		}
	}
	var encoded []struct {
		Progress Progress
		Code     string
	}
	fixture(t, "password-encode.json.gz", &encoded)
	if len(encoded) != len(rows) {
		t.Fatal(len(encoded))
	}
	for _, r := range encoded {
		code, e := EncodePassword(r.Progress)
		if e != nil || code != r.Code {
			t.Fatal(r.Code, code, e)
		}
	}
	fixture(t, "password-invalid.json.gz", &rows)
	for _, r := range rows {
		if r.Accepted {
			t.Fatal("invalid fixture accepted", r.Code)
		}
		if _, e := DecodePassword(r.Code); e == nil {
			t.Fatal("accepted source-invalid code", r.Code)
		}
	}
	for _, code := range []string{"000044", "", "12345", "1234567", "GG0000", " 000000", "000000 "} {
		if _, e := DecodePassword(code); e == nil {
			t.Fatal("unsupported/invalid code", code)
		}
	}
}
func TestOriginalEquipmentCalls(t *testing.T) {
	var rows []struct {
		Kind             string
		Character, Index uint8
		Before, After    Progress
	}
	fixture(t, "equipment-calls.json.gz", &rows)
	if len(rows) != 884 {
		t.Fatal(len(rows))
	}
	discarded := 0
	for n, r := range rows {
		var got Progress
		var e error
		switch r.Kind {
		case "equip":
			got, e = Equip(r.Before, r.Character, r.Index)
			if r.Before.Inventory[r.Index] == Empty {
				if e == nil || got != r.Before {
					t.Fatal("empty ownership", n, e)
				}
				continue
			}
		case "unequip":
			var step UnequipStep
			step, e = Unequip(r.Before, r.Character, Slot(r.Index))
			got = step.Progress
			if step.DiscardedItem != Empty {
				discarded++
				if step.DiscardedItem != r.Before.Equipment[r.Character][r.Index] {
					t.Fatal("wrong discarded source item")
				}
			}
		case "cancel":
			got = r.Before // No submitted operation: B is an adapter concern.
		}
		if e != nil || got != r.After {
			t.Fatalf("source call%d %s got%+v want%+v err%v", n, r.Kind, got, r.After, e)
		}
	}
	if discarded != 12 {
		t.Fatal("full-inventory source behavior missing", discarded)
	}
}
func TestOriginalOpponentAndRewardCalls(t *testing.T) {
	var maps []struct {
		Progress          Progress
		Opponents, Courts []uint8
	}
	fixture(t, "map-calls.json.gz", &maps)
	if len(maps) == 0 {
		t.Fatal("no original map calls")
	}
	for _, r := range maps {
		got, e := AvailableOpponents(r.Progress)
		if e != nil || !reflect.DeepEqual(got, r.Opponents) {
			t.Fatal(r.Progress, got, r.Opponents, e)
		}
		for i, opponent := range r.Opponents {
			court, e := CourtForOpponent(r.Progress, opponent)
			if e != nil || court != r.Courts[i] {
				t.Fatal(court, r.Courts[i], e)
			}
		}
	}
	var rewards []struct {
		RewardSet, Opponent, PostMatchPeriod uint8
		Item                                 Item
	}
	fixture(t, "reward-calls.json.gz", &rewards)
	if len(rewards) != 112 {
		t.Fatal(len(rewards))
	}
	for _, r := range rewards {
		item, e := RewardItem(r.RewardSet, r.Opponent, r.PostMatchPeriod)
		if e != nil || item != r.Item {
			t.Fatal(r, item, e)
		}
	}
}
func TestOriginalNormalMenuDataFlow(t *testing.T) {
	var traces []struct {
		Run    string
		Frames []struct {
			Frame    int
			Progress Progress
			Court    uint8
		}
	}
	fixture(t, "normal-menu.json", &traces)
	m := map[string][]Progress{}
	for _, trace := range traces {
		for _, f := range trace.Frames {
			m[trace.Run] = append(m[trace.Run], f.Progress)
		}
	}
	p, e := DecodePassword("020010")
	if e != nil || p != m["campaign-audit-password-equip-commit"][0] {
		t.Fatal(p, e)
	}
	p, e = Equip(p, 0, 0)
	if e != nil || p != m["campaign-audit-password-equip-commit"][1] {
		t.Fatal(p, e)
	}
	data, e := json.Marshal(p)
	if e != nil {
		t.Fatal(e)
	}
	var restored Progress
	if e = json.Unmarshal(data, &restored); e != nil || restored != p {
		t.Fatal("restore", e)
	}
	court, e := CourtForOpponent(restored, 1)
	if e != nil || court != 0 || restored.Period != 2 || restored != m["campaign-audit-equipped-home-match"][0] {
		t.Fatal(court, restored, e)
	}
	step, e := Unequip(restored, 0, Shirt)
	if e != nil || step.Progress != m["campaign-audit-equipment-remove"][1] || step.DiscardedItem != Empty {
		t.Fatal(step, e)
	}
	p, e = DecodePassword("020120")
	if e != nil || p != m["campaign-audit-equipment-swap-final"][0] {
		t.Fatal(p, e)
	}
	p, e = Equip(p, 0, 0)
	if e != nil || p != m["campaign-audit-equipment-swap-final"][1] {
		t.Fatal(p, e)
	}
	p, e = Equip(p, 0, 0)
	if e != nil || p != m["campaign-audit-equipment-swap-final"][2] {
		t.Fatal(p, e)
	}
	// Cancel issues no command and must not rebuild already confirmed gear from
	// the password. The password intentionally cannot preserve equipment.
	data, _ = json.Marshal(p)
	if e = json.Unmarshal(data, &restored); e != nil || restored != p {
		t.Fatal(e)
	}
	code, e := EncodePassword(p)
	if e != nil || code != "020120" {
		t.Fatal(code, e)
	}
	passwordRestored, e := DecodePassword(code)
	if e != nil || passwordRestored == p || passwordRestored != m["campaign-audit-equipment-swap-final"][0] {
		t.Fatal("password is not a JSON snapshot", e)
	}
	loss := m["campaign-audit-loss-continue"]
	if loss[0] != loss[1] {
		t.Fatal("original continue reset progression")
	}
}
func TestPreparationRejectsInvalidAPIInputsAtomically(t *testing.T) {
	p, e := New(0, 0)
	if e != nil {
		t.Fatal(e)
	}
	for _, bad := range [][2]uint8{{2, 0}, {0, 4}, {128, 0}} {
		if _, e := New(bad[0], bad[1]); e == nil {
			t.Fatal(bad)
		}
	}
	for _, args := range [][2]uint8{{0, 0}, {4, 0}, {0, 16}} {
		got, e := Equip(p, args[0], args[1])
		if e == nil || got != p {
			t.Fatal(got, e)
		}
	}
	for _, args := range [][2]uint8{{4, 0}, {0, 3}} {
		got, e := Unequip(p, args[0], Slot(args[1]))
		if e == nil || got.Progress != p || got.DiscardedItem != Empty {
			t.Fatal(got, e)
		}
	}
	for _, opponent := range []uint8{0, 8, 255} {
		if _, e := CourtForOpponent(p, opponent); e == nil {
			t.Fatal(opponent)
		}
	}
	won, e := DecodePassword("020120")
	if e != nil {
		t.Fatal(e)
	}
	if _, e := CourtForOpponent(won, 1); e == nil {
		t.Fatal("defeated opponent accepted")
	}
	bad := p
	bad.Inventory[1] = 1
	if got, e := Equip(bad, 0, 1); e == nil || got != bad {
		t.Fatal("inventory hole", e)
	}
	bad = p
	bad.Equipment[0][0] = 21
	if got, e := Equip(bad, 0, 0); e == nil || got != bad {
		t.Fatal("wrong category", e)
	}
	bad = p
	bad.Period = 2
	if _, e := EncodePassword(bad); e == nil {
		t.Fatal("inconsistent period")
	}
	for _, args := range [][3]uint8{{4, 1, 2}, {0, 0, 2}, {0, 8, 2}, {0, 1, 1}, {0, 1, 30}} {
		if _, e := RewardItem(args[0], args[1], args[2]); e == nil {
			t.Fatal(args)
		}
	}
}
