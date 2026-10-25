package server

import (
	"database/sql"
	"net/http"
	"slices"
	"strings"
)

func (s *Server) loadGameOrder() error {
	rows, err := s.db.Query("SELECT kind FROM game_order ORDER BY position,kind")
	if err != nil {
		return err
	}
	defer rows.Close()
	order := []string{}
	seen := map[string]bool{}
	for rows.Next() {
		var kind string
		if err = rows.Scan(&kind); err != nil {
			return err
		}
		if s.gameNames[kind] != "" && !seen[kind] {
			order = append(order, kind)
			seen[kind] = true
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for _, kind := range s.gameKinds {
		if !seen[kind] {
			order = append(order, kind)
		}
	}
	s.gameKinds = order
	return nil
}

func (s *Server) adminGameOrder(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	actor, ok := s.needAdmin(w, r)
	if !ok {
		return
	}
	var req struct {
		Order    []string `json:"order"`
		Expected []string `json:"expectedOrder"`
	}
	if !decode(w, r, &req) {
		return
	}
	if len(req.Order) != len(s.gameKinds) || len(req.Expected) != len(s.gameKinds) {
		fail(w, 400, "游戏列表不完整，请刷新后重新排序")
		return
	}
	seen := map[string]bool{}
	names := []string{}
	for _, kind := range req.Order {
		if s.gameNames[kind] == "" || seen[kind] {
			fail(w, 400, "游戏顺序包含无效或重复项目")
			return
		}
		seen[kind] = true
		names = append(names, s.gameNames[kind])
	}
	if !slices.Equal(req.Expected, s.gameKinds) {
		fail(w, 409, "游戏顺序已更新，请取消本次排序后重新调整")
		return
	}
	if slices.Equal(req.Order, s.gameKinds) {
		respond(w, 200, map[string]bool{"ok": true})
		return
	}
	err := s.adminCommit(actor, "game_order", "games", strings.Join(names, "、"), nil, func(tx *sql.Tx) error {
		if _, err := tx.Exec("DELETE FROM game_order"); err != nil {
			return err
		}
		for i, kind := range req.Order {
			if _, err := tx.Exec("INSERT INTO game_order(kind,position) VALUES(?,?)", kind, i); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		fail(w, 500, "游戏顺序未保存，请重试")
		return
	}
	s.gameKinds = slices.Clone(req.Order)
	respond(w, 200, map[string]bool{"ok": true})
}
