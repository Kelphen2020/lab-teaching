<template>
  <div class="pager" v-if="totalPages > 1 || total > 0">
    <span class="pg-info">每页</span>
    <select class="pg-size" :value="pageSize" @change="$emit('sizeChange', Number($event.target.value))">
      <option v-for="s in [10, 20, 50, 100]" :key="s" :value="s">{{ s }}</option>
    </select>
    <span class="pg-info">条</span>
    <button class="pg-btn" :disabled="page <= 1" @click="$emit('change', page - 1)">‹ 上一页</button>
    <template v-for="p in visiblePages" :key="p">
      <span v-if="p === '...'" class="pg-ellipsis">…</span>
      <button v-else class="pg-btn" :class="{ active: p === page }" @click="$emit('change', p)">{{ p }}</button>
    </template>
    <button class="pg-btn" :disabled="page >= totalPages" @click="$emit('change', page + 1)">下一页 ›</button>
    <span class="pg-info">共 {{ total }} 条</span>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  page: { type: Number, required: true },
  total: { type: Number, required: true },
  pageSize: { type: Number, default: 20 },
})

defineEmits(['change', 'sizeChange'])

const totalPages = computed(() => Math.ceil(props.total / props.pageSize) || 1)

const visiblePages = computed(() => {
  const tp = totalPages.value
  const p = props.page
  if (tp <= 7) return Array.from({ length: tp }, (_, i) => i + 1)
  const pages = []
  if (p <= 3) { pages.push(1, 2, 3, 4, '...', tp) }
  else if (p >= tp - 2) { pages.push(1, '...', tp - 3, tp - 2, tp - 1, tp) }
  else { pages.push(1, '...', p - 1, p, p + 1, '...', tp) }
  return pages
})
</script>

<style scoped>
.pager { display: flex; align-items: center; gap: 4px; margin-top: 16px; justify-content: center; }
.pg-btn { padding: 4px 10px; border: 1px solid var(--border); border-radius: 4px; background: var(--card); color: var(--text); font-size: 13px; cursor: pointer; }
.pg-btn:hover:not(:disabled) { border-color: var(--accent); }
.pg-btn:disabled { opacity: .4; cursor: default; }
.pg-btn.active { background: var(--accent); color: #fff; border-color: var(--accent); }
.pg-ellipsis { padding: 0 4px; color: var(--muted); }
.pg-info { font-size: 12px; color: var(--muted); }
.pg-size { padding: 3px 6px; border: 1px solid var(--border); border-radius: 4px; font-size: 12px; background: var(--card); }
</style>