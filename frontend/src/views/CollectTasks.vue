<template>
  <div class="card">
    <div class="toolbar">
      <button class="btn btn-primary" @click="openCreate">＋ 发起作业收集</button>
      <button class="btn btn-outline" :disabled="loading" @click="load"><SvgIcon name="refresh" /> 刷新</button>
      <span class="poll-tip" :class="connected ? 'live' : ''">
        <span class="dot"></span>{{ connected ? '实时推送已连接（文件到达自动刷新）' : '实时推送未连接' }}
      </span>
    </div>

    <ListState :state="state" empty-text="暂无收集任务" empty-hint="点击「发起作业收集」，学生机上的指定文件将被打包上传到服务器" error-text="任务列表加载失败" @retry="load">
      <table class="table">
        <thead>
          <tr><th>ID</th><th>标题</th><th>文件模式</th><th>目标台数</th><th>已收集</th><th>状态</th><th>创建人</th><th>创建时间</th><th>操作</th></tr>
        </thead>
        <tbody>
          <tr v-for="t in tasks" :key="t.id">
            <td>{{ t.id }}</td>
            <td>{{ t.title }}</td>
            <td class="mono">{{ t.path_pattern }}</td>
            <td>{{ countAgents(t.agent_ids) }}</td>
            <td><b :style="{ color: t.collected > 0 ? 'var(--green)' : 'var(--text2)' }">{{ t.collected }}</b></td>
            <td><span class="tag" :class="t.status === 'done' ? 'tag-green' : 'tag-orange'">{{ t.status === 'done' ? '已完成' : '收集中' }}</span></td>
            <td>{{ t.created_by }}</td>
            <td class="mono">{{ fmt(t.created_at) }}</td>
            <td>
              <button class="btn btn-xs btn-outline" @click="openDetail(t)">查看文件</button>
            </td>
          </tr>
        </tbody>
      </table>
    </ListState>
  </div>

  <!-- 发起收集弹窗 -->
  <div v-if="showDlg" class="modal-mask" @click.self="showDlg = false">
    <div class="modal-card" style="width:560px;max-width:94vw">
      <h3>发起作业收集</h3>
      <div class="form-row"><label>任务标题</label><input class="input" v-model="form.title" placeholder="如：Python 作业第 3 周" /></div>
      <div class="form-row"><label>目标机房</label>
        <select class="input" v-model="form.room_id">
          <option value="">选择机房…</option>
          <option v-for="r in rooms" :key="r.id" :value="r.id">{{ r.name }}</option>
        </select>
      </div>
      <div class="form-row"><label>文件模式</label><input class="input" v-model="form.pattern" placeholder="~/Desktop/作业*.docx" /></div>
      <p class="hint">支持通配符 * 和 ?，~ 展开为学生主目录。例：<span class="mono">~/Desktop/*.docx</span> 收集所有学生桌面上的 Word 文档。</p>
      <p v-if="errMsg" class="err-msg">{{ errMsg }}</p>
      <div class="modal-foot">
        <button class="btn" @click="showDlg = false">取消</button>
        <button class="btn btn-primary" :disabled="saving" @click="submit">{{ saving ? '下发中…' : '发起收集' }}</button>
      </div>
    </div>
  </div>

  <!-- 文件列表抽屉 -->
  <div v-if="detail" class="drawer-mask" @click.self="detail = null">
    <div class="drawer">
      <div class="drawer-head"><h3>收集文件 — {{ detail.title }}</h3><button class="btn btn-sm" @click="detail = null"><SvgIcon name="x" /> </button></div>
      <div class="drawer-body">
        <div v-if="!detailFiles.length" style="color:var(--muted);text-align:center;padding:24px 0">尚未收到文件</div>
        <div v-for="f in detailFiles" :key="f.id" class="file-row">
          <div class="file-info">
            <span class="file-name">{{ f.file_name }}</span>
            <span class="file-meta mono">{{ f.agent_id }} · {{ fmtSize(f.file_size) }} · {{ fmt(f.created_at) }}</span>
          </div>
          <a class="btn btn-xs btn-outline" :href="downloadUrl(f.id)" target="_blank">下载</a>
        </div>
      </div>
      <div class="drawer-foot">
        <button class="btn" @click="loadDetail(detail)"><SvgIcon name="refresh" /> 刷新</button>
        <button class="btn btn-primary" @click="detail = null">关闭</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '../api'
import ListState from '../components/ListState.vue'
import { useEventStream } from '../composables/useEventStream'

const tasks = ref([])
const state = ref('loading')
const loading = ref(false)
const rooms = ref([])

const showDlg = ref(false)
const saving = ref(false)
const errMsg = ref('')
const form = ref({ title: '', room_id: '', pattern: '' })

const detail = ref(null)
const detailFiles = ref([])

const { connected, on } = useEventStream()
on('collect_file', p => {
  if (!p) return
  // 新文件到达：对应任务计数 +1，若正在查看该任务则追加
  const t = tasks.value.find(x => x.id === p.task_id)
  if (t) t.collected = (t.collected || 0) + 1
  if (detail.value && detail.value.id === p.task_id) {
    detailFiles.value.unshift({ id: -Date.now(), file_name: p.file_name, agent_id: p.agent_id, file_size: p.size, created_at: new Date().toISOString() })
  }
})

function fmt(t) { if (!t) return '-'; return String(t).replace('T', ' ').slice(0, 16) }
function fmtSize(n) {
  if (!n) return '-'
  if (n < 1024) return n + ' B'
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB'
  return (n / 1024 / 1024).toFixed(1) + ' MB'
}
function countAgents(s) { try { return (JSON.parse(s) || []).length } catch { return 0 } }
function downloadUrl(fid) { return `/api/v1/collect/download/${fid}` }

async function load() {
  loading.value = true
  try {
    const r = await api.get('/collect/tasks')
    tasks.value = r.data || []
    state.value = tasks.value.length ? 'ready' : 'empty'
  } catch { if (!tasks.value.length) state.value = 'error' }
  finally { loading.value = false }
}

async function loadRooms() {
  try { const r = await api.get('/rooms'); rooms.value = r.data || [] } catch {}
}

function openCreate() {
  form.value = { title: '', room_id: rooms.value.length ? rooms.value[0].id : '', pattern: '' }
  errMsg.value = ''
  showDlg.value = true
}

async function submit() {
  errMsg.value = ''
  if (!form.value.title.trim()) { errMsg.value = '请输入任务标题'; return }
  if (!form.value.room_id) { errMsg.value = '请选择目标机房'; return }
  if (!form.value.pattern.trim()) { errMsg.value = '请输入文件模式'; return }
  saving.value = true
  try {
    const r = await api.post('/collect/tasks', {
      title: form.value.title.trim(),
      room_id: Number(form.value.room_id),
      path_pattern: form.value.pattern.trim(),
    })
    if (r.code === 0) { showDlg.value = false; load() }
    else errMsg.value = r.msg || '下发失败'
  } catch (e) {
    errMsg.value = e.response?.data?.msg || e.message || '下发失败'
  } finally { saving.value = false }
}

async function openDetail(t) {
  detail.value = t
  detailFiles.value = []
  await loadDetail(t)
}

async function loadDetail(t) {
  try {
    const r = await api.get(`/collect/tasks/${t.id}`)
    detailFiles.value = (r.data && r.data.files) || []
    if (r.data && r.data.task) {
      const idx = tasks.value.findIndex(x => x.id === t.id)
      if (idx >= 0) tasks.value[idx].collected = r.data.task.collected
    }
  } catch { detailFiles.value = [] }
}

onMounted(() => { load(); loadRooms() })
</script>

<style scoped>
.toolbar { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.hint { color: var(--text2); font-size: 12px; margin-top: 4px; }
.err-msg { color: var(--red); font-size: 12px; margin-top: 8px; }
.file-row { display: flex; align-items: center; gap: 10px; padding: 10px 0; border-bottom: 1px dashed var(--border); }
.file-info { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
.file-name { font-size: 13px; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.file-meta { font-size: 11px; color: var(--text2); }
.drawer-mask { position: fixed; inset: 0; z-index: 180; background: rgba(15, 25, 50, .35); }
.drawer { position: absolute; top: 0; right: 0; height: 100%; width: 460px; max-width: 92vw; background: var(--card); box-shadow: -8px 0 32px rgba(20, 40, 90, .18); display: flex; flex-direction: column; }
.drawer-head { display: flex; justify-content: space-between; align-items: center; padding: 16px 20px; border-bottom: 1px solid var(--border); }
.drawer-head h3 { font-size: 15px; }
.drawer-body { flex: 1; overflow-y: auto; padding: 12px 20px; }
.drawer-foot { padding: 14px 20px; border-top: 1px solid var(--border); display: flex; justify-content: flex-end; gap: 8px; }
.mono { font-family: ui-monospace, monospace; font-size: 12px; }
</style>
