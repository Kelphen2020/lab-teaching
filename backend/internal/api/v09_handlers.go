package api

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"lab-teaching/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// === V0.9 文件收发增强：断点续传、大文件分片、作业归档 ===

// ChunkUploadSession 分片上传会话
type ChunkUploadSession struct {
	FileID     string    `json:"file_id"`
	FileName   string    `json:"file_name"`
	FileSize   int64     `json:"file_size"`
	ChunkSize  int64     `json:"chunk_size"`
	TotalChunks int      `json:"total_chunks"`
	Uploaded   []int     `json:"uploaded_chunks"`
	Checksum   string    `json:"checksum"` // MD5
	Status     string    `json:"status"`   // uploading / completed / expired
	CreatedAt  time.Time `json:"created_at"`
	TempDir    string    `json:"-"`
}

// FileTransferManager 文件传输管理器
type FileTransferManager struct {
	mu       sync.RWMutex
	sessions map[string]*ChunkUploadSession
	baseDir  string
}

func NewFileTransferManager(baseDir string) *FileTransferManager {
	if baseDir == "" {
		baseDir = "/var/lib/lab-teaching/files"
	}
	os.MkdirAll(baseDir, 0755)
	os.MkdirAll(filepath.Join(baseDir, "chunks"), 0755)
	os.MkdirAll(filepath.Join(baseDir, "homework"), 0755)
	return &FileTransferManager{
		sessions: make(map[string]*ChunkUploadSession),
		baseDir:  baseDir,
	}
}

// InitUpload 初始化分片上传
func (ftm *FileTransferManager) InitUpload(fileName string, fileSize int64, checksum string) *ChunkUploadSession {
	ftm.mu.Lock()
	defer ftm.mu.Unlock()

	chunkSize := int64(5 * 1024 * 1024) // 5MB per chunk
	totalChunks := int((fileSize + chunkSize - 1) / chunkSize)

	fileID := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s_%d_%d", fileName, fileSize, time.Now().UnixNano()))))

	session := &ChunkUploadSession{
		FileID:      fileID,
		FileName:    fileName,
		FileSize:    fileSize,
		ChunkSize:   chunkSize,
		TotalChunks: totalChunks,
		Uploaded:    []int{},
		Checksum:    checksum,
		Status:      "uploading",
		CreatedAt:   time.Now(),
		TempDir:     filepath.Join(ftm.baseDir, "chunks", fileID),
	}
	os.MkdirAll(session.TempDir, 0755)
	ftm.sessions[fileID] = session
	return session
}

// UploadChunk 上传单个分片
func (ftm *FileTransferManager) UploadChunk(fileID string, chunkIndex int, data []byte) error {
	ftm.mu.Lock()
	defer ftm.mu.Unlock()

	session, ok := ftm.sessions[fileID]
	if !ok {
		return fmt.Errorf("上传会话不存在: %s", fileID)
	}
	if chunkIndex < 0 || chunkIndex >= session.TotalChunks {
		return fmt.Errorf("分片索引越界: %d/%d", chunkIndex, session.TotalChunks)
	}

	chunkPath := filepath.Join(session.TempDir, fmt.Sprintf("chunk_%04d", chunkIndex))
	if err := os.WriteFile(chunkPath, data, 0644); err != nil {
		return fmt.Errorf("写入分片失败: %w", err)
	}

	// 记录已上传（去重）
	found := false
	for _, idx := range session.Uploaded {
		if idx == chunkIndex {
			found = true
			break
		}
	}
	if !found {
		session.Uploaded = append(session.Uploaded, chunkIndex)
		sort.Ints(session.Uploaded)
	}

	// 检查是否全部完成
	if len(session.Uploaded) == session.TotalChunks {
		session.Status = "completed"
	}

	return nil
}

