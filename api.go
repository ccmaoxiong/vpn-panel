package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func okJSON(msg string) map[string]any {
	return map[string]any{"success": true, "msg": msg}
}

func failJSON(msg string) map[string]any {
	return map[string]any{"success": false, "msg": msg}
}

func parseID(r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

type userRequest struct {
	Email        string `json:"email"`
	Protocol     string `json:"protocol"`
	PlanID       *int   `json:"plan_id"`
	TrafficGB    int    `json:"traffic_gb"`
	DurationDays int    `json:"duration_days"`
	DeviceLimit  int    `json:"device_limit"`
	Password     string `json:"password"`
	Enabled      *int   `json:"enabled"`
}

type planRequest struct {
	Name         string  `json:"name"`
	TrafficGB    int     `json:"traffic_gb"`
	DurationDays int     `json:"duration_days"`
	Price        float64 `json:"price"`
	DeviceLimit  int     `json:"device_limit"`
	Enabled      *int    `json:"enabled"`
}

type nodeRequest struct {
	Name          string `json:"name"`
	Address       string `json:"address"`
	Port          int    `json:"port"`
	Network       string `json:"network"`
	Security      string `json:"security"`
	SNI           string `json:"sni"`
	Flow          string `json:"flow"`
	AllowInsecure *int   `json:"allow_insecure"`
	Remarks       string `json:"remarks"`
	Enabled       *int   `json:"enabled"`
}

type settingsRequest struct {
	SiteName          string `json:"site_name"`
	SubDomain         string `json:"sub_domain"`
	TrafficResetCycle string `json:"traffic_reset_cycle"`
	NewPassword       string `json:"new_password"`
}

func findPlanLocked(d *Data, id int) *Plan {
	for i := range d.Plans {
		if d.Plans[i].ID == id {
			return &d.Plans[i]
		}
	}
	return nil
}

// ---------------------------------------------------------------- users API

func (s *Server) apiCreateUser(w http.ResponseWriter, r *http.Request) {
	var req userRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, failJSON("请求格式错误"))
		return
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		writeJSON(w, http.StatusBadRequest, failJSON("邮箱不能为空"))
		return
	}

	s.db.mu.Lock()
	for _, u := range s.db.Data.Users {
		if u.Email == email {
			s.db.mu.Unlock()
			writeJSON(w, http.StatusBadRequest, failJSON("该邮箱已存在"))
			return
		}
	}
	protocol := req.Protocol
	if protocol == "" {
		protocol = "vless"
	}
	trafficGB := req.TrafficGB
	duration := req.DurationDays
	deviceLimit := req.DeviceLimit
	planID := 0
	if req.PlanID != nil {
		planID = *req.PlanID
	}
	if planID > 0 {
		if plan := findPlanLocked(&s.db.Data, planID); plan != nil {
			if trafficGB <= 0 {
				trafficGB = plan.TrafficGB
			}
			if duration <= 0 {
				duration = plan.DurationDays
			}
			if req.DeviceLimit <= 0 {
				deviceLimit = plan.DeviceLimit
			}
		}
	}
	if trafficGB <= 0 {
		trafficGB = 50
	}
	password := req.Password
	if password == "" {
		password = newSecret()
	}
	expireAt := ""
	if duration > 0 {
		expireAt = time.Now().Add(time.Duration(duration) * 24 * time.Hour).Format("2006-01-02 15:04:05")
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled != 0
	}
	s.db.Data.NextUserID++
	s.db.Data.Users = append(s.db.Data.Users, User{
		ID:          s.db.Data.NextUserID,
		Email:       email,
		UUID:        newUUID(),
		Password:    password,
		Protocol:    protocol,
		PlanID:      planID,
		TrafficGB:   trafficGB,
		ExpireAt:    expireAt,
		DeviceLimit: deviceLimit,
		Enabled:     enabled,
		Token:       newToken(),
		CreatedAt:   time.Now().Format("2006-01-02 15:04:05"),
	})
	s.db.saveLocked()
	s.db.mu.Unlock()

	s.logAction(r, "创建用户", fmt.Sprintf("邮箱: %s, 协议: %s", email, protocol))
	writeJSON(w, http.StatusOK, okJSON("用户创建成功"))
}

