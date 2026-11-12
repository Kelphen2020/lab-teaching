import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import legacy from '@vitejs/plugin-legacy'

// V1.0-15 国产浏览器兼容:
//   - 奇安信浏览器 / 360 安全浏览器: 极速模式内核为 Chromium 86 左右，
//     兼容模式为 IE（不支持，提示切极速模式）
//   - Firefox ESR 78+（信创常用版本）
//   - Chrome 86+ / Chromium 系国产浏览器
// legacy 插件为旧内核生成 ES2015 polyfill 回退包；build.target 控制主产物语法。
export default defineConfig({
  plugins: [
    vue(),
    legacy({
      targets: ['chrome >= 86', 'firefox >= 78', 'defaults'],
      modernPolyfills: false,
      renderLegacyChunks: false // 主产物已降到 chrome86 语法，无需独立 legacy chunk
    })
  ],
  build: {
    target: 'chrome86',
    cssTarget: 'chrome86',
    chunkSizeWarningLimit: 1500
  },
  server: {
    port: 3100,
    proxy: {
      '/api': { target: 'http://localhost:8090', changeOrigin: true },
      '/agent': { target: 'ws://localhost:8090', ws: true }
    }
  }
})
