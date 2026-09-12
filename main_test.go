package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	db, err := newDB(t.TempDir() + "/data.json")
	if err != nil {
		t.Fatalf("newDB: %v", err)
	}
	s := &Server{db: db, sess: newSessionStore(), tmpl: parseTemplates()}
	return s
}

func newClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func apiReq(t *testing.T, client *http.Client, method, path string, body []byte) map[string]any {
	t.Helper()
	req, err := http.NewRequest(method, path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode %s %s: %v", method, path, err)
	}
	return out
}

func TestLoginRequired(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s.routes())
	defer ts.Close()

	client := newClient()
	resp, err := client.Get(ts.URL + "/dashboard")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}
	loc, _ := resp.Location()
	if loc == nil || !strings.Contains(loc.String(), "/login") {
		t.Fatalf("expected /login redirect, got %v", loc)
	}
}

func TestLoginAndPages(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s.routes())
	defer ts.Close()

	client := newClient()

	// wrong password
	form := url.Values{"username": {"admin"}, "password": {"wrong"}}
	resp, err := client.PostForm(ts.URL+"/login", form)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := readBody(resp)
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, "用户名或密码错误") {
		t.Fatalf("expected login error, got %d %s", resp.StatusCode, body)
	}

	// correct login
	form = url.Values{"username": {"admin"}, "password": {"admin123"}}
	resp, err = client.PostForm(ts.URL+"/login", form)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("login failed: %d", resp.StatusCode)
	}
	if len(resp.Cookies()) == 0 {
		t.Fatal("no session cookie set")
	}

	// all pages render
	for _, p := range []string{"/dashboard", "/users", "/plans", "/nodes", "/subscriptions", "/traffic", "/logs", "/settings"} {
		r, err := client.Get(ts.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		page, _ := readBody(r)
		if r.StatusCode != http.StatusOK {
			t.Fatalf("%s -> %d", p, r.StatusCode)
		}
		if !strings.Contains(page, "VPN 管理面板") {
			t.Fatalf("%s missing site name", p)
		}
	}
}

func TestAPIFlow(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s.routes())
	defer ts.Close()

	client := newClient()
	resp, err := client.PostForm(ts.URL+"/login", url.Values{"username": {"admin"}, "password": {"admin123"}})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// create plan
	body, _ := json.Marshal(map[string]any{"name": "测试套餐", "traffic_gb": 100, "duration_days": 60, "price": 25.5, "device_limit": 4, "enabled": 1})
	out := apiReq(t, client, "POST", ts.URL+"/api/plans", body)
	if out["success"] != true {
		t.Fatalf("create plan: %v", out)
	}

	// create user
	body, _ = json.Marshal(map[string]any{"email": "test@example.com", "protocol": "vless", "enabled": 1})
	out = apiReq(t, client, "POST", ts.URL+"/api/users", body)
	if out["success"] != true {
		t.Fatalf("create user: %v", out)
	}

	// duplicate email rejected
	out = apiReq(t, client, "POST", ts.URL+"/api/users", body)
	if out["success"] != false {
		t.Fatalf("duplicate user should fail: %v", out)
	}

	// config api
	cfg := apiReq(t, client, "GET", ts.URL+"/api/users/1/config", nil)
	links, ok := cfg["links"].([]any)
	if !ok || len(links) == 0 || !strings.HasPrefix(links[0].(string), "vless://") {
		t.Fatalf("bad config: %v", cfg)
	}

	// subscription api (plain text)
	r, err := client.Get(ts.URL + "/api/subscription/1")
	if err != nil {
		t.Fatal(err)
	}
	sub, _ := readBody(r)
	if r.StatusCode != 200 || sub == "" {
		t.Fatalf("subscription failed: %d", r.StatusCode)
	}

	// traffic simulation
	out = apiReq(t, client, "POST", ts.URL+"/api/traffic", nil)
	if out["success"] != true {
		t.Fatalf("traffic: %v", out)
	}

	// reset traffic
	out = apiReq(t, client, "POST", ts.URL+"/api/users/1/reset-traffic", nil)
	if out["success"] != true {
		t.Fatalf("reset traffic: %v", out)
	}

	// settings
	body, _ = json.Marshal(map[string]any{"site_name": "测试面板", "sub_domain": "sub.example.com"})
	out = apiReq(t, client, "POST", ts.URL+"/api/settings", body)
	if out["success"] != true {
		t.Fatalf("settings: %v", out)
	}
	if s.db.getSetting("site_name", "") != "测试面板" {
		t.Fatal("site_name not saved")
	}

	// update user
	body, _ = json.Marshal(map[string]any{"traffic_gb": 500, "enabled": 1})
	out = apiReq(t, client, "PUT", ts.URL+"/api/users/1", body)
	if out["success"] != true {
		t.Fatalf("update user: %v", out)
	}

	// subscription all
	out = apiReq(t, client, "GET", ts.URL+"/api/subscription/all", nil)
	if out["success"] != true {
		t.Fatalf("subscription all: %v", out)
	}

	// cleanup
	out = apiReq(t, client, "DELETE", ts.URL+"/api/users/1", nil)
	if out["success"] != true {
		t.Fatalf("delete user: %v", out)
	}
	out = apiReq(t, client, "DELETE", ts.URL+"/api/plans/4", nil)
	if out["success"] != true {
		t.Fatalf("delete plan: %v", out)
	}
	out = apiReq(t, client, "DELETE", ts.URL+"/api/nodes/1", nil)
	if out["success"] != true {
		t.Fatalf("delete node: %v", out)
	}
}

