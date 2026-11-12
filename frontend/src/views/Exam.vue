<template>
  <div class="exam-page">
    <!-- Toast 通知 -->
    <div v-if="toast.show" class="toast" :class="toast.type">{{ toast.msg }}</div>

    <div class="card">
      <div class="tabs">
        <button :class="{active: tab==='questions'}" @click="tab='questions'"><SvgIcon name="doc" /> 题库</button>
        <button :class="{active: tab==='papers'}" @click="tab='papers'"><SvgIcon name="doc" /> 试卷</button>
        <button :class="{active: tab==='exams'}" @click="tab='exams'"><SvgIcon name="target" /> 考试</button>
        <button :class="{active: tab==='results'}" @click="tab='results'"><SvgIcon name="chart" /> 成绩</button>
        <button :class="{active: tab==='violations'}" @click="tab='violations'"><SvgIcon name="alert" /> 违规</button>
      </div>

      <!-- ===== 题库管理 ===== -->
      <div v-if="tab==='questions'" class="panel">
        <div class="toolbar">
          <select v-model="qFilter" @change="loadQuestions">
            <option value="">全部题型</option>
            <option value="single">单选</option>
            <option value="multi">多选</option>
            <option value="judge">判断</option>
            <option value="fill">填空</option>
          </select>
          <button class="btn primary" @click="openAddQ">+ 新增题目</button>
        </div>
        <table class="tbl">
          <thead><tr><th>ID</th><th>题型</th><th>题目</th><th>选项</th><th>答案</th><th>分值</th><th>知识点</th><th>操作</th></tr></thead>
          <tbody>
            <tr v-for="q in questions" :key="q.id">
              <td>{{q.id}}</td>
              <td><span class="tag">{{typeLabel(q.type)}}</span></td>
              <td class="content-cell" :title="q.content">{{q.content}}</td>
              <td class="content-cell" :title="q.options">{{formatOptions(q.options)}}</td>
              <td>{{q.answer}}</td>
              <td>{{q.score}}</td>
              <td>{{q.knowledge || '-'}}</td>
              <td class="ops">
                <button class="btn sm" @click="openEditQ(q)">编辑</button>
                <button class="btn sm red" @click="deleteQuestion(q.id)">删除</button>
              </td>
            </tr>
            <tr v-if="!questions.length"><td colspan="8" class="empty">暂无题目</td></tr>
          </tbody>
        </table>
      </div>

      <!-- ===== 试卷管理 ===== -->
      <div v-if="tab==='papers'" class="panel">
        <div class="toolbar">
          <button class="btn primary" @click="openAddP">+ 新建试卷</button>
        </div>
        <table class="tbl">
          <thead><tr><th>ID</th><th>名称</th><th>时长(分)</th><th>总分</th><th>策略</th><th>状态</th><th>操作</th></tr></thead>
          <tbody>
            <tr v-for="p in papers" :key="p.id">
              <td>{{p.id}}</td>
              <td>{{p.name}}</td>
              <td>{{p.duration}}</td>
              <td>{{p.total_score}}</td>
              <td>{{p.strategy}}</td>
              <td><span class="tag" :class="p.status==='已发布'?'green':'gray'">{{p.status}}</span></td>
              <td class="ops">
                <button class="btn sm" @click="viewPaperDetail(p.id)">详情</button>
                <button v-if="p.status==='草稿'" class="btn sm green" @click="publishPaper(p.id)">发布</button>
                <button class="btn sm red" @click="deletePaper(p.id)">删除</button>
              </td>
            </tr>
            <tr v-if="!papers.length"><td colspan="7" class="empty">暂无试卷</td></tr>
          </tbody>
        </table>
      </div>

      <!-- ===== 考试管理 ===== -->
      <div v-if="tab==='exams'" class="panel">
        <div class="toolbar">
          <button class="btn primary" @click="openAddE">+ 创建考试</button>
        </div>
        <table class="tbl">
          <thead><tr><th>ID</th><th>名称</th><th>试卷</th><th>状态</th><th>防切屏</th><th>开始时间</th><th>操作</th></tr></thead>
          <tbody>
            <tr v-for="e in exams" :key="e.id">
              <td>{{e.id}}</td>
              <td>{{e.name}}</td>
              <td>{{paperName(e.paper_id)}}</td>
              <td><span class="tag" :class="examStatusClass(e.status)">{{e.status}}</span></td>
              <td><SvgIcon :name="e.anti_cheat ? 'check-circle' : 'x-circle'" /></td>
              <td>{{e.start_at ? fmtTime(e.start_at) : '-'}}</td>
              <td class="ops">
                <button v-if="e.status==='草稿'" class="btn sm green" @click="startExam(e.id)">开始</button>
                <button v-if="e.status==='进行中'" class="btn sm red" @click="endExam(e.id)">结束</button>
                <button class="btn sm" @click="viewResults(e.id)">成绩</button>
                <button class="btn sm" @click="viewViolations(e.id)">违规</button>
                <button v-if="e.status==='草稿'" class="btn sm red" @click="deleteExam(e.id)">删除</button>
              </td>
            </tr>
            <tr v-if="!exams.length"><td colspan="7" class="empty">暂无考试</td></tr>
          </tbody>
        </table>
      </div>

      <!-- ===== 成绩查看 ===== -->
      <div v-if="tab==='results'" class="panel">
        <div class="toolbar">
          <select v-model="resultExamId" @change="loadResults">
            <option value="">选择考试</option>
            <option v-for="e in exams" :key="e.id" :value="e.id">{{e.name}}</option>
          </select>
          <button v-if="resultExamId" class="btn" @click="loadResults">刷新</button>
        </div>
        <table class="tbl" v-if="results.length">
          <thead><tr><th>Agent</th><th>学号</th><th>总分</th><th>正确/总题</th><th>正确率</th></tr></thead>
          <tbody>
            <tr v-for="r in results" :key="r.agent_id">
              <td>{{r.agent_id}}</td>
              <td>{{r.student_sno || '-'}}</td>
              <td class="score">{{r.total_score}}</td>
              <td>{{r.correct}}/{{r.total}}</td>
              <td>{{r.total ? Math.round(r.correct/r.total*100) : 0}}%</td>
            </tr>
          </tbody>
        </table>
        <div v-else class="empty">选择考试查看成绩</div>
      </div>

      <!-- ===== 违规记录 ===== -->
      <div v-if="tab==='violations'" class="panel">
        <div class="toolbar">
          <select v-model="violationExamId" @change="loadViolations">
            <option value="">选择考试</option>
            <option v-for="e in exams" :key="e.id" :value="e.id">{{e.name}}</option>
          </select>
          <button v-if="violationExamId" class="btn" @click="loadViolations">刷新</button>
        </div>
        <table class="tbl" v-if="violations.length">
          <thead><tr><th>时间</th><th>Agent</th><th>类型</th><th>详情</th></tr></thead>
          <tbody>
            <tr v-for="v in violations" :key="v.id">
              <td>{{fmtTime(v.created_at)}}</td>
              <td>{{v.agent_id}}</td>
              <td><span class="tag red">{{v.type}}</span></td>
              <td class="content-cell" :title="v.detail">{{v.detail}}</td>
            </tr>
          </tbody>
        </table>
        <div v-else class="empty">选择考试查看违规记录</div>
      </div>
    </div>

    <!-- 试卷详情弹窗 -->
    <div v-if="showPaperDetail" class="modal-mask" @click.self="showPaperDetail=false">
      <div class="modal wide">
        <h3>试卷详情 — {{paperDetail?.paper?.name}}</h3>
        <div v-if="paperDetail" class="paper-info">
          <span>时长: {{paperDetail.paper.duration}}分钟</span>
          <span>总分: {{paperDetail.paper.total_score}}</span>
          <span>策略: {{paperDetail.paper.strategy}}</span>
          <span>状态: {{paperDetail.paper.status}}</span>
        </div>
        <table class="tbl" v-if="paperDetail?.questions?.length">
          <thead><tr><th>#</th><th>题型</th><th>题目</th><th>分值</th></tr></thead>
          <tbody>
            <tr v-for="(pq, i) in paperDetail.questions" :key="i">
              <td>{{i+1}}</td>
              <td><span class="tag">{{typeLabel(pq.question?.type)}}</span></td>
              <td class="content-cell">{{pq.question?.content}}</td>
              <td>{{pq.score}}</td>
            </tr>
          </tbody>
        </table>
        <div v-else class="empty">暂无题目</div>
        <div class="modal-actions"><button class="btn" @click="showPaperDetail=false">关闭</button></div>
      </div>
    </div>

    <!-- 新增/编辑题目弹窗 -->
    <div v-if="showAddQ" class="modal-mask" @click.self="showAddQ=false">
      <div class="modal">
        <h3>{{editingQ ? '编辑题目' : '新增题目'}}</h3>
        <div class="form-grid">
          <label>题型</label>
          <select v-model="qForm.type">
            <option value="single">单选</option>
            <option value="multi">多选</option>
            <option value="judge">判断</option>
            <option value="fill">填空</option>
          </select>
          <label>题目内容</label>
          <textarea v-model="qForm.content" rows="3"></textarea>
          <label>选项(JSON)</label>
          <input v-model="qForm.options" placeholder='["A. xxx","B. xxx"]'>
          <label>答案</label>
          <input v-model="qForm.answer" placeholder="A / AB / true / 答案文本">
          <label>解析</label>
          <input v-model="qForm.analysis" placeholder="答案解析（可选）">
          <label>分值</label>
          <input v-model.number="qForm.score" type="number" min="1">
          <label>难度</label>
          <select v-model.number="qForm.difficulty">
            <option :value="1">1 - 简单</option>
            <option :value="2">2</option>
            <option :value="3">3 - 中等</option>
            <option :value="4">4</option>
            <option :value="5">5 - 困难</option>
          </select>
          <label>知识点</label>
          <input v-model="qForm.knowledge">
        </div>
        <div class="modal-actions">
          <button class="btn" @click="showAddQ=false">取消</button>
          <button class="btn primary" @click="saveQuestion" :disabled="saving">{{saving ? '保存中...' : '保存'}}</button>
        </div>
      </div>
    </div>

    <!-- 新建试卷弹窗 -->
    <div v-if="showAddP" class="modal-mask" @click.self="showAddP=false">
      <div class="modal">
        <h3>新建试卷</h3>
        <div class="form-grid">
          <label>试卷名称</label>
          <input v-model="pForm.name">
          <label>时长(分钟)</label>
          <input v-model.number="pForm.duration" type="number" min="1">
          <label>组卷策略</label>
          <select v-model="pForm.strategy">
            <option value="固定">固定</option>
            <option value="随机">随机</option>
          </select>
          <label>选择题目</label>
          <div class="q-picker">
            <label v-for="q in questions" :key="q.id" class="q-item">
              <input type="checkbox" :value="q.id" v-model="pForm.question_ids">
              <span class="tag">{{typeLabel(q.type)}}</span>
              <span>{{q.content.slice(0,40)}}{{q.content.length>40?'…':''}}</span>
              <span class="q-score">{{q.score}}分</span>
            </label>
            <div v-if="!questions.length" class="empty">请先在题库中添加题目</div>
          </div>
        </div>
        <div class="modal-actions">
          <button class="btn" @click="showAddP=false">取消</button>
          <button class="btn primary" @click="addPaper" :disabled="saving">{{saving ? '保存中...' : '保存'}}</button>
        </div>
      </div>
    </div>

    <!-- 创建考试弹窗 -->
    <div v-if="showAddE" class="modal-mask" @click.self="showAddE=false">
      <div class="modal">
        <h3>创建考试</h3>
        <div class="form-grid">
          <label>考试名称</label>
          <input v-model="eForm.name">
          <label>选择试卷</label>
          <select v-model="eForm.paper_id">
            <option :value="null" disabled>请选择</option>
            <option v-for="p in papers" :key="p.id" :value="p.id">{{p.name}} ({{p.total_score}}分)</option>
          </select>
          <label>防切屏</label>
          <select v-model="eForm.anti_cheat">
            <option :value="true">启用</option>
            <option :value="false">不启用</option>
          </select>
        </div>
        <div class="modal-actions">
          <button class="btn" @click="showAddE=false">取消</button>
          <button class="btn primary" @click="addExam" :disabled="saving">{{saving ? '创建中...' : '创建'}}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '../api'

