<template>
  <div class="api-docs">
    <div class="card">
      <div class="toolbar">
        <h2 style="margin:0"><SvgIcon name="book" /> API 接口文档</h2>
        <span class="muted">OpenAPI 3.0 · 共 {{ endpoints.length }} 个端点</span>
        <div style="flex:1"></div>
        <input class="input" v-model="keyword" placeholder="搜索接口..." style="width:220px" />
        <select class="input" v-model="tagFilter" style="width:150px">
          <option value="">全部分类</option>
          <option v-for="t in tags" :key="t" :value="t">{{ t }}</option>
        </select>
        <button class="btn" @click="load"><SvgIcon name="refresh" /> 刷新</button>
        <a class="btn" :href="specUrl" target="_blank"><SvgIcon name="download" /> openapi.json</a>
      </div>

      <div v-if="state === 'loading'" class="empty-tip">加载中...</div>
      <div v-else-if="state === 'error'" class="empty-tip"><SvgIcon name="x-circle" /> 加载失败：{{ errMsg }}</div>
      <template v-else>
        <div class="doc-info">
          <strong>{{ info.title }}</strong> <span class="muted">v{{ info.version }}</span>
          <p class="muted" style="margin:6px 0 0">{{ info.description }}</p>
        </div>

        <div v-for="t in visibleTags" :key="t" class="tag-section">
          <div class="tag-title">{{ t }} <span class="muted">({{ byTag(t).length }})</span></div>
          <table class="api-table">
            <tr v-for="ep in byTag(t)" :key="ep.method + ep.path"
                class="ep-row" @click="toggle(ep)">
              <td class="ep-method"><span class="m-badge" :class="'m-' + ep.method">{{ ep.method }}</span></td>
              <td class="ep-path"><code>{{ ep.path }}</code></td>
              <td class="ep-summary">{{ ep.summary }}</td>
              <td class="ep-auth"><SvgIcon v-if="ep.auth" name="shield" /><template v-else>公开</template></td>
            </tr>
            <template v-for="ep in byTag(t)" :key="'d' + ep.path">
              <tr v-if="expanded === ep.method + ep.path" class="ep-detail">
                <td colspan="4">
                  <div class="detail-box">
                    <div class="muted">标签：{{ ep.tags.join(' / ') }}</div>
                    <div v-if="ep.body" style="margin-top:8px">
                      <strong>请求体示例</strong>
                      <pre class="code-block">{{ JSON.stringify(ep.bodyExample, null, 2) }}</pre>
                      <div v-if="ep.body.required">
                        <span class="req-chip" v-for="r in ep.body.required" :key="r">必填 {{ r }}</span>
                      </div>
                    </div>
                    <div style="margin-top:8px">
                      <strong>curl 示例</strong>
                      <pre class="code-block">{{ curlExample(ep) }}</pre>
                    </div>
                  </div>
                </td>
              </tr>
            </template>
          </table>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import api from '../api'

const specUrl = '/api/v1/openapi.json'
const state = ref('loading')
const errMsg = ref('')
const keyword = ref('')
const tagFilter = ref('')
const expanded = ref('')
const endpoints = ref([])
const info = ref({ title: '', version: '', description: '' })

const tags = computed(() => {
  const s = new Set()
  endpoints.value.forEach(e => e.tags.forEach(t => s.add(t)))
  return [...s]
})

const filtered = computed(() => {
  let list = endpoints.value
  const kw = keyword.value.trim().toLowerCase()
  if (kw) {
    list = list.filter(e =>
      e.path.toLowerCase().includes(kw) ||
      e.summary.toLowerCase().includes(kw) ||
      e.tags.some(t => t.toLowerCase().includes(kw)))
  }
  if (tagFilter.value) list = list.filter(e => e.tags.includes(tagFilter.value))
  return list
})

const visibleTags = computed(() => {
  const s = new Set()
  filtered.value.forEach(e => e.tags.forEach(t => s.add(t)))
  return [...s]
})

function byTag(t) {
  return filtered.value.filter(e => e.tags.includes(t))
}

