package server

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	wishBodyLimit      = 6 << 20
	wishImageLimit     = 5 << 20
	wishTextMinRunes   = 2
	wishTextMaxRunes   = 2000
	wishNoteMaxRunes   = 500
	wishSubmitPerHour  = 10
	wishListLimit      = 100
	wishAdminListLimit = 200
	wishAgentLimit     = 200
)

type wish struct {
	ID             string `json:"id"`
	UserName       string `json:"userName"`
	Text           string `json:"text"`
	ImageURL       string `json:"imageUrl"`
	Status         string `json:"status"`
	Note           string `json:"note"`
	CreatedAt      int64  `json:"createdAt"`
	ReviewedAt     int64  `json:"reviewedAt,omitempty"`
	ApprovedAt     int64  `json:"approvedAt,omitempty"`
	ReviewedByName string `json:"reviewedByName,omitempty"`
}

func validWishURL(raw string) (*url.URL, bool) {
	if raw == "" || strings.ContainsAny(raw, "\r\n\"' ;") {
		return nil, false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, false
	}
	return u, true
}

func normalizeWishConfig(cfg *Config) error {
	cfg.WishImageBaseURL = strings.TrimRight(strings.TrimSpace(cfg.WishImageBaseURL), "/")
	if cfg.WishImageBaseURL == "" {
		cfg.WishImageBaseURL = "https://cdn.ichenj.com"
	}
	if _, ok := validWishURL(cfg.WishImageBaseURL); !ok {
		return errors.New("WISH_IMAGE_BASE_URL 必须是 HTTP(S) 图片目录地址")
	}
	cfg.WishImagePrefix = strings.Trim(strings.TrimSpace(cfg.WishImagePrefix), "/")
	if cfg.WishImagePrefix == "" {
		cfg.WishImagePrefix = "bga/wishes"
	}
	for _, segment := range strings.Split(cfg.WishImagePrefix, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return errors.New("WISH_IMAGE_PREFIX 必须是安全的 OSS 对象前缀")
		}
		for _, c := range segment {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
				return errors.New("WISH_IMAGE_PREFIX 必须是安全的 OSS 对象前缀")
			}
		}
	}
	configured := 0
	for _, value := range []string{cfg.OSSEndpoint, cfg.OSSBucket, cfg.OSSAccessKeyID, cfg.OSSAccessKeySecret} {
		if strings.TrimSpace(value) != "" {
			configured++
		}
	}
	if configured != 0 && configured != 4 {
		return errors.New("OSS_ENDPOINT、OSS_BUCKET、OSS_ACCESS_KEY_ID、OSS_ACCESS_KEY_SECRET 必须同时配置")
	}
	if configured == 4 {
		cfg.OSSEndpoint = strings.TrimRight(strings.TrimSpace(cfg.OSSEndpoint), "/")
		u, ok := validWishURL(cfg.OSSEndpoint)
		if !ok || u.Path != "" {
			return errors.New("OSS_ENDPOINT 必须是 HTTP(S) 地址，不带路径")
		}
		cfg.OSSBucket = strings.TrimSpace(cfg.OSSBucket)
		if cfg.OSSBucket == "" || strings.ContainsAny(cfg.OSSBucket, "/\\?# ") {
			return errors.New("OSS_BUCKET 名称不合法")
		}
		cfg.OSSAccessKeyID = strings.TrimSpace(cfg.OSSAccessKeyID)
		cfg.OSSAccessKeySecret = strings.TrimSpace(cfg.OSSAccessKeySecret)
	}
	cfg.WishAgentToken = strings.TrimSpace(cfg.WishAgentToken)
	if cfg.WishAgentToken != "" && len(cfg.WishAgentToken) < 16 {
		return errors.New("WISH_AGENT_TOKEN 至少需要 16 字节")
	}
	return nil
}

func ossConfigured(cfg Config) bool {
	return cfg.OSSEndpoint != "" && cfg.OSSBucket != "" && cfg.OSSAccessKeyID != "" && cfg.OSSAccessKeySecret != ""
}

