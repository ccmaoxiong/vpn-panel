package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed templates static
var webFS embed.FS

type UserView struct {
	User
	PlanName string
	Expired  bool
}

type PlanView struct {
	Plan
	UserCount int
}

type TrafficLogView struct {
	TrafficLog
	Email string
}

type PageData struct {
	Title        string
	SiteName     string
	Admin        *Admin
	Active       string
	ContentBlock string
	ExtraScript  template.JS
	LoginError   string

	// dashboard
	TotalUsers  int
	ActiveUsers int
	TotalNodes  int
	UsedBytes   int64
	TodayBytes  int64
	RecentUsers []UserView
	RecentLogs  []OpLog

	// users / subscriptions / traffic
	Users []UserView
	Plans []PlanView
	Nodes []Node

	// subscriptions
	Subs      map[int]string
	SubDomain string

	// traffic
	TrafficLogs []TrafficLogView

	// logs
	Logs []OpLog

	// settings
	Settings map[string]string
}

type Server struct {
	db   *DB
	sess *sessionStore
	tmpl *template.Template
}

func main() {
	dataFile := os.Getenv("DATA_FILE")
	if dataFile == "" {
		dataFile = "data.json"
	}
	db, err := newDB(dataFile)
	if err != nil {
		log.Fatalf("初始化数据失败: %v", err)
	}
	s := &Server{db: db, sess: newSessionStore()}
	s.tmpl = template.Must(template.New("").Funcs(funcMap).ParseFS(webFS, "templates/*.html"))
	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}
	host := os.Getenv("HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	log.Printf("* VPN 管理面板已启动: http://%s:%s", host, port)
	log.Printf("* 默认账号: admin / admin123")
	if err := http.ListenAndServe(host+":"+port, s.routes()); err != nil {
		log.Fatal(err)
	}
}

var funcMap = template.FuncMap{
	"fmtBytes":    fmtBytes,
	"pct":         pct,
	"minInt":      minInt,
	"nz":          nz,
	"inc":         inc,
	"secBadge":    secBadge,
	"isExpired":   isExpired,
	"adminInitial": adminInitial,
	"subPreview":  subPreview,
	"jsJSON":      jsJSON,
}

