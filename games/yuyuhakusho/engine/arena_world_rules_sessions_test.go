package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func TestArenaWorldRulesRegistryAndLegacy(t *testing.T) {
	library := arenaWorldLibraryTest(t)
	if len(library.ArenaResources) != 8 || len(library.ArenaScenes.LinearRows) != 241 || len(library.ArenaScenes.Arena4Rows) != 64 || len(library.ArenaScenes.WaveX) != 1024 || &library.ArenaScenes.WaveX[0] != &library.Arena.WaveX[0] {
		t.Fatal("arena registry lost complete typed data or duplicated the shared wave owner")
	}
	for _, arena := range arenaWorldIDsTest {
		resource, err := ChuSessionResourceForArena(arena, library)
		if err != nil || resource != library.ArenaResources[arena] || resource.Source != library.MatchStart.ArenaResources[arena/2] {
			t.Fatal("arena resource differs from independently recovered match-start binding", arena, err)
		}
	}
	if library.ArenaResources[0] != DefaultChuSessionResource() {
		t.Fatal("default resource value changed")
	}
	raw, _ := json.Marshal(DefaultChuSessionResource())
	if string(raw) != `{"Source":136992,"Palette0":[2662,512,3822,3786,2150,1126,2728,3240,3240,2184,1092,2694,1058,1604,3786,1090]}` {
		t.Fatal("default resource wire JSON changed", string(raw))
	}
	for _, schema := range []int{1, 2, 3} {
		rules := arenaWorldRuleJSONTest(t)
		rules["schema"], _ = json.Marshal(schema)
		delete(rules, "arena_scenes")
		delete(rules, "arena_resources")
		raw, _ := json.Marshal(rules)
		legacy, err := DecodeChuWorldRules(raw)
		if err != nil || len(legacy.ArenaScenes.LinearRows) != 0 || len(legacy.ArenaResources) != 0 {
			t.Fatal("legacy schema invented unavailable arena data", schema, err)
		}
		if resource, err := ChuSessionResourceForArena(0, legacy); err != nil || resource != DefaultChuSessionResource() {
			t.Fatal("legacy default resource became unavailable", schema, err)
		}
		if _, err := ChuSessionResourceForArena(2, legacy); err == nil {
			t.Fatal("legacy rules enabled an unrecovered arena", schema)
		}
	}
}

func TestArenaWorldRulesInvalidExtensionIsAtomic(t *testing.T) {
	for _, kind := range []string{"missing-scenes", "missing-resources", "null-scenes", "null-resources", "missing-row", "short-row", "extra-row", "missing-four-row", "short-four-row", "missing-colors", "short-colors", "missing-resource", "duplicate-resource", "wrong-arena", "wrong-source", "invalid-palette", "changed-default", "short-palette", "wrong-behavior", "conflicting-wave"} {
		t.Run(kind, func(t *testing.T) {
			rules := arenaWorldRuleJSONTest(t)
			var scenes map[string]json.RawMessage
			_ = json.Unmarshal(rules["arena_scenes"], &scenes)
			var resources []map[string]json.RawMessage
			_ = json.Unmarshal(rules["arena_resources"], &resources)
			switch kind {
			case "missing-scenes":
				delete(rules, "arena_scenes")
			case "missing-resources":
				delete(rules, "arena_resources")
			case "null-scenes":
				rules["arena_scenes"] = json.RawMessage(`null`)
			case "null-resources":
				rules["arena_resources"] = json.RawMessage(`null`)
			case "missing-row", "short-row", "extra-row":
				var rows map[string][]int16
				_ = json.Unmarshal(scenes["linear_rows"], &rows)
				if kind == "missing-row" {
					delete(rows, "744")
				} else if kind == "short-row" {
					rows["744"] = rows["744"][:127]
				} else {
					rows["743"] = rows["744"]
				}
				scenes["linear_rows"], _ = json.Marshal(rows)
			case "missing-four-row", "short-four-row":
				var rows map[string][]int16
				_ = json.Unmarshal(scenes["arena4_rows"], &rows)
				if kind == "missing-four-row" {
					delete(rows, "0")
				} else {
					rows["0"] = rows["0"][:111]
				}
				scenes["arena4_rows"], _ = json.Marshal(rows)
			case "missing-colors":
				delete(scenes, "arena4_colors")
			case "short-colors":
				scenes["arena4_colors"] = json.RawMessage(`[[0,2,4],[0,2,4],[0,2,4]]`)
			case "missing-resource":
				resources = resources[:7]
			case "duplicate-resource":
				resources[7] = resources[1]
			case "wrong-arena":
				resources[7]["arena"] = json.RawMessage(`15`)
			case "wrong-source":
				resources[7]["source"] = json.RawMessage(`136992`)
			case "invalid-palette", "changed-default", "short-palette":
				i := 7
				if kind == "changed-default" {
					i = 0
				}
				var palette []uint16
				_ = json.Unmarshal(resources[i]["palette_0"], &palette)
				if kind == "short-palette" {
					palette = palette[:15]
				} else if kind == "invalid-palette" {
					palette[0] = 1
				} else {
					palette[0] ^= 2
				}
				resources[i]["palette_0"], _ = json.Marshal(palette)
			case "wrong-behavior":
				var start map[string]json.RawMessage
				_ = json.Unmarshal(rules["match_start"], &start)
				var behaviors []uint32
				_ = json.Unmarshal(start["arena_behaviors"], &behaviors)
				behaviors[7] = 0x20918
				start["arena_behaviors"], _ = json.Marshal(behaviors)
				rules["match_start"], _ = json.Marshal(start)
			case "conflicting-wave":
				scenes["wave_x"] = json.RawMessage(`[1]`)
			}
			if _, ok := rules["arena_scenes"]; ok && kind != "null-scenes" {
				rules["arena_scenes"], _ = json.Marshal(scenes)
			}
			if _, ok := rules["arena_resources"]; ok && kind != "null-resources" {
				rules["arena_resources"], _ = json.Marshal(resources)
			}
			raw, _ := json.Marshal(rules)
			got, err := DecodeChuWorldRules(raw)
			if err == nil || !reflect.DeepEqual(got, ChuWorldLibrary{}) {
				t.Fatal("invalid arena extension returned an available or partial library", err)
			}
		})
	}
}

