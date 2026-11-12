<template>
  <div>
    <div class="card">
      <div class="toolbar">
        <button class="btn btn-primary" @click="openCreate">＋ 新增学生</button>
        <ImportButton endpoint="/students" :sample-headers="['sno','name','clazz','card_no']"
          :map-row="(cols, h) => ({ sno: h.sno || cols[0], name: h.name || cols[1], clazz_id: h.clazz ? (clazzes.find(c => c.name === h.clazz)?.id || null) : null, card_no: h.card_no || '' })"
          @done="search" />
        <button v-if="selected.size" class="btn btn-danger" @click="batchDel"><SvgIcon name="trash" /> 批量删除 ({{ selected.size }})</button>
        <button class="btn" @click="exportCSV"><SvgIcon name="download" /> 导出</button>
        <input class="input" v-model="searchKeyword" style="width:200px" placeholder="搜索学号/姓名" @keyup.enter="search">
        <select class="input" v-model="searchClazz" @change="search" style="width:180px">
          <option value="">全部班级</option>
          <option v-for="c in clazzes" :key="c.id" :value="c.id">{{ c.name }}</option>
        </select>
        <button class="btn" @click="search"><SvgIcon name="search" /> 搜索</button>
        <button class="btn" @click="load"><SvgIcon name="refresh" /> 刷新</button>
      </div>
      <table class="data-table">
        <thead>
          <tr><th style="width:40px"><input type="checkbox" @change="toggleAll" :checked="allSelected"></th><th>ID</th><th>学号</th><th>姓名</th><th>班级</th><th>卡号</th><th>创建时间</th><th style="width:200px">操作</th></tr>
        </thead>
        <tbody>
          <tr v-for="s in list" :key="s.id">
            <td><input type="checkbox" :checked="selected.has(s.id)" @change="toggleOne(s.id)"></td>
            <td>{{ s.id }}</td>
            <td>{{ s.sno }}</td>
            <td>{{ s.name }}</td>
            <td>{{ clazzName(s.clazz_id) }}</td>
            <td>{{ s.card_no || '-' }}</td>
            <td>{{ fmtTime(s.created_at) }}</td>
            <td>
              <button class="btn btn-sm" @click="openEdit(s)"><SvgIcon name="pen" /> 编辑</button>
              <button class="btn btn-sm btn-danger" @click="del(s)"><SvgIcon name="trash" /> 删除</button>
            </td>
          </tr>
          <tr v-if="!list.length"><td colspan="7" class="empty">暂无学生</td></tr>
        </tbody>
      </table>
      <Pagination :page="page" :total="total" :page-size="pageSize" @change="p => { page = p; search() }" @size-change="s => { pageSize = s; page = 1; search() }" />
    </div>

    <div class="modal-mask" v-if="showModal" @click.self="showModal=false">
      <div class="modal-card">
        <h3>{{ editing ? '编辑学生' : '新增学生' }}</h3>
        <div class="form-row"><label>学号 *</label><input class="input" v-model="form.sno" placeholder="如：2024001"></div>
        <div class="form-row"><label>姓名 *</label><input class="input" v-model="form.name" placeholder="如：张三"></div>
        <div class="form-row"><label>班级</label>
          <select class="input" v-model.number="form.clazz_id">
            <option :value="null">— 不分配 —</option>
            <option v-for="c in clazzes" :key="c.id" :value="c.id">{{ c.name }}</option>
          </select>
        </div>
        <div class="form-row"><label>卡号</label><input class="input" v-model="form.card_no" placeholder="一卡通卡号"></div>
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
const clazzes = ref([])
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const searchKeyword = ref('')
const searchClazz = ref('')
const showModal = ref(false)
const editing = ref(null)
const saving = ref(false)
const form = ref({ sno: '', name: '', clazz_id: null, card_no: '' })
const selected = ref(new Set())
const allSelected = computed(() => list.value.length > 0 && list.value.every(s => selected.value.has(s.id)))

function toggleOne(id) {
  const s = new Set(selected.value)
  s.has(id) ? s.delete(id) : s.add(id)
  selected.value = s
}
function toggleAll(e) {
  selected.value = e.target.checked ? new Set(list.value.map(s => s.id)) : new Set()
}

async function batchDel() {
  if (!confirm(`确定删除选中的 ${selected.value.size} 名学生？`)) return
  const ids = [...selected.value]
  let ok = 0
  for (const id of ids) {
    try { await api.delete(`/students/${id}`); ok++ } catch {}
  }
  selected.value = new Set()
  alert(`删除完成：${ok}/${ids.length} 成功`)
  search()
}

function exportCSV() {
  const headers = ['sno', 'name', 'clazz_id', 'card_no']
  const rows = list.value.map(s => ({
    sno: s.sno, name: s.name,
    clazz_id: clazzName(s.clazz_id),
    card_no: s.card_no || ''
  }))
  downloadCSV('students.csv', headers, rows)
}

async function loadClazzes() {
  const res = await api.get('/clazzes')
  clazzes.value = res.data || []
}

async function search() {
  selected.value = new Set()
  const params = { keyword: searchKeyword.value, page: page.value, page_size: pageSize.value }
  if (searchClazz.value) params.clazz_id = searchClazz.value
  const res = await api.get('/students', { params })
  const d = res.data || {}
  list.value = d.items || d || []
  total.value = d.total || 0
}

async function load() { searchKeyword.value = ''; searchClazz.value = ''; page.value = 1; search() }

function clazzName(id) {
  if (!id) return '-'
  const c = clazzes.value.find(x => x.id === id)
  return c ? c.name : '班级#' + id
}

function openCreate() { editing.value = null; form.value = { sno: '', name: '', clazz_id: null, card_no: '' }; showModal.value = true }
function openEdit(s) { editing.value = s; form.value = { sno: s.sno, name: s.name, clazz_id: s.clazz_id || null, card_no: s.card_no || '' }; showModal.value = true }

async function save() {
  if (!form.value.sno || !form.value.name) return alert('学号和姓名为必填')
  saving.value = true
  try {
    const payload = { sno: form.value.sno, name: form.value.name, clazz_id: form.value.clazz_id || null, card_no: form.value.card_no }
    if (editing.value) {
      await api.put(`/students/${editing.value.id}`, payload)
    } else {
      await api.post('/students', payload)
    }
    showModal.value = false
    search()
  } catch (e) { alert('保存失败: ' + (e.response?.data?.msg || e.message)) }
  finally { saving.value = false }
}

async function del(s) {
  if (!confirm(`确定删除学生「${s.name}(${s.sno})」？`)) return
  await api.delete(`/students/${s.id}`)
  search()
}

function fmtTime(t) { return t ? new Date(t).toLocaleDateString('zh-CN') : '-' }

onMounted(() => { loadClazzes(); search() })
</script>