func pct(used int64, gb int) int {
	if gb <= 0 {
		return 0
	}
	p := float64(used) / (float64(gb) * 1024 * 1024 * 1024) * 100
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	return int(p)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func nz(v int) string {
	if v == 0 {
		return ""
	}
	return strconv.Itoa(v)
}

func inc(i int) int {
	return i + 1
}

func secBadge(s string) string {
	switch s {
	case "tls":
		return "green"
	case "reality":
		return "purple"
	default:
		return "gray"
	}
}

func isExpired(expireAt string) bool {
	if expireAt == "" {
		return false
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", expireAt, time.Local)
	if err != nil {
		return false
	}
	return t.Before(time.Now())
}

func adminInitial(name string) string {
	if name == "" {
		return "?"
	}
	r := []rune(name)
	return strings.ToUpper(string(r[0]))
}

func subPreview(s string) string {
	if len(s) > 48 {
		return s[:48]
	}
	return s
}

func jsJSON(v any) template.JS {
	b, err := json.Marshal(v)
	if err != nil {
		return template.JS("null")
	}
	return template.JS(b)
}

func (s *Server) routes() *http.ServeMux {
	staticFS, err := fs.Sub(webFS, "static")
	if err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /login", s.loginPage)
	mux.HandleFunc("POST /login", s.loginPost)
	mux.HandleFunc("GET /logout", s.requireLogin(s.logoutPage))
	mux.HandleFunc("GET /", s.requireLogin(s.indexPage))
	mux.HandleFunc("GET /dashboard", s.requireLogin(s.dashboardPage))
	mux.HandleFunc("GET /users", s.requireLogin(s.usersPage))
	mux.HandleFunc("GET /plans", s.requireLogin(s.plansPage))
	mux.HandleFunc("GET /nodes", s.requireLogin(s.nodesPage))
	mux.HandleFunc("GET /subscriptions", s.requireLogin(s.subscriptionsPage))
	mux.HandleFunc("GET /traffic", s.requireLogin(s.trafficPage))
	mux.HandleFunc("GET /logs", s.requireLogin(s.logsPage))
	mux.HandleFunc("GET /settings", s.requireLogin(s.settingsPage))

	mux.HandleFunc("POST /api/users", s.requireLogin(s.apiCreateUser))
	mux.HandleFunc("PUT /api/users/{id}", s.requireLogin(s.apiUpdateUser))
	mux.HandleFunc("DELETE /api/users/{id}", s.requireLogin(s.apiDeleteUser))
	mux.HandleFunc("POST /api/users/{id}/reset-traffic", s.requireLogin(s.apiResetTraffic))
	mux.HandleFunc("POST /api/users/{id}/reset-uuid", s.requireLogin(s.apiResetUUID))
	mux.HandleFunc("GET /api/users/{id}/config", s.requireLogin(s.apiUserConfig))

	mux.HandleFunc("POST /api/plans", s.requireLogin(s.apiCreatePlan))
	mux.HandleFunc("PUT /api/plans/{id}", s.requireLogin(s.apiUpdatePlan))
	mux.HandleFunc("DELETE /api/plans/{id}", s.requireLogin(s.apiDeletePlan))

	mux.HandleFunc("POST /api/nodes", s.requireLogin(s.apiCreateNode))
	mux.HandleFunc("PUT /api/nodes/{id}", s.requireLogin(s.apiUpdateNode))
	mux.HandleFunc("DELETE /api/nodes/{id}", s.requireLogin(s.apiDeleteNode))

	mux.HandleFunc("POST /api/traffic", s.requireLogin(s.apiTraffic))
	mux.HandleFunc("GET /api/subscription/{id}", s.requireLogin(s.apiSubscription))
	mux.HandleFunc("GET /api/subscription/all", s.requireLogin(s.apiSubscriptionAll))
	mux.HandleFunc("POST /api/settings", s.requireLogin(s.apiSettings))
	mux.HandleFunc("POST /api/logs/clear", s.requireLogin(s.apiClearLogs))

	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	return mux
}

// ---------------------------------------------------------------- auth

func (s *Server) requireLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.currentAdmin(r) == nil {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"success": false, "msg": "未登录"})
				return
			}
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.Path), http.StatusFound)
			return
		}
		next(w, r)
	}
}

func (s *Server) currentAdminNoLock(r *http.Request) *Admin {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return nil
	}
	adminID, ok := s.sess.validate(c.Value)
	if !ok {
		return nil
	}
	for i := range s.db.Data.Admins {
		if s.db.Data.Admins[i].ID == adminID {
			return &s.db.Data.Admins[i]
		}
	}
	return nil
}

func (s *Server) currentAdmin(r *http.Request) *Admin {
	s.db.mu.RLock()
	defer s.db.mu.RUnlock()
	return s.currentAdminNoLock(r)
}

func (s *Server) logAction(r *http.Request, action, detail string) {
	name := "-"
	if admin := s.currentAdmin(r); admin != nil {
		name = admin.Username
	}
	s.db.mu.Lock()
	s.db.Data.NextOpLogID++
	s.db.Data.OpLogs = append(s.db.Data.OpLogs, OpLog{
		ID:        s.db.Data.NextOpLogID,
		Admin:     name,
		Action:    action,
		Detail:    detail,
		IP:        r.RemoteAddr,
		CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
	})
	if len(s.db.Data.OpLogs) > 2000 {
		s.db.Data.OpLogs = s.db.Data.OpLogs[len(s.db.Data.OpLogs)-2000:]
	}
	s.db.saveLocked()
	s.db.mu.Unlock()
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data *PageData) {
	data.SiteName = s.db.getSetting("site_name", "VPN 管理面板")
	if data.Admin == nil {
		data.Admin = s.currentAdmin(r)
	}
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("模板渲染失败 %s: %v", name, err)
	}
}

// ---------------------------------------------------------------- login

func (s *Server) loginPage(w http.ResponseWriter, r *http.Request) {
	if s.currentAdmin(r) != nil {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}
	s.render(w, r, "login", &PageData{})
}

