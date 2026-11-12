<template>
  <div>
    <div class="card">
      <h3 style="margin-top:0"><SvgIcon name="key" /> 授权状态</h3>
      <div v-if="st" class="lic-status" :class="st.licensed ? 'lic-on' : 'lic-off'">
        <template v-if="st.licensed">
          <div class="lic-row"><span class="lic-label">授权客户</span><b>{{ st.customer }}</b></div>
          <div class="lic-row"><span class="lic-label">到期时间</span>{{ fmt(st.expires_at) }}（剩余 {{ st.days_left }} 天）</div>
          <div class="lic-row"><span class="lic-label">终端授权</span>{{ st.device_count }} / {{ st.max_devices }} 台
            <span :class="['tag', st.device_count > st.max_devices ? 'tag-off' : 'tag-on']" style="margin-left:8px">{{ st.device_count > st.max_devices ? '已超额' : '正常' }}</span>
          </div>
          <div class="lic-row"><span class="lic-label">授权模块</span><span v-for="m in (st.modules || [])" :key="m" class="tag" style="margin-right:6px">{{ modName(m) }}</span></div>
          <div class="lic-row"><span class="lic-label">状态</span><b :style="{ color: st.days_left <= 30 ? 'var(--orange)' : 'var(--green)' }">{{ st.status }}</b></div>
        </template>
        <template v-else>
          <b>{{ st.status }}</b>
          <div style="color:var(--text2);margin-top:8px">当前终端数：{{ st.device_count }} 台（演示模式不限容量）</div>
        </template>
      </div>
      <div class="toolbar" style="margin-top:12px">
        <button class="btn" @click="load"><SvgIcon name="refresh" /> 刷新状态</button>
      </div>
    </div>

    <div class="card">
      <h3 style="margin-top:0"><SvgIcon name="download" /> 导入授权文件</h3>
      <div class="hint">粘贴东青签发的 license.json 内容，或选择文件导入。导入前会校验签名，篡改即失效。</div>
      <textarea class="input lic-text" v-model="uploadText" rows="6" placeholder='{ "customer": "...", "signature": "..." }'></textarea>
      <div class="toolbar" style="margin-top:8px">
        <input type="file" accept=".json" @change="onFile" style="display:none" ref="fileInput">
        <button class="btn" @click="$refs.fileInput.click()"><SvgIcon name="folder" /> 选择文件</button>
        <button class="btn btn-primary" @click="upload">导入</button>
      </div>
    </div>

    <div class="card">
      <h3 style="margin-top:0"><SvgIcon name="pen" /> 签发授权（授权方使用）</h3>
      <div class="form-row"><label>客户名称 *</label><input class="input" v-model="issue.customer" placeholder="如 XX 职业技术学院"></div>
      <div class="form-row"><label>终端上限</label><input class="input" v-model.number="issue.max_devices" placeholder="默认 200"></div>
      <div class="form-row"><label>有效期</label>
        <select class="input" v-model="issue.years">
          <option :value="0.5">6 个月</option>
          <option :value="1">1 年</option>
          <option :value="2">2 年</option>
          <option :value="3">3 年</option>
        </select>
      </div>
      <div class="form-row"><label>授权模块</label>
        <label v-for="m in allModules" :key="m.key" class="mod-check">
          <input type="checkbox" v-model="m.on"> {{ m.label }}
        </label>
      </div>
      <div class="form-row"><label>备注</label><input class="input" v-model="issue.remark"></div>
      <div class="toolbar">
        <button class="btn btn-primary" @click="doIssue">签发并下载 license.json</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import api from '../api'

const st = ref(null)
const uploadText = ref('')
const issue = ref({ customer: '', max_devices: 200, years: 1, remark: '' })
const allModules = reactive([
  { key: 'teaching', label: '教学', on: true },
  { key: 'exam', label: '考试', on: true },
  { key: 'access', label: '门禁', on: true },
  { key: 'campus', label: '多校区', on: true },
  { key: 'ops', label: '运维', on: true }
])

async function load() {
  const res = await api.get('/license')
  st.value = res.data
}

async function upload() {
  if (!uploadText.value.trim()) { alert('请粘贴或选择 license 文件'); return }
  let lic
  try { lic = JSON.parse(uploadText.value) } catch { alert('JSON 格式错误'); return }
  try {
    const res = await api.post('/license', { license: lic })
    alert(`导入成功：${res.data.customer}，剩余 ${res.data.days_left} 天`)
    load()
  } catch (e) { alert(e.response?.data?.msg || '导入失败') }
}

function onFile(ev) {
  const f = ev.target.files[0]
  if (!f) return
  const reader = new FileReader()
  reader.onload = () => { uploadText.value = reader.result }
  reader.readAsText(f)
}

async function doIssue() {
  if (!issue.value.customer) { alert('客户名称必填'); return }
  const exp = new Date()
  exp.setMonth(exp.getMonth() + Math.round(issue.value.years * 12))
  const body = {
    customer: issue.value.customer,
    max_devices: issue.value.max_devices || 200,
    expires_at: exp.toISOString(),
    modules: allModules.filter(m => m.on).map(m => m.key),
    remark: issue.value.remark
  }
  try {
    const res = await api.post('/license/issue', body)
    const blob = new Blob([JSON.stringify(res.data, null, 2)], { type: 'application/json' })
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = 'license.json'
    a.click()
    URL.revokeObjectURL(a.href)
  } catch (e) { alert(e.response?.data?.msg || '签发失败') }
}

function modName(k) {
  return { teaching: '教学', exam: '考试', access: '门禁', campus: '多校区', ops: '运维' }[k] || k
}

function fmt(t) {
  if (!t) return '-'
  return new Date(t).toLocaleDateString('zh-CN')
}

onMounted(load)
</script>

<style scoped>
.lic-status { border-radius: 8px; padding: 16px; }
.lic-on { background: rgba(0,179,104,.13); border: 1px solid rgba(0,179,104,.28); }
.lic-off { background: rgba(245,158,11,.12); border: 1px solid rgba(245,158,11,.30); }
.lic-row { margin: 6px 0; }
.lic-label { display: inline-block; width: 90px; color: var(--text2); }
.lic-text { width: 100%; font-family: monospace; font-size: 12px; box-sizing: border-box; }
.form-row { margin-bottom: 12px; }
.form-row > label:first-child { display: inline-block; width: 90px; color: var(--text2); }
.input { border: 1px solid var(--border2); border-radius: 4px; padding: 6px 8px; }
.mod-check { margin-right: 16px; cursor: pointer; }
.hint { color: var(--text2); font-size: 13px; margin-bottom: 8px; }
</style>
