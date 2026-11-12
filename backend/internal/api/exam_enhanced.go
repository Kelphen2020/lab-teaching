package api

import (
	"encoding/csv"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"lab-teaching/backend/internal/model"
	"lab-teaching/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// === V0.9-9 考试模块增强：随机组卷、计时、自动阅卷增强、成绩导出 ===

// --- 随机组卷 ---

type randomPaperReq struct {
	Name     string `json:"name" binding:"required"`
	Duration int    `json:"duration"` // 分钟
	Rules    []struct {
		Type       string `json:"type"`       // single/multi/judge/fill
		Difficulty int    `json:"difficulty"` // 1-5, 0=不限
		Knowledge  string `json:"knowledge"`  // 知识点关键词
		Count      int    `json:"count"`      // 抽取数量
		Score      float64 `json:"score"`     // 每题分值
	} `json:"rules" binding:"required"`
}

// RandomGeneratePaper 随机组卷
func (h *Handler) RandomGeneratePaper(c *gin.Context) {
	var req randomPaperReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	teacherID := uint(0)
	if uid, exists := c.Get("user_id"); exists {
		teacherID = uid.(uint)
	}

	if req.Duration <= 0 {
		req.Duration = 60
	}

	paper := model.Paper{
		Name:      req.Name,
		Duration:  req.Duration,
		Strategy:  "随机",
		Status:    "草稿",
		CreatorID: teacherID,
	}
	if err := h.Repo.CreatePaper(&paper); err != nil {
		response.Error(c, 500, "创建试卷失败")
		return
	}

	var items []model.PaperQuestion
	var totalScore float64
	sortOrder := 1

	for _, rule := range req.Rules {
		if rule.Count <= 0 {
			continue
		}
		questions, err := h.Repo.ListQuestionsByFilter(rule.Type, rule.Difficulty, rule.Knowledge, rule.Count)
		if err != nil || len(questions) == 0 {
			continue
		}
		for _, q := range questions {
			score := rule.Score
			if score <= 0 {
				score = q.Score
			}
			items = append(items, model.PaperQuestion{
				PaperID:    paper.ID,
				QuestionID: q.ID,
				SortOrder:  sortOrder,
				Score:      score,
			})
			totalScore += score
			sortOrder++
		}
	}

	if len(items) > 0 {
		h.Repo.AddPaperQuestions(items)
	}
	h.Repo.UpdatePaper(paper.ID, map[string]interface{}{"total_score": totalScore})
	paper.TotalScore = totalScore

	response.Success(c, gin.H{
		"paper":       paper,
		"question_count": len(items),
		"total_score": totalScore,
	})
}

// --- 考试计时 ---

// GetExamTimer 获取考试剩余时间
func (h *Handler) GetExamTimer(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	exam, err := h.Repo.GetExam(uint(id))
	if err != nil {
		response.Error(c, 404, "考试不存在")
		return
	}

	now := time.Now()
	result := gin.H{
		"exam_id": exam.ID,
		"status":  exam.Status,
	}

	if exam.StartAt != nil {
		result["start_at"] = exam.StartAt.Format(time.RFC3339)
		result["elapsed_sec"] = int(now.Sub(*exam.StartAt).Seconds())
	}
	if exam.EndAt != nil {
		remaining := exam.EndAt.Sub(now).Seconds()
		if remaining < 0 {
			remaining = 0
		}
		result["end_at"] = exam.EndAt.Format(time.RFC3339)
		result["remaining_sec"] = int(remaining)
		result["expired"] = remaining <= 0
	}

	response.Success(c, result)
}

// ExtendExam 延长考试时间
func (h *Handler) ExtendExam(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req struct {
		Minutes int `json:"minutes" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	exam, err := h.Repo.GetExam(uint(id))
	if err != nil {
		response.Error(c, 404, "考试不存在")
		return
	}
	if exam.Status != "进行中" {
		response.Error(c, 400, "只能延长进行中的考试")
		return
	}

	var newEnd time.Time
	if exam.EndAt != nil {
		newEnd = exam.EndAt.Add(time.Duration(req.Minutes) * time.Minute)
	} else {
		newEnd = time.Now().Add(time.Duration(req.Minutes) * time.Minute)
	}
	h.Repo.UpdateExam(uint(id), map[string]interface{}{"end_at": &newEnd})

	response.Success(c, gin.H{
		"exam_id": id,
		"new_end_at": newEnd.Format(time.RFC3339),
		"extended_minutes": req.Minutes,
	})
}

// --- 自动阅卷增强（多选题部分得分） ---

// partialScore 计算多选题部分得分
// 规则：选对得分，选错扣分，最低0分
// 例如：答案ABC，学生选AB → 得 2/3 分；选ABD → 选错D，得 max(0, 2/3 - 1/3) = 1/3 分
func partialScore(studentAnswer, correctAnswer string, fullScore float64) float64 {
	correct := normalizeAnswer(correctAnswer)
	student := normalizeAnswer(studentAnswer)

	if student == correct {
		return fullScore
	}

	// 完全错误
	if student == "" {
		return 0
	}

	correctSet := make(map[rune]bool)
	for _, ch := range correct {
		correctSet[ch] = true
	}
	studentSet := make(map[rune]bool)
	for _, ch := range student {
		studentSet[ch] = true
	}

	correctCount := len(correctSet)
	if correctCount == 0 {
		return 0
	}

	// 计算选对数和选错数
	rightCount := 0
	wrongCount := 0
	for ch := range studentSet {
		if correctSet[ch] {
			rightCount++
		} else {
			wrongCount++
		}
	}

	// 部分得分 = (选对/总正确) * 满分 - (选错/总正确) * 满分
	perScore := fullScore / float64(correctCount)
	score := float64(rightCount)*perScore - float64(wrongCount)*perScore
	if score < 0 {
		score = 0
	}
	// 保留2位小数
	score = math.Round(score*100) / 100
	return score
}

// ReGradeExam 重新阅卷（支持多选题部分得分）
func (h *Handler) ReGradeExam(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	exam, err := h.Repo.GetExam(uint(id))
	if err != nil {
		response.Error(c, 404, "考试不存在")
		return
	}

	// 获取所有答案
	answers, _ := h.Repo.ListExamAnswers(uint(id), "")
	if len(answers) == 0 {
		response.Success(c, gin.H{"message": "无答案记录", "regraded": 0})
		return
	}

	// 加载试卷题目分值
	pqs, _ := h.Repo.ListPaperQuestions(exam.PaperID)
	pqScoreMap := make(map[uint]float64)
	for _, pq := range pqs {
		pqScoreMap[pq.QuestionID] = pq.Score
	}

	regraded := 0
	for i := range answers {
		q, err := h.Repo.GetQuestion(answers[i].QuestionID)
		if err != nil {
			continue
		}

		fullScore := pqScoreMap[answers[i].QuestionID]
		if fullScore == 0 {
			fullScore = q.Score
		}

		var newScore float64
		var correct bool

		if q.Type == "multi" {
			// 多选题：部分得分
			newScore = partialScore(answers[i].Answer, q.Answer, fullScore)
			correct = newScore >= fullScore
		} else {
			// 单选/判断/填空：全对才得分
			correct = normalizeAnswer(answers[i].Answer) == normalizeAnswer(q.Answer)
			if correct {
				newScore = fullScore
			}
		}

		if answers[i].Score != newScore || (answers[i].Correct != nil && *answers[i].Correct != correct) {
			h.Repo.DB.Model(&model.ExamAnswer{}).Where("id = ?", answers[i].ID).Updates(map[string]interface{}{
				"score":   newScore,
				"correct": correct,
			})
			regraded++
		}
	}

	response.Success(c, gin.H{
		"exam_id":  id,
		"regraded": regraded,
		"total":    len(answers),
	})
}

// --- 成绩导出 ---

// ExportExamScores 导出考试成绩（CSV）
func (h *Handler) ExportExamScores(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	exam, err := h.Repo.GetExam(uint(id))
	if err != nil {
		response.Error(c, 404, "考试不存在")
		return
	}

	answers, _ := h.Repo.ListExamAnswers(uint(id), "")

	// 按学生汇总
	type studentScore struct {
		StudentSno string
		AgentID    string
		TotalScore float64
		Correct    int
		Total      int
	}
	scoreMap := make(map[string]*studentScore)
	for _, a := range answers {
		key := a.StudentSno
		if key == "" {
			key = a.AgentID
		}
		s, ok := scoreMap[key]
		if !ok {
			s = &studentScore{StudentSno: a.StudentSno, AgentID: a.AgentID}
			scoreMap[key] = s
		}
		s.TotalScore += a.Score
		s.Total++
		if a.Correct != nil && *a.Correct {
			s.Correct++
		}
	}

	// 排序
	var scores []studentScore
	for _, s := range scoreMap {
		scores = append(scores, *s)
	}
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].TotalScore > scores[j].TotalScore
	})

	// 生成 CSV
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=exam_%d_scores.csv", id))
	c.Writer.Write([]byte("\xEF\xBB\xBF")) // BOM for Excel

	writer := csv.NewWriter(c.Writer)
	writer.Write([]string{"排名", "学号", "终端ID", "总分", "正确数", "总题数", "正确率", "考试名称"})

	for i, s := range scores {
		rate := "0%"
		if s.Total > 0 {
			rate = fmt.Sprintf("%.1f%%", float64(s.Correct)/float64(s.Total)*100)
		}
		writer.Write([]string{
			strconv.Itoa(i + 1),
			s.StudentSno,
			s.AgentID,
			fmt.Sprintf("%.1f", s.TotalScore),
			strconv.Itoa(s.Correct),
			strconv.Itoa(s.Total),
			rate,
			exam.Name,
		})
	}
	writer.Flush()
}

// GetExamAnalysis 考试分析（难度/区分度/知识点分布）
func (h *Handler) GetExamAnalysis(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	exam, err := h.Repo.GetExam(uint(id))
	if err != nil {
		response.Error(c, 404, "考试不存在")
		return
	}

	answers, _ := h.Repo.ListExamAnswers(uint(id), "")
	pqs, _ := h.Repo.ListPaperQuestions(exam.PaperID)

	// 按题目统计正确率
	type questionStat struct {
		QuestionID uint    `json:"question_id"`
		Type       string  `json:"type"`
		Knowledge  string  `json:"knowledge"`
		CorrectRate float64 `json:"correct_rate"`
		AvgScore   float64 `json:"avg_score"`
		Total      int     `json:"total"`
	}

	qStats := make(map[uint]*questionStat)
	for _, pq := range pqs {
		q, err := h.Repo.GetQuestion(pq.QuestionID)
		if err != nil {
			continue
		}
		qStats[pq.QuestionID] = &questionStat{
			QuestionID: pq.QuestionID,
			Type:       q.Type,
			Knowledge:  q.Knowledge,
		}
	}

	for _, a := range answers {
		s, ok := qStats[a.QuestionID]
		if !ok {
			continue
		}
		s.Total++
		s.AvgScore += a.Score
		if a.Correct != nil && *a.Correct {
			s.CorrectRate++
		}
	}

	var stats []questionStat
	for _, s := range qStats {
		if s.Total > 0 {
			s.CorrectRate = s.CorrectRate / float64(s.Total) * 100
			s.AvgScore = s.AvgScore / float64(s.Total)
		}
		stats = append(stats, *s)
	}

	// 总体统计
	var totalStudents int
	scoreSet := make(map[string]float64)
	for _, a := range answers {
		key := a.StudentSno
		if key == "" {
			key = a.AgentID
		}
		scoreSet[key] += a.Score
	}
	totalStudents = len(scoreSet)

	var sumScore, maxScore, minScore float64
	first := true
	for _, s := range scoreSet {
		sumScore += s
		if first || s > maxScore {
			maxScore = s
		}
		if first || s < minScore {
			minScore = s
		}
		first = false
	}
	avgScore := 0.0
	if totalStudents > 0 {
		avgScore = sumScore / float64(totalStudents)
	}

	response.Success(c, gin.H{
		"exam_id":        id,
		"total_students": totalStudents,
		"avg_score":      math.Round(avgScore*100) / 100,
		"max_score":      maxScore,
		"min_score":      minScore,
		"question_stats": stats,
	})
}

// suppress unused
var _ = strings.TrimSpace
