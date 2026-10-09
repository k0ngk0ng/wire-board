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
	Bot             bool `json:"bot,omitempty"`
	AutoPlay        bool `json:"autoPlay,omitempty"`
	TimeoutAutoPlay bool `json:"timeoutAutoPlay,omitempty"`
	Ready           bool `json:"ready"`
	Left            bool `json:"left"`
}
type Room struct {
	CatanRiversWorldMap    *game.CatanRiversWorldMap      `json:"catanRiversWorldMap,omitempty"`
	CatanEvents            string                         `json:"catanEvents,omitempty"`
	CatanFishing           bool                           `json:"catanFishing,omitempty"`
	CatanFishingLakes      bool                           `json:"catanFishingLakes,omitempty"`
	CatanTwoRules          string                         `json:"catanTwoRules,omitempty"`
	CatanTwoScenario       string                         `json:"catanTwoScenario,omitempty"`
	CatanScenario          string                         `json:"catanScenario,omitempty"`
	CatanFriendlyRobber    *game.CatanFriendlyRobberSetup `json:"catanFriendlyRobber,omitempty"`
	CatanHarbors           *game.CatanHarborsSetup        `json:"catanHarbors,omitempty"`
	CatanCitiesKnights     *game.CatanCitiesKnightsSetup  `json:"catanCitiesKnights,omitempty"`
	CatanBaseConfiguration *game.CatanBaseConfiguration   `json:"catanBaseConfiguration,omitempty"`
	CatanSeafarers         *game.CatanSeafarersSetup      `json:"catanSeafarers,omitempty"`
	CatanNewWorldMap       *game.CatanNewWorldMap         `json:"catanNewWorldMap,omitempty"`
	CatanTimeLeft          int64                          `json:"catanTimeLeft,omitempty"`
	CatanDiscardPaused     bool                           `json:"catanDiscardPaused,omitempty"`
	CatanOptions           game.CatanOptions              `json:"catanOptions,omitempty"`
	SplendorOptions        game.SplendorOptions           `json:"splendorOptions,omitempty"`
	SanguoshaOptions       game.SGOptions                 `json:"sanguoshaOptions,omitempty"`
	RailMap                string                         `json:"railMap,omitempty"`
	SGTimeLeft             int64                          `json:"sgTimeLeft,omitempty"`
	Rated                  bool                           `json:"rated,omitempty"`
	CatanPendingVersion    int                            `json:"catanPendingVersion,omitempty"`
	CatanTradeVersion      int                            `json:"catanTradeVersion,omitempty"`
	MatchID                string                         `json:"matchId,omitempty"`
	Spectators             []User                         `json:"spectators,omitempty"`
	Chat                   []ChatMessage                  `json:"chat,omitempty"`
	BotAt                  int64                          `json:"botAt,omitempty"`
	SetupVersion           int                            `json:"setupVersion,omitempty"`
	TurnDeadline           int64                          `json:"turnDeadline,omitempty"`
	ID                     string                         `json:"id"`
	Name                   string                         `json:"name"`
	Kind                   string                         `json:"kind"`
	Host                   string                         `json:"host"`
	Capacity               int                            `json:"capacity"`
	Seats                  []Seat                         `json:"seats"`
	Version                int                            `json:"version"`
	Status                 string                         `json:"status"`
	CloseReason            string                         `json:"closeReason,omitempty"`
	Password               string                         `json:"password,omitempty"`
	Game                   *game.State                    `json:"game,omitempty"`
	Updated                int64                          `json:"updated"`
	LastActive             int64                          `json:"lastActive"`
}

const turnLimit = 120 * time.Second

