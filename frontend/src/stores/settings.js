import { defineStore } from 'pinia'

// 主题预设：light-tech（默认）为 :root 内置，无需额外 CSS
export const THEME_PRESETS = [
  { key: 'light-tech',    name: '极客蓝（默认）', desc: '浅色科技风 · 蓝色渐变', dark: false },
  { key: 'emerald-green', name: '翡翠绿',       desc: '清新绿调 · 自然护眼',   dark: false },
  { key: 'royal-purple',  name: '典雅紫',       desc: '紫色渐变 · 稳重典雅',   dark: false },
  { key: 'midnight-dark', name: '暗夜蓝',       desc: '深色模式 · 夜间护眼',   dark: true },
  { key: 'obsidian-black', name: '曜石黑',      desc: '深色模式 · 深空碳黑 · 琥珀金', dark: true },
  { key: 'nebula-purple', name: '暮紫星河',     desc: '深色模式 · 星空紫调 · 优雅', dark: true }
]

const DEFAULT_SETTINGS = {
  site_title: '机房教学管理系统',
  site_logo: '',
  theme: 'light-tech'
}

export const useSettingsStore = defineStore('settings', {
  state: () => ({
    site_title: DEFAULT_SETTINGS.site_title,
    site_logo: DEFAULT_SETTINGS.site_logo,
    theme: DEFAULT_SETTINGS.theme,
    loaded: false
  }),
  actions: {
    applyTheme(theme) {
      const preset = THEME_PRESETS.find(t => t.key === theme) || THEME_PRESETS[0]
      if (preset.key === 'light-tech') {
        document.documentElement.removeAttribute('data-theme')
      } else {
        document.documentElement.setAttribute('data-theme', preset.key)
      }
      // data-dark 标记：供全局深色覆盖样式使用（深色主题共用一套修正规则）
      if (preset.dark) {
        document.documentElement.setAttribute('data-dark', '1')
      } else {
        document.documentElement.removeAttribute('data-dark')
      }
    },
    applyAll() {
      this.applyTheme(this.theme)
      document.title = this.site_title
    },
    // 从后端加载系统设置（登录页与 Layout 启动时调用；后端不可达时用默认值）
    async load() {
      try {
        const api = (await import('../api')).default
        const res = await api.get('/settings')
        if (res.code === 0 && res.data) {
          const s = res.data
          this.site_title = s.site_title || DEFAULT_SETTINGS.site_title
          this.site_logo = s.site_logo || ''
          this.theme = s.theme || DEFAULT_SETTINGS.theme
          this.loaded = true
        }
      } catch (e) {
        // 静默降级为默认值，不阻塞登录
      }
      this.applyAll()
    },
    updateLocal(patch) {
      Object.assign(this, patch)
      this.applyAll()
    }
  }
})
