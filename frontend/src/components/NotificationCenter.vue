<template>
  <div class="notif-wrap" ref="wrapRef">
    <!-- 铃铛入口：未读角标 -->
    <button class="bell-btn" :class="{ 'has-unread': unreadCount > 0 }" @click="toggle" title="通知中心">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
        <path d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9"/>
        <path d="M13.7 21a2 2 0 0 1-3.4 0"/>
      </svg>
      <span v-if="unreadCount > 0" class="bell-badge">{{ unreadCount > 99 ? '99+' : unreadCount }}</span>
    </button>

    <!-- 下拉面板 -->
    <transition name="notif">
      <div v-if="open" class="notif-panel">
        <div class="notif-head">
          <h3>通知中心</h3>
          <div class="notif-head-actions">
            <button class="btn btn-sm" @click="load" :disabled="loading">{{ loading ? '…' : '⟳' }}</button>
            <button class="btn btn-sm" @click="markAllRead" :disabled="!unreadCount">全部已读</button>
          </div>
        </div>

        <div class="notif-tabs">
          <button v-for="t in tabs" :key="t.key" class="tab" :class="{ active: tab === t.key }" @click="tab = t.key">
            {{ t.label }}<span v-if="t.count" class="tab-count">{{ t.count }}</span>
          </button>
        </div>

        <div class="notif-body">
          <template v-if="tab === 'alerts'">
            <div v-if="loading" class="notif-empty">加载中…</div>
            <div v-else-if="!alerts.length" class="notif-empty"><SvgIcon name="check" /> 暂无活跃告警</div>
            <div v-for="(a, i) in alerts" :key="'a' + i" class="notif-item" :class="['notif-' + a.level, unread(aKey(a)) && 'unread']">
              <span class="notif-badge">{{ a.level === 'critical' ? '严重' : '警告' }}</span>
              <div class="notif-text">
                <p>{{ a.message }}</p>
                <span class="notif-meta">规则：{{ a.rule }}</span>
              </div>
            </div>
          </template>
          <template v-else>
            <div v-if="loading" class="notif-empty">加载中…</div>
            <div v-else-if="!events.length" class="notif-empty">暂无最近事件</div>
            <div v-for="(e, i) in events" :key="'e' + i" class="notif-item">
              <span class="notif-badge badge-muted">{{ shortAction(e.action) }}</span>
              <div class="notif-text">
                <p>{{ actionText(e) }}</p>
                <span class="notif-meta">{{ e.detail ? e.detail + ' · ' : '' }}{{ e.ip }} · {{ fmtTime(e.created_at) }}</span>
              </div>
            </div>
          </template>
        </div>
      </div>
    </transition>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import api from '../api'

const READ_KEY = 'lab_notif_read_v1'
const open = ref(false)
const tab = ref('alerts')
const loading = ref(false)
const alerts = ref([])
const events = ref([])
const wrapRef = ref(null)
const readSet = ref(new Set(JSON.parse(localStorage.getItem(READ_KEY) || '[]')))

const tabs = computed(() => [
  { key: 'alerts', label: '活跃告警', count: alerts.value.length },
  { key: 'events', label: '最近事件', count: events.value.length }
])

const unreadCount = computed(() => alerts.value.filter(a => !readSet.value.has(aKey(a))).length)

function aKey(a) { return `${a.rule}|${a.level}|${a.message}` }
function unread(k) { return !readSet.value.has(k) }

// 审计 action 形如 "login" 或 "POST /api/v1/users"：前者映射中文，后者取 HTTP 方法
function shortAction(a) {
  if (!a) return '-'
  const s = String(a)
  const m = s.match(/^(GET|POST|PUT|DELETE|PATCH)\b/)
  if (m) return m[1]
  const map = { login: '登录', logout: '登出', broadcast: '广播', remote: '远程', exam: '考试' }
  return map[s.toLowerCase()] || s.slice(0, 6)
}

// 事件正文：HTTP 类显示「方法 + 目标」，普通动作显示「动作 + 目标」
function actionText(e) {
  const a = String(e.action || '')
  if (/^(GET|POST|PUT|DELETE|PATCH)\s/.test(a)) return `${a.split(' ')[0]} ${e.target || ''}`
  return e.target ? `${a} → ${e.target}` : a
}

