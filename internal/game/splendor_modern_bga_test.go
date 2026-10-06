package game

import (
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// BGA is independent corroboration of the fourteen condition sets, not proof
// of physical reverse-side pairing. Its fifteenth Madrid record is deliberately
// excluded: the 2025 physical unboxing shows black, not blue, alongside white.
func TestSplendorModernCityBGAConditions(t *testing.T) {
	raw, err := os.ReadFile("../../docs/research/splendor-bga-components.json")
	if err != nil {
		t.Fatal(err)
	}
	type cityFact struct {
		ID     int    `json:"id"`
		Name   string `json:"name"`
		Points int    `json:"points"`
		Cost   string `json:"cost"`
		Promo  bool   `json:"promo"`
	}
	var proof struct {
		Material struct {
			Cities map[string]cityFact `json:"cities"`
		} `json:"material"`
	}
	if err = json.Unmarshal(raw, &proof); err != nil {
		t.Fatal(err)
	}
	catalog, err := readSplendorModernCatalog()
	if err != nil {
		t.Fatal(err)
	}
	ids := []int{11, 4, 1, 8, 2, 9, 7, 14, 5, 12, 6, 13, 10, 3}
	names := []struct{ en, zh string }{
		{"Isabella the Catholic in Madrid", "马德里"},
		{"François Ier in Amboise", "昂布瓦兹"},
		{"Al-Qadi Aqib ibn Mahmud ibn Umar in Timbuktu", "廷巴克图"},
		{"Bega Begum in Delhi", "德里"},
		{"Muhammad Shaybani Khan in Samarkand", "撒马尔罕"},
		{"Sejong the Great in Seoul", "首尔"},
		{"Elisabeth von Habsburg in Krakow", "克拉科夫"},
	}
	if len(catalog.Cities) != len(ids) || len(proof.Material.Cities) != 15 {
		t.Fatal("unexpected city source size")
	}
	for i, id := range ids {
		source, ok := proof.Material.Cities[strconv.Itoa(id)]
		// BGA misspells Seoul on one side; retain the original captured fact.
		name := strings.ReplaceAll(source.Name, "Seoule", "Seoul")
		if !ok || source.ID != id || source.Promo || name != names[i/2].en || strings.Trim(source.Cost, "ECSORX") != "" {
			t.Fatal("unexpected BGA city fact", id, source)
		}
		want := GemCity{Tile: i/2 + 1, Side: i % 2, Name: names[i/2].zh, Points: source.Points, Any: strings.Count(source.Cost, "X")}
		for color, char := range "ECSOR" {
			want.Cost[color] = strings.Count(source.Cost, string(char))
		}
		if got := catalog.Cities[i]; got != want {
			t.Fatal("BGA city conditions mismatch", id, got, want)
		}
	}
	if extra := proof.Material.Cities["15"]; extra.ID != 15 || extra.Name != names[0].en || extra.Points != 12 || extra.Cost != "CCCEEERRRSSS" {
		t.Fatal("excluded Madrid variant changed; review its source", extra)
	}
}

// This is a separately captured component-fact source, not a fixture generated
// from our catalog. No BGA game logic, player data or network access is used.
func TestSplendorModernOrientBGAComponents(t *testing.T) {
	raw, err := os.ReadFile("../../docs/research/splendor-bga-components.json")
	if err != nil {
		t.Fatal(err)
	}
	var proof struct {
		Material struct {
			Orient map[string]struct {
				Level    int    `json:"lvl"`
				Points   int    `json:"points"`
				Type     int    `json:"type"`
				Cost     string `json:"cost"`
				CostCard string `json:"costCard"`
				Bonus    int    `json:"nbBonus"`
				Copy     int    `json:"symbolCopy"`
				Take     int    `json:"symbolTake"`
			} `json:"orient"`
		} `json:"material"`
	}
	if err = json.Unmarshal(raw, &proof); err != nil {
		t.Fatal(err)
	}
	catalog, err := readSplendorModernCatalog()
	if err != nil {
		t.Fatal(err)
	}
	// Local 1001..1030 -> BGA printed component-fact IDs. The ordering differs;
	// match actual fields rather than assuming either provider's index order.
	ids := []int{209, 208, 207, 206, 210, 201, 203, 204, 202, 205, 211, 212, 213, 214, 215, 220, 217, 219, 216, 218, 226, 227, 228, 229, 230, 224, 225, 222, 223, 221}
	if len(proof.Material.Orient) != 30 {
		t.Fatal("incomplete independent Orient source")
	}
	colors := []int{1, 2, 0, 4, 3, -1, -1}
	letters := "ECSOR" // our order: green, white, blue, black, red
	for i, id := range ids {
		source, ok := proof.Material.Orient[strconv.Itoa(id)]
		if !ok || source.Type < 0 || source.Type >= len(colors) {
			t.Fatal("missing/invalid BGA type", id)
		}
		want := Card{ID: 1001 + i, Tier: source.Level - 10, Points: source.Points, Color: colors[source.Type], Cost: make([]int, 5)}
		for color, char := range letters {
			want.Cost[color] = strings.Count(source.Cost, string(char))
		}
		switch {
		case source.Type == 6 && source.Copy == 0 && source.Take == 0 && source.Bonus == 0:
			want.Orient = GemOrientGold
		case source.Type == 5 && source.Copy == 1 && source.Take == 0 && source.Bonus == 0:
			want.Orient = GemOrientCopy
		case source.Type == 5 && source.Copy == 1 && source.Take == 1 && source.Bonus == 0:
			want.Orient = GemOrientCopyCascade
		case source.Type < 5 && source.Bonus == 2 && source.Copy == 0 && source.Take == 0:
			want.Orient, want.BonusCount = GemOrientDouble, 2
		case source.Type < 5 && source.Bonus == 1 && source.Copy == 0 && source.Take == 2:
			want.Orient = GemOrientCascade
		case source.Type < 5 && source.Bonus == 1 && source.Copy == 0 && source.Take == 0 && len(source.CostCard) == 2:
			if source.CostCard[0] != source.CostCard[1] {
				t.Fatal("sacrifice is not two same-color cards", id)
			}
			want.Orient = GemOrientSacrifice
			want.SacrificeColor = strings.IndexByte(letters, source.CostCard[0])
			if want.SacrificeColor < 0 {
				t.Fatal("unknown sacrifice color", id)
			}
		default:
			t.Fatal("unknown source effect", id, source)
		}
		if want.Orient != GemOrientSacrifice && source.CostCard != "" {
			t.Fatal("unexpected sacrifice cost", id)
		}
		if strings.Trim(source.Cost, letters) != "" {
			t.Fatal("unknown cost letter", id)
		}
		if got := catalog.Orient[i]; !reflect.DeepEqual(got, want) {
			t.Fatal("independent BGA component mismatch", id, got, want)
		}
	}
}
