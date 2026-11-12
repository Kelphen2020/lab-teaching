<template>
  <div class="login-wrap">
    <div class="login-box">
      <div v-if="settings.site_logo" class="login-logo">
        <img :src="settings.site_logo" alt="logo" />
      </div>
      <h2>{{ settings.site_title }}</h2>
      <p class="login-sub">Light-Tech Console · 北京东青互联科技</p>
      <div class="err" v-if="err">{{ err }}</div>
      <input v-model="username" placeholder="用户名" @keyup.enter="login" autocomplete="username" />
      <input v-model="password" type="password" placeholder="密码" @keyup.enter="login" autocomplete="current-password" />
      <button class="btn btn-primary" @click="login" :disabled="loading">
        {{ loading ? '登录中...' : '登 录' }}
      </button>
      <p style="text-align:center;color:var(--muted);font-size:12px;margin-top:16px">初始账号请联系系统管理员获取</p>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useSettingsStore } from '../stores/settings'
import api from '../api'

const router = useRouter()
const auth = useAuthStore()
const settings = useSettingsStore()
// 记住上次登录的用户名（不记住密码）
const username = ref(localStorage.getItem('last_username') || '')
const password = ref('')
const err = ref('')
const loading = ref(false)
let lastAttempt = 0

onMounted(() => {
  settings.load().then(() => { document.title = settings.site_title })
})

async function login() {
  err.value = ''
  // 3 秒防抖，避免弱网重复点击触发登录锁定
  const now = Date.now()
  if (loading.value || now - lastAttempt < 3000) return
  lastAttempt = now
  if (!username.value || !password.value) {
    err.value = '请输入用户名和密码'
    return
  }
  loading.value = true
  try {
    const res = await api.post('/auth/login', { username: username.value, password: password.value })
    auth.setAuth(res.data.token, res.data.user, res.data.permissions)
    localStorage.setItem('last_username', username.value)
    router.push('/dashboard')
  } catch (e) {
    const status = e.response?.status
    const msg = e.response?.data?.msg
    if (status === 423) {
      err.value = msg || '失败次数过多，账号已临时锁定，请 15 分钟后再试'
    } else if (status === 401) {
      err.value = '用户名或密码错误'
    } else if (!e.response) {
      err.value = '网络异常，请检查网络连接后重试'
    } else {
      err.value = msg || '登录失败，请稍后重试'
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-logo { text-align: center; margin-bottom: 10px; }
.login-logo img { height: 44px; max-width: 240px; object-fit: contain; }
</style>
