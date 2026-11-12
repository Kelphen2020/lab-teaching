<template>
  <div class="card">
    <div class="toolbar">
      <button class="btn btn-primary" :disabled="refreshing" @click="load(true)">
        <SvgIcon name="refresh" /> {{ refreshing ? '刷新中…' : '刷新' }}
      </button>
      <span class="poll-tip" :class="connected ? 'live' : { paused: !autoPoll }">
        <span class="dot"></span>
        {{ connected ? '实时推送已连接' : autoPoll ? `自动刷新：每 15s（${countdown}s 后）` : '自动刷新已暂停' }}
      </span>
      <span style="color:var(--text2)">实时在线：<b style="color:var(--green)">{{ data.online_count }}</b> / {{ sessions.length }} 台</span>
      <span v-if="lastUpdated" style="color:var(--muted);font-size:12px">更新于 {{ lastUpdated }}</span>

      <div style="margin-left:auto;display:flex;gap:8px">
        <select v-model="statusFilter" style="padding:6px 10px;border:1px solid var(--border2);border-radius:6px">
          <option value="all">全部状态</option>
          <option value="online">仅在线</option>
          <option value="offline">仅离线</option>
        </select>
        <input v-model="keyword" placeholder="搜索 ID / MAC / 系统" style="width:200px" />
      </div>
    </div>

    <!-- P0-1 批量控制栏 -->
    <div v-if="selected.length" class="batch-bar">
      <span>已选 <b>{{ selected.length }}</b> 台：</span>
      <button class="btn btn-sm btn-outline" @click="confirmAction('lock')"><SvgIcon name="lock" /> 锁定屏幕</button>
      <button class="btn btn-sm btn-outline" @click="confirmAction('unlock')"><SvgIcon name="unlock" /> 解除锁定</button>
      <button class="btn btn-sm btn-danger-outline" @click="confirmAction('shutdown')">⏻ 关机</button>
      <button class="btn btn-sm btn-danger-outline" @click="confirmAction('reboot')"><SvgIcon name="power" /> 重启</button>
      <button class="btn btn-sm btn-outline" @click="openExec()"><SvgIcon name="play" /> 执行命令</button>
      <button class="btn btn-sm btn-outline" @click="openMsgDlg()"><SvgIcon name="chat" /> 发送消息</button>
      <button class="btn btn-sm btn-outline" @click="confirmAction('snapshot')"><SvgIcon name="camera" /> 屏幕截图</button>
      <button class="btn btn-sm btn-outline" @click="doWol()"><SvgIcon name="plug" /> 远程开机</button>
      <button class="btn btn-sm btn-danger-outline" @click="confirmAction('session_logoff')"><SvgIcon name="door" /> 注销会话</button>
      <button class="btn btn-sm btn-ghost" @click="selected = []">取消选择</button>
    </div>

    <ListState :state="state" empty-text="暂无 Agent 注册记录" empty-hint="终端启动 Agent 后将自动注册到此处" error-text="Agent 状态加载失败" @retry="load">
      <table>
        <thead>
          <tr>
            <th style="width:34px"><input type="checkbox" :checked="allChecked" @change="toggleAll" /></th>
            <th>Agent ID</th><th>操作系统</th><th>架构</th><th>MAC</th><th>启动模式</th><th>版本</th><th>实时在线</th><th>最后心跳</th><th style="width:190px">控制 / 操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!paged.length"><td colspan="10" class="empty-cell">没有匹配的 Agent</td></tr>
          <tr v-for="s in paged" :key="s.id">
            <td><input type="checkbox" :checked="selected.includes(s.agent_id)" @change="toggleOne(s.agent_id)" /></td>
            <td class="clickable" @click="openDetail(s)">{{ s.agent_id }}</td>
            <td>{{ s.os || '-' }}</td>
            <td>{{ s.arch || '-' }}</td>
            <td class="mono">{{ s.mac || '-' }}</td>
            <td>{{ s.boot_mode || '-' }}</td>
            <td>{{ s.agent_version || '-' }}</td>
            <td>
              <span class="tag" :class="onlineIds.includes(s.agent_id) ? 'tag-on' : 'tag-off'">
                {{ onlineIds.includes(s.agent_id) ? '在线' : '离线' }}
              </span>
            </td>
            <td class="mono">{{ fmt(s.last_heartbeat) }}</td>
            <td class="ctrl-cell">
              <button class="btn btn-xs btn-outline" title="锁定屏幕" @click="confirmAction('lock', s)"><SvgIcon name="lock" /> </button>
              <button class="btn btn-xs btn-outline" title="解除锁定" @click="confirmAction('unlock', s)"><SvgIcon name="unlock" /> </button>
              <button class="btn btn-xs btn-danger-outline" title="关机" @click="confirmAction('shutdown', s)">⏻</button>
              <button class="btn btn-xs btn-danger-outline" title="重启" @click="confirmAction('reboot', s)"><SvgIcon name="power" /> </button>
              <button class="btn btn-xs btn-outline" title="执行命令" @click="openExec(s)"><SvgIcon name="play" /> </button>
              <button class="btn btn-xs btn-outline" title="发送消息" @click="openMsgDlg(s)"><SvgIcon name="chat" /> </button>
              <button class="btn btn-xs btn-outline" title="屏幕截图" @click="confirmAction('snapshot', s)"><SvgIcon name="camera" /> </button>
              <button class="btn btn-xs btn-outline" title="远程开机" :disabled="onlineIds.includes(s.agent_id)" @click="doWol(s)"><SvgIcon name="plug" /> </button>
              <button class="btn btn-xs btn-danger-outline" title="注销会话" @click="confirmAction('session_logoff', s)"><SvgIcon name="door" /> </button>
              <button class="btn btn-xs" @click="openDetail(s)">详情</button>
            </td>
          </tr>
        </tbody>
      </table>
    </ListState>

    <Pagination v-if="filtered.length > pageSize" :page="page" :page-size="pageSize" :total="filtered.length" @change="p => page = p" />

    <!-- P0-1 指令执行结果（实时） -->
    <details class="log-section" style="margin-top:16px" :open="results.length > 0">
      <summary style="cursor:pointer;font-size:13px;font-weight:600;color:var(--text2)"><SvgIcon name="target" /> 指令执行结果（{{ results.length }} 条，实时推送）</summary>
      <div v-if="results.length" style="margin-top:8px;max-height:320px;overflow-y:auto;font-size:12px;font-family:monospace;background:var(--bg);border:1px solid var(--border2);border-radius:6px;padding:8px">
        <div v-for="r in results" :key="r.msg_id" style="padding:5px 0;border-bottom:1px solid var(--border);display:flex;gap:8px;align-items:baseline">
          <span style="color:var(--muted);white-space:nowrap">{{ r.time }}</span>
          <span style="color:var(--accent);white-space:nowrap">{{ r.agent_id }}</span>
          <span class="tag" style="font-size:10px;padding:1px 5px">{{ r.action }}</span>
          <span class="tag" :class="statusClass(r.status)" style="font-size:10px;padding:1px 5px">{{ statusText(r.status) }}</span>
          <span style="word-break:break-all;white-space:pre-wrap">{{ r.detail || '—' }}</span>
        </div>
      </div>
      <div v-else style="margin-top:8px;color:var(--muted);font-size:13px">暂无指令结果。下发控制指令后，Agent 执行结果将实时显示在此处。</div>
    </details>

    <!-- Agent 日志 -->
    <details class="log-section" style="margin-top:16px">
      <summary style="cursor:pointer;font-size:13px;font-weight:600;color:var(--text2)"><SvgIcon name="clipboard" /> Agent 日志</summary>
      <div style="margin-top:8px;display:flex;gap:8px;align-items:center">
        <input v-model="logAgent" placeholder="Agent ID（留空=全部）" style="width:200px;font-size:13px;padding:4px 8px;border:1px solid var(--border2);border-radius:4px" />
        <button class="btn btn-sm" @click="loadLogs" :disabled="logLoading">{{ logLoading ? '加载中…' : '查询' }}</button>
      </div>
      <div v-if="logs.length" style="margin-top:8px;max-height:300px;overflow-y:auto;font-size:12px;font-family:monospace;background:var(--bg);border:1px solid var(--border2);border-radius:6px;padding:8px">
        <div v-for="l in logs" :key="l.id" style="padding:3px 0;border-bottom:1px solid var(--border);display:flex;gap:8px">
          <span style="color:var(--muted);white-space:nowrap">{{ l.created_at?.slice(0,19) || '' }}</span>
          <span style="color:var(--accent);white-space:nowrap">{{ l.agent_id }}</span>
          <span :class="levelClass(l.level)" class="tag" style="font-size:10px;padding:1px 5px">[{{ l.level }}]</span>
          <span>{{ l.content }}</span>
        </div>
      </div>
      <div v-else-if="logLoading" style="margin-top:8px;color:var(--muted);font-size:13px">加载中…</div>
    </details>
  </div>

  <!-- P0-1 危险操作确认框（关机/重启需密码二次验证） -->
  <div v-if="confirmDlg" class="modal-mask" @click.self="confirmDlg = false">
    <div class="modal">
      <h3 class="card-title">{{ actionLabel(confirmDlg.action) }} — 确认</h3>
      <p style="color:var(--text2);font-size:13px;line-height:1.7">
        即将对 <b style="color:var(--accent)">{{ confirmDlg.agents.length }}</b> 台终端执行
        <b>{{ actionLabel(confirmDlg.action) }}</b>：
      </p>
      <div class="agent-chips">
        <span v-for="a in confirmDlg.agents.slice(0, 12)" :key="a" class="chip">{{ a }}</span>
        <span v-if="confirmDlg.agents.length > 12" class="chip">…等 {{ confirmDlg.agents.length }} 台</span>
      </div>
      <p v-if="isDangerous(confirmDlg.action)" style="color:var(--red);font-size:12px;margin-top:10px">
        <SvgIcon name="alert" /> 危险操作，请输入当前账号密码二次验证：
      </p>
      <input v-if="isDangerous(confirmDlg.action)" v-model="confirmPwd" type="password" placeholder="输入密码" style="width:100%;margin-top:4px" @keyup.enter="doSend" />
      <div v-if="confirmErr" class="err">{{ confirmErr }}</div>
      <div style="display:flex;gap:10px;justify-content:flex-end;margin-top:14px">
        <button class="btn btn-outline" @click="confirmDlg = false">取消</button>
        <button class="btn" :class="isDangerous(confirmDlg.action) ? 'btn-danger' : 'btn-primary'" :disabled="sending" @click="doSend">
          {{ sending ? '下发中…' : '确认执行' }}
        </button>
      </div>
    </div>
  </div>

  <!-- P0-1 执行命令框 -->
  <div v-if="execDlg" class="modal-mask" @click.self="execDlg = false">
    <div class="modal" style="width:520px">
      <h3 class="card-title"><SvgIcon name="play" /> 远程执行命令</h3>
      <p style="color:var(--text2);font-size:12px;line-height:1.7">
        目标：<b style="color:var(--accent)">{{ execDlg.agents.length }}</b> 台（{{ execDlg.agents.slice(0, 3).join('、') }}{{ execDlg.agents.length > 3 ? '…' : '' }}）。
        命令必须命中<b>系统设置 → 命令执行白名单</b>，否则服务端拒绝并记录审计。
      </p>
      <div style="display:flex;gap:8px;margin-top:10px">
        <select v-model="execCmd" style="flex:1" v-if="whitelist.length">
          <option value="" disabled>选择白名单命令模板…</option>
          <option v-for="w in whitelist" :key="w" :value="w">{{ w }}</option>
        </select>
        <input v-model="execCmd" style="flex:1" placeholder="输入完整命令，如：ip a" @keyup.enter="confirmAction('exec')" />
      </div>
      <div v-if="confirmErr" class="err">{{ confirmErr }}</div>
      <div style="display:flex;gap:10px;justify-content:flex-end;margin-top:14px">
        <button class="btn btn-outline" @click="execDlg = false">取消</button>
        <button class="btn btn-primary" :disabled="sending" @click="confirmAction('exec')">{{ sending ? '下发中…' : '下一步' }}</button>
      </div>
    </div>
  </div>

  <!-- P1 消息广播框 -->
  <div v-if="msgDlg" class="modal-mask" @click.self="msgDlg = null">
    <div class="modal" style="width:520px">
      <h3 class="card-title"><SvgIcon name="chat" /> 发送消息</h3>
      <p style="color:var(--text2);font-size:12px;line-height:1.7">
        目标：<b style="color:var(--accent)">{{ msgDlg.agents.length }}</b> 台（{{ msgDlg.agents.slice(0, 3).join('、') }}{{ msgDlg.agents.length > 3 ? '…' : '' }}）。
        消息将在学生端弹窗显示。
      </p>
      <div style="display:flex;gap:8px;margin-top:10px;align-items:center">
        <input v-model="msgTitle" style="flex:1" placeholder="标题（可留空，默认：教师消息）" />
        <select v-model="msgMode" style="width:150px">
          <option value="notify">通知模式（定时关闭）</option>
          <option value="confirm">确认模式（需学生点已读）</option>
        </select>
        <input v-if="msgMode === 'notify'" v-model.number="msgDuration" type="number" min="3" max="600" style="width:80px" title="显示秒数" />
        <span v-if="msgMode === 'notify'" style="color:var(--muted);font-size:12px">秒</span>
      </div>
      <textarea v-model="msgBody" rows="4" style="width:100%;margin-top:8px;resize:vertical" placeholder="消息内容（必填）"></textarea>
      <div v-if="confirmErr" class="err">{{ confirmErr }}</div>
      <div style="display:flex;gap:10px;justify-content:flex-end;margin-top:14px">
        <button class="btn btn-outline" @click="msgDlg = null">取消</button>
        <button class="btn btn-primary" :disabled="sending" @click="doSendMsg">{{ sending ? '发送中…' : '发送' }}</button>
      </div>
    </div>
  </div>

  <!-- 详情抽屉 -->
  <div v-if="detail" class="drawer-mask" @click.self="detail=null">
    <div class="drawer">
      <div class="drawer-head"><h3>Agent 详情</h3><button class="btn btn-sm" @click="detail=null"><SvgIcon name="x" /> </button></div>
      <div class="drawer-body">
        <div class="kv"><span>Agent ID</span><b>{{ detail.agent_id }}</b></div>
        <div class="kv"><span>操作系统</span><b>{{ detail.os || '-' }}</b></div>
        <div class="kv"><span>架构</span><b>{{ detail.arch || '-' }}</b></div>
        <div class="kv"><span>MAC</span><b>{{ detail.mac || '-' }}</b></div>
        <div class="kv"><span>启动模式</span><b>{{ detail.boot_mode || '-' }}</b></div>
        <div class="kv"><span>版本</span><b>{{ detail.agent_version || '-' }}</b></div>
        <div class="kv"><span>在线状态</span><b :class="onlineIds.includes(detail.agent_id) ? 'on' : 'off'">{{ onlineIds.includes(detail.agent_id) ? '在线' : '离线' }}</b></div>
        <div class="kv"><span>最后心跳</span><b>{{ fmt(detail.last_heartbeat) }}</b></div>
        <div style="margin-top:14px;display:flex;gap:6px;flex-wrap:wrap">
          <button class="btn btn-sm btn-outline" @click="confirmAction('lock', detail)"><SvgIcon name="lock" /> 锁定</button>
          <button class="btn btn-sm btn-outline" @click="confirmAction('unlock', detail)"><SvgIcon name="unlock" /> 解锁</button>
          <button class="btn btn-sm btn-danger-outline" @click="confirmAction('shutdown', detail)">⏻ 关机</button>
          <button class="btn btn-sm btn-danger-outline" @click="confirmAction('reboot', detail)"><SvgIcon name="power" /> 重启</button>
          <button class="btn btn-sm btn-outline" @click="openExec(detail)"><SvgIcon name="play" /> 执行命令</button>
        </div>
      </div>
      <div class="drawer-foot"><button class="btn" @click="detail=null">关闭</button></div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import api from '../api'
