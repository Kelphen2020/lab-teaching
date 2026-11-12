<template>
  <div class="card">
    <div class="toolbar">
      <button class="btn btn-primary" :disabled="state === 'loading'" @click="load"><SvgIcon name="refresh" /> 刷新</button>
      <span class="poll-tip" :class="connected ? 'live' : ''">
        <span class="dot"></span>{{ connected ? '实时推送已连接（新截图自动出现）' : '实时推送未连接' }}
      </span>
      <div style="margin-left:auto;display:flex;gap:8px;align-items:center">
        <select v-model="sourceFilter" @change="page = 1; load()">
          <option value="">全部来源</option>
          <option value="manual">手动截图</option>
          <option value="violation">违纪截图</option>
        </select>
        <input v-model="agentFilter" placeholder="按 Agent ID 过滤" style="width:180px" @keyup.enter="page = 1; load()" />
        <button class="btn btn-outline" @click="page = 1; load()">筛选</button>
      </div>
    </div>

    <ListState :state="state" empty-text="暂无截图" empty-hint="在 Agent 页面点击截图按钮手动截图，或考试违纪时自动截图" error-text="截图列表加载失败" @retry="load">
      <div class="shot-grid">
        <div v-for="s in shots" :key="s.id" class="shot-card" @click="preview = s">
          <img :src="s.url" loading="lazy" alt="截图" />
          <div class="shot-meta">
            <span class="tag" :class="s.source === 'violation' ? 'tag-red' : 'tag-blue'">{{ s.source === 'violation' ? '违纪' : '手动' }}</span>
            <span class="mono">{{ s.agent_id }}</span>
            <span style="color:var(--muted);font-size:11px;margin-left:auto">{{ fmt(s.created_at) }}</span>
          </div>
        </div>
      </div>
    </ListState>

    <Pagination v-if="total > pageSize" :page="page" :page-size="pageSize" :total="total" @change="p => { page = p; load() }" />
  </div>

  <!-- 预览大图 -->
  <div v-if="preview" class="modal-mask" style="z-index:999" @click.self="preview = null">
    <div style="max-width:92vw;max-height:92vh;display:flex;flex-direction:column;gap:8px">
      <div style="display:flex;gap:8px;align-items:center;color:#fff;font-size:13px">
        <span class="tag" :class="preview.source === 'violation' ? 'tag-red' : 'tag-blue'">{{ preview.source === 'violation' ? '违纪截图' : '手动截图' }}</span>
        <span class="mono">{{ preview.agent_id }}</span>
        <span style="opacity:.7">{{ fmt(preview.created_at) }}</span>
        <button class="btn btn-sm" style="margin-left:auto" @click="preview = null"><SvgIcon name="x" /> 关闭</button>
      </div>
      <img :src="preview.url" style="max-width:92vw;max-height:82vh;object-fit:contain;border-radius:8px" />
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import api from '../api'
import ListState from '../components/ListState.vue'
import Pagination from '../components/Pagination.vue'
import { useEventStream } from '../composables/useEventStream'

const state = ref('loading')
const shots = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const sourceFilter = ref('')
const agentFilter = ref('')
const preview = ref(null)

const { connected, on } = useEventStream()
on('screenshot_new', p => {
  if (!p) return
  // 新截图到达：若当前过滤条件匹配则插入顶部
  if (sourceFilter.value && p.source !== sourceFilter.value) return
  if (agentFilter.value && p.agent_id !== agentFilter.value) return
  shots.value.unshift({ id: p.id, agent_id: p.agent_id, source: p.source, url: p.url, created_at: new Date().toISOString() })
  if (shots.value.length > pageSize) shots.value.pop()
  total.value++
})

function fmt(t) {
  if (!t) return '-'
  return String(t).replace('T', ' ').slice(0, 19)
}

async function load() {
  state.value = shots.value.length ? state.value : 'loading'
  try {
    const params = { page: page.value, page_size: pageSize }
    if (sourceFilter.value) params.source = sourceFilter.value
    if (agentFilter.value.trim()) params.agent_id = agentFilter.value.trim()
    const r = await api.get('/screenshots', { params })
    const d = r.data || {}
    shots.value = d.items || []
    total.value = d.total || 0
    state.value = shots.value.length ? 'ready' : 'empty'
  } catch {
    state.value = 'error'
  }
}
onMounted(load)
</script>

<style scoped>
.shot-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 12px; margin-top: 12px; }
.shot-card { border: 1px solid var(--border2); border-radius: 8px; overflow: hidden; cursor: pointer; transition: box-shadow .15s, transform .15s; background: var(--card); }
.shot-card:hover { box-shadow: 0 4px 16px rgba(0,0,0,.12); transform: translateY(-2px); }
.shot-card img { width: 100%; aspect-ratio: 16/10; object-fit: cover; display: block; background: #1a1d24; }
.shot-meta { display: flex; gap: 6px; align-items: center; padding: 7px 9px; font-size: 12px; }
.mono { font-family: monospace; font-size: 12px; }
</style>
