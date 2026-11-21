package server

import (
	"container/list"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const sessionCacheCapacity = 1024

type authSessionStats struct {
	SQLReads int64
	Hits     int64
	Entries  int
}

type verifiedSessionEntry struct {
	key     string
	user    User
	expires int64
}

type verifiedSessions struct {
	mu         sync.Mutex
	entries    map[string]*list.Element
	order      list.List
	generation uint64
	dirty      bool
	hits       int64
}

func newVerifiedSessions() *verifiedSessions {
	return &verifiedSessions{entries: make(map[string]*list.Element)}
}

func (cache *verifiedSessions) lookup(key string, now int64) (User, bool, uint64) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if !cache.dirty {
		if element := cache.entries[key]; element != nil {
			entry := element.Value.(verifiedSessionEntry)
			if entry.expires > now {
				cache.order.MoveToFront(element)
				cache.hits++
				return entry.user, true, cache.generation
			}
			delete(cache.entries, key)
			cache.order.Remove(element)
		}
	}
	return User{}, false, cache.generation
}

func (cache *verifiedSessions) admit(key string, user User, expires int64, generation uint64) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.dirty || generation != cache.generation || expires <= time.Now().Unix() {
		return
	}
	if element := cache.entries[key]; element != nil {
		element.Value = verifiedSessionEntry{key: key, user: user, expires: expires}
		cache.order.MoveToFront(element)
		return
	}
	if len(cache.entries) >= sessionCacheCapacity {
		oldest := cache.order.Back()
		delete(cache.entries, oldest.Value.(verifiedSessionEntry).key)
		cache.order.Remove(oldest)
	}
	cache.entries[key] = cache.order.PushFront(verifiedSessionEntry{key: key, user: user, expires: expires})
}

func (cache *verifiedSessions) invalidate(dirty bool) {
	cache.mu.Lock()
	cache.generation++
	clear(cache.entries)
	cache.order.Init()
	cache.dirty = dirty
	cache.mu.Unlock()
}

func (cache *verifiedSessions) finish() {
	cache.mu.Lock()
	if cache.dirty {
		cache.generation++
		clear(cache.entries)
		cache.order.Init()
		cache.dirty = false
	}
	cache.mu.Unlock()
}

func (cache *verifiedSessions) changed(data sqlite.SQLitePreUpdateData) {
	if data.DatabaseName != "main" {
		return
	}
	switch data.TableName {
	case "sessions", "users":
	case "user_controls":
		if data.Op == sqlite3.SQLITE_UPDATE {
			var previous, next [3]any
			if data.Old(previous[:]...) == nil && data.New(next[:]...) == nil && previous[0] == next[0] && previous[1] == next[1] && previous[2] == next[2] {
				return
			}
		}
	default:
		return
	}
	cache.invalidate(true)
}

type sessionSQLiteConnection interface {
	driver.Conn
	driver.ConnBeginTx
	driver.ConnPrepareContext
	driver.ExecerContext
	driver.QueryerContext
	driver.Pinger
	driver.SessionResetter
	driver.Validator
	sqlite.HookRegisterer
}

type sessionConnection struct {
	sessionSQLiteConnection
	cache *verifiedSessions
}

func (connection *sessionConnection) Close() error {
	connection.RegisterPreUpdateHook(nil)
	connection.RegisterCommitHook(nil)
	connection.RegisterRollbackHook(nil)
	connection.cache.invalidate(false)
	return connection.sessionSQLiteConnection.Close()
}

type sessionConnector struct {
	path   string
	driver *sqlite.Driver
	cache  *verifiedSessions
}

func newSessionConnector(path string, cache *verifiedSessions) *sessionConnector {
	return &sessionConnector{path: path, driver: &sqlite.Driver{}, cache: cache}
}

func (connector *sessionConnector) Driver() driver.Driver { return connector.driver }

func (connector *sessionConnector) Connect(ctx context.Context) (driver.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw, err := connector.driver.Open(connector.path)
	if err != nil {
		return nil, err
	}
	connection, ok := raw.(sessionSQLiteConnection)
	if !ok {
		raw.Close()
		return nil, errors.New("数据库缺少会话失效钩子")
	}
	connector.cache.invalidate(false)
	connection.RegisterPreUpdateHook(connector.cache.changed)
	connection.RegisterCommitHook(func() int32 { connector.cache.finish(); return 0 })
	connection.RegisterRollbackHook(connector.cache.finish)
	return &sessionConnection{sessionSQLiteConnection: connection, cache: connector.cache}, nil
}

func (s *Server) verifiedSession(token string) (User, error) {
	if len(token) == 0 || len(token) > 256 {
		return User{}, sql.ErrNoRows
	}
	key := digest(token)
	now := time.Now().Unix()
	var generation uint64
	if s.authSessions != nil {
		user, found, captured := s.authSessions.lookup(key, now)
		if found {
			return user, nil
		}
		generation = captured
	}
	s.sessionSQLReads.Add(1)
	var user User
	var expires int64
	err := s.db.QueryRow("SELECT users.id,users.name,sessions.expires FROM sessions JOIN users ON users.id=sessions.user_id LEFT JOIN user_controls c ON c.user_id=users.id WHERE token=? AND expires>? AND COALESCE(c.disabled,0)=0", key, now).Scan(&user.ID, &user.Name, &expires)
	if err == nil && expires <= time.Now().Unix() {
		err = sql.ErrNoRows
	}
	if err == nil && s.authSessions != nil {
		s.authSessions.admit(key, user, expires, generation)
	}
	return user, err
}

func (s *Server) authSessionMetrics() authSessionStats {
	stats := authSessionStats{SQLReads: s.sessionSQLReads.Load()}
	if s.authSessions != nil {
		s.authSessions.mu.Lock()
		stats.Hits, stats.Entries = s.authSessions.hits, len(s.authSessions.entries)
		s.authSessions.mu.Unlock()
	}
	return stats
}
