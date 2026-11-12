<template>
  <div class="card">
    <div class="toolbar">
      <button class="btn btn-primary" @click="openCreate">＋ 新建定时任务</button>
      <button class="btn btn-outline" :disabled="loading" @click="load"><SvgIcon name="refresh" /> 刷新</button>
      <span style="color:var(--text2);font-size:12px;margin-left:auto">到期任务由后端调度器每 10 秒扫描一次自动执行</span>
    </div>

    <ListState :state="state" empty-text="暂无定时电源任务" empty-hint="点击「新建定时任务」创建延时关机 / 重启 / 批量开机计划" error-text="任务列表加载失败" @retry="load">
      <table class="table">
        <thead>
          <tr>
            <th>ID</th><th>名称</th><th>动作</th><th>目标台数</th><th>执行时间</th>
            <th>状态</th><th>结果</th><th>创建人</th><th>创建时间</th><th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="t in tasks" :key="t.id">
            <td>{{ t.id }}</td>
            <td>{{ t.name }}</td>
            <td><span class="tag" :class="actionTag(t.action)">{{ actionLabel(t.action) }}</span></td>
            <td>{{ countAgents(t.agent_ids) }}</td>
            <td class="mono">{{ fmt(t.execute_at) }}</td>
            <td><span class="tag" :class="statusTag(t.status)">{{ statusLabel(t.status) }}</span></td>
            <td style="font-size:12px;color:var(--text2)">{{ t.detail || '-' }}</td>
            <td>{{ t.created_by }}</td>
            <td class="mono">{{ fmt(t.created_at) }}</td>
            <td>
              <button class="btn btn-xs btn-danger-outline" :disabled="t.status !== 'pending'" @click="remove(t)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </ListState>
  </div>

  <!-- 新建任务弹窗 -->
  <div v-if="showDlg" class="modal-mask" @click.self="showDlg = false">
    <div class="modal-card" style="width:560px;max-width:94vw">
      <h3>新建定时电源任务</h3>
      <div class="form-row"><label>任务名称</label><input class="input" v-model="form.name" placeholder="如：周三下午放学关机" /></div>
      <div class="form-row"><label>动作</label>
        <select class="input" v-model="form.action">
          <option value="shutdown">远程关机</option>
          <option value="reboot">远程重启</option>
          <option value="wol">批量开机 (WOL)</option>
        </select>
      </div>
      <div class="form-row"><label>执行时间</label><input class="input" type="datetime-local" v-model="form.execute_at" /></div>
      <div class="form-row"><label>目标机房</label>
        <select class="input" v-model="form.room_id" @change="pickRoom">
          <option value="">手动勾选 Agent…</option>
          <option v-for="r in rooms" :key="r.id" :value="r.id">{{ r.name }}</option>
        </select>
      </div>
      <div class="form-row"><label>目标 Agent（{{ form.agent_ids.length }} 台）</label>
        <div class="agent-box">
          <label v-for="s in sessions" :key="s.agent_id" class="agent-item">
            <input type="checkbox" :value="s.agent_id" v-model="form.agent_ids" />
            <span class="mono">{{ s.agent_id }}</span>
            <span class="on-dot" :class="s.online ? 'on' : 'off'"></span>
          </label>
          <div v-if="!sessions.length" class="empty-cell">暂无已登记 Agent</div>
        </div>
      </div>
      <p v-if="errMsg" class="err-msg">{{ errMsg }}</p>
      <div class="modal-foot">
        <button class="btn" @click="showDlg = false">取消</button>
        <button class="btn btn-primary" :disabled="saving" @click="submit">{{ saving ? '创建中…' : '创建任务' }}</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '../api'
import ListState from '../components/ListState.vue'

const tasks = ref([])
const state = ref('loading')
const loading = ref(false)
const rooms = ref([])
const sessions = ref([])

const showDlg = ref(false)
const saving = ref(false)
const errMsg = ref('')
const form = ref(emptyForm())

function emptyForm() {
  return { name: '', action: 'shutdown', execute_at: '', room_id: '', agent_ids: [] }
}

