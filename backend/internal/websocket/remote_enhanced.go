package websocket

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// === V0.9 远程桌面增强：多画面轮巡、键鼠锁定、批量操作 ===

func mustJSON(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// PatrolSession 多画面轮巡会话
type PatrolSession struct {
	ID        string    `json:"id"`
	TeacherID uint      `json:"teacher_id"`
	AgentIDs  []string  `json:"agent_ids"`
	Interval  int       `json:"interval_sec"`
	Current   int       `json:"current_index"`
	Status    string    `json:"status"`
	StartAt   time.Time `json:"start_at"`
	stopCh    chan struct{}
}

// LockState 键鼠锁定状态
type LockState struct {
	AgentID  string    `json:"agent_id"`
	Locked   bool      `json:"locked"`
	LockType string    `json:"lock_type"`
	LockedAt time.Time `json:"locked_at"`
	LockedBy uint      `json:"locked_by"`
}

// RemoteEnhanced 增强远程桌面管理器
type RemoteEnhanced struct {
	mu      sync.RWMutex
	patrols map[string]*PatrolSession
	locks   map[string]*LockState
	hub     *Hub
	nextID  int
}

func NewRemoteEnhanced(hub *Hub) *RemoteEnhanced {
	return &RemoteEnhanced{
		patrols: make(map[string]*PatrolSession),
		locks:   make(map[string]*LockState),
		hub:     hub,
		nextID:  1,
	}
}

// StartPatrol 启动多画面轮巡
func (re *RemoteEnhanced) StartPatrol(teacherID uint, agentIDs []string, intervalSec int) *PatrolSession {
	re.mu.Lock()
	defer re.mu.Unlock()

	if intervalSec <= 0 {
		intervalSec = 5
	}

	id := fmt.Sprintf("patrol_%d_%d", teacherID, re.nextID)
	re.nextID++

	session := &PatrolSession{
		ID:        id,
		TeacherID: teacherID,
		AgentIDs:  agentIDs,
		Interval:  intervalSec,
		Current:   0,
		Status:    "running",
		StartAt:   time.Now(),
		stopCh:    make(chan struct{}),
	}
	re.patrols[id] = session

	go func() {
		ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-session.stopCh:
				return
			case <-ticker.C:
				re.mu.Lock()
				session.Current = (session.Current + 1) % len(session.AgentIDs)
				currentAgent := session.AgentIDs[session.Current]
				idx := session.Current
				re.mu.Unlock()

				re.hub.SendToAgent(currentAgent, Frame{
					Type:    "patrol_focus",
					Payload: mustJSON(map[string]interface{}{"patrol_id": id, "index": idx}),
				})
				log.Printf("[remote] 轮巡切换: %s -> agent=%s", id, currentAgent)
			}
		}
	}()

	log.Printf("[remote] 轮巡启动: %s, %d台终端, 间隔%ds", id, len(agentIDs), intervalSec)
	return session
}

// StopPatrol 停止轮巡
func (re *RemoteEnhanced) StopPatrol(patrolID string) {
	re.mu.Lock()
	defer re.mu.Unlock()
	if s, ok := re.patrols[patrolID]; ok {
		close(s.stopCh)
		s.Status = "stopped"
		delete(re.patrols, patrolID)
	}
}

// ListPatrols 列出轮巡会话
func (re *RemoteEnhanced) ListPatrols() []PatrolSession {
	re.mu.RLock()
	defer re.mu.RUnlock()
	var list []PatrolSession
	for _, s := range re.patrols {
		list = append(list, *s)
	}
	return list
}

// LockInput 锁定键鼠
func (re *RemoteEnhanced) LockInput(agentIDs []string, lockType string, teacherID uint) int {
	re.mu.Lock()
	defer re.mu.Unlock()

	count := 0
	for _, agentID := range agentIDs {
		re.locks[agentID] = &LockState{
			AgentID:  agentID,
			Locked:   true,
			LockType: lockType,
			LockedAt: time.Now(),
			LockedBy: teacherID,
		}
		re.hub.SendToAgent(agentID, Frame{
			Type:    "input_lock",
			Payload: mustJSON(map[string]interface{}{"lock_type": lockType, "locked": true}),
		})
		count++
	}
	log.Printf("[remote] 键鼠锁定: %d台, type=%s", count, lockType)
	return count
}

// UnlockInput 解锁键鼠
func (re *RemoteEnhanced) UnlockInput(agentIDs []string) int {
	re.mu.Lock()
	defer re.mu.Unlock()

	count := 0
	for _, agentID := range agentIDs {
		if _, ok := re.locks[agentID]; ok {
			delete(re.locks, agentID)
			re.hub.SendToAgent(agentID, Frame{
				Type:    "input_lock",
				Payload: mustJSON(map[string]interface{}{"locked": false}),
			})
			count++
		}
	}
	return count
}

// LockScreen 锁屏
func (re *RemoteEnhanced) LockScreen(agentIDs []string, teacherID uint) int {
	re.mu.Lock()
	defer re.mu.Unlock()

	count := 0
	for _, agentID := range agentIDs {
		re.hub.SendToAgent(agentID, Frame{
			Type:    "screen_lock",
			Payload: mustJSON(map[string]interface{}{"locked": true, "message": "教师已锁定屏幕"}),
		})
		count++
	}
	log.Printf("[remote] 锁屏: %d台", count)
	return count
}

// UnlockScreen 解锁屏幕
func (re *RemoteEnhanced) UnlockScreen(agentIDs []string) int {
	re.mu.Lock()
	defer re.mu.Unlock()

	count := 0
	for _, agentID := range agentIDs {
		re.hub.SendToAgent(agentID, Frame{
			Type:    "screen_lock",
			Payload: mustJSON(map[string]interface{}{"locked": false}),
		})
		count++
	}
	return count
}

// USBControl USB管控
func (re *RemoteEnhanced) USBControl(agentIDs []string, action string) int {
	re.mu.Lock()
	defer re.mu.Unlock()

	count := 0
	for _, agentID := range agentIDs {
		re.hub.SendToAgent(agentID, Frame{
			Type:    "usb_control",
			Payload: mustJSON(map[string]interface{}{"action": action}),
		})
		count++
	}
	log.Printf("[remote] USB管控: %d台, action=%s", count, action)
	return count
}

// BatchCommand 批量命令
func (re *RemoteEnhanced) BatchCommand(agentIDs []string, cmdType string, payload map[string]interface{}) int {
	re.mu.Lock()
	defer re.mu.Unlock()

	count := 0
	for _, agentID := range agentIDs {
		re.hub.SendToAgent(agentID, Frame{
			Type:    cmdType,
			Payload: mustJSON(payload),
		})
		count++
	}
	return count
}

// GetLocks 获取所有锁定状态
func (re *RemoteEnhanced) GetLocks() []LockState {
	re.mu.RLock()
	defer re.mu.RUnlock()
	var list []LockState
	for _, l := range re.locks {
		list = append(list, *l)
	}
	return list
}