func wishImageKey(cfg Config, ext string) string {
	return fmt.Sprintf("%s/%s/%s%s", cfg.WishImagePrefix, time.Now().UTC().Format("20060102"), randomID(16), ext)
}

func wishImageURL(cfg Config, key string) string {
	return strings.TrimRight(cfg.WishImageBaseURL, "/") + "/" + key
}

// OSS signature v1 is intentionally implemented with the standard library:
// the runtime image only needs to publish the small wish image objects and
// should not carry the full cloud SDK.
func ossAuthorization(cfg Config, method, key, contentType, contentMD5, date string, extra map[string]string) string {
	headers := ""
	for name, value := range extra {
		headers += strings.ToLower(name) + ":" + value + "\n"
	}
	resource := "/" + cfg.OSSBucket + "/" + key
	toSign := method + "\n" + contentMD5 + "\n" + contentType + "\n" + date + "\n" + headers + resource
	mac := hmac.New(sha1.New, []byte(cfg.OSSAccessKeySecret))
	_, _ = mac.Write([]byte(toSign))
	return "OSS " + cfg.OSSAccessKeyID + ":" + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Server) ossDo(ctx context.Context, method, key string, body []byte, contentType string) error {
	date := time.Now().UTC().Format(http.TimeFormat)
	contentMD5 := ""
	extra := map[string]string{}
	if len(body) > 0 {
		sum := md5.Sum(body)
		contentMD5 = base64.StdEncoding.EncodeToString(sum[:])
		extra["x-oss-forbid-overwrite"] = "true"
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(s.cfg.OSSEndpoint, "/")+"/"+key, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Date", date)
	req.Header.Set("Authorization", ossAuthorization(s.cfg, method, key, contentType, contentMD5, date, extra))
	if len(body) > 0 {
		req.Header.Set("Content-MD5", contentMD5)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Cache-Control", "public, max-age=31536000, immutable")
		req.Header.Set("x-oss-forbid-overwrite", "true")
	}
	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("OSS %s 返回 %d: %s", method, res.StatusCode, strings.TrimSpace(string(detail)))
	}
	return nil
}

func detectWishImage(data []byte) (string, string, bool) {
	switch http.DetectContentType(data) {
	case "image/png":
		return "image/png", ".png", true
	case "image/jpeg":
		return "image/jpeg", ".jpg", true
	case "image/webp":
		return "image/webp", ".webp", true
	case "image/gif":
		return "image/gif", ".gif", true
	default:
		return "", "", false
	}
}

func validWishText(raw string) (string, bool) {
	text := strings.TrimSpace(raw)
	n := utf8.RuneCountInString(text)
	return text, n >= wishTextMinRunes && n <= wishTextMaxRunes
}

func (s *Server) recentWishCount(userID string) (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM wishes WHERE user_id=? AND created_at>?", userID, time.Now().Add(-time.Hour).Unix()).Scan(&count)
	return count, err
}

