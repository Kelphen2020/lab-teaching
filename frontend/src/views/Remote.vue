<template>
  <div>
    <!-- 远程控制操作栏 -->
    <div class="card">
      <h3 style="margin-bottom:16px"><SvgIcon name="monitor" /> 远程桌面控制</h3>
      <div class="form-grid">
        <label>目标学生机
          <select v-model="selectedAgent" :disabled="connected">
            <option value="" disabled>— 选择在线终端 —</option>
            <option v-for="a in onlineAgents" :key="a.agent_id" :value="a.agent_id">
              {{ a.agent_id }}{{ a.hostname ? '（' + a.hostname + '）' : '' }}
            </option>
          </select>
        </label>
        <label>帧率
          <select v-model="fps" :disabled="connected">
            <option :value="2">2 FPS（低带宽）</option>
            <option :value="5">5 FPS（流畅）</option>
            <option :value="10">10 FPS（高刷）</option>
          </select>
        </label>
        <label>画质
          <select v-model="quality" :disabled="connected">
            <option :value="50">50%（省流量）</option>
            <option :value="75">75%（均衡）</option>
            <option :value="90">90%（高清）</option>
          </select>
        </label>
      </div>
      <div class="toolbar" style="margin-top:12px">
        <button class="btn btn-success" :disabled="!selectedAgent || connected" @click="startRemote">
          <SvgIcon :name="connecting ? 'clock' : 'play'" /> {{ connecting ? '连接中...' : '连接控制' }}
        </button>
        <button class="btn btn-danger" :disabled="!connected" @click="stopRemote"><SvgIcon name="stop" /> 断开</button>
        <span v-if="connected" class="tag tag-on">● 已连接 {{ selectedAgent }}</span>
        <span v-else-if="connecting" class="tag tag-warn">连接中…</span>
        <span v-else class="tag tag-off">未连接</span>
      </div>
      <div v-if="result" :class="['result-box', resultOk ? 'result-ok' : 'result-err']" style="margin-top:12px">
        {{ result }}
      </div>
    </div>

    <!-- 远程桌面查看器 -->
    <div class="viewer-shell" :class="{ active: connected }">
      <div class="viewer-header">
        <span class="viewer-dot" :class="connected ? 'dot-live' : 'dot-idle'"></span>
        <span class="viewer-title">{{ connected ? selectedAgent + ' — 远程桌面' : '远程桌面查看器' }}</span>
        <span v-if="connected" class="viewer-stats">
          帧 {{ frameCount }} · {{ fpsLive }} FPS · {{ latency }}ms
        </span>
      </div>
      <div class="viewer-body" ref="viewerRef">
        <div v-if="!connected" class="viewer-placeholder">
          <div class="placeholder-icon"><SvgIcon name="monitor" /> </div>
          <p>选择学生机后点击「连接控制」</p>
          <p class="placeholder-hint">连接后可实时查看学生屏幕，并进行鼠标键盘操作</p>
        </div>
        <canvas
          v-show="connected"
          ref="canvasRef"
          class="remote-canvas"
          tabindex="0"
          @mousedown="onMouseDown"
          @mouseup="onMouseUp"
          @mousemove="onMouseMove"
          @wheel.prevent="onWheel"
          @contextmenu.prevent
          @keydown="onKeyDown"
        />
      </div>
      <div v-if="connected" class="viewer-footer">
        <span><SvgIcon name="bulb" /> 点击画面获取焦点后可发送键盘输入 · 滚轮可滚动学生屏幕</span>
        <button class="btn btn-sm btn-primary" @click="sendCtrlAltDel">发送 Ctrl+Alt+Del</button>
      </div>
    </div>

    <!-- 会话记录 -->
    <div class="card">
      <h3 style="margin-bottom:12px"><SvgIcon name="clipboard" /> 会话记录</h3>
      <table class="data-table">
        <thead>
          <tr><th>ID</th><th>学生机</th><th>状态</th><th>开始时间</th></tr>
        </thead>
        <tbody>
          <tr v-for="s in pagedSessions" :key="s.id">
            <td>{{ s.id }}</td>
            <td>{{ s.agent_id }}</td>
            <td><span :class="['tag', s.status === 'active' ? 'tag-on' : 'tag-off']">{{ s.status }}</span></td>
            <td>{{ s.started_at || '-' }}</td>
          </tr>
          <tr v-if="!pagedSessions.length"><td colspan="4" style="text-align:center;color:var(--muted)">暂无记录</td></tr>
        </tbody>
      </table>
      <Pagination :page="sessPage" :page-size="10" :total="sessions.length" @update:page="p => sessPage = p" />
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import api from '../api'
import Pagination from '../components/Pagination.vue'