func (s *Server) apiUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, failJSON("参数错误"))
		return
	}
	var req userRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, failJSON("请求格式错误"))
		return
	}

	s.db.mu.Lock()
	idx := -1
	for i := range s.db.Data.Users {
		if s.db.Data.Users[i].ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		s.db.mu.Unlock()
		writeJSON(w, http.StatusNotFound, failJSON("用户不存在"))
		return
	}
	u := &s.db.Data.Users[idx]
	if req.Email != "" {
		u.Email = req.Email
	}
	if req.Protocol != "" {
		u.Protocol = req.Protocol
	}
	if req.Enabled != nil {
		u.Enabled = *req.Enabled != 0
	}
	if req.TrafficGB > 0 {
		u.TrafficGB = req.TrafficGB
	}
	if req.PlanID != nil {
		u.PlanID = *req.PlanID
	}
	u.DeviceLimit = req.DeviceLimit
	if req.Password != "" {
		u.Password = req.Password
	}
	s.db.saveLocked()
	s.db.mu.Unlock()

	s.logAction(r, "修改用户", fmt.Sprintf("ID: %d", id))
	writeJSON(w, http.StatusOK, okJSON("用户已更新"))
}

func (s *Server) apiDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, failJSON("参数错误"))
		return
	}
	s.db.mu.Lock()
	idx := -1
	for i := range s.db.Data.Users {
		if s.db.Data.Users[i].ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		s.db.mu.Unlock()
		writeJSON(w, http.StatusNotFound, failJSON("用户不存在"))
		return
	}
	email := s.db.Data.Users[idx].Email
	s.db.Data.Users = append(s.db.Data.Users[:idx], s.db.Data.Users[idx+1:]...)
	logs := s.db.Data.TrafficLogs[:0]
	for _, t := range s.db.Data.TrafficLogs {
		if t.UserID != id {
			logs = append(logs, t)
		}
	}
	s.db.Data.TrafficLogs = logs
	s.db.saveLocked()
	s.db.mu.Unlock()

	s.logAction(r, "删除用户", fmt.Sprintf("邮箱: %s", email))
	writeJSON(w, http.StatusOK, okJSON("用户已删除"))
}

func (s *Server) apiResetTraffic(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, failJSON("参数错误"))
		return
	}
	s.db.mu.Lock()
	found := false
	var email string
	for i := range s.db.Data.Users {
		if s.db.Data.Users[i].ID == id {
			s.db.Data.Users[i].UsedBytes = 0
			email = s.db.Data.Users[i].Email
			found = true
			break
		}
	}
	if !found {
		s.db.mu.Unlock()
		writeJSON(w, http.StatusNotFound, failJSON("用户不存在"))
		return
	}
	s.db.saveLocked()
	s.db.mu.Unlock()

	s.logAction(r, "重置流量", fmt.Sprintf("用户: %s", email))
	writeJSON(w, http.StatusOK, okJSON("流量已重置"))
}

func (s *Server) apiResetUUID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, failJSON("参数错误"))
		return
	}
	s.db.mu.Lock()
	found := false
	var email string
	for i := range s.db.Data.Users {
		if s.db.Data.Users[i].ID == id {
			s.db.Data.Users[i].UUID = newUUID()
			email = s.db.Data.Users[i].Email
			found = true
			break
		}
	}
	if !found {
		s.db.mu.Unlock()
		writeJSON(w, http.StatusNotFound, failJSON("用户不存在"))
		return
	}
	s.db.saveLocked()
	s.db.mu.Unlock()

	s.logAction(r, "重置 UUID", fmt.Sprintf("用户: %s", email))
	writeJSON(w, http.StatusOK, okJSON("UUID 已重置"))
}

