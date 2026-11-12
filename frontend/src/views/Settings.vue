<template>
  <div>
    <!-- Tab 切换 -->
    <div class="set-tabs card" style="padding: 0 20px; margin-bottom: 16px">
      <button v-for="t in tabs" :key="t.key" class="set-tab" :class="{ active: tab === t.key }" @click="tab = t.key">
        {{ t.label }}
      </button>
    </div>

    <!-- ============ 1. 主题设置 ============ -->
    <div v-if="tab === 'theme'" class="card">
      <h3 class="card-title">系统主题（预设 {{ THEME_PRESETS.length }} 套）</h3>
      <p style="color: var(--text2); font-size: 12px; margin-bottom: 16px">点击卡片即时预览，确认后点「保存主题」对所有用户生效。</p>
      <div class="theme-grid">
        <div v-for="t in THEME_PRESETS" :key="t.key" class="theme-card" :class="{ active: previewTheme === t.key }" @click="previewTheme = t.key">
          <div class="theme-preview" :style="previewStyle(t.key)">
            <div class="tp-bar"></div>
            <div class="tp-side"></div>
            <div class="tp-body">
              <div class="tp-line"></div><div class="tp-line short"></div>
            </div>
          </div>
          <div class="theme-meta">
            <b>{{ t.name }}</b>
            <span v-if="previewTheme === t.key" class="tag tag-on" style="margin-left:6px">当前</span>
            <p>{{ t.desc }}</p>
          </div>
        </div>
      </div>
      <div style="margin-top: 18px; display: flex; gap: 10px">
        <button class="btn btn-primary" :disabled="saving" @click="saveTheme">{{ saving ? '保存中...' : '保存主题' }}</button>
        <button class="btn btn-outline" @click="previewTheme = settings.theme">恢复上次保存</button>
      </div>
    </div>

    <!-- ============ 2. 品牌设置 ============ -->
    <div v-if="tab === 'brand'" class="card">
      <h3 class="card-title">系统标识（Logo 与标题）</h3>
      <div class="form-row">
        <label>顶部标题</label>
        <input v-model="brandTitle" placeholder="如：机房教学管理系统" maxlength="32" />
      </div>
      <div class="form-row">
        <label>系统 Logo</label>
        <div class="logo-up">
          <div class="logo-preview">
            <img v-if="brandLogo" :src="brandLogo" alt="logo" />
            <span v-else style="color: var(--muted)">未上传<br/>使用默认 ◈</span>
          </div>
          <div style="flex:1">
            <input type="file" ref="logoFile" accept="image/png,image/jpeg,image/svg+xml" style="display:none" @change="onLogoPick" />
            <button class="btn btn-outline" @click="$refs.logoFile.click()">选择图片（PNG/JPG/SVG，≤1MB）</button>
            <button v-if="brandLogo" class="btn btn-ghost" style="margin-left:8px" @click="removeLogo">移除 Logo</button>
            <p style="color: var(--muted); font-size: 12px; margin-top: 8px">Logo 显示在顶栏左侧，高 28px，建议使用透明底 PNG。</p>
          </div>
        </div>
      </div>
      <div style="margin-top: 18px">
        <button class="btn btn-primary" :disabled="saving" @click="saveBrand">{{ saving ? '保存中...' : '保存品牌设置' }}</button>
      </div>
    </div>

    <!-- ============ 3. 用户与权限分组 ============ -->
    <div v-if="tab === 'users'">
      <!-- 3.1 权限分组 -->
      <div class="card">
        <div class="toolbar">
          <h3 class="card-title" style="margin:0">权限分组</h3>
          <button class="btn btn-primary btn-sm" @click="openGroup(null)">＋ 新建分组</button>
        </div>
        <table>
          <thead><tr><th>分组名称</th><th>编码</th><th>说明</th><th>权限范围</th><th>成员数</th><th style="width:130px">操作</th></tr></thead>
          <tbody>
            <tr v-if="!groups.length"><td colspan="6" style="text-align:center;color:var(--muted)">暂无分组，点击右上角新建</td></tr>
            <tr v-for="g in groups" :key="g.id">
              <td><b>{{ g.name }}</b></td>
              <td><code style="font-size:12px;color:var(--text2)">{{ g.code }}</code></td>
              <td style="color:var(--text2)">{{ g.description || '—' }}</td>
              <td>
                <span v-if="permList(g).includes('*')" class="tag tag-on">全部权限</span>
                <span v-else style="color:var(--text2);font-size:12px">{{ permList(g).length }} 项模块</span>
              </td>
              <td>{{ g.member_count ?? '—' }}</td>
              <td>
                <button class="btn btn-outline btn-sm" @click="openGroup(g)">编辑</button>
                <button class="btn btn-ghost btn-sm" @click="delGroup(g)">删除</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- 3.2 用户管理 -->
      <div class="card">
        <div class="toolbar">
          <h3 class="card-title" style="margin:0">用户管理</h3>
          <button class="btn btn-primary btn-sm" @click="openUser(null)">＋ 新建用户</button>
        </div>
        <table>
          <thead><tr><th>用户名</th><th>姓名</th><th>角色</th><th>所属分组</th><th>状态</th><th style="width:170px">操作</th></tr></thead>
          <tbody>
            <tr v-for="u in users" :key="u.id">
              <td><b>{{ u.username }}</b></td>
              <td>{{ u.real_name || '—' }}</td>
              <td><span class="tag" :class="u.role === 'admin' ? 'tag-on' : 'tag-off'" style="background:var(--accent-glow);color:var(--accent)">{{ u.role === 'admin' ? '管理员' : '教师' }}</span></td>
              <td style="color:var(--text2)">{{ u.group_name || '未分组' }}</td>
              <td><span class="tag" :class="u.status === 'active' ? 'tag-on' : 'tag-off'">{{ u.status === 'active' ? '启用' : '停用' }}</span></td>
              <td>
                <button class="btn btn-outline btn-sm" @click="openUser(u)">编辑</button>
                <button class="btn btn-ghost btn-sm" @click="delUser(u)" v-if="u.username !== 'admin'">删除</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- ============ 模态框：分组编辑 ============ -->
    <div v-if="groupModal" class="modal-mask" @click.self="groupModal = false">
      <div class="modal">
        <h3 class="card-title">{{ editingGroup.id ? '编辑分组' : '新建分组' }}</h3>
        <div class="form-row"><label>分组名称 *</label><input v-model="editingGroup.name" placeholder="如：教务处" maxlength="64" /></div>
        <div class="form-row"><label>编码 *（唯一，小写字母/数字）</label><input v-model="editingGroup.code" :disabled="!!editingGroup.id" placeholder="如：jwc" maxlength="32" /></div>
        <div class="form-row"><label>说明</label><input v-model="editingGroup.description" placeholder="可选" maxlength="256" /></div>
        <div class="form-row">
          <label>权限范围</label>
          <div class="perm-grid">
            <label class="perm-item" v-for="m in MODULES" :key="m.key">
              <input type="checkbox" :value="m.key" v-model="editingPerms" /> {{ m.label }}
            </label>
          </div>
          <label class="perm-item" style="margin-top:6px"><input type="checkbox" v-model="editingPermsAll" /> 全部权限（*）</label>
        </div>
        <div class="err" v-if="groupErr">{{ groupErr }}</div>
        <div style="display:flex;gap:10px;justify-content:flex-end;margin-top:8px">
          <button class="btn btn-outline" @click="groupModal = false">取消</button>
          <button class="btn btn-primary" :disabled="saving" @click="saveGroup">保存</button>
        </div>
      </div>
    </div>

    <!-- ============ 模态框：用户编辑 ============ -->
    <div v-if="userModal" class="modal-mask" @click.self="userModal = false">
      <div class="modal">
        <h3 class="card-title">{{ editingUser.id ? '编辑用户' : '新建用户' }}</h3>
        <div class="form-row"><label>用户名 *</label><input v-model="editingUser.username" :disabled="!!editingUser.id" placeholder="登录账号" maxlength="32" /></div>
        <div class="form-row"><label>{{ editingUser.id ? '重置密码（留空不修改）' : '密码 *' }}</label><input v-model="editingUser.password" type="password" :placeholder="editingUser.id ? '至少8位，留空不修改' : '至少8位'" /></div>
        <div class="form-row"><label>姓名</label><input v-model="editingUser.real_name" placeholder="真实姓名" maxlength="32" /></div>
        <div class="form-row">
          <label>角色</label>
          <select v-model="editingUser.role">
            <option value="teacher">教师（普通）</option>
            <option value="admin">管理员</option>
          </select>
        </div>
        <div class="form-row">
          <label>所属分组</label>
          <select v-model="editingUser.group_id">
            <option :value="0">未分组</option>
            <option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }}</option>
          </select>
          <p style="color:var(--muted);font-size:12px;margin-top:4px">非管理员用户的菜单可见性由分组权限决定</p>
        </div>
        <div class="form-row" v-if="editingUser.id">
          <label>状态</label>
          <select v-model="editingUser.status">
            <option value="active">启用</option>
            <option value="disabled">停用</option>
          </select>
        </div>
        <div class="err" v-if="userErr">{{ userErr }}</div>
        <div style="display:flex;gap:10px;justify-content:flex-end;margin-top:8px">
          <button class="btn btn-outline" @click="userModal = false">取消</button>
          <button class="btn btn-primary" :disabled="saving" @click="saveUser">保存</button>
        </div>
      </div>
    </div>

    <!-- ============ 4. P0-1 命令执行白名单 ============ -->
    <div v-if="tab === 'cmdwl'" class="card">
      <h3 class="card-title">远程执行命令白名单（exec 指令）</h3>
      <p style="color:var(--text2);font-size:12px;line-height:1.8">
        教师端「Agent 在线」页面对终端下发 <b>执行命令</b> 指令时，命令必须完整命中下列白名单条目（前缀匹配）才会下发到 Agent；
        未命中的命令会被服务端<b style="color:var(--red)">直接拒绝并记录审计日志</b>。每行一条命令前缀，如：<code style="font-size:11px">ip a</code>、<code style="font-size:11px">cat /etc/hostname</code>。
      </p>
      <div style="display:flex;gap:8px;margin:12px 0">
        <input v-model="wlNew" placeholder="输入命令前缀，回车添加，如：ip a" style="flex:1" @keyup.enter="addWl" />
        <button class="btn btn-primary btn-sm" @click="addWl">＋ 添加</button>
      </div>
      <div v-if="wlList.length" style="display:flex;flex-wrap:wrap;gap:8px;margin-bottom:14px">
        <span v-for="(w, i) in wlList" :key="w + i" class="wl-chip">
          <code>{{ w }}</code>
          <button class="wl-x" @click="wlList.splice(i, 1)"><SvgIcon name="x" /> </button>
        </span>
      </div>
      <p v-else style="color:var(--muted);font-size:13px">白名单为空：所有 exec 命令都将被拒绝。</p>
      <div style="display:flex;gap:10px">
        <button class="btn btn-primary" :disabled="saving || !isAdmin" @click="saveWl">{{ saving ? '保存中…' : '保存白名单' }}</button>
        <button class="btn btn-outline" @click="loadWl">重新加载</button>
        <span v-if="!isAdmin" style="color:var(--muted);font-size:12px;align-self:center">仅管理员可修改白名单</span>
      </div>
    </div>

    <div v-if="toast" class="toast">{{ toast }}</div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, watch } from 'vue'
