package junqi_test

// Written before implementation. Failure inventory is maintained in
// docs/research/chess-family/junqi-implementation.md. Real HTTP/SQLite checks:
// publication, invalid player count/formation, per-seat concurrent deployment,
// hidden ranks/order/IDs, road/rail/camp/HQ, combat, elimination and restoration.
import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/junqi/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func state(c *gametest.Client) game.View {
	var v game.View
	b, _ := json.Marshal(gametest.Data(c.Room()))
	if e := json.Unmarshal(b, &v); e != nil {
		panic(e)
	}
	return v
}
func request(v game.View, kind string) map[string]any {
	return map[string]any{"type": kind, "round": v.Round, "ply": v.Ply, "revision": v.Revision}
}
func record(t *testing.T, name string, v any) {
	t.Helper()
	dir := filepath.Join("..", "..", "..", ".local", "junqi", "http")
	if e := os.MkdirAll(dir, 0755); e != nil {
		t.Fatal(e)
	}
	b, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "evidence": v}, "", "  ")
	if e := os.WriteFile(filepath.Join(dir, name+".json"), b, 0600); e != nil {
		t.Fatal(e)
	}
}
func own(v game.View, seat int, kind string) game.PieceView {
	for _, p := range v.Pieces {
		if p.Seat == seat && p.Kind == kind {
			return p
		}
	}
	panic("missing own " + kind)
}
func ready(h *gametest.HTTP) {
	for _, p := range h.People {
		v := state(p)
		a := request(v, "deploy_ready")
		a["ready"] = true
		p.Action(a, 200)
	}
}
func TestJunqiHTTPDeploymentAndPrivacy(t *testing.T) {
	for _, n := range []int{2, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			h := gametest.NewHTTP(t, n, game.Module(), "junqi")
			h.AssertUnpublished()
			h.Start(map[string]any{})
			v := state(h.People[0])
			if v.Phase != "deploy" || len(v.Pieces) != 25*n || len(v.Graph.Nodes) != 30*n+map[int]int{2: 0, 4: 9}[n] {
				t.Fatal("wrong board/setup", v)
			}
			for seat, p := range h.People {
				view := state(p)
				counts := map[string]int{}
				ids := map[string]bool{}
				last := -1
				for _, unit := range view.Pieces {
					if ids[unit.ID] || len(unit.ID) < 16 {
						t.Fatal("predictable/duplicate piece ID")
					}
					ids[unit.ID] = true
					if unit.Pos < last {
						t.Fatal("view order leaks rank inventory")
					}
					last = unit.Pos
					if unit.Seat != seat && unit.Kind != "unknown" {
						t.Fatal("other rank leaked", unit)
					}
					if unit.Seat == seat {
						counts[unit.Kind]++
					}
				}
				if !reflect.DeepEqual(counts, map[string]int{"commander": 1, "general": 1, "division": 2, "brigade": 2, "regiment": 2, "battalion": 2, "company": 3, "platoon": 3, "engineer": 3, "bomb": 2, "mine": 3, "flag": 1}) {
					t.Fatal("wrong inventory", counts)
				}
			}
			for _, unit := range state(h.Spectator).Pieces {
				if unit.Kind != "unknown" {
					t.Fatal("spectator rank leaked")
				}
			}
			flag := own(v, 0, "flag")
			front := game.PieceView{}
			for _, p := range v.Pieces {
				if p.Seat == 0 && p.Pos/5%6 == 0 {
					front = p
					break
				}
			}
			a := request(v, "swap")
			a["unit"], a["other"] = flag.ID, front.ID
			h.People[0].Action(a, 400)
			if !reflect.DeepEqual(v, state(h.People[0])) {
				t.Fatal("illegal flag swap mutated setup")
			}
			old := state(h.People[1])
			oldDeadline := old.Deadlines[1]
			a = request(v, "shuffle")
			payload := h.People[0].ActionRequest(a)
			url := "/api/rooms/" + h.People[0].Room()["id"].(string)
			h.People[0].Request("POST", url, payload, 200)
			shuffled := state(h.People[0])
			h.People[0].Request("POST", url, payload, 200)
			if !reflect.DeepEqual(shuffled, state(h.People[0])) {
				t.Fatal("duplicate shuffle")
			}
			if state(h.People[1]).Deadlines[1] != oldDeadline {
				t.Fatal("other deployment deadline changed")
			}
			ready(h)
			v = state(h.People[0])
			if v.Phase != "playing" || v.Turn != 0 {
				t.Fatal("did not start battle")
			}
			before := []game.View{}
			for _, p := range h.People {
				before = append(before, state(p))
			}
			h.Restart()
			for i, p := range h.People {
				if !reflect.DeepEqual(before[i], state(p)) {
					t.Fatal("restore changed hidden view", i)
				}
			}
			record(t, fmt.Sprintf("deploy-privacy-%d", n), before)
		})
	}
}
func TestJunqiHTTPConcurrentDeployment(t *testing.T) {
	h := gametest.NewHTTP(t, 4, game.Module(), "junqi")
	h.Start(map[string]any{})
	oldVersion := h.People[0].Room()["version"]
	url := "/api/rooms/" + h.People[0].Room()["id"].(string)
	actions := []map[string]any{}
	for _, p := range h.People {
		v := state(p)
		a := request(v, "deploy_ready")
		a["ready"] = true
		actions = append(actions, a)
	}
	// All four requests intentionally carry the same earlier room version.
	var group sync.WaitGroup
	for i, p := range h.People {
		group.Add(1)
		go func() {
			defer group.Done()
			p.Request("POST", url, map[string]any{"type": "action", "version": oldVersion, "nonce": fmt.Sprintf("deployment-seat-%d", i), "action": actions[i]}, 200)
		}()
	}
	group.Wait()
	if t.Failed() {
		return
	}
	v := state(h.People[0])
	if v.Phase != "playing" {
		t.Fatal("other seat updates rejected own legal deployment")
	}
	h.People[0].Action(actions[0], 400)
	record(t, "concurrent-deployment", v)
}

