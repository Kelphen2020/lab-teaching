<template>
  <!-- SVG 雪碧图（内联 symbol，零外部请求） -->
  <svg style="display:none" aria-hidden="true">
    <symbol id="i-chart" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <path d="M3 3v18h18"/><path d="M7 15l4-5 3 3 5-7"/>
    </symbol>
    <symbol id="i-box" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <path d="M21 8l-9-5-9 5v8l9 5 9-5V8z"/><path d="M3 8l9 5 9-5"/><path d="M12 13v8"/>
    </symbol>
    <symbol id="i-cpu" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <rect x="6" y="6" width="12" height="12" rx="2"/><rect x="10" y="10" width="4" height="4"/>
      <path d="M9 2v4M15 2v4M9 18v4M15 18v4M2 9h4M2 15h4M18 9h4M18 15h4"/>
    </symbol>
    <symbol id="i-broadcast" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <circle cx="12" cy="12" r="2"/><path d="M7.8 7.8a6 6 0 0 0 0 8.4M16.2 7.8a6 6 0 0 1 0 8.4"/>
      <path d="M4.9 4.9a10 10 0 0 0 0 14.2M19.1 4.9a10 10 0 0 1 0 14.2"/>
    </symbol>
    <symbol id="i-folder" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7z"/>
    </symbol>
    <symbol id="i-screen" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <rect x="3" y="4" width="18" height="13" rx="2"/><path d="M8 21h8M12 17v4"/>
    </symbol>
    <symbol id="i-doc" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8l-5-5z"/>
      <path d="M14 3v5h5M9 13h6M9 17h4"/>
    </symbol>
    <symbol id="i-building" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <path d="M4 21V5a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v16M16 9h3a1 1 0 0 1 1 1v11M2 21h20"/>
      <path d="M8 7h2M8 11h2M8 15h2"/>
    </symbol>
    <symbol id="i-swap" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <path d="M4 8h13l-3-3M20 16H7l3 3"/>
    </symbol>
    <symbol id="i-shield" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <path d="M12 3l7 3v6c0 4.5-3 7.5-7 9-4-1.5-7-4.5-7-9V6l7-3z"/><path d="M9.5 12l2 2 3.5-4"/>
    </symbol>
    <symbol id="i-key" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <circle cx="8" cy="15" r="4"/><path d="M10.8 12.2L20 3M16 4l3 3M13 7l3 3"/>
    </symbol>
    <symbol id="i-wrench" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <path d="M14.7 6.3a4.5 4.5 0 0 0-6 6L3 18l3 3 5.7-5.7a4.5 4.5 0 0 0 6-6L14 13l-3-3 3.7-3.7z"/>
    </symbol>
    <symbol id="i-gear" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <circle cx="12" cy="12" r="3.2"/>
      <path d="M19.4 15a1.6 1.6 0 0 0 .32 1.77l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.6 1.6 0 0 0-1.77-.32 1.6 1.6 0 0 0-.97 1.47V21a2 2 0 1 1-4 0v-.09a1.6 1.6 0 0 0-1.05-1.47 1.6 1.6 0 0 0-1.77.32l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.6 1.6 0 0 0 .32-1.77 1.6 1.6 0 0 0-1.47-.97H3a2 2 0 1 1 0-4h.09a1.6 1.6 0 0 0 1.47-1.05 1.6 1.6 0 0 0-.32-1.77l-.06-.06A2 2 0 1 1 7 4.51l.06.06a1.6 1.6 0 0 0 1.77.32H9a1.6 1.6 0 0 0 .97-1.47V3a2 2 0 1 1 4 0v.09a1.6 1.6 0 0 0 .97 1.47 1.6 1.6 0 0 0 1.77-.32l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.6 1.6 0 0 0-.32 1.77V9a1.6 1.6 0 0 0 1.47.97H21a2 2 0 1 1 0 4h-.09a1.6 1.6 0 0 0-1.51.97z"/>
    </symbol>
  </svg>

  <div class="app-shell">
    <!-- L1 顶栏 -->
    <header class="topbar">
      <div class="logo">
        <img v-if="settings.site_logo" :src="settings.site_logo" class="logo-img" alt="logo" />
        <span v-else class="logo-mark">◈</span>
        <span class="logo-text">{{ settings.site_title }}</span>
      </div>
      <div class="topbar-right">
        <!-- P2: 全局搜索 -->
        <div class="global-search">
          <input v-model="searchQ" placeholder="搜索课程/学生/机房…" @keyup.enter="doSearch" />
          <div v-if="searchResults.length && searchQ" class="search-drop">
            <router-link v-for="r in searchResults" :key="r.id" :to="r.to" class="search-item">
              <span class="search-type">{{ r.type }}</span>
              <span>{{ r.name }}</span>
            </router-link>
          </div>
        </div>
        <!-- d3: 全局通知中心入口 -->
        <NotificationCenter v-if="auth.can('monitor') || auth.can('audits') || auth.isAdmin" />
        <button class="btn btn-outline btn-sm" title="新手引导" @click="openGuide"><SvgIcon name="help" /> 引导</button>
        <span class="user-chip" @click="showPwdModal = true" title="点击修改密码">
          <b>{{ auth.user?.real_name || auth.user?.username }}</b>
          <span class="role-tag">{{ auth.user?.role }}</span>
        </span>
        <button class="btn btn-outline btn-sm" @click="logout">退出</button>
      </div>
      <div class="trail-line"></div>
    </header>

    <div class="shell-body">
      <!-- L2 侧边导航 -->
      <aside class="sidebar">
        <nav>
          <router-link v-if="auth.can('dashboard')" to="/dashboard"><span class="nav-ico chip-blue"><svg><use href="#i-chart"/></svg></span>仪表盘</router-link>
          <router-link v-if="auth.can('rooms')" to="/rooms"><span class="nav-ico chip-cyan"><svg><use href="#i-box"/></svg></span>机房管理</router-link>
          <router-link v-if="auth.can('agents')" to="/agents"><span class="nav-ico chip-green"><svg><use href="#i-cpu"/></svg></span>Agent 在线</router-link>
          <router-link v-if="auth.can('broadcast')" to="/broadcast"><span class="nav-ico chip-purple"><svg><use href="#i-broadcast"/></svg></span>广播控制</router-link>
          <router-link v-if="auth.can('files')" to="/files"><span class="nav-ico chip-orange"><svg><use href="#i-folder"/></svg></span>文件收发</router-link>
          <router-link v-if="auth.can('remote')" to="/remote"><span class="nav-ico chip-teal"><svg><use href="#i-screen"/></svg></span>远程桌面</router-link>
          <router-link v-if="auth.can('exam')" to="/exam"><span class="nav-ico chip-red"><svg><use href="#i-doc"/></svg></span>考试管理</router-link>
          <div class="nav-group">教学资源</div>
          <router-link to="/courses"><span class="nav-ico chip-blue"><svg><use href="#i-doc"/></svg></span>课程管理</router-link>
          <router-link to="/clazzes"><span class="nav-ico chip-cyan"><svg><use href="#i-doc"/></svg></span>班级管理</router-link>
          <router-link to="/students"><span class="nav-ico chip-green"><svg><use href="#i-doc"/></svg></span>学生管理</router-link>
          <router-link to="/schedules"><span class="nav-ico chip-purple"><svg><use href="#i-doc"/></svg></span>课表管理</router-link>
          <div v-if="auth.can('monitor') || auth.can('audits') || auth.isAdmin || auth.can('campuses') || auth.can('edusync') || auth.can('access') || auth.can('license') || auth.can('ops')" class="nav-group">系统功能</div>
          <router-link v-if="auth.can('campuses')" to="/campuses"><span class="nav-ico chip-blue"><svg><use href="#i-building"/></svg></span>多校区管理</router-link>
          <router-link v-if="auth.can('edusync')" to="/edusync"><span class="nav-ico chip-cyan"><svg><use href="#i-swap"/></svg></span>教务对接</router-link>
          <router-link v-if="auth.can('ops')" to="/ops"><span class="nav-ico chip-purple"><svg><use href="#i-wrench"/></svg></span>运维升级</router-link>
          <router-link v-if="auth.can('access')" to="/access"><span class="nav-ico chip-green"><svg><use href="#i-shield"/></svg></span>一卡通/门禁</router-link>
          <router-link v-if="auth.can('license')" to="/license"><span class="nav-ico chip-orange"><svg><use href="#i-key"/></svg></span>授权管理</router-link>
          <router-link v-if="auth.can('monitor')" to="/monitor"><span class="nav-ico chip-red"><svg><use href="#i-chart"/></svg></span>监控告警</router-link>
          <router-link v-if="auth.can('audits')" to="/audits"><span class="nav-ico chip-blue"><svg><use href="#i-doc"/></svg></span>审计日志</router-link>
          <router-link to="/screenshots"><span class="nav-ico chip-purple"><svg><use href="#i-screen"/></svg></span>屏幕截图</router-link>
          <router-link to="/wall"><span class="nav-ico chip-cyan"><svg><use href="#i-chart"/></svg></span>监控墙</router-link>
          <router-link to="/desktop-services"><span class="nav-ico chip-green"><svg><use href="#i-cpu"/></svg></span>桌面服务</router-link>
          <router-link to="/power-tasks"><span class="nav-ico chip-orange"><svg><use href="#i-gear"/></svg></span>定时电源任务</router-link>
          <router-link to="/collect"><span class="nav-ico chip-blue"><svg><use href="#i-doc"/></svg></span>作业收集</router-link>
          <router-link v-if="auth.isAdmin" to="/settings"><span class="nav-ico chip-teal"><svg><use href="#i-gear"/></svg></span>系统设置</router-link>
          <router-link v-if="auth.isAdmin" to="/api-docs"><span class="nav-ico chip-blue"><svg><use href="#i-doc"/></svg></span>API 接口文档</router-link>
        </nav>
      </aside>

      <!-- L4 内容区 -->
      <main class="main">
        <div class="page-head">
          <h1>{{ title }}</h1>
        </div>
        <router-view />
      </main>
    </div>

    <!-- d2: 首次登录新手引导（分步指引） -->
    <GuideTour :open="guideOpen" :steps="guideSteps" @close="onGuideClose" />

    <!-- P0-2: 修改密码弹窗 -->
    <div class="modal-mask" v-if="showPwdModal" @click.self="showPwdModal=false">
      <div class="modal-card">
        <h3>修改密码</h3>
        <div class="form-row"><label>旧密码</label><input class="input" v-model="pwdForm.old" type="password" placeholder="输入当前密码"></div>
        <div class="form-row"><label>新密码</label><input class="input" v-model="pwdForm.new1" type="password" placeholder="至少8位，含字母+数字"></div>
        <div class="form-row"><label>确认新密码</label><input class="input" v-model="pwdForm.new2" type="password" placeholder="再次输入新密码"></div>
        <p v-if="pwdMsg" class="pwd-msg" :class="{ 'pwd-err': pwdErr }">{{ pwdMsg }}</p>
        <div class="modal-foot">
          <button class="btn" @click="showPwdModal=false">取消</button>
          <button class="btn btn-primary" @click="changePwd" :disabled="pwdSaving">{{ pwdSaving ? '修改中…' : '确认修改' }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useSettingsStore } from '../stores/settings'
