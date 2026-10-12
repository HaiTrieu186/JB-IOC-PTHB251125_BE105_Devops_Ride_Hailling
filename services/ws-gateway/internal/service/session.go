package service

import (
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
)

type Session struct {
	UserID    string
	Role      string
	conn      *websocket.Conn
	sendChan  chan []byte
	closeOnce sync.Once
	done      chan struct{}
	hub       ConnectionHub
}

func NewSession(userID, role string, conn *websocket.Conn, bufferSize int, hub ConnectionHub) *Session {
	return &Session{
		UserID:   userID,
		Role:     role,
		conn:     conn,
		sendChan: make(chan []byte, bufferSize),
		done:     make(chan struct{}),
		hub:      hub,
	}
}

func (s *Session) Send(msg []byte) {
	select {
	case <-s.done:
		return
	default:
	}

	select {
	case s.sendChan <- msg:
	default:
		// Send buffer full -> SLOW_CLIENT (Close 1008)
		go s.CloseWithCode(websocket.ClosePolicyViolation, "SLOW_CLIENT")
	}
}

func (s *Session) CloseWithCode(code int, text string) {
	s.closeOnce.Do(func() {
		close(s.done)
		closeMsg := websocket.FormatCloseMessage(code, text)
		_ = s.conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(time.Second))
		_ = s.conn.Close()
		s.hub.Unregister(s)
	})
}

func (s *Session) Conn() *websocket.Conn {
	return s.conn
}

func (s *Session) SendChan() <-chan []byte {
	return s.sendChan
}

func (s *Session) Done() <-chan struct{} {
	return s.done
}
