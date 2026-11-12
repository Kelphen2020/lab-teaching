import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import SvgIcon from './components/SvgIcon.vue'
import './style.css'

const app = createApp(App)
const pinia = createPinia()
app.component('SvgIcon', SvgIcon)
app.use(pinia)
app.use(router)
app.mount('#app')

// 启动时预加载系统设置（公开接口）：标题/Logo/主题，未登录也生效
import('./stores/settings').then(({ useSettingsStore }) => {
  const settings = useSettingsStore(pinia)
  settings.load().then(() => { document.title = settings.site_title })
})
