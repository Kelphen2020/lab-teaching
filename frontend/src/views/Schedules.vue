<template>
  <div>
    <div class="card">
      <div class="toolbar">
        <button class="btn btn-primary" @click="openCreate">＋ 新增排课</button>
        <select class="input" v-model="filterClazz" @change="load" style="width:180px">
          <option value="">全部班级</option>
          <option v-for="c in clazzes" :key="c.id" :value="c.id">{{ c.name }}</option>
        </select>
        <select class="input" v-model="filterCourse" @change="load" style="width:180px">
          <option value="">全部课程</option>
          <option v-for="c in courses" :key="c.id" :value="c.id">{{ c.name }}</option>
        </select>
        <select class="input" v-model="filterWeekday" @change="load" style="width:120px">
          <option value="">全部星期</option>
          <option v-for="d in 7" :key="d" :value="d">周{{ '一二三四五六日'[d-1] }}</option>
        </select>
        <button class="btn" @click="load"><SvgIcon name="refresh" /> 刷新</button>
        <button class="btn" :class="{ 'btn-primary': viewMode === 'week' }" @click="viewMode = viewMode === 'table' ? 'week' : 'table'"><SvgIcon :name="viewMode === 'table' ? 'cal' : 'clipboard'" /> {{ viewMode === 'table' ? '周视图' : '列表' }}</button>
      </div>

      <!-- 列表视图 -->
      <table class="data-table" v-if="viewMode === 'table'">
        <thead>
          <tr><th>ID</th><th>课程</th><th>班级</th><th>机房</th><th>教师</th><th>星期</th><th>节次</th><th>周范围</th><th>状态</th><th style="width:200px">操作</th></tr>
        </thead>
        <tbody>
          <tr v-for="s in list" :key="s.id">
            <td>{{ s.id }}</td>
            <td>{{ courseName(s.course_id) }}</td>
            <td>{{ clazzName(s.clazz_id) }}</td>
            <td>{{ roomName(s.room_id) }}</td>
            <td>{{ s.teacher_id || '-' }}</td>
            <td>周{{ '一二三四五六日'[s.weekday-1] || s.weekday }}</td>
            <td>{{ s.period }}</td>
            <td>{{ s.week_range || '-' }}</td>
            <td><span :class="['badge', s.status === '正常' ? 'badge-green' : 'badge-gray']">{{ s.status }}</span></td>
            <td>
              <button class="btn btn-sm" @click="openEdit(s)"><SvgIcon name="pen" /> 编辑</button>
              <button class="btn btn-sm btn-danger" @click="del(s)"><SvgIcon name="trash" /> 删除</button>
            </td>
          </tr>
          <tr v-if="!list.length"><td colspan="10" class="empty">暂无排课记录</td></tr>
        </tbody>
      </table>

      <!-- 周视图网格 -->
      <div class="week-grid" v-else>
        <div class="week-header">
          <div class="week-time-col"></div>
          <div class="week-day-col" v-for="d in 7" :key="d">周{{ '一二三四五六日'[d-1] }}</div>
        </div>
        <div class="week-body">
          <div class="week-row" v-for="p in periods" :key="p">
            <div class="week-time-col">第{{ p }}节</div>
            <div class="week-cell" v-for="d in 7" :key="d"
              :class="{ 'has-slot': weekSlots[d]?.[p] }">
              <div v-if="weekSlots[d]?.[p]" class="slot-item" :class="'slot-' + (weekSlots[d][p].status || '正常')"
                @click="openEdit(weekSlots[d][p])">
                <div class="slot-name">{{ courseName(weekSlots[d][p].course_id) }}</div>
                <div class="slot-info">{{ roomName(weekSlots[d][p].room_id) }} · {{ clazzName(weekSlots[d][p].clazz_id) }}</div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="modal-mask" v-if="showModal" @click.self="showModal=false">
      <div class="modal-card">
        <h3>{{ editing ? '编辑排课' : '新增排课' }}</h3>
        <div class="form-row"><label>课程 *</label>
          <select class="input" v-model.number="form.course_id">
            <option :value="0">— 请选择 —</option>
            <option v-for="c in courses" :key="c.id" :value="c.id">{{ c.name }}</option>
          </select>
        </div>
        <div class="form-row"><label>班级 *</label>
          <select class="input" v-model.number="form.clazz_id">
            <option :value="0">— 请选择 —</option>
            <option v-for="c in clazzes" :key="c.id" :value="c.id">{{ c.name }}</option>
          </select>
        </div>
        <div class="form-row"><label>机房 *</label>
          <select class="input" v-model.number="form.room_id">
            <option :value="0">— 请选择 —</option>
            <option v-for="r in rooms" :key="r.id" :value="r.id">{{ r.name }}</option>
          </select>
        </div>
        <div class="form-row"><label>星期 *</label>
          <select class="input" v-model.number="form.weekday">
            <option :value="0">— 请选择 —</option>
            <option v-for="d in 7" :key="d" :value="d">周{{ '一二三四五六日'[d-1] }}</option>
          </select>
        </div>
        <div class="form-row"><label>节次 *</label><input class="input" v-model="form.period" placeholder="如：1-2, 3-4"></div>
        <div class="form-row"><label>周范围</label><input class="input" v-model="form.week_range" placeholder="如：1-16"></div>
        <div class="form-row"><label>状态</label>
          <select class="input" v-model="form.status">
            <option value="正常">正常</option>
            <option value="停课">停课</option>
            <option value="调课">调课</option>
          </select>
        </div>
        <div class="modal-foot">
          <button class="btn" @click="showModal=false">取消</button>
          <button class="btn btn-primary" @click="save" :disabled="saving">{{ saving ? '保存中…' : '保存' }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import api from '../api'

const list = ref([])
const clazzes = ref([])
const courses = ref([])
const rooms = ref([])
const viewMode = ref('table')
const filterClazz = ref('')
const filterCourse = ref('')
const filterWeekday = ref('')
const showModal = ref(false)
const editing = ref(null)
const saving = ref(false)
const form = ref({ course_id: 0, clazz_id: 0, room_id: 0, weekday: 0, period: '', week_range: '', status: '正常' })

const periods = ['1-2', '3-4', '5-6', '7-8', '9-10']
const weekSlots = computed(() => {
  const grid = {}
  for (const s of list.value) {
    const day = s.weekday
    const slot = s.period || '1-2'
    if (!grid[day]) grid[day] = {}
    grid[day][slot] = s
  }
  return grid
})

async function loadRefs() {
  const [rc, rco, rr] = await Promise.all([
    api.get('/clazzes'), api.get('/courses'), api.get('/rooms')
  ])
  clazzes.value = rc.data || []
  courses.value = rco.data || []
  rooms.value = rr.data || []
}

async function load() {
  const params = {}
  if (filterClazz.value) params.clazz_id = filterClazz.value
  if (filterCourse.value) params.course_id = filterCourse.value
  if (filterWeekday.value) params.weekday = filterWeekday.value
  const res = await api.get('/schedules', { params })
  list.value = res.data || []
}

function courseName(id) { return (courses.value.find(x => x.id === id) || {}).name || '课程#' + id }
function clazzName(id) { return (clazzes.value.find(x => x.id === id) || {}).name || '班级#' + id }
function roomName(id) { return (rooms.value.find(x => x.id === id) || {}).name || '机房#' + id }

function openCreate() {
  editing.value = null
  form.value = { course_id: 0, clazz_id: 0, room_id: 0, weekday: 0, period: '', week_range: '', status: '正常' }
  showModal.value = true
}
function openEdit(s) {
  editing.value = s
  form.value = { course_id: s.course_id, clazz_id: s.clazz_id, room_id: s.room_id, weekday: s.weekday, period: s.period || '', week_range: s.week_range || '', status: s.status || '正常' }
  showModal.value = true
}

async function save() {
  const f = form.value
  if (!f.course_id || !f.clazz_id || !f.room_id || !f.weekday || !f.period) return alert('课程、班级、机房、星期、节次为必填')
  saving.value = true
  try {
    const payload = { ...f }
    if (editing.value) {
      await api.put(`/schedules/${editing.value.id}`, payload)
    } else {
      await api.post('/schedules', payload)
    }
    showModal.value = false
    load()
  } catch (e) { alert('保存失败: ' + (e.response?.data?.msg || e.message)) }
  finally { saving.value = false }
}

async function del(s) {
  if (!confirm(`确定删除排课记录 #${s.id}？`)) return
  await api.delete(`/schedules/${s.id}`)
  load()
}

onMounted(() => { loadRefs(); load() })
</script>

<style scoped>
.week-grid { border: 1px solid var(--border2); border-radius: 8px; overflow: hidden; margin-top: 8px; }
.week-header { display: flex; background: var(--bg); border-bottom: 1px solid var(--border2); }
.week-time-col { width: 72px; flex-shrink: 0; padding: 8px 4px; text-align: center; font-size: 12px; color: var(--muted); font-weight: 600; }
.week-day-col { flex: 1; padding: 10px 4px; text-align: center; font-size: 13px; font-weight: 600; border-left: 1px solid var(--border2); }
.week-row { display: flex; border-bottom: 1px solid var(--border); min-height: 72px; }
.week-row:last-child { border-bottom: none; }
.week-cell { flex: 1; border-left: 1px solid var(--border); padding: 4px; }
.week-cell.has-slot { background: rgba(59,130,246,.04); }
.slot-item { background: var(--card); border-radius: 6px; padding: 6px 8px; height: 100%; cursor: pointer; border: 1px solid var(--border2); transition: border-color .15s; }
.slot-item:hover { border-color: var(--accent); }
.slot-停课 { opacity: .5; }
.slot-调课 { border-color: var(--orange); background: rgba(245,158,11,.06); }
.slot-name { font-size: 13px; font-weight: 600; }
.slot-info { font-size: 11px; color: var(--muted); margin-top: 2px; }
</style>