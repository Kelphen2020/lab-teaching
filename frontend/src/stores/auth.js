import { defineStore } from 'pinia'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem('token') || '',
    user: JSON.parse(localStorage.getItem('user') || 'null'),
    permissions: JSON.parse(localStorage.getItem('permissions') || 'null')
  }),
  getters: {
    isLogin: s => !!s.token,
    isAdmin: s => s.user?.role === 'admin',
    // 权限判定：admin 或 permissions 含 "*" 全通；否则精确匹配
    can() {
      return key => {
        if (this.user?.role === 'admin') return true
        if (!this.permissions) return false
        return this.permissions.includes('*') || this.permissions.includes(key)
      }
    }
  },
  actions: {
    setAuth(token, user, permissions) {
      this.token = token
      this.user = user
      this.permissions = permissions || []
      localStorage.setItem('token', token)
      localStorage.setItem('user', JSON.stringify(user))
      localStorage.setItem('permissions', JSON.stringify(this.permissions))
    },
    logout() {
      this.token = ''
      this.user = null
      this.permissions = null
      localStorage.removeItem('token')
      localStorage.removeItem('user')
      localStorage.removeItem('permissions')
    }
  }
})
