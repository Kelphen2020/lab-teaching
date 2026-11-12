<template>
  <div v-if="visible" class="cd-mask" @click.self="cancel">
    <div class="cd-box" role="dialog" aria-modal="true">
      <div class="cd-icon" :class="tone"><SvgIcon :name="iconChar" :size="22" /></div>
      <div class="cd-title">{{ title }}</div>
      <div class="cd-body"><slot>{{ message }}</slot></div>
      <div v-if="requirePassword" class="cd-pwd">
        <input
          type="password"
          v-model="pwd"
          placeholder="请输入当前登录密码以确认身份"
          @keyup.enter="confirm"
          autofocus
        />
        <p v-if="pwdErr" class="cd-pwd-err">{{ pwdErr }}</p>
      </div>
      <div class="cd-actions">
        <button class="btn" @click="cancel">{{ cancelText }}</button>
        <button class="btn" :class="confirmClass" :disabled="busy || (requirePassword && !pwd)" @click="confirm">
          {{ busy ? '处理中...' : confirmText }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'

const props = defineProps({
  visible: { type: Boolean, default: false },
  title: { type: String, default: '操作确认' },
  message: { type: String, default: '' },
  tone: { type: String, default: 'danger' }, // danger | warn | info
  confirmText: { type: String, default: '确认' },
  cancelText: { type: String, default: '取消' },
  requirePassword: { type: Boolean, default: false },
})

const emit = defineEmits(['confirm', 'cancel', 'update:visible'])

const pwd = ref('')
const pwdErr = ref('')
const busy = ref(false)

const iconChar = computed(() => props.tone === 'danger' ? 'alert' : props.tone === 'warn' ? 'bolt' : 'info')
const confirmClass = computed(() =>
  props.tone === 'danger' ? 'btn-danger' : props.tone === 'warn' ? 'btn-warn' : 'btn-primary')

function cancel() {
  pwd.value = ''
  pwdErr.value = ''
  busy.value = false
  emit('cancel')
  emit('update:visible', false)
}

function confirm() {
  pwdErr.value = ''
  if (props.requirePassword && !pwd.value) {
    pwdErr.value = '请输入密码'
    return
  }
  busy.value = true
  // 若父组件需要异步处理（如密码校验），由父组件在 @confirm 中处理并关闭
  emit('confirm', { password: pwd.value })
}

// 供父组件在异步校验失败时调用
function setPwdError(msg) { pwdErr.value = msg || '密码错误'; busy.value = false }
function reset() { pwd.value = ''; pwdErr.value = ''; busy.value = false }

defineExpose({ setPwdError, reset })
</script>

<style scoped>
.cd-mask {
  position: fixed; inset: 0; z-index: 200;
  background: rgba(15, 25, 50, .45);
  backdrop-filter: blur(2px);
  display: flex; align-items: center; justify-content: center;
}
.cd-box {
  background: var(--card); border-radius: var(--radius);
  box-shadow: 0 12px 48px rgba(20, 40, 90, .22);
  padding: 24px; width: 420px; max-width: calc(100vw - 48px);
  animation: cd-in .18s ease-out;
}
@keyframes cd-in { from { opacity: 0; transform: translateY(8px) scale(.98); } to { opacity: 1; transform: none; } }
.cd-icon {
  width: 44px; height: 44px; border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  font-size: 22px; margin: 0 auto 12px;
}
.cd-icon.danger { background: rgba(229,72,77,.12); color: var(--red); }
.cd-icon.warn { background: rgba(245,158,11,.14); color: var(--orange); }
.cd-icon.info { background: var(--accent-glow); color: var(--accent); }
.cd-title { text-align: center; font-size: 16px; font-weight: 600; margin-bottom: 8px; }
.cd-body { text-align: center; color: var(--text2); font-size: 13px; line-height: 1.6; margin-bottom: 16px; }
.cd-pwd { margin-bottom: 14px; }
.cd-pwd input {
  width: 100%; padding: 9px 12px; border: 1px solid var(--border2);
  border-radius: 6px; font-size: 14px; box-sizing: border-box;
}
.cd-pwd input:focus { border-color: var(--accent); outline: none; }
.cd-pwd-err { color: var(--red); font-size: 12px; margin-top: 6px; text-align: center; }
.cd-actions { display: flex; gap: 10px; justify-content: center; }
.cd-actions .btn { min-width: 96px; }
.btn-warn { background: var(--orange); border-color: var(--orange); color: #fff; }
.btn-warn:hover { filter: brightness(.95); }
</style>