func (s *Server) apiUserConfig(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, failJSON("参数错误"))
		return
	}
	s.db.mu.RLock()
	var user *User
	for i := range s.db.Data.Users {
		if s.db.Data.Users[i].ID == id {
			user = &s.db.Data.Users[i]
			break
		}
	}
	if user == nil {
		s.db.mu.RUnlock()
		writeJSON(w, http.StatusNotFound, failJSON("用户不存在"))
		return
	}
	nodes := make([]Node, 0, len(s.db.Data.Nodes))
	for _, n := range s.db.Data.Nodes {
		if n.Enabled {
			nodes = append(nodes, n)
		}
	}
	s.db.mu.RUnlock()

	links := buildUserLinksAll(nodes, *user)
	writeJSON(w, http.StatusOK, map[string]any{
		"success":      true,
		"uuid":         user.UUID,
		"password":     user.Password,
		"protocol":     user.Protocol,
		"links":        links,
		"subscription": buildSubscription(nodes, *user),
	})
}

// ---------------------------------------------------------------- plans API

func (s *Server) apiCreatePlan(w http.ResponseWriter, r *http.Request) {
	var req planRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, failJSON("请求格式错误"))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeJSON(w, http.StatusBadRequest, failJSON("套餐名称不能为空"))
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled != 0
	}
	s.db.mu.Lock()
	s.db.Data.NextPlanID++
	s.db.Data.Plans = append(s.db.Data.Plans, Plan{
		ID:           s.db.Data.NextPlanID,
		Name:         name,
		TrafficGB:    req.TrafficGB,
		DurationDays: req.DurationDays,
		Price:        req.Price,
		DeviceLimit:  req.DeviceLimit,
		Enabled:      enabled,
		CreatedAt:    time.Now().Format("2006-01-02 15:04:05"),
	})
	s.db.saveLocked()
	s.db.mu.Unlock()

	s.logAction(r, "创建套餐", fmt.Sprintf("名称: %s", name))
	writeJSON(w, http.StatusOK, okJSON("套餐创建成功"))
}

func (s *Server) apiUpdatePlan(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, failJSON("参数错误"))
		return
	}
	var req planRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, failJSON("请求格式错误"))
		return
	}
	s.db.mu.Lock()
	plan := findPlanLocked(&s.db.Data, id)
	if plan == nil {
		s.db.mu.Unlock()
		writeJSON(w, http.StatusNotFound, failJSON("套餐不存在"))
		return
	}
	if req.Name != "" {
		plan.Name = req.Name
	}
	if req.TrafficGB > 0 {
		plan.TrafficGB = req.TrafficGB
	}
	if req.DurationDays > 0 {
		plan.DurationDays = req.DurationDays
	}
	plan.Price = req.Price
	plan.DeviceLimit = req.DeviceLimit
	if req.Enabled != nil {
		plan.Enabled = *req.Enabled != 0
	}
	s.db.saveLocked()
	s.db.mu.Unlock()

	s.logAction(r, "修改套餐", fmt.Sprintf("ID: %d", id))
	writeJSON(w, http.StatusOK, okJSON("套餐已更新"))
}

func (s *Server) apiDeletePlan(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, failJSON("参数错误"))
		return
	}
	s.db.mu.Lock()
	idx := -1
	for i := range s.db.Data.Plans {
		if s.db.Data.Plans[i].ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		s.db.mu.Unlock()
		writeJSON(w, http.StatusNotFound, failJSON("套餐不存在"))
		return
	}
	name := s.db.Data.Plans[idx].Name
	s.db.Data.Plans = append(s.db.Data.Plans[:idx], s.db.Data.Plans[idx+1:]...)
	s.db.saveLocked()
	s.db.mu.Unlock()

	s.logAction(r, "删除套餐", fmt.Sprintf("名称: %s", name))
	writeJSON(w, http.StatusOK, okJSON("套餐已删除"))
}

// ---------------------------------------------------------------- nodes API

