package broadcast

import (
	"fmt"
	"log"
	"os/exec"
	"sync"
	"time"
)

// === V0.9 增强：多机房广播、录制回放、音频同步 ===

// MultiRoomBroadcast 多机房广播任务
type MultiRoomBroadcast struct {
	ID        uint      `json:"id"`
	TeacherID uint      `json:"teacher_id"`
	RoomIDs   []uint    `json:"room_ids"`
	Source    string    `json:"source"`    // screen / file / test
	Audio     bool      `json:"audio"`     // 是否同步音频
	Record    bool      `json:"record"`    // 是否录制
	RecordDir string    `json:"record_dir"`
	Status    string    `json:"status"`    // running / stopped / error
	StartAt   time.Time `json:"start_at"`
	Streams   map[uint]*Stream `json:"-"` // roomID -> stream
}

// RecordingSession 录制会话
type RecordingSession struct {
	ID        uint      `json:"id"`
	TaskID    uint      `json:"task_id"`
	FilePath  string    `json:"file_path"`
	Duration  float64   `json:"duration"`
	Size      int64     `json:"size"`
	Status    string    `json:"status"` // recording / completed / error
	StartAt   time.Time `json:"start_at"`
	Cmd       *exec.Cmd `json:"-"`
}

// AudioManager 音频同步管理
type AudioManager struct {
	mu       sync.RWMutex
	sessions map[uint]*AudioSession // taskID -> session
}

type AudioSession struct {
	TaskID   uint
	Device   string // 音频设备 alsa/pulse
	Cmd      *exec.Cmd
	Status   string
}

func NewAudioManager() *AudioManager {
	return &AudioManager{sessions: make(map[uint]*AudioSession)}
}

// StartAudio 启动音频采集推流（RTP）
func (am *AudioManager) StartAudio(taskID uint, mcastAddr string, device string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if _, ok := am.sessions[taskID]; ok {
		return fmt.Errorf("任务 %d 音频已在推流", taskID)
	}

	if device == "" {
		device = "default"
	}

	// 音频 RTP 推流: ALSA -> Opus -> RTP
	args := []string{
		"-hide_banner", "-loglevel", "warning",
		"-f", "alsa", "-i", device,
		"-c:a", "libopus", "-b:a", "128k",
		"-f", "rtp",
		fmt.Sprintf("rtp://%s", mcastAddr),
	}

	cmd := exec.Command("ffmpeg", args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动音频推流失败: %w", err)
	}

	am.sessions[taskID] = &AudioSession{
		TaskID: taskID,
		Device: device,
		Cmd:    cmd,
		Status: "running",
	}
	log.Printf("[broadcast] 音频推流启动 task=%d device=%s", taskID, device)

	go func() {
		cmd.Wait()
		am.mu.Lock()
		if s, ok := am.sessions[taskID]; ok {
			s.Status = "stopped"
		}
		am.mu.Unlock()
	}()

	return nil
}

// StopAudio 停止音频推流
func (am *AudioManager) StopAudio(taskID uint) {
	am.mu.Lock()
	defer am.mu.Unlock()
	if s, ok := am.sessions[taskID]; ok {
		if s.Cmd != nil && s.Cmd.Process != nil {
			s.Cmd.Process.Kill()
		}
		s.Status = "stopped"
		delete(am.sessions, taskID)
	}
}

// === 录制管理 ===

// StartRecording 启动录制（将推流同时写入文件）
func (m *Manager) StartRecording(taskID uint, mcastAddr string, recordDir string) (*RecordingSession, error) {
	if recordDir == "" {
		recordDir = "/var/lib/lab-teaching/recordings"
	}

	filePath := fmt.Sprintf("%s/broadcast_%d_%s.ts", recordDir, taskID, time.Now().Format("20060102_150405"))

	// 从组播地址录制
	args := []string{
		"-hide_banner", "-loglevel", "warning",
		"-i", fmt.Sprintf("rtp://%s", mcastAddr),
		"-c", "copy",
		"-f", "mpegts",
		filePath,
	}

	cmd := exec.Command("ffmpeg", args...)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动录制失败: %w", err)
	}

	session := &RecordingSession{
		TaskID:   taskID,
		FilePath: filePath,
		Status:   "recording",
		StartAt:  time.Now(),
		Cmd:      cmd,
	}

	go func() {
		cmd.Wait()
		session.Status = "completed"
		session.Duration = time.Since(session.StartAt).Seconds()
		log.Printf("[broadcast] 录制完成: %s (%.1fs)", filePath, session.Duration)
	}()

	log.Printf("[broadcast] 录制启动 task=%d -> %s", taskID, filePath)
	return session, nil
}