// Explicit reduced middle-game fixture, never a production creation option.
func battle(n int, pieces []game.Piece) gameapi.Module {
	m := game.Module()
	start := m.Start
	m.Start = func(c gameapi.Context, o json.RawMessage) (gameapi.Match, error) {
		v, e := start(c, o)
		if e != nil {
			return v, e
		}
		var s game.State
		_ = json.Unmarshal(v.Data, &s)
		s.Phase, s.Turn = "playing", 0
		s.Pieces = append([]game.Piece(nil), pieces...)
		for i := range s.Ready {
			s.Ready[i] = true
			s.Deadlines[i] = 0
		}
		s.Deadlines[0] = c.Now.Add(120 * time.Second).UnixMilli()
		s.Keys = []string{game.PositionKey(&s)}
		v.Actors = make([]int, n)
		for i := range v.Actors {
			v.Actors[i] = i
		}
		v.TimeoutActors = []int{0}
		v.Attention = []int{0}
		v.Deadline = s.Deadlines[0]
		v.Data, _ = json.Marshal(s)
		return v, nil
	}
	return m
}
func piece(id string, seat int, kind string, pos int) game.Piece {
	return game.Piece{ID: id, Seat: seat, Kind: kind, Pos: pos, Alive: true}
}
func flags(n int) []game.Piece {
	p := []game.Piece{}
	for seat := 0; seat < n; seat++ {
		p = append(p, piece(fmt.Sprintf("flag-seat-%d", seat), seat, "flag", game.Home(seat, 5, 1)), piece(fmt.Sprintf("guard-seat-%d", seat), seat, "company", game.Home(seat, 3, 2)))
	}
	return p
}
func actMove(h *gametest.HTTP, unit string, to int, status int) game.View {
	v := state(h.People[0])
	a := request(state(h.People[v.Turn]), "move")
	a["unit"], a["to"] = unit, to
	h.People[v.Turn].Action(a, status)
	return state(h.People[0])
}
func TestJunqiHTTPCombat(t *testing.T) {
	for _, tc := range []struct {
		a, b, outcome  string
		aliveA, aliveB bool
	}{
		{"commander", "general", "attacker", true, false}, {"platoon", "regiment", "defender", false, true}, {"division", "division", "both", false, false}, {"engineer", "mine", "attacker", true, false}, {"commander", "mine", "defender", false, true}, {"bomb", "mine", "both", false, false}, {"engineer", "bomb", "both", false, false}, {"bomb", "commander", "both", false, false},
	} {
		t.Run(tc.a+"-"+tc.b, func(t *testing.T) {
			p := append(flags(2), piece("attacker", 0, tc.a, game.Home(0, 0, 2)), piece("defender", 1, tc.b, game.Home(1, 0, 2)))
			h := gametest.NewHTTP(t, 2, battle(2, p), "junqi")
			h.Start(map[string]any{})
			v := actMove(h, "attacker", game.Home(1, 0, 2), 200)
			if v.Last.Outcome != tc.outcome {
				t.Fatal(v.Last)
			}
			found := func(id string) bool {
				for _, p := range v.Pieces {
					if p.ID == id {
						return true
					}
				}
				return false
			}
			if found("attacker") != tc.aliveA || found("defender") != tc.aliveB {
				t.Fatal("wrong casualties", v.Pieces)
			}
			if tc.a == "commander" && !tc.aliveA && !v.Players[0].FlagRevealed {
				t.Fatal("own commander death did not reveal flag")
			}
			if tc.b == "commander" && !tc.aliveB && !v.Players[1].FlagRevealed {
				t.Fatal("enemy commander death did not reveal flag")
			}
			if strings.Contains(strings.Join(v.Log, " "), tc.b) && tc.b != "commander" {
				t.Fatal("combat log exposed private kind")
			}
			for _, privateName := range []string{"军长", "师长", "旅长", "团长", "营长", "连长", "排长", "工兵", "炸弹", "地雷"} {
				if strings.Contains(strings.Join(v.Log, " "), privateName) {
					t.Fatal("combat log exposed a hidden Chinese rank", privateName)
				}
			}
			record(t, "combat-"+tc.a+"-"+tc.b, v)
		})
	}
}
func TestJunqiHTTPPaths(t *testing.T) {
	for _, tc := range []struct {
		name          string
		n             int
		kind          string
		from, to      int
		valid, turned bool
		block         *game.Piece
	}{
		{"road", 2, "company", game.Home(0, 2, 1), game.Home(0, 2, 2), true, false, nil},
		{"camp-diagonal", 2, "company", game.Home(0, 0, 0), game.Home(0, 1, 1), true, false, nil},
		{"rail-straight", 2, "company", game.Home(0, 4, 0), game.Home(1, 4, 4), true, false, nil},
		{"ordinary-no-right-angle", 2, "company", game.Home(0, 3, 0), game.Home(0, 4, 3), false, false, nil},
		{"engineer-turn", 2, "engineer", game.Home(0, 3, 0), game.Home(0, 4, 3), true, true, nil},
		{"headquarters-stuck", 2, "company", game.Home(0, 5, 3), game.Home(0, 4, 3), false, false, nil},
		{"mine-stuck", 2, "mine", game.Home(0, 4, 3), game.Home(0, 4, 2), false, false, nil},
		{"four-country-curve", 4, "company", game.Home(0, 3, 0), game.Home(1, 3, 4), true, false, nil},
		{"central-straight", 4, "company", game.Home(0, 0, 2), game.Home(2, 0, 2), true, false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := append(flags(tc.n), piece("mover", 0, tc.kind, tc.from))
			h := gametest.NewHTTP(t, tc.n, battle(tc.n, p), "junqi")
			h.Start(map[string]any{})
			before := state(h.People[0])
			status := 400
			if tc.valid {
				status = 200
			}
			v := actMove(h, "mover", tc.to, status)
			if !tc.valid && !reflect.DeepEqual(before, v) {
				t.Fatal("illegal route mutated state")
			}
			if tc.valid && v.Last.Moves[0].Turned != tc.turned {
				t.Fatal("wrong railway-turn classification", v.Last)
			}
			if tc.turned {
				other := state(h.People[1])
				seen := false
				for _, p := range other.Pieces {
					if p.ID == "mover" {
						seen = p.Kind == "engineer"
					}
				}
				if !seen {
					t.Fatal("turning engineer not public")
				}
			}
			record(t, "path-"+tc.name, v)
		})
	}
}
func TestJunqiHTTPTeammateCampAndBlocker(t *testing.T) {
	for _, name := range []string{"ally", "camp", "blocker"} {
		t.Run(name, func(t *testing.T) {
			p := flags(4)
			from, to := game.Home(0, 0, 2), game.Home(2, 0, 2)
			p = append(p, piece("mover", 0, "commander", from))
			switch name {
			case "ally":
				p = append(p, piece("target", 2, "platoon", to))
			case "camp":
				to = game.Home(0, 1, 1)
				p = append(p, piece("target", 1, "platoon", to))
			case "blocker":
				p = append(p, piece("target", 1, "platoon", 120+7))
			}
			h := gametest.NewHTTP(t, 4, battle(4, p), "junqi")
			h.Start(map[string]any{})
			before := state(h.People[0])
			actMove(h, "mover", to, 400)
			if !reflect.DeepEqual(before, state(h.People[0])) {
				t.Fatal("forbidden target changed state")
			}
			record(t, "forbidden-"+name, before)
		})
	}
}

