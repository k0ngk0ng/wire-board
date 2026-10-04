package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/coder/websocket"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"golang.org/x/crypto/bcrypt"
	"io/fs"
	"log"
	_ "modernc.org/sqlite"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type Config struct {
	DataDir, InviteCode, Origin                         string
	AssetsBaseURL                                       string
	SecureCookie                                        bool
	AdminUsername, AdminPassword, AdminExistingUsername string
}
type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Seat struct {
	User
	Bot      bool `json:"bot,omitempty"`
	AutoPlay bool `json:"autoPlay,omitempty"`
	Ready    bool `json:"ready"`
	Left     bool `json:"left"`
}
type Room struct {
	CatanNewWorldMap    *game.CatanNewWorldMap `json:"catanNewWorldMap,omitempty"`
	CatanTimeLeft       int64                  `json:"catanTimeLeft,omitempty"`
	CatanOptions        game.CatanOptions      `json:"catanOptions,omitempty"`
	SplendorOptions     game.SplendorOptions   `json:"splendorOptions,omitempty"`
	SanguoshaOptions    game.SGOptions         `json:"sanguoshaOptions,omitempty"`
	RailMap             string                 `json:"railMap,omitempty"`
	SGTimeLeft          int64                  `json:"sgTimeLeft,omitempty"`
	Rated               bool                   `json:"rated,omitempty"`
	CatanPendingVersion int                    `json:"catanPendingVersion,omitempty"`
	CatanTradeVersion   int                    `json:"catanTradeVersion,omitempty"`
	MatchID             string                 `json:"matchId,omitempty"`
	Spectators          []User                 `json:"spectators,omitempty"`
	Chat                []ChatMessage          `json:"chat,omitempty"`
	BotAt               int64                  `json:"botAt,omitempty"`
	SetupVersion        int                    `json:"setupVersion,omitempty"`
	TurnDeadline        int64                  `json:"turnDeadline,omitempty"`
	ID                  string                 `json:"id"`
	Name                string                 `json:"name"`
	Kind                string                 `json:"kind"`
	Host                string                 `json:"host"`
	Capacity            int                    `json:"capacity"`
	Seats               []Seat                 `json:"seats"`
	Version             int                    `json:"version"`
	Status              string                 `json:"status"`
	CloseReason         string                 `json:"closeReason,omitempty"`
	Password            string                 `json:"password,omitempty"`
	Game                *game.State            `json:"game,omitempty"`
	Updated             int64                  `json:"updated"`
	LastActive          int64                  `json:"lastActive"`
}

const turnLimit = 120 * time.Second

func (r *Room) startTurnClock(now time.Time) {
	if r.Status == "playing" {
		r.TurnDeadline = now.Add(turnLimit).UnixMilli()
		if r.Game != nil && r.Game.Sanguosha != nil {
			r.SGTimeLeft = turnLimit.Milliseconds()
			if q := r.Game.Sanguosha.Pending; q != nil && q.Kind != "general" && q.Kind != "heg_generals" {
				r.TurnDeadline = now.Add(20 * time.Second).UnixMilli()
			}
		}
	} else {
		r.TurnDeadline = 0
	}
}

type bucket struct {
	At    time.Time
	Count int
}
type Server struct {
	newGame     func(string, int) (*game.State, error)
	cancel      context.CancelFunc
	done        chan struct{}
	mu          sync.Mutex
	db          *sql.DB
	cfg         Config
	rooms       map[string]*Room
	watchers    map[chan struct{}]bool
	limits      map[string]bucket
	files       fs.FS
	hiddenGames map[string]bool
	presence    map[string]time.Time
	connections map[chan struct{}]string
}

