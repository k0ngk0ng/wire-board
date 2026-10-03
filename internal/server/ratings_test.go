package server

import (
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
)

func TestRatingLedgerAtomicIdempotentAndHistory(t *testing.T) {
	s, ts := setupServer(t)
	a, b := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("RankOne")
	b.register("RankTwo")
	aid := a.state()["user"].(map[string]any)["id"].(string)
	bid := b.state()["user"].(map[string]any)["id"].(string)
	g, _ := game.New("splendor", 2)
	g.Finished = true
	g.Winners = []int{0}
	g.Splendor.Players[0].Score = 16
	g.Splendor.Players[1].Score = 12
	room := &Room{ID: "rated", MatchID: "rated-first", Kind: "splendor", Status: "finished", Rated: true, Game: g, Seats: []Seat{{User: User{ID: aid}}, {User: User{ID: bid}}}}
	s.mu.Lock()
	for range 3 {
		if err := s.save(room); err != nil {
			t.Fatal(err)
		}
	}
	x, err := s.rating(aid)
	if err != nil || x.Points != 1020 || x.Played != 1 {
		t.Fatal(x, err)
	}
	y, _ := s.rating(bid)
	if y.Points != 995 {
		t.Fatal(y)
	}
	// An aborted match, a bot match, and a pre-feature match must never score.
	room.MatchID = "aborted"
	room.Status = "closed"
	if err = s.save(room); err != nil {
		t.Fatal(err)
	}
	room.MatchID = "computer"
	room.Status = "finished"
	room.Seats[1].Bot = true
	if err = s.save(room); err != nil {
		t.Fatal(err)
	}
	room.MatchID = "old"
	room.Seats[1].Bot = false
	room.Rated = false
	if err = s.save(room); err != nil {
		t.Fatal(err)
	}
	x, _ = s.rating(aid)
	if x.Points != 1020 || x.Played != 1 {
		t.Fatal(x)
	}
	// A rollback cannot leave points detached from the match snapshot.
	room.MatchID = "rollback"
	room.Rated = true
	tx, _ := s.db.Begin()
	if err = archiveGame(tx, room); err != nil {
		t.Fatal(err)
	}
	tx.Rollback()
	x, _ = s.rating(aid)
	if x.Points != 1020 {
		t.Fatal(x)
	}
	s.mu.Unlock()
	code, data := a.request("GET", "/api/leaderboard", nil)
	if code != 200 {
		t.Fatal(code, data)
	}
	list := data["players"].([]any)
	if len(list) != 2 || list[0].(map[string]any)["points"] != float64(1020) {
		t.Fatal(data)
	}
	_, profile := a.request("GET", "/api/players/"+aid, nil)
	if profile["rating"].(map[string]any)["points"] != float64(1020) {
		t.Fatal(profile)
	}
}
func TestRatingRanksRespectTiesAndElimination(t *testing.T) {
	g, _ := game.New("splendor", 4)
	g.Finished = true
	g.Winners = []int{0}
	for i, v := range []int{15, 10, 10, 30} {
		g.Splendor.Players[i].Score = v
	}
	g.Splendor.Players[3].Eliminated = true
	r := &Room{Rated: true, Status: "finished", Kind: "splendor", Game: g, Seats: make([]Seat, 4)}
	rec := MatchRecord{Players: make([]MatchPlayer, 4)}
	rateMatch(r, &rec)
	for i, want := range []int{20, -5, -5, -15} {
		if rec.Players[i].RatingDelta != want {
			t.Fatal(rec)
		}
	}
	g.Winners = []int{0, 1}
	g.Splendor.Players[1].Score = 15
	rateMatch(r, &rec)
	if rec.Players[1].RatingDelta != 20 || rec.Players[2].RatingDelta != -10 {
		t.Fatal(rec)
	}
	r.Kind = "sanguosha"
	rateMatch(r, &rec)
	if rec.Players[0].RatingDelta != 20 || rec.Players[1].RatingDelta != 20 || rec.Players[2].RatingDelta != -10 || rec.Players[3].RatingDelta != -10 {
		t.Fatal(rec)
	}
}