func TestArenaSessionSelectedReadinessAndStartup(t *testing.T) {
	library := arenaWorldLibraryTest(t)
	for _, arena := range arenaWorldIDsTest {
		t.Run(fmt.Sprintf("%x", arena), func(t *testing.T) {
			options := sessionOptions(false)
			options.Arena = arena
			state, events, err := NewChuSession(options, library)
			if err != nil || state.Match.Arena != arena || state.Clock.Frame != 0 || state.ResourceBusy == 0 || state.ResourceList != library.MatchStart.ArenaResources[arena/2] {
				t.Fatal("selected arena did not use ordinary preparation manager", err)
			}
			requests := 0
			for _, event := range events {
				if event.Kind == "start" && event.Start.Kind == "resource" {
					requests++
					if event.Start.Source != state.ResourceList {
						t.Fatal("selected arena requested another resource")
					}
				}
			}
			if requests != 1 {
				t.Fatal("selected arena did not request resources exactly once")
			}
			resource, err := ChuSessionResourceForArena(arena, library)
			if err != nil {
				t.Fatal(err)
			}
			state, err = CompleteChuSessionResource(state, resource, library)
			if err != nil || state.ResourceBusy != 0 || state.Clock.Frame != 0 || state.Palette.Primary[0] != resource.Palette0[0] {
				t.Fatal("selected resource completion failed", err)
			}
			for frame := 0; frame < 360; frame++ {
				var out WorldFrameOutcome
				state, events, out, err = StepChuSessionFrame(state, [8]Buttons{}, library)
				if err != nil || out.ResetRequested {
					t.Fatal("fresh arena session failed", frame, err)
				}
				if frame == 0 {
					if state.Objects[0x8180].Header.Behavior != library.MatchStart.ArenaBehaviors[arena/2] || state.Objects[0x8180].Header.Stage != 2 {
						t.Fatal("arena callback did not initialize in its ordinary world pass")
					}
					if arena == 10 {
						count := 0
						for _, event := range events {
							if event.RawArenaSprite != nil {
								count++
							}
						}
						if state.Objects[0x9e00].Header.Behavior != 0x21258 || state.Objects[0x9e00].Header.Stage != 2 || count != 6 {
							t.Fatal("new child did not submit six immediate sprites later in the same pass")
						}
					}
				}
				if frame == 180 {
					encoded, _ := json.Marshal(state)
					var restored ChuWorldState
					if err := json.Unmarshal(encoded, &restored); err != nil || !reflect.DeepEqual(restored, state) {
						t.Fatal("fresh selected session restore changed state", err)
					}
					state = restored
				}
			}
			if state.Match.Disabled != 0 || state.Objects[0x8400].InputChannel != 0 {
				t.Fatal("arena session never opened controls or changed explicit human mapping")
			}
		})
	}
}

