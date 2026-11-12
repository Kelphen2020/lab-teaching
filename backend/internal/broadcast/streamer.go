package broadcast

import (
	"fmt"
	"log"
	"os/exec"
	"sync"
	"time"
)

// StreamConfig 推流配置
type StreamConfig struct {
	TaskID    uint   // 广播任务ID
	McastAddr string // 组播地址 如 239.1.1.1:5004
	Source    string // 采集源: "screen" | "test" | "file:/path"
	Width     int
	Height    int
	FPS       int
	Bitrate   string // 如 "2M"
	Display   string // X11 DISPLAY（教师机）
}

// Stream 一个活跃的推流会话
type Stream struct {
	Config  StreamConfig
	Cmd     *exec.Cmd
	StartAt time.Time
	Status  string // running / stopped / error
}

// Manager 管理所有广播推流进程
type Manager struct {
	mu      sync.RWMutex
	streams map[uint]*Stream // taskID -> Stream
}

func NewManager() *Manager {
	return &Manager{streams: make(map[uint]*Stream)}
}

// Start 启动一个推流会话
func (m *Manager) Start(cfg StreamConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.streams[cfg.TaskID]; ok && s.Status == "running" {
		return fmt.Errorf("任务 %d 已在推流中", cfg.TaskID)
	}

	// 默认参数
	if cfg.Width == 0 {
		cfg.Width = 1920
	}
	if cfg.Height == 0 {
		cfg.Height = 1080
	}
	if cfg.FPS == 0 {
		cfg.FPS = 25
	}
	if cfg.Bitrate == "" {
		cfg.Bitrate = "4M"
	}

	args := m.buildFFmpegArgs(cfg)
	cmd := exec.Command("ffmpeg", args...)
	cmd.Env = append(cmd.Environ(), "DISPLAY="+cfg.Display)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 ffmpeg 失败: %w", err)
	}

	s := &Stream{Config: cfg, Cmd: cmd, StartAt: time.Now(), Status: "running"}
	m.streams[cfg.TaskID] = s
	log.Printf("[broadcast] 推流启动 task=%d mcast=%s source=%s pid=%d",
		cfg.TaskID, cfg.McastAddr, cfg.Source, cmd.Process.Pid)

	// 后台监控进程退出
	go func() {
		err := cmd.Wait()
		m.mu.Lock()
		defer m.mu.Unlock()
		if s, ok := m.streams[cfg.TaskID]; ok {
			if err != nil {
				s.Status = "error"
				log.Printf("[broadcast] 推流异常退出 task=%d: %v", cfg.TaskID, err)
			} else {
				s.Status = "stopped"
			}
		}
	}()

	return nil
}

// Stop 停止推流
func (m *Manager) Stop(taskID uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.streams[taskID]
	if !ok {
		return fmt.Errorf("任务 %d 不存在", taskID)
	}
	if s.Cmd != nil && s.Cmd.Process != nil {
		s.Cmd.Process.Kill()
	}
	s.Status = "stopped"
	log.Printf("[broadcast] 推流停止 task=%d", taskID)
	return nil
}

// StopAll 停止所有推流
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.streams {
		if s.Cmd != nil && s.Cmd.Process != nil {
			s.Cmd.Process.Kill()
		}
		s.Status = "stopped"
		log.Printf("[broadcast] 停止推流 task=%d", id)
	}
}

// Status 查询推流状态
func (m *Manager) Status(taskID uint) *Stream {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.streams[taskID]
}

// ActiveStreams 返回所有活跃推流
func (m *Manager) ActiveStreams() []Stream {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []Stream
	for _, s := range m.streams {
		if s.Status == "running" {
			list = append(list, *s)
		}
	}
	return list
}

// buildFFmpegArgs 构建 ffmpeg 命令行参数
func (m *Manager) buildFFmpegArgs(cfg StreamConfig) []string {
	args := []string{"-hide_banner", "-loglevel", "warning"}

	// 输入源
	switch cfg.Source {
	case "screen":
		// X11 屏幕采集（教师机有显示环境）
		args = append(args,
			"-f", "x11grab",
			"-video_size", fmt.Sprintf("%dx%d", cfg.Width, cfg.Height),
			"-framerate", fmt.Sprintf("%d", cfg.FPS),
			"-i", fmt.Sprintf("%s+0,0", cfg.Display),
		)
	case "test":
		// 测试图案（无头环境验证用）
		args = append(args,
			"-re",
			"-f", "lavfi",
			"-i", fmt.Sprintf("testsrc=size=%dx%d:rate=%d", cfg.Width, cfg.Height, cfg.FPS),
		)
	default:
		// 文件/设备源
		args = append(args, "-re", "-i", cfg.Source)
	}

	// 编码 + 输出
	args = append(args,
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-tune", "zerolatency",
		"-b:v", cfg.Bitrate,
		"-g", fmt.Sprintf("%d", cfg.FPS), // GOP = 1秒（快速加入）
		"-x264-params", "repeat_headers=1", // 每个关键帧内联 SPS/PPS
		"-pix_fmt", "yuv420p",
		"-f", "rtp_mpegts",
		fmt.Sprintf("rtp://%s", cfg.McastAddr),
	)

	return args
}
