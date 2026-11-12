<template>
  <div>
    <div class="card">
      <h3 style="margin-top:0"><SvgIcon name="link" /> 对接配置</h3>
      <div class="toolbar">
        <button class="btn btn-primary" @click="showCfg = true">＋ 新增对接</button>
        <button class="btn" @click="load"><SvgIcon name="refresh" /> 刷新</button>
      </div>
      <table>
        <thead><tr><th>ID</th><th>对接名称</th><th>推送密钥（X-Sync-Key）</th><th>成绩回推地址</th><th>创建时间</th></tr></thead>
        <tbody>
          <tr v-for="c in configs" :key="c.id">
            <td>{{ c.id }}</td>
            <td>{{ c.name }}</td>
            <td><code>{{ c.sync_key }}</code></td>
            <td>{{ c.callback_url || '-' }}</td>
            <td>{{ fmt(c.created_at) }}</td>
          </tr>
          <tr v-if="!configs.length"><td colspan="5" style="text-align:center;color:var(--muted)">暂无对接配置</td></tr>
        </tbody>
      </table>
      <div class="hint">
        教务系统推送地址：<code>POST /api/v1/edu/push/students|courses|classes|schedules</code>，请求头携带 <code>X-Sync-Key: {推送密钥}</code>
      </div>
    </div>

    <div class="card">
      <h3 style="margin-top:0"><SvgIcon name="clipboard" /> 同步日志（最近 100 条）</h3>
      <table>
        <thead><tr><th>ID</th><th>来源</th><th>数据类型</th><th>接收条数</th><th>结果</th><th>时间</th></tr></thead>
        <tbody>
          <tr v-for="l in logs" :key="l.id">
            <td>{{ l.id }}</td>
            <td>{{ l.source }}</td>
            <td><span class="tag">{{ actionName(l.action) }}</span></td>
            <td>{{ l.total }}</td>
            <td><span :class="['tag', l.success ? 'tag-on' : 'tag-off']">{{ l.success ? '成功' : '失败' }}</span></td>
            <td>{{ fmt(l.created_at) }}</td>
          </tr>
          <tr v-if="!logs.length"><td colspan="6" style="text-align:center;color:var(--muted)">暂无同步记录</td></tr>
        </tbody>
      </table>
    </div>

    <div class="card">
      <h3 style="margin-top:0"><SvgIcon name="grad" /> 考试成绩查询</h3>
      <div class="toolbar">
        <input class="input" v-model="scoreQuery.exam_id" placeholder="考试 ID" style="width:120px">
        <input class="input" v-model="scoreQuery.sno" placeholder="学号（可选）" style="width:140px">
        <button class="btn btn-primary" @click="queryScores">查询</button>
        <a class="btn" :href="exportUrl" v-if="scoreQuery.exam_id"><SvgIcon name="download" /> 导出 CSV</a>
      </div>
      <table v-if="scores !== null">
        <thead><tr><th>学号</th><th>姓名</th><th>总分</th><th>答对题数</th><th>交卷时间</th></tr></thead>
        <tbody>
          <tr v-for="s in scores" :key="s.sno">
            <td>{{ s.sno }}</td>
            <td>{{ s.name }}</td>
            <td><b>{{ s.total_score }}</b></td>
            <td>{{ s.correct_count }}</td>
            <td>{{ fmt(s.submitted_at) }}</td>
          </tr>
          <tr v-if="!scores.length"><td colspan="5" style="text-align:center;color:var(--muted)">该考试暂无成绩数据</td></tr>
        </tbody>
      </table>
    </div>

    <!-- 新增对接弹窗 -->
    <div v-if="showCfg" class="modal-mask" @click.self="showCfg = false">
      <div class="modal">
        <h3 style="margin-top:0">新增教务对接</h3>
        <div class="form-row"><label>对接名称 *</label><input class="input" v-model="cfgForm.name" placeholder="如 正方教务"></div>
        <div class="form-row"><label>推送密钥</label><input class="input" v-model="cfgForm.sync_key" placeholder="留空自动生成"></div>
        <div class="form-row"><label>回推地址</label><input class="input" v-model="cfgForm.callback_url" placeholder="可选"></div>
        <div class="modal-actions">
          <button class="btn" @click="showCfg = false">取消</button>
          <button class="btn btn-primary" @click="saveCfg">保存</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import api from '../api'

const configs = ref([])
const logs = ref([])
const scores = ref(null)
const showCfg = ref(false)
const cfgForm = ref({ name: '', sync_key: '', callback_url: '' })
const scoreQuery = ref({ exam_id: '', sno: '' })

const exportUrl = computed(() => `/api/v1/edu/scores/export?exam_id=${scoreQuery.value.exam_id}`)

async function load() {
  const [cs, ls] = await Promise.all([api.get('/edu/sync-configs'), api.get('/edu/sync-logs')])
  configs.value = cs.data || []
  logs.value = ls.data || []
}

async function saveCfg() {
  if (!cfgForm.value.name) { alert('对接名称必填'); return }
  try {
    await api.post('/edu/sync-configs', cfgForm.value)
    showCfg.value = false
    cfgForm.value = { name: '', sync_key: '', callback_url: '' }
    load()
  } catch (e) { alert(e.response?.data?.msg || '保存失败') }
}

async function queryScores() {
  if (!scoreQuery.value.exam_id) { alert('请填写考试 ID'); return }
  try {
    const params = new URLSearchParams({ exam_id: scoreQuery.value.exam_id })
    if (scoreQuery.value.sno) params.set('sno', scoreQuery.value.sno)
    const res = await api.get('/edu/scores?' + params)
    scores.value = res.data || []
  } catch (e) { alert(e.response?.data?.msg || '查询失败') }
}

function actionName(a) {
  return { students: '学生名单', courses: '课程', classes: '班级', schedules: '课表', scores: '成绩' }[a] || a
}

function fmt(t) {
  if (!t) return '-'
  return new Date(t).toLocaleString('zh-CN')
}

onMounted(load)
</script>

<style scoped>
.modal-mask { position: fixed; inset: 0; background: rgba(0,0,0,.45); display: flex; align-items: center; justify-content: center; z-index: 100; }
.modal { background: var(--card); border-radius: 8px; padding: 24px; width: 460px; box-shadow: 0 8px 32px rgba(0,0,0,.2); }
.form-row { margin-bottom: 12px; }
.form-row label { display: inline-block; width: 90px; color: var(--text2); }
.input { border: 1px solid var(--border2); border-radius: 4px; padding: 6px 8px; }
.modal-actions { text-align: right; margin-top: 16px; }
.hint { margin-top: 12px; color: var(--text2); font-size: 13px; }
code { background: var(--bg2); padding: 1px 6px; border-radius: 3px; }
</style>
