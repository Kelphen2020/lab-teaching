<template>
  <div class="card">
    <div class="toolbar">
      <select v-model="roomId" @change="loadSeats">
        <option value="">选择机房…</option>
        <option v-for="r in rooms" :key="r.id" :value="r.id">{{ r.name }}</option>
      </select>
      <select v-model="fps">
        <option :value="1">1 帧/秒</option>
        <option :value="2">2 帧/秒</option>
        <option :value="3">3 帧/秒</option>
      </select>
      <button class="btn btn-primary" :disabled="!roomId || wallRunning" @click="startWall"><SvgIcon name="play" /> 开始监控</button>
      <button class="btn btn-danger-outline" :disabled="!wallRunning" @click="stopWall"><SvgIcon name="stop" /> 停止</button>
      <span class="poll-tip" :class="wsConnected ? 'live' : ''">
        <span class="dot"></span>{{ wsConnected ? `缩略图流已连接（${frameCount} 帧）` : '缩略图流未连接' }}
      </span>
      <span v-if="roomId" style="color:var(--text2);font-size:12px;margin-left:auto">
        在线 <b style="color:var(--green)">{{ onlineCount }}</b> / {{ seats.length }} 座
      </span>
    </div>

    <ListState :state="state" empty-text="请先选择机房" empty-hint="选择机房后点击「开始监控」，将实时显示各座位屏幕缩略图" error-text="座位数据加载失败" @retry="loadSeats">
      <div class="wall-grid" :style="{ gridTemplateColumns: `repeat(${cols}, 1fr)` }">
        <div v-for="s in seats" :key="s.seat_id" class="wall-cell" :class="{ offline: !s.online }" @click="openRemote(s)">
          <div class="wall-head">
            <span class="seat-no">{{ s.seat_no }}</span>
            <span class="mono agent">{{ s.agent_id || '-' }}</span>
            <span class="on-dot" :class="s.online ? 'on' : 'off'"></span>
          </div>
          <div class="wall-frame">
            <img v-if="frames[s.agent_id]" :src="frames[s.agent_id]" alt="" />
            <div v-else class="wall-placeholder">{{ s.online ? (wallRunning ? '等待画面…' : '未开始监控') : '离线' }}</div>
          </div>
        </div>
      </div>
    </ListState>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import api from '../api'
import ListState from '../components/ListState.vue'
import { useRouter } from 'vue-router'

const router = useRouter()
const rooms = ref([])
const roomId = ref('')
const seats = ref([])
const state = ref('empty')
const fps = ref(1)
const wallRunning = ref(false)
const wsConnected = ref(false)
const frameCount = ref(0)
const frames = ref({})   // agent_id → objectURL
let ws = null
const COLS = 5

const cols = computed(() => {
  const n = seats.value.length
  if (!n) return COLS
  return Math.min(COLS, Math.max(2, Math.ceil(Math.sqrt(n))))
})
const onlineCount = computed(() => seats.value.filter(s => s.online).length)

async function loadRooms() {
  try {
    const r = await api.get('/rooms')
    rooms.value = r.data || []
    if (rooms.value.length && !roomId.value) {
      roomId.value = rooms.value[0].id
      await loadSeats()
    }
  } catch { rooms.value = [] }
}

async function loadSeats() {
  if (!roomId.value) { state.value = 'empty'; return }
  state.value = 'loading'
  try {
    const r = await api.get('/wall/seats', { params: { room_id: roomId.value } })
    seats.value = r.data || []
    state.value = 'ready'
  } catch {
    if (!seats.value.length) state.value = 'error'
  }
}

// ---- WebSocket 缩略图流 ----
// 帧格式（server → browser）：[2字节 agent_id 长度 LE][agent_id][4字节头(最高位=1)][JPEG]
function connectWS() {
  disconnectWS()
  const token = localStorage.getItem('token') || ''
  const proto = location.protocol === 'https:' ? 'wss://' : 'ws://'
  const url = `${proto}//${location.host}/ws/wall?token=${encodeURIComponent(token)}`
  ws = new WebSocket(url)
  ws.binaryType = 'arraybuffer'
  ws.onopen = () => { wsConnected.value = true }
  ws.onclose = () => { wsConnected.value = false }
  ws.onerror = () => { wsConnected.value = false }
  ws.onmessage = (ev) => {
    if (!(ev.data instanceof ArrayBuffer)) return
    const buf = new Uint8Array(ev.data)
    if (buf.length < 6) return
    const idLen = buf[0] | (buf[1] << 8)
    if (2 + idLen + 4 > buf.length) return
    const agentId = new TextDecoder().decode(buf.slice(2, 2 + idLen))
    const jpeg = buf.slice(2 + idLen + 4)  // 跳过 4 字节帧头
    const blob = new Blob([jpeg], { type: 'image/jpeg' })
    const url2 = URL.createObjectURL(blob)
    const old = frames.value[agentId]
    frames.value = { ...frames.value, [agentId]: url2 }
    if (old) URL.revokeObjectURL(old)
    frameCount.value++
  }
}

function disconnectWS() {
  if (ws) {
    ws.onclose = null
    ws.close()
    ws = null
  }
  wsConnected.value = false
}

async function startWall() {
  if (!roomId.value) return
  try {
    const r = await api.post('/wall/start', { room_id: roomId.value, fps: fps.value, quality: 40 })
    if (r.code === 0) {
      wallRunning.value = true
      frameCount.value = 0
      connectWS()
    }
  } catch (e) {
    alert(e.response?.data?.msg || '启动失败')
  }
}

async function stopWall() {
  try { await api.post('/wall/stop', { room_id: roomId.value }) } catch {}
  wallRunning.value = false
  disconnectWS()
  // 清空画面
  for (const u of Object.values(frames.value)) URL.revokeObjectURL(u)
  frames.value = {}
}

function openRemote(s) {
  if (!s.online || !s.agent_id) return
  router.push(`/remote?agent_id=${encodeURIComponent(s.agent_id)}`)
}

watch(roomId, () => { if (wallRunning.value) stopWall() })

onMounted(() => { loadRooms() })
onUnmounted(() => { disconnectWS(); for (const u of Object.values(frames.value)) URL.revokeObjectURL(u) })
</script>

<style scoped>
.toolbar { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.wall-grid { display: grid; gap: 10px; margin-top: 14px; }
.wall-cell { border: 1px solid var(--border); border-radius: 8px; overflow: hidden; background: var(--card); cursor: pointer; transition: box-shadow .15s; }
.wall-cell:hover { box-shadow: 0 2px 12px rgba(20, 40, 90, .14); }
.wall-cell.offline { opacity: .55; cursor: default; }
.wall-head { display: flex; align-items: center; gap: 6px; padding: 5px 8px; font-size: 11px; border-bottom: 1px solid var(--border); }
.seat-no { font-weight: 600; color: var(--accent); }
.agent { color: var(--text2); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.on-dot { width: 7px; height: 7px; border-radius: 50%; margin-left: auto; flex: none; }
.on-dot.on { background: var(--green); }
.on-dot.off { background: var(--muted); }
.wall-frame { aspect-ratio: 16 / 10; background: #0d1526; display: flex; align-items: center; justify-content: center; }
.wall-frame img { width: 100%; height: 100%; object-fit: contain; }
.wall-placeholder { color: var(--muted); font-size: 11px; }
</style>