func (s *Server) createWish(w http.ResponseWriter, r *http.Request) {
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	text := ""
	var image []byte
	imageType, imageExt := "", ""
	switch mediaType {
	case "application/json":
		var req struct {
			Text string `json:"text"`
		}
		if !decode(w, r, &req) {
			return
		}
		text = req.Text
	case "multipart/form-data":
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				fail(w, 413, "图片不能超过 5 MB")
			} else {
				fail(w, 400, "上传内容格式不正确")
			}
			return
		}
		defer r.MultipartForm.RemoveAll()
		text = r.FormValue("text")
		file, _, err := r.FormFile("image")
		if err == nil {
			defer file.Close()
			image, err = io.ReadAll(io.LimitReader(file, wishImageLimit+1))
			if err != nil {
				fail(w, 400, "无法读取图片")
				return
			}
			if len(image) == 0 {
				fail(w, 400, "图片文件为空")
				return
			}
			if len(image) > wishImageLimit {
				fail(w, 413, "图片不能超过 5 MB")
				return
			}
		} else if !errors.Is(err, http.ErrMissingFile) {
			fail(w, 400, "无法读取图片")
			return
		}
	default:
		fail(w, 415, "请使用 JSON 或表单上传许愿")
		return
	}
	text, valid := validWishText(text)
	if !valid {
		fail(w, 400, "许愿内容需 2–2000 字")
		return
	}
	if len(image) > 0 {
		imageType, imageExt, ok = detectWishImage(image)
		if !ok {
			fail(w, 400, "图片仅支持 PNG、JPEG、WebP 或 GIF")
			return
		}
		if !ossConfigured(s.cfg) {
			fail(w, 503, "图片上传尚未配置，可先提交纯文字许愿")
			return
		}
	}
	count, err := s.recentWishCount(u.ID)
	if err != nil {
		fail(w, 500, "无法检查许愿频率")
		return
	}
	if count >= wishSubmitPerHour {
		fail(w, 429, "每小时最多提交 10 个许愿，请稍后再试")
		return
	}
	imageURL, key := "", ""
	if len(image) > 0 {
		key = wishImageKey(s.cfg, imageExt)
		if err = s.ossDo(r.Context(), "PUT", key, image, imageType); err != nil {
			fail(w, 502, "图片上传失败，请稍后重试")
			return
		}
		imageURL = wishImageURL(s.cfg, key)
	}
	wishID := randomID(16)
	now := time.Now().Unix()
	_, err = s.db.Exec("INSERT INTO wishes(id,user_id,user_name,text,image_url,status,created_at) VALUES(?,?,?,?,?,'pending',?)", wishID, u.ID, u.Name, text, imageURL, now)
	if err != nil {
		if key != "" {
			_ = s.ossDo(context.Background(), "DELETE", key, nil, "")
		}
		fail(w, 500, "许愿未保存，请重试")
		return
	}
	respond(w, 200, map[string]any{"wish": wish{ID: wishID, UserName: u.Name, Text: text, ImageURL: imageURL, Status: "pending", CreatedAt: now}})
}

func scanWish(scanner interface{ Scan(...any) error }) (wish, error) {
	var w wish
	err := scanner.Scan(&w.ID, &w.UserName, &w.Text, &w.ImageURL, &w.Status, &w.Note, &w.CreatedAt, &w.ReviewedAt, &w.ReviewedByName)
	if w.Status == "approved" {
		w.ApprovedAt = w.ReviewedAt
	}
	return w, err
}

func (s *Server) listWishes(w http.ResponseWriter, r *http.Request) {
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	rows, err := s.db.Query("SELECT id,user_name,text,image_url,status,review_note,created_at,reviewed_at,COALESCE(reviewed_by_name,'') FROM wishes WHERE user_id=? ORDER BY created_at DESC,id DESC LIMIT ?", u.ID, wishListLimit)
	if err != nil {
		fail(w, 500, "无法读取许愿")
		return
	}
	defer rows.Close()
	list := []wish{}
	for rows.Next() {
		item, err := scanWish(rows)
		if err != nil {
			fail(w, 500, "无法读取许愿")
			return
		}
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		fail(w, 500, "无法读取许愿")
		return
	}
	respond(w, 200, map[string]any{"wishes": list})
}

func (s *Server) adminWishes() ([]wish, map[string]int, error) {
	rows, err := s.db.Query(`SELECT id,user_name,text,image_url,status,review_note,created_at,reviewed_at,COALESCE(reviewed_by_name,'') FROM wishes
ORDER BY CASE status WHEN 'pending' THEN 0 WHEN 'approved' THEN 1 ELSE 2 END,created_at DESC,id DESC LIMIT ?`, wishAdminListLimit)
	if err != nil {
		return nil, nil, err
	}
	list := []wish{}
	for rows.Next() {
		item, err := scanWish(rows)
		if err != nil {
			rows.Close()
			return nil, nil, err
		}
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, nil, err
	}
	stats := map[string]int{"pending": 0, "approved": 0, "rejected": 0}
	counts, err := s.db.Query("SELECT status,COUNT(*) FROM wishes GROUP BY status")
	if err != nil {
		return nil, nil, err
	}
	defer counts.Close()
	for counts.Next() {
		var status string
		var n int
		if err := counts.Scan(&status, &n); err != nil {
			return nil, nil, err
		}
		if _, exists := stats[status]; exists {
			stats[status] = n
		}
	}
	if err := counts.Err(); err != nil {
		return nil, nil, err
	}
	return list, stats, nil
}

