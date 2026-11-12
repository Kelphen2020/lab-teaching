package api

import (
	"strconv"

	"lab-teaching/backend/internal/model"
	"lab-teaching/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ============ 课程 CRUD ============

func (h *Handler) CreateCourse(c *gin.Context) {
	var course model.Course
	if err := c.ShouldBindJSON(&course); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if course.Name == "" {
		response.BadRequest(c, "课程名称不能为空")
		return
	}
	if err := h.Repo.CreateCourse(&course); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Created(c, course)
}

func (h *Handler) UpdateCourse(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	course, err := h.Repo.GetCourse(uint(id))
	if err != nil {
		response.NotFound(c, "课程不存在")
		return
	}
	var req struct {
		Name      *string  `json:"name"`
		Credit    *float64 `json:"credit"`
		TeacherID *uint    `json:"teacher_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if req.Name != nil {
		if *req.Name == "" {
			response.BadRequest(c, "课程名称不能为空")
			return
		}
		course.Name = *req.Name
	}
	if req.Credit != nil {
		course.Credit = *req.Credit
	}
	if req.TeacherID != nil {
		course.TeacherID = req.TeacherID
	}
	if err := h.Repo.UpdateCourse(course); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, course)
}

func (h *Handler) DeleteCourse(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.Repo.DeleteCourse(uint(id)); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// ============ 班级 CRUD ============

func (h *Handler) CreateClazz(c *gin.Context) {
	var clazz model.Clazz
	if err := c.ShouldBindJSON(&clazz); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if clazz.Name == "" {
		response.BadRequest(c, "班级名称不能为空")
		return
	}
	if err := h.Repo.CreateClazz(&clazz); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Created(c, clazz)
}

func (h *Handler) UpdateClazz(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	clazz, err := h.Repo.GetClazz(uint(id))
	if err != nil {
		response.NotFound(c, "班级不存在")
		return
	}
	var req struct {
		Name       *string `json:"name"`
		Department *string `json:"department"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if req.Name != nil {
		if *req.Name == "" {
			response.BadRequest(c, "班级名称不能为空")
			return
		}
		clazz.Name = *req.Name
	}
	if req.Department != nil {
		clazz.Department = *req.Department
	}
	if err := h.Repo.UpdateClazz(clazz); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, clazz)
}

func (h *Handler) DeleteClazz(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.Repo.DeleteClazz(uint(id)); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// ============ 学生 CRUD ============

func (h *Handler) CreateStudent(c *gin.Context) {
	var s model.Student
	if err := c.ShouldBindJSON(&s); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if s.Sno == "" || s.Name == "" {
		response.BadRequest(c, "学号和姓名不能为空")
		return
	}
	if err := h.Repo.CreateStudent(&s); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Created(c, s)
}

func (h *Handler) UpdateStudent(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	student, err := h.Repo.GetStudent(uint(id))
	if err != nil {
		response.NotFound(c, "学生不存在")
		return
	}
	var req struct {
		Sno     *string `json:"sno"`
		Name    *string `json:"name"`
		ClazzID *uint   `json:"clazz_id"`
		CardNo  *string `json:"card_no"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if req.Sno != nil {
		if *req.Sno == "" {
			response.BadRequest(c, "学号不能为空")
			return
		}
		student.Sno = *req.Sno
	}
	if req.Name != nil {
		if *req.Name == "" {
			response.BadRequest(c, "姓名不能为空")
			return
		}
		student.Name = *req.Name
	}
	if req.ClazzID != nil {
		student.ClazzID = req.ClazzID
	}
	if req.CardNo != nil {
		student.CardNo = *req.CardNo
	}
	if err := h.Repo.UpdateStudent(student); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, student)
}

func (h *Handler) DeleteStudent(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.Repo.DeleteStudent(uint(id)); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *Handler) SearchStudents(c *gin.Context) {
	keyword := c.Query("keyword")
	clazzID, _ := strconv.ParseUint(c.Query("clazz_id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 { page = 1 }
	if size < 1 || size > 100 { size = 20 }
	list, total, err := h.Repo.SearchStudentsPaged(keyword, uint(clazzID), page, size)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, response.PageData{Items: list, Total: total, Page: page, Size: size})
}

// ============ 课表 CRUD ============

func (h *Handler) CreateSchedule(c *gin.Context) {
	var s model.Schedule
	if err := c.ShouldBindJSON(&s); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if s.CourseID == 0 || s.ClazzID == 0 || s.RoomID == 0 || s.Weekday == 0 || s.Period == "" {
		response.BadRequest(c, "课程ID、班级ID、机房ID、星期、节次不能为空")
		return
	}
	if err := h.Repo.CreateSchedule(&s); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Created(c, s)
}

func (h *Handler) UpdateSchedule(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	schedule, err := h.Repo.GetSchedule(uint(id))
	if err != nil {
		response.NotFound(c, "课表不存在")
		return
	}
	var req struct {
		CourseID  *uint   `json:"course_id"`
		ClazzID   *uint   `json:"clazz_id"`
		RoomID    *uint   `json:"room_id"`
		TeacherID *uint   `json:"teacher_id"`
		Weekday   *int    `json:"weekday"`
		Period    *string `json:"period"`
		WeekRange *string `json:"week_range"`
		Status    *string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if req.CourseID != nil {
		schedule.CourseID = *req.CourseID
	}
	if req.ClazzID != nil {
		schedule.ClazzID = *req.ClazzID
	}
	if req.RoomID != nil {
		schedule.RoomID = *req.RoomID
	}
	if req.TeacherID != nil {
		schedule.TeacherID = req.TeacherID
	}
	if req.Weekday != nil {
		schedule.Weekday = *req.Weekday
	}
	if req.Period != nil {
		schedule.Period = *req.Period
	}
	if req.WeekRange != nil {
		schedule.WeekRange = *req.WeekRange
	}
	if req.Status != nil {
		schedule.Status = *req.Status
	}
	if err := h.Repo.UpdateSchedule(schedule); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, schedule)
}

func (h *Handler) DeleteSchedule(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.Repo.DeleteSchedule(uint(id)); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *Handler) ListSchedules(c *gin.Context) {
	clazzID, _ := strconv.ParseUint(c.Query("clazz_id"), 10, 64)
	courseID, _ := strconv.ParseUint(c.Query("course_id"), 10, 64)
	weekday, _ := strconv.Atoi(c.Query("weekday"))
	list, err := h.Repo.ListSchedules(uint(clazzID), uint(courseID), weekday)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, list)
}