// MergeChunks 合并分片
func (ftm *FileTransferManager) MergeChunks(fileID string) (string, error) {
	ftm.mu.Lock()
	defer ftm.mu.Unlock()

	session, ok := ftm.sessions[fileID]
	if !ok {
		return "", fmt.Errorf("上传会话不存在")
	}
	if len(session.Uploaded) != session.TotalChunks {
		return "", fmt.Errorf("分片未全部上传: %d/%d", len(session.Uploaded), session.TotalChunks)
	}

	finalPath := filepath.Join(ftm.baseDir, session.FileName)
	outFile, err := os.Create(finalPath)
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	for i := 0; i < session.TotalChunks; i++ {
		chunkPath := filepath.Join(session.TempDir, fmt.Sprintf("chunk_%04d", i))
		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			return "", fmt.Errorf("读取分片 %d 失败: %w", i, err)
		}
		io.Copy(outFile, chunkFile)
		chunkFile.Close()
	}

	// 清理临时目录
	os.RemoveAll(session.TempDir)
	delete(ftm.sessions, fileID)

	return finalPath, nil
}

// GetUploadProgress 获取上传进度
func (ftm *FileTransferManager) GetUploadProgress(fileID string) *ChunkUploadSession {
	ftm.mu.RLock()
	defer ftm.mu.RUnlock()
	return ftm.sessions[fileID]
}

// === 作业归档 ===

// HomeworkSubmission 作业提交
type HomeworkSubmission struct {
	ID         uint      `json:"id"`
	StudentID  string    `json:"student_id"`
	StudentName string   `json:"student_name"`
	AssignmentID uint    `json:"assignment_id"`
	FileName   string    `json:"file_name"`
	FilePath   string    `json:"file_path"`
	FileSize   int64     `json:"file_size"`
	SubmittedAt time.Time `json:"submitted_at"`
	Status     string    `json:"status"` // submitted / graded
	Score      int       `json:"score"`
	Comment    string    `json:"comment"`
}

// HomeworkManager 作业管理器
type HomeworkManager struct {
	mu          sync.RWMutex
	submissions map[uint]*HomeworkSubmission
	nextID      uint
	baseDir     string
}

func NewHomeworkManager(baseDir string) *HomeworkManager {
	if baseDir == "" {
		baseDir = "/var/lib/lab-teaching/homework"
	}
	os.MkdirAll(baseDir, 0755)
	return &HomeworkManager{
		submissions: make(map[uint]*HomeworkSubmission),
		nextID:      1,
		baseDir:     baseDir,
	}
}

// Submit 提交作业
func (hm *HomeworkManager) Submit(studentID, studentName string, assignmentID uint, fileName string, data []byte) *HomeworkSubmission {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	subDir := filepath.Join(hm.baseDir, fmt.Sprintf("assignment_%d", assignmentID), studentID)
	os.MkdirAll(subDir, 0755)
	filePath := filepath.Join(subDir, fileName)
	os.WriteFile(filePath, data, 0644)

	sub := &HomeworkSubmission{
		ID:           hm.nextID,
		StudentID:    studentID,
		StudentName:  studentName,
		AssignmentID: assignmentID,
		FileName:     fileName,
		FilePath:     filePath,
		FileSize:     int64(len(data)),
		SubmittedAt:  time.Now(),
		Status:       "submitted",
	}
	hm.nextID++
	hm.submissions[sub.ID] = sub
	return sub
}

// ListByAssignment 按作业列出提交
func (hm *HomeworkManager) ListByAssignment(assignmentID uint) []HomeworkSubmission {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	var list []HomeworkSubmission
	for _, s := range hm.submissions {
		if s.AssignmentID == assignmentID {
			list = append(list, *s)
		}
	}
	return list
}

// === 课堂管理：签到、监控墙 ===

// CheckInRecord 签到记录
type CheckInRecord struct {
	ID        uint      `json:"id"`
	RoomID    uint      `json:"room_id"`
	CourseID  uint      `json:"course_id"`
	StudentID string    `json:"student_id"`
	StudentName string  `json:"student_name"`
	SeatNo    string    `json:"seat_no"`
	CheckInAt time.Time `json:"check_in_at"`
	Method    string    `json:"method"` // auto / manual
	Status    string    `json:"status"` // present / late / absent
}

// ClassroomManager 课堂管理器
type ClassroomManager struct {
	mu       sync.RWMutex
	checkins map[uint][]CheckInRecord // courseID -> records
	nextID   uint
}