func TestArenaSessionInvalidSelectionAndReadyAreAtomic(t *testing.T) {
	library := arenaWorldLibraryTest(t)
	for _, arena := range []uint16{1, 3, 15, 16, 0xffff} {
		options := sessionOptions(false)
		options.Arena = arena
		if state, events, err := NewChuSession(options, library); err == nil || !reflect.DeepEqual(state, ChuWorldState{}) || events != nil {
			t.Fatal("unsupported arena constructed a partial session", arena, err)
		}
	}
	for _, arena := range arenaWorldIDsTest {
		options := sessionOptions(false)
		options.Arena = arena
		state, _, err := NewChuSession(options, library)
		if err != nil {
			t.Fatal(err)
		}
		resource, _ := ChuSessionResourceForArena(arena, library)
		for _, kind := range []string{"wrong-source", "wrong-palette", "wrong-arena", "wrong-stage", "missing-registry", "missing-scene", "late", "many-libraries"} {
			before := state
			incoming, selected := resource, library
			switch kind {
			case "wrong-source":
				incoming.Source++
			case "wrong-palette":
				incoming.Palette0[15] ^= 2
			case "wrong-arena":
				before.Match.Arena = (arena + 2) % 16
			case "wrong-stage":
				before.Objects = cloneWorldObjects(before.Objects)
				manager := before.Objects[0x8040]
				manager.Header.Stage = 4
				before.Objects[0x8040] = manager
			case "missing-registry":
				selected.ArenaResources = make(map[uint16]ChuSessionResource)
				for id, value := range library.ArenaResources {
					if id != arena {
						selected.ArenaResources[id] = value
					}
				}
			case "missing-scene":
				if arena == 0 {
					continue
				}
				selected.ArenaScenes = ArenaSceneLibrary{}
			case "late":
				before, _ = CompleteChuSessionResource(before, incoming, selected)
			}
			var got ChuWorldState
			if kind == "many-libraries" {
				got, err = CompleteChuSessionResource(before, incoming, selected, selected)
			} else {
				got, err = CompleteChuSessionResource(before, incoming, selected)
			}
			if err == nil || !reflect.DeepEqual(got, before) {
				t.Fatal("invalid selected completion mutated world or succeeded", arena, kind, err)
			}
		}
		if arena != 0 {
			if got, err := CompleteChuSessionResource(state, resource); err == nil || !reflect.DeepEqual(got, state) {
				t.Fatal("nondefault readiness invented an absent resource registry")
			}
		}
	}
	options := sessionOptions(false)
	raw, _ := json.Marshal(options)
	if bytes.Contains(raw, []byte(`"Arena"`)) {
		t.Fatal("default session options wire JSON changed")
	}
}

func TestArenaWorldGatingAndLateRollback(t *testing.T) {
	library := arenaWorldLibraryTest(t)
	for _, arena := range arenaWorldIDsTest[1:] {
		rows := arenaWorldPassesTest(t, arena)
		base := decodeChuWorldTest(t, rows[0].Before)
		base.Objects = WorldObjects{0x8180: base.Objects[0x8180]}
		if arena == 10 {
			base.Objects[0x9e00] = decodeChuWorldTest(t, rows[0].Before).Objects[0x9e00]
		}
		for _, slow := range []bool{false, true} {
			state := base
			if slow {
				state.Clock.SlowFlag = 0x80
			} else {
				state.Clock.PauseFlags |= 0x80
			}
			got, events, err := StepChuWorldObjects(state, ChuWorldView{}, library)
			if err != nil || !reflect.DeepEqual(got, state) {
				t.Fatal("arena world gate changed owned state", arena, slow, err)
			}
			count := 0
			for _, event := range events {
				if event.RawArenaSprite != nil {
					count++
				}
			}
			if count != 0 && arena != 10 || arena == 10 && count != 6 {
				t.Fatal("arena child gate suppressed or invented raw sprites", arena, count)
			}
		}
		state := base
		state.Clock.PanOffset = 123
		state.BackdropColor = 0x3f
		state.Objects = cloneWorldObjects(base.Objects)
		state.Objects[0x9f00] = WorldObject{ObjectID: 0x9f00, Header: MatchObjectHeader{Behavior: 0x123456, Size: 64}}
		before, _ := json.Marshal(state)
		got, events, err := StepChuWorldObjects(state, ChuWorldView{}, library)
		after, _ := json.Marshal(state)
		if err == nil || events != nil || !reflect.DeepEqual(got, state) || !bytes.Equal(before, after) {
			t.Fatal("late callback failure leaked earlier arena/child/Stream mutations", arena, err)
		}
	}
}