import ListState from '../components/ListState.vue'
import Pagination from '../components/Pagination.vue'
import { useEventStream } from '../composables/useEventStream'

const data = ref({ online_count: 0, online_ids: [], sessions: [] })
const sessions = computed(() => data.value.sessions || [])
const onlineIds = computed(() => data.value.online_ids || [])

const state = ref('loading')
const refreshing = ref(false)
const keyword = ref('')
const statusFilter = ref('all')
const page = ref(1)
const pageSize = 15
const lastUpdated = ref('')
const detail = ref(null)

const POLL_INTERVAL = 15
const autoPoll = ref(true)
const countdown = ref(POLL_INTERVAL)
let pollTimer = null

const { connected, on } = useEventStream()
on('agent_status', p => { load() })

const filtered = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  let list = sessions.value
  if (statusFilter.value === 'online') list = list.filter(s => onlineIds.value.includes(s.agent_id))
  if (statusFilter.value === 'offline') list = list.filter(s => !onlineIds.value.includes(s.agent_id))
  if (k) list = list.filter(s => (s.agent_id || '').toLowerCase().includes(k) || (s.mac || '').toLowerCase().includes(k) || (s.os || '').toLowerCase().includes(k))
  return list
})

const paged = computed(() => filtered.value.slice((page.value - 1) * pageSize, page.value * pageSize))