function toggle(ep) {
  const key = ep.method + ep.path
  expanded.value = expanded.value === key ? '' : key
}

function curlExample(ep) {
  const base = 'http://服务器:8090/api/v1' + ep.path.replace(/\{(\w+)\}/g, '1')
  let cmd = `curl -s -X ${ep.method}`
  if (ep.auth) cmd += ` \\\n  -H "Authorization: Bearer $TOKEN"`
  if (ep.body && ep.bodyExample) {
    cmd += ` \\\n  -H "Content-Type: application/json" \\\n  -d '${JSON.stringify(ep.bodyExample)}'`
  }
  cmd += ` \\\n  "${base}"`
  return cmd
}

function exampleFromSchema(schema) {
  if (!schema || !schema.properties) return null
  const out = {}
  for (const [k, v] of Object.entries(schema.properties)) {
    if (v.example !== undefined) { out[k] = v.example; continue }
    if (v.type === 'string') out[k] = ''
    else if (v.type === 'integer') out[k] = 1
    else if (v.type === 'boolean') out[k] = false
    else if (v.type === 'array') out[k] = []
    else out[k] = {}
  }
  return out
}

async function load() {
  state.value = 'loading'
  try {
    const res = await api.get('/openapi.json')
    const spec = res.data || res
    info.value = spec.info || {}
    const eps = []
    for (const [path, methods] of Object.entries(spec.paths || {})) {
      for (const [method, op] of Object.entries(methods)) {
        const body = op.requestBody?.content?.['application/json']?.schema
        eps.push({
          method: method.toUpperCase(),
          path,
          summary: op.summary || '',
          tags: op.tags || [],
          auth: !!(op.security && op.security.length),
          body: body || null,
          bodyExample: exampleFromSchema(body),
        })
      }
    }
    eps.sort((a, b) => a.path.localeCompare(b.path))
    endpoints.value = eps
    state.value = 'ready'
  } catch (e) {
    errMsg.value = e.message || String(e)
    state.value = 'error'
  }
}

onMounted(load)
</script>

<style scoped>
.toolbar { display: flex; align-items: center; gap: 10px; margin-bottom: 14px; }
.muted { color: var(--text-dim, #888); font-size: 13px; }
.doc-info { padding: 12px; background: var(--bg-soft, #f6f8fa); border-radius: 8px; margin-bottom: 16px; }
.tag-section { margin-bottom: 20px; }
.tag-title { font-weight: 600; font-size: 15px; margin-bottom: 8px; color: var(--accent, #2563eb); }
.api-table { width: 100%; border-collapse: collapse; }
.api-table tr { border-bottom: 1px solid var(--border, #e5e7eb); }
.ep-row { cursor: pointer; }
.ep-row:hover { background: var(--bg-hover, #f9fafb); }
.ep-method { width: 70px; padding: 8px; }
.ep-path { width: 34%; padding: 8px; }
.ep-path code { font-family: monospace; font-size: 13px; word-break: break-all; }
.ep-summary { padding: 8px; font-size: 13px; }
.ep-auth { width: 60px; text-align: center; font-size: 13px; }
.m-badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 11px; font-weight: 700; color: #fff; font-family: monospace; }
.m-get { background: #10b981; }
.m-post { background: #3b82f6; }
.m-put { background: #f59e0b; }
.m-delete { background: #ef4444; }
.ep-detail td { background: var(--bg-soft, #f6f8fa); padding: 12px; }
.detail-box { font-size: 13px; }
.code-block { background: #1e293b; color: #e2e8f0; padding: 10px; border-radius: 6px; overflow-x: auto; font-size: 12px; margin: 6px 0; }
.req-chip { display: inline-block; background: #fef3c7; color: #92400e; border-radius: 4px; padding: 1px 8px; font-size: 12px; margin: 2px 4px 2px 0; }
.empty-tip { padding: 40px; text-align: center; color: var(--text-dim, #888); }
.input { padding: 6px 10px; border: 1px solid var(--border, #d1d5db); border-radius: 6px; font-size: 13px; }
</style>
