package syncwire

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

func replayLibraryTest(t *testing.T) engine.ChuWorldLibrary {
	t.Helper()
	raw, err := os.ReadFile("../data/chu-world-rules.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	data, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	library, err := engine.DecodeChuWorldRules(data)
	if err != nil {
		t.Fatal(err)
	}
	return library
}
func replayNewTest(t *testing.T, characters []uint8, arena uint16, team, human bool, library engine.ChuWorldLibrary) (engine.ChuWorldState, uint8) {
	t.Helper()
	options := engine.ChuSessionOptions{Seed: 0x12345678, Difficulty: 6, Arena: arena, TeamBattle: team}
	slots := []uint8{0, 3}
	if len(characters) == 3 {
		slots = []uint8{0, 1, 3}
	} else if len(characters) == 4 {
		slots = []uint8{0, 1, 2, 3}
	}
	var mask uint8
	for seat, character := range characters {
		computer := !(human && seat == 0)
		if computer {
			mask |= 1 << seat
		}
		options.Players = append(options.Players, engine.ChuSessionPlayer{Slot: slots[seat], CharacterID: character, Color: uint8(seat), Computer: computer, HumanChannel: uint8(seat)})
	}
	state, _, err := engine.NewChuSession(options, library)
	if err != nil {
		t.Fatal(err)
	}
	resource, err := engine.ChuSessionResourceForArena(arena, library)
	if err != nil {
		t.Fatal(err)
	}
	state, err = engine.CompleteChuSessionResource(state, resource, library)
	if err != nil {
		t.Fatal(err)
	}
	return state, mask
}
func TestFullIndependentMatchesThroughEncodedWindows(t *testing.T) {
	library := replayLibraryTest(t)
	for _, tc := range []struct {
		name        string
		characters  []uint8
		arena       uint16
		team, human bool
	}{{"duel", []uint8{0, 1}, 2, false, false}, {"three", []uint8{2, 3, 4}, 4, false, false}, {"four", []uint8{5, 6, 7, 8}, 10, false, false}, {"team", []uint8{9, 10, 0, 1}, 14, true, false}, {"human-idle", []uint8{7, 9}, 8, false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			direct, mask := replayNewTest(t, tc.characters, tc.arena, tc.team, tc.human, library)
			decoded, _ := replayNewTest(t, tc.characters, tc.arena, tc.team, tc.human, library)
			stateTrace, eventTrace, wireTrace := sha256.New(), sha256.New(), sha256.New()
			var history []InputFrame
			frames, callbacks, windows := uint64(0), 0, 0
			damage, ko := false, false
			for frames < 20000 && !direct.MatchComplete {
				rows := make([]InputFrame, 120)
				for index := range rows {
					frame := frames + uint64(index) + 1
					rows[index] = InputFrame{Frame: frame, Automated: mask}
					// EveryMD bit other than Start is exercised through sampled controller
					// history. Active human stays explicitly idle; no automatic takeover.
					for seat := range rows[index].Held {
						if tc.human && seat == 0 {
							continue
						}
						phase := (frame/17 + uint64(seat)*11) % 5
						switch phase {
						case 0:
							rows[index].Held[seat] = engine.ButtonZ | engine.ButtonY | engine.ButtonX | engine.ButtonMode
						case 1:
							rows[index].Held[seat] = engine.ButtonRight | engine.ButtonA
						case 2:
							rows[index].Held[seat] = engine.ButtonLeft | engine.ButtonB
						case 3:
							rows[index].Held[seat] = engine.ButtonDown | engine.ButtonC
						}
					}
				}
				window, err := Encode(rows)
				if err != nil {
					t.Fatal(err)
				}
				wire, _ := json.Marshal(window)
				roundtrip, err := DecodeJSON(wire)
				if err != nil || !reflect.DeepEqual(roundtrip, rows) {
					t.Fatal("window changed complete physical input", err)
				}
				wireTrace.Write(wire)
				windows++
				for index, input := range rows {
					var originalHeld, wireHeld [8]engine.Buttons
					copy(originalHeld[:4], input.Held[:])
					copy(wireHeld[:4], roundtrip[index].Held[:])
					var originalEvents, wireEvents []engine.ChuWorldEvent
					var originalOutcome, wireOutcome engine.WorldFrameOutcome
					direct, originalEvents, originalOutcome, err = engine.StepChuSessionFrame(direct, originalHeld, library)
					if err != nil {
						t.Fatal("direct independent frame", input.Frame, err)
					}
					decoded, wireEvents, wireOutcome, err = engine.StepChuSessionFrame(decoded, wireHeld, library)
					if err != nil {
						t.Fatal("decoded independent frame", input.Frame, err)
					}
					originalJSON, _ := json.Marshal(direct)
					wireJSON, _ := json.Marshal(decoded)
					originalEventJSON, _ := json.Marshal(originalEvents)
					wireEventJSON, _ := json.Marshal(wireEvents)
					if !bytes.Equal(originalJSON, wireJSON) || !bytes.Equal(originalEventJSON, wireEventJSON) || !reflect.DeepEqual(originalOutcome, wireOutcome) {
						t.Fatal("complete state/events/outcome changed through wire", input.Frame)
					}
					// WholeWorld.Controllers is deliberately included above; no digest mask.
					stateTrace.Write(originalJSON)
					eventTrace.Write(originalEventJSON)
					history, err = Push(history, input)
					if err != nil {
						t.Fatal(err)
					}
					frames = input.Frame
					for _, event := range originalEvents {
						if event.Kind == "callback" {
							callbacks++
						}
					}
					for _, object := range direct.Objects {
						if object.Header.Behavior == 0x12668 {
							damage = damage || object.Body.Health < 223
							ko = ko || object.Body.Health < 0
						}
					}
					if tc.human {
						object := direct.Objects[0x8400]
						if object.Header.Behavior == 0x12668 && object.InputChannel != 0 {
							t.Fatal("idle human changed into computer")
						}
					}
					if direct.MatchComplete {
						break
					}
				}
			}
			if !direct.MatchComplete || !damage || !ko || frames < 100 || callbacks == 0 || len(history) != 120 {
				t.Fatal("new independent lifecycle incomplete", frames, direct.MatchComplete, damage, ko, callbacks, len(history))
			}
			historyWindow, err := Encode(history)
			if err != nil {
				t.Fatal(err)
			}
			historyFrames, err := Decode(historyWindow)
			if err != nil || !reflect.DeepEqual(historyFrames, history) {
				t.Fatal("final recovery window differs", err)
			}
			report := struct {
				Case                 string      `json:"case"`
				Arena                uint16      `json:"arena"`
				Characters           []uint8     `json:"characters"`
				Team                 bool        `json:"team"`
				HumanIdle            bool        `json:"human_idle"`
				Frames               uint64      `json:"frames"`
				Callbacks            int         `json:"callbacks"`
				Windows              int         `json:"windows"`
				Complete             bool        `json:"complete"`
				Winner               int16       `json:"winner"`
				StateTrace           string      `json:"state_trace_sha256"`
				EventTrace           string      `json:"event_trace_sha256"`
				WireTrace            string      `json:"wire_trace_sha256"`
				FinalWindow          InputWindow `json:"final_window"`
				ReferenceStateLoaded bool        `json:"reference_state_loaded"`
			}{tc.name, tc.arena, tc.characters, tc.team, tc.human, frames, callbacks, windows, true, direct.Match.WinnerCode, hex.EncodeToString(stateTrace.Sum(nil)), hex.EncodeToString(eventTrace.Sum(nil)), hex.EncodeToString(wireTrace.Sum(nil)), historyWindow, false}
			raw, _ := json.Marshal(report)
			t.Log("SYNCWIRE_MATCH_FACTS " + string(raw))
		})
	}
}
func TestInputWindowActualPayloadSizes(t *testing.T) {
	reports := make(map[string]map[string]int)
	for _, name := range []string{"idle", "ordinary", "dense"} {
		rows := make([]InputFrame, 120)
		for i := range rows {
			rows[i] = InputFrame{Frame: 100000 + uint64(i), Automated: 15}
			if name == "ordinary" {
				rows[i].Held[0] = engine.Buttons(((i / 10) % 4) * 64)
				rows[i].Held[3] = engine.Buttons(((i / 20) % 4) * 256)
			} else if name == "dense" {
				for seat := range rows[i].Held {
					rows[i].Held[seat] = engine.Buttons((i*37 + seat*991) & 4095)
				}
				rows[i].Automated = uint8(i & 15)
			}
		}
		w, err := Encode(rows)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := base64SizeTest(w.Data)
		jsonData, _ := json.Marshal(w)
		reports[name] = map[string]int{"frames": 120, "rle_bytes": len(data), "base64_bytes": len(w.Data), "window_json_bytes": len(jsonData)}
		if len(jsonData) > 1800 {
			t.Fatal("window carries more than bounded input metadata", name, len(jsonData))
		}
	}
	raw, _ := json.Marshal(reports)
	t.Log("SYNCWIRE_SIZE_FACTS " + string(raw))
}

func base64SizeTest(value string) ([]byte, error) { return base64.StdEncoding.DecodeString(value) }