function fmtTime(t) {
  if (!t) return ''
  return new Date(t).toLocaleString('zh-CN', { hour12: false })
}

async function load() {
  loading.value = true
  try {
    const [ra, re] = await Promise.allSettled([
      api.get('/alerts'),
      api.get('/audit/logs', { params: { page: 1, page_size: 10 } })
    ])
    if (ra.status === 'fulfilled') alerts.value = (ra.value.data || {}).alerts || []
    if (re.status === 'fulfilled') events.value = ((re.value.data || {}).list) || []
  } finally { loading.value = false }
}

function toggle() {
  open.value = !open.value
  if (open.value) load()
}

function markAllRead() {
  alerts.value.forEach(a => readSet.value.add(aKey(a)))
  localStorage.setItem(READ_KEY, JSON.stringify([...readSet.value].slice(-200)))
}

// 点击面板外关闭
function onClickOutside(e) {
  if (open.value && wrapRef.value && !wrapRef.value.contains(e.target)) open.value = false
}

// 未读告警变化时（WebSocket metrics 推送场景：告警数增加）
watch(unreadCount, v => { if (v > 0 && open.value) load() })

onMounted(() => {
  document.addEventListener('click', onClickOutside)
  load() // 预加载，保证铃铛角标及时显示
})
onUnmounted(() => document.removeEventListener('click', onClickOutside))
</script>

<style scoped>
.notif-wrap { position: relative; }
.bell-btn {
  position: relative; width: 34px; height: 34px;
  display: inline-flex; align-items: center; justify-content: center;
  border: 1px solid var(--border2); border-radius: 8px;
  background: transparent; color: var(--text2); cursor: pointer;
}
.bell-btn:hover { color: var(--text); border-color: var(--green); }
.bell-btn svg { width: 17px; height: 17px; }
.bell-badge {
  position: absolute; top: -6px; right: -6px;
  min-width: 16px; height: 16px; padding: 0 4px;
  border-radius: 8px; background: var(--red); color: #fff;
  font-size: 10px; line-height: 16px; text-align: center;
}
.notif-panel {
  position: absolute; right: 0; top: 42px; z-index: 200;
  width: 380px; max-width: 92vw;
  background: var(--card); border: 1px solid var(--border2);
  border-radius: 12px; box-shadow: 0 12px 40px rgba(20, 40, 90, .22);
  overflow: hidden;
}
.notif-head { display: flex; justify-content: space-between; align-items: center; padding: 12px 16px; border-bottom: 1px solid var(--border); }
.notif-head h3 { font-size: 14px; }
.notif-head-actions { display: flex; gap: 6px; }
.notif-tabs { display: flex; border-bottom: 1px solid var(--border); }
.tab {
  flex: 1; padding: 9px 0; font-size: 13px; color: var(--muted);
  background: transparent; border: none; cursor: pointer;
  border-bottom: 2px solid transparent;
}
.tab.active { color: var(--green); border-bottom-color: var(--green); }
.tab-count { margin-left: 4px; font-size: 11px; color: var(--muted); }
.notif-body { max-height: 340px; overflow-y: auto; padding: 8px 0; }
.notif-empty { text-align: center; color: var(--muted); font-size: 13px; padding: 28px 0; }
.notif-item {
  display: flex; gap: 10px; padding: 10px 16px;
  border-bottom: 1px dashed var(--border); font-size: 13px;
}
.notif-item.unread { background: rgba(0, 179, 104, .05); }
.notif-item:last-child { border-bottom: none; }
.notif-badge {
  flex-shrink: 0; height: fit-content; margin-top: 2px;
  padding: 1px 8px; border-radius: 4px; font-size: 11px; font-weight: 600;
}
.notif-warning .notif-badge { background: rgba(245, 158, 11, .15); color: #f59e0b; }
.notif-critical .notif-badge { background: rgba(239, 68, 68, .15); color: #ef4444; }
.badge-muted { background: rgba(138, 153, 184, .12); color: var(--muted); }
.notif-text { flex: 1; min-width: 0; }
.notif-text p { margin: 0 0 3px; line-height: 1.5; word-break: break-all; }
.notif-meta { font-size: 11px; color: var(--muted); }
.notif-enter-active, .notif-leave-active { transition: all .2s; }
.notif-enter-from, .notif-leave-to { opacity: 0; transform: translateY(-6px); }
</style>