async function load(manual = false) {
  if (manual) refreshing.value = true
  try {
    const res = await api.get('/agents/online')
    data.value = res.data || { online_count: 0, online_ids: [], sessions: [] }
    state.value = sessions.value.length ? 'ready' : 'empty'
    lastUpdated.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
    countdown.value = POLL_INTERVAL
  } catch (e) { if (!sessions.value.length) state.value = 'error' }
  finally { refreshing.value = false }
}

function fmt(t) { if (!t) return '-'; return new Date(t).toLocaleString('zh-CN', { hour12: false }) }
function openDetail(s) { detail.value = s }

onMounted(() => {
  load()
  pollTimer = setInterval(() => {
    if (connected.value) return
    if (!autoPoll.value) return
    if (countdown.value > 1) { countdown.value--; return }
    load()
  }, 1000)
})
onUnmounted(() => clearInterval(pollTimer))

// ============ P0-1 远程控制 ============
const selected = ref([])
const allChecked = computed(() => paged.value.length > 0 && paged.value.every(s => selected.value.includes(s.agent_id)))
function toggleAll(e) {
  if (e.target.checked) selected.value = [...new Set([...selected.value, ...paged.value.map(s => s.agent_id)])]
  else selected.value = selected.value.filter(id => !paged.value.some(s => s.agent_id === id))
}
function toggleOne(id) {
  selected.value = selected.value.includes(id) ? selected.value.filter(x => x !== id) : [...selected.value, id]
}

