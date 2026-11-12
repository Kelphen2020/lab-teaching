package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Frame 控制通道报文格式 {ver,type,msg_id,ts,payload}
type Frame struct {
	Ver     int             `json:"ver"`
	Type    string          `json:"type"`
	MsgID   string          `json:"msg_id"`
	TS      int64           `json:"ts"`
	Payload json.RawMessage `json:"payload"`
}

// Client 表示一个已连接的 Agent
type Client struct {
	AgentID string
	Conn    *websocket.Conn
	Send    chan []byte
	Hub     *Hub
}

// Hub 管理所有 Agent WebSocket 连接
type Hub struct {
	clients    map[string]*Client // agent_id -> client
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	OnEvent    func(f Frame, agentID string)
	OnBinary   func(data []byte, agentID string) // 屏幕帧等二进制数据
	SendBuf    int                               // 每连接发送缓冲区大小
	ReadLimitMB int                              // 读取限制(MB)
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client, 64),
		unregister: make(chan *Client, 64),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c.AgentID] = c
			h.mu.Unlock()
			log.Printf("[ws] agent registered: %s (online=%d)", c.AgentID, h.OnlineCount())
		case c := <-h.unregister:
			h.mu.Lock()
			if existing, ok := h.clients[c.AgentID]; ok && existing == c {
				delete(h.clients, c.AgentID)
				close(c.Send)
			}
			h.mu.Unlock()
			log.Printf("[ws] agent unregistered: %s (online=%d)", c.AgentID, h.OnlineCount())
		}
	}
}

func (h *Hub) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) OnlineAgents() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]string, 0, len(h.clients))
	for id := range h.clients {
		ids = append(ids, id)
	}
	return ids
}

func (h *Hub) IsOnline(agentID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.clients[agentID]
	return ok
}

// SendToAgent 向指定 Agent 下发控制指令
func (h *Hub) SendToAgent(agentID string, f Frame) bool {
	h.mu.RLock()
	c, ok := h.clients[agentID]
	h.mu.RUnlock()
	if !ok {
		return false
	}
	f.Ver = 1
	f.TS = time.Now().Unix()
	data, _ := json.Marshal(f)
	select {
	case c.Send <- data:
		return true
	default:
		return false
	}
}

// BroadcastToRoom 向一组 Agent 广播指令（如 broadcast_join）
func (h *Hub) BroadcastToAgents(agentIDs []string, f Frame) int {
	n := 0
	for _, id := range agentIDs {
		if h.SendToAgent(id, f) {
			n++
		}
	}
	return n
}

// BroadcastToAll 向所有在线 Agent 广播指令（如 broadcast_stop）
func (h *Hub) BroadcastToAll(f Frame) int {
	h.mu.RLock()
	ids := make([]string, 0, len(h.clients))
	for id := range h.clients {
		ids = append(ids, id)
	}
	h.mu.RUnlock()
	return h.BroadcastToAgents(ids, f)
}

// ServeWS 处理 Agent WebSocket 连接升级
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func (h *Hub) Register(c *Client)   { h.register <- c }
func (h *Hub) Unregister(c *Client) { h.unregister <- c }

// Upgrade 将 HTTP 连接升级为 WebSocket，返回 Client
func (h *Hub) Upgrade(w http.ResponseWriter, r *http.Request) (*Client, error) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	sendBuf := h.SendBuf
	if sendBuf <= 0 {
		sendBuf = 512
	}
	return &Client{Conn: conn, Send: make(chan []byte, sendBuf), Hub: h}, nil
}

func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister(c)
		c.Conn.Close()
	}()
	readLimitMB := c.Hub.ReadLimitMB
	if readLimitMB <= 0 {
		readLimitMB = 4
	}
	c.Conn.SetReadLimit(int64(readLimitMB) * 1048576)
	c.Conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		return nil
	})
	for {
		msgType, message, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}
		// 二进制帧（屏幕帧等）直接回调
		if msgType == websocket.BinaryMessage {
			if c.Hub.OnBinary != nil {
				c.Hub.OnBinary(message, c.AgentID)
			}
			continue
		}
		var f Frame
		if err := json.Unmarshal(message, &f); err != nil {
			continue
		}
		// 注册帧确定 agent_id
		if f.Type == "register" {
			var p struct {
				AgentID string `json:"agent_id"`
			}
			json.Unmarshal(f.Payload, &p)
			if p.AgentID != "" {
				c.AgentID = p.AgentID
				c.Hub.Register(c)
			}
		}
		if c.Hub.OnEvent != nil {
			c.Hub.OnEvent(f, c.AgentID)
		}
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(5 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.Conn.WriteMessage(websocket.TextMessage, message)
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
