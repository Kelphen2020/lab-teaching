<template>
  <transition name="guide">
    <div v-if="open" class="guide-mask" @click.self="skip">
      <div class="guide-card">
        <div class="guide-head">
          <span class="guide-step-badge">{{ step + 1 }} / {{ steps.length }}</span>
          <button class="btn btn-sm" @click="skip"><SvgIcon name="x" /> 跳过</button>
        </div>

        <div class="guide-ico" :class="cur.chip"><svg v-if="cur.icon"><use :href="'#' + cur.icon"/></svg></div>
        <h2>{{ cur.title }}</h2>
        <p class="guide-desc">{{ cur.desc }}</p>
        <ul v-if="cur.tips && cur.tips.length" class="guide-tips">
          <li v-for="(t, i) in cur.tips" :key="i">{{ t }}</li>
        </ul>

        <div class="guide-dots">
          <span v-for="(s, i) in steps" :key="i" class="dot" :class="{ active: i === step, done: i < step }"></span>
        </div>

        <div class="guide-foot">
          <button class="btn" :disabled="step === 0" @click="prev">上一步</button>
          <button v-if="step < steps.length - 1" class="btn btn-primary" @click="next">下一步 →</button>
          <button v-else class="btn btn-primary" @click="finish"><SvgIcon name="check" /> 开始使用</button>
        </div>
      </div>
    </div>
  </transition>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'

const props = defineProps({
  open: { type: Boolean, default: false },
  steps: { type: Array, default: () => [] }
})
const emit = defineEmits(['close'])
const router = useRouter()
const step = ref(0)
const cur = computed(() => props.steps[step.value] || {})

function goto() {
  // 步骤关联页面：引导时实际跳转，让用户直观看到对应界面
  if (cur.value.to) router.push(cur.value.to)
}

function next() {
  if (step.value < props.steps.length - 1) { step.value++; goto() }
}
function prev() {
  if (step.value > 0) { step.value--; goto() }
}
function skip() { emit('close', false) }
function finish() { emit('close', true) }
</script>

<style scoped>
.guide-mask {
  position: fixed; inset: 0; z-index: 300;
  background: rgba(15, 25, 50, .45); backdrop-filter: blur(2px);
  display: flex; align-items: center; justify-content: center;
}
.guide-card {
  width: 480px; max-width: 92vw; background: var(--card);
  border: 1px solid var(--border2); border-radius: 14px;
  box-shadow: 0 16px 48px rgba(20, 40, 90, .25);
  padding: 20px 28px 24px; text-align: center;
}
.guide-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
.guide-step-badge { font-size: 12px; color: var(--text2); background: rgba(138,153,184,.12); padding: 3px 10px; border-radius: 12px; }
.guide-ico {
  width: 56px; height: 56px; margin: 6px auto 12px; border-radius: 14px;
  display: flex; align-items: center; justify-content: center;
}
.guide-ico svg { width: 28px; height: 28px; }
.guide-card h2 { font-size: 18px; margin-bottom: 10px; }
.guide-desc { font-size: 13px; color: var(--text2); line-height: 1.8; }
.guide-tips { margin: 12px auto 0; max-width: 380px; text-align: left; font-size: 13px; color: var(--text2); line-height: 2; list-style: none; padding: 0; }
.guide-tips li::before { content: "▸ "; color: var(--green); }
.guide-dots { display: flex; justify-content: center; gap: 8px; margin: 18px 0 14px; }
.dot { width: 8px; height: 8px; border-radius: 50%; background: var(--border2); transition: all .2s; }
.dot.active { background: var(--green); transform: scale(1.25); }
.dot.done { background: rgba(0,179,104,.4); }
.guide-foot { display: flex; justify-content: center; gap: 12px; }
.guide-enter-active, .guide-leave-active { transition: opacity .25s; }
.guide-enter-from, .guide-leave-to { opacity: 0; }
</style>
