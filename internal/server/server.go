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
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

type Config struct {
	DataDir, InviteCode, Origin string
	AssetsBaseURL               string
	SecureCookie                bool
}
type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Seat struct {
	User
	Ready bool `json:"ready"`
	Left  bool `json:"left"`
}
type Room struct {
	SetupVersion int         `json:"setupVersion,omitempty"`
	TurnDeadline int64       `json:"turnDeadline,omitempty"`
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Kind         string      `json:"kind"`
	Host         string      `json:"host"`
	Capacity     int         `json:"capacity"`
	Seats        []Seat      `json:"seats"`
	Version      int         `json:"version"`
	Status       string      `json:"status"`
	Password     string      `json:"password,omitempty"`
	Game         *game.State `json:"game,omitempty"`
	Updated      int64       `json:"updated"`
}

const turnLimit = 120 * time.Second

func (r *Room) startTurnClock(now time.Time) {
	if r.Status == "playing" {
		r.TurnDeadline = now.Add(turnLimit).UnixMilli()
	} else {
		r.TurnDeadline = 0
	}
}

type bucket struct {
	At    time.Time
	Count int
}
type Server struct {
	cancel   context.CancelFunc
	done     chan struct{}
	mu       sync.Mutex
	db       *sql.DB
	cfg      Config
	rooms    map[string]*Room
	watchers map[chan struct{}]bool
	limits   map[string]bucket
	files    fs.FS
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
	_, e = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; CREATE TABLE IF NOT EXISTS users(id TEXT PRIMARY KEY,name TEXT UNIQUE NOT NULL,password TEXT NOT NULL); CREATE TABLE IF NOT EXISTS sessions(token TEXT PRIMARY KEY,user_id TEXT NOT NULL,expires INTEGER NOT NULL); CREATE TABLE IF NOT EXISTS rooms(id TEXT PRIMARY KEY,snapshot BLOB NOT NULL); CREATE TABLE IF NOT EXISTS actions(room_id TEXT,user_id TEXT,nonce TEXT,version INTEGER,action BLOB,created INTEGER,PRIMARY KEY(room_id,user_id,nonce));`)
	if e != nil {
		db.Close()
		return nil, e
	}
	s := &Server{db: db, cfg: cfg, rooms: map[string]*Room{}, watchers: map[chan struct{}]bool{}, limits: map[string]bucket{}, files: files}
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
		if changed {
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
			s.expireSetups(now)
			s.mu.Unlock()
		}
	}
}

// Called with s.mu held. No browser needs to be connected for setup to finish.
func (s *Server) expireSetups(now time.Time) {
	for id, room := range s.rooms {
		if room.Status != "playing" || room.Game.Rail == nil || !room.Game.Rail.Setup || room.TurnDeadline == 0 || room.TurnDeadline > now.UnixMilli() {
			continue
		}
		b, _ := json.Marshal(room)
		var next Room
		_ = json.Unmarshal(b, &next)
		next.Game.AutoChooseRailSetup()
		next.startTurnClock(now)
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
	mux.HandleFunc("GET /api/catalog", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, game.MapData()) })
	mux.HandleFunc("POST /api/rooms", s.create)
	mux.HandleFunc("POST /api/rooms/{id}", s.command)
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
	e = s.db.QueryRow("SELECT users.id,users.name FROM sessions JOIN users ON users.id=sessions.user_id WHERE token=? AND expires>?", digest(c.Value), time.Now().Unix()).Scan(&u.ID, &u.Name)
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
	for _, c := range req.Name {
		if !unicode.IsLetter(c) && !unicode.IsNumber(c) && c != '_' && c != '-' {
			fail(w, 400, "昵称只允许文字、数字、下划线与短横线")
			return
		}
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
		e := s.db.QueryRow("SELECT id,name,password FROM users WHERE name=?", req.Name).Scan(&u.ID, &u.Name, &hash)
		if e != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
			fail(w, 401, "昵称或密码不正确")
			return
		}
	}
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
	if c, e := r.Cookie("wb_session"); e == nil {
		_, _ = s.db.Exec("DELETE FROM sessions WHERE token=?", digest(c.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: "wb_session", Value: "", Path: "/", HttpOnly: true, Secure: s.cfg.SecureCookie, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	respond(w, 200, map[string]bool{"ok": true})
	s.mu.Lock()
	s.broadcast()
	s.mu.Unlock()
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
	return map[string]any{"id": r.ID, "name": r.Name, "kind": r.Kind, "host": r.Host, "capacity": r.Capacity, "seats": r.Seats, "status": r.Status, "locked": r.Password != "", "version": r.Version, "updated": r.Updated}
}
func (s *Server) state(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	rooms := []any{}
	for _, room := range s.rooms {
		rooms = append(rooms, summary(room))
	}
	out := map[string]any{"user": u, "rooms": rooms, "serverNow": time.Now().UnixMilli(), "assetsBaseURL": s.cfg.AssetsBaseURL}
	if room := s.current(u.ID); room != nil {
		v := summary(room)
		v["you"] = seatIndex(room, u.ID)
		v["turnDeadline"] = room.TurnDeadline
		if room.Game != nil {
			v["game"] = room.Game.View(seatIndex(room, u.ID))
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
	_, e = s.db.Exec("INSERT INTO rooms(id,snapshot) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET snapshot=excluded.snapshot", r.ID, b)
	return e
}
func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	if s.current(u.ID) != nil {
		fail(w, 409, "请先离开当前房间")
		return
	}
	var req struct {
		Name     string `json:"name"`
		Kind     string `json:"kind"`
		Capacity int    `json:"capacity"`
		Password string `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || utf8.RuneCountInString(req.Name) > 40 || len(req.Password) > 72 {
		fail(w, 400, "房间名需 1–40 字，密码最多 72 字节")
		return
	}
	maxPlayers := 4
	if req.Kind == "rail" {
		maxPlayers = 5
	} else if req.Kind != "splendor" {
		fail(w, 400, "未知游戏")
		return
	}
	if req.Capacity < 2 || req.Capacity > maxPlayers {
		fail(w, 400, "人数不符合游戏要求")
		return
	}
	hash := ""
	if req.Password != "" {
		h, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		hash = string(h)
	}
	room := &Room{ID: randomID(4), Name: req.Name, Kind: req.Kind, Host: u.ID, Capacity: req.Capacity, Seats: []Seat{{User: u}}, Version: 1, Status: "waiting", Password: hash, Updated: time.Now().Unix()}
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
		Type     string      `json:"type"`
		Target   string      `json:"target,omitempty"`
		Password string      `json:"password"`
		Version  int         `json:"version"`
		Nonce    string      `json:"nonce"`
		Action   game.Action `json:"action"`
	}
	if !decode(w, r, &req) {
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
	if req.Version != room.Version && !parallelSetup {
		fail(w, 409, "局面已更新，请根据最新画面重试")
		return
	}
	b, _ := json.Marshal(room)
	var next Room
	_ = json.Unmarshal(b, &next)
	idx := seatIndex(&next, u.ID)
	now := time.Now()
	var err error
	switch req.Type {
	case "join":
		if idx >= 0 {
			break
		}
		if s.current(u.ID) != nil {
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
			if !seat.Left {
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
			next.Seats[i].Ready = false
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
			next.Game, err = game.New(next.Kind, len(next.Seats))
			if err == nil {
				next.Status = "playing"
				next.SetupVersion = next.Version + 1
				next.startTurnClock(now)
			}
		}
	case "action":
		if idx < 0 || next.Status != "playing" {
			err = errors.New("无法执行游戏行动")
			break
		}
		turn, round := next.Game.Turn, next.Game.Round
		setup := next.Game.Rail != nil && next.Game.Rail.Setup
		err = next.Game.Apply(idx, req.Action)
		if err == nil && next.Game.Finished {
			next.Status = "finished"
		}
		if err == nil && (next.Game.Turn != turn || next.Game.Round != round || next.Game.Finished || (setup && !next.Game.Rail.Setup)) {
			next.startTurnClock(now)
		}
	case "kick_timeout":
		if idx < 0 || next.Status != "playing" || idx == next.Game.Turn || (next.Game.Rail != nil && next.Game.Rail.Setup) {
			err = errors.New("只有同局的其他玩家可以移出超时玩家")
			break
		}
		target := next.Game.Turn
		if req.Target != next.Seats[target].ID || next.TurnDeadline == 0 || now.UnixMilli() < next.TurnDeadline {
			err = errors.New("该玩家尚未超时，或当前回合已改变")
			break
		}
		if next.Kind == "splendor" {
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
				if !seat.Left {
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
		for i := range next.Seats {
			next.Seats[i].Ready = false
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
	next.Updated = time.Now().Unix()
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
	if _, ok := s.needUser(w, r); !ok {
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
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.watchers, ch); s.mu.Unlock() }()
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
		}
	}
}
func (s *Server) String() string { return fmt.Sprintf("wire-board (%d rooms)", len(s.rooms)) }
