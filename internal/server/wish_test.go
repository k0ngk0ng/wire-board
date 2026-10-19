package server

// Failure scenarios covered here before implementation:
// - anonymous or disabled users submit/read wishes; unknown JSON fields accepted
// - empty/oversized wish text or oversized/spoofed/SVG uploads reach OSS
// - OSS failure creates a wish row or exposes credentials/signature in responses
// - non-admins review wishes; review updates are not audited
// - rejected/pending wishes leak into the agent feed; a missing/wrong token works
// - restart loses wishes; approved images are not stable public URLs

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
)

type ossPut struct {
	method      string
	path        string
	contentType string
	md5         string
	forbid      string
	body        []byte
}

type fakeOSS struct {
	mu   sync.Mutex
	puts []ossPut
	fail bool
}

func (f *fakeOSS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.puts = append(f.puts, ossPut{
		method: r.Method, path: r.URL.EscapedPath(), contentType: r.Header.Get("Content-Type"),
		md5: r.Header.Get("Content-MD5"), forbid: r.Header.Get("x-oss-forbid-overwrite"), body: body,
	})
	if f.fail {
		http.Error(w, "oss unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (f *fakeOSS) recorded() []ossPut {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]ossPut(nil), f.puts...)
}

type wishFixture struct {
	s   *Server
	ts  *httptest.Server
	cfg *Config
	oss *fakeOSS
}

func newWishFixture(t *testing.T, edit func(*Config)) *wishFixture {
	t.Helper()
	oss := &fakeOSS{}
	ossServer := httptest.NewServer(oss)
	t.Cleanup(ossServer.Close)
	cfg := Config{
		DataDir: t.TempDir(), InviteCode: "test-invite",
		AdminUsername: "WishRoot", AdminPassword: "wish-root-password-123",
		OSSEndpoint: ossServer.URL, OSSBucket: "test-bucket",
		OSSAccessKeyID: "test-key", OSSAccessKeySecret: "test-secret",
		WishImageBaseURL: "https://cdn.example.test", WishImagePrefix: "bga/wishes",
		WishAgentToken: "agent-token-for-wishes-only",
	}
	if edit != nil {
		edit(&cfg)
	}
	s, err := New(cfg, fstest.MapFS{"index.html": {Data: []byte("Wire Board")}})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); s.Close() })
	return &wishFixture{s: s, ts: ts, cfg: &cfg, oss: oss}
}

