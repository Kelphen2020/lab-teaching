package repository

import (
	"database/sql"
	"time"

	"lab-teaching/backend/internal/model"

	"gorm.io/gorm"
)

type Repo struct {
	DB *gorm.DB
}

func (r *Repo) SQLDB() *sql.DB {
	db, _ := r.DB.DB()
	return db
}

func New(db *gorm.DB) *Repo {
	return &Repo{DB: db}
}

// ============ 用户 ============
func (r *Repo) FindUserByUsername(username string) (*model.User, error) {
	var u model.User
	err := r.DB.Where("username = ?", username).First(&u).Error
	return &u, err
}

func (r *Repo) FindUserByID(id uint) (*model.User, error) {
	var u model.User
	err := r.DB.First(&u, id).Error
	return &u, err
}

func (r *Repo) CreateUser(u *model.User) error {
	return r.DB.Create(u).Error
}

func (r *Repo) CountUsers() int64 {
	var n int64
	r.DB.Model(&model.User{}).Count(&n)
	return n
}

// ============ 机房 ============
func (r *Repo) ListRooms() ([]model.Room, error) {
	var rooms []model.Room
	err := r.DB.Order("id").Find(&rooms).Error
	return rooms, err
}

func (r *Repo) GetRoom(id uint) (*model.Room, error) {
	var room model.Room
	err := r.DB.First(&room, id).Error
	return &room, err
}

func (r *Repo) CreateRoom(room *model.Room) error {
	return r.DB.Create(room).Error
}

func (r *Repo) UpdateRoom(room *model.Room) error {
	return r.DB.Save(room).Error
}

func (r *Repo) DeleteRoom(id uint) error {
	return r.DB.Delete(&model.Room{}, id).Error
}

// ============ 机位 ============
func (r *Repo) ListSeatsByRoom(roomID uint) ([]model.Seat, error) {
	var seats []model.Seat
	err := r.DB.Where("room_id = ?", roomID).Order("seat_no").Find(&seats).Error
	return seats, err
}

func (r *Repo) CreateSeat(seat *model.Seat) error {
	return r.DB.Create(seat).Error
}

// ============ V1.0-13 启动策略 ============

// GetSeatByMAC 通过设备 MAC 查找对应座位（Device.MAC -> Seat.DeviceID）
func (r *Repo) GetSeatByMAC(mac string) (*model.Seat, error) {
	var dev model.Device
	if err := r.DB.Where("LOWER(mac) = LOWER(?)", mac).First(&dev).Error; err != nil {
		return nil, err
	}
	var seat model.Seat
	if err := r.DB.Where("device_id = ?", dev.ID).First(&seat).Error; err != nil {
		return nil, err
	}
	return &seat, nil
}

// UpdateRoomBootMode 更新机房级启动策略
func (r *Repo) UpdateRoomBootMode(roomID uint, mode string) error {
	return r.DB.Model(&model.Room{}).Where("id = ?", roomID).Update("boot_mode", mode).Error
}

// UpdateSeatBootMode 更新座位级启动策略
func (r *Repo) UpdateSeatBootMode(seatID uint, mode string) error {
	return r.DB.Model(&model.Seat{}).Where("id = ?", seatID).Update("boot_mode", mode).Error
}

// BatchUpdateSeatBootMode 批量更新指定机房座位的启动策略
func (r *Repo) BatchUpdateSeatBootMode(roomID uint, seatNos []string, mode string) (int64, error) {
	q := r.DB.Model(&model.Seat{}).Where("room_id = ?", roomID)
	if len(seatNos) > 0 {
		q = q.Where("seat_no IN ?", seatNos)
	}
	res := q.Update("boot_mode", mode)
	return res.RowsAffected, res.Error
}

// ============ 设备 ============
func (r *Repo) ListDevices() ([]model.Device, error) {
	var devices []model.Device
	err := r.DB.Order("id").Find(&devices).Error
	return devices, err
}

func (r *Repo) CreateDevice(d *model.Device) error {
	return r.DB.Create(d).Error
}

// ============ Agent 会话 ============
func (r *Repo) UpsertAgentSession(s *model.AgentSession) error {
	var existing model.AgentSession
	err := r.DB.Where("agent_id = ?", s.AgentID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.DB.Create(s).Error
	}
	if err != nil {
		return err
	}
	s.ID = existing.ID
	return r.DB.Save(s).Error
}