const ACTION_LABELS = { lock: '锁定屏幕', unlock: '解除锁定', shutdown: '远程关机', reboot: '远程重启', exec: '执行命令', snapshot: '屏幕截图', message: '发送消息', session_logoff: '注销会话', wol: '远程开机' }
function actionLabel(a) { return ACTION_LABELS[a] || a }
function isDangerous(a) { return ['shutdown', 'reboot', 'exec', 'agent_upgrade', 'session_logoff'].includes(a) }

const confirmDlg = ref(null)   // { action, agents[] }
const confirmPwd = ref('')
const confirmErr = ref('')
const sending = ref(false)
const execDlg = ref(null)      // { agents[] }
const execCmd = ref('')
const whitelist = ref([])

// 打开 exec 输入框（第一步）
async function openExec(single) {
  confirmErr.value = ''
  const agents = single ? [single.agent_id] : (selected.value.length ? [...selected.value] : [])
  if (!agents.length) return
  execDlg.value = { agents }
  execCmd.value = ''
  try {
    const r = await api.get('/system/cmd-whitelist')
    whitelist.value = r.data?.whitelist || []
  } catch { whitelist.value = [] }
}

// 确认入口：lock/unlock 直接弹确认；exec 先选命令再弹确认；危险操作在确认框内输密码
function confirmAction(action, single) {
  confirmErr.value = ''
  if (action === 'exec' && !execDlg.value) { openExec(single); return }
  if (action === 'exec' && execDlg.value) {
    const cmd = execCmd.value.trim()
    if (!cmd) { confirmErr.value = '请输入要执行的命令'; return }
    confirmDlg.value = { action: 'exec', agents: execDlg.value.agents, cmd }
    confirmPwd.value = ''
    return
  }
  const agents = single ? [single.agent_id] : (selected.value.length ? [...selected.value] : [])
  if (!agents.length) return
  confirmDlg.value = { action, agents }
  confirmPwd.value = ''
}