func multipartWish(t *testing.T, text, filename string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := mw.WriteField("text", text); err != nil {
		t.Fatal(err)
	}
	if filename != "" {
		part, err := mw.CreateFormFile("image", filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = part.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return &body, mw.FormDataContentType()
}

func (c *testClient) wishMultipart(t *testing.T, body io.Reader, contentType string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest("POST", c.base+"/api/wishes", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", contentType)
	res, err := c.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var v map[string]any
	if err = json.NewDecoder(res.Body).Decode(&v); err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, v
}

func (c *testClient) wishes(t *testing.T) []any {
	t.Helper()
	code, v := c.request("GET", "/api/wishes", nil)
	if code != 200 {
		t.Fatalf("GET /api/wishes got %d: %v", code, v)
	}
	list, _ := v["wishes"].([]any)
	return list
}

func agentFeed(t *testing.T, base, query, token string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest("GET", base+"/api/agent/wishes"+query, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var feed map[string]any
	if err = json.NewDecoder(res.Body).Decode(&feed); err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, feed
}

func agentPost(t *testing.T, base, path, token string, body any) (int, map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", base+path, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

func wishID(t *testing.T, v map[string]any) string {
	t.Helper()
	w, _ := v["wish"].(map[string]any)
	id, _ := w["id"].(string)
	if id == "" {
		t.Fatalf("missing wish id: %v", v)
	}
	return id
}

var onePixelPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00,
	0x0d, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0xf8, 0xcf, 0xc0, 0xf0,
	0x1f, 0x00, 0x05, 0x00, 0x01, 0xff, 0x89, 0x99, 0x3d, 0x1d, 0x00, 0x00,
	0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

func TestWishesRequireAuthAndValidateText(t *testing.T) {
	f := newWishFixture(t, nil)
	anon := newClient(t, f.ts.URL)
	if code, _ := anon.request("GET", "/api/wishes", nil); code != 401 {
		t.Fatalf("anonymous read got %d", code)
	}
	if code, _ := anon.request("POST", "/api/wishes", map[string]any{"text": "想做一款潜水艇游戏"}); code != 401 {
		t.Fatalf("anonymous submit got %d", code)
	}

	alice := newClient(t, f.ts.URL)
	alice.register("WishAlice")
	for _, text := range []string{"", " ", "短"} {
		if code, _ := alice.request("POST", "/api/wishes", map[string]any{"text": text}); code != 400 {
			t.Fatalf("text %q got %d", text, code)
		}
	}
	if code, _ := alice.request("POST", "/api/wishes", map[string]any{"text": strings.Repeat("愿", 2001)}); code != 400 {
		t.Fatal("oversized text accepted")
	}
	if code, _ := alice.request("POST", "/api/wishes", map[string]any{"text": "想做一款潜水艇游戏", "position": "ignored"}); code != 400 {
		t.Fatal("unknown field accepted")
	}
	v := alice.post("/api/wishes", map[string]any{"text": "  想做一款双人潜水艇游戏\n希望支持手机  "}, 200)
	id := wishID(t, v)
	w := v["wish"].(map[string]any)
	if w["status"] != "pending" || w["text"] != "想做一款双人潜水艇游戏\n希望支持手机" || w["imageUrl"] != "" {
		t.Fatalf("bad created wish: %v", w)
	}
	if w["userName"] != "WishAlice" {
		t.Fatalf("missing submitter: %v", w)
	}
	list := alice.wishes(t)
	if len(list) != 1 || list[0].(map[string]any)["id"] != id {
		t.Fatalf("own list: %v", list)
	}

	bob := newClient(t, f.ts.URL)
	bob.register("WishBob")
	bob.post("/api/wishes", map[string]any{"text": "想要一款六人航海游戏"}, 200)
	if len(alice.wishes(t)) != 1 || len(bob.wishes(t)) != 1 {
		t.Fatal("wish list leaked between users")
	}
}

func TestWishImagesUploadToOSSAndRejectUnsafeInput(t *testing.T) {
	f := newWishFixture(t, nil)
	alice := newClient(t, f.ts.URL)
	alice.register("WishImage")

	body, contentType := multipartWish(t, "想做一款潜艇对战，附图是参考", "submarine.png", onePixelPNG)
	code, v := alice.wishMultipart(t, body, contentType)
	if code != 200 {
		t.Fatalf("upload got %d: %v", code, v)
	}
	w := v["wish"].(map[string]any)
	url, _ := w["imageUrl"].(string)
	if !strings.HasPrefix(url, "https://cdn.example.test/bga/wishes/") || !strings.HasSuffix(url, ".png") {
		t.Fatalf("bad public image URL: %q", url)
	}
	if strings.Contains(url, "test-secret") || strings.Contains(url, "Signature") {
		t.Fatal("secret leaked into image URL")
	}

	puts := f.oss.recorded()
	if len(puts) != 1 {
		t.Fatalf("expected one OSS PUT, got %d", len(puts))
	}
	put := puts[0]
	if put.method != "PUT" || put.forbid != "true" || put.contentType != "image/png" {
		t.Fatalf("bad OSS request: %+v", put)
	}
	sum := sha256.Sum256(onePixelPNG)
	if len(put.body) != len(onePixelPNG) || sha256.Sum256(put.body) != sum {
		t.Fatal("OSS body changed")
	}
	if put.md5 != base64.StdEncoding.EncodeToString(md5Sum(onePixelPNG)) {
		t.Fatal("missing content MD5")
	}
	if !strings.HasPrefix(put.path, "/bga/wishes/") {
		t.Fatalf("bad object key: %s", put.path)
	}
	if list := alice.wishes(t); len(list) != 1 || list[0].(map[string]any)["imageUrl"] != url {
		t.Fatalf("image URL not persisted: %v", list)
	}

	// SVG is executable active content on the CDN and must never be accepted.
	body, contentType = multipartWish(t, "参考图", "x.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`))
	if code, _ := alice.wishMultipart(t, body, contentType); code != 400 {
		t.Fatalf("SVG accepted: %d", code)
	}
	// File extension must not override sniffed content.
	body, contentType = multipartWish(t, "伪装图", "x.png", []byte("not an image"))
	if code, _ := alice.wishMultipart(t, body, contentType); code != 400 {
		t.Fatalf("spoofed MIME accepted: %d", code)
	}
	// Oversized image is rejected before any OSS write.
	body, contentType = multipartWish(t, "太大", "big.png", append(append([]byte{}, onePixelPNG...), bytes.Repeat([]byte{0}, 5<<20)...))
	if code, _ := alice.wishMultipart(t, body, contentType); code != 413 {
		t.Fatalf("oversized upload got %d", code)
	}

	// A failed object write must not leave a visible wish.
	f.oss.fail = true
	body, contentType = multipartWish(t, "OSS 故障也要原子", "ok.png", onePixelPNG)
	if code, _ := alice.wishMultipart(t, body, contentType); code != 502 {
		t.Fatalf("OSS failure got %d", code)
	}
	if len(alice.wishes(t)) != 1 {
		t.Fatal("failed upload created a wish")
	}
}

func md5Sum(b []byte) []byte { sum := md5.Sum(b); return sum[:] }

// Reference signatures were generated with the same V1 canonicalization used by
// the existing Python asset publisher, so a refactor cannot silently break the
// production OSS upload.
func TestOSSSignatureReferenceVectors(t *testing.T) {
	cfg := Config{OSSBucket: "test-bucket", OSSAccessKeyID: "test-key", OSSAccessKeySecret: "test-secret"}
	key := "bga/wishes/20261019/abc.png"
	date := "Mon, 19 Oct 2026 07:00:00 GMT"
	put := ossAuthorization(cfg, "PUT", key, "image/png", "q2hlY2s=", date, map[string]string{"x-oss-forbid-overwrite": "true"})
	if put != "OSS test-key:nGk+c5FRaqPNcUI05JcgMzcUPrY=" {
		t.Fatalf("PUT signature changed: %s", put)
	}
	del := ossAuthorization(cfg, "DELETE", key, "", "", date, nil)
	if del != "OSS test-key:0Dz2on7hyiPlJi4QmNbvxS45Xo0=" {
		t.Fatalf("DELETE signature changed: %s", del)
	}
}

func TestWishReviewAuditAndAgentFeed(t *testing.T) {
	f := newWishFixture(t, nil)
	root := newClient(t, f.ts.URL)
	root.post("/api/login", map[string]string{"name": "WishRoot", "password": "wish-root-password-123"}, 200)
	alice := newClient(t, f.ts.URL)
	alice.register("WishReviewer")
	first := wishID(t, alice.post("/api/wishes", map[string]any{"text": "想做一款双人潜艇游戏"}, 200))
	second := wishID(t, alice.post("/api/wishes", map[string]any{"text": "想做一款农场经营游戏"}, 200))

	data := overview(t, root)
	if len(data["wishes"].([]any)) != 2 || data["wishStats"].(map[string]any)["pending"] != float64(2) {
		t.Fatalf("admin overview missing wishes: %v", data)
	}
	alice.post("/api/admin/wishes/"+first, map[string]string{"action": "approve"}, 403)

	root.post("/api/admin/wishes/"+first, map[string]string{"action": "approve", "note": "适合排期"}, 200)
	root.post("/api/admin/wishes/"+second, map[string]string{"action": "reject", "note": "暂不考虑经营类"}, 200)
	data = overview(t, root)
	stats := data["wishStats"].(map[string]any)
	if stats["pending"] != float64(0) || stats["approved"] != float64(1) || stats["in_progress"] != float64(0) ||
		stats["implemented"] != float64(0) || stats["rejected"] != float64(1) {
		t.Fatalf("bad wish stats after review: %v", stats)
	}
	own := alice.wishes(t)
	statuses := map[string]string{}
	for _, item := range own {
		w := item.(map[string]any)
		statuses[w["id"].(string)] = w["status"].(string)
	}
	if statuses[first] != "approved" || statuses[second] != "rejected" {
		t.Fatalf("decision not reflected in owner list: %v", statuses)
	}

	if code, _ := agentFeed(t, f.ts.URL, "", ""); code != 401 {
		t.Fatalf("missing agent token got %d", code)
	}
	if code, _ := agentFeed(t, f.ts.URL, "", "wrong-token"); code != 401 {
		t.Fatalf("wrong agent token got %d", code)
	}
	code, feed := agentFeed(t, f.ts.URL, "?status=approved", "agent-token-for-wishes-only")
	if code != 200 || len(feed["wishes"].([]any)) != 1 {
		t.Fatalf("approved agent feed: %d %v", code, feed)
	}
	approved := feed["wishes"].([]any)[0].(map[string]any)
	if approved["id"] != first || approved["text"] != "想做一款双人潜艇游戏" || approved["note"] != "适合排期" ||
		approved["status"] != "approved" {
		t.Fatalf("bad approved item: %v", approved)
	}
	if int64(approved["approvedAt"].(float64)) <= 0 {
		t.Fatalf("missing approval time: %v", approved)
	}

	root.post("/api/admin/wishes/"+first, map[string]string{"action": "start", "note": "本周开工"}, 200)
	data = overview(t, root)
	stats = data["wishStats"].(map[string]any)
	if stats["approved"] != float64(0) || stats["in_progress"] != float64(1) || stats["implemented"] != float64(0) {
		t.Fatalf("wish did not become in_progress: %v", stats)
	}
	code, feed = agentFeed(t, f.ts.URL, "?status=in_progress", "agent-token-for-wishes-only")
	if code != 200 || len(feed["wishes"].([]any)) != 1 {
		t.Fatalf("in-progress agent feed: %d %v", code, feed)
	}

	root.post("/api/admin/wishes/"+first, map[string]string{"action": "implement", "note": "已上线"}, 200)
	data = overview(t, root)
	stats = data["wishStats"].(map[string]any)
	if stats["in_progress"] != float64(0) || stats["implemented"] != float64(1) {
		t.Fatalf("wish did not become implemented: %v", stats)
	}
	if code, feed = agentFeed(t, f.ts.URL, "?status=approved", "agent-token-for-wishes-only"); code != 200 || len(feed["wishes"].([]any)) != 0 {
		t.Fatalf("implemented wish still approved: %d %v", code, feed)
	}
	code, feed = agentFeed(t, f.ts.URL, "", "agent-token-for-wishes-only")
	if code != 200 || len(feed["wishes"].([]any)) != 1 {
		t.Fatalf("default agent feed: %d %v", code, feed)
	}
	implemented := feed["wishes"].([]any)[0].(map[string]any)
	if implemented["status"] != "implemented" || implemented["note"] != "已上线" {
		t.Fatalf("bad implemented item: %v", implemented)
	}
	reviewedAt := int64(implemented["reviewedAt"].(float64))
	if reviewedAt <= 0 {
		t.Fatalf("missing review time: %v", implemented)
	}

	own = alice.wishes(t)
	statuses = map[string]string{}
	for _, item := range own {
		w := item.(map[string]any)
		statuses[w["id"].(string)] = w["status"].(string)
	}
	if statuses[first] != "implemented" || statuses[second] != "rejected" {
		t.Fatalf("final status not reflected in owner list: %v", statuses)
	}

	entries := []any{}
	code, audit := root.request("GET", "/api/admin/audit", nil)
	if code != 200 {
		t.Fatal(code, audit)
	}
	for _, raw := range audit["entries"].([]any) {
		entry := raw.(map[string]any)
		if strings.HasPrefix(entry["action"].(string), "wish_") {
			entries = append(entries, entry)
		}
	}
	if len(entries) != 4 {
		t.Fatalf("wish transitions not audited: %v", audit)
	}

	// Polling after the latest transition returns nothing; since is exclusive.
	code, feed = agentFeed(t, f.ts.URL, "?since="+strconv.FormatInt(reviewedAt, 10), "agent-token-for-wishes-only")
	if code != 200 || len(feed["wishes"].([]any)) != 0 {
		t.Fatalf("since did not filter: %d %v", code, feed)
	}
	if f.s.cfg.WishAgentToken != "agent-token-for-wishes-only" {
		t.Fatal("agent token not loaded")
	}
}

func TestAgentCanAdvanceApprovedWish(t *testing.T) {
	f := newWishFixture(t, nil)
	root := newClient(t, f.ts.URL)
	root.post("/api/login", map[string]string{"name": "WishRoot", "password": "wish-root-password-123"}, 200)
	alice := newClient(t, f.ts.URL)
	alice.register("WishAgentFlow")
	id := wishID(t, alice.post("/api/wishes", map[string]any{"text": "想做一款深海采矿游戏"}, 200))
	root.post("/api/admin/wishes/"+id, map[string]string{"action": "approve", "note": "可以开工"}, 200)

	call := func(token string, body map[string]any) (int, map[string]any) {
		t.Helper()
		raw, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", f.ts.URL+"/api/agent/wishes/"+id, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(res.Body).Decode(&out)
		return res.StatusCode, out
	}
	if code, _ := call("", map[string]any{"action": "start"}); code != 401 {
		t.Fatalf("anonymous transition got %d", code)
	}
	if code, _ := call("wrong-token", map[string]any{"action": "start"}); code != 401 {
		t.Fatalf("wrong token transition got %d", code)
	}
	if code, out := call("agent-token-for-wishes-only", map[string]any{"action": "start", "note": "代理已领取"}); code != 200 || out["status"] != "in_progress" {
		t.Fatalf("start transition: %d %v", code, out)
	}
	if code, feed := agentFeed(t, f.ts.URL, "?status=in_progress", "agent-token-for-wishes-only"); code != 200 || len(feed["wishes"].([]any)) != 1 {
		t.Fatalf("in-progress feed after start: %d %v", code, feed)
	}
	if code, out := call("agent-token-for-wishes-only", map[string]any{"action": "implement", "note": "已完成并部署"}); code != 200 || out["status"] != "implemented" {
		t.Fatalf("implement transition: %d %v", code, out)
	}
	if code, _ := call("agent-token-for-wishes-only", map[string]any{"action": "implement"}); code != 409 {
		t.Fatalf("repeat implement got %d", code)
	}
	if code, _ := call("agent-token-for-wishes-only", map[string]any{"action": "start"}); code != 409 {
		t.Fatalf("start after implement got %d", code)
	}
	feedCode, feed := agentFeed(t, f.ts.URL, "?status=implemented", "agent-token-for-wishes-only")
	if feedCode != 200 || len(feed["wishes"].([]any)) != 1 || feed["wishes"].([]any)[0].(map[string]any)["note"] != "已完成并部署" {
		t.Fatalf("implemented feed after agent transition: %d %v", feedCode, feed)
	}
	code, audit := root.request("GET", "/api/admin/audit", nil)
	if code != 200 {
		t.Fatal(code, audit)
	}
	seen := map[string]string{}
	for _, raw := range audit["entries"].([]any) {
		entry := raw.(map[string]any)
		if strings.HasPrefix(entry["action"].(string), "wish_") {
			seen[entry["action"].(string)] = entry["actor"].(string)
		}
	}
	if seen["wish_in_progress"] != "本地制作代理" || seen["wish_implemented"] != "本地制作代理" {
		t.Fatalf("agent transitions not audited: %v", seen)
	}
}

func TestWishClarificationQuestionsAndPendingStart(t *testing.T) {
	f := newWishFixture(t, nil)
	alice := newClient(t, f.ts.URL)
	alice.register("WishClarify")
	id := wishID(t, alice.post("/api/wishes", map[string]any{"text": "想做一个节奏游戏，但还没想好几人玩"}, 200))
	path := "/api/agent/wishes/" + id + "/questions"

	if code, _ := agentPost(t, f.ts.URL, path, "", map[string]any{"question": "想要几人模式？"}); code != 401 {
		t.Fatalf("anonymous question got %d", code)
	}
	if code, _ := agentPost(t, f.ts.URL, path, "agent-token-for-wishes-only", map[string]any{"question": ""}); code != 400 {
		t.Fatalf("empty question got %d", code)
	}
	if code, out := agentPost(t, f.ts.URL, path, "agent-token-for-wishes-only", map[string]any{"question": "更希望单人还是多人？"}); code != 200 || out["questionCount"] != float64(1) {
		t.Fatalf("first question: %d %v", code, out)
	}
	code, feed := agentFeed(t, f.ts.URL, "?status=pending", "agent-token-for-wishes-only")
	if code != 200 || len(feed["wishes"].([]any)) != 1 {
		t.Fatalf("pending feed: %d %v", code, feed)
	}
	item := feed["wishes"].([]any)[0].(map[string]any)
	if item["needsAnswer"] != true || item["questionCount"] != float64(1) {
		t.Fatalf("pending feed missing question: %v", item)
	}
	if qs := item["questions"].([]any); len(qs) != 1 || qs[0].(map[string]any)["question"] != "更希望单人还是多人？" {
		t.Fatalf("bad questions: %v", qs)
	}
	if code, _ := agentPost(t, f.ts.URL, "/api/agent/wishes/"+id, "agent-token-for-wishes-only", map[string]any{"action": "start"}); code != 409 {
		t.Fatalf("start with unanswered question got %d", code)
	}
	if code, _ := agentPost(t, f.ts.URL, path, "agent-token-for-wishes-only", map[string]any{"question": "再问一次？"}); code != 409 {
		t.Fatalf("second unanswered question got %d", code)
	}

	bob := newClient(t, f.ts.URL)
	bob.register("WishClarifyOther")
	if code, _ := bob.request("POST", "/api/wishes/"+id+"/answer", map[string]any{"answer": "不该允许"}); code != 403 {
		t.Fatalf("other user answered: %d", code)
	}
	if code, out := alice.request("POST", "/api/wishes/"+id+"/answer", map[string]any{"answer": "希望 2 人合作"}); code != 200 {
		t.Fatalf("answer: %d %v", code, out)
	}
	code, feed = agentFeed(t, f.ts.URL, "?status=pending", "agent-token-for-wishes-only")
	item = feed["wishes"].([]any)[0].(map[string]any)
	if item["needsAnswer"] != false || item["questions"].([]any)[0].(map[string]any)["answer"] != "希望 2 人合作" {
		t.Fatalf("answer not in feed: %v", item)
	}
	if code, out := agentPost(t, f.ts.URL, path, "agent-token-for-wishes-only", map[string]any{"question": "希望有剧情吗？"}); code != 200 || out["questionCount"] != float64(2) {
		t.Fatalf("second question: %d %v", code, out)
	}
	if code, _ := alice.request("POST", "/api/wishes/"+id+"/answer", map[string]any{"answer": "不需要"}); code != 200 {
		t.Fatal("second answer failed")
	}
	if code, _ := agentPost(t, f.ts.URL, path, "agent-token-for-wishes-only", map[string]any{"question": "第三次？"}); code != 409 {
		t.Fatalf("third question got %d", code)
	}
	if code, out := agentPost(t, f.ts.URL, "/api/agent/wishes/"+id, "agent-token-for-wishes-only", map[string]any{"action": "start", "note": "问题已问完，开始实现"}); code != 200 || out["status"] != "in_progress" {
		t.Fatalf("pending start after questions: %d %v", code, out)
	}
	list := alice.wishes(t)
	item = list[0].(map[string]any)
	if item["status"] != "in_progress" || len(item["questions"].([]any)) != 2 || item["questions"].([]any)[1].(map[string]any)["answer"] != "不需要" {
		t.Fatalf("owner view missing Q&A: %v", item)
	}

	admin := newClient(t, f.ts.URL)
	admin.post("/api/login", map[string]string{"name": "WishRoot", "password": "wish-root-password-123"}, 200)
	data := overview(t, admin)
	card := data["wishes"].([]any)[0].(map[string]any)
	if len(card["questions"].([]any)) != 2 || card["questionCount"] != float64(2) {
		t.Fatalf("admin view missing Q&A: %v", card)
	}
	code, audit := admin.request("GET", "/api/admin/audit", nil)
	if code != 200 {
		t.Fatal(code, audit)
	}
	questions := 0
	for _, raw := range audit["entries"].([]any) {
		entry := raw.(map[string]any)
		if entry["action"] == "wish_question" {
			questions++
		}
	}
	if questions != 2 {
		t.Fatalf("question audit missing: %v", audit)
	}
}

func TestAgentFeedDisabledAndStatusGuard(t *testing.T) {
	f := newWishFixture(t, func(cfg *Config) { cfg.WishAgentToken = "" })
	req, _ := http.NewRequest("GET", f.ts.URL+"/api/agent/wishes", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 503 {
		t.Fatalf("disabled agent feed got %d", res.StatusCode)
	}
	f = newWishFixture(t, nil)
	if code, _ := agentFeed(t, f.ts.URL, "?status=pending", "agent-token-for-wishes-only"); code != 200 {
		t.Fatalf("pending agent feed got %d", code)
	}
	if code, _ := agentFeed(t, f.ts.URL, "?status=rejected", "agent-token-for-wishes-only"); code != 400 {
		t.Fatalf("rejected agent feed got %d", code)
	}
}

func TestWishesPersistAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	files := fstest.MapFS{"index.html": {Data: []byte("Wire Board")}}
	cfg := Config{DataDir: dir, InviteCode: "test-invite"}
	s, err := New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	c := newClient(t, ts.URL)
	c.register("WishPersist")
	c.post("/api/wishes", map[string]any{"text": "重启后仍要保留的愿望"}, 200)
	ts.Close()
	s.Close()

	s, err = New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ts = httptest.NewServer(s.Handler())
	defer ts.Close()
	c = newClient(t, ts.URL)
	c.post("/api/login", map[string]string{"name": "WishPersist", "password": "test-password-123"}, 200)
	if list := c.wishes(t); len(list) != 1 || list[0].(map[string]any)["text"] != "重启后仍要保留的愿望" {
		t.Fatalf("wish lost after restart: %v", list)
	}
}

func TestWishRateLimitAndUnconfiguredUpload(t *testing.T) {
	f := newWishFixture(t, func(cfg *Config) {
		cfg.OSSEndpoint, cfg.OSSBucket, cfg.OSSAccessKeyID, cfg.OSSAccessKeySecret = "", "", "", ""
	})
	c := newClient(t, f.ts.URL)
	c.register("WishLimits")
	body, contentType := multipartWish(t, "没有素材服务时也要友好报错", "x.png", onePixelPNG)
	if code, _ := c.wishMultipart(t, body, contentType); code != 503 {
		t.Fatalf("unconfigured upload got %d", code)
	}
	for i := 0; i < 10; i++ {
		c.post("/api/wishes", map[string]any{"text": "第" + strconv.Itoa(i) + "个愿望"}, 200)
	}
	if code, _ := c.request("POST", "/api/wishes", map[string]any{"text": "超额愿望"}); code != 429 {
		t.Fatalf("rate limit got %d", code)
	}
}