func (r *Repo) SetAgentOnline(agentID string, online bool) error {
	return r.DB.Model(&model.AgentSession{}).
		Where("agent_id = ?", agentID).
		Updates(map[string]interface{}{"online": online, "last_heartbeat": gorm.Expr("now()")}).Error
}

func (r *Repo) ListAgentSessions() ([]model.AgentSession, error) {
	var sessions []model.AgentSession
	err := r.DB.Order("id").Find(&sessions).Error
	return sessions, err
}

func (r *Repo) ListAgentSessionsPaged(page, size int) ([]model.AgentSession, int64, error) {
	var sessions []model.AgentSession
	var total int64
	q := r.DB.Model(&model.AgentSession{})
	q.Count(&total)
	err := q.Order("id").Offset((page - 1) * size).Limit(size).Find(&sessions).Error
	return sessions, total, err
}

func (r *Repo) CountOnlineAgents() int64 {
	var n int64
	r.DB.Model(&model.AgentSession{}).Where("online = ?", true).Count(&n)
	return n
}

// ============ 无盘镜像 ============
func (r *Repo) ListDiskImages() ([]model.DiskImage, error) {
	var images []model.DiskImage
	err := r.DB.Order("id").Find(&images).Error
	return images, err
}

func (r *Repo) CreateDiskImage(img *model.DiskImage) error {
	return r.DB.Create(img).Error
}

// ============ 广播任务 ============
func (r *Repo) CreateBroadcastTask(t *model.BroadcastTask) error {
	return r.DB.Create(t).Error
}

func (r *Repo) ListBroadcastTasks() ([]model.BroadcastTask, error) {
	var tasks []model.BroadcastTask
	err := r.DB.Order("id desc").Limit(50).Find(&tasks).Error
	return tasks, err
}