func (s *Server) loginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")

	s.db.mu.RLock()
	var admin *Admin
	for i := range s.db.Data.Admins {
		if s.db.Data.Admins[i].Username == username {
			a := s.db.Data.Admins[i]
			admin = &a
			break
		}
	}
	s.db.mu.RUnlock()

	if admin == nil || !checkPassword(password, admin.PasswordHash) {
		s.render(w, r, "login", &PageData{LoginError: "用户名或密码错误"})
		return
	}
	token := s.sess.create(admin.ID)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   int(sessionTTL.Seconds()),
	})
	s.logAction(r, "登录", fmt.Sprintf("管理员 %s 登录成功", username))
	next := r.URL.Query().Get("next")
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		next = "/dashboard"
	}
	http.Redirect(w, r, next, http.StatusFound)
}

func (s *Server) logoutPage(w http.ResponseWriter, r *http.Request) {
	name := "?"
	if admin := s.currentAdmin(r); admin != nil {
		name = admin.Username
	}
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.sess.delete(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	s.logAction(r, "退出", fmt.Sprintf("管理员 %s 退出", name))
	http.Redirect(w, r, "/login", http.StatusFound)
}

// ---------------------------------------------------------------- pages

func (s *Server) indexPage(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func userViewOf(d *Data, u User) UserView {
	v := UserView{User: u}
	for _, p := range d.Plans {
		if p.ID == u.PlanID {
			v.PlanName = p.Name
			break
		}
	}
	v.Expired = isExpired(u.ExpireAt)
	return v
}

func (s *Server) dashboardPage(w http.ResponseWriter, r *http.Request) {
	s.db.mu.RLock()
	d := s.db.Data
	totalUsers := len(d.Users)
	activeUsers := 0
	var used int64
	for _, u := range d.Users {
		if u.Enabled {
			activeUsers++
		}
		used += u.UsedBytes
	}
	totalNodes := 0
	for _, n := range d.Nodes {
		if n.Enabled {
			totalNodes++
		}
	}
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Format("2006-01-02 15:04:05")
	var todayUsed int64
	for _, t := range d.TrafficLogs {
		if t.RecordedAt >= todayStart {
			todayUsed += t.TotalBytes
		}
	}
	recentUsers := make([]UserView, 0, 6)
	for i := len(d.Users) - 1; i >= 0 && len(recentUsers) < 6; i-- {
		recentUsers = append(recentUsers, userViewOf(&d, d.Users[i]))
	}
	recentLogs := make([]OpLog, 0, 8)
	for i := len(d.OpLogs) - 1; i >= 0 && len(recentLogs) < 8; i-- {
		recentLogs = append(recentLogs, d.OpLogs[i])
	}
	s.db.mu.RUnlock()

	s.render(w, r, "page_dashboard", &PageData{
		Title: "仪表盘", Active: "dashboard", ContentBlock: "content_dashboard",
		TotalUsers: totalUsers, ActiveUsers: activeUsers, TotalNodes: totalNodes,
		UsedBytes: used, TodayBytes: todayUsed,
		RecentUsers: recentUsers, RecentLogs: recentLogs,
	})
}

func (s *Server) usersPage(w http.ResponseWriter, r *http.Request) {
	s.db.mu.RLock()
	users := make([]UserView, 0, len(s.db.Data.Users))
	for _, u := range s.db.Data.Users {
		users = append(users, userViewOf(&s.db.Data, u))
	}
	plans := make([]PlanView, 0, len(s.db.Data.Plans))
	for _, p := range s.db.Data.Plans {
		if p.Enabled {
			plans = append(plans, PlanView{Plan: p})
		}
	}
	nodes := make([]Node, 0, len(s.db.Data.Nodes))
	for _, n := range s.db.Data.Nodes {
		if n.Enabled {
			nodes = append(nodes, n)
		}
	}
	s.db.mu.RUnlock()

	s.render(w, r, "page_users", &PageData{
		Title: "用户管理", Active: "users", ContentBlock: "content_users",
		Users: users, Plans: plans, Nodes: nodes,
	})
}

func (s *Server) plansPage(w http.ResponseWriter, r *http.Request) {
	s.db.mu.RLock()
	plans := make([]PlanView, 0, len(s.db.Data.Plans))
	for _, p := range s.db.Data.Plans {
		pv := PlanView{Plan: p}
		for _, u := range s.db.Data.Users {
			if u.PlanID == p.ID {
				pv.UserCount++
			}
		}
		plans = append(plans, pv)
	}
	s.db.mu.RUnlock()

	s.render(w, r, "page_plans", &PageData{
		Title: "套餐管理", Active: "plans", ContentBlock: "content_plans", Plans: plans,
	})
}

func (s *Server) nodesPage(w http.ResponseWriter, r *http.Request) {
	s.db.mu.RLock()
	nodes := make([]Node, len(s.db.Data.Nodes))
	copy(nodes, s.db.Data.Nodes)
	s.db.mu.RUnlock()

	s.render(w, r, "page_nodes", &PageData{
		Title: "节点管理", Active: "nodes", ContentBlock: "content_nodes", Nodes: nodes,
	})
}

func (s *Server) subscriptionsPage(w http.ResponseWriter, r *http.Request) {
	s.db.mu.RLock()
	users := make([]UserView, 0, len(s.db.Data.Users))
	for _, u := range s.db.Data.Users {
		users = append(users, userViewOf(&s.db.Data, u))
	}
	nodes := make([]Node, 0, len(s.db.Data.Nodes))
	for _, n := range s.db.Data.Nodes {
		if n.Enabled {
			nodes = append(nodes, n)
		}
	}
	subs := make(map[int]string)
	for _, u := range s.db.Data.Users {
		if u.Enabled {
			subs[u.ID] = buildSubscription(nodes, u)
		}
	}
	subDomain := s.db.Data.Settings["sub_domain"]
	s.db.mu.RUnlock()

	s.render(w, r, "page_subscriptions", &PageData{
		Title: "订阅管理", Active: "subscriptions", ContentBlock: "content_subscriptions",
		Users: users, Subs: subs, SubDomain: subDomain,
		ExtraScript: template.JS("const subsMap = " + string(jsJSON(subs)) + ";"),
	})
}

func (s *Server) trafficPage(w http.ResponseWriter, r *http.Request) {
	s.db.mu.RLock()
	users := make([]UserView, 0, len(s.db.Data.Users))
	for _, u := range s.db.Data.Users {
		users = append(users, userViewOf(&s.db.Data, u))
	}
	sort.Slice(users, func(i, j int) bool { return users[i].UsedBytes > users[j].UsedBytes })
	emailByID := make(map[int]string, len(s.db.Data.Users))
	for _, u := range s.db.Data.Users {
		emailByID[u.ID] = u.Email
	}
	logs := make([]TrafficLogView, 0, 200)
	for i := len(s.db.Data.TrafficLogs) - 1; i >= 0 && len(logs) < 200; i-- {
		t := s.db.Data.TrafficLogs[i]
		logs = append(logs, TrafficLogView{TrafficLog: t, Email: emailByID[t.UserID]})
	}
	s.db.mu.RUnlock()

	s.render(w, r, "page_traffic", &PageData{
		Title: "流量统计", Active: "traffic", ContentBlock: "content_traffic",
		Users: users, TrafficLogs: logs,
	})
}

func (s *Server) logsPage(w http.ResponseWriter, r *http.Request) {
	s.db.mu.RLock()
	logs := make([]OpLog, 0, 500)
	for i := len(s.db.Data.OpLogs) - 1; i >= 0 && len(logs) < 500; i-- {
		logs = append(logs, s.db.Data.OpLogs[i])
	}
	s.db.mu.RUnlock()

	s.render(w, r, "page_logs", &PageData{
		Title: "操作日志", Active: "logs", ContentBlock: "content_logs", Logs: logs,
	})
}

func (s *Server) settingsPage(w http.ResponseWriter, r *http.Request) {
	s.db.mu.RLock()
	settings := make(map[string]string, len(s.db.Data.Settings))
	for k, v := range s.db.Data.Settings {
		settings[k] = v
	}
	s.db.mu.RUnlock()

	s.render(w, r, "page_settings", &PageData{
		Title: "系统设置", Active: "settings", ContentBlock: "content_settings", Settings: settings,
	})
}
