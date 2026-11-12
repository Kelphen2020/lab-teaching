<template>
  <div class="card">
    <div class="toolbar">
      <span class="sev" :class="'sev-' + severity">
        <span class="sev-dot"></span>
        {{ severity === 'ok' ? '系统运行正常' : severity === 'warning' ? '存在告警，请关注' : '存在严重告警' }}
      </span>
      <span class="live-badge" :class="connected ? 'live-on' : 'live-off'">
        <span class="live-dot"></span>
        {{ connected ? '实时推送已连接' : '轮询模式' }}
      </span>
      <button class="btn" style="margin-left:auto" :disabled="loading" @click="loadAll">⟳ 刷新</button>
    </div>

    <!-- 告警列表 -->
    <div class="alert-box">
      <div v-if="loading" class="empty-cell">加载中…</div>
      <div v-else-if="!alerts.length" class="ok-box"><SvgIcon name="check" /> 当前没有触发的告警规则</div>
      <div v-else class="alert-item" :class="'alert-' + a.level" v-for="(a, i) in alerts" :key="i">
        <span class="alert-badge">{{ a.level === 'critical' ? '严重' : '警告' }}</span>
        <span class="alert-msg">{{ a.message }}</span>
        <span class="alert-rule">规则: {{ a.rule }}</span>
      </div>
    </div>

    <!-- 指标卡片 -->
    <div class="stat-grid">
      <div class="stat-card">
        <div class="stat-label">运行时长</div>
        <div class="stat-value">{{ fmtUptime(metrics.uptime_seconds) }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">终端在线</div>
        <div class="stat-value" :class="{ 'stat-bad': metrics.agents_total > 0 && metrics.agents_online / metrics.agents_total < 0.8 }">
          {{ metrics.agents_online ?? '—' }} / {{ metrics.agents_total ?? '—' }}
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-label">HTTP 请求总数</div>
        <div class="stat-value">{{ metrics.http_requests_total ?? '—' }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">HTTP 错误数</div>
        <div class="stat-value" :class="{ 'stat-bad': metrics.http_errors_total > 0 }">{{ metrics.http_errors_total ?? '—' }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">平均延迟</div>
        <div class="stat-value">{{ metrics.avg_latency_micros != null ? (metrics.avg_latency_micros / 1000).toFixed(1) + ' ms' : '—' }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">数据库</div>
        <div class="stat-value" :class="dbOk ? 'stat-good' : 'stat-bad'">{{ dbOk == null ? '检测中…' : dbOk ? '正常' : '异常' }}</div>
      </div>
    </div>

    <!-- P2-2: 资源监控 -->
    <h3 class="section-title">系统资源</h3>
    <div class="stat-grid">
      <div class="stat-card">
        <div class="stat-label">CPU 负载 (1/5/15m)</div>
        <div class="stat-value">{{ res.cpu?.load1 ?? '—' }} / {{ res.cpu?.load5 ?? '—' }} / {{ res.cpu?.load15 ?? '—' }}</div>
        <div class="stat-sub">{{ res.cpu?.cores ?? '—' }} 核</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">内存 (Go 堆)</div>
        <div class="stat-value">{{ res.memory?.alloc_mb ?? '—' }} MB</div>
        <div class="stat-sub">系统占用 {{ res.memory?.sys_mb ?? '—' }} MB · GC {{ res.memory?.gc_runs ?? '—' }} 次</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">磁盘</div>
        <div class="stat-value" :class="{ 'stat-bad': (res.disk?.used_pct ?? 0) > 90 }">{{ res.disk?.free_gb ?? '—' }} GB 可用</div>
        <div class="stat-sub">总计 {{ res.disk?.total_gb ?? '—' }} GB · {{ res.disk?.used_pct ?? '—' }}% 已用</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">数据库连接池</div>
        <div class="stat-value">{{ res.db_pool?.in_use ?? '—' }} / {{ res.db_pool?.open_conns ?? '—' }}</div>
        <div class="stat-sub">空闲 {{ res.db_pool?.idle ?? '—' }} · 等待 {{ res.db_pool?.wait_count ?? '—' }}</div>
      </div>
    </div>

    <p class="monitor-tip">
      告警规则：终端在线率 &lt;80% 警告 / &lt;50% 严重；最近升级任务失败节点 ≥50% 或任务失败为严重；HTTP 错误率 ≥5% 警告 / ≥20% 严重（样本量 &gt;100）。数据随请求实时统计，刷新查看最新状态。
    </p>

    <!-- P2-1: 告警规则编辑 -->
    <details class="alert-rules-section">
      <summary><SvgIcon name="gear" /> 告警阈值设置</summary>
      <div class="rules-grid">
        <label>Agent 在线率警告阈值 (%)<input type="number" v-model.number="alertRules.agent_online_warn" min="0" max="100" /></label>
        <label>Agent 在线率严重阈值 (%)<input type="number" v-model.number="alertRules.agent_online_critical" min="0" max="100" /></label>
        <label>HTTP 错误率警告阈值 (%)<input type="number" v-model.number="alertRules.http_error_warn" min="0" max="100" /></label>
        <label>HTTP 错误率严重阈值 (%)<input type="number" v-model.number="alertRules.http_error_critical" min="0" max="100" /></label>
      </div>
      <button class="btn btn-primary btn-sm" @click="saveAlertRules" :disabled="rulesSaving">{{ rulesSaving ? '保存中…' : '保存设置' }}</button>
      <span v-if="rulesMsg" :class="rulesErr ? 'rules-err' : 'rules-ok'">{{ rulesMsg }}</span>
      <button class="btn btn-sm" style="margin-left:12px" @click="triggerBackup" :disabled="backupBusy"><SvgIcon name="save" /> {{ backupBusy ? '备份中…' : '手动触发备份' }}</button>
      <span v-if="backupMsg" :class="backupErr ? 'rules-err' : 'rules-ok'">{{ backupMsg }}</span>
    </details>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import api from '../api'
import { useEventStream } from '../composables/useEventStream'

const alerts = ref([])
const metrics = ref({})
const res = ref({})
const alertRules = ref({ agent_online_warn: 80, agent_online_critical: 50, http_error_warn: 5, http_error_critical: 20 })
const severity = ref('ok')
const dbOk = ref(null)
const loading = ref(false)
let timer = null
let healthTimer = null
let resTimer = null

// d1: WebSocket 实时推送订阅（连接失败自动重连；断开时降级为轮询）
const { connected, on } = useEventStream()
on('metrics', d => {
  if (!d) return
  alerts.value = d.alerts || []
  metrics.value = d.metrics || {}
  severity.value = d.severity || 'ok'
})

async function loadAll() {
  loading.value = true
  try {
    const res = await api.get('/alerts')
    const d = res.data || {}
    alerts.value = d.alerts || []
    metrics.value = d.metrics || {}
    severity.value = d.severity || 'ok'
  } catch { /* 保持旧数据 */ } finally {
    loading.value = false
  }
}

async function checkHealth() {
  try {
    const r = await fetch('/health')
    const j = await r.json()
    dbOk.value = !!(j && (j.status === 'ok' || j.database === 'ok' || j.db === true))
  } catch {
    dbOk.value = false
  }
}

async function loadResources() {
  try {
    const r = await api.get('/system/resources')
    res.value = r.data || {}
  } catch { /* keep old */ }
}

async function loadAlertRules() {
  try {
    const r = await api.get('/system/alert-rules')
    if (r.data) alertRules.value = r.data
  } catch {}
}

const rulesSaving = ref(false)
const rulesMsg = ref('')
const rulesErr = ref(false)
async function saveAlertRules() {
  rulesSaving.value = true; rulesMsg.value = ''; rulesErr.value = false
  try {
    await api.put('/system/alert-rules', alertRules.value)
    rulesMsg.value = '阈值已保存'
  } catch (e) {
    rulesMsg.value = '保存失败: ' + (e.response?.data?.msg || e.message)
    rulesErr.value = true
  } finally { rulesSaving.value = false }
}

const backupBusy = ref(false)
const backupMsg = ref('')
const backupErr = ref(false)
async function triggerBackup() {
  backupBusy.value = true; backupMsg.value = ''; backupErr.value = false
  try {
    const r = await api.post('/system/backup')
    backupMsg.value = '备份完成'
  } catch (e) {
    backupMsg.value = '备份失败: ' + (e.response?.data?.msg || e.message)
    backupErr.value = true
  } finally { backupBusy.value = false }
}

onMounted(() => {
  loadAll()
  checkHealth()
  loadResources()
  loadAlertRules()
  // d1: 兜底轮询——实时推送已连接时跳过 HTTP 拉取，避免重复请求
  timer = setInterval(() => { if (!connected.value) loadAll() }, 30000)
  healthTimer = setInterval(checkHealth, 60000)
  resTimer = setInterval(loadResources, 30000)
})
onUnmounted(() => { clearInterval(timer); clearInterval(healthTimer); clearInterval(resTimer) })

function fmtUptime(s) {
  if (s == null) return '—'
  const d = Math.floor(s / 86400), h = Math.floor((s % 86400) / 3600), m = Math.floor((s % 3600) / 60)
  if (d > 0) return d + ' 天 ' + h + ' 小时'
  if (h > 0) return h + ' 小时 ' + m + ' 分钟'
  return m + ' 分钟'
}
</script>

<style scoped>
.sev { display: inline-flex; align-items: center; gap: 8px; font-size: 14px; font-weight: 600; }
.sev-dot { width: 10px; height: 10px; border-radius: 50%; display: inline-block; }
.live-badge {
  display: inline-flex; align-items: center; gap: 6px;
  font-size: 12px; padding: 4px 10px; border-radius: 14px; margin-left: 12px;
}
.live-dot { width: 7px; height: 7px; border-radius: 50%; }
.live-on { background: rgba(0,179,104,.1); color: var(--green); }
.live-on .live-dot { background: var(--green); animation: pulse 2s infinite; }
.live-off { background: rgba(138,153,184,.12); color: var(--muted); }
.live-off .live-dot { background: var(--muted); }
.sev-ok { color: #22c55e; } .sev-ok .sev-dot { background: #22c55e; box-shadow: 0 0 8px rgba(34,197,94,.6); }
.sev-warning { color: #f59e0b; } .sev-warning .sev-dot { background: #f59e0b; box-shadow: 0 0 8px rgba(245,158,11,.6); }
.sev-critical { color: #ef4444; } .sev-critical .sev-dot { background: #ef4444; box-shadow: 0 0 8px rgba(239,68,68,.6); animation: pulse 1s infinite; }
@keyframes pulse { 50% { opacity: .4; } }

.alert-box { margin: 16px 0; display: flex; flex-direction: column; gap: 8px; }
.ok-box { padding: 18px; border: 1px dashed var(--border2); border-radius: 8px; text-align: center; color: var(--muted); font-size: 13px; }
.alert-item { display: flex; align-items: center; gap: 12px; padding: 10px 14px; border-radius: 8px; font-size: 13px; }
.alert-warning { background: rgba(245,158,11,.08); border: 1px solid rgba(245,158,11,.35); }
.alert-critical { background: rgba(239,68,68,.08); border: 1px solid rgba(239,68,68,.4); }
.alert-badge { padding: 2px 8px; border-radius: 4px; font-size: 12px; font-weight: 600; flex-shrink: 0; }
.alert-warning .alert-badge { background: rgba(245,158,11,.2); color: #f59e0b; }
.alert-critical .alert-badge { background: rgba(239,68,68,.2); color: #ef4444; }
.alert-msg { flex: 1; }
.alert-rule { font-size: 12px; color: var(--muted); }

.stat-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 12px; margin-top: 8px; }
.stat-card { border: 1px solid var(--border2); border-radius: 10px; padding: 14px 16px; background: var(--card); }
.stat-label { font-size: 12px; color: var(--muted); margin-bottom: 6px; }
.stat-value { font-size: 20px; font-weight: 700; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.stat-good { color: #22c55e; }
.stat-bad { color: #ef4444; }
.stat-sub { font-size: 11px; color: var(--muted); margin-top: 4px; }
.section-title { font-size: 14px; font-weight: 600; margin: 20px 0 10px; color: var(--text2); }
.monitor-tip { margin-top: 16px; font-size: 12px; color: var(--muted); line-height: 1.7; }
.alert-rules-section { margin-top: 16px; padding: 12px 16px; background: var(--bg); border-radius: 8px; border: 1px solid var(--border2); }
.alert-rules-section summary { cursor: pointer; font-size: 13px; font-weight: 600; color: var(--text2); }
.rules-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin: 12px 0; }
.rules-grid label { font-size: 12px; color: var(--text2); display: flex; flex-direction: column; gap: 4px; }
.rules-grid input { padding: 6px 8px; border: 1px solid var(--border2); border-radius: 4px; font-size: 13px; width: 100px; }
.rules-ok { font-size: 12px; color: var(--green); margin-left: 10px; }
.rules-err { font-size: 12px; color: var(--red); margin-left: 10px; }
</style>