func TestJunqiHTTPFlagAndTeams(t *testing.T) {
	for _, kind := range []string{"company", "bomb"} {
		t.Run(kind, func(t *testing.T) {
			p := append(flags(2), piece("invader", 0, kind, game.Home(1, 4, 1)))
			h := gametest.NewHTTP(t, 2, battle(2, p), "junqi")
			h.Start(map[string]any{})
			v := actMove(h, "invader", game.Home(1, 5, 1), 200)
			if v.Phase != "finished" || v.Winner != 0 || !v.Last.FlagCaptured || v.Players[1].Remaining != 0 || v.Players[1].Alive {
				t.Fatal("flag did not eliminate whole army", v)
			}
			if kind == "bomb" {
				for _, p := range v.Pieces {
					if p.ID == "invader" {
						t.Fatal("bomb survived flag capture")
					}
				}
			}
			record(t, "flag-"+kind, v)
		})
	}
	h := gametest.NewHTTP(t, 4, battle(4, append(flags(4), piece("partner-invader", 2, "engineer", game.Home(3, 4, 1)))), "junqi")
	h.Start(map[string]any{})
	for _, seat := range []int{0, 1} {
		v := state(h.People[seat])
		h.People[seat].Action(request(v, "resign"), 200)
		if state(h.People[2]).Phase != "playing" {
			t.Fatal("one teammate eliminated ended the team game")
		}
	}
	v := actMove(h, "partner-invader", game.Home(3, 5, 1), 200)
	if v.Phase != "finished" || v.Winner != 2 {
		t.Fatal("wrong team outcome", v)
	}
	room := h.People[2].Room()
	w := room["game"].(map[string]any)["winners"]
	if !reflect.DeepEqual(w, []any{float64(0), float64(2)}) {
		t.Fatal("eliminated teammate excluded from team win", w)
	}
	record(t, "eliminated-partner-team-win", room)
}
func TestJunqiHTTPNoMovesAndDrawClock(t *testing.T) {
	p := []game.Piece{}
	for _, u := range flags(2) {
		if u.ID != "guard-seat-1" {
			p = append(p, u)
		}
	}
	p = append(p, piece("immobile-mine", 1, "mine", game.Home(1, 4, 0)))
	h := gametest.NewHTTP(t, 2, battle(2, p), "junqi")
	h.Start(map[string]any{})
	v := actMove(h, "guard-seat-0", game.Home(0, 3, 1), 200)
	if v.Phase != "finished" || v.Winner != 0 || v.Players[1].Alive {
		t.Fatal("no-move army not eliminated", v)
	}
	record(t, "no-legal-moves", v)
	h = gametest.NewHTTP(t, 4, game.Module(), "junqi")
	h.Start(map[string]any{})
	ready(h)
	v = state(h.People[0])
	deadlines := append([]int64(nil), v.Deadlines...)
	h.People[0].Action(request(v, "offer_draw"), 200)
	v = state(h.People[1])
	old := request(v, "vote_draw")
	old["offerID"], old["accept"] = v.Offer.ID, true
	reject := request(v, "vote_draw")
	reject["offerID"], reject["accept"] = v.Offer.ID, false
	h.People[1].Action(reject, 200)
	h.People[0].Action(request(state(h.People[0]), "offer_draw"), 200)
	h.People[1].Action(old, 400)
	for seat := 1; seat < 4; seat++ {
		v = state(h.People[seat])
		if !reflect.DeepEqual(v.Deadlines, deadlines) {
			t.Fatal("votes changed turn deadlines")
		}
		a := request(v, "vote_draw")
		a["offerID"], a["accept"] = v.Offer.ID, true
		h.People[seat].Action(a, 200)
	}
	v = state(h.People[0])
	if v.Phase != "finished" || v.Winner != -1 || v.Reason != "agreement" {
		t.Fatal("unanimous draw did not settle")
	}
	record(t, "draw-clock-and-offer-identity", h.People[0].Room())
}
func TestJunqiHTTPRepetitionAndNoCasualty(t *testing.T) {
	p := append(flags(2), piece("shuttle0", 0, "company", game.Home(0, 2, 1)), piece("shuttle1", 1, "company", game.Home(1, 2, 1)))
	h := gametest.NewHTTP(t, 2, battle(2, p), "junqi")
	h.Start(map[string]any{})
	for n := 0; n < 12; n++ {
		seat := n % 2
		col := 2
		if n%4 >= 2 {
			col = 1
		}
		actMove(h, fmt.Sprintf("shuttle%d", seat), game.Home(seat, 2, col), 200)
	}
	v := state(h.People[0])
	if v.Phase != "finished" || v.Reason != "repetition" || v.Winner != -1 {
		t.Fatal("repetition not drawn", v.Reason)
	}
	record(t, "repetition", v)
	m := battle(2, flags(2))
	start := m.Start
	m.Start = func(c gameapi.Context, o json.RawMessage) (gameapi.Match, error) {
		v, e := start(c, o)
		var s game.State
		_ = json.Unmarshal(v.Data, &s)
		s.NoCasualty = 199
		v.Data, _ = json.Marshal(s)
		return v, e
	}
	h = gametest.NewHTTP(t, 2, m, "junqi")
	h.Start(map[string]any{})
	v = actMove(h, "guard-seat-0", game.Home(0, 3, 1), 200)
	if v.Reason != "no_casualty" || v.NoCasualty != 200 || v.Winner != -1 {
		t.Fatal("no-casualty limit incorrect", v)
	}
	record(t, "no-casualty-boundary", v)
}

