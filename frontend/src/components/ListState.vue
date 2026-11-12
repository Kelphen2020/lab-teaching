<template>
  <div v-if="state !== 'ready'" class="ls" :class="'ls-' + state">
    <template v-if="state === 'loading'">
      <div class="ls-spinner"><div></div></div>
      <p>加载中…</p>
    </template>
    <template v-else-if="state === 'empty'">
      <div class="ls-emoji"><SvgIcon name="inbox" /> </div>
      <p class="ls-title">{{ emptyText }}</p>
      <p v-if="emptyHint" class="ls-hint">{{ emptyHint }}</p>
      <button v-if="showRetry" class="btn" @click="emit('retry')"><SvgIcon name="refresh" /> 重新加载</button>
    </template>
    <template v-else-if="state === 'error'">
      <div class="ls-emoji"><SvgIcon name="alert" /> </div>
      <p class="ls-title">{{ errorText || '加载失败' }}</p>
      <p v-if="errorDetail" class="ls-hint">{{ errorDetail }}</p>
      <button v-if="showRetry" class="btn btn-primary" @click="emit('retry')"><SvgIcon name="refresh" /> 重试</button>
    </template>
  </div>
  <slot v-else></slot>
</template>

<script setup>
defineProps({
  state: { type: String, default: 'ready' }, // loading | empty | error | ready
  emptyText: { type: String, default: '暂无数据' },
  emptyHint: { type: String, default: '' },
  errorText: { type: String, default: '' },
  errorDetail: { type: String, default: '' },
  showRetry: { type: Boolean, default: true },
})
const emit = defineEmits(['retry'])
</script>

<style scoped>
.ls {
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  gap: 8px; padding: 40px 16px; text-align: center; color: var(--text2);
}
.ls-spinner { width: 28px; height: 28px; }
.ls-spinner div {
  width: 100%; height: 100%; border-radius: 50%;
  border: 3px solid var(--border); border-top-color: var(--accent);
  animation: ls-spin .8s linear infinite;
}
@keyframes ls-spin { to { transform: rotate(360deg); } }
.ls-emoji { font-size: 36px; line-height: 1; opacity: .7; }
.ls-title { font-size: 14px; font-weight: 600; color: var(--text); }
.ls-hint { font-size: 12px; color: var(--muted); }
.ls-error .ls-title { color: var(--red); }
</style>
