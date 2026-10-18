package basketball

// Failure inventory before implementation: calculate stats in JS; bind equipment
// to a live seat instead of one of four own characters; clamp after each item;
// preview an unowned ID or wrong inventory index; mutate inventory, RNG or time
// while projecting; reuse committed rather than pending halftime gear; trust
// client-supplied display stats; or mistake a numeric no-change for no effect.
import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
)

type menuGradeRow struct {
	Mode, Character uint8
	Slots           [3]int8
	Grades          engine.CharacterGrades
}

func menuGradeRows(t *testing.T) []menuGradeRow {
	t.Helper()
	file, e := os.Open("../engine/testdata/campaign-menu-grades.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer file.Close()
	z, e := gzip.NewReader(file)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	data, e := io.ReadAll(z)
	if e != nil {
		t.Fatal(e)
	}
	meta, e := os.ReadFile("../engine/testdata/campaign-menu-grades-source.json")
	if e != nil {
		t.Fatal(e)
	}
	var source struct{ FixtureJSONSHA256 string }
	if e = json.Unmarshal(meta, &source); e != nil {
		t.Fatal(e)
	}
	h := sha256.Sum256(data)
	if hex.EncodeToString(h[:]) != source.FixtureJSONSHA256 {
		t.Fatal("source fixture digest differs")
	}
	var rows []menuGradeRow
	if e = json.Unmarshal(data, &rows); e != nil {
		t.Fatal(e)
	}
	if len(rows) != 12672 {
		t.Fatal("incomplete source menu matrix", len(rows))
	}
	return rows
}

func TestCampaignAttributesOriginalMenu(t *testing.T) {
	rows := menuGradeRows(t)
	var baseline [4]engine.CharacterGrades
	for _, r := range rows {
		if r.Mode == 0 && r.Slots == ([3]int8{-1, -1, -1}) {
			baseline[r.Character] = r.Grades
		}
	}
	for index, r := range rows {
		p, e := campaign.New(r.Mode, 0)
		if e != nil {
			t.Fatal(e)
		}
		for slot, item := range r.Slots {
			p.Equipment[r.Character][slot] = campaign.Item(item)
		}
		before := p
		view, e := campaignAttributes(p)
		if e != nil {
			t.Fatal(e)
		}
		if p != before || view.Current[r.Character] != r.Grades || len(view.Inventory) != 0 {
			t.Fatal("original equipped stats differ", index, r, view, e)
		}
		for other := range baseline {
			if other != int(r.Character) && view.Current[other] != baseline[other] {
				t.Fatal("equipment leaked across characters", index)
			}
		}
	}
	t.Logf("original menu calls=%d; all four characters and 1584 legal slot combinations in each mode", len(rows))
}

func TestCampaignAttributesOwnedPreviewAndProjection(t *testing.T) {
	lookup := map[[4]int8]engine.CharacterGrades{}
	for _, r := range menuGradeRows(t) {
		if r.Mode == 0 {
			lookup[[4]int8{int8(r.Character), r.Slots[0], r.Slots[1], r.Slots[2]}] = r.Grades
		}
	}
	p, e := campaign.DecodePassword("020120")
	if e != nil {
		t.Fatal(e)
	}
	p, e = campaign.Equip(p, 0, 1)
	if e != nil {
		t.Fatal(e)
	}
	before := p
	view, e := campaignAttributes(p)
	if e != nil {
		t.Fatal(e)
	}
	if p != before || len(view.Inventory) != 1 || view.Inventory[0].Index != 0 || view.Inventory[0].Item != 0 {
		t.Fatal("preview lost source ownership/index")
	}
	for character := uint8(0); character < 4; character++ {
		next, e := campaign.Equip(p, character, 0)
		if e != nil {
			t.Fatal(e)
		}
		slots := next.Equipment[character]
		want := lookup[[4]int8{int8(character), int8(slots[0]), int8(slots[1]), int8(slots[2])}]
		if view.Inventory[0].After[character] != want {
			t.Fatal("prospective stats differ from original after confirm", character)
		}
	}
	for _, kind := range []string{"category", "unknown", "hole", "progress"} {
		bad := p
		switch kind {
		case "category":
			bad.Equipment[0][0] = 21
		case "unknown":
			bad.Inventory[0] = 32
		case "hole":
			bad.Inventory[0] = -1
			bad.Inventory[1] = 0
		case "progress":
			bad.Period = 0
		}
		if _, e = campaignAttributes(bad); e == nil {
			t.Fatal("invalid owned preparation accepted", kind)
		}
	}
	raw, e := configure(1, json.RawMessage(`{"mode":"solo","campaign":{"password":"020010"}}`))
	if e != nil {
		t.Fatal(e)
	}
	var o Options
	json.Unmarshal(raw, &o)
	if o.Campaign.Attributes == nil {
		t.Fatal("waiting stats missing")
	}
	want := *o.Campaign.Attributes
	o.Campaign.Attributes.Current[0].Run = 0
	raw, _ = json.Marshal(o)
	raw, e = configure(1, raw)
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal(raw, &o)
	if !reflect.DeepEqual(*o.Campaign.Attributes, want) {
		t.Fatal("client supplied display stats became authority")
	}
	ctx, m := campaignHostHalftime(t)
	initial := string(m.Data)
	visible, e := project(m, 0)
	if e != nil {
		t.Fatal(e)
	}
	if visible.(View).Attributes == nil || string(m.Data) != initial {
		t.Fatal("projection mutated source or omitted attributes")
	}
	a := engine.CampaignPreparationAction{Type: "unequip", Character: 0, Period: 3, Revision: 0}
	m, e = apply(ctx, m, campaignEditJSON(t, m, a))
	if e != nil {
		t.Fatal(e)
	}
	visible, e = project(m, -1)
	if e != nil {
		t.Fatal(e)
	}
	if visible.(View).Attributes == nil || len(visible.(View).Attributes.Inventory) != 1 || visible.(View).Attributes.Inventory[0].Item != 1 {
		t.Fatal("halftime preview used stale committed inventory")
	}
	// Retain actual normalized options for the independent browser view test.
	options, e := configure(1, json.RawMessage(`{"mode":"solo","campaign":{"password":"020120"}}`))
	if e != nil {
		t.Fatal(e)
	}
	dir := filepath.Join("..", "..", "..", ".local", "campaign-attributes")
	if e = os.MkdirAll(dir, 0755); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, "preview-options.json"), options, 0600); e != nil {
		t.Fatal(e)
	}
	// This original generated password owns items0/3. Replacing the steel shirt
	// with item0 has both gains and a reduction; no unearned item is injected.
	mixed, e := campaign.DecodePassword("400110")
	if e != nil {
		t.Fatal(e)
	}
	mixed, e = campaign.Equip(mixed, 0, 1)
	if e != nil {
		t.Fatal(e)
	}
	entry := defaultOptions()
	entry.Mode = "solo"
	entry.Campaign = &CampaignOptions{Password: "400110", Preparation: &mixed}
	encoded, _ := json.Marshal(entry)
	options, e = configure(1, encoded)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, "mixed-preview-options.json"), options, 0600); e != nil {
		t.Fatal(e)
	}
	options, e = configure(1, json.RawMessage(`{"mode":"solo","campaign":{"password":"FE7ED0"}}`))
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, "large-preview-options.json"), options, 0600); e != nil {
		t.Fatal(e)
	}
}
