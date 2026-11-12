<template>
  <div class="student-exam">
    <!-- 阶段1: 加入考试 -->
    <div v-if="stage === 'join'" class="join-panel">
      <h2>学生考试入口</h2>
      <div class="form-group">
        <label>学号</label>
        <input v-model="sno" placeholder="请输入学号" @keyup.enter="joinExam" />
      </div>
      <div class="form-group">
        <label>姓名（选填）</label>
        <input v-model="sname" placeholder="请输入姓名" />
      </div>
      <div class="form-group">
        <label>考试码</label>
        <input v-model="accessCode" placeholder="6位数字考试码" maxlength="6" @keyup.enter="joinExam" />
      </div>
      <button class="btn-primary" :disabled="joining" @click="joinExam">
        {{ joining ? '加入中...' : '加入考试' }}
      </button>
      <p v-if="hasDraft" class="draft-hint"><SvgIcon name="doc" /> 检测到未完成的考试，点击「加入考试」将恢复之前的作答进度</p>
      <p v-if="errMsg" class="error-msg">{{ errMsg }}</p>
    </div>

    <!-- 阶段2: 答题 -->
    <div v-else-if="stage === 'answering'" class="exam-panel">
      <div class="exam-header">
        <h3>{{ examName }}</h3>
        <div class="timer" :class="{ warn: remainSecs < 60 }">
          剩余时间: {{ formatTime(remainSecs) }}
        </div>
      </div>

      <!-- 断线续答提示 -->
      <div v-if="offline" class="net-banner"><SvgIcon name="alert" /> 网络已断开，你的作答已保存在本机，恢复联网后可继续答题，请勿关闭页面</div>
      <div v-else-if="netMsg" class="net-banner ok"><SvgIcon name="check-circle" /> {{ netMsg }}</div>
      <!-- 倒计时告警 -->
      <div v-if="remainSecs <= 300 && remainSecs > 0" class="time-banner"><SvgIcon name="clock" /> 剩余时间不足 5 分钟，请检查作答并及时交卷</div>
      <!-- 轻提示 -->
      <div v-if="toastMsg" class="exam-toast">{{ toastMsg }}</div>
      <div class="question-list">
        <div v-for="(q, idx) in questions" :key="q.id" class="question-card">
          <div class="q-title">
            <span class="q-num">{{ idx + 1 }}.</span>
            <span class="q-type">[{{ typeLabel(q.type) }}]</span>
            <span class="q-score">({{ q.score }}分)</span>
            {{ q.content }}
          </div>
          <div class="q-options" v-if="q.type === 'single' || q.type === 'multi'">
            <label v-for="opt in parseOptions(q.options)" :key="opt.key" class="opt-item">
              <input
                :type="q.type === 'single' ? 'radio' : 'checkbox'"
                :name="'q' + q.id"
                :value="opt.key"
                v-model="answers[q.id]"
              />
              <span>{{ opt.key }}. {{ opt.text }}</span>
            </label>
          </div>
          <div class="q-options" v-else-if="q.type === 'judge'">
            <label class="opt-item">
              <input type="radio" :name="'q' + q.id" value="对" v-model="answers[q.id]" />
              <span>对</span>
            </label>
            <label class="opt-item">
              <input type="radio" :name="'q' + q.id" value="错" v-model="answers[q.id]" />
              <span>错</span>
            </label>
          </div>
          <div class="q-options" v-else>
            <input v-model="answers[q.id]" class="fill-input" placeholder="请输入答案" />
          </div>
        </div>
      </div>
      <div class="exam-footer">
        <span class="answered-count">已答 {{ answeredCount }} / {{ questions.length }}</span>
        <button class="btn-primary" :disabled="submitting" @click="askSubmit">
          {{ submitting ? '提交中...' : '交卷' }}
        </button>
      </div>
      <p v-if="errMsg" class="error-msg">{{ errMsg }}</p>

      <!-- 交卷二次确认 -->
      <div v-if="confirmVisible" class="confirm-mask" @click.self="confirmVisible = false">
        <div class="confirm-box">
          <h3>确认交卷？</h3>
          <p>你已完成 <b>{{ answeredCount }}</b> / {{ questions.length }} 题<span v-if="unansweredCount">，尚有 <b class="warn-text">{{ unansweredCount }} 题未作答</b></span>。</p>
          <p class="confirm-sub">交卷后不可再修改答案，请确认。</p>
          <div class="confirm-actions">
            <button class="btn-cancel" @click="confirmVisible = false">继续作答</button>
            <button class="btn-confirm" :disabled="submitting" @click="confirmSubmit">
              {{ submitting ? '提交中…' : '确认交卷' }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- 阶段3: 成绩 -->
    <div v-else-if="stage === 'result'" class="result-panel">
      <h2>考试结果</h2>
      <div class="score-display">
        <span class="score-num">{{ result.total_score }}</span>
        <span class="score-label">分</span>
      </div>
      <p>答对 {{ result.correct_count }} / {{ result.total_count }} 题</p>
      <div class="result-detail" v-if="result.details">
        <div v-for="(d, i) in result.details" :key="i" class="result-item" :class="d.correct ? 'correct' : 'wrong'">
          <span>第{{ i + 1 }}题</span>
          <span>你的答案: {{ d.answer }}</span>
          <span><SvgIcon :name="d.correct ? 'check' : 'x'" /> {{ d.correct ? '正确' : '错误' }}</span>
          <span>+{{ d.score }}分</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import api from '../api'

const stage = ref('join') // join | answering | result
const sno = ref('')
const sname = ref('')
const accessCode = ref('')
const joining = ref(false)
const submitting = ref(false)
const errMsg = ref('')

const examId = ref(null)
const examName = ref('')
const duration = ref(0)
const questions = ref([])
const answers = ref({})
const remainSecs = ref(0)
const result = ref({})

let timer = null
let toastTimer = null

// 断线检测与提示
const offline = ref(!navigator.onLine)
const netMsg = ref('')
const toastMsg = ref('')
const confirmVisible = ref(false)

// 本地草稿（断线续答保障：答案存 localStorage，联网后可恢复）
const draftKey = () => `exam-draft:${sno.value.trim()}`
function saveDraft() {
  try {
    localStorage.setItem(draftKey(), JSON.stringify({
      exam_id: examId.value,
      answers: answers.value,
      remain_secs: remainSecs.value,
      saved_at: Date.now()
    }))
  } catch (e) { /* 本地存储不可用时忽略 */ }
}
function clearDraft() {
  try { localStorage.removeItem(draftKey()) } catch (e) {}
}
function restoreDraft() {
  try {
    const raw = localStorage.getItem(draftKey())
    if (!raw) return false
    const draft = JSON.parse(raw)
    if (draft.exam_id !== examId.value || !draft.answers) return false
    for (const q of questions.value) {
      const v = draft.answers[q.id]
      if (v !== undefined && v !== null && !(typeof v === 'string' && !v)) {
        answers.value[q.id] = Array.isArray(v) ? [...v] : v
      }
    }
    if (typeof draft.remain_secs === 'number' && draft.remain_secs > 0 && draft.remain_secs < remainSecs.value) {
      remainSecs.value = draft.remain_secs
    }
    return true
  } catch (e) { return false }
}

function toast(msg) {
  toastMsg.value = msg
  if (toastTimer) clearTimeout(toastTimer)
  toastTimer = setTimeout(() => { toastMsg.value = '' }, 3000)
}

const hasDraft = computed(() => {
  if (!sno.value.trim()) return false
  return !!localStorage.getItem(`exam-draft:${sno.value.trim()}`)
})

const answeredCount = computed(() => {
  return Object.values(answers.value).filter(v => v && v.length > 0).length
})

const unansweredCount = computed(() => questions.value.length - answeredCount.value)

// 作答变化自动保存草稿
watch(answers, saveDraft, { deep: true })

function typeLabel(t) {
  const map = { single: '单选', multi: '多选', judge: '判断', fill: '填空' }
  return map[t] || t
}

function parseOptions(str) {
  if (!str) return []
  return str.split('|').map(s => {
    const key = s.charAt(0)
    const text = s.substring(2).trim()
    return { key, text }
  })
}

function formatTime(secs) {
  const m = Math.floor(secs / 60)
  const s = secs % 60
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

async function joinExam() {
  errMsg.value = ''
  if (!sno.value.trim()) { errMsg.value = '请输入学号'; return }
  if (!accessCode.value.trim()) { errMsg.value = '请输入考试码'; return }
  joining.value = true
  try {
    const res = await api.post('/student/join', {
      access_code: accessCode.value.trim(),
      student_sno: sno.value.trim(),
      student_name: sname.value.trim()
    })
    const data = res.data
    examId.value = data.exam_id
    examName.value = data.exam_name
    duration.value = data.duration || 60
    // 获取题目
    const qRes = await api.get(`/student/exam/${examId.value}/questions`)
    questions.value = qRes.data.questions || []
    // 初始化答案
    for (const q of questions.value) {
      answers.value[q.id] = q.type === 'multi' ? [] : ''
    }
    // 倒计时
    remainSecs.value = duration.value * 60
    // 恢复本地草稿（断线续答）
    if (restoreDraft()) toast('已恢复之前的作答进度')
    timer = setInterval(() => {
      remainSecs.value--
      if (remainSecs.value <= 0) {
        clearInterval(timer)
        toast('考试时间到，正在自动交卷…')
        submitExam()
      }
    }, 1000)
    stage.value = 'answering'
  } catch (e) {
    errMsg.value = e.response?.data?.msg || e.message || '加入失败'
  } finally {
    joining.value = false
  }
}

function askSubmit() {
  confirmVisible.value = true
}

async function confirmSubmit() {
  confirmVisible.value = false
  await submitExam()
}

async function submitExam() {
  if (submitting.value) return
  submitting.value = true
  errMsg.value = ''
  try {
    const ansList = questions.value.map(q => {
      let ans = answers.value[q.id]
      if (Array.isArray(ans)) ans = ans.join('')
      return { question_id: q.id, answer: ans || '' }
    })
    const res = await api.post('/student/submit', {
      exam_id: examId.value,
      agent_id: 'WEB-' + sno.value,
      student_sno: sno.value.trim(),
      answers: ansList
    })
    if (timer) clearInterval(timer)
    clearDraft() // 交卷成功，清除本地草稿
    // 获取详细成绩
    const rRes = await api.get(`/student/exam/${examId.value}/result?sno=${sno.value.trim()}`)
    result.value = rRes.data
    stage.value = 'result'
  } catch (e) {
    errMsg.value = e.response?.data?.msg || e.message || '提交失败'
  } finally {
    submitting.value = false
  }
}

function onOnline() {
  offline.value = false
  netMsg.value = '网络已恢复，可继续答题'
  setTimeout(() => { netMsg.value = '' }, 5000)
}
function onOffline() {
  offline.value = true
  saveDraft()
}

onMounted(() => {
  window.addEventListener('online', onOnline)
  window.addEventListener('offline', onOffline)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
  if (toastTimer) clearTimeout(toastTimer)
  window.removeEventListener('online', onOnline)
  window.removeEventListener('offline', onOffline)
})
</script>

<style scoped>
.student-exam {
  max-width: 800px;
  margin: 0 auto;
  padding: 24px;
  font-family: 'PingFang SC', 'Microsoft YaHei', sans-serif;
}
.join-panel {
  max-width: 400px;
  margin: 80px auto;
  padding: 32px;
  background: var(--card);
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0,0,0,0.08);
}
.join-panel h2 { text-align: center; margin-bottom: 24px; color: var(--text); }
.form-group { margin-bottom: 16px; }
.form-group label { display: block; margin-bottom: 6px; font-size: 14px; color: var(--text2); }
.form-group input {
  width: 100%; padding: 10px 12px; border: 1px solid var(--border);
  border-radius: 6px; font-size: 15px; box-sizing: border-box;
}
.form-group input:focus { border-color: var(--accent); outline: none; }
.btn-primary {
  width: 100%; padding: 12px; background: var(--accent); color: #fff;
  border: none; border-radius: 6px; font-size: 16px; cursor: pointer;
  margin-top: 8px;
}
.btn-primary:hover { background: #0055d4; }
.btn-primary:disabled { background: rgba(0,102,255,.35); cursor: not-allowed; }
.error-msg { color: var(--red); font-size: 13px; margin-top: 12px; text-align: center; }
.draft-hint { color: var(--accent); font-size: 13px; margin-top: 12px; text-align: center; }

.net-banner {
  padding: 10px 14px; border-radius: 6px; margin-bottom: 12px; font-size: 14px;
  background: rgba(229,72,77,.10); border: 1px solid rgba(229,72,77,.35); color: var(--red);
}
.net-banner.ok {
  background: rgba(0,179,104,.10); border-color: rgba(0,179,104,.35); color: #00915b;
}
.time-banner {
  padding: 8px 14px; border-radius: 6px; margin-bottom: 12px; font-size: 14px;
  background: rgba(245,158,11,.12); border: 1px solid rgba(245,158,11,.4); color: #d48806;
}
.exam-toast {
  position: fixed; top: 24px; left: 50%; transform: translateX(-50%);
  padding: 10px 24px; border-radius: 6px; font-size: 14px; z-index: 100;
  background: #1f2937; color: #fff; box-shadow: 0 4px 16px rgba(0,0,0,.2);
}

.confirm-mask {
  position: fixed; inset: 0; background: rgba(0,0,0,.45); z-index: 50;
  display: flex; align-items: center; justify-content: center;
}
.confirm-box {
  width: 400px; max-width: 90vw; background: var(--card); border-radius: 10px;
  padding: 24px; box-shadow: 0 8px 32px rgba(0,0,0,.25);
}
.confirm-box h3 { margin: 0 0 12px; font-size: 17px; }
.confirm-box p { margin: 6px 0; font-size: 14px; color: var(--text); }
.confirm-sub { color: var(--muted) !important; font-size: 13px !important; }
.warn-text { color: var(--red); }
.confirm-actions { display: flex; gap: 10px; justify-content: flex-end; margin-top: 18px; }
.btn-cancel, .btn-confirm {
  padding: 9px 22px; border-radius: 6px; font-size: 14px; cursor: pointer; border: none;
}
.btn-cancel { background: var(--bg2); color: var(--text2); border: 1px solid var(--border); }
.btn-cancel:hover { background: var(--border); }
.btn-confirm { background: var(--accent); color: #fff; }
.btn-confirm:hover:not(:disabled) { background: #0055d4; }
.btn-confirm:disabled { opacity: .6; cursor: not-allowed; }

.exam-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: 12px 16px; background: var(--card2); border-radius: 8px; margin-bottom: 16px;
}
.exam-header h3 { margin: 0; font-size: 18px; }
.timer { font-size: 20px; font-weight: bold; color: #2c3e50; }
.timer.warn { color: var(--red); animation: blink 1s infinite; }
@keyframes blink { 50% { opacity: 0.5; } }

.question-card {
  background: var(--card); border: 1px solid var(--border); border-radius: 8px;
  padding: 16px; margin-bottom: 12px;
}
.q-title { font-size: 15px; line-height: 1.6; margin-bottom: 12px; }
.q-num { font-weight: bold; margin-right: 4px; }
.q-type { color: var(--accent); font-size: 13px; margin-right: 4px; }
.q-score { color: var(--muted); font-size: 13px; }
.q-options { padding-left: 8px; }
.opt-item {
  display: flex; align-items: center; gap: 8px;
  padding: 6px 0; font-size: 14px; cursor: pointer;
}
.opt-item input { width: 16px; height: 16px; }
.fill-input {
  width: 100%; padding: 8px 12px; border: 1px solid var(--border);
  border-radius: 4px; font-size: 14px;
}
.exam-footer {
  display: flex; justify-content: space-between; align-items: center;
  padding: 16px 0; border-top: 1px solid var(--border); margin-top: 16px;
}
.answered-count { font-size: 14px; color: var(--text2); }
.exam-footer .btn-primary { width: auto; padding: 10px 32px; }

.result-panel { text-align: center; padding: 40px 0; }
.score-display { margin: 24px 0; }
.score-num { font-size: 64px; font-weight: bold; color: var(--accent); }
.score-label { font-size: 24px; color: var(--text2); margin-left: 4px; }
.result-detail { text-align: left; max-width: 500px; margin: 24px auto; }
.result-item {
  display: flex; justify-content: space-between; padding: 10px 12px;
  border-bottom: 1px solid var(--border); font-size: 14px;
}
.result-item.correct { color: #27ae60; }
.result-item.wrong { color: var(--red); }
</style>
