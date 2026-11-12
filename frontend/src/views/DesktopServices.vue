<template>
  <div class="card">
    <div class="toolbar">
      <button class="btn btn-primary" @click="load" :disabled="state === 'loading'"><SvgIcon name="refresh" /> 刷新</button>
      <span style="color:var(--text2)">共 {{ services.length }} 项桌面服务</span>
      <div style="margin-left:auto">
        <button v-if="auth.isAdmin" class="btn btn-primary" @click="openEdit(null)">＋ 新增服务</button>
      </div>
    </div>

    <ListState :state="state" empty-text="暂无桌面服务" empty-hint="点击右上角「新增服务」添加应用或网址，即可一键推送到学生机打开" error-text="桌面服务加载失败" @retry="load">
      <table>
        <thead>
          <tr><th>名称</th><th>类型</th><th>目标（命令/网址）</th><th>说明</th><th>状态</th><th style="width:200px">操作</th></tr>
        </thead>
        <tbody>
          <tr v-for="s in services" :key="s.id" :style="s.status === 'disabled' ? 'opacity:.55' : ''">
            <td><b>{{ s.name }}</b></td>
            <td><span class="tag" :class="s.service_type === 'url' ? 'tag-blue' : 'tag-green'">{{ s.service_type === 'url' ? '网址' : '应用' }}</span></td>
            <td class="mono" style="max-width:340px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" :title="s.target">{{ s.target }}</td>
            <td style="color:var(--text2);font-size:12px">{{ s.description || '-' }}</td>
            <td><span class="tag" :class="s.status === 'active' ? 'tag-green' : 'tag-off'">{{ s.status === 'active' ? '启用' : '停用' }}</span></td>
            <td>
              <button class="btn btn-xs btn-primary" :disabled="s.status !== 'active'" @click="openLaunch(s)"><SvgIcon name="rocket" /> 推送启动</button>
              <button v-if="auth.isAdmin" class="btn btn-xs btn-outline" @click="openEdit(s)">编辑</button>
              <button v-if="auth.isAdmin" class="btn btn-xs btn-outline" @click="toggleStatus(s)">{{ s.status === 'active' ? '停用' : '启用' }}</button>
              <button v-if="auth.isAdmin" class="btn btn-xs btn-danger-outline" @click="del(s)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </ListState>
  </div>

  <!-- 编辑框 -->
  <div v-if="editDlg" class="modal-mask" @click.self="editDlg = null">
    <div class="modal" style="width:480px">
      <h3 class="card-title">{{ editDlg.id ? '编辑桌面服务' : '新增桌面服务' }}</h3>
      <div class="form-grid">
        <label>名称 *</label><input v-model="editDlg.name" placeholder="如：VS Code" />
        <label>类型 *</label>
        <select v-model="editDlg.service_type">
          <option value="app">应用（命令）</option>
          <option value="url">网址（xdg-open）</option>
        </select>
        <label>目标 *</label><input v-model="editDlg.target" :placeholder="editDlg.service_type === 'url' ? 'https://example.com' : 'code 或 firefox --new-window'" />
        <label>说明</label><input v-model="editDlg.description" placeholder="选填" />
      </div>
      <div v-if="editErr" class="err">{{ editErr }}</div>
      <div style="display:flex;gap:10px;justify-content:flex-end;margin-top:14px">
        <button class="btn btn-outline" @click="editDlg = null">取消</button>
        <button class="btn btn-primary" :disabled="saving" @click="saveEdit">{{ saving ? '保存中…' : '保存' }}</button>
      </div>
    </div>
  </div>

  <!-- 推送启动框 -->
  <div v-if="launchDlg" class="modal-mask" @click.self="launchDlg = null">
    <div class="modal" style="width:480px">
      <h3 class="card-title"><SvgIcon name="rocket" /> 推送启动 — {{ launchDlg.service.name }}</h3>
      <p style="color:var(--text2);font-size:13px">目标：<span class="mono">{{ launchDlg.service.target }}</span></p>
      <p style="font-size:13px;margin-top:8px">选择要启动的终端（可多选）：</p>
      <div style="max-height:220px;overflow-y:auto;border:1px solid var(--border2);border-radius:6px;padding:8px">
        <label v-for="a in agents" :key="a.agent_id" style="display:flex;gap:8px;align-items:center;padding:4px 2px;cursor:pointer;font-size:13px">
          <input type="checkbox" :value="a.agent_id" v-model="launchTargets" />
          <span class="mono">{{ a.agent_id }}</span>
          <span class="tag" :class="onlineIds.includes(a.agent_id) ? 'tag-on' : 'tag-off'" style="font-size:10px;padding:1px 5px">
            {{ onlineIds.includes(a.agent_id) ? '在线' : '离线' }}
          </span>
        </label>
        <div v-if="!agents.length" style="color:var(--muted);font-size:13px;padding:8px">暂无已注册 Agent</div>
      </div>
      <div v-if="launchErr" class="err">{{ launchErr }}</div>
      <div v-if="launchOk" style="color:var(--green);font-size:13px;margin-top:6px">{{ launchOk }}</div>
      <div style="display:flex;gap:10px;justify-content:flex-end;margin-top:14px">
        <button class="btn btn-outline" @click="launchDlg = null">关闭</button>
        <button class="btn btn-primary" :disabled="launching || !launchTargets.length" @click="doLaunch">
          {{ launching ? '下发中…' : `启动（${launchTargets.length} 台）` }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '../api'
import ListState from '../components/ListState.vue'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const state = ref('loading')
const services = ref([])
const agents = ref([])
const onlineIds = ref([])

async function load() {
  state.value = services.value.length ? 'ready' : 'loading'
  try {
    const r = await api.get('/desktop-services')
    services.value = r.data || []
    state.value = services.value.length ? 'ready' : 'empty'
  } catch { state.value = 'error' }
}

async function loadAgents() {
  try {
    const r = await api.get('/agents/online')
    const d = r.data || {}
    agents.value = d.sessions || []
    onlineIds.value = d.online_ids || []
  } catch { /* 忽略 */ }
}
onMounted(() => { load(); loadAgents() })

// ---- 编辑 ----
const editDlg = ref(null)
const editErr = ref('')
const saving = ref(false)
function openEdit(s) {
  editErr.value = ''
  editDlg.value = s
    ? { ...s }
    : { name: '', service_type: 'app', target: '', description: '', status: 'active' }
}
async function saveEdit() {
  const d = editDlg.value
  if (!d.name.trim() || !d.target.trim()) { editErr.value = '名称和目标必填'; return }
  saving.value = true
  try {
    if (d.id) {
      const res = await api.put(`/desktop-services/${d.id}`, { name: d.name, target: d.target, description: d.description, status: d.status })
      if (res.code !== 0) throw new Error(res.msg)
    } else {
      const res = await api.post('/desktop-services', { name: d.name, service_type: d.service_type, target: d.target, description: d.description })
      if (res.code !== 0) throw new Error(res.msg)
    }
    editDlg.value = null
    load()
  } catch (e) { editErr.value = e.response?.data?.msg || e.message || '保存失败' }
  finally { saving.value = false }
}
async function toggleStatus(s) {
  try {
    await api.put(`/desktop-services/${s.id}`, { status: s.status === 'active' ? 'disabled' : 'active' })
    load()
  } catch { /* ignore */ }
}
async function del(s) {
  if (!confirm(`确认删除桌面服务「${s.name}」？`)) return
  try { await api.delete(`/desktop-services/${s.id}`); load() } catch { /* ignore */ }
}

// ---- 推送启动 ----
const launchDlg = ref(null)
const launchTargets = ref([])
const launchErr = ref('')
const launchOk = ref('')
const launching = ref(false)
function openLaunch(s) {
  launchErr.value = ''
  launchOk.value = ''
  launchDlg.value = { service: s }
  launchTargets.value = [...onlineIds.value]
  if (!agents.value.length) loadAgents()
}
async function doLaunch() {
  launchErr.value = ''
  launching.value = true
  try {
    const res = await api.post(`/desktop-services/${launchDlg.value.service.id}/launch`, { agent_ids: launchTargets.value })
    if (res.code === 0) {
      const d = res.data || {}
      launchOk.value = `已下发 ${d.notified}/${d.total} 台，执行结果见 Agent 页面指令回执`
    } else {
      launchErr.value = res.msg || '下发失败'
    }
  } catch (e) {
    launchErr.value = e.response?.data?.msg || e.message || '下发失败'
  } finally { launching.value = false }
}
</script>

<style scoped>
.form-grid { display: grid; grid-template-columns: 80px 1fr; gap: 10px; align-items: center; margin-top: 10px; }
.form-grid label { font-size: 13px; color: var(--text2); }
.mono { font-family: monospace; font-size: 12px; }
</style>