const tab = ref('questions')
const questions = ref([])
const papers = ref([])
const exams = ref([])
const results = ref([])
const violations = ref([])
const qFilter = ref('')
const resultExamId = ref('')
const violationExamId = ref('')
const saving = ref(false)

const showAddQ = ref(false)
const showAddP = ref(false)
const showAddE = ref(false)
const showPaperDetail = ref(false)
const paperDetail = ref(null)
const editingQ = ref(null)

const qForm = ref({ type: 'single', content: '', options: '', answer: '', analysis: '', score: 5, difficulty: 3, knowledge: '' })
const pForm = ref({ name: '', duration: 60, strategy: '固定', question_ids: [] })
const eForm = ref({ name: '', paper_id: null, anti_cheat: true })

const toast = ref({ show: false, msg: '', type: 'ok' })
let toastTimer = null
function showToast(msg, type = 'ok') {
  toast.value = { show: true, msg, type }
  clearTimeout(toastTimer)
  toastTimer = setTimeout(() => { toast.value.show = false }, 3000)
}

function typeLabel(t) {
  return { single: '单选', multi: '多选', judge: '判断', fill: '填空' }[t] || t
}
function examStatusClass(s) {
  return { '草稿': 'gray', '进行中': 'green', '已结束': 'red' }[s] || ''
}
function paperName(pid) {
  const p = papers.value.find(x => x.id === pid)
  return p ? p.name : `#${pid}`
}
function fmtTime(t) {
  if (!t) return '-'
  return new Date(t).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}
