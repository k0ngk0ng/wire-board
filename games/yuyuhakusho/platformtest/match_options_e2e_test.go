package platformtest

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/runtimebridge"
	game "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/server"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/syncwire"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func TestRandomOneVsThreeRealRoomPersistence(t *testing.T) {
	h := gametest.NewHTTP(t, 4, registered(t), "yuyuhakusho")
	h.Start(map[string]any{"arena": 65535, "characters": []int{255, 9, 255, 255}, "teamBattle": true, "oneVsThree": true})
	initial := wire(t, h.People[0])
	if !initial.Options.OneVsThree || !initial.Options.TeamBattle || initial.Options.Arena > 14 || initial.Options.Characters[1] != 9 {
		t.Fatal("real start did not resolve random selections", initial.Options)
	}
	for _, person := range h.People {
		if !reflect.DeepEqual(wire(t, person).Options, initial.Options) {
			t.Fatal("independent clients received different selections")
		}
	}
	load(t, h)
	wait(t, h.People[0], func(view game.View) bool { return view.Frame > 5 })
	h.People[1].Command("autoplay", map[string]any{"enabled": true}, 200)
	wait(t, h.People[0], func(view game.View) bool { return view.Inputs[1].Automated })
	h.People[1].Command("autoplay", map[string]any{"enabled": false}, 200)
	wait(t, h.People[0], func(view game.View) bool { return !view.Inputs[1].Automated })
	h.Restart()
	view := wire(t, h.People[0])
	if view.Epoch != initial.Epoch || !reflect.DeepEqual(view.Options, initial.Options) || view.Phase == "loading" {
		t.Fatal("restart changed selections, mode or readiness", view.Options, view.Phase)
	}
	var saved game.Checkpoint
	if err := json.Unmarshal([]byte(checkpoint(t, h.People[0]).Checkpoint), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.World.Match.BaseTeams != [4]uint8{1, 4, 4, 4} || !saved.Options.OneVsThree {
		t.Fatal("persisted checkpoint lost teams", saved.World.Match.BaseTeams)
	}
	h.Record("four independent HTTP sessions agree on random selections; one-versus-three teams survive explicit autoplay and SQLite restart")
}

func TestOneVsThreeRealCheckpointClientReplay(t *testing.T) {
	assets := cursorAssets(t)
	h := gametest.NewHTTP(t, 4, registered(t), "yuyuhakusho")
	h.Start(map[string]any{"characters": []int{255, 255, 255, 255}, "teamBattle": true, "oneVsThree": true})
	load(t, h)
	initial := checkpoint(t, h.People[0])
	client, err := runtimebridge.RestoreCheckpoint([]byte(initial.Checkpoint), assets)
	if err != nil {
		t.Fatal(err)
	}
	confirmed := client.View().Tick
	for round := 0; round < 5; round++ {
		h.People[1].Command("autoplay", map[string]any{"enabled": round%2 == 0}, 200)
		view := wait(t, h.People[0], func(view game.View) bool { return view.Frame >= confirmed+10 })
		frames, err := syncwire.Decode(view.Replay)
		if err != nil {
			t.Fatal(err)
		}
		for _, input := range frames {
			if input.Frame <= confirmed {
				continue
			}
			if input.Frame != confirmed+1 {
				t.Fatal("replay missed a frame", input.Frame, confirmed)
			}
			if _, err = client.AdvanceControlled(input.Held, input.Automated); err != nil {
				t.Fatal(err)
			}
			confirmed++
		}
		rules, presentation, err := client.Digests()
		if err != nil || confirmed != view.Frame || rules != view.Digest || presentation != view.PresentationDigest {
			t.Fatal("client one-versus-three replay diverged", confirmed, view.Frame, err)
		}
	}
	h.Record("one-versus-three opaque checkpoint and five autoplay transitions replay identical client rule and presentation digests")
}
