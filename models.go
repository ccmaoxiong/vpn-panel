package main

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

type Admin struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"`
	CreatedAt    string `json:"created_at"`
}

type Node struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Address       string `json:"address"`
	Port          int    `json:"port"`
	Network       string `json:"network"`
	Security      string `json:"security"`
	SNI           string `json:"sni"`
	Flow          string `json:"flow"`
	AllowInsecure bool   `json:"allow_insecure"`
	Remarks       string `json:"remarks"`
	Enabled       bool   `json:"enabled"`
	CreatedAt     string `json:"created_at"`
}

type Plan struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	TrafficGB    int     `json:"traffic_gb"`
	DurationDays int     `json:"duration_days"`
	Price        float64 `json:"price"`
	DeviceLimit  int     `json:"device_limit"`
	Enabled      bool    `json:"enabled"`
	CreatedAt    string  `json:"created_at"`
}

type User struct {
	ID          int    `json:"id"`
	Email       string `json:"email"`
	UUID        string `json:"uuid"`
	Password    string `json:"password"`
	Protocol    string `json:"protocol"`
	PlanID      int    `json:"plan_id"`
	TrafficGB   int    `json:"traffic_gb"`
	UsedBytes   int64  `json:"used_bytes"`
	ExpireAt    string `json:"expire_at"`
	DeviceLimit int    `json:"device_limit"`
	Enabled     bool   `json:"enabled"`
	Token       string `json:"token"`
	CreatedAt   string `json:"created_at"`
}

type TrafficLog struct {
	ID         int    `json:"id"`
	UserID     int    `json:"user_id"`
	UpBytes    int64  `json:"up_bytes"`
	DownBytes  int64  `json:"down_bytes"`
	TotalBytes int64  `json:"total_bytes"`
	RecordedAt string `json:"recorded_at"`
}

type OpLog struct {
	ID        int    `json:"id"`
	Admin     string `json:"admin"`
	Action    string `json:"action"`
	Detail    string `json:"detail"`
	IP        string `json:"ip"`
	CreatedAt string `json:"created_at"`
}

type Data struct {
	Settings      map[string]string `json:"settings"`
	Admins        []Admin           `json:"admins"`
	Nodes         []Node            `json:"nodes"`
	Plans         []Plan            `json:"plans"`
	Users         []User            `json:"users"`
	TrafficLogs   []TrafficLog      `json:"traffic_logs"`
	OpLogs        []OpLog           `json:"op_logs"`
	NextUserID    int               `json:"next_user_id"`
	NextNodeID    int               `json:"next_node_id"`
	NextPlanID    int               `json:"next_plan_id"`
	NextTrafficID int               `json:"next_traffic_id"`
	NextOpLogID   int               `json:"next_op_log_id"`
}

type DB struct {
	mu   sync.RWMutex
	path string
	Data Data
}

func newDB(path string) (*DB, error) {
	db := &DB{path: path}
	if err := db.load(); err != nil {
		return nil, err
	}
	return db, nil
}

func (db *DB) load() error {
	b, err := os.ReadFile(db.path)
	if err != nil {
		if os.IsNotExist(err) {
			db.seed()
			return db.saveLocked()
		}
		return err
	}
	if err := json.Unmarshal(b, &db.Data); err != nil {
		return err
	}
	if db.Data.Settings == nil {
		db.Data.Settings = map[string]string{}
	}
	return nil
}

func (db *DB) seed() {
	now := time.Now().Format("2006-01-02 15:04:05")
	db.Data.Settings = map[string]string{
		"site_name":           "VPN 管理面板",
		"sub_domain":          "",
		"traffic_reset_cycle": "monthly",
	}
	db.Data.Admins = []Admin{
		{ID: 1, Username: "admin", PasswordHash: hashPassword("admin123"), CreatedAt: now},
	}
	db.Data.Nodes = []Node{
		{ID: 1, Name: "默认节点", Address: "vpn.example.com", Port: 443, Network: "ws", Security: "tls", SNI: "vpn.example.com", Enabled: true, CreatedAt: now},
	}
	db.Data.Plans = []Plan{
		{ID: 1, Name: "基础套餐", TrafficGB: 50, DurationDays: 30, Price: 9.9, DeviceLimit: 3, Enabled: true, CreatedAt: now},
		{ID: 2, Name: "标准套餐", TrafficGB: 200, DurationDays: 30, Price: 19.9, DeviceLimit: 5, Enabled: true, CreatedAt: now},
		{ID: 3, Name: "高级套餐", TrafficGB: 500, DurationDays: 90, Price: 39.9, DeviceLimit: 0, Enabled: true, CreatedAt: now},
	}
	db.Data.NextUserID = 1
	db.Data.NextNodeID = 2
	db.Data.NextPlanID = 4
	db.Data.NextTrafficID = 1
	db.Data.NextOpLogID = 1
}

func (db *DB) saveLocked() error {
	b, err := json.MarshalIndent(db.Data, "", "  ")
	if err != nil {
		return err
	}
	tmp := db.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, db.path)
}

func (db *DB) Save() error {
	return db.saveLocked()
}

func (db *DB) getSetting(key, def string) string {
	db.mu.RLock()
	defer db.mu.RUnlock()
	if v, ok := db.Data.Settings[key]; ok {
		return v
	}
	return def
}

func (db *DB) setSettingLocked(key, value string) {
	db.Data.Settings[key] = value
}