func NewClassroomManager() *ClassroomManager {
	return &ClassroomManager{
		checkins: make(map[uint][]CheckInRecord),
		nextID:   1,
	}
}

// CheckIn 签到
func (cm *ClassroomManager) CheckIn(roomID, courseID uint, studentID, studentName, seatNo, method string) *CheckInRecord {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	status := "present"
	rec := CheckInRecord{
		ID:          cm.nextID,
		RoomID:      roomID,
		CourseID:    courseID,
		StudentID:   studentID,
		StudentName: studentName,
		SeatNo:      seatNo,
		CheckInAt:   time.Now(),
		Method:      method,
		Status:      status,
	}
	cm.nextID++
	cm.checkins[courseID] = append(cm.checkins[courseID], rec)
	return &rec
}

// GetCheckins 获取签到记录
func (cm *ClassroomManager) GetCheckins(courseID uint) []CheckInRecord {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.checkins[courseID]
}

// === API Handlers ===

// InitChunkUpload 初始化分片上传
func (h *Handler) InitChunkUpload(c *gin.Context) {
	var req struct {
		FileName string `json:"file_name" binding:"required"`
		FileSize int64  `json:"file_size" binding:"required"`
		Checksum string `json:"checksum"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	session := h.FileTransfer.InitUpload(req.FileName, req.FileSize, req.Checksum)
	response.Success(c, session)
}

// UploadChunk 上传分片
func (h *Handler) UploadChunk(c *gin.Context) {
	fileID := c.Param("file_id")
	chunkIdxStr := c.Param("chunk_index")
	chunkIdx, _ := strconv.Atoi(chunkIdxStr)

	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.BadRequest(c, "读取分片数据失败")
		return
	}

	if err := h.FileTransfer.UploadChunk(fileID, chunkIdx, data); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	progress := h.FileTransfer.GetUploadProgress(fileID)
	response.Success(c, progress)
}

// MergeChunks 合并分片
func (h *Handler) MergeChunks(c *gin.Context) {
	fileID := c.Param("file_id")
	path, err := h.FileTransfer.MergeChunks(fileID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"file_path": path})
}

// UploadProgress 查询上传进度
func (h *Handler) UploadProgress(c *gin.Context) {
	fileID := c.Param("file_id")
	session := h.FileTransfer.GetUploadProgress(fileID)
	if session == nil {
		response.NotFound(c, "上传会话不存在")
		return
	}
	response.Success(c, session)
}

// SubmitHomework 提交作业
func (h *Handler) SubmitHomework(c *gin.Context) {
	assignmentID, _ := strconv.ParseUint(c.PostForm("assignment_id"), 10, 64)
	studentID := c.PostForm("student_id")
	studentName := c.PostForm("student_name")

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "文件上传失败")
		return
	}
	defer file.Close()

	data, _ := io.ReadAll(file)
	sub := h.Homework.Submit(studentID, studentName, uint(assignmentID), header.Filename, data)
	response.Success(c, sub)
}

// ListHomework 列出作业提交
func (h *Handler) ListHomework(c *gin.Context) {
	assignmentID, _ := strconv.ParseUint(c.Query("assignment_id"), 10, 64)
	list := h.Homework.ListByAssignment(uint(assignmentID))
	response.Success(c, list)
}

// StudentCheckIn 学生签到
func (h *Handler) StudentCheckIn(c *gin.Context) {
	var req struct {
		RoomID      uint   `json:"room_id" binding:"required"`
		CourseID    uint   `json:"course_id" binding:"required"`
		StudentID   string `json:"student_id" binding:"required"`
		StudentName string `json:"student_name"`
		SeatNo      string `json:"seat_no"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	rec := h.Classroom.CheckIn(req.RoomID, req.CourseID, req.StudentID, req.StudentName, req.SeatNo, "auto")
	response.Success(c, rec)
}

// ListCheckins 获取签到记录
func (h *Handler) ListCheckins(c *gin.Context) {
	courseID, _ := strconv.ParseUint(c.Query("course_id"), 10, 64)
	list := h.Classroom.GetCheckins(uint(courseID))
	response.Success(c, list)
}

// PatrolStart 启动轮巡
func (h *Handler) PatrolStart(c *gin.Context) {
	var req struct {
		AgentIDs []string `json:"agent_ids" binding:"required"`
		Interval int      `json:"interval_sec"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	teacherID := c.GetUint("user_id")
	session := h.RemoteEnhanced.StartPatrol(teacherID, req.AgentIDs, req.Interval)
	response.Success(c, session)
}

// PatrolStop 停止轮巡
func (h *Handler) PatrolStop(c *gin.Context) {
	patrolID := c.Param("id")
	h.RemoteEnhanced.StopPatrol(patrolID)
	response.Success(c, gin.H{"message": "轮巡已停止"})
}

// PatrolList 列出轮巡
func (h *Handler) PatrolList(c *gin.Context) {
	response.Success(c, h.RemoteEnhanced.ListPatrols())
}

// InputLock 键鼠锁定
func (h *Handler) InputLock(c *gin.Context) {
	var req struct {
		AgentIDs []string `json:"agent_ids" binding:"required"`
		LockType string   `json:"lock_type"` // keyboard / mouse / both
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if req.LockType == "" {
		req.LockType = "both"
	}
	teacherID := c.GetUint("user_id")
	count := h.RemoteEnhanced.LockInput(req.AgentIDs, req.LockType, teacherID)
	response.Success(c, gin.H{"locked_count": count})
}

// InputUnlock 键鼠解锁
func (h *Handler) InputUnlock(c *gin.Context) {
	var req struct {
		AgentIDs []string `json:"agent_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	count := h.RemoteEnhanced.UnlockInput(req.AgentIDs)
	response.Success(c, gin.H{"unlocked_count": count})
}

// ScreenLock 锁屏
func (h *Handler) ScreenLock(c *gin.Context) {
	var req struct {
		AgentIDs []string `json:"agent_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	teacherID := c.GetUint("user_id")
	count := h.RemoteEnhanced.LockScreen(req.AgentIDs, teacherID)
	response.Success(c, gin.H{"locked_count": count})
}

// ScreenUnlock 解锁屏幕
func (h *Handler) ScreenUnlock(c *gin.Context) {
	var req struct {
		AgentIDs []string `json:"agent_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	count := h.RemoteEnhanced.UnlockScreen(req.AgentIDs)
	response.Success(c, gin.H{"unlocked_count": count})
}

// USBControl USB管控
func (h *Handler) USBControl(c *gin.Context) {
	var req struct {
		AgentIDs []string `json:"agent_ids" binding:"required"`
		Action   string   `json:"action" binding:"required"` // enable / disable
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	count := h.RemoteEnhanced.USBControl(req.AgentIDs, req.Action)
	response.Success(c, gin.H{"affected_count": count})
}

// MultiRoomBroadcastStart 多机房广播
func (h *Handler) MultiRoomBroadcastStart(c *gin.Context) {
	var req struct {
		RoomMcast map[uint]string `json:"room_mcast" binding:"required"`
		Source    string          `json:"source"`
		Audio     bool            `json:"audio"`
		Record    bool            `json:"record"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if req.Source == "" {
		req.Source = "test"
	}
	teacherID := c.GetUint("user_id")
	task, err := h.MultiRoom.StartMultiRoom(teacherID, req.RoomMcast, req.Source, req.Audio, req.Record)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, task)
}

// MultiRoomBroadcastStop 停止多机房广播
func (h *Handler) MultiRoomBroadcastStop(c *gin.Context) {
	taskID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.MultiRoom.StopMultiRoom(uint(taskID)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"message": "广播已停止"})
}

// MultiRoomBroadcastList 列出多机房广播
func (h *Handler) MultiRoomBroadcastList(c *gin.Context) {
	response.Success(c, h.MultiRoom.ListTasks())
}

// GetLocks 获取锁定状态
func (h *Handler) GetLocks(c *gin.Context) {
	response.Success(c, h.RemoteEnhanced.GetLocks())
}
