package server

import (
	"database/sql"
	"net/http"
)

type gameBaseConfig struct {
	AutoFullscreen bool `json:"autoFullscreen"`
}

func (s *Server) initGameConfigs() error {
	if _, err := s.db.Exec("CREATE TABLE IF NOT EXISTS game_base_configs(kind TEXT PRIMARY KEY, auto_fullscreen INTEGER NOT NULL DEFAULT 0 CHECK(auto_fullscreen IN (0,1)))"); err != nil {
		return err
	}
	s.gameConfigs = make(map[string]gameBaseConfig, len(s.gameKinds))
	for _, kind := range s.gameKinds {
		s.gameConfigs[kind] = gameBaseConfig{}
	}
	rows, err := s.db.Query("SELECT kind,auto_fullscreen FROM game_base_configs")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var config gameBaseConfig
		if err := rows.Scan(&kind, &config.AutoFullscreen); err != nil {
			return err
		}
		if s.gameNames[kind] != "" {
			s.gameConfigs[kind] = config
		}
	}
	return rows.Err()
}

func (s *Server) adminGameConfig(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	actor, ok := s.needAdmin(w, r)
	if !ok {
		return
	}
	kind := r.PathValue("kind")
	if s.gameNames[kind] == "" {
		fail(w, 404, "游戏不存在")
		return
	}
	var request struct {
		AutoFullscreen *bool `json:"autoFullscreen"`
	}
	if !decode(w, r, &request) {
		return
	}
	if request.AutoFullscreen == nil {
		fail(w, 400, "缺少自动全屏设置")
		return
	}
	config := gameBaseConfig{AutoFullscreen: *request.AutoFullscreen}
	detail := s.gameNames[kind] + "：关闭开始后自动全屏"
	if config.AutoFullscreen {
		detail = s.gameNames[kind] + "：开启开始后自动全屏"
	}
	err := s.adminCommit(actor, "game_config", kind, detail, nil, func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO game_base_configs(kind,auto_fullscreen) VALUES(?,?) ON CONFLICT(kind) DO UPDATE SET auto_fullscreen=excluded.auto_fullscreen", kind, config.AutoFullscreen)
		return err
	}, s.clientIP(r))
	if err != nil {
		fail(w, 500, "游戏基础配置未保存，请重试")
		return
	}
	s.gameConfigs[kind] = config
	respond(w, 200, config)
}
