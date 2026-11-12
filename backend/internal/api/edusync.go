package api

import (
	"strconv"
	"strings"

	"lab-teaching/backend/internal/model"
	"lab-teaching/backend/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ============ V1.x-16 教务系统对接（课表/名单/成绩） ============
//
// 对接模式：
//   1. 管理端手工导入（JWT 认证，/edu/* 全部接口）
//   2. 教务系统推送（X-Sync-Key 头鉴权，免登录 /api/v1/edu/push/*）
//   3. 成绩查询：按考试聚合学生成绩，供教务系统拉取或导出 CSV

// checkSyncKey 校验教务推送密钥，返回对接配置
func (h *Handler) checkSyncKey(c *gin.Context) (*model.EduSyncConfig, bool) {
	key := c.GetHeader("X-Sync-Key")
	if key == "" {
		response.Unauthorized(c, "缺少 X-Sync-Key 请求头")
		return nil, false
	}
	var cfg model.EduSyncConfig
	if err := h.Repo.DB.Where("sync_key = ? AND enabled = ?", key, true).First(&cfg).Error; err != nil {
		response.Unauthorized(c, "同步密钥无效或未启用")
		return nil, false
	}
	return &cfg, true
}

// logSync 记录同步日志
func (h *Handler) logSync(source, action string, total, ok, failed int, detail string, ip string) {
	h.Repo.DB.Create(&model.EduSyncLog{
		Source: source, Action: action, Total: total,
		Succeeded: ok, Failed: failed, Detail: detail, IP: ip,
	})
}

// ---- 学生名单同步 ----
type eduStudentItem struct {
	Sno     string `json:"sno" binding:"required"`
	Name    string `json:"name" binding:"required"`
	Clazz   string `json:"clazz"`   // 班级名（不存在则创建）
	CardNo  string `json:"card_no"` // 一卡通号（可选，对接 p4-2）
}

func (h *Handler) syncStudents(source string, items []eduStudentItem, c *gin.Context) {
	ok, failed, details := 0, 0, []string{}
	err := h.Repo.DB.Transaction(func(tx *gorm.DB) error {
		for _, it := range items {
			var clazzID *uint
			if it.Clazz != "" {
				var cl model.Clazz
				if e := tx.Where("name = ?", it.Clazz).First(&cl).Error; e != nil {
					cl = model.Clazz{Name: it.Clazz}
					if e := tx.Create(&cl).Error; e != nil {
						return e
					}
				}
				clazzID = &cl.ID
			}
			var st model.Student
			e := tx.Where("sno = ?", it.Sno).First(&st).Error
			if e == gorm.ErrRecordNotFound {
				st = model.Student{Sno: it.Sno, Name: it.Name, ClazzID: clazzID, CardNo: it.CardNo}
				if e := tx.Create(&st).Error; e != nil {
					failed++
					details = append(details, it.Sno+":"+e.Error())
					continue
				}
				ok++
			} else if e == nil {
				st.Name = it.Name
				st.ClazzID = clazzID
				if it.CardNo != "" {
					st.CardNo = it.CardNo
				}
				if e := tx.Save(&st).Error; e != nil {
					failed++
					details = append(details, it.Sno+":"+e.Error())
					continue
				}
				ok++
			} else {
				failed++
				details = append(details, it.Sno+":"+e.Error())
			}
		}
		return nil
	})
	if err != nil {
		response.ServerError(c, "同步失败: "+err.Error())
		return
	}
	detail := strings.Join(details, "; ")
	if len(detail) > 1000 {
		detail = detail[:1000]
	}
	h.logSync(source, "students", len(items), ok, failed, detail, c.ClientIP())
	response.Success(c, gin.H{"total": len(items), "succeeded": ok, "failed": failed})
}

// SyncStudentsPush 教务推送学生名单（X-Sync-Key 鉴权）
// POST /api/v1/edu/push/students
func (h *Handler) SyncStudentsPush(c *gin.Context) {
	cfg, okAuth := h.checkSyncKey(c)
	if !okAuth {
		return
	}
	var items []eduStudentItem
	if err := c.ShouldBindJSON(&items); err != nil || len(items) == 0 {
		response.BadRequest(c, "请求体须为非空学生数组 [{sno,name,clazz}]")
		return
	}
	h.syncStudents(cfg.Name, items, c)
}

// ---- 课程同步 ----
type eduCourseItem struct {
	Name   string  `json:"name" binding:"required"`
	Credit float64 `json:"credit"`
}

// SyncCoursesPush POST /api/v1/edu/push/courses
func (h *Handler) SyncCoursesPush(c *gin.Context) {
	cfg, okAuth := h.checkSyncKey(c)
	if !okAuth {
		return
	}
	var items []eduCourseItem
	if err := c.ShouldBindJSON(&items); err != nil || len(items) == 0 {
		response.BadRequest(c, "请求体须为非空课程数组 [{name,credit}]")
		return
	}
	ok, failed := 0, 0
	for _, it := range items {
		var co model.Course
		e := h.Repo.DB.Where("name = ?", it.Name).First(&co).Error
		if e == gorm.ErrRecordNotFound {
			co = model.Course{Name: it.Name, Credit: it.Credit}
			if e = h.Repo.DB.Create(&co).Error; e != nil {
				failed++
				continue
			}
		} else if e == nil {
			co.Credit = it.Credit
			h.Repo.DB.Save(&co)
		} else {
			failed++
			continue
		}
		ok++
	}
	h.logSync(cfg.Name, "courses", len(items), ok, failed, "", c.ClientIP())
	response.Success(c, gin.H{"total": len(items), "succeeded": ok, "failed": failed})
}

// ---- 班级同步 ----
type eduClassItem struct {
	Name       string `json:"name" binding:"required"`
	Department string `json:"department"`
}

// SyncClassesPush POST /api/v1/edu/push/classes
func (h *Handler) SyncClassesPush(c *gin.Context) {
	cfg, okAuth := h.checkSyncKey(c)
	if !okAuth {
		return
	}
	var items []eduClassItem
	if err := c.ShouldBindJSON(&items); err != nil || len(items) == 0 {
		response.BadRequest(c, "请求体须为非空班级数组 [{name,department}]")
		return
	}
	ok, failed := 0, 0
	for _, it := range items {
		var cl model.Clazz
		e := h.Repo.DB.Where("name = ?", it.Name).First(&cl).Error
		if e == gorm.ErrRecordNotFound {
			cl = model.Clazz{Name: it.Name, Department: it.Department}
			if e = h.Repo.DB.Create(&cl).Error; e != nil {
				failed++
				continue
			}
		} else if e == nil {
			cl.Department = it.Department
			h.Repo.DB.Save(&cl)
		} else {
			failed++
			continue
		}
		ok++
	}
	h.logSync(cfg.Name, "classes", len(items), ok, failed, "", c.ClientIP())
	response.Success(c, gin.H{"total": len(items), "succeeded": ok, "failed": failed})
}

// ---- 课表同步 ----
type eduScheduleItem struct {
	Course  string `json:"course" binding:"required"` // 课程名
	Clazz   string `json:"clazz" binding:"required"`  // 班级名
	Room    string `json:"room" binding:"required"`   // 机房名
	Weekday int    `json:"weekday"`                   // 1-7
	Period  string `json:"period"`                    // 如 "1-2节"
	Weeks   string `json:"weeks"`                     // 如 "1-16周"
}

// SyncSchedulesPush POST /api/v1/edu/push/schedules
func (h *Handler) SyncSchedulesPush(c *gin.Context) {
	cfg, okAuth := h.checkSyncKey(c)
	if !okAuth {
		return
	}
	var items []eduScheduleItem
	if err := c.ShouldBindJSON(&items); err != nil || len(items) == 0 {
		response.BadRequest(c, "请求体须为非空课表数组")
		return
	}
	ok, failed, details := 0, 0, []string{}
	for _, it := range items {
		var course model.Course
		if e := h.Repo.DB.Where("name = ?", it.Course).First(&course).Error; e != nil {
			failed++
			details = append(details, "课程不存在:"+it.Course)
			continue
		}
		var clazz model.Clazz
		if e := h.Repo.DB.Where("name = ?", it.Clazz).First(&clazz).Error; e != nil {
			failed++
			details = append(details, "班级不存在:"+it.Clazz)
			continue
		}
		var room model.Room
		if e := h.Repo.DB.Where("name = ?", it.Room).First(&room).Error; e != nil {
			failed++
			details = append(details, "机房不存在:"+it.Room)
			continue
		}
		// 同课程+班级+星期+节次视为更新
		var sc model.Schedule
		e := h.Repo.DB.Where("course_id = ? AND clazz_id = ? AND weekday = ? AND period = ?",
			course.ID, clazz.ID, it.Weekday, it.Period).First(&sc).Error
		if e == gorm.ErrRecordNotFound {
			sc = model.Schedule{
				CourseID: course.ID, ClazzID: clazz.ID, RoomID: room.ID,
				Weekday: it.Weekday, Period: it.Period, WeekRange: it.Weeks,
			}
			if e = h.Repo.DB.Create(&sc).Error; e != nil {
				failed++
				details = append(details, e.Error())
				continue
			}
		} else if e == nil {
			sc.RoomID = room.ID
			sc.WeekRange = it.Weeks
			h.Repo.DB.Save(&sc)
		} else {
			failed++
			continue
		}
		ok++
	}
	detail := strings.Join(details, "; ")
	if len(detail) > 1000 {
		detail = detail[:1000]
	}
	h.logSync(cfg.Name, "schedules", len(items), ok, failed, detail, c.ClientIP())
	response.Success(c, gin.H{"total": len(items), "succeeded": ok, "failed": failed})
}

// ---- 成绩查询/导出（供教务系统拉取） ----

// QueryExamScores GET /api/v1/edu/scores?exam_id=&sno=
// 按考试聚合学生成绩：总分/答对数/交卷时间
func (h *Handler) QueryExamScores(c *gin.Context) {
	examID := c.Query("exam_id")
	sno := c.Query("sno")
	if examID == "" {
		response.BadRequest(c, "缺少 exam_id 参数")
		return
	}
	q := h.Repo.DB.Model(&model.ExamAnswer{}).
		Select("student_sno as sno, sum(score) as total_score, count(*) as answered, max(submitted_at) as submitted_at").
		Where("exam_id = ?", examID).Group("student_sno")
	if sno != "" {
		q = q.Where("student_sno = ?", sno)
	}
	var rows []gin.H
	if err := q.Find(&rows).Error; err != nil {
		response.ServerError(c, err.Error())
		return
	}
	// 补充学生姓名
	type scoreRow struct {
		Sno        string  `json:"sno"`
		Name       string  `json:"name"`
		TotalScore float64 `json:"total_score"`
		Answered   int64   `json:"answered"`
	}
	result := make([]scoreRow, 0, len(rows))
	for _, r := range rows {
		snoVal, _ := r["sno"].(string)
		var st model.Student
		name := ""
		if e := h.Repo.DB.Where("sno = ?", snoVal).First(&st).Error; e == nil {
			name = st.Name
		}
		ts, _ := toFloat(r["total_score"])
		an, _ := toInt64(r["answered"])
		result = append(result, scoreRow{Sno: snoVal, Name: name, TotalScore: ts, Answered: an})
	}
	response.Success(c, gin.H{"exam_id": examID, "count": len(result), "scores": result})
}

// ExportExamScoresCSV GET /api/v1/edu/scores/export?exam_id=
// 导出成绩 CSV（带 BOM，Excel 直接打开）
func (h *Handler) ExportExamScoresCSV(c *gin.Context) {
	examID := c.Query("exam_id")
	if examID == "" {
		response.BadRequest(c, "缺少 exam_id 参数")
		return
	}
	var exam model.Exam
	if e := h.Repo.DB.First(&exam, examID).Error; e != nil {
		response.NotFound(c, "考试不存在")
		return
	}
	var rows []gin.H
	h.Repo.DB.Model(&model.ExamAnswer{}).
		Select("student_sno as sno, sum(score) as total_score, count(*) as answered").
		Where("exam_id = ?", examID).Group("student_sno").Find(&rows)

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=scores_exam_"+examID+".csv")
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM
	c.Writer.WriteString("学号,姓名,总分,答题数,考试名称\n")
	for _, r := range rows {
		snoVal, _ := r["sno"].(string)
		var st model.Student
		name := ""
		if e := h.Repo.DB.Where("sno = ?", snoVal).First(&st).Error; e == nil {
			name = st.Name
		}
		ts, _ := toFloat(r["total_score"])
		an, _ := toInt64(r["answered"])
		c.Writer.WriteString(strings.Join([]string{
			snoVal, name, fmtFloat(ts), fmtInt(an), exam.Name,
		}, ",") + "\n")
	}
	c.Writer.Flush()
}

// ---- 同步配置管理 ----

// CreateSyncConfig POST /api/v1/edu/sync-configs（JWT）
func (h *Handler) CreateSyncConfig(c *gin.Context) {
	var cfg model.EduSyncConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if cfg.Name == "" || cfg.SyncKey == "" {
		response.BadRequest(c, "name 与 sync_key 必填")
		return
	}
	if e := h.Repo.DB.Create(&cfg).Error; e != nil {
		response.ServerError(c, e.Error())
		return
	}
	response.Created(c, cfg)
}

// ListSyncConfigs GET /api/v1/edu/sync-configs（JWT）
func (h *Handler) ListSyncConfigs(c *gin.Context) {
	var configs []model.EduSyncConfig
	h.Repo.DB.Find(&configs)
	// 脱敏：不返回完整密钥
	for i := range configs {
		if len(configs[i].SyncKey) > 8 {
			configs[i].SyncKey = configs[i].SyncKey[:4] + "****" + configs[i].SyncKey[len(configs[i].SyncKey)-4:]
		}
	}
	response.Success(c, configs)
}

// ListSyncLogs GET /api/v1/edu/sync-logs（JWT）
func (h *Handler) ListSyncLogs(c *gin.Context) {
	var logs []model.EduSyncLog
	h.Repo.DB.Order("id DESC").Limit(100).Find(&logs)
	response.Success(c, logs)
}

// ---- 工具函数 ----
func toFloat(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int64:
		return float64(x), true
	case int:
		return float64(x), true
	}
	return 0, false
}

func toInt64(v interface{}) (int64, bool) {
	f, ok := toFloat(v)
	return int64(f), ok
}

func fmtFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', 1, 64)
}

func fmtInt(i int64) string {
	return strconv.FormatInt(i, 10)
}