import api from '../api'
import GuideTour from '../components/GuideTour.vue'
import NotificationCenter from '../components/NotificationCenter.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const settings = useSettingsStore()

const titleMap = { '/dashboard': '仪表盘', '/rooms': '机房管理', '/agents': 'Agent 在线状态', '/broadcast': '屏幕广播控制', '/files': '文件收发', '/remote': '远程桌面', '/exam': '考试管理', '/courses': '课程管理', '/clazzes': '班级管理', '/students': '学生管理', '/schedules': '课表管理', '/campuses': '多校区管理', '/edusync': '教务系统对接', '/access': '一卡通/门禁', '/license': '授权管理', '/ops': '运维升级', '/monitor': '监控告警', '/wall': '监控墙', '/power-tasks': '定时电源任务', '/collect': '作业收集', '/api-docs': 'API 接口文档', '/audits': '审计日志', '/settings': '系统设置' }
const title = computed(() => titleMap[route.path] || settings.site_title)

// ── d2: 首次登录新手引导 ─────────────────────────────
const GUIDE_KEY = 'lab_guide_done_v1'
const guideOpen = ref(false)

// 步骤按当前用户权限过滤，只引导能访问的功能
const guideSteps = computed(() => {
  const all = [
    { icon: 'i-chart', chip: 'chip-blue', perm: 'dashboard', to: '/dashboard', title: '仪表盘总览',
      desc: '登录后的第一屏：机房规模、Agent 在线率、近期待办一屏掌握。',
      tips: ['在线率低于 80% 会在监控告警页产生警告', '点击左侧导航可切换各功能模块'] },
    { icon: 'i-box', chip: 'chip-cyan', perm: 'rooms', to: '/rooms', title: '机房与终端管理',
      desc: '先创建机房，再为每台终端分配 Agent。终端是广播、收发文件、远程桌面的操作对象。',
      tips: ['建议按「校区 → 机房 → 终端」层级规划命名', '终端分组后可对分组批量下发任务'] },
    { icon: 'i-cpu', chip: 'chip-green', perm: 'agents', to: '/agents', title: 'Agent 在线状态',
      desc: '终端运行 Agent 程序后自动注册到平台。本页实时展示每台终端的在线状态与心跳（已支持 WebSocket 实时推送）。',
      tips: ['Agent 启动参数：服务器地址 + Agent ID', '离线终端会在心跳超时 60 秒后自动标记'] },
    { icon: 'i-broadcast', chip: 'chip-purple', perm: 'broadcast', to: '/broadcast', title: '屏幕广播',
      desc: '教师机画面实时推送到选中的学生终端，支持全屏广播与停止广播。',
      tips: ['先选目标终端再点「开始广播」', '广播中可随时暂停或切换目标'] },
    { icon: 'i-folder', chip: 'chip-orange', perm: 'files', to: '/files', title: '文件收发',
      desc: '向终端下发课件、作业等文件，也可收集学生提交的文件。',
      tips: ['支持按机房/分组批量下发', '下发进度与失败重试都在本页跟踪'] },
    { icon: 'i-screen', chip: 'chip-teal', perm: 'remote', to: '/remote', title: '远程桌面',
      desc: '实时查看任意终端屏幕，可发送鼠标键盘指令远程协助学生。',
      tips: ['仅在线终端可查看', '操作会被审计日志记录'] },
    { icon: 'i-doc', chip: 'chip-red', perm: 'exam', to: '/exam', title: '考试管理',
      desc: '创建试卷、安排考试、监控作答、收卷评分一站式完成。',
      tips: ['开考前请确认目标终端 Agent 全部在线', '考试中学生端自动锁定到答题界面'] },
    { icon: 'i-chart', chip: 'chip-red', perm: 'monitor', to: '/monitor', title: '监控告警',
      desc: '终端在线率、HTTP 错误率、升级任务失败率实时告警，支持实时推送（无需手动刷新）。',
      tips: ['在线率 <80% 警告、<50% 严重', 'HTTP 错误率 ≥5% 警告、≥20% 严重'] },
    { icon: 'i-doc', chip: 'chip-blue', perm: 'audits', to: '/audits', title: '审计日志',
      desc: '所有登录、配置变更、批量操作都有迹可循，支持按时间/用户/模块筛选。' },
    { icon: 'i-gear', chip: 'chip-teal', perm: null, admin: true, to: '/settings', title: '系统设置',
      desc: '站点名称、Logo、管理员账号、角色权限等全局配置（仅管理员可见）。',
      tips: ['建议第一时间修改默认管理员密码', '新增教师账号请分配合适角色'] }
  ]
  return all.filter(s => s.admin ? auth.isAdmin : auth.can(s.perm))
})