function fmt(t) { if (!t) return '-'; return String(t).replace('T', ' ').slice(0, 16) }
function countAgents(s) { try { return (JSON.parse(s) || []).length } catch { return 0 } }
function actionLabel(a) { return { shutdown: '远程关机', reboot: '远程重启', wol: '批量开机' }[a] || a }
function actionTag(a) { return a === 'wol' ? 'tag-green' : 'tag-red' }
function statusLabel(s) { return { pending: '待执行', done: '已完成', failed: '失败', cancelled: '已取消' }[s] || s }
function statusTag(s) { return { pending: 'tag-orange', done: 'tag-green', failed: 'tag-red', cancelled: 'tag-blue' }[s] || 'tag-blue' }

async function load() {
  loading.value = true
  try {
    const r = await api.get('/power-tasks')
    tasks.value = r.data || []
    state.value = tasks.value.length ? 'ready' : 'empty'
  } catch { if (!tasks.value.length) state.value = 'error' }
  finally { loading.value = false }
}

async function loadRooms() {
  try { const r = await api.get('/rooms'); rooms.value = r.data || [] } catch {}
  try {
    const r = await api.get('/agents/online')
    sessions.value = (r.data && r.data.sessions) || []
  } catch {}
}

function openCreate() { form.value = emptyForm(); errMsg.value = ''; showDlg.value = true }

function pickRoom() {
  if (!form.value.room_id) return
  // 该机房座位对应的在线 Agent 全选（简化：sessions 中带 room_id 匹配）
  const ids = sessions.value
    .filter(s => String(s.room_id || '') === String(form.value.room_id))
    .map(s => s.agent_id)
  if (ids.length) form.value.agent_ids = [...new Set([...form.value.agent_ids, ...ids])]
}

async function submit() {
  errMsg.value = ''
  if (!form.value.name.trim()) { errMsg.value = '请输入任务名称'; return }
  if (!form.value.execute_at) { errMsg.value = '请选择执行时间'; return }
  if (!form.value.agent_ids.length) { errMsg.value = '请至少选择一台目标 Agent'; return }
  saving.value = true
  try {
    const executeAt = form.value.execute_at.replace('T', ' ')
    const r = await api.post('/power-tasks', {
      name: form.value.name.trim(),
      action: form.value.action,
      agent_ids: form.value.agent_ids,
      execute_at: executeAt,
    })
    if (r.code === 0) { showDlg.value = false; load() }
    else errMsg.value = r.msg || '创建失败'
  } catch (e) {
    errMsg.value = e.response?.data?.msg || e.message || '创建失败'
  } finally { saving.value = false }
}

async function remove(t) {
  if (!window.confirm(`删除任务「${t.name}」？仅待执行任务可删除。`)) return
  try {
    const r = await api.delete(`/power-tasks/${t.id}`)
    if (r.code === 0) load()
    else alert(r.msg || '删除失败')
  } catch (e) { alert(e.response?.data?.msg || '删除失败') }
}

onMounted(() => { load(); loadRooms() })
</script>

<style scoped>
.toolbar { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.agent-box { max-height: 180px; overflow-y: auto; border: 1px solid var(--border); border-radius: 6px; padding: 8px; display: grid; grid-template-columns: 1fr 1fr; gap: 4px; }
.agent-item { display: flex; align-items: center; gap: 6px; font-size: 12px; cursor: pointer; padding: 2px 4px; border-radius: 4px; }
.agent-item:hover { background: rgba(0, 102, 255, .06); }
.agent-item .mono { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.on-dot { width: 7px; height: 7px; border-radius: 50%; flex: none; }
.on-dot.on { background: var(--green); }
.on-dot.off { background: var(--muted); }
.err-msg { color: var(--red); font-size: 12px; margin-top: 8px; }
.empty-cell { grid-column: 1 / -1; text-align: center; color: var(--muted); padding: 12px 0; }
.mono { font-family: ui-monospace, monospace; font-size: 12px; }
</style>