function formatOptions(opts) {
  if (!opts) return '-'
  try {
    const arr = JSON.parse(opts)
    return Array.isArray(arr) ? arr.join(' | ') : opts
  } catch { return opts }
}

// ===== 数据加载（修复：拦截器已解包 res.data，直接取 r.data） =====
async function loadQuestions() {
  try {
    const params = qFilter.value ? `?type=${qFilter.value}` : ''
    const r = await api.get('/questions' + params)
    questions.value = r.data || []
  } catch (e) {
    showToast('加载题库失败: ' + (e.response?.data?.msg || e.message), 'err')
  }
}
async function loadPapers() {
  try {
    const r = await api.get('/papers')
    papers.value = r.data || []
  } catch (e) {
    showToast('加载试卷失败', 'err')
  }
}
async function loadExams() {
  try {
    const r = await api.get('/exams')
    exams.value = r.data || []
  } catch (e) {
    showToast('加载考试失败', 'err')
  }
}
async function loadResults() {
  if (!resultExamId.value) { results.value = []; return }
  try {
    const r = await api.get(`/exams/${resultExamId.value}/results`)
    results.value = r.data || []
  } catch (e) {
    showToast('加载成绩失败', 'err')
  }
}
async function loadViolations() {
  if (!violationExamId.value) { violations.value = []; return }
  try {
    const r = await api.get(`/exams/${violationExamId.value}/violations`)
    violations.value = r.data || []
  } catch (e) {
    showToast('加载违规记录失败', 'err')
  }
}

