<template>
  <div>
    <div class="card">
      <div class="toolbar">
        <button class="btn btn-primary" @click="openCreate">＋ 新增校区</button>
        <button class="btn" @click="load"><SvgIcon name="refresh" /> 刷新</button>
        <span style="color:var(--text2)">全局看板：{{ global.campus_count || 0 }} 个校区 / {{ global.room_count || 0 }} 个机房 / 终端 {{ global.device_count || 0 }} 台（在线 {{ global.online_count || 0 }}）</span>
      </div>
      <table>
        <thead>
          <tr><th>ID</th><th>校区编码</th><th>名称</th><th>地址</th><th>备注</th><th style="width:280px">操作</th></tr>
        </thead>
        <tbody>
          <tr v-for="c in campuses" :key="c.id">
            <td>{{ c.id }}</td>
            <td>{{ c.code }}</td>
            <td>{{ c.name }}</td>
            <td>{{ c.address || '-' }}</td>
            <td>{{ c.remark || '-' }}</td>
            <td>
              <button class="btn btn-sm" @click="viewCampus(c)"><SvgIcon name="chart" /> 看板</button>
              <button class="btn btn-sm" @click="openEdit(c)"><SvgIcon name="pen" /> 编辑</button>
              <button class="btn btn-sm btn-danger" @click="del(c)"><SvgIcon name="trash" /> 删除</button>
            </td>
          </tr>
          <tr v-if="!campuses.length"><td colspan="6" style="text-align:center;color:var(--muted)">暂无校区，点击「新增校区」创建</td></tr>
        </tbody>
      </table>
    </div>

    <div class="card">
      <h3 style="margin-top:0">机房归属分配</h3>
      <table>
        <thead>
          <tr><th>机房</th><th>座位数</th><th>当前校区</th><th style="width:320px">分配到校区</th></tr>
        </thead>
        <tbody>
          <tr v-for="r in rooms" :key="r.id">
            <td>{{ r.name }}</td>
            <td>{{ r.seat_count }}</td>
            <td>{{ campusName(r.campus_id) || '— 未分配 —' }}</td>
            <td>
              <select v-model="assignMap[r.id]" class="input" style="display:inline-block;width:180px">
                <option :value="0">— 不分配 —</option>
                <option v-for="c in campuses" :key="c.id" :value="c.id">{{ c.name }}</option>
              </select>
              <button class="btn btn-sm btn-primary" @click="assign(r)">应用</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 编辑/新建弹窗 -->
    <div v-if="showForm" class="modal-mask">
      <div class="modal">
        <h3 style="margin-top:0">{{ editing.id ? '编辑校区' : '新增校区' }}</h3>
        <div class="form-row"><label>校区编码 *</label><input class="input" v-model="editing.code" placeholder="如 HQ / EAST"></div>
        <div class="form-row"><label>名称 *</label><input class="input" v-model="editing.name" placeholder="如 东校区"></div>
        <div class="form-row"><label>地址</label><input class="input" v-model="editing.address"></div>
        <div class="form-row"><label>备注</label><input class="input" v-model="editing.remark"></div>
        <div class="modal-actions">
          <button class="btn" @click="showForm = false">取消</button>
          <button class="btn btn-primary" @click="save">保存</button>
        </div>
      </div>
    </div>

    <!-- 看板弹窗 -->
    <div v-if="showDash" class="modal-mask" @click.self="showDash = false">
      <div class="modal" style="width:640px">
        <h3 style="margin-top:0"><SvgIcon name="chart" /> {{ dashCampusName }} — 校区看板</h3>
        <div class="stat-grid">
          <div class="stat"><div class="stat-num">{{ dashboard.room_count || 0 }}</div><div class="stat-label">机房数</div></div>
          <div class="stat"><div class="stat-num">{{ dashboard.device_count || 0 }}</div><div class="stat-label">终端数</div></div>
          <div class="stat"><div class="stat-num">{{ dashboard.online_count || 0 }}</div><div class="stat-label">在线终端</div></div>
        </div>
        <table v-if="dashboard.rooms && dashboard.rooms.length">
          <thead><tr><th>机房</th><th>终端数</th><th>在线</th></tr></thead>
          <tbody>
            <tr v-for="r in dashboard.rooms" :key="r.room_id || r.id">
              <td>{{ r.room_name || r.name }}</td>
              <td>{{ r.device_count }}</td>
              <td>{{ r.online_count }}</td>
            </tr>
          </tbody>
        </table>
        <div class="modal-actions"><button class="btn" @click="showDash = false">关闭</button></div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import api from '../api'

