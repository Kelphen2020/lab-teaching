<template>
  <div>
    <div class="card">
      <h3 style="margin-top:0"><SvgIcon name="rocket" /> 升级任务</h3>
      <div class="toolbar">
        <button class="btn btn-primary" @click="openCreate">＋ 新建升级任务</button>
        <button class="btn" @click="load"><SvgIcon name="refresh" /> 刷新</button>
      </div>
      <table>
        <thead><tr><th>ID</th><th>名称</th><th>目标版本</th><th>架构</th><th>范围</th><th>灰度</th><th>进度</th><th>状态</th><th style="width:230px">操作</th></tr></thead>
        <tbody>
          <tr v-for="t in tasks" :key="t.id">
            <td>{{ t.id }}</td>
            <td>{{ t.name }}</td>
            <td><code>{{ t.version }}</code></td>
            <td>{{ t.arch }}</td>
            <td>{{ t.room_ids ? ('机房 ' + t.room_ids) : '全部机房' }}</td>
            <td>{{ t.gray_pct ? t.gray_pct + '%' : '全量' }}</td>
            <td>
              <div class="progress"><div class="progress-bar" :style="{ width: pct(t) + '%' }"></div></div>
              <span style="font-size:12px;color:var(--text2)">{{ t.succeeded + t.failed }}/{{ t.total }}（成功 {{ t.succeeded }} 失败 {{ t.failed }}）</span>
            </td>
            <td><span :class="['tag', statusClass(t.status)]">{{ statusName(t.status) }}</span></td>
            <td>
              <button class="btn btn-sm" @click="viewTask(t)"><SvgIcon name="clipboard" /> 节点</button>
              <button v-if="t.status === 'pending'" class="btn btn-sm btn-primary" @click="askStart(t)"><SvgIcon name="play" /> 启动</button>
              <button v-if="['rolling', 'failed'].includes(t.status)" class="btn btn-sm btn-danger" @click="rollback(t)"><SvgIcon name="undo" /> 回滚</button>
            </td>
          </tr>
          <tr v-if="!tasks.length"><td colspan="9" style="text-align:center;color:var(--muted)">暂无升级任务</td></tr>
        </tbody>
      </table>
    </div>

    <!-- 启动中任务的实时进度看板 -->
    <div v-if="liveTask" class="card live-card">
      <h3 style="margin-top:0"><SvgIcon name="signal" /> 任务 #{{ liveTask.id }}「{{ liveTask.name }}」实时进度</h3>
      <div class="stages">
        <div v-for="(s, i) in stages" :key="i" class="stage" :class="{ done: stageIdx > i, active: stageIdx === i }">
          <div class="stage-dot"><SvgIcon v-if="stageIdx > i" name="check" :size="12" /><template v-else>{{ i + 1 }}</template></div>
          <div class="stage-name">{{ s }}</div>
        </div>
      </div>
      <div class="progress live-progress"><div class="progress-bar" :style="{ width: pct(liveTask) + '%' }"></div></div>
      <div class="live-info">
        <span>状态：<b :class="statusClass(liveTask.status)">{{ statusName(liveTask.status) }}</b></span>
        <span>已完成 {{ liveTask.succeeded + liveTask.failed }}/{{ liveTask.total }}（成功 {{ liveTask.succeeded }} 失败 {{ liveTask.failed }}）</span>
        <span v-if="liveTask.gray_pct">灰度比例 {{ liveTask.gray_pct }}%</span>
      </div>
      <button class="btn btn-sm" @click="stopLive"><SvgIcon name="x" /> 关闭看板</button>
    </div>

    <!-- 新建任务弹窗 -->
    <div v-if="showCreate" class="modal-mask" @click.self="showCreate = false">
      <div class="modal">
        <h3 style="margin-top:0">新建升级任务</h3>
        <div class="form-row"><label>任务名称 *</label><input class="input" v-model="form.name" placeholder="如 Agent 1.1.0 升级"></div>
        <div class="form-row"><label>目标版本 *</label><input class="input" v-model="form.version" placeholder="如 1.1.0"></div>
        <div class="form-row"><label>架构</label>
          <select class="input" v-model="form.arch">
            <option value="amd64">amd64（x86_64）</option>
            <option value="arm64">arm64（飞腾/鲲鹏）</option>
            <option value="loong64">loong64（龙芯）</option>
          </select>
        </div>
        <div class="form-row"><label>机房范围</label>
          <select class="input" v-model="form.room_ids">
            <option value="">全部机房</option>
            <option v-for="r in rooms" :key="r.id" :value="String(r.id)">{{ r.name }}</option>
          </select>
        </div>
        <div class="form-row"><label>灰度比例</label>
          <select class="input" v-model.number="form.gray_pct">
            <option :value="0">全量发布</option>
            <option :value="10">10% 先行</option>
            <option :value="30">30%</option>
            <option :value="50">50%</option>
          </select>
        </div>
        <div class="form-row"><label>备注</label><input class="input" v-model="form.remark"></div>
        <p v-if="errMsg" class="error-msg">{{ errMsg }}</p>
        <div class="modal-actions">
          <button class="btn" @click="showCreate = false">取消</button>
          <button class="btn btn-primary" @click="create">创建</button>
        </div>
      </div>
    </div>

    <!-- 启动前密码二次验证弹窗 -->
    <div v-if="showVerify" class="modal-mask" @click.self="showVerify = false">
      <div class="modal" style="width:420px">
        <h3 style="margin-top:0"><SvgIcon name="shield" /> 身份验证</h3>
        <p class="verify-tip">启动升级任务「<b>{{ pendingTask?.name }}</b>」将向 <b>{{ pendingTask?.total }}</b> 台终端下发升级指令，请输入当前账号密码确认身份。</p>
        <div class="form-row">
          <input class="input" style="width:100%" type="password" v-model="verifyPwd" placeholder="请输入当前账号密码" @keyup.enter="confirmStart" autocomplete="current-password">
        </div>
        <p v-if="errMsg" class="error-msg">{{ errMsg }}</p>
        <div class="modal-actions">
          <button class="btn" @click="showVerify = false">取消</button>
          <button class="btn btn-primary" :disabled="verifying" @click="confirmStart">{{ verifying ? '验证中…' : '验证并启动' }}</button>
        </div>
      </div>
    </div>

    <!-- 节点明细弹窗 -->
    <div v-if="showNodes" class="modal-mask" @click.self="showNodes = false">
      <div class="modal" style="width:720px">
        <h3 style="margin-top:0"><SvgIcon name="clipboard" /> 任务 #{{ nodeTask?.id }} 节点明细（{{ nodes.length }} 台）</h3>
        <div style="max-height:400px;overflow:auto">
          <table>
            <thead><tr><th>节点ID</th><th>座位ID</th><th>Agent</th><th>状态</th><th>新版本</th><th>错误信息</th></tr></thead>
            <tbody>
              <tr v-for="n in nodes" :key="n.id">
                <td>{{ n.id }}</td>
                <td>{{ n.seat_id }}</td>
                <td>{{ n.agent_id }}</td>
                <td><span :class="['tag', nodeStatusClass(n.status)]">{{ nodeStatusName(n.status) }}</span></td>
                <td>{{ n.new_ver || '-' }}</td>
                <td style="color:var(--red)">{{ n.error || '-' }}</td>
              </tr>
              <tr v-if="!nodes.length"><td colspan="6" style="text-align:center;color:var(--muted)">任务尚未启动，暂无节点记录</td></tr>
            </tbody>
          </table>
        </div>
        <div class="modal-actions"><button class="btn" @click="showNodes = false">关闭</button></div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import api from '../api'

