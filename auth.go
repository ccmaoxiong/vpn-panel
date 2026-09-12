package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

const sessionCookie = "vpn_session"
const sessionTTL = 12 * time.Hour

type session struct {
	AdminID int
	Expires time.Time
}

type sessionStore struct {
	mu sync.Mutex
	m  map[string]session
}

func newSessionStore() *sessionStore {
	return &sessionStore{m: make(map[string]session)}
}

func (s *sessionStore) create(adminID int) string {
	token := randHex(32)
	s.mu.Lock()
	s.m[token] = session{AdminID: adminID, Expires: time.Now().Add(sessionTTL)}
	s.mu.Unlock()
	return token
}

func (s *sessionStore) validate(token string) (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ss, ok := s.m[token]
	if !ok {
		return 0, false
	}
	if time.Now().After(ss.Expires) {
		delete(s.m, token)
		return 0, false
	}
	return ss.AdminID, true
}

func (s *sessionStore) delete(token string) {
	s.mu.Lock()
	delete(s.m, token)
	s.mu.Unlock()
}

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func hashPassword(pw string) string {
	salt := randHex(8)
	sum := sha256.Sum256([]byte(salt + pw))
	return salt + ":" + hex.EncodeToString(sum[:])
}

func checkPassword(pw, stored string) bool {
	parts := strings.SplitN(stored, ":", 2)
	if len(parts) != 2 {
		return false
	}
	sum := sha256.Sum256([]byte(parts[0] + pw))
	return hex.EncodeToString(sum[:]) == parts[1]
}
