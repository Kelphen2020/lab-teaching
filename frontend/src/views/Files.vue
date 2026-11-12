<template>
  <div>
    <!-- 文件上传与分发 -->
    <div class="card">
      <h3 style="margin-bottom:16px"><SvgIcon name="folder" /> 文件分发</h3>

      <div class="form-grid">
        <label>目标机房
          <select v-model="roomId">
            <option v-for="r in rooms" :key="r.id" :value="r.id">{{ r.name }}</option>
          </select>
        </label>
        <label>选择文件
          <input type="file" ref="fileInput" @change="onFileSelect" />
        </label>
      </div>

      <div v-if="selectedFile" class="file-info">
        <SvgIcon name="doc" /> {{ selectedFile.name }}（{{ formatSize(selectedFile.size) }}）
      </div>
      <div v-if="fileWarn" class="file-warn"><SvgIcon name="alert" /> {{ fileWarn }}</div>

      <div class="toolbar" style="margin-top:12px">
        <button class="btn btn-success" :disabled="!selectedFile || uploading || !!fileWarn" @click="uploadAndDistribute">
          <SvgIcon name="upload" /> {{ uploading ? '上传中...' : '上传并分发' }}
        </button>
        <button class="btn btn-primary" :disabled="!uploadedTask" @click="distributeOnly">
          <SvgIcon name="upload" /> 仅分发（已上传）
        </button>
      </div>

      <!-- 上传/分发进度 -->
      <div v-if="uploadProgress > 0 && uploadProgress < 100" class="progress-bar">
        <div class="progress-fill" :style="{width: uploadProgress + '%'}"></div>
        <span class="progress-text">上传 {{ uploadProgress }}%</span>
      </div>

      <div v-if="result" class="card" :style="resultStyle" style="margin-top:12px">
        <SvgIcon :name="resultOk ? 'check-circle' : 'x-circle'" :size="16" /> {{ result }}
      </div>
    </div>

    <!-- 分发任务列表 -->
    <div class="card">
      <h3 style="margin-bottom:16px"><SvgIcon name="clipboard" /> 分发任务</h3>
      <table>
        <thead>
          <tr><th>ID</th><th>文件名</th><th>大小</th><th>方向</th><th>状态</th><th>进度</th><th>操作</th></tr>
        </thead>
        <tbody>
          <tr v-for="t in pagedTasks" :key="t.id">
            <td>{{ t.id }}</td>
            <td>{{ t.name }}</td>
            <td>{{ formatSize(t.size) }}</td>
            <td><span class="tag" :class="t.direction === 'distribute' ? 'tag-on' : 'tag-off'">{{ t.direction === 'distribute' ? '下发' : '收集' }}</span></td>
            <td><span class="tag" :class="statusClass(t.status)">{{ t.status }}</span></td>
            <td>{{ t.completed || 0 }}/{{ t.total || 0 }}</td>
            <td>
              <button class="btn btn-primary btn-sm" @click="viewProgress(t.id)">详情</button>
              <button v-if="canRetry(t)" class="btn btn-warning btn-sm" :disabled="retrying === t.id" @click="retryTask(t)">
                <SvgIcon name="refresh" /> {{ retrying === t.id ? '重试中…' : '重试' }}
              </button>
            </td>
          </tr>
          <tr v-if="!pagedTasks.length"><td colspan="7" style="text-align:center;color:var(--muted)">暂无任务</td></tr>
        </tbody>
      </table>
      <Pagination :page="page" :page-size="pageSize" :total="tasks.length" @update:page="p => page = p" />
    </div>

    <!-- 进度详情弹窗 -->
    <div v-if="progressDetail" class="card" style="border:2px solid var(--accent)">
      <h3 style="margin-bottom:12px"><SvgIcon name="chart" /> 任务 #{{ progressDetail.task.id }} 传输详情</h3>
      <table>
        <thead><tr><th>Agent</th><th>状态</th><th>进度</th><th>错误</th></tr></thead>
        <tbody>
          <tr v-for="p in progressDetail.progress" :key="p.id">
            <td>{{ p.agent_id }}</td>
            <td><span class="tag" :class="statusClass(p.status)">{{ p.status }}</span></td>
            <td>
              <div class="mini-progress">
                <div class="mini-fill" :style="{width: p.percent + '%'}"></div>
              </div>
              {{ p.percent }}%
            </td>
            <td style="color:var(--red);font-size:12px">{{ p.error || '—' }}</td>
          </tr>
        </tbody>
      </table>
      <button class="btn btn-primary btn-sm" style="margin-top:8px" @click="progressDetail = null">关闭</button>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import api from '../api'