async function doSend() {
  const dlg = confirmDlg.value
  if (!dlg) return
  confirmErr.value = ''
  sending.value = true
  try {
    const body = { agent_ids: dlg.agents, action: dlg.action, params: {} }
    if (dlg.action === 'exec') body.params.cmd = dlg.cmd
    if (isDangerous(dlg.action)) body.password = confirmPwd.value
    const res = await api.post('/ctrl/command', body)
    if (res.code === 0) {
      const d = res.data || {}
      results.value.unshift({
        time: new Date().toLocaleTimeString('zh-CN', { hour12: false }),
        agent_id: dlg.agents.length > 1 ? `${d.notified}/${d.total} 台已下发` : dlg.agents[0],
        action: dlg.action, status: 'pending', detail: '指令已下发，等待 Agent 回执…',
        msg_id: '__issued_' + Date.now(),
      })
      confirmDlg.value = false
      execDlg.value = null
    } else {
      confirmErr.value = res.msg || '下发失败'
    }
  } catch (e) {
    confirmErr.value = e.response?.data?.msg || e.message || '下发失败'
  } finally { sending.value = false }
}

// ---- P1: 消息广播 ----
const msgDlg = ref(null)      // { agents[] }
const msgTitle = ref('')
const msgBody = ref('')
const msgMode = ref('notify')
const msgDuration = ref(15)
function openMsgDlg(single) {
  confirmErr.value = ''
  const agents = single ? [single.agent_id] : (selected.value.length ? [...selected.value] : [])
  if (!agents.length) return
  msgDlg.value = { agents }
  msgTitle.value = ''
  msgBody.value = ''
  msgMode.value = 'notify'
  msgDuration.value = 15
}
async function doSendMsg() {
  const dlg = msgDlg.value
  if (!dlg) return
  confirmErr.value = ''
  if (!msgBody.value.trim()) { confirmErr.value = '消息内容不能为空'; return }
  sending.value = true
  try {
    const res = await api.post('/msg/send', {
      agent_ids: dlg.agents,
      title: msgTitle.value.trim() || '教师消息',
      body: msgBody.value.trim(),
      mode: msgMode.value,
      duration: msgMode.value === 'notify' ? msgDuration.value : 0,
    })
    if (res.code === 0) {
      const d = res.data || {}
      results.value.unshift({
        time: new Date().toLocaleTimeString('zh-CN', { hour12: false }),
        agent_id: dlg.agents.length > 1 ? `${d.notified}/${d.total} 台已下发` : dlg.agents[0],
        action: 'message', status: 'pending',
        detail: `「${msgBody.value.trim().slice(0, 40)}${msgBody.value.trim().length > 40 ? '…' : ''}」已下发，等待弹窗回执…`,
        msg_id: '__issued_' + Date.now(),
      })
      msgDlg.value = null
    } else {
      confirmErr.value = res.msg || '发送失败'
    }
  } catch (e) {
    confirmErr.value = e.response?.data?.msg || e.message || '发送失败'
  } finally { sending.value = false }
}