// ===== 题库 CRUD =====
function openAddQ() {
  editingQ.value = null
  qForm.value = { type: 'single', content: '', options: '', answer: '', analysis: '', score: 5, difficulty: 3, knowledge: '' }
  showAddQ.value = true
}
function openEditQ(q) {
  editingQ.value = q.id
  qForm.value = { type: q.type, content: q.content, options: q.options || '', answer: q.answer, analysis: q.analysis || '', score: q.score, difficulty: q.difficulty || 3, knowledge: q.knowledge || '' }
  showAddQ.value = true
}
async function saveQuestion() {
  if (!qForm.value.content || !qForm.value.answer) {
    showToast('题目内容和答案不能为空', 'err'); return
  }
  saving.value = true
  try {
    if (editingQ.value) {
      await api.put(`/questions/${editingQ.value}`, qForm.value)
      showToast('题目已更新')
    } else {
      await api.post('/questions', qForm.value)
      showToast('题目已创建')
    }
    showAddQ.value = false
    loadQuestions()
  } catch (e) {
    showToast('保存失败: ' + (e.response?.data?.msg || e.message), 'err')
  } finally { saving.value = false }
}
async function deleteQuestion(id) {
  if (!confirm('确定删除该题目？')) return
  try {
    await api.delete(`/questions/${id}`)
    showToast('已删除')
    loadQuestions()
  } catch (e) {
    showToast('删除失败: ' + (e.response?.data?.msg || e.message), 'err')
  }
}

