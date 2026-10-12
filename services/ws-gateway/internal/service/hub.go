package service

import (
	"sync"

	"github.com/gofiber/contrib/websocket"
)

type ConnectionHub interface {
	Register(s *Session) bool
	Unregister(s *Session)
	ActiveConnections() int
	PushToUser(userID string, msg []byte)
	DisconnectUser(userID string, code int, text string)
	CloseAll()
}

type connectionHub struct {
	mu             sync.RWMutex
	sessions       map[string]map[*Session]struct{}
	maxConnections int
	activeCount    int
}

func NewConnectionHub(maxConnections int) ConnectionHub {
	return &connectionHub{
		sessions:       make(map[string]map[*Session]struct{}),
		maxConnections: maxConnections,
	}
}

func (h *connectionHub) Register(s *Session) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.activeCount >= h.maxConnections {
		return false
	}

	userSessions, exists := h.sessions[s.UserID]
	if !exists {
		userSessions = make(map[*Session]struct{})
		h.sessions[s.UserID] = userSessions
	}
	userSessions[s] = struct{}{}
	h.activeCount++
	return true
}

func (h *connectionHub) Unregister(s *Session) {
	h.mu.Lock()
	defer h.mu.Unlock()

	userSessions, exists := h.sessions[s.UserID]
	if !exists {
		return
	}

	if _, found := userSessions[s]; found {
		delete(userSessions, s)
		h.activeCount--
		if len(userSessions) == 0 {
			delete(h.sessions, s.UserID)
		}
	}
}

func (h *connectionHub) ActiveConnections() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.activeCount
}

func (h *connectionHub) PushToUser(userID string, msg []byte) {
	h.mu.RLock()
	userMap, exists := h.sessions[userID]
	if !exists || len(userMap) == 0 {
		h.mu.RUnlock()
		return
	}

	targets := make([]*Session, 0, len(userMap))
	for s := range userMap {
		targets = append(targets, s)
	}
	h.mu.RUnlock()

	for _, s := range targets {
		s.Send(msg)
	}
}

func (h *connectionHub) DisconnectUser(userID string, code int, text string) {
	h.mu.RLock()
	userMap, exists := h.sessions[userID]
	if !exists || len(userMap) == 0 {
		h.mu.RUnlock()
		return
	}

	targets := make([]*Session, 0, len(userMap))
	for s := range userMap {
		targets = append(targets, s)
	}
	h.mu.RUnlock()

	for _, s := range targets {
		s.CloseWithCode(code, text)
	}
}

func (h *connectionHub) CloseAll() {
	h.mu.RLock()
	allSessions := make([]*Session, 0, h.activeCount)
	for _, userMap := range h.sessions {
		for s := range userMap {
			allSessions = append(allSessions, s)
		}
	}
	h.mu.RUnlock()

	for _, s := range allSessions {
		s.CloseWithCode(websocket.CloseGoingAway, "server shutdown")
	}
}
