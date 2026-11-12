<template>
  <div>
    <!-- 广播控制台 -->
    <div class="card">
      <h3 style="margin-bottom:16px"><SvgIcon name="cast" /> 屏幕广播控制台</h3>

      <!-- 当前状态指示 -->
      <div class="status-bar" :class="streaming ? 'live' : 'idle'">
        <span class="dot"></span>
        <span v-if="streaming">正在推流 → {{ activeStream.mcast_addr }}（{{ activeStream.source }}源）</span>
        <span v-else>空闲，未推流</span>
      </div>

      <!-- 参数配置 -->
      <div class="form-grid">
        <label>机房
          <select v-model="roomId">
            <option v-for="r in rooms" :key="r.id" :value="r.id">{{ r.name }}</option>
          </select>
        </label>
        <label>视频源
          <select v-model="source">
            <option value="screen">教师屏幕（x11grab）</option>
            <option value="test">测试图案（无头验证）</option>
          </select>
        </label>
        <label>分辨率
          <select v-model="resolution">
            <option value="1920x1080">1920×1080</option>
            <option value="1280x720">1280×720</option>
            <option value="640x480">640×480</option>
          </select>
        </label>
        <label>帧率
          <select v-model.number="fps">
            <option :value="30">30 fps</option>
            <option :value="25">25 fps</option>
            <option :value="15">15 fps</option>
          </select>
        </label>
        <label>码率
          <select v-model="bitrate">
            <option value="8M">8 Mbps（高清）</option>
            <option value="4M">4 Mbps（推荐）</option>
            <option value="2M">2 Mbps（低带宽）</option>
            <option value="1M">1 Mbps（极省）</option>
          </select>
        </label>
        <label>广播范围
          <select v-model="scope">
            <option value="全班">全班</option>
            <option value="指定">指定终端</option>
          </select>
        </label>
      </div>

      <!-- 指定终端 -->
      <div v-if="scope === '指定'" class="toolbar" style="margin-top:8px">
        <input v-model="targetInput" placeholder="Agent ID，逗号分隔（如 RUST-AGENT-01,AGENT-PC-002）" style="flex:1" />
        <span class="tag" :class="targetCount ? 'tag-on' : 'tag-off'">目标 {{ targetCount }} 台</span>
      </div>

      <!-- 操作按钮 -->
      <div class="toolbar" style="margin-top:16px">
        <button class="btn btn-success" :disabled="streaming || starting" @click="startBroadcast">
          <SvgIcon :name="starting ? 'clock' : 'play'" /> {{ starting ? '正在发起…' : '发起广播' }}
        </button>
        <button class="btn btn-danger" :disabled="!streaming || stopping" @click="askStop()">
          <SvgIcon :name="stopping ? 'clock' : 'stop'" /> {{ stopping ? '正在停止…' : '停止广播' }}
        </button>
        <button class="btn btn-primary" @click="refreshStatus"><SvgIcon name="refresh" /> 刷新状态</button>
      </div>

      <!-- 执行结果 -->
      <div v-if="result" class="card" :style="resultStyle" style="margin-top:12px">
        <b>{{ resultTitle }}：</b>{{ result }}
      </div>
    </div>

    <!-- 快捷指令 -->
    <div class="card">
      <h3 style="margin-bottom:16px"><SvgIcon name="sliders" /> 快捷指令</h3>
      <div class="toolbar">
        <input v-model="cmdTarget" placeholder="目标 Agent ID" style="width:200px" />
        <button class="btn btn-primary" @click="sendCmd('lock')"><SvgIcon name="lock" /> 锁屏</button>
        <button class="btn btn-primary" @click="sendCmd('unlock')"><SvgIcon name="unlock" /> 解锁</button>
        <button class="btn btn-danger" @click="sendCmd('shutdown')">⏻ 关机</button>
        <button class="btn btn-primary" @click="sendCmd('reboot')"><SvgIcon name="refresh" /> 重启</button>
        <button class="btn btn-primary" @click="sendCmd('exam_enter')"><SvgIcon name="doc" /> 考试模式</button>
        <button class="btn btn-primary" @click="sendCmd('exam_exit')"><SvgIcon name="upload" /> 退出考试</button>
      </div>
      <div v-if="cmdResult" class="card" style="background:rgba(0,179,104,.13);border:1px solid rgba(0,179,104,.28);margin-top:8px">
        {{ cmdResult }}
      </div>
    </div>

    <!-- 广播任务记录 -->
    <div class="card">
      <h3 style="margin-bottom:16px"><SvgIcon name="clipboard" /> 广播任务记录</h3>
      <table>
        <thead>
          <tr><th>ID</th><th>机房</th><th>范围</th><th>组播地址</th><th>状态</th><th>开始时间</th><th>操作</th></tr>
        </thead>
        <tbody>
          <tr v-for="t in tasks" :key="t.id">
            <td>{{ t.id }}</td>
            <td>机房#{{ t.room_id }}</td>
            <td>{{ t.scope }}</td>
            <td><code>{{ t.mcast_addr }}</code></td>
            <td>
              <span class="tag" :class="t.status === '进行中' ? 'tag-on' : 'tag-off'">{{ t.status }}</span>
            </td>
            <td>{{ fmt(t.start_at) }}</td>
            <td>
              <button v-if="t.status === '进行中'" class="btn btn-danger btn-sm" @click="askStop(t.id)">停止</button>
              <span v-else style="color:var(--muted)">—</span>
            </td>
          </tr>
          <tr v-if="!tasks.length"><td colspan="7" style="text-align:center;color:var(--muted)">暂无广播记录</td></tr>
        </tbody>
      </table>
    </div>

    <!-- 停止广播二次确认 -->
    <ConfirmDialog
      v-model:visible="stopVisible"
      tone="danger"
      title="停止屏幕广播"
      message="确定要停止当前屏幕广播吗？所有正在接收广播的学生终端将立即恢复本机画面。"
      confirm-text="确认停止"
      @confirm="doStop"
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import api from '../api'
import ConfirmDialog from '../components/ConfirmDialog.vue'

