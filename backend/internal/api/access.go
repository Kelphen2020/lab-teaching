package api

import (
	"time"

	"lab-teaching/backend/internal/model"
	"lab-teaching/backend/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ============ V1.x-17 一卡通/门禁对接 ============
//
// 流程：读卡器/一卡通前置机推送刷卡事件 → 卡号匹配学生（t_student.card_no）
//       → 门禁规则判定（t_access_rule）→ 命中课表则自动联动签到（ClassroomManager）

// 节次时间映射（默认，可按校历调整）
var periodTimes = []struct {
	Period string
	Start  string // HH:MM
	End    string
}{
	{"1-2节", "08:00", "09:40"},
	{"3-4节", "10:00", "11:40"},
	{"5-6节", "14:00", "15:40"},
	{"7-8节", "16:00", "17:40"},
	{"9-10节", "19:00", "20:40"},
}

// currentPeriod 当前时间对应的节次（空串=非上课时段）
func currentPeriod(now time.Time) string {
	hm := now.Format("15:04")
	for _, p := range periodTimes {
		if hm >= p.Start && hm <= p.End {
			return p.Period
		}
	}
	return ""
}

// checkAccessRules 门禁规则判定：有明确的"拒绝"规则则拒绝；有"允许"规则或无规则则放行
func (h *Handler) checkAccessRules(roomID uint, clazzID *uint, weekday int, period string) bool {
	var rules []model.AccessRule
	q := h.Repo.DB.Where("room_id = ?", roomID)
	if clazzID != nil {
		q = q.Where("(clazz_id IS NULL OR clazz_id = ?)", *clazzID)
	} else {
		q = q.Where("clazz_id IS NULL")
	}
	q.Find(&rules)
	for _, r := range rules {
		// 星期/节次不匹配的规则跳过
		if r.Weekday != 0 && r.Weekday != weekday {
			continue
		}
		if r.Period != "" && r.Period != period {
			continue
		}
		// nil 视为允许（默认 true）；显式 false 才拒绝
		if r.Allow != nil && !*r.Allow {
			return false // 命中拒绝规则
		}
	}
	return true
}

// ---- 刷卡事件推送 ----
type cardEventReq struct {
	CardNo     string `json:"card_no" binding:"required"`
	RoomName   string `json:"room_name"`             // 机房名（按名匹配）
	RoomID     *uint  `json:"room_id"`               // 或直接指定
	DeviceName string `json:"device_name"`           // 读卡器名称
	Type       string `json:"type"`                  // in/out，默认 in
	Source     string `json:"source"`                // card/access
}

// PushCardEvent POST /api/v1/access/card-event（X-Sync-Key 鉴权）
func (h *Handler) PushCardEvent(c *gin.Context) {
	cfg, okAuth := h.checkSyncKey(c)
	if !okAuth {
		return
	}
	var req cardEventReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: card_no 必填")
		return
	}
	if req.Type == "" {
		req.Type = "in"
	}
	if req.Source == "" {
		req.Source = "card"
	}

	ev := model.CardEvent{
		CardNo:     req.CardNo,
		RoomID:     req.RoomID,
		RoomName:   req.RoomName,
		DeviceName: req.DeviceName,
		Type:       req.Type,
		Source:     req.Source,
		CreatedAt:  time.Now(),
	}

	// 1. 卡号匹配学生
	var student model.Student
	if err := h.Repo.DB.Where("card_no = ?", req.CardNo).First(&student).Error; err == nil {
		ev.StudentSno = student.Sno
	}

	// 2. 定位机房
	var room *model.Room
	if req.RoomID != nil {
		var r model.Room
		if e := h.Repo.DB.First(&r, *req.RoomID).Error; e == nil {
			room = &r
		}
	} else if req.RoomName != "" {
		var r model.Room
		if e := h.Repo.DB.Where("name = ?", req.RoomName).First(&r).Error; e == nil {
			room = &r
		}
	}
	if room != nil {
		ev.RoomID = &room.ID
		ev.RoomName = room.Name
	}

	// 3. 门禁判定（进门时）
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	period := currentPeriod(now)
	allowed := true
	if req.Type == "in" && room != nil {
		var clazzID *uint
		if ev.StudentSno != "" {
			var st model.Student
			if e := h.Repo.DB.Where("sno = ?", ev.StudentSno).First(&st).Error; e == nil {
				clazzID = st.ClazzID
			}
		}
		allowed = h.checkAccessRules(room.ID, clazzID, weekday, period)
	}
	ev.Allowed = &allowed

	// 4. 课表联动签到：进门 + 已匹配学生 + 当前节次有该班课表
	checkInDone := false
	if req.Type == "in" && ev.StudentSno != "" && room != nil && period != "" && allowed {
		var st model.Student
		h.Repo.DB.Where("sno = ?", ev.StudentSno).First(&st)
		if st.ClazzID != nil {
			var sched model.Schedule
			e := h.Repo.DB.Where("room_id = ? AND clazz_id = ? AND weekday = ? AND period = ?",
				room.ID, *st.ClazzID, weekday, period).First(&sched).Error
			if e == nil {
				rec := h.Classroom.CheckIn(room.ID, sched.CourseID, ev.StudentSno, st.Name, "", "card")
				ev.CheckInID = &rec.ID
				checkInDone = true
			}
		}
	}

	if err := h.Repo.DB.Create(&ev).Error; err != nil {
		response.ServerError(c, err.Error())
		return
	}
	h.logSync(cfg.Name, "card_"+req.Type, 1, 1, 0, "", c.ClientIP())
	response.Success(c, gin.H{
		"event_id":     ev.ID,
		"student_sno":  ev.StudentSno,
		"allowed":      allowed,
		"check_in":     checkInDone,
		"period":       period,
	})
}

