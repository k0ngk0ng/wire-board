package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func (s *Server) passwordChangeAllowed(u User) bool {
	// Explicit bootstrap credentials rotate from private configuration at
	// startup. Existing-account administrators have no such managed password.
	return s.cfg.AdminPassword == "" || strings.TrimSpace(s.cfg.AdminUsername) != u.Name
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	if !s.passwordChangeAllowed(u) {
		fail(w, http.StatusForbidden, "该管理员密码由服务器私有配置维护")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var req struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(&req) != nil || d.Decode(new(any)) != io.EOF {
		fail(w, http.StatusBadRequest, "请求格式不正确")
		return
	}
	if req.CurrentPassword == "" || len(req.CurrentPassword) > 72 || len(req.NewPassword) < 8 || len(req.NewPassword) > 72 {
		fail(w, http.StatusBadRequest, "请填写原密码，新密码需 8–72 字节")
		return
	}
	if req.CurrentPassword == req.NewPassword {
		fail(w, http.StatusBadRequest, "新密码不能与原密码相同")
		return
	}
	// Expensive password work stays outside the platform state mutex.
	s.mu.Lock()
	now := time.Now()
	key := "password:" + u.ID
	b := s.limits[key]
	if now.Sub(b.At) > time.Minute {
		b = bucket{At: now}
	}
	b.Count++
	s.limits[key] = b
	s.mu.Unlock()
	if b.Count > 5 {
		fail(w, http.StatusTooManyRequests, "尝试过于频繁，请一分钟后重试")
		return
	}
	var previous string
	if err := s.db.QueryRow("SELECT password FROM users WHERE id=?", u.ID).Scan(&previous); err != nil {
		fail(w, http.StatusInternalServerError, "无法读取账号")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(previous), []byte(req.CurrentPassword)) != nil {
		fail(w, http.StatusForbidden, "原密码不正确")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		fail(w, http.StatusInternalServerError, "无法更新密码")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// A concurrent change, logout or disable must invalidate this request too.
	if _, ok = s.needUser(w, r); !ok {
		return
	}
	token := randomID(32)
	expiry := time.Now().Add(30 * 24 * time.Hour)
	tx, err := s.db.Begin()
	if err != nil {
		fail(w, http.StatusInternalServerError, "密码未修改，请重试")
		return
	}
	defer tx.Rollback()
	updated, err := tx.Exec("UPDATE users SET password=? WHERE id=? AND password=?", string(hash), u.ID, previous)
	if err == nil {
		count, e := updated.RowsAffected()
		err = e
		if err == nil && count != 1 {
			fail(w, http.StatusConflict, "账号密码已更新，请重新登录后再试")
			return
		}
	}
	if err == nil {
		_, err = tx.Exec("DELETE FROM sessions WHERE user_id=?", u.ID)
	}
	if err == nil {
		_, err = tx.Exec("INSERT INTO sessions(token,user_id,expires) VALUES(?,?,?)", digest(token), u.ID, expiry.Unix())
	}
	if err == nil {
		err = activityInsert(tx, u.ID, "password_change", "", s.clientIP(r))
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, "密码未修改，请重试")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "wb_session", Value: token, Path: "/", HttpOnly: true, Secure: s.cfg.SecureCookie, SameSite: http.SameSiteStrictMode, Expires: expiry, MaxAge: 30 * 86400})
	s.seen(u.ID)
	respond(w, http.StatusOK, map[string]bool{"ok": true})
	s.broadcast()
}
