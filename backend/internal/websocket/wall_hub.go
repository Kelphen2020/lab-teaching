package websocket

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WallHub P2-1 监控墙：把 Agent 缩略图二进制帧广播给所有订阅的浏览器。
// 与 RemoteManager（单对单远程桌面）不同：监控墙是多教师/多浏览器同时看全机房。
//
// 帧协议（server → browser，二进制消息）：
//   [2字节 agent_id 长度 LE][agent_id UTF-8][4字节原始帧头][JPEG 数据]
// Agent 缩略图帧的原始 4 字节帧头最高位为 1（与远程桌面帧区分）。
type WallHub struct {
	mu       sync.RWMutex
	conns    map[*wallConn]struct{}
	upgrader websocket.Upgrader
}

type wallConn struct {
	send chan []byte
	conn *websocket.Conn
}

func NewWallHub() *WallHub {
	return &WallHub{
		conns: make(map[*wallConn]struct{}),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 131072,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
	}
}

// Count 当前订阅者数量
func (w *WallHub) Count() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.conns)
}

// HandleWS 升级浏览器连接为监控墙订阅（鉴权由路由层完成）
func (w *WallHub) HandleWS(rw http.ResponseWriter, r *http.Request) {
	conn, err := w.upgrader.Upgrade(rw, r, nil)
	if err != nil {
		log.Printf("[wall] upgrade 失败: %v", err)
		return
	}
	wc := &wallConn{conn: conn, send: make(chan []byte, 128)}
	w.mu.Lock()
	w.conns[wc] = struct{}{}
	w.mu.Unlock()
	log.Printf("[wall] 浏览器订阅监控墙 (subscribers=%d)", w.Count())

	// 写泵
	go func() {
		for data := range wc.send {
			conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
			if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
				break
			}
		}
	}()

	// 读泵：浏览器端无上行数据，只检测断开
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
	conn.Close()
	w.mu.Lock()
	if _, ok := w.conns[wc]; ok {
		delete(w.conns, wc)
		close(wc.send)
	}
	w.mu.Unlock()
	log.Printf("[wall] 浏览器取消订阅 (subscribers=%d)", w.Count())
}

// Relay 将 Agent 缩略图帧转发给所有订阅浏览器。
// data 为 Agent 原始二进制帧（4字节头+JPEG），前面拼上 agent_id。
func (w *WallHub) Relay(agentID string, data []byte) {
	w.mu.RLock()
	if len(w.conns) == 0 {
		w.mu.RUnlock()
		return
	}
	conns := make([]*wallConn, 0, len(w.conns))
	for c := range w.conns {
		conns = append(conns, c)
	}
	w.mu.RUnlock()

	pkt := make([]byte, 0, 2+len(agentID)+len(data))
	idBytes := []byte(agentID)
	pkt = append(pkt, byte(len(idBytes)), byte(len(idBytes)>>8))
	pkt = append(pkt, idBytes...)
	pkt = append(pkt, data...)

	for _, c := range conns {
		select {
		case c.send <- pkt:
		default:
			// 订阅者处理不过来，丢弃该帧（缩略图可丢帧）
		}
	}
}

// IsThumbFrame 判断 Agent 二进制帧是否为缩略图帧（4字节头最高位=1）
func IsThumbFrame(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	header := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24
	return header&0x8000_0000 != 0
}
