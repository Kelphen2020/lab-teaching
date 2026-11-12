<template>
  <div class="card">
    <div class="toolbar">
      <input v-model.trim="keyword" placeholder="搜索动作/目标/IP" style="width:240px" @keyup.enter="search" />
      <button class="btn btn-primary" :disabled="loading" @click="search">查询</button>
      <button class="btn" @click="reset">重置</button>
      <button class="btn" @click="exportCSV"><SvgIcon name="download" /> 导出 CSV</button>
      <span class="audit-tip">管理操作自动留痕，数据只读</span>
    </div>

    <ListState :state="state" error-text="审计日志加载失败" @retry="load">
      <table>
        <thead>
          <tr><th style="width:60px">ID</th><th>时间</th><th>用户</th><th>动作</th><th>对象</th><th>IP</th><th>详情</th></tr>
        </thead>
        <tbody>
          <tr v-if="!list.length"><td colspan="7" class="empty-cell">暂无审计记录</td></tr>
          <tr v-for="a in list" :key="a.id">
            <td>{{ a.id }}</td>
            <td class="mono">{{ fmtTime(a.created_at) }}</td>
            <td>{{ a.user_id }}</td>
            <td><span class="tag" :class="methodClass(a.action)">{{ a.action }}</span></td>
            <td class="mono">{{ a.target || '—' }}</td>
            <td class="mono">{{ a.ip || '—' }}</td>
            <td class="detail-cell" :title="a.detail">{{ a.detail || '—' }}</td>
          </tr>
        </tbody>
      </table>
    </ListState>

    <Pagination :page="page" :page-size="size" :total="total" @update:page="p => { page = p; load() }" />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '../api'
import { useAuthStore } from '../stores/auth'
import ListState from '../components/ListState.vue'
import Pagination from '../components/Pagination.vue'

const auth = useAuthStore()

const list = ref([])
const keyword = ref('')
const page = ref(1)
const size = ref(20)
const total = ref(0)
const loading = ref(false)
const state = ref('loading')

async function load() {
  loading.value = true
  state.value = 'loading'
  try {
    const res = await api.get('/audit/logs', { params: { q: keyword.value, page: page.value, size: size.value } })
    const d = res.data || {}
    list.value = d.list || []
    total.value = d.total || 0
    state.value = list.value.length ? 'ready' : 'empty'
  } catch {
    state.value = 'error'
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  load()
}

function reset() {
  keyword.value = ''
  page.value = 1
  load()
}

async function exportCSV() {
  const params = new URLSearchParams()
  if (keyword.value) params.set('q', keyword.value)
  const url = `/api/v1/audit/logs/export?${params.toString()}`
  const resp = await fetch(url, { headers: { Authorization: 'Bearer ' + auth.token } })
  const blob = await resp.blob()
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = 'audit_logs.csv'
  a.click()
  URL.revokeObjectURL(a.href)
}

function fmtTime(t) {
  if (!t) return '—'
  return String(t).replace('T', ' ').slice(0, 19)
}

function methodClass(action) {
  const a = String(action || '').toUpperCase()
  if (a.includes('DELETE') || a.includes('删除')) return 'tag-red'
  if (a.includes('POST') || a.includes('新增') || a.includes('创建')) return 'tag-green'
  if (a.includes('PUT') || a.includes('PATCH') || a.includes('更新')) return 'tag-blue'
  return 'tag-on'
}

onMounted(load)
</script>

<style scoped>
.audit-tip { margin-left: auto; font-size: 12px; color: var(--muted); }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; }
.detail-cell { max-width: 320px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 12px; color: var(--text2); }
.tag-red { background: rgba(239,68,68,.12); color: #ef4444; border-color: rgba(239,68,68,.3); }
.tag-green { background: rgba(34,197,94,.12); color: #22c55e; border-color: rgba(34,197,94,.3); }
.tag-blue { background: rgba(59,130,246,.12); color: #3b82f6; border-color: rgba(59,130,246,.3); }
</style>