// ---- 指令结果实时展示（WS cmd_ack 推送 + 历史回填） ----
const results = ref([])
function statusClass(s) {
  return { ok: 'tag-green', pending: 'tag-orange', failed: 'tag-red', error: 'tag-red', rejected: 'tag-red', downloading: 'tag-orange', unknown_action: 'tag-orange' }[s] || 'tag-off'
}
function statusText(s) {
  return { ok: '成功', pending: '待回执', failed: '失败', error: '错误', rejected: '已拒绝', downloading: '下载中', unknown_action: '未知指令' }[s] || s
}
on('cmd_ack', p => {
  if (!p) return
  results.value.unshift({
    time: new Date().toLocaleTimeString('zh-CN', { hour12: false }),
    agent_id: p.agent_id, action: p.action, status: p.status, detail: p.detail || '', msg_id: p.msg_id,
  })
  if (results.value.length > 100) results.value.pop()
  // 把 pending 占位条目移除（任意一条回执到达）
  const idx = results.value.findIndex(r => r.msg_id && r.msg_id.startsWith('__issued_'))
  if (idx >= 0) results.value.splice(idx, 1)
})

async function loadResultHistory() {
  try {
    const r = await api.get('/ctrl/results', { params: { page: 1, page_size: 20 } })
    const items = r.data?.items || []
    items.reverse().forEach(it => {
      results.value.push({
        time: (it.created_at || '').replace('T', ' ').slice(11, 19),
        agent_id: it.agent_id, action: it.action, status: it.status, detail: it.detail || '', msg_id: it.msg_id,
      })
    })
    if (results.value.length > 100) results.value.splice(100)
  } catch { /* 历史加载失败不影响页面 */ }
}
onMounted(loadResultHistory)