import api from '../api'
import { useSettingsStore, THEME_PRESETS } from '../stores/settings'
import { useAuthStore } from '../stores/auth'

const settings = useSettingsStore()
const auth = useAuthStore()
const isAdmin = computed(() => auth.isAdmin)
const tabs = [
  { key: 'theme', label: '① 主题设置' },
  { key: 'brand', label: '② 系统标识' },
  { key: 'users', label: '③ 用户与权限' },
  { key: 'cmdwl', label: '④ 命令执行白名单' }
]
const tab = ref('theme')
const saving = ref(false)
const toast = ref('')

// 可分配权限的模块清单（与 Layout 侧边栏一致）
const MODULES = [
  { key: 'dashboard', label: '仪表盘' }, { key: 'rooms', label: '机房管理' },
  { key: 'agents', label: 'Agent 在线' }, { key: 'broadcast', label: '广播控制' },
  { key: 'files', label: '文件收发' }, { key: 'remote', label: '远程桌面' },
  { key: 'exam', label: '考试管理' }, { key: 'campuses', label: '多校区管理' },
  { key: 'edusync', label: '教务对接' }, { key: 'access', label: '一卡通/门禁' },
  { key: 'license', label: '授权管理' }, { key: 'ops', label: '运维升级' }
]

function showToast(msg) {
  toast.value = msg
  setTimeout(() => toast.value = '', 2200)
}