func (s *Server) apiCreateNode(w http.ResponseWriter, r *http.Request) {
	var req nodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, failJSON("请求格式错误"))
		return
	}
	name := strings.TrimSpace(req.Name)
	address := strings.TrimSpace(req.Address)
	if name == "" || address == "" {
		writeJSON(w, http.StatusBadRequest, failJSON("节点名称和地址不能为空"))
		return
	}
	port := req.Port
	if port <= 0 {
		port = 443
	}
	network := req.Network
	if network == "" {
		network = "ws"
	}
	security := req.Security
	if security == "" {
		security = "tls"
	}
	sni := req.SNI
	if sni == "" {
		sni = address
	}
	allowInsecure := false
	if req.AllowInsecure != nil {
		allowInsecure = *req.AllowInsecure != 0
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled != 0
	}
	s.db.mu.Lock()
	s.db.Data.NextNodeID++
	s.db.Data.Nodes = append(s.db.Data.Nodes, Node{
		ID:            s.db.Data.NextNodeID,
		Name:          name,
		Address:       address,
		Port:          port,
		Network:       network,
		Security:      security,
		SNI:           sni,
		Flow:          req.Flow,
		AllowInsecure: allowInsecure,
		Remarks:       req.Remarks,
		Enabled:       enabled,
		CreatedAt:     time.Now().Format("2006-01-02 15:04:05"),
	})
	s.db.saveLocked()
	s.db.mu.Unlock()

	s.logAction(r, "创建节点", fmt.Sprintf("名称: %s, 地址: %s:%d", name, address, port))
	writeJSON(w, http.StatusOK, okJSON("节点创建成功"))
}

func (s *Server) apiUpdateNode(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, failJSON("参数错误"))
		return
	}
	var req nodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, failJSON("请求格式错误"))
		return
	}
	s.db.mu.Lock()
	var node *Node
	for i := range s.db.Data.Nodes {
		if s.db.Data.Nodes[i].ID == id {
			node = &s.db.Data.Nodes[i]
			break
		}
	}
	if node == nil {
		s.db.mu.Unlock()
		writeJSON(w, http.StatusNotFound, failJSON("节点不存在"))
		return
	}
	if req.Name != "" {
		node.Name = req.Name
	}
	if req.Address != "" {
		node.Address = req.Address
	}
	if req.Port > 0 {
		node.Port = req.Port
	}
	if req.Network != "" {
		node.Network = req.Network
	}
	if req.Security != "" {
		node.Security = req.Security
	}
	node.SNI = req.SNI
	node.Flow = req.Flow
	node.Remarks = req.Remarks
	if req.AllowInsecure != nil {
		node.AllowInsecure = *req.AllowInsecure != 0
	}
	if req.Enabled != nil {
		node.Enabled = *req.Enabled != 0
	}
	s.db.saveLocked()
	s.db.mu.Unlock()

	s.logAction(r, "修改节点", fmt.Sprintf("ID: %d", id))
	writeJSON(w, http.StatusOK, okJSON("节点已更新"))
}

func (s *Server) apiDeleteNode(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, failJSON("参数错误"))
		return
	}
	s.db.mu.Lock()
	idx := -1
	for i := range s.db.Data.Nodes {
		if s.db.Data.Nodes[i].ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		s.db.mu.Unlock()
		writeJSON(w, http.StatusNotFound, failJSON("节点不存在"))
		return
	}
	name := s.db.Data.Nodes[idx].Name
	s.db.Data.Nodes = append(s.db.Data.Nodes[:idx], s.db.Data.Nodes[idx+1:]...)
	s.db.saveLocked()
	s.db.mu.Unlock()

	s.logAction(r, "删除节点", fmt.Sprintf("名称: %s", name))
	writeJSON(w, http.StatusOK, okJSON("节点已删除"))
}

// ---------------------------------------------------------------- traffic API