// 状态
const rooms = ref([])
const tasks = ref([])
const activeStream = ref({})
const streaming = ref(false)
const result = ref('')
const resultOk = ref(true)
const cmdResult = ref('')
const starting = ref(false)   // 发起中（中间态）
const stopping = ref(false)   // 停止中（中间态）
const stopVisible = ref(false)
const stopTaskId = ref(null)

// 表单
const roomId = ref(1)
const source = ref('test')
const resolution = ref('1920x1080')
const fps = ref(25)
const bitrate = ref('4M')
const scope = ref('全班')
const targetInput = ref('RUST-AGENT-01')
const cmdTarget = ref('RUST-AGENT-01')

// 轮询定时器
let pollTimer = null

const resultTitle = computed(() => resultOk.value ? '成功' : '失败')
const targetCount = computed(() =>
  targetInput.value.split(',').map(s => s.trim()).filter(Boolean).length)
const resultStyle = computed(() => resultOk.value
  ? 'background:rgba(0,179,104,.13);border:1px solid rgba(0,179,104,.28)'
  : 'background:rgba(229,72,77,.10);border:1px solid rgba(229,72,77,.28)')

async function load() {
  try {
    const [r, t] = await Promise.all([api.get('/rooms'), api.get('/ctrl/broadcast')])
    rooms.value = r.data || []
    tasks.value = t.data || []
    if (rooms.value.length && !roomId.value) roomId.value = rooms.value[0].id
  } catch (e) { console.error(e) }
}

async function refreshStatus() {
  try {
    const res = await api.get('/ctrl/broadcast/status')
    const streams = res.data?.active_streams || []
    streaming.value = streams.length > 0
    activeStream.value = streams[0] || {}
  } catch (e) {
    streaming.value = false
  }
}