const onlineAgents = ref([])
const selectedAgent = ref('')
const connected = ref(false)
const connecting = ref(false)
const sessions = ref([])
const sessPage = ref(1)
const frameCount = ref(0)
const fpsLive = ref(0)
const latency = ref(0)
const result = ref('')
const resultOk = ref(true)
const fps = ref(5)
const quality = ref(75)

const canvasRef = ref(null)
const viewerRef = ref(null)

let ws = null
let sessionId = null
let fpsTimer = null
let lastFrameTime = 0
let frameInSecond = 0

// 数据契约归一化：后端 /agents/online 返回 {online_count, online_ids, sessions} 对象
function normalizeAgents(data) {
  let list = []
  if (Array.isArray(data)) {
    list = data
  } else if (data && Array.isArray(data.sessions)) {
    list = data.sessions
  }
  return list.filter(a => a && (a.online || a.is_online))
}

async function loadAgents() {
  try {
    const res = await api.get('/agents/online')
    onlineAgents.value = normalizeAgents(res.data)
  } catch (e) { console.error(e) }
}

async function loadSessions() {
  try {
    const res = await api.get('/remote/sessions')
    sessions.value = Array.isArray(res.data) ? res.data : []
  } catch (e) { console.error(e) }
}

const pagedSessions = computed(() => {
  const size = 10
  const s = (sessPage.value - 1) * size
  return sessions.value.slice(s, s + size)
})

async function startRemote() {
  if (!selectedAgent.value) return
  connecting.value = true
  result.value = ''
  try {
    const res = await api.post('/remote/start', {
      agent_id: selectedAgent.value,
      fps: fps.value,
      quality: quality.value
    })
    sessionId = res.data.session_id
    connectViewer()
    resultOk.value = true
    result.value = `远程控制会话已创建（ID: ${sessionId}），等待画面…`
  } catch (e) {
    resultOk.value = false
    result.value = e.response?.data?.msg || e.message
    connecting.value = false
  }
}

function connectViewer() {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const token = localStorage.getItem('token') || ''
  const wsUrl = `${proto}//${location.host}/remote/ws?agent_id=${encodeURIComponent(selectedAgent.value)}&token=${encodeURIComponent(token)}`
  ws = new WebSocket(wsUrl)
  ws.binaryType = 'arraybuffer'

  ws.onopen = () => {
    connected.value = true
    connecting.value = false
    frameCount.value = 0
    fpsLive.value = 0
    lastFrameTime = performance.now()
    fpsTimer = setInterval(() => {
      fpsLive.value = frameInSecond
      frameInSecond = 0
    }, 1000)
    requestAnimationFrame(() => canvasRef.value?.focus())
  }

  ws.onmessage = (event) => {
    if (event.data instanceof ArrayBuffer) {
      renderFrame(event.data)
    }
  }

  ws.onclose = () => {
    const wasConnected = connected.value
    connected.value = false
    connecting.value = false
    cleanup()
    resetLiveStats()
    if (wasConnected) {
      resultOk.value = true
      result.value = '远程连接已断开，可重新选择终端后再次连接'
    }
  }

  ws.onerror = () => {
    resultOk.value = false
    result.value = 'WebSocket 连接失败，请检查后端服务后重试'
    connected.value = false
    connecting.value = false
    cleanup()
    resetLiveStats()
  }
}

