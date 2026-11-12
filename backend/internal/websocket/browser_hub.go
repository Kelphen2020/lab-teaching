package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// BrowserHub 管理管理端浏览器 WebSocket 连接（服务端单向事件推送）。
// 与 Agent Hub 分离：浏览器端只接收事件（agent 上下线、监控快照），不上报数据。
type BrowserHub struct {
	mu         sync.RWMutex
	conns      map[*BrowserConn]struct{}
	register   chan *BrowserConn
	unregister chan *BrowserConn
}

// BrowserConn 一个浏览器订阅连接
type BrowserConn struct {
	hub  *BrowserHub
	Conn *websocket.Conn
	send chan []byte
}

func NewBrowserHub() *BrowserHub {
	return &BrowserHub{
		conns:      make(map[*BrowserConn]struct{}),
		register:   make(chan *BrowserConn, 32),
		unregister: make(chan *BrowserConn, 32),
	}
}

func (h *BrowserHub) Run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.conns[c] = struct{}{}
			h.mu.Unlock()
			log.Printf("[browser-ws] connected (subscribers=%d)", h.Count())
		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.conns[c]; ok {
				delete(h.conns, c)
				close(c.send)
			}
			h.mu.Unlock()
			log.Printf("[browser-ws] disconnected (subscribers=%d)", h.Count())
		}
	}
}

// Count 当前订阅者数量
func (h *BrowserHub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}

// Upgrade 升级 HTTP 连接为浏览器订阅连接（鉴权由路由层完成）
func (h *BrowserHub) Upgrade(w http.ResponseWriter, r *http.Request) (*BrowserConn, error) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	return &BrowserConn{hub: h, Conn: conn, send: make(chan []byte, 128)}, nil
}

func (h *BrowserHub) Register(c *BrowserConn)   { h.register <- c }
func (h *BrowserHub) Unregister(c *BrowserConn) { h.unregister <- c }

// Broadcast 向所有订阅浏览器推送事件帧 {ver,type,ts,payload}。
// 发送缓冲满时丢弃该连接的此帧（不阻塞推送方）。
func (h *BrowserHub) Broadcast(evtType string, payload interface{}) {
	h.mu.RLock()
	if len(h.conns) == 0 {
		h.mu.RUnlock()
		return
	}
	conns := make([]*BrowserConn, 0, len(h.conns))
	for c := range h.conns {
		conns = append(conns, c)
	}
	h.mu.RUnlock()

	raw, _ := json.Marshal(payload)
	frame, err := json.Marshal(Frame{
		Ver:     1,
		Type:    evtType,
		TS:      time.Now().Unix(),
		Payload: raw,
	})
	if err != nil {
		return
	}
	for _, c := range conns {
		select {
		case c.send <- frame:
		default:
		}
	}
}

// ReadPump 浏览器端只接收事件，无需上行数据；
// 读取仅用于检测断开与响应 ping/pong。
func (c *BrowserConn) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		c.Conn.Close()
	}()
	c.Conn.SetReadLimit(4096)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		if _, _, err := c.Conn.ReadMessage(); err != nil {
			break
		}
	}
}

func (c *BrowserConn) WritePump() {
	ticker := time.NewTicker(25 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