// ===== 试卷 =====
function openAddP() {
  pForm.value = { name: '', duration: 60, strategy: '固定', question_ids: [] }
  showAddP.value = true
}
async function addPaper() {
  if (!pForm.value.name) { showToast('请输入试卷名称', 'err'); return }
  if (!pForm.value.question_ids.length) { showToast('请至少选择一道题', 'err'); return }
  saving.value = true
  try {
    await api.post('/papers', pForm.value)
    showToast('试卷已创建')
    showAddP.value = false
    loadPapers()
  } catch (e) {
    showToast('创建失败: ' + (e.response?.data?.msg || e.message), 'err')
  } finally { saving.value = false }
}
async function viewPaperDetail(id) {
  try {
    const r = await api.get(`/papers/${id}`)
    paperDetail.value = r.data
    showPaperDetail.value = true
  } catch (e) {
    showToast('加载试卷详情失败', 'err')
  }
}
async function publishPaper(id) {
  try {
    await api.put(`/papers/${id}`, { status: '已发布' })
    showToast('试卷已发布')
    loadPapers()
  } catch (e) {
    showToast('发布失败: ' + (e.response?.data?.msg || e.message), 'err')
  }
}
async function deletePaper(id) {
  if (!confirm('确定删除该试卷？')) return
  try {
    await api.delete(`/papers/${id}`)
    showToast('已删除')
    loadPapers()
  } catch (e) {
    showToast('删除失败: ' + (e.response?.data?.msg || e.message), 'err')
  }
}

// ===== 考试 =====
function openAddE() {
  eForm.value = { name: '', paper_id: null, anti_cheat: true }
  showAddE.value = true
}
async function addExam() {
  if (!eForm.value.name) { showToast('请输入考试名称', 'err'); return }
  if (!eForm.value.paper_id) { showToast('请选择试卷', 'err'); return }
  saving.value = true
  try {
    await api.post('/exams', eForm.value)
    showToast('考试已创建')
    showAddE.value = false
    loadExams()
  } catch (e) {
    showToast('创建失败: ' + (e.response?.data?.msg || e.message), 'err')
  } finally { saving.value = false }
}
async function startExam(id) {
  if (!confirm('确定开始考试？将通知所有在线Agent进入考试模式。')) return
  try {
    await api.post(`/exams/${id}/start`)
    showToast('考试已开始')
    loadExams()
  } catch (e) {
    showToast('开始失败: ' + (e.response?.data?.msg || e.message), 'err')
  }
}
async function endExam(id) {
  if (!confirm('确定结束考试？')) return
  try {
    await api.post(`/exams/${id}/end`)
    showToast('考试已结束')
    loadExams()
  } catch (e) {
    showToast('结束失败: ' + (e.response?.data?.msg || e.message), 'err')
  }
}
async function deleteExam(id) {
  if (!confirm('确定删除该考试？')) return
  try {
    await api.delete(`/exams/${id}`)
    showToast('已删除')
    loadExams()
  } catch (e) {
    showToast('删除失败: ' + (e.response?.data?.msg || e.message), 'err')
  }
}
function viewResults(id) {
  tab.value = 'results'
  resultExamId.value = id
  loadResults()
}
function viewViolations(id) {
  tab.value = 'violations'
  violationExamId.value = id
  loadViolations()
}