// --- Agent 日志 ---
const logs = ref([])
const logAgent = ref('')
const logLoading = ref(false)

// ---- P2-2: WOL 远程开机（magic packet 广播，无需 Agent 在线） ----
const wolSending = ref(false)
async function doWol(single) {
  if (wolSending.value) return
  const agents = single ? [single.agent_id] : (selected.value.length ? [...selected.value] : [])
  if (!agents.length) { confirmErr.value = '请先选择要开机的设备'; return }
  if (!window.confirm(`向 ${agents.length} 台设备发送开机指令（WOL magic packet）？\n设备需支持 Wake-on-LAN 并在 BIOS/网卡中启用。`)) return
  wolSending.value = true
  confirmErr.value = ''
  try {
    const res = await api.post('/wol', { agent_ids: agents })
    if (res.code === 0) {
      const d = res.data || {}
      results.value.unshift({
        time: new Date().toLocaleTimeString('zh-CN', { hour12: false }),
        agent_id: agents.length > 1 ? `${agents.length} 台批量开机` : agents[0],
        action: 'wol', status: d.sent ? 'sent' : 'partial', detail: d.detail || '已发送 magic packet',
        msg_id: '__issued_' + Date.now(),
      })
    } else {
      confirmErr.value = res.msg || '开机失败'
    }
  } catch (e) {
    confirmErr.value = e.response?.data?.msg || e.message || '开机失败'
  } finally { wolSending.value = false }
}
async function loadLogs() {
  logLoading.value = true
  try {
    const r = await api.get('/agent/logs', { params: { agent_id: logAgent.value || undefined, limit: 50 } })
    logs.value = r.data || []
  } catch { logs.value = [] }
  finally { logLoading.value = false }
}
function levelClass(l) { return l === 'error' ? 'tag-red' : l === 'warn' ? 'tag-orange' : 'tag-green' }
</script>

