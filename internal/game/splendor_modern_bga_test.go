package game

import (
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

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