const tasks = ref([])
const rooms = ref([])
const showCreate = ref(false)
const showNodes = ref(false)
const showVerify = ref(false)
const errMsg = ref('')
const form = ref({ name: '', version: '', arch: 'amd64', room_ids: '', gray_pct: 0, remark: '' })
const nodeTask = ref(null)
const nodes = ref([])

// 密码二次验证
const pendingTask = ref(null)
const verifyPwd = ref('')
const verifying = ref(false)

// 实时进度看板
const liveTask = ref(null)
const stages = ['任务下发', 'Agent 升级', '版本回报', '完成校验']
let liveTimer = null

const stageIdx = computed(() => {
  const t = liveTask.value
  if (!t) return 0
  if (t.status === 'pending') return 0
  if (t.status === 'rolling') {
    if (!t.total) return 1
    if (t.succeeded + t.failed < t.total) return 2
    return 3
  }
  return 4 // success / failed / rolled_back 都视为走完
})

async function load() {
  const [ts, rs] = await Promise.all([api.get('/ops/upgrade'), api.get('/rooms')])
  tasks.value = ts.data || []
  rooms.value = rs.data || []
}

function openCreate() {
  form.value = { name: '', version: '', arch: 'amd64', room_ids: '', gray_pct: 0, remark: '' }
  errMsg.value = ''
  showCreate.value = true
}

