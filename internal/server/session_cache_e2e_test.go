package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func sessionRequest(t *testing.T, client *testClient) *http.Request {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, client.base+"/api/state", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, cookie := range client.client.Jar.Cookies(request.URL) {
		request.AddCookie(cookie)
	}
	return request
}

func TestSessionCacheRealtimeAvoidsRepeatedSQL(t *testing.T) {
	fixture := newUnchangedTable(t, unchangedFixture(true))
	conn := streamDial(t, fixture.a, true)
	packets := unchangedReader(t, conn)
	stats := &unchangedStats{}
	unchangedKind(t, packets, stats, "state")
	before := fixture.s.authSessionMetrics()
	version := current(fixture.a)["version"]
	for index := range 40 {
		nonce := fmt.Sprintf("session-cache-%d", index)
		streamSend(t, conn, map[string]any{"type": "input", "roomId": fixture.id, "version": version, "nonce": nonce, "action": map[string]any{"drive": 1}})
		ack := unchangedKind(t, packets, stats, "ack")
		if unchangedInteger(t, ack["status"]) != 200 || ack["nonce"] != nonce {
			t.Fatal("session cache changed input authorization", ack)
		}
		version = ack["version"]
	}
	after := fixture.s.authSessionMetrics()
	queries := after.SQLReads - before.SQLReads
	hits := after.Hits - before.Hits
	unchangedEvidence(t, "session-cache-queries", map[string]any{"passed": queries <= 2 && hits >= 40, "inputs": 40, "sqlReads": queries, "cacheHits": hits})
	if queries > 2 || hits < 40 {
		t.Fatal("realtime frames still repeatedly query SQL sessions", before, after)
	}
}