// ---------- 主题 ----------
const previewTheme = ref(settings.theme)
const THEME_SWATCH = {
  'light-tech':    { bg: '#F4F7FC', card: '#FFFFFF', accent: '#0066FF', text: '#1A2B4A' },
  'emerald-green': { bg: '#F4F7FC', card: '#FFFFFF', accent: '#00A670', text: '#1A2B4A' },
  'royal-purple':  { bg: '#F4F7FC', card: '#FFFFFF', accent: '#7C3AED', text: '#1A2B4A' },
  'midnight-dark': { bg: '#0E1320', card: '#161E31', accent: '#4D8DFF', text: '#E8EDF8' },
  'obsidian-black': { bg: '#0B0B0E', card: '#16161C', accent: '#F5A623', text: '#EDEDF2' },
  'nebula-purple': { bg: '#131022', card: '#1E1A38', accent: '#9D7BFF', text: '#EBE7FA' }
}
function previewStyle(key) {
  const s = THEME_SWATCH[key] || THEME_SWATCH['light-tech']
  return { '--tp-bg': s.bg, '--tp-card': s.card, '--tp-accent': s.accent, '--tp-text': s.text }
}
watch(previewTheme, v => settings.applyTheme(v))
async function saveTheme() {
  saving.value = true
  try {
    const res = await api.put('/settings', { theme: previewTheme.value })
    if (res.code === 0) {
      settings.updateLocal({ theme: previewTheme.value })
      showToast('主题已保存，对所有用户生效')
    } else showToast(res.msg || '保存失败')
  } catch (e) { showToast(e.response?.data?.msg || '保存失败') }
  finally { saving.value = false }
}