async function create() {
  if (!form.value.name || !form.value.version) { errMsg.value = '任务名称与目标版本必填'; return }
  errMsg.value = ''
  try {
    const res = await api.post('/ops/upgrade', form.value)
    showCreate.value = false
    errMsg.value = `创建成功，目标终端 ${res.data.target_count} 台`
    setTimeout(() => { errMsg.value = '' }, 3000)
    load()
  } catch (e) { errMsg.value = e.response?.data?.msg || '创建失败' }
}

function askStart(t) {
  pendingTask.value = t
  verifyPwd.value = ''
  errMsg.value = ''
  showVerify.value = true
}

async function confirmStart() {
  if (!verifyPwd.value) { errMsg.value = '请输入密码'; return }
  verifying.value = true
  errMsg.value = ''
  try {
    // 第一步：密码二次验证
    await api.post('/ops/verify-password', { password: verifyPwd.value })
    // 第二步：启动任务
    await api.post(`/ops/upgrade/${pendingTask.value.id}/start`)
    showVerify.value = false
    // 第三步：打开实时进度看板并轮询
    openLive(pendingTask.value)
    load()
  } catch (e) {
    const msg = e.response?.data?.msg || '操作失败'
    errMsg.value = e.response?.status === 401 || /密码/.test(msg) ? '密码验证失败：' + msg : msg
  } finally {
    verifying.value = false
  }
}

function openLive(t) {
  stopLive()
  liveTask.value = { ...t }
  liveTimer = setInterval(async () => {
    try {
      const res = await api.get(`/ops/upgrade/${t.id}`)
      liveTask.value = res.data.task || res.data
      // 任务结束后停止轮询，但保留看板
      if (!['rolling', 'pending'].includes(liveTask.value.status)) {
        clearInterval(liveTimer)
        liveTimer = null
        load()
      }
    } catch (e) { /* 网络抖动忽略，下个周期重试 */ }
  }, 3000)
}

function stopLive() {
  if (liveTimer) { clearInterval(liveTimer); liveTimer = null }
  liveTask.value = null
}

async function rollback(t) {
  if (!confirm(`回滚任务「${t.name}」？已升级/升级中的节点将标记回滚`)) return
  try {
    await api.post(`/ops/upgrade/${t.id}/rollback`)
    load()
    if (liveTask.value?.id === t.id) liveTask.value = { ...liveTask.value, status: 'rolled_back' }
  } catch (e) { alert(e.response?.data?.msg || '回滚失败') }
}