func TestCertAndTLS(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s.routes())
	defer ts.Close()

	client := newClient()
	resp, err := client.PostForm(ts.URL+"/login", url.Values{"username": {"admin"}, "password": {"admin123"}})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// self-signed cert generation
	body, _ := json.Marshal(map[string]any{"hosts": "example.com,127.0.0.1"})
	out := apiReq(t, client, "POST", ts.URL+"/api/cert/generate", body)
	if out["success"] != true {
		t.Fatalf("generate cert: %v", out)
	}
	cert, _ := out["cert"].(string)
	key, _ := out["key"].(string)
	if !strings.Contains(cert, "BEGIN CERTIFICATE") || !strings.Contains(key, "PRIVATE KEY") {
		t.Fatal("generated cert/key invalid")
	}

	// enable TLS with the generated pair
	body, _ = json.Marshal(map[string]any{
		"tls_enabled": "1", "tls_cert": cert, "tls_key": key, "tls_redirect_http": "1",
	})
	out = apiReq(t, client, "POST", ts.URL+"/api/settings", body)
	if out["success"] != true {
		t.Fatalf("enable tls: %v", out)
	}

	// invalid cert must be rejected
	body, _ = json.Marshal(map[string]any{"tls_cert": "not-a-cert"})
	out = apiReq(t, client, "POST", ts.URL+"/api/settings", body)
	if out["success"] != false {
		t.Fatalf("invalid cert should fail: %v", out)
	}

	// enabling without key must be rejected
	body, _ = json.Marshal(map[string]any{"tls_enabled": "1", "tls_cert": cert, "tls_key": ""})
	out = apiReq(t, client, "POST", ts.URL+"/api/settings", body)
	if out["success"] != false {
		t.Fatalf("enable tls without key should fail: %v", out)
	}

	// disable again (cleanup)
	body, _ = json.Marshal(map[string]any{"tls_enabled": "0"})
	out = apiReq(t, client, "POST", ts.URL+"/api/settings", body)
	if out["success"] != true {
		t.Fatalf("disable tls: %v", out)
	}
}

func TestSubscriptionByToken(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s.routes())
	defer ts.Close()

	client := newClient()
	resp, err := client.PostForm(ts.URL+"/login", url.Values{"username": {"admin"}, "password": {"admin123"}})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	body, _ := json.Marshal(map[string]any{"email": "sub@test.com", "protocol": "vless", "enabled": 1})
	out := apiReq(t, client, "POST", ts.URL+"/api/users", body)
	if out["success"] != true {
		t.Fatalf("create user: %v", out)
	}

	s.db.mu.RLock()
	token := s.db.Data.Users[0].Token
	s.db.mu.RUnlock()

	// 公开访问 (无登录 Cookie): 正常返回订阅
	anon := newClient()
	r, err := anon.Get(ts.URL + "/sub/" + token)
	if err != nil {
		t.Fatal(err)
	}
	content, _ := readBody(r)
	if r.StatusCode != http.StatusOK || content == "" {
		t.Fatalf("public sub failed: %d", r.StatusCode)
	}
	if ui := r.Header.Get("subscription-userinfo"); !strings.Contains(ui, "total=") {
		t.Fatalf("missing userinfo header: %q", ui)
	}

	// 错误 token -> 404
	r, _ = anon.Get(ts.URL + "/sub/nonexistent")
	r.Body.Close()
	if r.StatusCode != http.StatusNotFound {
		t.Fatalf("bad token should 404, got %d", r.StatusCode)
	}

	// 禁用用户 -> 404
	body, _ = json.Marshal(map[string]any{"enabled": 0})
	out = apiReq(t, client, "PUT", ts.URL+"/api/users/1", body)
	if out["success"] != true {
		t.Fatalf("disable user: %v", out)
	}
	r, _ = anon.Get(ts.URL + "/sub/" + token)
	r.Body.Close()
	if r.StatusCode != http.StatusNotFound {
		t.Fatalf("disabled user should 404, got %d", r.StatusCode)
	}
}

func TestPanelPortSetting(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s.routes())
	defer ts.Close()

	client := newClient()
	resp, err := client.PostForm(ts.URL+"/login", url.Values{"username": {"admin"}, "password": {"admin123"}})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// valid port
	body, _ := json.Marshal(map[string]any{"panel_port": "8443"})
	out := apiReq(t, client, "POST", ts.URL+"/api/settings", body)
	if out["success"] != true {
		t.Fatalf("save port: %v", out)
	}
	if s.db.getSetting("panel_port", "") != "8443" {
		t.Fatal("panel_port not saved")
	}

	// invalid ports rejected
	for _, bad := range []string{"0", "65536", "abc", "-1"} {
		body, _ = json.Marshal(map[string]any{"panel_port": bad})
		out = apiReq(t, client, "POST", ts.URL+"/api/settings", body)
		if out["success"] != false {
			t.Fatalf("port %s should fail: %v", bad, out)
		}
	}

	// empty port clears (fallback to env)
	body, _ = json.Marshal(map[string]any{"panel_port": ""})
	out = apiReq(t, client, "POST", ts.URL+"/api/settings", body)
	if out["success"] != true || s.db.getSetting("panel_port", "") != "" {
		t.Fatalf("clear port: %v", out)
	}
}

func TestGenerateSelfSigned(t *testing.T) {
	cert, key, err := generateSelfSigned("")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cert, "BEGIN CERTIFICATE") || !strings.Contains(key, "BEGIN RSA PRIVATE KEY") {
		t.Fatal("missing PEM markers")
	}
	if _, err := tls.X509KeyPair([]byte(cert), []byte(key)); err != nil {
		t.Fatalf("generated pair invalid: %v", err)
	}
}

func readBody(r *http.Response) (string, error) {
	defer r.Body.Close()
	var buf bytes.Buffer
	_, err := buf.ReadFrom(r.Body)
	return buf.String(), err
}