// StopRecording 停止录制
func (session *RecordingSession) Stop() {
	if session.Cmd != nil && session.Cmd.Process != nil {
		session.Cmd.Process.Kill()
	}
	session.Status = "completed"
	session.Duration = time.Since(session.StartAt).Seconds()
}

// === 多机房广播管理 ===

// MultiRoomManager 多机房广播管理器
type MultiRoomManager struct {
	mu       sync.RWMutex
	tasks    map[uint]*MultiRoomBroadcast
	nextID   uint
	audio    *AudioManager
	streamer *Manager
}

func NewMultiRoomManager(streamer *Manager) *MultiRoomManager {
	return &MultiRoomManager{
		tasks:    make(map[uint]*MultiRoomBroadcast),
		nextID:   1,
		audio:    NewAudioManager(),
		streamer: streamer,
	}
}

// StartMultiRoom 启动多机房广播
// roomMcastMap: roomID -> 组播地址
func (mrm *MultiRoomManager) StartMultiRoom(teacherID uint, roomMcastMap map[uint]string, source string, audio bool, record bool) (*MultiRoomBroadcast, error) {
	mrm.mu.Lock()
	defer mrm.mu.Unlock()

	task := &MultiRoomBroadcast{
		ID:        mrm.nextID,
		TeacherID: teacherID,
		Source:    source,
		Audio:     audio,
		Record:    record,
		Status:    "running",
		StartAt:   time.Now(),
		Streams:   make(map[uint]*Stream),
	}
	mrm.nextID++

	var roomIDs []uint
	for roomID, mcast := range roomMcastMap {
		roomIDs = append(roomIDs, roomID)

		cfg := StreamConfig{
			TaskID:    task.ID*1000 + roomID, // 唯一子任务ID
			McastAddr: mcast,
			Source:    source,
			Width:     1920,
			Height:    1080,
			FPS:       25,
			Bitrate:   "4M",
			Display:   ":0",
		}

		if err := mrm.streamer.Start(cfg); err != nil {
			log.Printf("[broadcast] 机房 %d 推流失败: %v", roomID, err)
			continue
		}
		task.Streams[roomID] = mrm.streamer.Status(cfg.TaskID)
	}

	task.RoomIDs = roomIDs

	// 音频同步
	if audio {
		// 音频使用独立组播地址（视频地址端口+2）
		for _, mcast := range roomMcastMap {
			audioMcast := mcast // 简化：同地址
			mrm.audio.StartAudio(task.ID, audioMcast, "default")
			break // 只需一个音频源
		}
	}

	mrm.tasks[task.ID] = task
	log.Printf("[broadcast] 多机房广播启动 task=%d rooms=%v", task.ID, roomIDs)
	return task, nil
}

// StopMultiRoom 停止多机房广播
func (mrm *MultiRoomManager) StopMultiRoom(taskID uint) error {
	mrm.mu.Lock()
	defer mrm.mu.Unlock()

	task, ok := mrm.tasks[taskID]
	if !ok {
		return fmt.Errorf("广播任务 %d 不存在", taskID)
	}

	// 停止所有子推流
	for roomID := range task.Streams {
		mrm.streamer.Stop(taskID*1000 + roomID)
	}

	// 停止音频
	mrm.audio.StopAudio(taskID)

	task.Status = "stopped"
	log.Printf("[broadcast] 多机房广播停止 task=%d", taskID)
	return nil
}

// ListTasks 列出所有多机房广播任务
func (mrm *MultiRoomManager) ListTasks() []MultiRoomBroadcast {
	mrm.mu.RLock()
	defer mrm.mu.RUnlock()
	var list []MultiRoomBroadcast
	for _, t := range mrm.tasks {
		list = append(list, *t)
	}
	return list
}

// GetTask 获取单个任务
func (mrm *MultiRoomManager) GetTask(taskID uint) *MultiRoomBroadcast {
	mrm.mu.RLock()
	defer mrm.mu.RUnlock()
	return mrm.tasks[taskID]
}