import Pagination from '../components/Pagination.vue'

const MAX_SIZE = 2 * 1024 * 1024 * 1024 // 单文件上限 2GB
const BLOCK_EXT = ['.exe', '.bat', '.sh', '.ps1', '.vbs', '.cmd', '.dll', '.iso']

const rooms = ref([])
const tasks = ref([])
const roomId = ref(1)
const selectedFile = ref(null)
const uploadedTask = ref(null)
const uploading = ref(false)
const uploadProgress = ref(0)
const result = ref('')
const resultOk = ref(true)
const progressDetail = ref(null)
const fileInput = ref(null)
const retrying = ref(null)
const page = ref(1)
const pageSize = 10
let pollTimer = null

const fileWarn = computed(() => {
  const f = selectedFile.value
  if (!f) return ''
  if (f.size > MAX_SIZE) return `文件过大（${formatSize(f.size)}），单文件上限 ${formatSize(MAX_SIZE)}`
  const lower = f.name.toLowerCase()
  const hit = BLOCK_EXT.find(ext => lower.endsWith(ext))
  if (hit) return `不允许分发可执行类文件（${hit}）`
  return ''
})

const totalPages = computed(() => Math.max(1, Math.ceil(tasks.value.length / pageSize)))
const pagedTasks = computed(() => {
  if (page.value > totalPages.value) page.value = totalPages.value
  const s = (page.value - 1) * pageSize
  return tasks.value.slice(s, s + pageSize)
})

const resultStyle = computed(() => resultOk.value
  ? 'background:rgba(0,179,104,.13);border:1px solid rgba(0,179,104,.28)'
  : 'background:rgba(229,72,77,.10);border:1px solid rgba(229,72,77,.28)')

function onFileSelect(e) {
  selectedFile.value = e.target.files[0] || null
  uploadedTask.value = null
}

async function uploadAndDistribute() {
  if (!selectedFile.value) return
  uploading.value = true
  uploadProgress.value = 0
  result.value = ''

  try {
    // 上传文件
    const formData = new FormData()
    formData.append('file', selectedFile.value)
    formData.append('room_id', roomId.value)

    const uploadRes = await api.post('/files/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      onUploadProgress: (e) => {
        if (e.total) uploadProgress.value = Math.round((e.loaded * 100) / e.total)
      }
    })

    const taskId = uploadRes.data.task_id
    uploadedTask.value = taskId
    uploadProgress.value = 100

    // 立即分发
    const distRes = await api.post('/files/distribute', { task_id: taskId })
    resultOk.value = true
    result.value = `文件「${selectedFile.value.name}」已上传并分发 → 通知 ${distRes.data.notified}/${distRes.data.total} 台终端`
    loadTasks()
  } catch (e) {
    resultOk.value = false
    result.value = e.response?.data?.msg || e.message
  } finally {
    uploading.value = false
  }
}

async function distributeOnly() {
  if (!uploadedTask.value) return
  try {
    const res = await api.post('/files/distribute', { task_id: uploadedTask.value })
    resultOk.value = true
    result.value = `分发已发起 → 通知 ${res.data.notified}/${res.data.total} 台终端`
    loadTasks()
  } catch (e) {
    resultOk.value = false
    result.value = e.response?.data?.msg || e.message
  }
}

async function viewProgress(taskId) {
  try {
    const res = await api.get(`/files/tasks/${taskId}/progress`)
    progressDetail.value = res.data
  } catch (e) {
    result.value = '查询失败: ' + (e.response?.data?.msg || e.message)
    resultOk.value = false
  }
}