// ---- 刷卡事件查询 ----
// ListCardEvents GET /api/v1/access/card-events?room_id=&sno=&limit=（JWT）
func (h *Handler) ListCardEvents(c *gin.Context) {
	q := h.Repo.DB.Order("id DESC").Limit(200)
	if v := c.Query("room_id"); v != "" {
		q = q.Where("room_id = ?", v)
	}
	if v := c.Query("sno"); v != "" {
		q = q.Where("student_sno = ?", v)
	}
	var events []model.CardEvent
	q.Find(&events)
	response.Success(c, events)
}

// ---- 门禁规则管理 ----
// CreateAccessRule POST /api/v1/access/rules（JWT）
func (h *Handler) CreateAccessRule(c *gin.Context) {
	var rule model.AccessRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if rule.RoomID == 0 {
		response.BadRequest(c, "room_id 必填")
		return
	}
	if e := h.Repo.DB.Create(&rule).Error; e != nil {
		response.ServerError(c, e.Error())
		return
	}
	response.Created(c, rule)
}

// ListAccessRules GET /api/v1/access/rules?room_id=（JWT）
func (h *Handler) ListAccessRules(c *gin.Context) {
	q := h.Repo.DB.Order("id DESC")
	if v := c.Query("room_id"); v != "" {
		q = q.Where("room_id = ?", v)
	}
	var rules []model.AccessRule
	q.Find(&rules)
	response.Success(c, rules)
}

// DeleteAccessRule DELETE /api/v1/access/rules/:id（JWT）
func (h *Handler) DeleteAccessRule(c *gin.Context) {
	id := c.Param("id")
	if e := h.Repo.DB.Delete(&model.AccessRule{}, id).Error; e != nil {
		if e == gorm.ErrRecordNotFound {
			response.NotFound(c, "规则不存在")
			return
		}
		response.ServerError(c, e.Error())
		return
	}
	response.Success(c, gin.H{"deleted": id})
}

// ---- 卡号绑定 ----
// BindCard PUT /api/v1/access/bind（JWT）：为学生绑定/换绑一卡通卡号
func (h *Handler) BindCard(c *gin.Context) {
	var req struct {
		Sno    string `json:"sno" binding:"required"`
		CardNo string `json:"card_no" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "sno 与 card_no 必填")
		return
	}
	// 卡号查重
	var dup model.Student
	if e := h.Repo.DB.Where("card_no = ? AND sno != ?", req.CardNo, req.Sno).First(&dup).Error; e == nil {
		response.BadRequest(c, "该卡号已绑定学号 "+dup.Sno)
		return
	}
	var st model.Student
	if e := h.Repo.DB.Where("sno = ?", req.Sno).First(&st).Error; e != nil {
		response.NotFound(c, "学生不存在")
		return
	}
	st.CardNo = req.CardNo
	if e := h.Repo.DB.Save(&st).Error; e != nil {
		response.ServerError(c, e.Error())
		return
	}
	response.Success(c, gin.H{"sno": st.Sno, "card_no": st.CardNo})
}
