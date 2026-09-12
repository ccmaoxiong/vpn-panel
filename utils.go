package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func newUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func newToken() string {
	return randHex(16)
}

func newSecret() string {
	return randHex(12)
}

func fmtBytes(n int64) string {
	if n < 0 {
		n = 0
	}
	f := float64(n)
	units := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d B", n)
	}
	return fmt.Sprintf("%.2f %s", f, units[i])
}

func buildVlessConfig(n Node, u User) string {
	netw := n.Network
	if netw == "" {
		netw = "tcp"
	}
	security := n.Security
	if security == "" {
		security = "none"
	}
	sni := n.SNI
	if sni == "" {
		sni = n.Address
	}
	params := []string{
		"type=" + url.QueryEscape(netw),
		"security=" + url.QueryEscape(security),
		"encryption=none",
	}
	switch security {
	case "reality":
		params = append(params, "pbk=", "fp=chrome", "sni="+url.QueryEscape(sni), "sid=", "spx=%2F")
	case "tls":
		params = append(params, "sni="+url.QueryEscape(sni))
		if n.AllowInsecure {
			params = append(params, "allowInsecure=1")
		}
	}
	if n.Flow != "" {
		params = append(params, "flow="+url.QueryEscape(n.Flow))
	}
	if netw == "ws" {
		params = append(params, "path=%2F")
	}
	if netw == "grpc" {
		params = append(params, "mode=gun")
	}
	name := url.QueryEscape(n.Name + " - " + u.Email)
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s", u.UUID, n.Address, n.Port, strings.Join(params, "&"), name)
}

func buildVmessConfig(n Node, u User) string {
	netw := n.Network
	if netw == "" {
		netw = "tcp"
	}
	security := n.Security
	if security == "" {
		security = "none"
	}
	sni := n.SNI
	if sni == "" {
		sni = n.Address
	}
	cfg := map[string]string{
		"v":    "2",
		"ps":   n.Name + " - " + u.Email,
		"add":  n.Address,
		"port": strconv.Itoa(n.Port),
		"id":   u.UUID,
		"aid":  "0",
		"scy":  "auto",
		"net":  netw,
		"type": "none",
		"host": sni,
		"path": "/",
		"sni":  sni,
		"alpn": "",
	}
	if security == "tls" || security == "reality" {
		cfg["tls"] = security
	}
	raw, _ := json.Marshal(cfg)
	return "vmess://" + base64.URLEncoding.EncodeToString(raw)
}

func buildTrojanConfig(n Node, u User) string {
	netw := n.Network
	if netw == "" {
		netw = "tcp"
	}
	sni := n.SNI
	if sni == "" {
		sni = n.Address
	}
	params := []string{
		"type=" + url.QueryEscape(netw),
		"security=tls",
		"sni=" + url.QueryEscape(sni),
	}
	if n.AllowInsecure {
		params = append(params, "allowInsecure=1")
	}
	if netw == "ws" {
		params = append(params, "path=%2F")
	}
	if netw == "grpc" {
		params = append(params, "mode=gun")
	}
	name := url.QueryEscape(n.Name + " - " + u.Email)
	return fmt.Sprintf("trojan://%s@%s:%d?%s#%s", u.Password, n.Address, n.Port, strings.Join(params, "&"), name)
}

func buildUserLinks(n Node, u User) string {
	switch u.Protocol {
	case "vless":
		return buildVlessConfig(n, u)
	case "vmess":
		return buildVmessConfig(n, u)
	case "trojan":
		return buildTrojanConfig(n, u)
	default:
		return ""
	}
}

func buildUserLinksAll(nodes []Node, u User) []string {
	var links []string
	for _, n := range nodes {
		if link := buildUserLinks(n, u); link != "" {
			links = append(links, link)
		}
	}
	return links
}

func buildSubscription(nodes []Node, u User) string {
	links := buildUserLinksAll(nodes, u)
	return base64.URLEncoding.EncodeToString([]byte(strings.Join(links, "\n")))
}