func New(cfg Config, files fs.FS) (*Server, error) {
	if cfg.AssetsBaseURL != "" {
		u, err := url.Parse(cfg.AssetsBaseURL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.ContainsAny(cfg.AssetsBaseURL, "\r\n\"' ;") {
			return nil, errors.New("ASSETS_BASE_URL 必须是 HTTPS 素材目录地址")
		}
		cfg.AssetsBaseURL = strings.TrimRight(cfg.AssetsBaseURL, "/")
	}
	if cfg.InviteCode == "" {
		return nil, errors.New("请设置 INVITE_CODE 注册邀请码")
	}
	if cfg.Origin != "" {
		u, e := url.Parse(cfg.Origin)
		if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.Path != "" {
			return nil, errors.New("PUBLIC_ORIGIN 必须为完整源地址，例如 https://board.example.com，不带末尾斜杠")
		}
	}
	if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(cfg.DataDir, "wire-board.db")
	db, e := sql.Open("sqlite", path)
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	_, e = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; CREATE TABLE IF NOT EXISTS users(id TEXT PRIMARY KEY,name TEXT UNIQUE NOT NULL,password TEXT NOT NULL); CREATE TABLE IF NOT EXISTS sessions(token TEXT PRIMARY KEY,user_id TEXT NOT NULL,expires INTEGER NOT NULL); CREATE TABLE IF NOT EXISTS rooms(id TEXT PRIMARY KEY,snapshot BLOB NOT NULL); CREATE TABLE IF NOT EXISTS match_history(id TEXT PRIMARY KEY,ended INTEGER NOT NULL,snapshot BLOB NOT NULL); CREATE TABLE IF NOT EXISTS match_members(match_id TEXT,user_id TEXT,PRIMARY KEY(match_id,user_id)); CREATE INDEX IF NOT EXISTS match_members_user ON match_members(user_id); CREATE TABLE IF NOT EXISTS friendships(a TEXT,b TEXT,requester TEXT NOT NULL,status TEXT NOT NULL,PRIMARY KEY(a,b)); CREATE TABLE IF NOT EXISTS rating_ledger(match_id TEXT NOT NULL,user_id TEXT NOT NULL,delta INTEGER NOT NULL,rank INTEGER NOT NULL,PRIMARY KEY(match_id,user_id)); CREATE INDEX IF NOT EXISTS rating_user ON rating_ledger(user_id); CREATE TABLE IF NOT EXISTS actions(room_id TEXT,user_id TEXT,nonce TEXT,version INTEGER,action BLOB,created INTEGER,PRIMARY KEY(room_id,user_id,nonce));`)
	if e != nil {
		db.Close()
		return nil, e
	}
	s := &Server{newGame: game.New, db: db, cfg: cfg, rooms: map[string]*Room{}, watchers: map[chan struct{}]bool{}, limits: map[string]bucket{}, files: files, hiddenGames: map[string]bool{}, presence: map[string]time.Time{}, connections: map[chan struct{}]string{}}
	if e = s.initAdmin(); e != nil {
		db.Close()
		return nil, e
	}
	rows, e := db.Query("SELECT snapshot FROM rooms")
	if e != nil {
		db.Close()
		return nil, e
	}
	defer rows.Close()
	var migrated []*Room
	for rows.Next() {
		var b []byte
		if e = rows.Scan(&b); e != nil {
			db.Close()
			return nil, e
		}
		var r Room
		if e = json.Unmarshal(b, &r); e != nil {
			db.Close()
			return nil, e
		}
		s.rooms[r.ID] = &r
		changed := r.Game != nil && r.Game.UpgradeRailSetup()
		if changed {
			r.SetupVersion = r.Version
		}
		if r.Status == "playing" && r.TurnDeadline == 0 {
			r.startTurnClock(time.Now())
			changed = true
		}
		if r.LastActive == 0 {
			r.LastActive = r.Updated
			if r.LastActive == 0 {
				r.LastActive = time.Now().Unix()
			}
			changed = true
		}
		if changed || (r.Game != nil && (r.Status == "finished" || r.Status == "closed")) {
			migrated = append(migrated, &r)
		}
	}
	if e = rows.Err(); e != nil {
		db.Close()
		return nil, e
	}
	rows.Close()
	for _, r := range migrated {
		if e = s.save(r); e != nil {
			db.Close()
			return nil, e
		}
	}
	s.expireIdleRooms(time.Now())
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel, s.done = cancel, make(chan struct{})
	go s.runTimers(ctx)
	return s, nil
}
func (s *Server) Close() error {
	s.cancel()
	<-s.done
	return s.db.Close()
}

func (s *Server) runTimers(ctx context.Context) {
	defer close(s.done)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.mu.Lock()
			s.expireIdleRooms(now)
			s.expireSetups(now)
			s.runBots(now)
			s.mu.Unlock()
		}
	}
}

// Called with s.mu held. No browser needs to be connected for setup to finish.
func (s *Server) expireSetups(now time.Time) {
	for id, room := range s.rooms {
		if room.Status != "playing" || room.Game == nil || room.TurnDeadline == 0 || room.TurnDeadline > now.UnixMilli() {
			continue
		}
		railSetup := room.Game.Rail != nil && room.Game.Rail.Setup
		catanPending := room.Game.Catan != nil && (room.Game.Catan.SetupStep < room.Game.Catan.SetupLimit() || room.Game.Phase == "catan_discard" || room.Game.CatanPendingActor() >= 0)
		sgPending := room.Game.Sanguosha != nil
		if room.Game.Dota != nil {
			s.expireDota(room, now)
			continue
		}
		if !railSetup && !catanPending && !sgPending {
			continue
		}
		b, _ := json.Marshal(room)
		var next Room
		_ = json.Unmarshal(b, &next)
		if sgPending {
			failed := false
			// A shared nullification deadline expires for all outstanding responders
			// together, not one additional second per seat.
			for step := 0; step < len(next.Seats) && next.Status == "playing" && next.TurnDeadline <= now.UnixMilli(); step++ {
				a, err := next.Game.SanguoshaTimeoutAction()
				if err != nil {
					log.Printf("sanguosha timeout: %v", err)
					failed = true
					break
				}
				if err = next.applyGameAction(next.Game.SanguoshaActor(), a, now); err != nil {
					log.Printf("sanguosha timeout action: %v", err)
					failed = true
					break
				}
			}
			if failed {
				continue
			}
		} else if railSetup {
			next.Game.AutoChooseRailSetup()
		} else {
			next.Game.AutoCatanPending()
		}
		if next.Game.Finished {
			next.Status = "finished"
		}
		previousSetupStep := -1
		if room.Game.Catan != nil {
			previousSetupStep = room.Game.Catan.SetupStep
		}
		if !sgPending && !next.adjustCatanResponseClock(room.Game.Phase, room.Game.CatanPendingActor(), previousSetupStep, now) {
			next.startTurnClock(now)
		}
		if next.Game.Catan != nil && room.Game.Phase != "catan_discard" && next.Game.Phase == "catan_discard" {
			next.CatanPendingVersion = next.Version + 1
		}
		next.Version++
		next.Updated = now.Unix()
		if err := s.save(&next); err != nil {
			log.Printf("save automatic destination selection: %v", err)
			continue
		}
		s.rooms[id] = &next
		s.broadcast()
	}
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if e := s.db.PingContext(r.Context()); e != nil {
			http.Error(w, "unhealthy", 503)
			return
		}
		respond(w, 200, map[string]bool{"ok": true})
	})
	mux.HandleFunc("POST /api/register", s.auth)
	mux.HandleFunc("POST /api/login", s.auth)
	mux.HandleFunc("POST /api/logout", s.logout)
	mux.HandleFunc("GET /api/state", s.state)
	mux.HandleFunc("GET /api/admin", s.adminOverview)
	mux.HandleFunc("GET /api/admin/audit", s.adminAudit)
	mux.HandleFunc("POST /api/admin/games/{kind}", s.adminGame)
	mux.HandleFunc("POST /api/admin/users/{id}", s.adminManageUser)
	mux.HandleFunc("POST /api/admin/rooms/{id}/close", s.adminCloseRoom)
	mux.HandleFunc("GET /api/players", s.players)
	mux.HandleFunc("GET /api/players/{id}", s.profile)
	mux.HandleFunc("GET /api/friends", s.friends)
	mux.HandleFunc("GET /api/leaderboard", s.leaderboard)
	mux.HandleFunc("POST /api/players/{id}/friend", s.friendship)
	mux.HandleFunc("GET /api/catalog", func(w http.ResponseWriter, r *http.Request) {
		catalog, ok := game.RailCatalog(r.URL.Query().Get("map"))
		if !ok {
			fail(w, 400, "未知铁路地图")
			return
		}
		respond(w, 200, catalog)
	})
	mux.HandleFunc("POST /api/rooms", s.create)
	mux.HandleFunc("POST /api/rooms/{id}", s.command)
	mux.HandleFunc("POST /api/rooms/{id}/chat", s.chat)
	mux.HandleFunc("POST /api/rooms/{id}/watch", s.watch)
	mux.HandleFunc("GET /api/ws", s.socket)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { fail(w, 404, "接口不存在") })
	mux.Handle("/", http.FileServerFS(s.files))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		imageSource := ""
		if s.cfg.AssetsBaseURL != "" {
			u, _ := url.Parse(s.cfg.AssetsBaseURL)
			imageSource = " " + u.Scheme + "://" + u.Host
		}
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:"+imageSource+"; connect-src 'self'; font-src 'self'; media-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			if !s.originOK(r) {
				fail(w, 403, "请求来源不匹配")
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 32768)
		}
		mux.ServeHTTP(w, r)
	})
}
func (s *Server) originOK(r *http.Request) bool {
	o := r.Header.Get("Origin")
	if o == "" {
		return true
	}
	if s.cfg.Origin != "" {
		return o == s.cfg.Origin
	}
	u, e := url.Parse(o)
	return e == nil && u.Host == r.Host && (u.Scheme == "http" || u.Scheme == "https")
}
func randomID(n int) string {
	b := make([]byte, n)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func digest(t string) string { b := sha256.Sum256([]byte(t)); return hex.EncodeToString(b[:]) }
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, msg string) {
	respond(w, status, map[string]string{"error": msg})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		fail(w, 400, "请求格式不正确")
		return false
	}
	return true
}
func (s *Server) user(r *http.Request) (User, error) {
	var u User
	c, e := r.Cookie("wb_session")
	if e != nil {
		return u, e
	}
	e = s.db.QueryRow("SELECT users.id,users.name FROM sessions JOIN users ON users.id=sessions.user_id LEFT JOIN user_controls c ON c.user_id=users.id WHERE token=? AND expires>? AND COALESCE(c.disabled,0)=0", digest(c.Value), time.Now().Unix()).Scan(&u.ID, &u.Name)
	return u, e
}
func (s *Server) needUser(w http.ResponseWriter, r *http.Request) (User, bool) {
	u, e := s.user(r)
	if e != nil {
		fail(w, 401, "请先登录")
		return u, false
	}
	return u, true
}
func (s *Server) auth(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	now := time.Now()
	b := s.limits[host]
	if now.Sub(b.At) > time.Minute {
		b = bucket{At: now}
	}
	b.Count++
	s.limits[host] = b
	for k, v := range s.limits {
		if now.Sub(v.At) > 2*time.Minute {
			delete(s.limits, k)
		}
	}
	if b.Count > 20 {
		fail(w, 429, "尝试过于频繁，请一分钟后重试")
		return
	}
	var req struct {
		Name     string `json:"name"`
		Password string `json:"password"`
		Invite   string `json:"invite"`
	}
	if !decode(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if utf8.RuneCountInString(req.Name) < 2 || utf8.RuneCountInString(req.Name) > 20 || len(req.Password) < 8 || len(req.Password) > 72 {
		fail(w, 400, "昵称需 2–20 字，密码需 8–72 字节")
		return
	}
	if !validName(req.Name) {
		fail(w, 400, "昵称只允许文字、数字、下划线与短横线")
		return
	}
	var u User
	var hash string
	if r.URL.Path == "/api/register" {
		if digest(req.Invite) != digest(s.cfg.InviteCode) {
			fail(w, 403, "邀请码不正确")
			return
		}
		h, e := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if e != nil {
			fail(w, 500, "无法创建账号")
			return
		}
		u = User{randomID(12), req.Name}
		_, e = s.db.Exec("INSERT INTO users VALUES(?,?,?)", u.ID, u.Name, string(h))
		if e != nil {
			fail(w, 409, "昵称已被使用")
			return
		}
	} else {
		e := s.db.QueryRow("SELECT u.id,u.name,u.password FROM users u LEFT JOIN user_controls c ON c.user_id=u.id WHERE u.name=? AND COALESCE(c.disabled,0)=0", req.Name).Scan(&u.ID, &u.Name, &hash)
		if e != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
			fail(w, 401, "昵称或密码不正确")
			return
		}
	}
	if _, err := s.db.Exec(`INSERT INTO user_controls(user_id,created_at,last_login) VALUES(?,?,?) ON CONFLICT(user_id) DO UPDATE SET last_login=excluded.last_login`, u.ID, now.Unix(), now.Unix()); err != nil {
		fail(w, 500, "无法更新账号状态")
		return
	}
	s.seen(u.ID)
	token := randomID(32)
	expiry := time.Now().Add(30 * 24 * time.Hour)
	if _, e := s.db.Exec("INSERT INTO sessions VALUES(?,?,?)", digest(token), u.ID, expiry.Unix()); e != nil {
		fail(w, 500, "无法创建会话")
		return
	}
	_, _ = s.db.Exec("DELETE FROM sessions WHERE expires<?", now.Unix())
	http.SetCookie(w, &http.Cookie{Name: "wb_session", Value: token, Path: "/", HttpOnly: true, Secure: s.cfg.SecureCookie, SameSite: http.SameSiteStrictMode, Expires: expiry, MaxAge: 30 * 86400})
	respond(w, 200, u)
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u, err := s.user(r); err == nil {
		delete(s.presence, u.ID)
	}
	if c, e := r.Cookie("wb_session"); e == nil {
		_, _ = s.db.Exec("DELETE FROM sessions WHERE token=?", digest(c.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: "wb_session", Value: "", Path: "/", HttpOnly: true, Secure: s.cfg.SecureCookie, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	respond(w, 200, map[string]bool{"ok": true})
	s.broadcast()
}
func seatIndex(room *Room, id string) int {
	for i, p := range room.Seats {
		if p.ID == id && !p.Left {
			return i
		}
	}
	return -1
}
func (s *Server) current(id string) *Room {
	for _, r := range s.rooms {
		if seatIndex(r, id) >= 0 {
			return r
		}
	}
	return nil
}
func summary(r *Room) map[string]any {
	return map[string]any{"id": r.ID, "name": r.Name, "kind": r.Kind, "railMap": r.RailMap, "sanguoshaOptions": r.SanguoshaOptions, "splendorOptions": r.SplendorOptions, "catanOptions": r.CatanOptions, "catanNewWorldMap": r.CatanNewWorldMap, "host": r.Host, "capacity": r.Capacity, "seats": r.Seats, "status": r.Status, "closeReason": r.CloseReason, "locked": r.Password != "", "version": r.Version, "updated": r.Updated, "spectatorCount": len(r.Spectators)}
}
func (s *Server) state(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	s.seen(u.ID)
	rooms := []any{}
	for _, room := range s.rooms {
		if !s.hiddenGames[room.Kind] && (room.Status == "waiting" || room.Status == "playing") {
			rooms = append(rooms, summary(room))
		}
	}
	out := map[string]any{"user": map[string]any{"id": u.ID, "name": u.Name, "role": s.role(u.ID)}, "availableGames": s.availableGames(), "rooms": rooms, "serverNow": time.Now().UnixMilli(), "assetsBaseURL": s.cfg.AssetsBaseURL, "railMaps": game.RailMapList()}
	room := s.current(u.ID)
	if room == nil {
		room = s.watching(u.ID)
	}
	if room != nil {
		v := summary(room)
		v["you"] = seatIndex(room, u.ID)
		v["spectating"] = seatIndex(room, u.ID) < 0
		v["turnDeadline"] = room.TurnDeadline
		v["chat"] = room.Chat
		if room.Game != nil {
			v["game"] = room.Game.View(seatIndex(room, u.ID))
			if room.Status == "finished" && room.MatchID != "" {
				var raw []byte
				if err := s.db.QueryRow("SELECT snapshot FROM match_history WHERE id=?", room.MatchID).Scan(&raw); err == nil {
					var result MatchRecord
					if json.Unmarshal(raw, &result) == nil {
						v["result"] = result
					}
				}
			}
		}
		out["room"] = v
	}
	respond(w, 200, out)
}
func (s *Server) save(r *Room) error {
	b, e := json.Marshal(r)
	if e != nil {
		return e
	}
	tx, e := s.db.Begin()
	if e != nil {
		return e
	}
	_, e = tx.Exec("INSERT INTO rooms(id,snapshot) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET snapshot=excluded.snapshot", r.ID, b)
	if e == nil {
		e = archiveGame(tx, r)
	}
	if e != nil {
		_ = tx.Rollback()
		return e
	}
	return tx.Commit()
}
func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	if s.current(u.ID) != nil || s.watching(u.ID) != nil {
		fail(w, 409, "请先离开当前房间")
		return
	}
	var req struct {
		CatanOptions     game.CatanOptions    `json:"catanOptions"`
		SplendorOptions  game.SplendorOptions `json:"splendorOptions"`
		SanguoshaOptions game.SGOptions       `json:"sanguoshaOptions"`
		Name             string               `json:"name"`
		Kind             string               `json:"kind"`
		RailMap          string               `json:"railMap"`
		Capacity         int                  `json:"capacity"`
		Password         string               `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	if s.hiddenGames[req.Kind] {
		fail(w, 403, "该桌游已下架，暂时不能创建牌桌")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || utf8.RuneCountInString(req.Name) > 40 || len(req.Password) > 72 {
		fail(w, 400, "房间名需 1–40 字，密码最多 72 字节")
		return
	}
	maxPlayers := 4
	minPlayers := 2
	if req.Kind == "dota" {
		maxPlayers = 6
		if req.Capacity%2 != 0 {
			fail(w, 400, "兵线争锋需要 2、4 或 6 个席位")
			return
		}
	} else if req.Kind == "sanguosha" {
		o, err := game.NormalizeSGOptions(req.SanguoshaOptions)
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		req.SanguoshaOptions = o
		minPlayers = 4
		maxPlayers = 8
	} else if req.Kind == "rail" {
		info, ok := game.RailMapInfo(req.RailMap)
		if !ok {
			fail(w, 400, "未知铁路地图")
			return
		}
		req.RailMap = info.ID
		minPlayers, maxPlayers = info.MinPlayers, info.MaxPlayers
	} else if req.Kind == "carcassonne" {
		maxPlayers = 5
	} else if req.Kind == "catan" {
		options, err := game.NormalizeCatanOptions(req.CatanOptions)
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		req.CatanOptions = options
		minPlayers = 3
		if options.FiveSix {
			minPlayers, maxPlayers = 5, 6
		}
	} else if req.Kind == "splendor" {
		options, err := game.NormalizeSplendorOptions(req.SplendorOptions)
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		req.SplendorOptions = options
	} else {
		fail(w, 400, "未知游戏")
		return
	}
	if req.Capacity < minPlayers || req.Capacity > maxPlayers {
		fail(w, 400, "人数不符合游戏要求")
		return
	}
	hash := ""
	if req.Password != "" {
		h, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		hash = string(h)
	}
	room := &Room{ID: randomID(4), Name: req.Name, Kind: req.Kind, RailMap: req.RailMap, SanguoshaOptions: req.SanguoshaOptions, SplendorOptions: req.SplendorOptions, CatanOptions: req.CatanOptions, Host: u.ID, Capacity: req.Capacity, Seats: []Seat{{User: u}}, Version: 1, Status: "waiting", Password: hash, Updated: time.Now().Unix()}
	room.LastActive = room.Updated
	if e := s.save(room); e != nil {
		fail(w, 500, "无法保存房间")
		return
	}
	s.rooms[room.ID] = room
	s.broadcast()
	respond(w, 201, summary(room))
}
func (s *Server) command(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireIdleRooms(time.Now())
	s.expireSetups(time.Now())
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	room := s.rooms[r.PathValue("id")]
	if room == nil {
		fail(w, 404, "房间不存在")
		return
	}
	var req struct {
		CatanNewWorldMap *game.CatanNewWorldMap `json:"catanNewWorldMap"`
		CatanOptions     game.CatanOptions      `json:"catanOptions"`
		SplendorOptions  game.SplendorOptions   `json:"splendorOptions"`
		SanguoshaOptions game.SGOptions         `json:"sanguoshaOptions"`
		Type             string                 `json:"type"`
		RailMap          string                 `json:"railMap"`
		Target           string                 `json:"target,omitempty"`
		Password         string                 `json:"password"`
		Version          int                    `json:"version"`
		Nonce            string                 `json:"nonce"`
		Action           game.Action            `json:"action"`
		Enabled          *bool                  `json:"enabled,omitempty"`
	}
	if !decode(w, r, &req) {
		return
	}
	if s.hiddenGames[room.Kind] && (req.Type == "join" || req.Type == "start" || req.Type == "rematch") {
		fail(w, 403, "该桌游已下架，暂时不能加入或新开局")
		return
	}
	if len(req.Nonce) < 8 || len(req.Nonce) > 100 {
		fail(w, 400, "缺少有效操作编号")
		return
	}
	var prior int
	if e := s.db.QueryRow("SELECT version FROM actions WHERE room_id=? AND user_id=? AND nonce=?", room.ID, u.ID, req.Nonce).Scan(&prior); e == nil {
		respond(w, 200, map[string]int{"version": prior})
		return
	}
	parallelSetup := req.Type == "action" && req.Action.Type == "keep" && room.Status == "playing" && room.Game.Rail != nil && room.Game.Rail.Setup && room.SetupVersion > 0 && req.Version >= room.SetupVersion && req.Version <= room.Version
	parallelCatan := false
	if req.Type == "action" && room.Status == "playing" && room.Game.Catan != nil {
		g := room.Game.Catan
		parallelCatan = (req.Action.Type == "catan_discard" && room.Game.Phase == "catan_discard" && room.CatanPendingVersion > 0 && req.Version >= room.CatanPendingVersion && req.Version <= room.Version) || ((req.Action.Type == "catan_trade_accept" || req.Action.Type == "catan_trade_reject") && g.Trade != nil && req.Action.Offer == g.Trade.ID && room.CatanTradeVersion > 0 && req.Version >= room.CatanTradeVersion && req.Version <= room.Version)
	}
	parallelSG := req.Type == "action" && room.Game != nil && room.Game.Sanguosha != nil && room.Game.Sanguosha.Pending != nil && room.Game.Sanguosha.Pending.Kind == "nullification" && req.Action.Prompt == room.Game.Sanguosha.Pending.ID && req.Version <= room.Version
	parallelDota := req.Type == "action" && room.Status == "playing" && room.Game != nil && room.Game.Dota != nil && req.Action.Prompt == room.Game.Dota.Sequence && req.Version >= room.SetupVersion && req.Version <= room.Version
	// Control of one's own seat is independent of board versions. In particular,
	// a returning player must be able to cancel while the bot is taking actions.
	seatControl := req.Type == "autoplay" && req.Version >= max(1, room.SetupVersion) && req.Version <= room.Version
	if req.Version != room.Version && !parallelSetup && !parallelCatan && !parallelSG && !parallelDota && !seatControl {
		fail(w, 409, "局面已更新，请根据最新画面重试")
		return
	}
	b, _ := json.Marshal(room)
	var next Room
	_ = json.Unmarshal(b, &next)
	idx := seatIndex(&next, u.ID)
	if idx < 0 && req.Type != "join" {
		fail(w, 400, "只有牌桌玩家可以操作游戏")
		return
	}
	now := time.Now()
	var err error
	switch req.Type {
	case "join":
		if idx >= 0 {
			break
		}
		if s.current(u.ID) != nil || s.watching(u.ID) != nil {
			err = errors.New("请先离开当前房间")
			break
		}
		if next.Status != "waiting" || len(next.Seats) >= next.Capacity {
			err = errors.New("房间已开局或已满")
			break
		}
		if next.Password != "" && bcrypt.CompareHashAndPassword([]byte(next.Password), []byte(req.Password)) != nil {
			err = errors.New("房间密码不正确")
			break
		}
		next.Seats = append(next.Seats, Seat{User: u})
	case "leave":
		if idx < 0 {
			err = errors.New("你不在此房间")
			break
		}
		if next.Status == "playing" {
			err = errors.New("游戏进行中会保留座位；房主可以结束牌桌")
			break
		}
		if next.Game != nil {
			next.Seats[idx].Left = true
		} else {
			next.Seats = append(next.Seats[:idx], next.Seats[idx+1:]...)
		}
		active := 0
		for _, seat := range next.Seats {
			if !seat.Left && !seat.Bot {
				active++
				if next.Host == u.ID {
					next.Host = seat.ID
				}
			}
		}
		if active == 0 {
			next.Seats = nil
		}
		for i := range next.Seats {
			next.Seats[i].Ready = next.Seats[i].Bot
		}
	case "add_bot":
		if idx < 0 || next.Host != u.ID || next.Status != "waiting" || len(next.Seats) >= next.Capacity {
			err = errors.New("只有房主能在未满的等待房间添加电脑玩家")
			break
		}
		name := ""
		for number := 1; name == ""; number++ {
			candidate := fmt.Sprintf("电脑 %d", number)
			used := false
			for _, seat := range next.Seats {
				used = used || seat.Name == candidate
			}
			if !used {
				name = candidate
			}
		}
		next.Seats = append(next.Seats, Seat{User: User{ID: "bot-" + randomID(12), Name: name}, Bot: true, Ready: true})
	case "remove_bot":
		target := seatIndex(&next, req.Target)
		if idx < 0 || next.Host != u.ID || next.Status != "waiting" || target < 0 || !next.Seats[target].Bot {
			err = errors.New("只有房主能在开局前移除电脑玩家")
			break
		}
		next.Seats = append(next.Seats[:target], next.Seats[target+1:]...)
	case "catan_world_map", "catan_world_map_shuffle":
		// Drafts are provisioned internally until the complete scenario picker ships.
		if next.Host != u.ID || next.Kind != "catan" || next.Status != "waiting" || next.CatanNewWorldMap == nil {
			err = errors.New("只有房主能在新世界开局前调整地图")
			break
		}
		layout := req.CatanNewWorldMap
		if req.Type == "catan_world_map_shuffle" {
			layout, err = game.GenerateCatanNewWorldMap(max(3, next.Capacity))
		}
		if err == nil {
			err = game.ValidateCatanNewWorldMap(max(3, next.Capacity), layout)
		}
		if err == nil && !slices.Equal(next.CatanNewWorldMap.Hexes, layout.Hexes) {
			next.CatanNewWorldMap = layout
			for i := range next.Seats {
				next.Seats[i].Ready = next.Seats[i].Bot
			}
		}
	case "catan_options":
		if next.Host != u.ID || next.Kind != "catan" || next.Status != "waiting" {
			err = errors.New("只有房主能在开局前选择卡坦岛扩展")
			break
		}
		options, optionErr := game.NormalizeCatanOptions(req.CatanOptions)
		err = optionErr
		if err == nil && !options.FiveSix && len(next.Seats) > 4 {
			err = errors.New("基础版最多四人，请先移除多余座位")
		}
		if err == nil {
			if options.FiveSix {
				next.Capacity = max(5, next.Capacity)
			} else {
				next.Capacity = min(4, next.Capacity)
			}
			if next.CatanNewWorldMap != nil && options.FiveSix != next.CatanOptions.FiveSix {
				next.CatanNewWorldMap, err = game.GenerateCatanNewWorldMap(max(3, next.Capacity))
			}
			next.CatanOptions = options
		}
		if err == nil {
			for i := range next.Seats {
				next.Seats[i].Ready = next.Seats[i].Bot
			}
		}
	case "splendor_options":
		if next.Host != u.ID || next.Kind != "splendor" || next.Status != "waiting" {
			err = errors.New("只有房主能在开局前选择璀璨宝石扩展")
			break
		}
		next.SplendorOptions, err = game.NormalizeSplendorOptions(req.SplendorOptions)
		if err == nil {
			for i := range next.Seats {
				next.Seats[i].Ready = next.Seats[i].Bot
			}
		}
	case "sanguosha_options":
		if next.Host != u.ID || next.Kind != "sanguosha" || next.Status != "waiting" {
			err = errors.New("只有房主能在开局前选择三国杀规则")
			break
		}
		next.SanguoshaOptions, err = game.NormalizeSGOptions(req.SanguoshaOptions)
		if err == nil {
			for i := range next.Seats {
				next.Seats[i].Ready = next.Seats[i].Bot
			}
		}
	case "rail_map":
		info, ok := game.RailMapInfo(req.RailMap)
		if next.Host != u.ID || next.Kind != "rail" || next.Status != "waiting" {
			err = errors.New("只有房主能在开局前选择地图")
			break
		}
		if !ok {
			err = errors.New("未知铁路地图")
			break
		}
		if len(next.Seats) > info.MaxPlayers {
			err = fmt.Errorf("%s地图最多 %d 人，请先调整座位", info.Name, info.MaxPlayers)
			break
		}
		if next.RailMap != info.ID {
			previous, _ := game.RailMapInfo(next.RailMap)
			if next.Capacity == previous.MaxPlayers {
				next.Capacity = info.MaxPlayers
			} else {
				next.Capacity = min(next.Capacity, info.MaxPlayers)
			}
			next.RailMap = info.ID
			for i := range next.Seats {
				next.Seats[i].Ready = next.Seats[i].Bot
			}
		}
	case "ready":
		if idx < 0 || next.Status != "waiting" {
			err = errors.New("无法设置准备状态")
			break
		}
		next.Seats[idx].Ready = !next.Seats[idx].Ready
	case "start":
		if next.Host != u.ID || next.Status != "waiting" {
			err = errors.New("只有房主能开始游戏")
			break
		}
		for _, p := range next.Seats {
			if !p.Ready {
				err = errors.New("请等待所有玩家准备")
			}
		}
		if err == nil {
			if next.Kind == "rail" && next.RailMap != "" && next.RailMap != "usa" {
				next.Game, err = game.NewRailMap(next.RailMap, len(next.Seats))
			} else if next.Kind == "splendor" && (next.SplendorOptions.TradingPosts || next.SplendorOptions.Strongholds) {
				next.Game, err = game.NewSplendor(len(next.Seats), next.SplendorOptions)
			} else if next.Kind == "catan" {
				if next.CatanNewWorldMap != nil {
					next.Game, err = game.NewCatanNewWorldWithMap(len(next.Seats), next.CatanOptions, next.CatanNewWorldMap)
				} else {
					next.Game, err = game.NewCatan(len(next.Seats), next.CatanOptions)
				}
			} else if next.Kind == "sanguosha" {
				next.Game, err = game.NewSanguosha(len(next.Seats), next.SanguoshaOptions)
			} else {
				next.Game, err = s.newGame(next.Kind, len(next.Seats))
			}
			if err == nil {
				next.Status = "playing"
				if next.Game.Dota != nil {
					next.Game.Dota.Captain = idx
				}
				next.MatchID = randomID(12)
				next.Rated = true
				for _, seat := range next.Seats {
					if seat.Bot {
						next.Rated = false
					}
				}
				next.SetupVersion = next.Version + 1
				next.startTurnClock(now)
			}
		}
	case "autoplay":
		if next.Status != "playing" || next.Game == nil || next.Game.Finished || next.Seats[idx].Bot || req.Target != "" || req.Enabled == nil {
			err = errors.New("只能在对局中开启或取消自己的托管")
			break
		}
		if g := next.Game.Sanguosha; g != nil && g.Players[idx].Dead {
			err = errors.New("已阵亡的角色无需托管")
			break
		}
		if next.Seats[idx].AutoPlay != *req.Enabled {
			next.Seats[idx].AutoPlay = *req.Enabled
			message := "取消了托管，恢复手动操作"
			if *req.Enabled {
				message = "开启了托管，由电脑代为行动"
			}
			next.Game.Log = append(next.Game.Log, fmt.Sprintf("玩家 %d %s", idx+1, message))
			if len(next.Game.Log) > 80 {
				next.Game.Log = next.Game.Log[len(next.Game.Log)-80:]
			}
		}
	case "action":
		if idx < 0 || next.Status != "playing" {
			err = errors.New("无法执行游戏行动")
			break
		}
		if next.Seats[idx].AutoPlay {
			err = errors.New("当前由电脑托管，请先取消托管再手动操作")
			break
		}
		err = next.applyGameAction(idx, req.Action, now)
	case "kick_timeout":
		if next.Kind == "dota" {
			err = errors.New("兵线争锋超时由电脑接管，保留席位和队伍")
			break
		}
		if next.Kind == "sanguosha" {
			err = errors.New("三国杀超时由系统自动结束操作，不能移除身份角色")
			break
		}
		if idx < 0 || next.Status != "playing" || idx == next.Game.Turn || (next.Game.Rail != nil && next.Game.Rail.Setup) {
			err = errors.New("只有同局的其他玩家可以移出超时玩家")
			break
		}
		target := next.Game.Turn
		if next.Seats[target].AutoPlay {
			err = errors.New("该玩家正在由电脑托管，不能因超时移出")
			break
		}
		if req.Target != next.Seats[target].ID || next.TurnDeadline == 0 || now.UnixMilli() < next.TurnDeadline {
			err = errors.New("该玩家尚未超时，或当前回合已改变")
			break
		}
		if next.Kind == "carcassonne" {
			err = next.Game.EliminateCarcassonne(target)
		} else if next.Kind == "catan" {
			err = next.Game.EliminateCatan(target)
		} else if next.Kind == "splendor" {
			err = next.Game.EliminateSplendor(target)
		} else {
			err = next.Game.EliminateRail(target)
		}
		if err != nil {
			break
		}
		next.Seats[target].Left = true
		if next.Host == req.Target {
			for _, seat := range next.Seats {
				if !seat.Left && !seat.Bot {
					next.Host = seat.ID
					break
				}
			}
		}
		if next.Game.Finished {
			next.Status = "finished"
		}
		next.startTurnClock(now)
	case "rematch":
		if next.Host != u.ID || (next.Status != "finished" && next.Status != "closed") {
			err = errors.New("只有房主能在结束后再开一局")
			break
		}
		next.Game = nil
		next.TurnDeadline = 0
		active := []Seat{}
		for _, seat := range next.Seats {
			if !seat.Left {
				active = append(active, seat)
			}
		}
		next.Seats = active
		next.Status = "waiting"
		next.CloseReason = ""
		for i := range next.Seats {
			next.Seats[i].Ready = next.Seats[i].Bot
			next.Seats[i].AutoPlay = false
		}
	case "close":
		if next.Host != u.ID || next.Status != "playing" {
			err = errors.New("只有房主能结束正在进行的牌桌")
			break
		}
		next.Status = "closed"
		next.TurnDeadline = 0
	default:
		err = errors.New("未知房间操作")
	}
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	next.Version++
	next.BotAt = now.Add(900 * time.Millisecond).UnixMilli()
	next.Updated = time.Now().Unix()
	next.LastActive = next.Updated
	snapshot, _ := json.Marshal(next)
	req.Password = "" // Never persist plaintext room passwords in the action journal.
	action, _ := json.Marshal(req)
	tx, e := s.db.Begin()
	if e == nil {
		if len(next.Seats) == 0 {
			_, e = tx.Exec("DELETE FROM rooms WHERE id=?", next.ID)
		} else {
			_, e = tx.Exec("UPDATE rooms SET snapshot=? WHERE id=?", snapshot, next.ID)
		}
		if e == nil {
			e = archiveGame(tx, room)
		}
		if e == nil {
			e = archiveGame(tx, &next)
		}
		if e == nil {
			_, e = tx.Exec("INSERT INTO actions VALUES(?,?,?,?,?,?)", next.ID, u.ID, req.Nonce, next.Version, action, time.Now().Unix())
		}
		if e == nil {
			e = tx.Commit()
		} else {
			_ = tx.Rollback()
		}
	}
	if e != nil {
		log.Printf("save action: %v", e)
		fail(w, 500, "保存失败，操作未生效，请重试")
		return
	}
	if len(next.Seats) == 0 {
		delete(s.rooms, next.ID)
	} else {
		s.rooms[next.ID] = &next
	}
	s.broadcast()
	respond(w, 200, map[string]int{"version": next.Version})
}
func (s *Server) broadcast() {
	for c := range s.watchers {
		select {
		case c <- struct{}{}:
		default:
		}
	}
}
func (s *Server) socket(w http.ResponseWriter, r *http.Request) {
	if !s.originOK(r) {
		fail(w, 403, "请求来源不匹配")
		return
	}
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	conn, e := websocket.Accept(w, r, &websocket.AcceptOptions{})
	if e != nil {
		return
	}
	defer conn.CloseNow()
	ctx := conn.CloseRead(r.Context())
	ch := make(chan struct{}, 1)
	s.mu.Lock()
	s.watchers[ch] = true
	s.connections[ch] = u.ID
	s.seen(u.ID)
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.watchers, ch); delete(s.connections, ch); s.mu.Unlock() }()
	ch <- struct{}{}
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ch:
			if _, e := s.user(r); e != nil {
				_ = conn.Close(websocket.StatusPolicyViolation, "session expired")
				return
			}
			c, cancel := context.WithTimeout(ctx, 5*time.Second)
			e = conn.Write(c, websocket.MessageText, []byte(`{"type":"changed"}`))
			cancel()
			if e != nil {
				return
			}
		case <-ticker.C:
			if _, e := s.user(r); e != nil {
				return
			}
			c, cancel := context.WithTimeout(ctx, 5*time.Second)
			e = conn.Ping(c)
			cancel()
			if e != nil {
				return
			}
			s.mu.Lock()
			if _, err := s.user(r); err == nil {
				s.seen(u.ID)
			}
			s.mu.Unlock()
		}
	}
}
func (s *Server) String() string { return fmt.Sprintf("wire-board (%d rooms)", len(s.rooms)) }