function resetLiveStats() {
  frameCount.value = 0
  fpsLive.value = 0
  latency.value = 0
  frameInSecond = 0
}

function renderFrame(buffer) {
  const now = performance.now()
  latency.value = Math.round(now - lastFrameTime)
  lastFrameTime = now
  frameCount.value++
  frameInSecond++

  // 前4字节是帧序号（小端），后面是 JPEG 数据
  const jpegData = new Uint8Array(buffer, 4)
  const blob = new Blob([jpegData], { type: 'image/jpeg' })
  const url = URL.createObjectURL(blob)
  const img = new Image()
  img.onload = () => {
    const canvas = canvasRef.value
    if (!canvas) { URL.revokeObjectURL(url); return }
    if (canvas.width !== img.width || canvas.height !== img.height) {
      canvas.width = img.width
      canvas.height = img.height
    }
    const ctx = canvas.getContext('2d')
    ctx.drawImage(img, 0, 0)
    URL.revokeObjectURL(url)
  }
  img.src = url
}

async function stopRemote() {
  if (sessionId) {
    try { await api.post(`/remote/${sessionId}/stop`) } catch (e) { /* ignore */ }
  }
  if (ws) { ws.close(); ws = null }
  connected.value = false
  cleanup()
  resetLiveStats()
  loadSessions()
  resultOk.value = true
  result.value = '远程控制已断开'
}

function cleanup() {
  if (fpsTimer) { clearInterval(fpsTimer); fpsTimer = null }
}

// ============ 输入事件 ============
function getCanvasCoords(e) {
  const canvas = canvasRef.value
  if (!canvas) return { x: 0, y: 0 }
  const rect = canvas.getBoundingClientRect()
  const scaleX = canvas.width / rect.width
  const scaleY = canvas.height / rect.height
  return {
    x: Math.round((e.clientX - rect.left) * scaleX),
    y: Math.round((e.clientY - rect.top) * scaleY),
  }
}

function sendInput(data) {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify(data))
  }
}

function onMouseDown(e) {
  canvasRef.value?.focus()
  const { x, y } = getCanvasCoords(e)
  sendInput({ type: 'mouse_click', x, y, button: e.button + 1 })
}

function onMouseUp(e) {
  // xdotool click = down+up 一次完成，mousedown 已处理
}

let lastMoveSend = 0
function onMouseMove(e) {
  // 节流：每 50ms 最多发一次移动事件
  const now = Date.now()
  if (now - lastMoveSend < 50) return
  lastMoveSend = now
  const { x, y } = getCanvasCoords(e)
  sendInput({ type: 'mouse_move', x, y })
}

function onWheel(e) {
  sendInput({ type: 'scroll', direction: e.deltaY < 0 ? 'up' : 'down' })
}

function onKeyDown(e) {
  if (['Tab', 'F5', 'F11', 'Escape'].includes(e.key)) {
    e.preventDefault()
  }
  let key = e.key
  if (key === ' ') key = 'space'
  else if (key === 'Enter') key = 'Return'
  else if (key === 'Backspace') key = 'BackSpace'
  else if (key === 'ArrowUp') key = 'Up'
  else if (key === 'ArrowDown') key = 'Down'
  else if (key === 'ArrowLeft') key = 'Left'
  else if (key === 'ArrowRight') key = 'Right'

  if (e.ctrlKey && key.length === 1) {
    sendInput({ type: 'key', key: `ctrl+${key}` })
  } else if (e.altKey && key.length === 1) {
    sendInput({ type: 'key', key: `alt+${key}` })
  } else {
    sendInput({ type: 'key', key })
  }
}