function openGuide() { guideOpen.value = true }
function onGuideClose(done) {
  guideOpen.value = false
  if (done) localStorage.setItem(GUIDE_KEY, '1')
}

// ── P0-2: 修改密码 ─────────────────────────────
const showPwdModal = ref(false)
const pwdForm = ref({ old: '', new1: '', new2: '' })
const pwdMsg = ref('')
const pwdErr = ref(false)
const pwdSaving = ref(false)
async function changePwd() {
  const { old, new1, new2 } = pwdForm.value
  if (!old || !new1 || !new2) { pwdMsg.value = '请填写所有字段'; pwdErr.value = true; return }
  if (new1.length < 6) { pwdMsg.value = '新密码至少6位'; pwdErr.value = true; return }
  if (new1 !== new2) { pwdMsg.value = '两次新密码不一致'; pwdErr.value = true; return }
  pwdSaving.value = true; pwdMsg.value = ''; pwdErr.value = false
  try {
    await api.put('/auth/password', { old_password: old, new_password: new1 })
    pwdMsg.value = '密码修改成功'
    pwdErr.value = false
    pwdForm.value = { old: '', new1: '', new2: '' }
    setTimeout(() => { showPwdModal.value = false; pwdMsg.value = '' }, 1500)
  } catch (e) {
    pwdMsg.value = e.response?.data?.msg || '修改失败'
    pwdErr.value = true
  } finally { pwdSaving.value = false }
}