// ---------- 品牌 ----------
const brandTitle = ref(settings.site_title)
const brandLogo = ref(settings.site_logo)
function onLogoPick(e) {
  const f = e.target.files?.[0]
  if (!f) return
  if (f.size > 1024 * 1024) { showToast('Logo 超过 1MB 限制'); return }
  const reader = new FileReader()
  reader.onload = async () => {
    saving.value = true
    try {
      const res = await api.post('/settings/logo', { data: reader.result })
      if (res.code === 0) {
        brandLogo.value = res.data.url
        showToast('Logo 已上传')
      } else showToast(res.msg || '上传失败')
    } catch (err) { showToast(err.response?.data?.msg || '上传失败') }
    finally { saving.value = false; e.target.value = '' }
  }
  reader.readAsDataURL(f)
}
async function removeLogo() {
  brandLogo.value = ''
  await persistSettings({ site_logo: '', site_title: brandTitle.value || '机房教学管理系统' })
}
async function saveBrand() {
  if (!brandTitle.value.trim()) { showToast('标题不能为空'); return }
  await persistSettings({ site_title: brandTitle.value.trim(), site_logo: brandLogo.value })
}
async function persistSettings(kv) {
  saving.value = true
  try {
    const res = await api.put('/settings', kv)
    if (res.code === 0) {
      settings.updateLocal(kv)
      showToast('品牌设置已保存')
    } else showToast(res.msg || '保存失败')
  } catch (e) { showToast(e.response?.data?.msg || '保存失败') }
  finally { saving.value = false }
}

// ---------- 分组 ----------
const groups = ref([])
const groupModal = ref(false)
const editingGroup = reactive({ id: 0, name: '', code: '', description: '' })
const editingPerms = ref([])
const editingPermsAll = ref(false)
const groupErr = ref('')
watch(editingPermsAll, v => { if (v) editingPerms.value = [] })
watch(editingPerms, v => { if (v.length) editingPermsAll.value = false })

