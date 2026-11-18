package pal_test

// Inventory integration: original hidden search events must survive the
// authority bundle byte-for-byte. This list deliberately records candidates,
// including inactive original events; a static grant is not gameplay proof.
// Branch/call-dependent rewards and optional NPC tasks require later review.
import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestOriginalHiddenSearchEventInventoryPreserved(t *testing.T) {
	files := os.DirFS("../../../.local/pal/original-98")
	tables, err := resources.Load98Tables(files)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := resources.BuildRuleBundle(files)
	if err != nil {
		t.Fatal(err)
	}
	original, err := fs.ReadFile(files, "sss.mkf")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, bundle.Files["sss.mkf"]) {
		t.Fatal("authority bundle changed/filtered original hidden events or scripts")
	}
	type grant struct {
		PC, Item, Amount, Cash int
		Name                   string
	}
	type candidate struct {
		Scene, Map, Event, X, Y, Sprite, InitialState, TriggerMode, Script int
		Classification                                                     string
		Grants                                                             []grant
		Messages                                                           []string
		Calls                                                              []int
	}
	rows := []candidate{}
	known := map[int]bool{}
	hidden, chests, other, active := 0, 0, 0, 0
	decode := func(raw []byte) string {
		text, err := simplifiedchinese.GBK.NewDecoder().String(string(raw))
		if err != nil {
			t.Fatal(err)
		}
		return text
	}
	for scene := 0; scene+1 < len(tables.Scenes); scene++ {
		a, b := int(tables.Scenes[scene].FirstEvent), int(tables.Scenes[scene+1].FirstEvent)
		for event := a; event < b; event++ {
			e := tables.Events[event]
			if e.TriggerScript == 0 || e.TriggerMode < 1 || e.TriggerMode > 3 {
				continue
			}
			row := candidate{Scene: scene + 1, Map: int(tables.Scenes[scene].Map), Event: event + 1, X: int(e.X), Y: int(e.Y), Sprite: int(e.Sprite), InitialState: int(e.State), TriggerMode: int(e.TriggerMode), Script: int(e.TriggerScript), Classification: "visible-search-object"}
			if e.Sprite == 0 {
				row.Classification = "map-baked-hidden-search"
			}
			if e.Sprite == 10 {
				row.Classification = "visible-chest-candidate"
			}
			for pc := int(e.TriggerScript); pc < len(tables.Scripts) && pc < int(e.TriggerScript)+128; pc++ {
				s := tables.Scripts[pc]
				if s.Opcode <= 3 {
					break
				}
				if s.Opcode == 4 {
					row.Calls = append(row.Calls, int(s.Operands[0]))
				}
				if s.Opcode == 0x1e && int16(s.Operands[0]) > 0 {
					row.Grants = append(row.Grants, grant{PC: pc, Cash: int(int16(s.Operands[0]))})
				}
				if s.Opcode == 0x1f && s.Operands[0] > 0 && int(s.Operands[0]) < len(tables.Words) && int16(s.Operands[1]) >= 0 {
					amount := int(s.Operands[1])
					if amount == 0 {
						amount = 1
					}
					row.Grants = append(row.Grants, grant{PC: pc, Item: int(s.Operands[0]), Amount: amount, Name: decode(tables.Words[s.Operands[0]])})
				}
				if s.Opcode == 0xffff && int(s.Operands[0]) < len(tables.Messages) {
					row.Messages = append(row.Messages, decode(tables.Messages[s.Operands[0]]))
				}
			}
			if len(row.Grants) == 0 {
				continue
			}
			rows = append(rows, row)
			known[row.Event] = true
			if e.State > 0 {
				active++
			}
			switch row.Classification {
			case "map-baked-hidden-search":
				hidden++
			case "visible-chest-candidate":
				chests++
			default:
				other++
			}
		}
	}
	for _, event := range []int{5, 6, 7, 32, 34, 42, 43, 44} {
		if !known[event] {
			t.Fatal("known original inn reward missing from candidate inventory", event)
		}
	}
	if len(rows) == 0 || len(tables.Events) != 5077 {
		t.Fatal("original hidden-event inventory incomplete", len(rows), len(tables.Events))
	}
	hash := sha256.Sum256(original)
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "complete unchanged original SSS event/script payload in authority bundle; bounded direct-search reward candidate inventory only, not all conditional loot or all gameplay acceptance", "sssSHA256": hex.EncodeToString(hash[:]), "allOriginalEventRecordsPreserved": len(tables.Events), "allOriginalScriptRecordsPreserved": len(tables.Scripts), "sceneRecords": len(tables.Scenes), "traversedSceneRanges": len(tables.Scenes) - 1, "directSearchRewardCandidates": len(rows), "mapBakedHiddenCandidates": hidden, "visibleChestCandidates": chests, "otherVisibleSearchCandidates": other, "initiallyActiveCandidates": active, "unreviewedCandidates": len(rows), "candidates": rows}, "", "  ")
	if err = os.WriteFile("../../../.local/pal/verification/hidden-search-candidates.json", report, 0600); err != nil {
		t.Fatal(err)
	}
}