onMounted(() => {
  settings.load()
  document.title = settings.site_title
  // 首次登录自动弹出引导（跳过不记录，下次登录仍会提示）
  if (!localStorage.getItem(GUIDE_KEY) && guideSteps.value.length) {
    guideOpen.value = true
  }
})

function logout() {
  auth.logout()
  router.push('/login')
}

// ── P2: 全局搜索 ─────────────────────────────
const searchQ = ref('')
const searchResults = ref([])
let searchTimer = null
async function doSearch() {
  const q = searchQ.value.trim()
  if (!q) { searchResults.value = []; return }
  clearTimeout(searchTimer)
  searchTimer = setTimeout(async () => {
    try {
      const [courses, students, rooms] = await Promise.all([
        api.get('/courses', { params: { keyword: q, page_size: 3 } }),
        api.get('/students', { params: { keyword: q, page_size: 3 } }),
        api.get('/rooms', { params: { keyword: q, page_size: 3 } }),
      ])
      const results = []
      for (const c of (courses.data?.items||courses.data||[])) { if (results.length>=8) break; results.push({ id: 'c'+c.id, type: '课程', name: c.name, to: '/courses' }) }
      for (const s of (students.data?.items||students.data||[])) { if (results.length>=8) break; results.push({ id: 's'+s.id, type: '学生', name: s.name||s.sno, to: '/students' }) }
      for (const r of (rooms.data?.items||rooms.data||[])) { if (results.length>=8) break; results.push({ id: 'r'+r.id, type: '机房', name: r.name, to: '/rooms' }) }
      searchResults.value = results
    } catch { searchResults.value = [] }
  }, 200)
}
</script>