function permList(g) {
  try { return JSON.parse(g.permissions || '[]') } catch { return [] }
}
async function loadGroups() {
  const res = await api.get('/groups')
  if (res.code === 0) groups.value = res.data || []
}
function openGroup(g) {
  groupErr.value = ''
  if (g) {
    Object.assign(editingGroup, { id: g.id, name: g.name, code: g.code, description: g.description })
    const perms = permList(g)
    editingPermsAll.value = perms.includes('*')
    editingPerms.value = perms.filter(p => p !== '*')
  } else {
    Object.assign(editingGroup, { id: 0, name: '', code: '', description: '' })
    editingPerms.value = []
    editingPermsAll.value = false
  }
  groupModal.value = true
}
async function saveGroup() {
  const perms = editingPermsAll.value ? ['*'] : editingPerms.value
  if (!editingGroup.name.trim() || !editingGroup.code.trim()) { groupErr.value = '名称与编码必填'; return }
  saving.value = true
  try {
    const payload = { name: editingGroup.name.trim(), description: editingGroup.description.trim(), permissions: JSON.stringify(perms) }
    const res = editingGroup.id
      ? await api.put(`/groups/${editingGroup.id}`, payload)
      : await api.post('/groups', { ...payload, code: editingGroup.code.trim() })
    if (res.code === 0) {
      groupModal.value = false
      await loadGroups()
      showToast(editingGroup.id ? '分组已更新' : '分组已创建')
    } else groupErr.value = res.msg || '保存失败'
  } catch (e) { groupErr.value = e.response?.data?.msg || '保存失败' }
  finally { saving.value = false }
}
async function delGroup(g) {
  if (!confirm(`确定删除分组「${g.name}」？组内用户将变为未分组。`)) return
  const res = await api.delete(`/groups/${g.id}`)
  if (res.code === 0) { await loadGroups(); await loadUsers(); showToast('分组已删除') }
  else showToast(res.msg || '删除失败')
}

// ---------- 用户 ----------
const users = ref([])
const userModal = ref(false)
const editingUser = reactive({ id: 0, username: '', password: '', real_name: '', role: 'teacher', group_id: 0, status: 'active' })
const userErr = ref('')
async function loadUsers() {
  const res = await api.get('/users')
  if (res.code === 0) users.value = res.data || []
}
function openUser(u) {
  userErr.value = ''
  if (u) Object.assign(editingUser, { id: u.id, username: u.username, password: '', real_name: u.real_name || '', role: u.role, group_id: u.group_id || 0, status: u.status })
  else Object.assign(editingUser, { id: 0, username: '', password: '', real_name: '', role: 'teacher', group_id: 0, status: 'active' })
  userModal.value = true
}
async function saveUser() {
  saving.value = true
  try {
    let res
    if (editingUser.id) {
      const payload = { real_name: editingUser.real_name, role: editingUser.role, status: editingUser.status, group_id: editingUser.group_id }
      if (editingUser.password) payload.password = editingUser.password
      res = await api.put(`/users/${editingUser.id}`, payload)
    } else {
      if (!editingUser.username.trim() || editingUser.password.length < 8) {
        userErr.value = '用户名必填，密码至少8位'
        saving.value = false
        return
      }
      res = await api.post('/users', { username: editingUser.username.trim(), password: editingUser.password, real_name: editingUser.real_name, role: editingUser.role, group_id: editingUser.group_id })
    }
    if (res.code === 0) {
      userModal.value = false
      await loadUsers()
      showToast(editingUser.id ? '用户已更新' : '用户已创建')
    } else userErr.value = res.msg || '保存失败'
  } catch (e) { userErr.value = e.response?.data?.msg || '保存失败' }
  finally { saving.value = false }
}
async function delUser(u) {
  if (!confirm(`确定删除用户「${u.username}」？`)) return
  const res = await api.delete(`/users/${u.id}`)
  if (res.code === 0) { await loadUsers(); showToast('用户已删除') }
  else showToast(res.msg || '删除失败')
}

// ============ ④ 命令执行白名单（P0-1） ============
const wlNew = ref('')
const wlList = ref([])
function addWl() {
  const v = wlNew.value.trim()
  if (!v) return
  if (wlList.value.includes(v)) { showToast('该条目已存在'); return }
  wlList.value.push(v)
  wlNew.value = ''
}
async function loadWl() {
  try {
    const res = await api.get('/system/cmd-whitelist')
    wlList.value = (res.data && res.data.whitelist) || []
  } catch (e) { /* 静默：首次可能无权限或网络抖动 */ }
}
async function saveWl() {
  saving.value = true
  try {
    const res = await api.put('/system/cmd-whitelist', { whitelist: wlList.value })
    if (res.code === 0) {
      wlList.value = (res.data && res.data.whitelist) || wlList.value
      showToast('执行白名单已保存')
    } else showToast(res.msg || '保存失败')
  } catch (e) { showToast(e.response?.data?.msg || '保存失败') }
  finally { saving.value = false }
}

onMounted(async () => {
  if (!settings.loaded) await settings.load()
  previewTheme.value = settings.theme
  brandTitle.value = settings.site_title
  brandLogo.value = settings.site_logo
  await Promise.all([loadGroups(), loadUsers(), loadWl()])
})
</script>