func TestJunqiHTTPRejectedFormationAndOptions(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "junqi")
	a := h.People[0]
	a.Request("POST", "/api/admin/games/junqi", map[string]bool{"enabled": true}, 200)
	for _, capacity := range []int{1, 3, 5} {
		a.Request("POST", "/api/rooms", map[string]any{"kind": "junqi", "name": "拒绝不支持人数", "capacity": capacity}, 400)
	}
	for _, opts := range []map[string]any{{"layout": []int{1, 2}}, {"difficulty": "unknown"}, {"setupSeconds": 1}} {
		a.Request("POST", "/api/rooms", map[string]any{"kind": "junqi", "name": "拒绝非法布阵选项", "capacity": 2, "options": opts}, 400)
	}
	h.Start(map[string]any{})
	v := state(a)
	front := game.PieceView{}
	for _, p := range v.Pieces {
		if p.Seat == 0 && p.Pos < 5 {
			front = p
			break
		}
	}
	for _, kind := range []string{"mine", "bomb"} {
		unit := own(v, 0, kind)
		req := request(v, "swap")
		req["unit"], req["other"] = unit.ID, front.ID
		a.Action(req, 400)
		if !reflect.DeepEqual(v, state(a)) {
			t.Fatal("invalid formation mutated", kind)
		}
	}
	enemy := ""
	for _, p := range v.Pieces {
		if p.Seat == 1 {
			enemy = p.ID
			break
		}
	}
	req := request(v, "swap")
	req["unit"], req["other"] = front.ID, enemy
	a.Action(req, 400)
	h.Spectator.Action(req, 400)
	req = request(v, "deploy_ready")
	req["ready"] = true
	a.Action(req, 200)
	locked := state(a)
	a.Action(request(locked, "shuffle"), 400)
	req = request(locked, "deploy_ready")
	req["ready"] = false
	a.Action(req, 200)
	unlocked := state(a)
	if !unlocked.CanArrange || unlocked.Deadlines[0] != v.Deadlines[0] {
		t.Fatal("unready changed deadline")
	}
	record(t, "invalid-formation-options", unlocked)
}