async function loadTasks() {
  try {
    const res = await api.get('/files/tasks')
    tasks.value = res.data || []
  } catch (e) { console.error(e) }
}

// 失败或部分失败的任务可重试
function canRetry(t) {
  return t.status === 'failed' || t.status === 'partial' ||
    (t.total > 0 && t.completed < t.total && !['distributing', 'transferring'].includes(t.status))
}

async function retryTask(t) {
  retrying.value = t.id
  try {
    const res = await api.post('/files/distribute', { task_id: t.id })
    resultOk.value = true
    result.value = `任务 #${t.id} 已重新分发 → 通知 ${res.data.notified}/${res.data.total} 台终端`
    loadTasks()
  } catch (e) {
    resultOk.value = false
    result.value = `重试失败: ${e.response?.data?.msg || e.message}`
  } finally { retrying.value = null }
}

async function load() {
  try {
    const res = await api.get('/rooms')
    rooms.value = res.data || []
    if (rooms.value.length) roomId.value = rooms.value[0].id
  } catch (e) { console.error(e) }
  loadTasks()
}

function formatSize(bytes) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let i = 0
  let size = bytes
  while (size >= 1024 && i < units.length - 1) { size /= 1024; i++ }
  return size.toFixed(i > 0 ? 1 : 0) + ' ' + units[i]
}

function statusClass(s) {
  if (s === 'done') return 'tag-on'
  if (s === 'failed') return 'tag-danger'
  if (s === 'distributing' || s === 'transferring') return 'tag-warn'
  return 'tag-off'
}

onMounted(() => {
  load()
  pollTimer = setInterval(() => {
    // 有进行中任务时轮询刷新列表与进度详情
    if (tasks.value.some(t => ['distributing', 'transferring', 'collecting'].includes(t.status))) {
      loadTasks()
      if (progressDetail.value?.task) viewProgress(progressDetail.value.task.id)
    }
  }, 5000)
})

onUnmounted(() => { if (pollTimer) clearInterval(pollTimer) })
</script>

<style scoped>
.form-grid {
  display: grid; grid-template-columns: repeat(2, 1fr); gap: 12px;
}
.form-grid label {
  display: flex; flex-direction: column; gap: 4px;
  font-size: 13px; color: var(--text2);
}
.form-grid select, .form-grid input {
  padding: 6px 10px; border: 1px solid var(--border2); border-radius: 4px; font-size: 14px;
}
.file-info {
  margin-top: 10px; padding: 8px 12px;
  background: var(--accent-glow); border: 1px solid #adc6ff; border-radius: 4px;
  font-size: 14px;
}
.file-warn {
  margin-top: 8px; padding: 8px 12px;
  background: rgba(245,158,11,.10); border: 1px solid rgba(245,158,11,.35);
  border-radius: 4px; font-size: 13px; color: #d48806;
}
.btn-warning { background: #f59e0b; border-color: #f59e0b; color: #fff; }
.btn-warning:hover:not(:disabled) { background: #d97706; }
.btn-warning:disabled { opacity: .5; cursor: not-allowed; }
.btn-warning.btn-sm { padding: 2px 8px; font-size: 12px; }
.progress-bar {
  position: relative; height: 24px; background: var(--border);
  border-radius: 4px; margin-top: 12px; overflow: hidden;
}
.progress-fill {
  height: 100%; background: linear-gradient(90deg, var(--accent), var(--green));
  transition: width 0.3s;
}
.progress-text {
  position: absolute; top: 3px; left: 50%; transform: translateX(-50%);
  font-size: 12px; font-weight: 500;
}
.mini-progress {
  display: inline-block; width: 60px; height: 8px;
  background: var(--border); border-radius: 4px; margin-right: 6px;
  vertical-align: middle; overflow: hidden;
}
.mini-fill { height: 100%; background: var(--accent); border-radius: 4px; }
.btn-sm { padding: 2px 8px; font-size: 12px; }
.tag-warn { background: rgba(245,158,11,.12); color: #d48806; }
.tag-danger { background: rgba(229,72,77,.10); color: var(--red); }
.tag-off { background: var(--bg2); color: var(--muted); }
</style>