func (s *Server) adminWish(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	actor, ok := s.needAdmin(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	var owner, text, current string
	if err := s.db.QueryRow("SELECT user_name,text,status FROM wishes WHERE id=?", id).Scan(&owner, &text, &current); err != nil {
		fail(w, 404, "许愿不存在")
		return
	}
	var req struct {
		Action string `json:"action"`
		Note   string `json:"note"`
	}
	if !decode(w, r, &req) {
		return
	}
	status := ""
	switch req.Action {
	case "approve":
		status = "approved"
	case "reject":
		status = "rejected"
	default:
		fail(w, 400, "审核操作只能是通过或驳回")
		return
	}
	note := strings.TrimSpace(req.Note)
	if utf8.RuneCountInString(note) > wishNoteMaxRunes {
		fail(w, 400, "审核备注最多 500 字")
		return
	}
	now := time.Now().Unix()
	detail := owner + "：" + text
	if utf8.RuneCountInString(detail) > 120 {
		detail = string([]rune(detail)[:120]) + "…"
	}
	if note != "" {
		detail += " · " + note
	}
	err := s.adminCommit(actor, "wish_"+status, id, detail, nil, func(tx *sql.Tx) error {
		res, err := tx.Exec("UPDATE wishes SET status=?,review_note=?,reviewed_by=?,reviewed_by_name=?,reviewed_at=? WHERE id=?", status, note, actor.ID, actor.Name, now, id)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("wish %s disappeared", id)
		}
		return nil
	})
	if err != nil {
		fail(w, 500, "审核结果未保存，请重试")
		return
	}
	respond(w, 200, map[string]any{"ok": true})
}

func (s *Server) agentWishes(w http.ResponseWriter, r *http.Request) {
	if s.cfg.WishAgentToken == "" {
		fail(w, 503, "尚未配置许愿代理令牌")
		return
	}
	token := strings.TrimSpace(r.Header.Get("X-Wish-Token"))
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		token = strings.TrimSpace(auth[len("Bearer "):])
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.WishAgentToken)) != 1 {
		fail(w, 401, "许愿代理令牌无效")
		return
	}
	if status := r.URL.Query().Get("status"); status != "" && status != "approved" {
		fail(w, 400, "代理只读取已审核通过的许愿")
		return
	}
	var since int64
	if raw := r.URL.Query().Get("since"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v < 0 {
			fail(w, 400, "since 必须是 Unix 秒")
			return
		}
		since = v
	}
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 || v > wishAgentLimit {
			fail(w, 400, "limit 需在 1–200")
			return
		}
		limit = v
	}
	rows, err := s.db.Query(`SELECT id,user_name,text,image_url,status,review_note,created_at,reviewed_at,COALESCE(reviewed_by_name,'') FROM wishes
WHERE status='approved' AND reviewed_at>? ORDER BY reviewed_at,id LIMIT ?`, since, limit)
	if err != nil {
		fail(w, 500, "无法读取已通过的许愿")
		return
	}
	defer rows.Close()
	list := []wish{}
	for rows.Next() {
		item, err := scanWish(rows)
		if err != nil {
			fail(w, 500, "无法读取已通过的许愿")
			return
		}
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		fail(w, 500, "无法读取已通过的许愿")
		return
	}
	respond(w, 200, map[string]any{"wishes": list, "serverNow": time.Now().Unix()})
}