<style scoped>
.set-tabs { display: flex; gap: 4px; }
.set-tab {
  padding: 12px 20px; border: none; background: none; cursor: pointer;
  font-size: 13px; font-family: inherit; color: var(--text2);
  border-bottom: 2px solid transparent; transition: all .18s;
}
.set-tab:hover { color: var(--accent); }
.set-tab.active { color: var(--accent); font-weight: 600; border-bottom-color: var(--accent); }

.theme-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 14px; }
.theme-card {
  border: 2px solid var(--border); border-radius: 10px; overflow: hidden; cursor: pointer;
  transition: all .2s; background: var(--card2);
}
.theme-card:hover { transform: translateY(-2px); border-color: var(--border2); }
.theme-card.active { border-color: var(--accent); box-shadow: 0 4px 16px var(--accent-glow); }
.theme-preview {
  position: relative; height: 92px; background: var(--tp-bg);
  border-bottom: 1px solid var(--border);
}
.tp-bar { position: absolute; top: 0; left: 0; right: 0; height: 14px; background: var(--tp-card); border-bottom: 1px solid var(--tp-accent); }
.tp-side { position: absolute; top: 14px; left: 0; bottom: 0; width: 34px; background: var(--tp-card); border-right: 1px solid var(--tp-accent); opacity: .7; }
.tp-body { position: absolute; top: 24px; left: 44px; right: 12px; }
.tp-line { height: 7px; border-radius: 4px; background: var(--tp-accent); opacity: .55; margin-bottom: 7px; }
.tp-line.short { width: 60%; opacity: .3; }
.theme-meta { padding: 10px 12px; }
.theme-meta b { font-size: 13px; }
.theme-meta p { font-size: 11px; color: var(--text2); margin-top: 3px; }

.form-row { margin-bottom: 14px; }
.form-row label { display: block; font-size: 12px; color: var(--text2); margin-bottom: 5px; font-weight: 600; }
.form-row input, .form-row select { width: 100%; max-width: 420px; }
.logo-up { display: flex; gap: 16px; align-items: flex-start; }
.logo-preview {
  width: 120px; height: 64px; border: 1px dashed var(--border2); border-radius: 8px;
  display: flex; align-items: center; justify-content: center; background: var(--card2);
  font-size: 11px; text-align: center; flex-shrink: 0;
}
.logo-preview img { max-height: 48px; max-width: 104px; object-fit: contain; }

.perm-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 6px; }
.perm-item { display: flex; align-items: center; gap: 5px; font-size: 12px; color: var(--text); cursor: pointer; font-weight: 400 !important; }
.perm-item input { width: auto; }

.modal-mask {
  position: fixed; inset: 0; background: rgba(10, 20, 45, .45); z-index: 200;
  display: flex; align-items: center; justify-content: center; backdrop-filter: blur(3px);
}
.modal {
  background: var(--card); border: 1px solid var(--border); border-radius: 12px;
  padding: 24px; width: 520px; max-width: 92vw; max-height: 86vh; overflow-y: auto;
  box-shadow: 0 16px 48px rgba(10, 20, 45, .25); animation: fadeUp .25s ease both;
}
.toast {
  position: fixed; bottom: 28px; left: 50%; transform: translateX(-50%);
  background: var(--text); color: var(--card); padding: 9px 22px; border-radius: 20px;
  font-size: 13px; z-index: 300; box-shadow: var(--shadow); animation: toastIn .25s ease both;
}
@keyframes toastIn { from { opacity: 0; } to { opacity: 1; } }
@media (max-width: 768px) { .perm-grid { grid-template-columns: repeat(2, 1fr); } }

/* P0-1 白名单 chip */
.wl-chip {
  display: inline-flex; align-items: center; gap: 6px;
  background: var(--card2); border: 1px solid var(--border2); border-radius: 6px;
  padding: 4px 8px 4px 10px; font-size: 12px; color: var(--text);
}
.wl-chip code { font-size: 12px; color: var(--accent); background: none; }
.wl-x {
  border: none; background: none; cursor: pointer; color: var(--muted);
  font-size: 12px; line-height: 1; padding: 2px 4px; border-radius: 4px;
}
.wl-x:hover { color: var(--red); background: rgba(220, 60, 60, .12); }
</style>