<style scoped>
.empty-cell { text-align: center; color: var(--muted); padding: 24px 0 !important; }
.clickable { cursor: pointer; }
.poll-tip { display: inline-flex; align-items: center; gap: 6px; font-size: 12px; color: var(--text2); padding: 4px 10px; border-radius: 14px; background: rgba(0, 179, 104, .08); }
.poll-tip.paused { background: rgba(138, 153, 184, .12); color: var(--muted); }
.poll-tip.live { background: rgba(0, 179, 104, .1); color: var(--green); }
.poll-tip .dot { width: 7px; height: 7px; border-radius: 50%; background: var(--green); animation: blink 2s infinite; }
.poll-tip.paused .dot { background: var(--muted); animation: none; }
@keyframes blink { 0%, 100% { opacity: 1; } 50% { opacity: .35; } }
.drawer-mask { position: fixed; inset: 0; z-index: 180; background: rgba(15, 25, 50, .35); }
.drawer { position: absolute; top: 0; right: 0; height: 100%; width: 400px; max-width: 92vw; background: var(--card); box-shadow: -8px 0 32px rgba(20, 40, 90, .18); display: flex; flex-direction: column; }
.drawer-head { display: flex; justify-content: space-between; align-items: center; padding: 16px 20px; border-bottom: 1px solid var(--border); }
.drawer-head h3 { font-size: 15px; }
.drawer-body { flex: 1; overflow-y: auto; padding: 12px 20px; }
.kv { display: flex; justify-content: space-between; align-items: center; padding: 10px 0; border-bottom: 1px dashed var(--border); font-size: 13px; }
.kv span { color: var(--text2); }
.kv b.on { color: var(--green); }
.kv b.off { color: var(--red); }
.drawer-foot { padding: 14px 20px; border-top: 1px solid var(--border); text-align: right; }
.drawer-enter-active, .drawer-leave-active { transition: opacity .2s; }
.drawer-enter-from, .drawer-leave-to { opacity: 0; }
/* P0-1 */
.batch-bar { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; padding: 10px 14px; margin: 10px 0; background: var(--accent-glow, rgba(0, 102, 255, .06)); border: 1px solid var(--border2); border-radius: 8px; font-size: 13px; }
.ctrl-cell { white-space: nowrap; }
.btn-xs { padding: 2px 7px; font-size: 12px; margin-right: 3px; }
.btn-danger-outline { border: 1px solid var(--red); color: var(--red); background: transparent; }
.btn-danger-outline:hover { background: rgba(220, 60, 60, .08); }
.btn-danger { background: var(--red); color: #fff; }
.modal-mask { position: fixed; inset: 0; z-index: 200; background: rgba(15, 25, 50, .45); display: flex; align-items: center; justify-content: center; }
.modal { background: var(--card); border-radius: 10px; padding: 22px 24px; width: 440px; max-width: 92vw; box-shadow: 0 16px 48px rgba(20, 40, 90, .25); }
.agent-chips { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 8px; max-height: 90px; overflow-y: auto; }
.chip { font-size: 11px; font-family: monospace; padding: 2px 8px; background: var(--bg); border: 1px solid var(--border2); border-radius: 10px; }
.err { color: var(--red); font-size: 12px; margin-top: 8px; }
input[type="password"], .modal input, .modal select { padding: 8px 10px; border: 1px solid var(--border2); border-radius: 6px; font-size: 13px; }
</style>
