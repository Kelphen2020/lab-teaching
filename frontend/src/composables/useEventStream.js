import { ref, onMounted, onUnmounted } from 'vue'
import { useAuthStore } from '../stores/auth'

// d1: 浏览器事件推送 WebSocket 订阅（/ws/events，JWT query 鉴权）。
// 用法：
//   const { connected, on, close } = useEventStream()
//   const off = on('metrics', payload => { ... })
//   onMounted(() => { ... })  // composable 在 setup 期间自动 connect
// 连接失败/断开时指数退避重连（1s/2s/4s/8s，上限 15s）；
// 页面侧通过 connected.value === false 判断降级轮询。
export function useEventStream() {
  const connected = ref(false)
  const auth = useAuthStore()

  let ws = null
  let retryTimer = null
  let retries = 0
  let closed = false
  const handlers = new Map() // type -> Set<fn>

  function connect() {
    if (closed || !auth.token) return
    try {
      const proto = location.protocol === 'https:' ? 'wss://' : 'ws://'
      ws = new WebSocket(`${proto}${location.host}/ws/events?token=${encodeURIComponent(auth.token)}`)
    } catch (e) {
      scheduleRetry()
      return
    }

    ws.onopen = () => {
      connected.value = true
      retries = 0
    }

    ws.onmessage = ev => {
      let frame
      try { frame = JSON.parse(ev.data) } catch { return }
      if (!frame || !frame.type) return
      const set = handlers.get(frame.type)
      if (!set) return
      const payload = typeof frame.payload === 'string'
        ? safeParse(frame.payload)
        : frame.payload
      set.forEach(fn => { try { fn(payload) } catch { /* 订阅方异常不影响其他订阅 */ } })
    }

    ws.onclose = () => {
      connected.value = false
      ws = null
      scheduleRetry()
    }

    ws.onerror = () => { try { ws.close() } catch { /* noop */ } }
  }

  function scheduleRetry() {
    if (closed) return
    clearTimeout(retryTimer)
    const delay = Math.min(1000 * Math.pow(2, retries), 15000)
    retries++
    retryTimer = setTimeout(connect, delay)
  }

  // 订阅事件，返回取消函数
  function on(type, fn) {
    if (!handlers.has(type)) handlers.set(type, new Set())
    handlers.get(type).add(fn)
    return () => off(type, fn)
  }

  function off(type, fn) {
    const set = handlers.get(type)
    if (set) set.delete(fn)
  }

  function close() {
    closed = true
    clearTimeout(retryTimer)
    if (ws) { try { ws.close() } catch { /* noop */ } ws = null }
    connected.value = false
  }

  onMounted(connect)
  onUnmounted(close)

  return { connected, on, close }
}

function safeParse(s) {
  try { return JSON.parse(s) } catch { return s }
}