async function viewTask(t) {
  const res = await api.get(`/ops/upgrade/${t.id}`)
  nodeTask.value = res.data.task
  nodes.value = res.data.nodes || []
  showNodes.value = true
}

function pct(t) {
  if (!t.total) return 0
  return Math.round(((t.succeeded + t.failed) / t.total) * 100)
}

function statusName(s) {
  return { pending: '待启动', rolling: '发布中', success: '已完成', failed: '有失败', rolled_back: '已回滚' }[s] || s
}
function statusClass(s) {
  return { pending: '', rolling: 'tag-warn', success: 'tag-on', failed: 'tag-off', rolled_back: 'tag-off' }[s] || ''
}
function nodeStatusName(s) {
  return { pending: '待升级', upgrading: '升级中', success: '成功', failed: '失败', rolled_back: '已回滚' }[s] || s
}
function nodeStatusClass(s) {
  return { pending: '', upgrading: 'tag-warn', success: 'tag-on', failed: 'tag-off', rolled_back: 'tag-off' }[s] || ''
}

onMounted(load)
onUnmounted(() => { if (liveTimer) clearInterval(liveTimer) })
</script>

<style scoped>
.modal-mask { position: fixed; inset: 0; background: rgba(0,0,0,.45); display: flex; align-items: center; justify-content: center; z-index: 100; }
.modal { background: var(--card); border-radius: 8px; padding: 24px; width: 480px; box-shadow: 0 8px 32px rgba(0,0,0,.2); }
.form-row { margin-bottom: 12px; }
.form-row label { display: inline-block; width: 90px; color: var(--text2); }
.input { border: 1px solid var(--border2); border-radius: 4px; padding: 6px 8px; }
.modal-actions { text-align: right; margin-top: 16px; }
.error-msg { color: var(--red); font-size: 13px; margin: 8px 0 0; }
.verify-tip { font-size: 14px; color: var(--text2); line-height: 1.6; }
code { background: var(--bg2); padding: 1px 6px; border-radius: 3px; }
.progress { width: 120px; height: 8px; background: var(--border); border-radius: 4px; overflow: hidden; display: inline-block; vertical-align: middle; margin-right: 8px; }
.progress-bar { height: 100%; background: #1677ff; transition: width .4s ease; }
.tag-warn { background: rgba(245,158,11,.12); color: var(--orange); }

.live-card { margin-top: 16px; border-left: 3px solid var(--accent); }
.stages { display: flex; gap: 0; margin: 16px 0; }
.stage { flex: 1; text-align: center; position: relative; }
.stage::before {
  content: ''; position: absolute; top: 14px; left: -50%; width: 100%; height: 2px;
  background: var(--border); z-index: 0;
}
.stage:first-child::before { display: none; }
.stage.done::before { background: var(--green, #00b368); }
.stage.active::before { background: var(--accent); }
.stage-dot {
  width: 28px; height: 28px; border-radius: 50%; margin: 0 auto 6px;
  background: var(--bg2); border: 2px solid var(--border); color: var(--muted);
  display: flex; align-items: center; justify-content: center; font-size: 13px; font-weight: 600;
  position: relative; z-index: 1;
}
.stage.done .stage-dot { background: var(--green, #00b368); border-color: var(--green, #00b368); color: #fff; }
.stage.active .stage-dot { background: var(--accent); border-color: var(--accent); color: #fff; animation: pulse 1.5s infinite; }
.stage-name { font-size: 12px; color: var(--text2); }
.stage.done .stage-name, .stage.active .stage-name { color: var(--text); font-weight: 500; }
@keyframes pulse { 0%,100% { box-shadow: 0 0 0 0 rgba(22,119,255,.4); } 50% { box-shadow: 0 0 0 6px rgba(22,119,255,0); } }
.live-progress { width: 100%; display: block; margin: 8px 0; }
.live-info { display: flex; gap: 24px; font-size: 13px; color: var(--text2); margin-bottom: 12px; }
</style>