async function startBroadcast() {
  const [w, h] = resolution.value.split('x').map(Number)
  const targets = scope.value === '指定'
    ? targetInput.value.split(',').map(s => s.trim()).filter(Boolean)
    : []
  if (scope.value === '指定' && !targets.length) {
    resultOk.value = false
    result.value = '请填写至少一个目标 Agent ID'
    return
  }
  starting.value = true
  resultOk.value = true
  result.value = '正在发起广播，请稍候…'
  try {
    const res = await api.post('/ctrl/broadcast', {
      room_id: roomId.value,
      scope: scope.value,
      targets,
      source: source.value,
      width: w,
      height: h,
      fps: fps.value,
      bitrate: bitrate.value,
    })
    resultOk.value = true
    result.value = `广播已发起 → 组播 ${res.data.mcast_addr}，通知 ${res.data.notified} 台 Agent，源=${res.data.source}`
    streaming.value = true
    activeStream.value = { mcast_addr: res.data.mcast_addr, source: res.data.source }
    load()
  } catch (e) {
    resultOk.value = false
    result.value = e.response?.data?.msg || e.message
  } finally { starting.value = false }
}

function askStop(taskId) {
  const id = taskId || activeStream.value.task_id || tasks.value.find(t => t.status === '进行中')?.id
  if (!id) { result.value = '未找到活跃任务'; resultOk.value = false; return }
  stopTaskId.value = id
  stopVisible.value = true
}

function doStop() {
  stopVisible.value = false
  doStopTask(stopTaskId.value)
}

async function doStopTask(taskId) {
  stopping.value = true
  try {
    await api.post(`/ctrl/broadcast/${taskId}/stop`)
    resultOk.value = true
    result.value = `任务 #${taskId} 已停止，学生终端已恢复本机画面`
    streaming.value = false
    activeStream.value = {}
    load()
    refreshStatus()
  } catch (e) {
    resultOk.value = false
    result.value = e.response?.data?.msg || e.message
  } finally { stopping.value = false }
}

async function sendCmd(action) {
  try {
    const res = await api.post('/ctrl/command', {
      agent_ids: [cmdTarget.value],
      action,
      params: {}
    })
    cmdResult.value = `指令「${action}」已下发 → 送达 ${res.data.notified}/${res.data.total} 台`
  } catch (e) {
    cmdResult.value = `下发失败: ${e.response?.data?.msg || e.message}`
  }
}

function fmt(t) { return t ? new Date(t).toLocaleString('zh-CN') : '-' }

onMounted(() => {
  load()
  refreshStatus()
  pollTimer = setInterval(refreshStatus, 5000) // 5秒轮询推流状态
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>

<style scoped>
.status-bar {
  display: flex; align-items: center; gap: 8px;
  padding: 10px 16px; border-radius: 6px; margin-bottom: 16px;
  font-weight: 500;
}
.status-bar.live { background: rgba(0,179,104,.13); border: 1px solid rgba(0,179,104,.28); color: #00915b; }
.status-bar.idle { background: var(--card2); border: 1px solid var(--border2); color: var(--text2); }
.status-bar .dot {
  width: 10px; height: 10px; border-radius: 50%;
}
.status-bar.live .dot { background: var(--green); animation: pulse 1.5s infinite; }
.status-bar.idle .dot { background: var(--border2); }
@keyframes pulse { 0%,100% { opacity: 1; } 50% { opacity: 0.4; } }

.form-grid {
  display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px;
}
.form-grid label {
  display: flex; flex-direction: column; gap: 4px;
  font-size: 13px; color: var(--text2);
}
.form-grid select, .form-grid input {
  padding: 6px 10px; border: 1px solid var(--border2); border-radius: 4px; font-size: 14px;
}
.btn-sm { padding: 2px 8px; font-size: 12px; }
.tag-off { background: var(--bg2); color: var(--muted); }
</style>
