<template>
  <span style="display:inline">
    <input type="file" accept=".csv" ref="fileInput" style="display:none" @change="onFile" />
    <button class="btn" @click="$refs.fileInput.click()" :disabled="busy">
      <SvgIcon name="download" /> {{ busy ? `导入中 ${done}/${total}…` : '导入 CSV' }}
    </button>
    <span v-if="result" class="import-result" :class="{ 'import-err': result.fail > 0 }">
      {{ result.msg }}
    </span>
  </span>
</template>

<script setup>
import { ref } from 'vue'
import api from '../api'

const props = defineProps({
  endpoint: { type: String, required: true },       // 创建 API 路径，如 '/rooms'
  sampleHeaders: { type: Array, default: () => [] }, // 示例列名，下载模板用
  mapRow: { type: Function, required: true },        // (cols, headerMap) => payload object
})

const emit = defineEmits(['done'])
const fileInput = ref(null)
const busy = ref(false)
const total = ref(0)
const done = ref(0)
const result = ref(null)

async function onFile(e) {
  const file = e.target.files[0]
  if (!file) return
  result.value = null
  const text = await file.text()
  const lines = text.trim().split(/\r?\n/).filter(l => l.trim())
  if (lines.length < 2) { result.value = { msg: 'CSV 至少需要标题行+1行数据', fail: 1 }; return }

  const headers = lines[0].split(',').map(h => h.trim())
  const rows = lines.slice(1)
  total.value = rows.length
  done.value = 0
  busy.value = true
  let ok = 0, fail = 0

  for (const line of rows) {
    const cols = line.split(',').map(c => c.trim())
    const headerMap = {}
    headers.forEach((h, i) => { if (cols[i] !== undefined) headerMap[h] = cols[i] })
    try {
      const payload = props.mapRow(cols, headerMap)
      if (!payload) { fail++; done.value++; continue }
      await api.post(props.endpoint, payload)
      ok++
    } catch (e) {
      fail++
    }
    done.value++
  }

  busy.value = false
  result.value = { ok, fail, msg: fail > 0 ? `完成：${ok} 成功，${fail} 失败` : `全部 ${ok} 条导入成功` }
  fileInput.value.value = ''
  emit('done')
}

function downloadTemplate() {
  if (!props.sampleHeaders.length) return
  const csv = props.sampleHeaders.join(',') + '\n' + props.sampleHeaders.map(() => '').join(',')
  const blob = new Blob([csv], { type: 'text/csv' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = 'template.csv'
  a.click()
}
</script>

<style scoped>
.import-result { font-size: 12px; margin-left: 8px; color: var(--green); }
.import-result.import-err { color: var(--red); }
</style>