func (s *Server) apiTraffic(w http.ResponseWriter, r *http.Request) {
	s.db.mu.Lock()
	count := 0
	for i := range s.db.Data.Users {
		u := &s.db.Data.Users[i]
		if !u.Enabled {
			continue
		}
		up := int64(rand.Intn(490)+10) * 1024 * 1024
		down := int64(rand.Intn(1950)+50) * 1024 * 1024
		u.UsedBytes += up + down
		s.db.Data.NextTrafficID++
		s.db.Data.TrafficLogs = append(s.db.Data.TrafficLogs, TrafficLog{
			ID:         s.db.Data.NextTrafficID,
			UserID:     u.ID,
			UpBytes:    up,
			DownBytes:  down,
			TotalBytes: up + down,
			RecordedAt: time.Now().Format("2006-01-02 15:04:05"),
		})
		count++
	}
	if len(s.db.Data.TrafficLogs) > 3000 {
		s.db.Data.TrafficLogs = s.db.Data.TrafficLogs[len(s.db.Data.TrafficLogs)-3000:]
	}
	s.db.saveLocked()
	s.db.mu.Unlock()

	s.logAction(r, "记录流量", fmt.Sprintf("为 %d 个用户写入模拟流量", count))
	writeJSON(w, http.StatusOK, okJSON(fmt.Sprintf("已记录 %d 个用户的流量", count)))
}

// ---------------------------------------------------------------- subscription API

func (s *Server) apiSubscription(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	s.db.mu.RLock()
	var user *User
	for i := range s.db.Data.Users {
		if s.db.Data.Users[i].ID == id {
			user = &s.db.Data.Users[i]
			break
		}
	}
	if user == nil {
		s.db.mu.RUnlock()
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	nodes := make([]Node, 0, len(s.db.Data.Nodes))
	for _, n := range s.db.Data.Nodes {
		if n.Enabled {
			nodes = append(nodes, n)
		}
	}
	sub := buildSubscription(nodes, *user)
	s.db.mu.RUnlock()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(sub))
}

func (s *Server) apiSubscriptionAll(w http.ResponseWriter, r *http.Request) {
	s.db.mu.RLock()
	nodes := make([]Node, 0, len(s.db.Data.Nodes))
	for _, n := range s.db.Data.Nodes {
		if n.Enabled {
			nodes = append(nodes, n)
		}
	}
	result := make(map[string]string)
	for _, u := range s.db.Data.Users {
		if u.Enabled {
			result[u.Email] = buildSubscription(nodes, u)
		}
	}
	s.db.mu.RUnlock()

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": result})
}

// ---------------------------------------------------------------- settings API

func (s *Server) apiSettings(w http.ResponseWriter, r *http.Request) {
	var req settingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, failJSON("请求格式错误"))
		return
	}
	s.db.mu.Lock()
	s.db.setSettingLocked("site_name", req.SiteName)
	s.db.setSettingLocked("sub_domain", req.SubDomain)
	s.db.setSettingLocked("traffic_reset_cycle", req.TrafficResetCycle)
	if req.NewPassword != "" {
		if len(req.NewPassword) < 6 {
			s.db.mu.Unlock()
			writeJSON(w, http.StatusBadRequest, failJSON("新密码至少 6 位"))
			return
		}
		admin := s.currentAdminNoLock(r)
		if admin != nil {
			for i := range s.db.Data.Admins {
				if s.db.Data.Admins[i].ID == admin.ID {
					s.db.Data.Admins[i].PasswordHash = hashPassword(req.NewPassword)
					break
				}
			}
		}
	}
	s.db.saveLocked()
	s.db.mu.Unlock()

	s.logAction(r, "修改设置", "保存了系统设置")
	writeJSON(w, http.StatusOK, okJSON("设置已保存"))
}

func (s *Server) apiClearLogs(w http.ResponseWriter, r *http.Request) {
	s.db.mu.Lock()
	s.db.Data.OpLogs = nil
	s.db.saveLocked()
	s.db.mu.Unlock()

	s.logAction(r, "清空日志", "清空了操作日志")
	writeJSON(w, http.StatusOK, okJSON("日志已清空"))
}
