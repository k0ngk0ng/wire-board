package basketball_test

// Before implementation: the saved room never crosses frame 8963; its frozen
// input clock rejects every later press; HTTP recovery cannot repair the same
// deterministic error; reconnect may reset scores or resurrect held inputs.
import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
	game "github.com/k0ngk0ng/wire-board/games/basketball/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func hoopIncidentModule(t *testing.T) gameapi.Module {
	t.Helper()
	f, err := os.Open("../engine/testdata/hoop-air-throw-runtime.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var runtime engine.RuntimeState
	if err = json.NewDecoder(z).Decode(&runtime); err != nil {
		t.Fatal(err)
	}
	var original struct {
		ServerAt      int64           `json:"serverAt"`
		SoundSequence uint64          `json:"soundSequence"`
		Music         game.MusicState `json:"music"`
	}
	music, err := os.ReadFile("testdata/hoop-incident-music.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(music, &original); err != nil {
		t.Fatal(err)
	}
	mod := observedModule(t)
	start := mod.Start
	mod.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		m, err := start(ctx, raw)
		if err != nil {
			return m, err
		}
		var state game.State
		if err = json.Unmarshal(m.Data, &state); err != nil {
			return m, err
		}
		// Only the initial saved engine/media are supplied; subsequent timers, input
		// authorization, WebSocket frames, persistence and restore are real.
		state.Runtime = runtime
		state.SoundSequence = original.SoundSequence
		state.Music = original.Music
		state.Music.ElapsedMS = original.Music.PositionMS(original.ServerAt)
		state.Music.StartedAt = ctx.Now.UnixMilli()
		state.Options.Characters = runtime.Config.Match.Characters
		state.Options.Teams = runtime.Config.Match.Teams
		m.Data, err = json.Marshal(state)
		return m, err
	}
	return mod
}

func TestBasketballHoopIncidentStreamRecovery(t *testing.T) {
	mod := hoopIncidentModule(t)
	h := gametest.NewHTTP(t, 1, mod, "basketball")
	h.Start(map[string]any{"mode": "solo", "court": "court-4"})
	basketballReadyHTTP(t, h)
	c := h.People[0]
	if current := view(t, c); current.Music.TrackID != 60 || current.Music.PositionMS < 149299 {
		t.Fatal("incident music timeline was not restored", current.Music)
	}
	conn := basketballStream(t, c)
	state := basketballPacket(t, conn, "state")["state"].(map[string]any)
	room := state["room"].(map[string]any)
	var authority map[string]any
	for n := 0; n < 30; n++ {
		packet := basketballPacket(t, conn, "game")
		authority = packet["game"].(map[string]any)["data"].(map[string]any)
		if _, exists := authority["runtime"]; exists {
			t.Fatal("steady stream contains full runtime")
		}
		if authority["frame"].(float64) > 9000 {
			break
		}
	}
	if authority["frame"].(float64) <= 9000 {
		t.Fatal("saved incident stayed frozen")
	}
	seq := authority["inputs"].([]any)[0].(map[string]any)["sequence"].(float64) + 1
	basketballSend(t, conn, map[string]any{"type": "input", "roomId": room["id"], "version": room["version"], "serverInstance": state["serverInstance"], "nonce": "incident-live-input", "action": map[string]any{"type": "inputs", "epoch": authority["epoch"], "seat": 0, "basedOn": authority["serverAt"], "sequence": seq, "events": []any{map[string]any{"sequence": seq, "buttons": 1, "offsetMS": 0, "release": false}}}})
	if ack := basketballPacket(t, conn, "ack"); ack["status"] != float64(200) {
		t.Fatal("fresh input rejected after incident", ack)
	}
	before := view(t, c)
	conn.CloseNow()
	h.Restart()
	after := view(t, c)
	if after.Epoch != before.Epoch || after.Frame < before.Frame || after.Scores != before.Scores || after.Inputs[0].Buttons != 0 {
		t.Fatal("restart changed saved game or retained input")
	}
	conn = basketballStream(t, c)
	basketballPacket(t, conn, "state")
	packet := basketballPacket(t, conn, "game")
	if packet["game"].(map[string]any)["data"].(map[string]any)["frame"].(float64) <= float64(after.Frame) {
		t.Fatal("restored stream stopped")
	}
	h.Record("original failing snapshot crosses frame 8964; compact WebSocket updates and authenticated input continue; restart preserves score and clears held input")
}
