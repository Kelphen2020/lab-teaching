package websocket

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// RemoteManager 管理教师端远程桌面查看器连接，中继 Agent 屏幕帧
type RemoteManager struct {
	mu            sync.RWMutex
	viewers       map[string]*ViewerConn // agent_id -> viewer
	onViewerInput func(agentID string, data []byte)
	upgrader      websocket.Upgrader
}

// ViewerConn 教师端 WebSocket 查看器连接
type ViewerConn struct {
	AgentID string
	Conn    *websocket.Conn
	Send    chan []byte
}

func NewRemoteManager() *RemoteManager {
	return &RemoteManager{
		viewers: make(map[string]*ViewerConn),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 65536,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
	}
}

// SetInputHandler 设置教师端输入回调（转发给 Agent）
func (rm *RemoteManager) SetInputHandler(fn func(agentID string, data []byte)) {
	rm.mu.Lock()
	rm.onViewerInput = fn
	rm.mu.Unlock()
}

// HandleViewerWS 处理教师端远程桌面 WebSocket 连接
// URL: /remote/ws?agent_id=XXX
func (rm *RemoteManager) HandleViewerWS(w http.ResponseWriter, r *http.Request, agentID string) {
	conn, err := rm.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[remote] viewer upgrade 失败: %v", err)
		return
	}

	vc := &ViewerConn{
		AgentID: agentID,
		Conn:    conn,
		Send:    make(chan []byte, 64),
	}

	rm.mu.Lock()
	if old, ok := rm.viewers[agentID]; ok {
		close(old.Send)
		old.Conn.Close()
	}
	rm.viewers[agentID] = vc
	rm.mu.Unlock()

	log.Printf("[remote] 教师查看器已连接: agent=%s", agentID)

	// 写泵
	go func() {
		defer func() {
			rm.mu.Lock()
			if rm.viewers[agentID] == vc {
				delete(rm.viewers, agentID)
			}
			rm.mu.Unlock()
			conn.Close()
			log.Printf("[remote] 教师查看器断开: agent=%s", agentID)
		}()
		for msg := range vc.Send {
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
				return
			}
		}
	}()

	// 读泵：接收教师端输入事件
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}
		rm.mu.RLock()
		fn := rm.onViewerInput
		rm.mu.RUnlock()
		if fn != nil {
			fn(agentID, message)
		}
	}
}

// RelayFrame 将 Agent 屏幕帧转发给教师查看器
func (rm *RemoteManager) RelayFrame(agentID string, data []byte) {
	rm.mu.RLock()
	vc, ok := rm.viewers[agentID]
	rm.mu.RUnlock()
	if !ok {
		return
	}
	select {
	case vc.Send <- data:
	default:
		// 丢弃帧（查看器处理不过来）
	}
}

// HasViewer 检查某 Agent 是否有教师查看器连接
func (rm *RemoteManager) HasViewer(agentID string) bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	_, ok := rm.viewers[agentID]
	return ok
}

// ViewerCount 当前查看器数量
func (rm *RemoteManager) ViewerCount() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return len(rm.viewers)
}