onMounted(() => {
  loadQuestions()
  loadPapers()
  loadExams()
})
</script>

<style scoped>
.exam-page { padding: 0; position: relative; }
.toast {
  position: fixed; top: 20px; right: 20px; z-index: 9999;
  padding: 10px 20px; border-radius: 8px; font-size: 13px; color: #fff;
  animation: slideIn .3s ease;
}
.toast.ok { background: #00915b; }
.toast.err { background: #c93a3f; }
@keyframes slideIn { from { transform: translateX(100px); opacity: 0; } to { transform: translateX(0); opacity: 1; } }

.card { background: var(--card); border-radius: 12px; padding: 20px; }
.tabs { display: flex; gap: 8px; margin-bottom: 16px; flex-wrap: wrap; }
.tabs button {
  padding: 8px 16px; border: none; border-radius: 8px;
  background: var(--bg2); color: var(--muted); cursor: pointer; font-size: 14px;
}
.tabs button.active { background: var(--accent); color: #fff; }
.toolbar { display: flex; gap: 10px; margin-bottom: 12px; align-items: center; }
.toolbar select {
  padding: 6px 10px; border-radius: 6px; border: 1px solid var(--border2);
  background: var(--card); color: var(--text);
}
.btn { padding: 7px 14px; border: 1px solid var(--border2); border-radius: 6px; background: var(--card); color: var(--text2); cursor: pointer; font-size: 13px; }
.btn:hover:not(:disabled) { border-color: var(--accent); color: var(--accent); }
.btn:disabled { opacity: .5; cursor: not-allowed; }
.btn.primary { background: var(--accent); color: #fff; }
.btn.green { background: #00915b; color: #fff; }
.btn.red { background: #c93a3f; color: #fff; }
.btn.sm { padding: 4px 10px; font-size: 12px; }
.tbl { width: 100%; border-collapse: collapse; font-size: 13px; }
.tbl th { text-align: left; padding: 8px; color: var(--text2); border-bottom: 1px solid var(--border); }
.tbl td { padding: 8px; border-bottom: 1px solid var(--border); color: var(--text); }
.tbl .content-cell { max-width: 250px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.tag { padding: 2px 8px; border-radius: 12px; font-size: 12px; background: var(--accent-glow); color: var(--accent); }
.tag.green { background: rgba(0,179,104,.13); color: #00915b; }
.tag.gray { background: var(--bg2); color: var(--muted); }
.tag.red { background: rgba(229,72,77,.10); color: #d13b40; }
.empty { text-align: center; color: var(--text2); padding: 30px; }
.ops { display: flex; gap: 6px; flex-wrap: wrap; }
.score { font-weight: bold; color: var(--accent); }

.modal-mask { position: fixed; inset: 0; background: rgba(0,0,0,.6); display: flex; align-items: center; justify-content: center; z-index: 999; }
.modal { background: var(--card); border-radius: 12px; padding: 24px; width: 520px; max-height: 80vh; overflow-y: auto; }
.modal.wide { width: 700px; }
.modal h3 { margin: 0 0 16px; color: var(--text); }
.paper-info { display: flex; gap: 16px; margin-bottom: 12px; color: var(--muted); font-size: 13px; }
.form-grid { display: grid; grid-template-columns: 80px 1fr; gap: 10px; align-items: center; }
.form-grid label { color: var(--muted); font-size: 13px; text-align: right; }
.form-grid input, .form-grid select, .form-grid textarea {
  padding: 7px 10px; border-radius: 6px; border: 1px solid var(--border2);
  background: var(--card); color: var(--text); font-size: 13px; width: 100%;
}
.form-grid textarea { resize: vertical; }
.q-picker { max-height: 200px; overflow-y: auto; display: flex; flex-direction: column; gap: 4px; }
.q-item { display: flex; align-items: center; gap: 6px; font-size: 12px; color: var(--text2); }
.q-score { color: var(--accent); margin-left: auto; white-space: nowrap; }
.modal-actions { display: flex; justify-content: flex-end; gap: 10px; margin-top: 16px; }
</style>