func (r *Repo) UpdateBroadcastStatus(taskID uint, status string) error {
	return r.DB.Model(&model.BroadcastTask{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{"status": status, "end_at": gorm.Expr("now()")}).Error
}

// ============ 课程/班级/学生 ============
func (r *Repo) ListCourses() ([]model.Course, error) {
	var list []model.Course
	err := r.DB.Order("id").Find(&list).Error
	return list, err
}

func (r *Repo) ListCoursesPaged(page, size int) ([]model.Course, int64, error) {
	var list []model.Course
	var total int64
	q := r.DB.Model(&model.Course{})
	q.Count(&total)
	err := q.Order("id").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

func (r *Repo) ListClazzes() ([]model.Clazz, error) {
	var list []model.Clazz
	err := r.DB.Order("id").Find(&list).Error
	return list, err
}

func (r *Repo) ListClazzesPaged(page, size int) ([]model.Clazz, int64, error) {
	var list []model.Clazz
	var total int64
	q := r.DB.Model(&model.Clazz{})
	q.Count(&total)
	err := q.Order("id").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

func (r *Repo) ListStudents() ([]model.Student, error) {
	var list []model.Student
	err := r.DB.Order("id").Find(&list).Error
	return list, err
}

// ============ 教学 CRUD（P0-1）============

// Course
func (r *Repo) CreateCourse(c *model.Course) error { return r.DB.Create(c).Error }
func (r *Repo) GetCourse(id uint) (*model.Course, error) {
	var c model.Course; err := r.DB.First(&c, id).Error; return &c, err
}
func (r *Repo) UpdateCourse(c *model.Course) error { return r.DB.Save(c).Error }
func (r *Repo) DeleteCourse(id uint) error { return r.DB.Delete(&model.Course{}, id).Error }

// Clazz
func (r *Repo) CreateClazz(c *model.Clazz) error { return r.DB.Create(c).Error }
func (r *Repo) GetClazz(id uint) (*model.Clazz, error) {
	var c model.Clazz; err := r.DB.First(&c, id).Error; return &c, err
}
func (r *Repo) UpdateClazz(c *model.Clazz) error { return r.DB.Save(c).Error }
func (r *Repo) DeleteClazz(id uint) error { return r.DB.Delete(&model.Clazz{}, id).Error }

// Student
func (r *Repo) CreateStudent(s *model.Student) error { return r.DB.Create(s).Error }
func (r *Repo) GetStudent(id uint) (*model.Student, error) {
	var s model.Student; err := r.DB.First(&s, id).Error; return &s, err
}
func (r *Repo) UpdateStudent(s *model.Student) error { return r.DB.Save(s).Error }
func (r *Repo) DeleteStudent(id uint) error { return r.DB.Delete(&model.Student{}, id).Error }
func (r *Repo) SearchStudents(keyword string, clazzID uint) ([]model.Student, error) {
	q := r.DB.Model(&model.Student{})
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("sno LIKE ? OR name LIKE ?", like, like)
	}
	if clazzID > 0 {
		q = q.Where("clazz_id = ?", clazzID)
	}
	var list []model.Student
	err := q.Order("id").Find(&list).Error
	return list, err
}

func (r *Repo) SearchStudentsPaged(keyword string, clazzID uint, page, size int) ([]model.Student, int64, error) {
	q := r.DB.Model(&model.Student{})
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("sno LIKE ? OR name LIKE ?", like, like)
	}
	if clazzID > 0 {
		q = q.Where("clazz_id = ?", clazzID)
	}
	var total int64
	q.Count(&total)
	var list []model.Student
	err := q.Order("id").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// Schedule
func (r *Repo) CreateSchedule(s *model.Schedule) error { return r.DB.Create(s).Error }
func (r *Repo) GetSchedule(id uint) (*model.Schedule, error) {
	var s model.Schedule; err := r.DB.First(&s, id).Error; return &s, err
}
func (r *Repo) UpdateSchedule(s *model.Schedule) error { return r.DB.Save(s).Error }
func (r *Repo) DeleteSchedule(id uint) error { return r.DB.Delete(&model.Schedule{}, id).Error }
func (r *Repo) ListSchedules(clazzID, courseID uint, weekday int) ([]model.Schedule, error) {
	q := r.DB.Model(&model.Schedule{})
	if clazzID > 0 {
		q = q.Where("clazz_id = ?", clazzID)
	}
	if courseID > 0 {
		q = q.Where("course_id = ?", courseID)
	}
	if weekday > 0 {
		q = q.Where("weekday = ?", weekday)
	}
	var list []model.Schedule
	err := q.Order("weekday,period").Find(&list).Error
	return list, err
}

// ============ 审计 ============
func (r *Repo) CreateAudit(a *model.Audit) error {
	return r.DB.Create(a).Error
}

func (r *Repo) ListAudits(limit int) ([]model.Audit, error) {
	var list []model.Audit
	err := r.DB.Order("id desc").Limit(limit).Find(&list).Error
	return list, err
}

// SearchAudits 审计查询：关键词过滤(action/target/detail/ip 模糊)+分页
func (r *Repo) SearchAudits(keyword string, page, size int) ([]model.Audit, int64, error) {
	q := r.DB.Model(&model.Audit{})
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("action LIKE ? OR target LIKE ? OR detail LIKE ? OR ip LIKE ?", like, like, like, like)
	}
	var total int64
	q.Count(&total)
	var list []model.Audit
	if page < 1 {
		page = 1
	}
	if size <= 0 || size > 200 {
		size = 20
	}
	err := q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

func (r *Repo) SearchAuditsAll(keyword string) ([]model.Audit, error) {
	q := r.DB.Model(&model.Audit{})
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("action LIKE ? OR target LIKE ? OR detail LIKE ? OR ip LIKE ?", like, like, like, like)
	}
	var list []model.Audit
	err := q.Order("id desc").Limit(10000).Find(&list).Error
	return list, err
}

// ============ 文件收发 ============
func (r *Repo) CreateFileTask(t *model.FileTask) error {
	return r.DB.Create(t).Error
}

func (r *Repo) GetFileTask(id uint) (*model.FileTask, error) {
	var t model.FileTask
	err := r.DB.First(&t, id).Error
	return &t, err
}

func (r *Repo) ListFileTasks() ([]model.FileTask, error) {
	var list []model.FileTask
	err := r.DB.Order("id desc").Limit(50).Find(&list).Error
	return list, err
}

func (r *Repo) UpdateFileTask(id uint, updates map[string]interface{}) error {
	return r.DB.Model(&model.FileTask{}).Where("id = ?", id).Updates(updates).Error
}

func (r *Repo) BatchCreateFileProgress(records []model.FileProgress) error {
	return r.DB.Create(&records).Error
}

func (r *Repo) ListFileProgress(taskID uint) ([]model.FileProgress, error) {
	var list []model.FileProgress
	err := r.DB.Where("task_id = ?", taskID).Find(&list).Error
	return list, err
}

func (r *Repo) UpdateFileProgress(taskID uint, agentID string, updates map[string]interface{}) error {
	return r.DB.Model(&model.FileProgress{}).
		Where("task_id = ? AND agent_id = ?", taskID, agentID).
		Updates(updates).Error
}

// ============ 远程控制 ============
func (r *Repo) CreateRemoteSession(s *model.RemoteSession) error {
	return r.DB.Create(s).Error
}

func (r *Repo) GetRemoteSession(id uint) (*model.RemoteSession, error) {
	var s model.RemoteSession
	err := r.DB.First(&s, id).Error
	return &s, err
}

func (r *Repo) UpdateRemoteSession(id uint, updates map[string]interface{}) error {
	return r.DB.Model(&model.RemoteSession{}).Where("id = ?", id).Updates(updates).Error
}

func (r *Repo) ListRemoteSessions() ([]model.RemoteSession, error) {
	var list []model.RemoteSession
	err := r.DB.Order("id desc").Limit(50).Find(&list).Error
	return list, err
}

// ============ 考试域 ============
func (r *Repo) CreateQuestion(q *model.Question) error {
	return r.DB.Create(q).Error
}

func (r *Repo) ListQuestions(qType string, limit int) ([]model.Question, error) {
	var list []model.Question
	db := r.DB.Order("id desc")
	if qType != "" {
		db = db.Where("type = ?", qType)
	}
	if limit <= 0 {
		limit = 100
	}
	err := db.Limit(limit).Find(&list).Error
	return list, err
}

func (r *Repo) GetQuestion(id uint) (*model.Question, error) {
	var q model.Question
	err := r.DB.First(&q, id).Error
	return &q, err
}

func (r *Repo) CreatePaper(p *model.Paper) error {
	return r.DB.Create(p).Error
}

func (r *Repo) GetPaper(id uint) (*model.Paper, error) {
	var p model.Paper
	err := r.DB.First(&p, id).Error
	return &p, err
}

func (r *Repo) ListPapers() ([]model.Paper, error) {
	var list []model.Paper
	err := r.DB.Order("id desc").Find(&list).Error
	return list, err
}

func (r *Repo) UpdatePaper(id uint, updates map[string]interface{}) error {
	return r.DB.Model(&model.Paper{}).Where("id = ?", id).Updates(updates).Error
}

func (r *Repo) AddPaperQuestions(items []model.PaperQuestion) error {
	return r.DB.Create(&items).Error
}

func (r *Repo) ListPaperQuestions(paperID uint) ([]model.PaperQuestion, error) {
	var list []model.PaperQuestion
	err := r.DB.Where("paper_id = ?", paperID).Order("sort_order").Find(&list).Error
	return list, err
}

func (r *Repo) CreateExam(e *model.Exam) error {
	return r.DB.Create(e).Error
}

func (r *Repo) GetExam(id uint) (*model.Exam, error) {
	var e model.Exam
	err := r.DB.First(&e, id).Error
	return &e, err
}

func (r *Repo) ListExams() ([]model.Exam, error) {
	var list []model.Exam
	err := r.DB.Order("id desc").Find(&list).Error
	return list, err
}

func (r *Repo) UpdateExam(id uint, updates map[string]interface{}) error {
	return r.DB.Model(&model.Exam{}).Where("id = ?", id).Updates(updates).Error
}

func (r *Repo) CreateExamAnswer(a *model.ExamAnswer) error {
	return r.DB.Create(a).Error
}

func (r *Repo) BatchCreateExamAnswers(answers []model.ExamAnswer) error {
	return r.DB.Create(&answers).Error
}

func (r *Repo) ListExamAnswers(examID uint, agentID string) ([]model.ExamAnswer, error) {
	var list []model.ExamAnswer
	db := r.DB.Where("exam_id = ?", examID)
	if agentID != "" {
		db = db.Where("agent_id = ?", agentID)
	}
	err := db.Find(&list).Error
	return list, err
}

func (r *Repo) GetExamByAccessCode(code string) (*model.Exam, error) {
	var e model.Exam
	err := r.DB.Where("access_code = ?", code).First(&e).Error
	return &e, err
}

func (r *Repo) ListExamAnswersBySno(examID uint, sno string) ([]model.ExamAnswer, error) {
	var list []model.ExamAnswer
	err := r.DB.Where("exam_id = ? AND student_sno = ?", examID, sno).Find(&list).Error
	return list, err
}

func (r *Repo) UpdateQuestion(id uint, updates map[string]interface{}) error {
	return r.DB.Model(&model.Question{}).Where("id = ?", id).Updates(updates).Error
}

func (r *Repo) DeleteQuestion(id uint) error {
	return r.DB.Delete(&model.Question{}, id).Error
}

func (r *Repo) GetQuestionsBatch(ids []uint) ([]model.Question, error) {
	var list []model.Question
	err := r.DB.Where("id IN ?", ids).Find(&list).Error
	return list, err
}

// ListQuestionsByFilter 按条件筛选题目（用于随机组卷）
func (r *Repo) ListQuestionsByFilter(qType string, difficulty int, knowledge string, limit int) ([]model.Question, error) {
	var list []model.Question
	db := r.DB.Model(&model.Question{})
	if qType != "" {
		db = db.Where("type = ?", qType)
	}
	if difficulty > 0 {
		db = db.Where("difficulty = ?", difficulty)
	}
	if knowledge != "" {
		db = db.Where("knowledge LIKE ?", "%"+knowledge+"%")
	}
	if limit > 0 {
		db = db.Limit(limit)
	}
	err := db.Order("RANDOM()").Find(&list).Error
	return list, err
}

func (r *Repo) DeletePaper(id uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("paper_id = ?", id).Delete(&model.PaperQuestion{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Paper{}, id).Error
	})
}