function sendCtrlAltDel() {
  sendInput({ type: 'key', key: 'ctrl+alt+Delete' })
}

onMounted(() => {
  loadAgents()
  loadSessions()
})

onBeforeUnmount(() => {
  if (ws) { ws.close(); ws = null }
  cleanup()
})
</script>

<style scoped>
.form-grid {
  display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px;
}
.form-grid label {
  display: flex; flex-direction: column; gap: 4px;
  font-size: 13px; color: var(--text2);
}
.form-grid select {
  padding: 6px 10px; border: 1px solid var(--border2); border-radius: 4px; font-size: 14px;
}
.toolbar { display: flex; align-items: center; gap: 10px; }
.tag-warn { background: rgba(245,158,11,.12); color: #d48806; border: 1px solid rgba(245,158,11,.30); }
.result-box { padding: 10px 14px; border-radius: 4px; font-size: 14px; }
.result-ok { background: rgba(0,179,104,.13); border: 1px solid rgba(0,179,104,.28); color: #00915b; }
.result-err { background: rgba(229,72,77,.10); border: 1px solid rgba(229,72,77,.28); color: #c93a3f; }

/* 查看器外壳 — 深色终端风格 */
.viewer-shell {
  background: #0d1117;
  border-radius: 8px;
  overflow: hidden;
  margin-bottom: 16px;
  border: 1px solid #21262d;
  transition: box-shadow 0.3s;
}
.viewer-shell.active {
  box-shadow: 0 0 0 1px #238636, 0 4px 24px rgba(35, 134, 54, 0.15);
}
.viewer-header {
  display: flex; align-items: center; gap: 10px;
  padding: 8px 14px;
  background: #161b22;
  border-bottom: 1px solid #21262d;
  font-size: 13px; color: #8b949e;
}
.viewer-dot {
  width: 8px; height: 8px; border-radius: 50%;
  background: #484f58;
  transition: background 0.3s;
}
.dot-live {
  background: #3fb950;
  animation: pulse 1.5s infinite;
}
@keyframes pulse {
  0%, 100% { box-shadow: 0 0 0 0 rgba(63, 185, 80, 0.5); }
  50% { box-shadow: 0 0 0 5px rgba(63, 185, 80, 0); }
}
.viewer-title { font-weight: 600; color: #c9d1d9; }
.viewer-stats {
  margin-left: auto;
  font-family: 'Courier New', monospace;
  font-size: 12px; color: #3fb950;
}
.viewer-body {
  position: relative;
  min-height: 420px;
  display: flex; align-items: center; justify-content: center;
  background:
    radial-gradient(ellipse at 50% 0%, rgba(56, 139, 253, 0.06) 0%, transparent 60%),
    #0d1117;
}
.viewer-placeholder {
  text-align: center; color: #484f58; padding: 60px 20px;
}
.placeholder-icon { font-size: 48px; margin-bottom: 12px; opacity: 0.6; }
.viewer-placeholder p { margin: 4px 0; font-size: 14px; }
.placeholder-hint { font-size: 12px; color: #30363d; }
.remote-canvas {
  max-width: 100%;
  max-height: 68vh;
  cursor: crosshair;
  outline: none;
  display: block;
}
.remote-canvas:focus {
  outline: 2px solid #238636;
  outline-offset: -2px;
}
.viewer-footer {
  display: flex; align-items: center; justify-content: space-between;
  padding: 6px 14px;
  background: #161b22;
  border-top: 1px solid #21262d;
  font-size: 12px; color: #8b949e;
}
.btn-sm { padding: 3px 10px; font-size: 12px; }

.data-table { width: 100%; border-collapse: collapse; font-size: 13px; }
.data-table th, .data-table td {
  padding: 8px 10px; border-bottom: 1px solid var(--border); text-align: left;
}
.data-table th { background: var(--card2); color: var(--text2); font-weight: 600; }
</style>