<style scoped>
.modal-mask { position: fixed; inset: 0; background: rgba(0,0,0,.45); display: flex; align-items: center; justify-content: center; z-index: 200; }
.modal-card { background: var(--card); border-radius: 12px; padding: 24px 28px; min-width: 360px; max-width: 92vw; box-shadow: 0 12px 40px rgba(0,0,0,.25); }
.modal-card h3 { margin: 0 0 16px; font-size: 16px; }
.modal-foot { display: flex; justify-content: flex-end; gap: 10px; margin-top: 16px; }
.pwd-msg { font-size: 13px; margin: 8px 0 0; color: var(--green); }
.pwd-msg.pwd-err { color: var(--red); }
.user-chip { cursor: pointer; }
.user-chip:hover { opacity: .85; }
/* 全局搜索 */
.global-search { position: relative; margin-right: 8px; }
.global-search input { width: 200px; padding: 5px 12px; border: 1px solid var(--border2); border-radius: 20px; font-size: 13px; background: var(--card2); outline: none; transition: border-color .2s; }
.global-search input:focus { border-color: var(--accent); width: 260px; }
.search-drop { position: absolute; top: 100%; left: 0; right: 0; background: var(--card); border: 1px solid var(--border2); border-radius: 8px; margin-top: 4px; box-shadow: 0 8px 24px rgba(0,0,0,.12); z-index: 300; max-height: 300px; overflow-y: auto; }
.search-item { display: flex; align-items: center; gap: 8px; padding: 8px 14px; font-size: 13px; text-decoration: none; color: var(--text); border-bottom: 1px solid var(--border); transition: background .15s; }
.search-item:hover { background: rgba(0,102,255,.04); }
.search-item:last-child { border-bottom: none; }
.search-type { font-size: 10px; padding: 1px 6px; border-radius: 3px; background: var(--accent-glow); color: var(--accent); font-weight: 600; white-space: nowrap; }
</style>