func (r *Repo) DeleteExam(id uint) error {
	return r.DB.Delete(&model.Exam{}, id).Error
}

func (r *Repo) CreateExamViolation(v *model.ExamViolation) error {
	return r.DB.Create(v).Error
}

func (r *Repo) ListExamViolations(examID uint) ([]model.ExamViolation, error) {
	var list []model.ExamViolation
	err := r.DB.Where("exam_id = ?", examID).Order("id desc").Find(&list).Error
	return list, err
}

// ============ 统计 ============
func (r *Repo) CountExamsByStatus(status string) int64 {
	var n int64
	if status == "" {
		r.DB.Model(&model.Exam{}).Count(&n)
	} else {
		r.DB.Model(&model.Exam{}).Where("status = ?", status).Count(&n)
	}
	return n
}

func (r *Repo) CountQuestions() int64 {
	var n int64
	r.DB.Model(&model.Question{}).Count(&n)
	return n
}

func (r *Repo) CountPapers() int64 {
	var n int64
	r.DB.Model(&model.Paper{}).Count(&n)
	return n
}

func (r *Repo) CountViolations() int64 {
	var n int64
	r.DB.Model(&model.ExamViolation{}).Count(&n)
	return n
}

func (r *Repo) RecentViolations(limit int) ([]model.ExamViolation, error) {
	var list []model.ExamViolation
	err := r.DB.Order("id desc").Limit(limit).Find(&list).Error
	return list, err
}

