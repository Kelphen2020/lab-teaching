<template>
  <div>
    <div class="card">
      <div class="toolbar">
        <button class="btn btn-primary" @click="openCreate">＋ 新增课程</button>
        <ImportButton endpoint="/courses" :sample-headers="['name','credit']"
          :map-row="(cols, h) => ({ name: h.name || cols[0], credit: parseFloat(h.credit) || 0 })"
          @done="load" />
        <button v-if="selected.size" class="btn btn-danger" @click="batchDel"><SvgIcon name="trash" /> 批量删除 ({{ selected.size }})</button>
        <button class="btn" @click="exportCSV"><SvgIcon name="download" /> 导出</button>
        <button class="btn" @click="load"><SvgIcon name="refresh" /> 刷新</button>
      </div>
      <table class="data-table">
        <thead>
          <tr><th style="width:40px"><input type="checkbox" @change="toggleAll" :checked="allSelected"></th><th @click="toggleSort('id')" class="sortable">ID {{ sortBy==='id' ? (sortDir==='asc'?'▲':'▼') : '' }}</th><th @click="toggleSort('name')" class="sortable">课程名称 {{ sortBy==='name' ? (sortDir==='asc'?'▲':'▼') : '' }}</th><th @click="toggleSort('credit')" class="sortable">学分 {{ sortBy==='credit' ? (sortDir==='asc'?'▲':'▼') : '' }}</th><th>教师ID</th><th>创建时间</th><th style="width:200px">操作</th></tr>
        </thead>
        <tbody>
          <tr v-for="c in list" :key="c.id">
            <td><input type="checkbox" :checked="selected.has(c.id)" @change="toggleOne(c.id)"></td>
            <td>{{ c.id }}</td>
            <td>{{ c.name }}</td>
            <td>{{ c.credit }}</td>
            <td>{{ c.teacher_id || '-' }}</td>
            <td>{{ fmtTime(c.created_at) }}</td>
            <td>
              <button class="btn btn-sm" @click="openEdit(c)"><SvgIcon name="pen" /> 编辑</button>
              <button class="btn btn-sm btn-danger" @click="del(c)"><SvgIcon name="trash" /> 删除</button>
            </td>
          </tr>
          <tr v-if="!list.length"><td colspan="6" class="empty">暂无课程</td></tr>
        </tbody>
      </table>
      <Pagination :page="page" :total="total" :page-size="pageSize" @change="p => { page = p; load() }" @size-change="s => { pageSize = s; page = 1; load() }" />
    </div>

    <!-- 编辑/新增弹窗 -->
    <div class="modal-mask" v-if="showModal" @click.self="showModal=false">
      <div class="modal-card">
        <h3>{{ editing ? '编辑课程' : '新增课程' }}</h3>
        <div class="form-row"><label>课程名称 *</label><input class="input" v-model="form.name" placeholder="如：高等数学"></div>
        <div class="form-row"><label>学分</label><input class="input" v-model.number="form.credit" type="number" step="0.5" placeholder="如：3.0"></div>
        <div class="form-row"><label>教师ID</label><input class="input" v-model.number="form.teacher_id" type="number" placeholder="用户ID"></div>
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
import ImportButton from '../components/ImportButton.vue'
import Pagination from '../components/Pagination.vue'
import { downloadCSV } from '../utils/export.js'

const list = ref([])
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const sortBy = ref('')
const sortDir = ref('asc')
const showModal = ref(false)
const editing = ref(null)
const saving = ref(false)
const form = ref({ name: '', credit: 0, teacher_id: null })
const selected = ref(new Set())
const allSelected = computed(() => list.value.length > 0 && list.value.every(c => selected.value.has(c.id)))

function toggleOne(id) { const s = new Set(selected.value); s.has(id) ? s.delete(id) : s.add(id); selected.value = s }
function toggleAll(e) { selected.value = e.target.checked ? new Set(list.value.map(c => c.id)) : new Set() }

async function batchDel() {
  if (!confirm(`确定删除选中的 ${selected.value.size} 门课程？`)) return
  const ids = [...selected.value]; let ok = 0
  for (const id of ids) { try { await api.delete(`/courses/${id}`); ok++ } catch {} }
  selected.value = new Set(); alert(`删除完成：${ok}/${ids.length} 成功`); load()
}

function exportCSV() {
  downloadCSV('courses.csv', ['name', 'credit', 'teacher_id'], list.value)
}

function toggleSort(col) {
  if (sortBy.value === col) { sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc' }
  else { sortBy.value = col; sortDir.value = 'asc' }
  load()
}

async function load() {
  selected.value = new Set()
  const params = { page: page.value, page_size: pageSize.value }
  if (sortBy.value) { params.sort_by = sortBy.value; params.sort_dir = sortDir.value }
  const res = await api.get('/courses', { params })
  const d = res.data || {}
  list.value = d.items || d || []
  total.value = d.total || 0
}

function openCreate() { editing.value = null; form.value = { name: '', credit: 0, teacher_id: null }; showModal.value = true }
function openEdit(c) { editing.value = c; form.value = { name: c.name, credit: c.credit, teacher_id: c.teacher_id || null }; showModal.value = true }

async function save() {
  if (!form.value.name) return alert('请输入课程名称')
  saving.value = true
  try {
    const payload = { name: form.value.name, credit: form.value.credit || 0 }
    if (form.value.teacher_id) payload.teacher_id = Number(form.value.teacher_id)
    if (editing.value) {
      await api.put(`/courses/${editing.value.id}`, payload)
    } else {
      await api.post('/courses', payload)
    }
    showModal.value = false
    load()
  } catch (e) { alert('保存失败: ' + (e.response?.data?.msg || e.message)) }
  finally { saving.value = false }
}

async function del(c) {
  if (!confirm(`确定删除课程「${c.name}」？`)) return
  await api.delete(`/courses/${c.id}`)
  load()
}

function fmtTime(t) { return t ? new Date(t).toLocaleDateString('zh-CN') : '-' }

onMounted(load)
</script>

<style scoped>
.sortable { cursor: pointer; user-select: none; }
.sortable:hover { color: var(--accent); }
</style>