const campuses = ref([])
const rooms = ref([])
const global = ref({})
const assignMap = reactive({})
const showForm = ref(false)
const editing = ref({})
const showDash = ref(false)
const dashboard = ref({})
const dashCampusName = ref('')

async function load() {
  const [cs, rs, gb] = await Promise.all([
    api.get('/campuses'),
    api.get('/rooms'),
    api.get('/dashboard/global').catch(() => ({ data: {} }))
  ])
  campuses.value = cs.data || []
  rooms.value = rs.data || []
  global.value = gb.data || {}
  for (const r of rooms.value) assignMap[r.id] = r.campus_id || 0
}

function campusName(id) {
  const c = campuses.value.find(x => x.id === id)
  return c ? c.name : ''
}

function openCreate() { editing.value = {}; showForm.value = true }
function openEdit(c) { editing.value = { ...c }; showForm.value = true }

async function save() {
  if (!editing.value.code || !editing.value.name) { alert('校区编码与名称必填'); return }
  try {
    if (editing.value.id) await api.put('/campuses/' + editing.value.id, editing.value)
    else await api.post('/campuses', editing.value)
    showForm.value = false
    load()
  } catch (e) { alert(e.response?.data?.msg || '保存失败') }
}

async function del(c) {
  if (!confirm(`确定删除校区「${c.name}」？（需先迁出下属机房）`)) return
  try {
    await api.delete('/campuses/' + c.id)
    load()
  } catch (e) { alert(e.response?.data?.msg || '删除失败') }
}

async function assign(r) {
  try {
    const target = assignMap[r.id]
    if (target) await api.put(`/campuses/${target}/rooms/${r.id}`)
    else await api.delete(`/campuses/rooms/${r.id}`)
    load()
  } catch (e) { alert(e.response?.data?.msg || '分配失败') }
}

async function viewCampus(c) {
  try {
    const res = await api.get(`/campuses/${c.id}/dashboard`)
    dashboard.value = res.data || {}
    dashCampusName.value = c.name
    showDash.value = true
  } catch (e) { alert(e.response?.data?.msg || '看板加载失败') }
}

onMounted(load)
</script>

<style scoped>
.modal-mask { position: fixed; inset: 0; background: rgba(0,0,0,.45); display: flex; align-items: center; justify-content: center; z-index: 100; }
.modal { background: var(--card); border-radius: 8px; padding: 24px; width: 460px; box-shadow: 0 8px 32px rgba(0,0,0,.2); }
.form-row { margin-bottom: 12px; }
.form-row label { display: inline-block; width: 90px; color: var(--text2); }
.input { border: 1px solid var(--border2); border-radius: 4px; padding: 6px 8px; }
.modal-actions { text-align: right; margin-top: 16px; }
.stat-grid { display: flex; gap: 16px; margin-bottom: 16px; }
.stat { flex: 1; background: var(--card2); border-radius: 8px; padding: 12px; text-align: center; border: 1px solid var(--border); }
.stat-num { font-size: 24px; font-weight: 700; background: var(--gradient); -webkit-background-clip: text; background-clip: text; -webkit-text-fill-color: transparent; color: var(--accent); }
.stat-label { color: var(--text2); font-size: 12px; margin-top: 4px; }
</style>
