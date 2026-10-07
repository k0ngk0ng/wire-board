package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

type MatchPlayer struct {
	Rank        int `json:"rank,omitempty"`
	RatingDelta int `json:"ratingDelta,omitempty"`
	User
	Bot   bool `json:"bot,omitempty"`
	Score *int `json:"score,omitempty"`
	Won   bool `json:"won"`
}
type MatchRecord struct {
	CatanExpansionRules map[string]string    `json:"catanExpansionRules,omitempty"`
	CatanExpansions     []string             `json:"catanExpansions,omitempty"`
	CatanLayout         string               `json:"catanLayout,omitempty"`
	CatanRules          string               `json:"catanRules,omitempty"`
	CatanScenario       string               `json:"catanScenario,omitempty"`
	CatanOptions        game.CatanOptions    `json:"catanOptions,omitempty"`
	SplendorOptions     game.SplendorOptions `json:"splendorOptions,omitempty"`
	SanguoshaOptions    game.SGOptions       `json:"sanguoshaOptions,omitempty"`
	RailMap             string               `json:"railMap,omitempty"`
	Rated               bool                 `json:"rated"`
	ID                  string               `json:"id"`
	Room                string               `json:"room"`
	Kind                string               `json:"kind"`
	Status              string               `json:"status"`
	Ended               int64                `json:"ended"`
	Players             []MatchPlayer        `json:"players"`
}