func TestJunqiHTTPRecentPublicMovesAndRestore(t *testing.T) {
	h := gametest.NewHTTP(t, 2, battle(2, flags(2)), "junqi")
	h.Start(map[string]any{})
	actMove(h, "guard-seat-0", game.Home(0, 3, 1), 200)
	actMove(h, "guard-seat-1", game.Home(1, 3, 1), 200)
	v := state(h.People[0])
	if len(v.Recent) != 2 || v.Recent[0].Unit != "guard-seat-0" || v.Recent[1].Unit != "guard-seat-1" {
		t.Fatal("missing prior own movement")
	}
	before := v
	h.Restart()
	if !reflect.DeepEqual(before, state(h.People[0])) {
		t.Fatal("recent public movement lost on restore")
	}
	record(t, "recent-public-moves", v)
}

func TestJunqiHTTPEliminationOrderDoesNotLeakRanks(t *testing.T) {
	h := gametest.NewHTTP(t, 4, game.Module(), "junqi")
	h.Start(map[string]any{})
	ready(h)
	v := state(h.People[1])
	h.People[1].Action(request(v, "resign"), 200)
	v = state(h.Spectator)
	if v.Phase != "playing" || len(v.Last.Removed) != 25 {
		t.Fatal("fixture did not clear an army while team play continues")
	}
	if !sort.StringsAreSorted(v.Last.Removed) {
		t.Fatal("removed IDs preserve private inventory order")
	}
	for _, p := range v.Pieces {
		if p.Kind != "unknown" {
			t.Fatal("spectator learned surviving ranks")
		}
	}
	record(t, "elimination-event-order", v)
}

func TestJunqiHTTPFourSeatRoomCannotStartAsDuel(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "junqi")
	a, b := h.People[0], h.People[1]
	a.Request("POST", "/api/admin/games/junqi", map[string]bool{"enabled": true}, 200)
	r := a.Request("POST", "/api/rooms", map[string]any{"kind": "junqi", "name": "四国席位绑定", "capacity": 4}, 201)
	b.Request("POST", "/api/rooms/"+r["id"].(string), map[string]any{"type": "join", "version": a.Room()["version"], "nonce": "join-second-of-four"}, 200)
	a.Command("ready", nil, 200)
	b.Command("ready", nil, 200)
	a.Command("start", nil, 400)
	if a.Room()["status"] != "waiting" {
		t.Fatal("four-country room silently became duel")
	}
	record(t, "capacity-bound-mode", a.Room())
}