func (r *Repo) ListAgentSessionsOnline() ([]model.AgentSession, error) {
	var list []model.AgentSession
	err := r.DB.Where("online = ?", true).Find(&list).Error
	return list, err
}

func (r *Repo) ListAgentSessionsOnlinePaged(page, size int) ([]model.AgentSession, int64, error) {
	var list []model.AgentSession
	var total int64
	q := r.DB.Model(&model.AgentSession{}).Where("online = ?", true)
	q.Count(&total)
	err := q.Order("id").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

func (r *Repo) CountDevicesByStatus(status string) int64 {
	var n int64
	r.DB.Model(&model.Device{}).Where("status = ?", status).Count(&n)
	return n
}

// ============ Agent 日志 ============
func (r *Repo) CreateAgentLog(log *model.AgentLog) error { return r.DB.Create(log).Error }
func (r *Repo) ListAgentLogs(agentID string, limit int) ([]model.AgentLog, error) {
	var list []model.AgentLog
	q := r.DB.Model(&model.AgentLog{})
	if agentID != "" { q = q.Where("agent_id = ?", agentID) }
	err := q.Order("id desc").Limit(limit).Find(&list).Error
	return list, err
}

func (r *Repo) FindStaleOnlineAgents(timeout time.Duration) []string {
	var agents []model.AgentSession
	cutoff := time.Now().Add(-timeout)
	r.DB.Where("online = ? AND last_heartbeat < ?", true, cutoff).Find(&agents)
	ids := make([]string, 0, len(agents))
	for _, a := range agents {
		ids = append(ids, a.AgentID)
	}
	return ids
}