func TestSessionCacheDirectSQLInvalidationAndRollback(t *testing.T) {
	fixture := newUnchangedTable(t, unchangedFixture(true))
	request := sessionRequest(t, fixture.a)
	user, err := fixture.s.user(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.s.db.Exec("UPDATE users SET name=? WHERE id=?", "缓存昵称已更新", user.ID); err != nil {
		t.Fatal(err)
	}
	renamed, err := fixture.s.user(request)
	if err != nil || renamed.Name != "缓存昵称已更新" {
		t.Fatal("cached name survived identity change", renamed, err)
	}
	tx, err := fixture.s.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec("DELETE FROM sessions WHERE user_id=?", user.ID); err != nil {
		t.Fatal(err)
	}
	ready := make(chan error, 1)
	go func() { _, err := fixture.s.user(request); ready <- err }()
	select {
	case err := <-ready:
		t.Fatal("dirty transaction reused an authenticated cache entry", err)
	case <-time.After(20 * time.Millisecond):
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-ready:
		if err != nil {
			t.Fatal("rollback invalidated the real session", err)
		}
	case <-time.After(time.Second):
		t.Fatal("session validation stayed blocked after rollback")
	}
	if _, err := fixture.s.db.Exec("UPDATE user_controls SET disabled=1 WHERE user_id=?", user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.s.user(request); err == nil {
		t.Fatal("direct disable did not revoke cached authorization")
	}
	if _, err := fixture.s.db.Exec("UPDATE user_controls SET disabled=0 WHERE user_id=?", user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.s.user(request); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.s.db.Exec("DELETE FROM sessions WHERE user_id=?", user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.s.user(request); err == nil {
		t.Fatal("direct session revocation was cached")
	}
	unchangedEvidence(t, "session-cache-direct-sql", map[string]any{"passed": true, "rename": true, "dirtyBlocked": true, "rollback": true, "disable": true, "revoke": true})
}

func TestSessionCacheRevokesWarmWebSocketsThroughAccountActions(t *testing.T) {
	for _, action := range []string{"logout", "disable", "password", "self-password"} {
		t.Run(action, func(t *testing.T) {
			fixture := newUnchangedTable(t, unchangedFixture(true))
			request := sessionRequest(t, fixture.a)
			user, err := fixture.s.user(request)
			if err != nil {
				t.Fatal(err)
			}
			conn := streamDial(t, fixture.a, true)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if _, _, err := conn.Read(ctx); err != nil {
				t.Fatal(err)
			}
			if action == "self-password" {
				fixture.a.post(passwordPath, passwordBody("test-password-123", "cache-new-password-123"), 200)
			} else if action == "logout" {
				fixture.a.post("/api/logout", map[string]any{}, 200)
			} else {
				root := newClient(t, fixture.ts.URL)
				root.post("/api/login", map[string]any{"name": "ModuleRoot", "password": "module-root-test-123"}, 200)
				body := map[string]any{"action": action}
				if action == "password" {
					body["password"] = "cache-reset-password-123"
				}
				root.post("/api/admin/users/"+user.ID, body, 200)
			}
			if _, err := fixture.s.user(request); err == nil {
				t.Fatal("old authenticated token survived account action", action)
			}
			if _, _, err := conn.Read(ctx); err == nil {
				t.Fatal("warm websocket survived account invalidation", action)
			}
		})
	}
	unchangedEvidence(t, "session-cache-account-actions", map[string]any{"passed": true, "actions": []string{"logout", "disable", "password", "self-password"}})
}

func TestSessionCacheExpiryRoleAndAccountIsolation(t *testing.T) {
	fixture := newUnchangedTable(t, unchangedFixture(true))
	first := sessionRequest(t, fixture.a)
	user, err := fixture.s.user(first)
	if err != nil {
		t.Fatal(err)
	}
	other := newClient(t, fixture.ts.URL)
	other.register("缓存隔离账号")
	second, err := fixture.s.user(sessionRequest(t, other))
	if err != nil || second.ID == user.ID {
		t.Fatal("accounts shared cached identity", user, second, err)
	}
	if _, err := fixture.s.db.Exec("UPDATE user_controls SET role='admin' WHERE user_id=?", user.ID); err != nil {
		t.Fatal(err)
	}
	fixture.a.request(http.MethodGet, "/api/admin", nil)
	if _, err := fixture.s.db.Exec("UPDATE user_controls SET role='player' WHERE user_id=?", user.ID); err != nil {
		t.Fatal(err)
	}
	if status, _ := fixture.a.request(http.MethodGet, "/api/admin", nil); status != 403 {
		t.Fatal("cached administrator role survived demotion", status)
	}
	expires := time.Now().Unix() + 1
	if _, err := fixture.s.db.Exec("UPDATE sessions SET expires=? WHERE user_id=?", expires, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.s.user(first); err != nil {
		t.Fatal(err)
	}
	for time.Now().Unix() < expires {
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := fixture.s.user(first); err == nil {
		t.Fatal("session survived its actual database expiration")
	}
	if _, err := fixture.s.user(sessionRequest(t, other)); err != nil {
		t.Fatal("expiration crossed accounts", err)
	}
	unchangedEvidence(t, "session-cache-expiry-isolation", map[string]any{"passed": true, "actualExpiry": true, "roleDemotion": true, "separateAccounts": true})
}

func TestSessionCacheMemoryRemainsBounded(t *testing.T) {
	fixture := newUnchangedTable(t, unchangedFixture(true))
	user, err := fixture.s.user(sessionRequest(t, fixture.a))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := fixture.s.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for index := range 1100 {
		if _, err := tx.Exec("INSERT INTO sessions(token,user_id,expires) VALUES(?,?,?)", digest(fmt.Sprintf("bounded-session-%d", index)), user.ID, time.Now().Add(time.Hour).Unix()); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	for index := range 1100 {
		request := sessionRequest(t, fixture.a)
		request.Header.Del("Cookie")
		request.AddCookie(&http.Cookie{Name: "wb_session", Value: fmt.Sprintf("bounded-session-%d", index)})
		if _, err := fixture.s.user(request); err != nil {
			t.Fatal(err)
		}
	}
	metrics := fixture.s.authSessionMetrics()
	if metrics.Entries > 1024 {
		t.Fatal("session cache grew without a bound", metrics)
	}
	parsed, _ := url.Parse(fixture.ts.URL)
	if strings.Contains(fmt.Sprint(metrics), fixture.a.client.Jar.Cookies(parsed)[0].Value) {
		t.Fatal("session metrics contain a raw token")
	}
	unchangedEvidence(t, "session-cache-bounds", map[string]any{"passed": true, "entries": metrics.Entries, "capacity": 1024, "queriedTokens": 1100})
}