func (r *Room) startTurnClock(now time.Time) {
	if r.Status == "playing" && !r.Game.CatanExplorerSetupBlocked() {
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

// Called with s.mu held. Deadlines and takeovers do not need a connected browser.
func (s *Server) expireSetups(now time.Time) {
	for _, room := range s.rooms {
		if room.Status != "playing" || room.Game == nil || room.Game.Finished || room.TurnDeadline == 0 || room.TurnDeadline > now.UnixMilli() || room.Game.CatanExplorerSetupBlocked() {
			continue
		}
		s.expireToAutoplay(room, now)
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
	result := map[string]any{"catanRiversWorldMap": r.CatanRiversWorldMap, "id": r.ID, "name": r.Name, "kind": r.Kind, "railMap": r.RailMap, "sanguoshaOptions": r.SanguoshaOptions, "splendorOptions": r.SplendorOptions, "catanOptions": r.CatanOptions, "catanSeafarers": r.CatanSeafarers, "catanBaseConfiguration": r.CatanBaseConfiguration, "catanCitiesKnights": r.CatanCitiesKnights, "catanHarbors": r.CatanHarbors, "catanFriendlyRobber": r.CatanFriendlyRobber, "catanNewWorldMap": r.CatanNewWorldMap, "host": r.Host, "capacity": r.Capacity, "seats": r.Seats, "status": r.Status, "closeReason": r.CloseReason, "locked": r.Password != "", "version": r.Version, "updated": r.Updated, "spectatorCount": len(r.Spectators)}
	if r.CatanFishing {
		result["catanFishing"] = true
		if publicCatanExplorerScenario(r.CatanScenario) {
			result["catanFishingLakes"] = r.CatanFishingLakes
		}
	}
	if r.CatanEvents != "" {
		result["catanEvents"] = r.CatanEvents
	}
	if r.CatanScenario != "" {
		result["catanScenario"] = r.CatanScenario
	}
	if r.CatanTwoRules != "" {
		result["catanTwoRules"] = r.CatanTwoRules
		if r.CatanTwoScenario != "" {
			result["catanTwoScenario"] = r.CatanTwoScenario
		}
	}
	if r.Capacity > 4 && (r.publicCatanBaseAvailable() || (r.CatanBaseConfiguration != nil && r.Status == "waiting" && r.CatanSeafarers == nil && r.CatanNewWorldMap == nil)) {
		result["catanBaseLayouts"] = game.CatanBaseLayouts(max(3, r.Capacity))
	}
	if (r.CatanFriendlyRobber != nil || r.publicCatanFriendlyAvailable()) && r.Status == "waiting" {
		reason := ""
		if err := r.validateCatanFriendlyRobber(max(3, r.Capacity)); err != nil {
			reason = err.Error()
		}
		result["catanFriendlyRobberAvailability"] = map[string]any{"allowed": reason == "", "reason": reason, "minPlayers": r.catanFriendlyMinimumPlayers()}
	}
	if r.CatanSeafarers != nil && r.Status == "waiting" {
		choices := game.CatanSeafarersScenarios(max(3, r.Capacity))
		if r.twoCatanSeafarers() {
			choices = slices.DeleteFunc(game.CatanSeafarersScenarios(4), func(info game.CatanSeafarersScenario) bool { return !game.CatanTwoSeafarersScenario(info.ID) })
		}
		if publicCatanSeaScenario(r.CatanScenario) {
			choices = slices.DeleteFunc(choices, func(info game.CatanSeafarersScenario) bool {
				return !publicCatanSeaScenario(info.ID)
			})
		}
		if r.CatanFishing {
			choices = slices.DeleteFunc(choices, func(info game.CatanSeafarersScenario) bool {
				return !publicCatanFishingSea(info.ID) || (r.Capacity > 4 && !publicCatanFishingSeaExtended(info.ID))
			})
			for i := range choices {
				if choices[i].ID == "desert" || choices[i].ID == "tribe" {
					choices[i].Layouts = []string{"fixed"}
				}
			}
		}
		if r.CatanCitiesKnights != nil {
			choices = slices.DeleteFunc(choices, func(info game.CatanSeafarersScenario) bool {
				return !game.CatanCitiesKnightsSeafarersSupported(info.ID)
			})
			for i := range choices {
				choices[i].VictoryPoints += 2
			}
		}
		if r.CatanHarbors != nil && r.CatanHarbors.Enabled {
			for i := range choices {
				choices[i].VictoryPoints++
			}
		}
		if r.friendlyRobberEnabled() {
			choices = slices.DeleteFunc(choices, func(info game.CatanSeafarersScenario) bool {
				return !game.CatanFriendlySeafarersSupported(max(3, r.Capacity), info.ID)
			})
		}
		result["catanSeafarersChoices"] = choices
	}
	return result
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
		CatanEvents            string                         `json:"catanEvents"`
		CatanBaseConfiguration *game.CatanBaseConfiguration   `json:"catanBaseConfiguration"`
		CatanFriendlyRobber    *game.CatanFriendlyRobberSetup `json:"catanFriendlyRobber"`
		CatanHarbors           *game.CatanHarborsSetup        `json:"catanHarbors"`
		CatanFishing           bool                           `json:"catanFishing"`
		CatanFishingLakes      bool                           `json:"catanFishingLakes"`
		CatanCitiesKnights     *game.CatanCitiesKnightsSetup  `json:"catanCitiesKnights"`
		CatanOptions           game.CatanOptions              `json:"catanOptions"`
		CatanTwoScenario       string                         `json:"catanTwoScenario"`
		CatanScenario          string                         `json:"catanScenario"`
		SplendorOptions        game.SplendorOptions           `json:"splendorOptions"`
		SanguoshaOptions       game.SGOptions                 `json:"sanguoshaOptions"`
		Name                   string                         `json:"name"`
		Kind                   string                         `json:"kind"`
		RailMap                string                         `json:"railMap"`
		Capacity               int                            `json:"capacity"`
		Password               string                         `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	if s.hiddenGames[req.Kind] {
		fail(w, 403, "该桌游已下架，暂时不能创建牌桌")
		return
	}
	if req.CatanTwoScenario != "" && (req.Kind != "catan" || req.Capacity != 2 || req.CatanScenario != "") {
		fail(w, 400, "双人剧本需要两人卡坦牌桌")
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
		if publicCatanExplorerScenario(req.CatanScenario) && !validCatanExplorerOptions(options) {
			fail(w, 400, "探索者按实际人数启用扩充，请勿混用基础五六人选项")
			return
		}
		minPlayers = 3
		if publicCatanFlexibleScenario(req.CatanScenario) {
			minPlayers, maxPlayers = 2, 6
		} else if req.CatanScenario == "barbarian-attack" {
			maxPlayers = 6
		}
		if req.Capacity == 2 {
			if !game.CatanTwoHelpersOptions(req.CatanTwoScenario, options) && !(publicCatanExplorerScenario(req.CatanScenario) && validCatanExplorerOptions(options)) {
				fail(w, 400, "此双人剧本不支持所选扩展，助手目前可与基础版或渔夫同开")
				return
			}
			minPlayers, maxPlayers = 2, 2
		}
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
	room := &Room{CatanFishing: req.CatanFishing, ID: randomID(4), Name: req.Name, Kind: req.Kind, RailMap: req.RailMap, SanguoshaOptions: req.SanguoshaOptions, SplendorOptions: req.SplendorOptions, CatanOptions: req.CatanOptions, Host: u.ID, Capacity: req.Capacity, Seats: []Seat{{User: u}}, Version: 1, Status: "waiting", Password: hash, Updated: time.Now().Unix()}
	if room.Kind == "catan" && room.Capacity == 2 && !publicCatanFlexibleScenario(req.CatanScenario) {
		if err := room.setCatanTwoScenario(req.CatanTwoScenario); err != nil {
			fail(w, 400, err.Error())
			return
		}
	}
	if req.CatanScenario != "" {
		if err := room.setCatanScenario(req.CatanScenario); err != nil {
			fail(w, 400, err.Error())
			return
		}
	}
	if req.CatanCitiesKnights != nil {
		if err := room.setPublicCatanCombinationKnights(req.CatanCitiesKnights); err != nil {
			fail(w, 400, err.Error())
			return
		}
	}
	if req.CatanFishing {
		if err := room.setCatanFishing(true); err != nil {
			fail(w, 400, err.Error())
			return
		}
	}
	if req.CatanFishingLakes {
		if err := room.setCatanFishingLakes(true); err != nil {
			fail(w, 400, err.Error())
			return
		}
	}
	if req.CatanBaseConfiguration != nil {
		if !room.publicCatanBaseAvailable() {
			fail(w, 400, "基础布局选择需要基础卡坦岛地图")
			return
		}
		if err := room.setCatanBaseConfiguration(*req.CatanBaseConfiguration); err != nil {
			fail(w, 400, err.Error())
			return
		}
	}
	if req.CatanHarbors != nil {
		if !room.publicCatanHarborsAvailable() {
			fail(w, 400, "此地图或人数尚未接通港口霸主")
			return
		}
		if err := room.setCatanHarbors(*req.CatanHarbors); err != nil {
			fail(w, 400, err.Error())
			return
		}
		if err := room.validateCatanScenario(); err != nil {
			fail(w, 400, err.Error())
			return
		}
	}
	if req.CatanFriendlyRobber != nil {
		if !room.publicCatanFriendlyAvailable() {
			fail(w, 400, "此地图或人数尚未接通友善强盗")
			return
		}
		if err := room.setCatanFriendlyRobber(*req.CatanFriendlyRobber); err != nil {
			fail(w, 400, err.Error())
			return
		}
		if err := room.validateCatanScenario(); err != nil {
			fail(w, 400, err.Error())
			return
		}
	}
	room.CatanEvents = req.CatanEvents
	if err := room.validateCatanEvents(); err != nil {
		fail(w, 400, err.Error())
		return
	}
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
		CatanRiversWorldMap    *game.CatanRiversWorldMap      `json:"catanRiversWorldMap"`
		CatanFriendlyRobber    *game.CatanFriendlyRobberSetup `json:"catanFriendlyRobber"`
		CatanHarbors           *game.CatanHarborsSetup        `json:"catanHarbors"`
		CatanCitiesKnights     *game.CatanCitiesKnightsSetup  `json:"catanCitiesKnights"`
		CatanNewWorldMap       *game.CatanNewWorldMap         `json:"catanNewWorldMap"`
		CatanSeafarers         *game.CatanSeafarersSetup      `json:"catanSeafarers"`
		CatanBaseConfiguration *game.CatanBaseConfiguration   `json:"catanBaseConfiguration"`
		CatanOptions           game.CatanOptions              `json:"catanOptions"`
		CatanTwoScenario       *string                        `json:"catanTwoScenario"`
		CatanScenario          *string                        `json:"catanScenario"`
		SplendorOptions        game.SplendorOptions           `json:"splendorOptions"`
		SanguoshaOptions       game.SGOptions                 `json:"sanguoshaOptions"`
		Type                   string                         `json:"type"`
		RailMap                string                         `json:"railMap"`
		Target                 string                         `json:"target,omitempty"`
		Password               string                         `json:"password"`
		Version                int                            `json:"version"`
		Nonce                  string                         `json:"nonce"`
		Action                 game.Action                    `json:"action"`
		Enabled                *bool                          `json:"enabled,omitempty"`
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
	case "catan_explorer_reset":
		if next.Host != u.ID || next.Status != "playing" || next.Game == nil || next.Seats[idx].Left {
			err = errors.New("只有房主能重新布置卡住的开局")
			break
		}
		err = next.Game.ResetCatanExplorerSetup()
		if err == nil {
			next.CatanTimeLeft, next.CatanPendingVersion, next.CatanTradeVersion = 0, 0, 0
			next.CatanDiscardPaused = false
			next.startTurnClock(now)
		}
	case "catan_friendly_robber":
		if next.Host != u.ID || req.CatanFriendlyRobber == nil || ((next.CatanFriendlyRobber == nil || req.CatanFriendlyRobber.Enabled) && !next.publicCatanFriendlyAvailable()) {
			err = errors.New("只有房主能在支持的卡坦等待房间调整友善强盗")
			break
		}
		err = next.setCatanFriendlyRobber(*req.CatanFriendlyRobber)
	case "catan_harbors":
		if next.Host != u.ID || req.CatanHarbors == nil || ((next.CatanHarbors == nil || req.CatanHarbors.Enabled) && !next.publicCatanHarborsAvailable()) {
			err = errors.New("只有房主能在支持的卡坦等待房间调整港口霸主")
			break
		}
		err = next.setCatanHarbors(*req.CatanHarbors)
	case "catan_events":
		if next.Host != u.ID || req.Enabled == nil {
			err = errors.New("只有房主能在开局前切换事件牌")
			break
		}
		err = next.setCatanEvents(*req.Enabled)
	case "catan_cities_knights":
		if next.Host == u.ID && (publicCatanSeaScenario(next.CatanScenario) || next.CatanScenario == "fishing" || publicCatanExplorerScenario(next.CatanScenario) || next.twoCatanSeafarers() || next.catanCaravanRecipe() || next.catanRiverRecipe() || next.CatanScenario == "transport" || (publicCatanTradersCombination(next.CatanScenario)) || next.catanAttackRecipe()) {
			err = next.setPublicCatanCombinationKnights(req.CatanCitiesKnights)
			break
		}
		if next.Host != u.ID || next.CatanCitiesKnights == nil || req.CatanCitiesKnights == nil {
			err = errors.New("只有房主能在已启用城市与骑士的房间调整设置")
			break
		}
		err = next.setCatanCitiesKnights(*req.CatanCitiesKnights)
	case "catan_base_configuration":
		if next.Host != u.ID || (next.CatanBaseConfiguration == nil && !next.publicCatanBaseAvailable()) || req.CatanBaseConfiguration == nil {
			err = errors.New("只有房主能在支持布局选择的等待房间调整基础地图")
			break
		}
		err = next.setCatanBaseConfiguration(*req.CatanBaseConfiguration)
	case "catan_seafarers":
		// A public scenario or an explicit internal recipe must provision this.
		if next.Host != u.ID || next.Kind != "catan" || next.Status != "waiting" || next.CatanSeafarers == nil || req.CatanSeafarers == nil {
			err = errors.New("只有房主能在航海家开局前选择剧本")
			break
		}
		err = next.setCatanSeafarers(*req.CatanSeafarers)
	case "catan_rivers_world_map", "catan_rivers_world_shuffle", "catan_rivers_world_default":
		if next.Host != u.ID || next.Kind != "catan" || next.Status != "waiting" || next.CatanScenario != "rivers-new-world" {
			err = errors.New("只有房主能在河流新世界开局前确认地图")
			break
		}
		layout := req.CatanRiversWorldMap
		n := next.Capacity
		if n == 2 {
			n = 4
		}
		if req.Type == "catan_rivers_world_shuffle" {
			layout, err = game.GenerateCatanRiversWorldMap(n)
		}
		if err == nil && req.Type != "catan_rivers_world_default" {
			err = game.ValidateCatanRiversWorldMap(n, layout)
		}
		if err == nil {
			next.CatanRiversWorldMap = layout
			if req.Type == "catan_rivers_world_default" {
				next.CatanRiversWorldMap = nil
			}
			for i := range next.Seats {
				next.Seats[i].Ready = next.Seats[i].Bot
			}
		}
	case "catan_world_map", "catan_world_map_shuffle":
		// A public New World selection or internal combination provisions the map.
		if next.Host != u.ID || next.Kind != "catan" || next.Status != "waiting" || next.CatanNewWorldMap == nil || (next.CatanCitiesKnights != nil && next.validateCatanCitiesKnightsMap() != nil) {
			err = errors.New("只有房主能在新世界开局前调整地图")
			break
		}
		layout := req.CatanNewWorldMap
		if req.Type == "catan_world_map_shuffle" {
			layout, err = next.generateCatanWorldMap()
		}
		if err == nil {
			mapPlayers := max(3, next.Capacity)
			if next.twoCatanSeafarers() {
				mapPlayers = 4
			}
			err = game.ValidateCatanNewWorldMap(mapPlayers, layout)
		}
		if err == nil && next.CatanFishing {
			if next.twoCatanSeafarers() {
				_, err = game.NewCatanTwoFishingSeafarers(2, next.CatanOptions, *next.CatanSeafarers, layout)
			} else {
				_, err = game.NewCatanFishingNewWorld(max(3, next.Capacity), next.CatanOptions, layout)
			}
		}
		if err == nil && !slices.Equal(next.CatanNewWorldMap.Hexes, layout.Hexes) {
			next.CatanNewWorldMap = layout
			for i := range next.Seats {
				next.Seats[i].Ready = next.Seats[i].Bot
			}
		}
	case "catan_fishing":
		if next.Host != u.ID || req.Enabled == nil {
			err = errors.New("只有房主能在开局前选择渔夫组合")
			break
		}
		err = next.setCatanFishing(*req.Enabled)
	case "catan_fishing_lakes":
		if next.Host != u.ID || req.Enabled == nil {
			err = errors.New("只有房主能在开局前选择捕鱼湖泊")
			break
		}
		err = next.setCatanFishingLakes(*req.Enabled)
	case "catan_scenario":
		if next.Host != u.ID || req.CatanScenario == nil {
			err = errors.New("只有房主能在开局前选择卡坦剧本")
			break
		}
		err = next.setCatanScenario(*req.CatanScenario)
	case "catan_two_scenario":
		if next.Host != u.ID || next.Kind != "catan" || next.Status != "waiting" || (next.CatanTwoRules != game.CatanTwoRules && !(next.Capacity == 2 && publicCatanFlexibleScenario(next.CatanScenario))) || req.CatanTwoScenario == nil {
			err = errors.New("只有房主能在双人卡坦开局前选择剧本")
			break
		}
		err = next.setCatanTwoScenario(*req.CatanTwoScenario)
	case "catan_options":
		if next.Host != u.ID || next.Kind != "catan" || next.Status != "waiting" {
			err = errors.New("只有房主能在开局前选择卡坦岛扩展")
			break
		}
		options, optionErr := game.NormalizeCatanOptions(req.CatanOptions)
		err = optionErr
		if err == nil && options == next.CatanOptions {
			break
		}
		if next.CatanTwoRules != "" {
			if err == nil {
				next.CatanOptions = options
				err = next.validateCatanTwoSetup()
			}
			if err == nil {
				for i := range next.Seats {
					next.Seats[i].Ready = next.Seats[i].Bot
				}
			}
			break
		}
		if publicCatanExplorerScenario(next.CatanScenario) {
			if err == nil && !validCatanExplorerOptions(options) {
				err = errors.New("探索者按实际人数启用扩充，只能在此选择助手")
			}
			if err == nil {
				next.CatanOptions = options
				err = next.validateCatanScenario()
			}
			if err == nil {
				for i := range next.Seats {
					next.Seats[i].Ready = next.Seats[i].Bot
				}
			}
			break
		}
		if err == nil && next.friendlyRobberEnabled() && options.FiveSix && !next.CatanOptions.FiveSix && !next.isCatanBaseRecipe() && !next.isCatanStandaloneKnightsRecipe() && next.CatanScenario != "fishing" && !publicCatanSeaScenario(next.CatanScenario) {
			err = errors.New("友善强盗的五六人公开组合需要基础、城市骑士、渔夫或已核验的航海地图")
		}
		if err == nil && next.CatanHarbors != nil && next.CatanHarbors.Enabled && options.FiveSix && !next.CatanOptions.FiveSix && !next.isCatanBaseRecipe() && !next.isCatanStandaloneKnightsRecipe() && next.CatanScenario != "fishing" && !publicCatanSeaScenario(next.CatanScenario) {
			err = errors.New("港口霸主的五六人公开组合需要基础、城市骑士、渔夫或航海地图")
		}
		if err == nil && next.CatanCitiesKnights != nil {
			if mapErr := next.validateCatanCitiesKnightsMap(); mapErr != nil {
				err = mapErr
			}
			if err == nil && options.FiveSix != next.CatanOptions.FiveSix {
				n := 4
				if options.FiveSix {
					n = 6
				}
				setup := *next.CatanCitiesKnights
				setup.Rules = ""
				setup, err = game.NormalizeCatanCitiesKnightsSetup(n, setup)
				if err == nil {
					next.CatanCitiesKnights = &setup
				}
			}
		}
		if err == nil && !options.FiveSix && len(next.Seats) > 4 {
			err = errors.New("基础版最多四人，请先移除多余座位")
		}
		if err == nil && next.CatanSeafarers != nil && options.FiveSix != next.CatanOptions.FiveSix {
			// Changing the extension also changes the official recipe. Resolve its
			// new default explicitly, then require every human to ready up again.
			n := 4
			if options.FiveSix {
				n = 6
			}
			setup := *next.CatanSeafarers
			setup.Layout = ""
			setup, err = game.NormalizeCatanSeafarersSetup(n, setup)
			if err == nil {
				next.CatanSeafarers = &setup
			}
		}
		if err == nil && next.CatanBaseConfiguration != nil && options.FiveSix != next.CatanOptions.FiveSix {
			n := 4
			if options.FiveSix {
				n = 6
			}
			setup := *next.CatanBaseConfiguration
			setup.Rules = ""
			if !options.FiveSix {
				setup.Layout = "variable"
			}
			setup, err = game.NormalizeCatanBaseConfiguration(n, setup)
			if err == nil {
				next.CatanBaseConfiguration = &setup
			}
		}
		if err == nil {
			if options.FiveSix {
				next.Capacity = max(5, next.Capacity)
			} else {
				next.Capacity = min(4, next.Capacity)
			}
			if next.CatanNewWorldMap != nil && options.FiveSix != next.CatanOptions.FiveSix {
				next.CatanNewWorldMap, err = next.generateCatanWorldMap()
			}
			next.CatanOptions = options
			if err == nil && next.friendlyRobberEnabled() {
				err = next.validateCatanFriendlyRobber(max(3, next.Capacity))
			}
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
			err = next.validateCatanScenario()
		}
		if err == nil && next.Kind == "catan" && next.friendlyRobberEnabled() {
			err = next.validateCatanFriendlyRobber(len(next.Seats))
		}
		if err == nil {
			if next.Kind == "rail" && next.RailMap != "" && next.RailMap != "usa" {
				next.Game, err = game.NewRailMap(next.RailMap, len(next.Seats))
			} else if next.Kind == "splendor" && next.SplendorOptions != (game.SplendorOptions{}) {
				next.Game, err = game.NewSplendor(len(next.Seats), next.SplendorOptions)
			} else if next.Kind == "catan" {
				if next.CatanScenario == "caravans-tribe" {
					next.Game, err = game.NewCatanCaravansTribeSeafarers(len(next.Seats))
				} else if next.CatanScenario == "caravans-desert" {
					next.Game, err = game.NewCatanCaravansDesertSeafarers(len(next.Seats))
				} else if setup, ok := publicCatanRiversSeaSetup(next.CatanScenario); ok {
					if next.CatanRiversWorldMap != nil {
						setup.Layout = "prepared"
					}
					next.Game, err = game.NewCatanRiversSeafarers(len(next.Seats), setup, next.CatanRiversWorldMap)
				} else if next.CatanTwoRules != "" || next.CatanTwoScenario != "" {
					err = next.validateCatanTwoSetup()
					if err == nil {
						if next.twoCatanSeafarers() {
							if next.CatanCitiesKnights != nil {
								next.Game, err = game.NewCatanTwoSeafarersCitiesKnights(len(next.Seats), next.CatanOptions, *next.CatanSeafarers, next.CatanNewWorldMap, next.CatanFishing)
							} else if next.CatanFishing {
								next.Game, err = game.NewCatanTwoFishingSeafarers(len(next.Seats), next.CatanOptions, *next.CatanSeafarers, next.CatanNewWorldMap)
							} else {
								next.Game, err = game.NewCatanTwoSeafarers(len(next.Seats), next.CatanOptions, *next.CatanSeafarers, next.CatanNewWorldMap)
							}
						} else if next.CatanTwoScenario == "barbarian-attack" {
							if next.CatanFishing {
								next.Game, err = game.NewCatanFishingAttack(len(next.Seats), next.CatanCitiesKnights != nil)
							} else if next.CatanCitiesKnights != nil {
								next.Game, err = game.NewCatanTwoAttackCitiesKnights(len(next.Seats), next.CatanOptions)
							} else {
								next.Game, err = game.NewCatanTwoAttack(len(next.Seats), next.CatanOptions)
							}
						} else if next.CatanTwoScenario == "caravans" {
							if next.CatanFishing {
								next.Game, err = game.NewCatanFishingCaravans(len(next.Seats), next.CatanOptions, next.CatanCitiesKnights != nil)
							} else if next.CatanCitiesKnights != nil {
								next.Game, err = game.NewCatanCaravansCitiesKnights(len(next.Seats), next.CatanOptions)
							} else {
								next.Game, err = game.NewCatanTwoCaravans(len(next.Seats), next.CatanOptions)
							}
						} else if next.CatanTwoScenario == "cities-knights" {
							if next.CatanFishing {
								next.Game, err = game.NewCatanTwoFishingCitiesKnights(len(next.Seats), next.CatanOptions)
							} else {
								next.Game, err = game.NewCatanTwoCitiesKnights(len(next.Seats), next.CatanOptions)
							}
						} else if next.CatanTwoScenario == "fishing" {
							next.Game, err = game.NewCatanTwoFishing(len(next.Seats), next.CatanOptions)
						} else if next.CatanTwoScenario == "rivers" {
							if next.CatanFishing {
								next.Game, err = game.NewCatanFishingRivers(len(next.Seats), next.CatanOptions, next.CatanCitiesKnights != nil)
							} else if next.CatanCitiesKnights != nil {
								next.Game, err = game.NewCatanRiversCitiesKnights(len(next.Seats), next.CatanOptions)
							} else {
								next.Game, err = game.NewCatanTwoRivers(len(next.Seats), next.CatanOptions)
							}
						} else {
							next.Game, err = game.NewCatanTwo(len(next.Seats), next.CatanOptions)
						}
					}
				} else if publicCatanExplorerScenario(next.CatanScenario) && next.CatanFishing {
					next.Game, err = game.NewCatanExplorerFishing(len(next.Seats), next.CatanScenario, next.CatanCitiesKnights != nil, next.CatanFishingLakes)
				} else if publicCatanExplorerScenario(next.CatanScenario) && next.CatanCitiesKnights != nil {
					next.Game, err = game.NewCatanExplorerCitiesKnights(len(next.Seats), next.CatanScenario)
				} else if next.CatanScenario == "spices-for-catan" {
					next.Game, err = game.NewCatanExplorerSpices(len(next.Seats))
				} else if publicCatanExplorerExtended(next.CatanScenario) {
					next.Game, err = game.NewCatanExplorerMission(len(next.Seats), next.CatanScenario)
				} else if next.CatanScenario == "land-ho" {
					next.Game, err = game.NewCatanExplorerLandHo(len(next.Seats))
				} else if next.CatanScenario == "barbarian-attack" {
					if next.CatanFishing {
						next.Game, err = game.NewCatanFishingAttack(len(next.Seats), next.CatanCitiesKnights != nil)
					} else if next.CatanCitiesKnights != nil {
						next.Game, err = game.NewCatanAttackCitiesKnights(len(next.Seats))
					} else {
						next.Game, err = game.NewCatanAttack(len(next.Seats))
					}
				} else if next.CatanScenario == "attack-transport" {
					next.Game, err = game.NewCatanAttackTransport(len(next.Seats), next.CatanCitiesKnights != nil)
				} else if next.CatanScenario == "caravans-transport" {
					next.Game, err = game.NewCatanCaravansTransport(len(next.Seats), next.CatanCitiesKnights != nil)
				} else if next.CatanScenario == "caravans-attack" {
					next.Game, err = game.NewCatanCaravansAttack(len(next.Seats), next.CatanCitiesKnights != nil)
				} else if next.CatanScenario == "rivers-transport" {
					next.Game, err = game.NewCatanRiversTransport(len(next.Seats), next.CatanCitiesKnights != nil)
				} else if next.CatanScenario == "rivers-attack" {
					next.Game, err = game.NewCatanRiversAttack(len(next.Seats), next.CatanCitiesKnights != nil)
				} else if next.CatanScenario == "rivers-caravans" {
					next.Game, err = game.NewCatanRiversCaravans(len(next.Seats), game.CatanOptions{FiveSix: len(next.Seats) > 4}, next.CatanCitiesKnights != nil)
				} else if next.CatanScenario == "transport" {
					if next.CatanFishing {
						next.Game, err = game.NewCatanFishingTransport(len(next.Seats), next.CatanCitiesKnights != nil)
					} else if next.CatanCitiesKnights != nil {
						next.Game, err = game.NewCatanTransportCitiesKnights(len(next.Seats))
					} else {
						next.Game, err = game.NewCatanTransport(len(next.Seats))
					}
				} else if next.CatanScenario == "fishing" {
					if next.CatanCitiesKnights != nil {
						next.Game, err = game.NewCatanFishingCitiesKnights(len(next.Seats), next.CatanOptions)
					} else {
						next.Game, err = game.NewCatanFishing(len(next.Seats), next.CatanOptions)
					}
				} else if next.CatanScenario == "rivers" {
					if next.CatanFishing {
						next.Game, err = game.NewCatanFishingRivers(len(next.Seats), next.CatanOptions, next.CatanCitiesKnights != nil)
					} else if next.CatanCitiesKnights != nil {
						next.Game, err = game.NewCatanRiversCitiesKnights(len(next.Seats), next.CatanOptions)
					} else {
						next.Game, err = game.NewCatanRivers(len(next.Seats), next.CatanOptions)
					}
				} else if next.CatanScenario == "caravans" {
					if next.CatanFishing {
						next.Game, err = game.NewCatanFishingCaravans(len(next.Seats), next.CatanOptions, next.CatanCitiesKnights != nil)
					} else if next.CatanCitiesKnights != nil {
						next.Game, err = game.NewCatanCaravansCitiesKnights(len(next.Seats), next.CatanOptions)
					} else {
						next.Game, err = game.NewCatanCaravans(len(next.Seats), next.CatanOptions)
					}
				} else if next.CatanFishing {
					if next.CatanCitiesKnights != nil {
						next.Game, err = game.NewCatanFishingCitiesKnightsSeafarers(len(next.Seats), next.CatanOptions, *next.CatanSeafarers, next.CatanNewWorldMap)
					} else if next.CatanScenario == "new_world" {
						next.Game, err = game.NewCatanFishingNewWorld(len(next.Seats), next.CatanOptions, next.CatanNewWorldMap)
					} else {
						next.Game, err = game.NewCatanFishingSeafarers(len(next.Seats), next.CatanOptions, *next.CatanSeafarers, nil)
					}
				} else if next.CatanCitiesKnights != nil {
					err = next.validateCatanCitiesKnightsMap()
					if err == nil {
						_, err = game.NormalizeCatanCitiesKnightsSetup(len(next.Seats), *next.CatanCitiesKnights)
					}
					if err == nil && next.CatanSeafarers != nil {
						next.Game, err = game.NewCatanCitiesKnightsSeafarers(len(next.Seats), next.CatanOptions, *next.CatanSeafarers, next.CatanNewWorldMap)
					} else if err == nil {
						next.Game, err = game.NewCatanCitiesKnightsConfigured(len(next.Seats), next.CatanOptions, *next.CatanCitiesKnights)
					}
				} else if next.CatanBaseConfiguration != nil && (next.CatanSeafarers != nil || next.CatanNewWorldMap != nil) {
					err = errors.New("基础布局不能与航海家剧本混用")
				} else if next.CatanBaseConfiguration != nil {
					next.Game, err = game.NewCatanConfigured(len(next.Seats), next.CatanOptions, *next.CatanBaseConfiguration)
				} else if next.CatanSeafarers != nil {
					next.Game, err = game.NewCatanSeafarers(len(next.Seats), next.CatanOptions, *next.CatanSeafarers, next.CatanNewWorldMap)
				} else if next.CatanNewWorldMap != nil {
					next.Game, err = game.NewCatanNewWorldWithMap(len(next.Seats), next.CatanOptions, next.CatanNewWorldMap)
				} else {
					next.Game, err = game.NewCatan(len(next.Seats), next.CatanOptions)
				}
				if err == nil && next.CatanFriendlyRobber != nil {
					err = next.Game.ConfigureCatanFriendlyRobber(*next.CatanFriendlyRobber)
				}
				if err == nil && next.CatanHarbors != nil {
					err = next.Game.ConfigureCatanHarbors(*next.CatanHarbors)
				}
			} else if next.Kind == "sanguosha" {
				next.Game, err = game.NewSanguosha(len(next.Seats), next.SanguoshaOptions)
			} else {
				next.Game, err = s.newGame(next.Kind, len(next.Seats))
			}
			if err == nil && next.Kind == "catan" && publicCatanExplorerScenario(next.CatanScenario) && next.CatanOptions.Helpers {
				err = next.Game.EnableCatanExplorerHelpers(next.CatanOptions.AllHelpers)
			}
			if err == nil && next.CatanEvents != "" {
				err = next.Game.EnableCatanEvents(next.CatanEvents)
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
		next.Seats[idx].TimeoutAutoPlay = false
		if !*req.Enabled && next.Seats[idx].AutoPlay && next.TurnDeadline <= now.UnixMilli() && slices.Contains(next.timeoutActors(), idx) {
			// Let a returning player actually act instead of being taken over
			// again on the very next timer tick. Live windows are unchanged.
			pausedSGTime := next.SGTimeLeft
			next.startTurnClock(now)
			if next.Game.Sanguosha != nil && next.Game.Sanguosha.Pending != nil {
				// Reclaiming a response gives that responder a new window,
				// without replenishing the paused turn owner's action budget.
				next.SGTimeLeft = pausedSGTime
			}
		}
		if !*req.Enabled && next.Seats[idx].AutoPlay && next.Game.Turn == idx {
			// A timed-out turn can be paused while another player responds.
			// Reclaim the exhausted owner's budget too, otherwise completing
			// that response would immediately take the owner over again.
			// Never extend the other responder's deadline or a positive budget.
			if next.Game.Sanguosha != nil && next.Game.Sanguosha.Pending != nil && next.SGTimeLeft == 0 {
				next.SGTimeLeft = turnLimit.Milliseconds()
			}
			if next.Game.Catan != nil && (next.Game.CatanPendingActor() >= 0 || next.Game.Phase == "catan_discard") && next.CatanTimeLeft == 0 {
				next.CatanTimeLeft = turnLimit.Milliseconds()
			}
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
		err = errors.New("超时会自动开启电脑托管，玩家保留席位并可随时取消托管")
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
			next.Seats[i].TimeoutAutoPlay = false
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
	if err == nil && next.Status == "waiting" {
		err = next.validateCatanTwoSetup()
		if err == nil {
			err = next.validateCatanScenario()
		}
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