func archiveGame(tx *sql.Tx, r *Room) error {
	if r.Game == nil || (r.Status != "finished" && r.Status != "closed") || len(r.Seats) == 0 {
		return nil
	}
	id := r.MatchID
	if id == "" {
		id = fmt.Sprintf("%s:%d", r.ID, r.SetupVersion)
	}
	record := MatchRecord{ID: id, Room: r.Name, Kind: r.Kind, RailMap: r.RailMap, SanguoshaOptions: r.SanguoshaOptions, SplendorOptions: r.SplendorOptions, CatanOptions: r.CatanOptions, Status: r.Status, Ended: r.Updated, Players: []MatchPlayer{}}
	if r.Game.Splendor != nil {
		record.SplendorOptions = r.Game.Splendor.Options
	}
	if r.Game.Catan != nil && r.Game.Catan.Seafarers != nil {
		record.CatanScenario = r.Game.Catan.Seafarers.Scenario
		record.CatanLayout = r.Game.Catan.Seafarers.Layout
		record.CatanRules = r.Game.Catan.Seafarers.Rules
	} else if r.Game != nil && r.Game.Catan != nil && r.Game.Catan.BaseSetup != nil {
		record.CatanLayout = r.Game.Catan.BaseSetup.Layout
		record.CatanRules = r.Game.Catan.BaseSetup.Rules
	}
	if g := r.Game.Catan; g != nil {
		record.CatanOptions = g.Options
		if g.Explorer != nil {
			record.CatanOptions.FiveSix = g.Paired != nil
			board := g.Explorer.Board
			record.CatanScenario, record.CatanLayout, record.CatanRules = board.Scenario, board.Layout, board.Rules
			record.CatanExpansions = append(record.CatanExpansions, "explorers_pirates")
			record.CatanExpansionRules = map[string]string{"explorers_pirates": board.Rules}
		}
		if g.Two != nil {
			record.CatanExpansions = append(record.CatanExpansions, "two_player")
			record.CatanExpansionRules = map[string]string{"two_player": g.Two.Rules}
			record.CatanRules, record.CatanLayout = g.Two.Rules, "variable"
		}
		if g.Rivers != nil {
			record.CatanScenario = "rivers"
			record.CatanLayout = "variable"
			record.CatanExpansions = append(record.CatanExpansions, "rivers")
			if record.CatanExpansionRules == nil {
				record.CatanExpansionRules = map[string]string{}
			}
			rules := g.Rivers.Rules
			if rules == "" {
				rules = game.CatanRiversRules
			}
			record.CatanExpansionRules["rivers"] = rules
			if g.Two == nil {
				record.CatanRules = rules
			}
		}
		if g.Caravans != nil {
			record.CatanScenario = "caravans"
			record.CatanLayout = "variable"
			record.CatanExpansions = append(record.CatanExpansions, "caravans")
			if record.CatanExpansionRules == nil {
				record.CatanExpansionRules = map[string]string{}
			}
			rules := g.Caravans.Rules
			if rules == "" {
				rules = game.CatanCaravansRules
			}
			record.CatanExpansionRules["caravans"] = rules
			if g.Two == nil {
				record.CatanRules = rules
			}
		}
		if g.Transport != nil {
			record.CatanScenario, record.CatanLayout = "transport", "variable"
			record.CatanExpansions = append(record.CatanExpansions, "transport")
			if record.CatanExpansionRules == nil {
				record.CatanExpansionRules = map[string]string{}
			}
			record.CatanExpansionRules["transport"] = g.Transport.Map.Rules
			if g.Two == nil {
				record.CatanRules = g.Transport.Map.Rules
			}
		}
		if g.Attack != nil {
			record.CatanScenario, record.CatanLayout, record.CatanRules = "barbarian-attack", "variable", g.Attack.Rules
			record.CatanExpansions = append(record.CatanExpansions, "barbarian_attack")
			if record.CatanExpansionRules == nil {
				record.CatanExpansionRules = map[string]string{}
			}
			record.CatanExpansionRules["barbarian_attack"] = g.Attack.Rules
		}
		if g.Fishing != nil {
			record.CatanExpansions = append(record.CatanExpansions, "fishing")
			if record.CatanExpansionRules == nil {
				record.CatanExpansionRules = map[string]string{}
			}
			record.CatanExpansionRules["fishing"] = game.CatanFishingRules
			if g.Seafarers == nil {
				record.CatanScenario, record.CatanLayout, record.CatanRules = "fishing", "variable", game.CatanFishingRules
			}
		}
		if g.FriendlyRobber != nil {
			record.CatanExpansions = append(record.CatanExpansions, "friendly_robber")
			if record.CatanExpansionRules == nil {
				record.CatanExpansionRules = map[string]string{}
			}
			record.CatanExpansionRules["friendly_robber"] = g.FriendlyRobber.Rules
		}
		if g.Harbors != nil {
			record.CatanExpansions = append(record.CatanExpansions, "harbors")
			if record.CatanExpansionRules == nil {
				record.CatanExpansionRules = map[string]string{}
			}
			record.CatanExpansionRules["harbors"] = g.Harbors.Rules
		}
		if g.Seafarers != nil {
			record.CatanExpansions = append(record.CatanExpansions, "seafarers")
			if record.CatanExpansionRules == nil {
				record.CatanExpansionRules = map[string]string{}
			}
			record.CatanExpansionRules["seafarers"] = g.Seafarers.Rules
		}
		if g.CitiesKnights != nil {
			record.CatanExpansions = append(record.CatanExpansions, "cities_knights")
			setup := g.CitiesKnightsSetup()
			if record.CatanExpansionRules == nil {
				record.CatanExpansionRules = map[string]string{}
			}
			record.CatanExpansionRules["cities_knights"] = setup.Rules
			if g.Seafarers == nil {
				record.CatanRules = setup.Rules
				record.CatanLayout = setup.Layout
			}
		}
	}
	for i, seat := range r.Seats {
		p := MatchPlayer{User: seat.User, Bot: seat.Bot}
		if r.Status == "finished" {
			score := 0
			if r.Game.Splendor != nil {
				score = r.Game.Splendor.Players[i].Score
			} else if r.Game.Carcassonne != nil {
				score = r.Game.Carcassonne.Players[i].Score
			} else if r.Game.Catan != nil {
				score = r.Game.Catan.Players[i].Score
			} else if r.Game.Rail != nil {
				score = r.Game.Rail.Players[i].Score
			}
			if r.Game.Sanguosha == nil && r.Game.Dota == nil {
				p.Score = &score
			}
			for _, winner := range r.Game.Winners {
				if winner == i {
					p.Won = true
				}
			}
		}
		record.Players = append(record.Players, p)
	}
	rateMatch(r, &record)
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	result, err := tx.Exec("INSERT OR IGNORE INTO match_history(id,ended,snapshot) VALUES(?,?,?)", id, r.Updated, raw)
	if err != nil {
		return err
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if inserted == 0 {
		return nil
	}
	if err = saveRatings(tx, record); err != nil {
		return err
	}
	for _, p := range record.Players {
		if !p.Bot {
			if _, err = tx.Exec("INSERT OR IGNORE INTO match_members(match_id,user_id) VALUES(?,?)", id, p.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Server) profile(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	viewer, ok := s.needUser(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	var user User
	if err := s.db.QueryRow("SELECT id,name FROM users WHERE id=?", id).Scan(&user.ID, &user.Name); err != nil {
		fail(w, 404, "玩家不存在")
		return
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.Query("SELECT h.snapshot FROM match_history h JOIN match_members m ON m.match_id=h.id WHERE m.user_id=? ORDER BY h.ended DESC,h.id DESC", id)
	if err != nil {
		fail(w, 500, "无法读取战绩")
		return
	}
	records := []MatchRecord{}
	stats := map[string]map[string]int{"dota": {"played": 0, "wins": 0}, "sanguosha": {"played": 0, "wins": 0}, "splendor": {"played": 0, "wins": 0}, "rail": {"played": 0, "wins": 0}, "catan": {"played": 0, "wins": 0}, "carcassonne": {"played": 0, "wins": 0}}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			break
		}
		var match MatchRecord
		if err = json.Unmarshal(raw, &match); err != nil {
			break
		}
		if match.Status == "finished" {
			for _, p := range match.Players {
				if p.ID == id {
					stats[match.Kind]["played"]++
					if p.Won {
						stats[match.Kind]["wins"]++
					}
				}
			}
		}
		if match.Status == "finished" || viewer.ID == id {
			records = append(records, match)
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		fail(w, 500, "无法读取战绩")
		return
	}
	total := len(records)
	start := min(offset, total)
	end := min(start+20, total)
	relationship := "self"
	if viewer.ID != id {
		relationship = s.relationship(viewer.ID, id)
	}
	rating, err := s.rating(id)
	if err != nil {
		fail(w, 500, "无法读取积分")
		return
	}
	respond(w, 200, map[string]any{"rating": rating, "user": user, "stats": stats, "history": records[start:end], "total": total, "offset": start, "hasMore": end < total, "relationship": relationship})
}
func friendPair(a, b string) (string, string) {
	if a > b {
		return b, a
	}
	return a, b
}
func (s *Server) relationship(viewer, target string) string {
	a, b := friendPair(viewer, target)
	var status, requester string
	if s.db.QueryRow("SELECT status,requester FROM friendships WHERE a=? AND b=?", a, b).Scan(&status, &requester) != nil {
		return "none"
	}
	if status == "accepted" {
		return "friends"
	}
	if requester == viewer {
		return "outgoing"
	}
	return "incoming"
}
func (s *Server) friendship(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	target := r.PathValue("id")
	var found string
	if target == u.ID {
		fail(w, 400, "不能添加自己")
		return
	}
	if s.db.QueryRow("SELECT id FROM users WHERE id=?", target).Scan(&found) != nil {
		fail(w, 404, "玩家不存在")
		return
	}
	var req struct {
		Action string `json:"action"`
	}
	if !decode(w, r, &req) {
		return
	}
	relation := s.relationship(u.ID, target)
	a, b := friendPair(u.ID, target)
	var err error
	switch req.Action {
	case "request":
		if relation != "none" {
			fail(w, 409, "已有好友关系或申请，请刷新")
			return
		}
		_, err = s.db.Exec("INSERT INTO friendships(a,b,requester,status) VALUES(?,?,?,'pending')", a, b, u.ID)
	case "accept":
		if relation != "incoming" {
			fail(w, 403, "只能接受发给自己的申请")
			return
		}
		_, err = s.db.Exec("UPDATE friendships SET status='accepted' WHERE a=? AND b=?", a, b)
	case "remove":
		_, err = s.db.Exec("DELETE FROM friendships WHERE a=? AND b=?", a, b)
	default:
		fail(w, 400, "未知好友操作")
		return
	}
	if err != nil {
		fail(w, 500, "无法保存好友关系")
		return
	}
	s.broadcast()
	respond(w, 200, map[string]bool{"ok": true})
}
func (s *Server) friends(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	rows, err := s.db.Query(`SELECT u.id,u.name,f.status,f.requester FROM friendships f JOIN users u ON u.id=CASE WHEN f.a=? THEN f.b ELSE f.a END WHERE f.a=? OR f.b=?`, u.ID, u.ID, u.ID)
	if err != nil {
		fail(w, 500, "无法读取好友")
		return
	}
	list := []map[string]any{}
	for rows.Next() {
		var id, name, status, requester string
		if err = rows.Scan(&id, &name, &status, &requester); err != nil {
			break
		}
		relation := "friends"
		if status == "pending" {
			relation = "incoming"
			if requester == u.ID {
				relation = "outgoing"
			}
		}
		list = append(list, map[string]any{"id": id, "name": name, "relationship": relation})
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		fail(w, 500, "无法读取好友")
		return
	}
	sort.Slice(list, func(i, j int) bool { return list[i]["name"].(string) < list[j]["name"].(string) })
	respond(w, 200, map[string]any{"friends": list})
}
func (s *Server) players(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.needUser(w, r)
	if !ok {
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(query) > 100 {
		fail(w, 400, "搜索内容过长")
		return
	}
	rows, err := s.db.Query("SELECT id,name FROM users WHERE instr(lower(name),lower(?))>0 ORDER BY name LIMIT 40", query)
	if err != nil {
		fail(w, 500, "无法搜索玩家")
		return
	}
	list := []User{}
	for rows.Next() {
		var u User
		if err = rows.Scan(&u.ID, &u.Name); err != nil {
			break
		}
		list = append(list, u)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		fail(w, 500, "无法搜索玩家")
		return
	}
	respond(w, 200, map[string]any{"players": list})
}
