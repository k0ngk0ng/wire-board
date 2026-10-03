package server

import (
	"database/sql"
	"net/http"
	"slices"
	"strconv"
)

const initialRating = 1000

// Ratings are a ledger, committed atomically with the first match archive.
// A room opts in when a new match starts, so deployment never re-rates history.
func rateMatch(r *Room, record *MatchRecord) {
	record.Rated = r.Rated && r.Status == "finished" && r.Game.Finished && len(r.Game.Winners) > 0
	for _, p := range r.Seats {
		if p.Bot {
			record.Rated = false
		}
	}
	if !record.Rated {
		return
	}
	type result struct {
		winner, out            bool
		score, tie, tie2, tie3 int
	}
	results := make([]result, len(r.Seats))
	for i := range results {
		x := &results[i]
		x.winner = slices.Contains(r.Game.Winners, i)
		x.out = r.Seats[i].Left
		if g := r.Game.Splendor; g != nil {
			p := g.Players[i]
			x.score = p.Score
			x.tie = -len(p.Cards)
			x.out = x.out || p.Eliminated
		}
		if g := r.Game.Rail; g != nil {
			p := g.Players[i]
			x.score = p.Score
			x.tie, x.tie2, x.tie3 = g.TieBreak(i)
			x.out = x.out || p.Eliminated
		}
		if g := r.Game.Catan; g != nil {
			p := g.Players[i]
			x.score = p.Score
			x.out = x.out || p.Eliminated
		}
		if g := r.Game.Carcassonne; g != nil {
			p := g.Players[i]
			x.score = p.Score
			x.out = x.out || p.Eliminated
		}
	}
	better := func(a, b result) bool {
		if a.winner != b.winner {
			return a.winner
		}
		if a.out != b.out {
			return !a.out
		}
		if a.out {
			return false
		}
		if a.score != b.score {
			return a.score > b.score
		}
		if a.tie != b.tie {
			return a.tie > b.tie
		}
		if a.tie2 != b.tie2 {
			return a.tie2 > b.tie2
		}
		return a.tie3 > b.tie3
	}
	for i, x := range results {
		rank := 1
		for _, y := range results {
			if better(y, x) {
				rank++
			}
		}
		delta := -5 * (rank - 1)
		if x.winner {
			rank = 1
			delta = 20
		} else if r.Kind == "sanguosha" {
			rank = 2
			delta = -10
		} else {
			rank = max(2, rank)
			delta = min(-5, delta)
		}
		record.Players[i].Rank = rank
		record.Players[i].RatingDelta = delta
	}
}

func saveRatings(tx *sql.Tx, record MatchRecord) error {
	if !record.Rated {
		return nil
	}
	for _, p := range record.Players {
		if _, err := tx.Exec("INSERT INTO rating_ledger(match_id,user_id,delta,rank) VALUES(?,?,?,?)", record.ID, p.ID, p.RatingDelta, p.Rank); err != nil {
			return err
		}
	}
	return nil
}

type Rating struct {
	Points int `json:"points"`
	Played int `json:"played"`
	Wins   int `json:"wins"`
}

func (s *Server) rating(id string) (Rating, error) {
	var v Rating
	err := s.db.QueryRow("SELECT ?+COALESCE(SUM(delta),0),COUNT(*),COALESCE(SUM(CASE WHEN delta>0 THEN 1 ELSE 0 END),0) FROM rating_ledger WHERE user_id=?", initialRating, id).Scan(&v.Points, &v.Played, &v.Wins)
	return v, err
}
func (s *Server) leaderboard(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	offset = max(0, offset)
	// Equal scores share a rank; name and id make pagination deterministic.
	rows, err := s.db.Query(`WITH scores AS (SELECT u.id,u.name,1000+SUM(l.delta) points,COUNT(*) played,SUM(CASE WHEN l.delta>0 THEN 1 ELSE 0 END) wins FROM users u JOIN rating_ledger l ON l.user_id=u.id GROUP BY u.id), ranked AS (SELECT *,RANK() OVER (ORDER BY points DESC) rank FROM scores) SELECT id,name,points,played,wins,rank FROM ranked ORDER BY points DESC,name,id LIMIT 51 OFFSET ?`, offset)
	if err != nil {
		fail(w, 500, "无法读取排行榜")
		return
	}
	type entry struct {
		User
		Rating
		Rank int `json:"rank"`
	}
	entries := []entry{}
	for rows.Next() {
		var e entry
		if err = rows.Scan(&e.ID, &e.Name, &e.Points, &e.Played, &e.Wins, &e.Rank); err != nil {
			break
		}
		entries = append(entries, e)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		fail(w, 500, "无法读取排行榜")
		return
	}
	mine, err := s.rating(u.ID)
	if err != nil {
		fail(w, 500, "无法读取积分")
		return
	}
	more := len(entries) > 50
	if more {
		entries = entries[:50]
	}
	respond(w, 200, map[string]any{"players": entries, "self": mine, "hasMore": more, "offset": offset})
}
