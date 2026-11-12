<template>
  <svg class="svg-icon" :width="size" :height="size" viewBox="0 0 24 24" fill="none"
    :stroke="color || 'currentColor'" :stroke-width="strokeWidth" stroke-linecap="round" stroke-linejoin="round"
    aria-hidden="true" focusable="false">
    <path :d="d" />
  </svg>
</template>

<script>
// 科技感线性图标库（与移动端 TechIcon 同源，PC 端 inline SVG 版，继承 currentColor）
export const ICON_PATHS = {
  // 通用
  grid: 'M4 4h7v7H4zM13 4h7v7h-7zM4 13h7v7H4zM13 13h7v7h-7z',
  menu: 'M4 6h16M4 12h16M4 18h16',
  check: 'M4.5 12.5 9.5 17.5 19.5 6.5',
  x: 'M6 6l12 12M18 6 6 18',
  alert: 'M12 3 2 20h20L12 3zM12 10v4M12 17.5v.5',
  info: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM12 11v5M12 8v.01',
  help: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM9.5 9a2.5 2.5 0 1 1 3.5 2.3c-.8.4-1 .9-1 1.7M12 17v.01',
  'check-circle': 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM8 12.5l2.8 2.8L16.5 9.5',
  'x-circle': 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM9 9l6 6M15 9l-6 6',
  'check-square': 'M4 5h16v14H4V5zM8 12l2.5 2.5L16 9',
  'arrow-left': 'M19 12H5M11 6l-6 6 6 6',
  'arrow-right': 'M5 12h14M13 6l6 6-6 6',
  'arrow-up': 'M12 19V5M6 11l6-6 6 6',
  'arrow-down': 'M12 5v14M6 13l6 6 6-6',
  expand: 'M4 9V4h5M20 9V4h-5M4 15v5h5M20 15v5h-5',
  search: 'M10.5 18a7.5 7.5 0 1 0 0-15 7.5 7.5 0 0 0 0 15zM16 16l4.5 4.5',
  bell: 'M18 9a6 6 0 1 0-12 0c0 6-2 7-2 7h16s-2-1-2-7M13.7 20a2 2 0 0 1-3.4 0',
  'bell-off': 'M18 9a6 6 0 0 0-9.3-5M6 9c0 6-2 7-2 7h13M4 4l16 16M13.7 20a2 2 0 0 1-3.4 0',
  clock: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM12 7v5l3.5 2',
  user: 'M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4 20c0-4 3.6-6 8-6s8 2 8 6',
  lock: 'M6 11h12v9H6v-9zM9 11V7.5a3 3 0 0 1 6 0V11',
  unlock: 'M6 11h12v9H6v-9zM9 11V7.5a3 3 0 0 1 5.8-1',
  key: 'M8.5 15.5a5 5 0 1 1 7-7 5 5 0 0 1-7 7zM12.5 11.5 21 20M17.5 16.5l2-2M15 14l1.5-1.5',
  shield: 'M12 3 5 6v5c0 4.4 3 8.5 7 10 4-1.5 7-5.6 7-10V6l-7-3zM9.2 12.2 11.3 14.3 15 10.6',
  pen: 'M4 20h4L20 8l-4-4L4 16v4zM14.5 5.5l4 4',
  doc: 'M6 3h8l4 4v14H6V3zM14 3v4h4M9 12h6M9 16h6',
  clipboard: 'M9 4h6v3H9V4zM7 5.5H5v15h14v-15h-2M9 11h6M9 15h4',
  folder: 'M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7z',
  book: 'M4 4.5A2.5 2.5 0 0 1 6.5 2H20v18H6.5A2.5 2.5 0 0 0 4 22V4.5zM4 17.5A2.5 2.5 0 0 1 6.5 15H20',
  log: 'M6 3h12v18l-2-1.5L14 21l-2-1.5L10 21l-2-1.5L6 21V3zM9 7h6M9 11h6M9 15h4',
  box: 'M12 3 3 7.5v9L12 21l9-4.5v-9L12 3zM3 7.5 12 12l9-4.5M12 12v9',
  tag: 'M3 12.5 12.5 3H21v8.5L11.5 21 3 12.5zM16.5 7.5v.01',
  link: 'M10 13a5 5 0 0 0 7.5.5l2-2a5 5 0 0 0-7-7l-1.5 1.5M14 11a5 5 0 0 0-7.5-.5l-2 2a5 5 0 0 0 7 7L13 18',
  pin: 'M12 21s7-6.2 7-11a7 7 0 1 0-14 0c0 4.8 7 11 7 11zM12 12.5a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5z',
  inbox: 'M4 13h4l1.5 3h5L16 13h4M4 13 6.5 5h11L20 13v6H4v-6z',
  // 动作
  save: 'M5 3h11l3 3v15H5V3zM8 3v6h8V3M8 14h8v7H8v-7z',
  upload: 'M12 16V4M7 9l5-5 5 5M4 20h16',
  download: 'M12 4v12M7 11l5 5 5-5M4 20h16',
  refresh: 'M20 11a8 8 0 1 0-2.3 6.3M20 5v6h-6',
  printer: 'M7 8V3h10v5M7 18H5a2 2 0 0 1-2-2v-4a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2v4a2 2 0 0 1-2 2h-2M7 14h10v7H7v-7z',
  camera: 'M4 7h3l2-2h6l2 2h3v13H4V7zM12 16.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7z',
  scan: 'M4 8V5.5A1.5 1.5 0 0 1 5.5 4H8M16 4h2.5A1.5 1.5 0 0 1 20 5.5V8M20 16v2.5a1.5 1.5 0 0 1-1.5 1.5H16M8 20H5.5A1.5 1.5 0 0 1 4 18.5V16M4 12h16',
  wrench: 'M14.5 6a3.5 3.5 0 0 1 4.9 3.2L21 10.7l-2.8 2.8-1.5-1.5A3.5 3.5 0 0 1 12 8.4L4.6 15.8a2 2 0 1 0 2.8 2.8L14.8 11',
  gear: 'M12 9.5a2.5 2.5 0 1 0 0 5 2.5 2.5 0 0 0 0-5zM12 2v3M12 19v3M2 12h3M19 12h3M4.9 4.9 7 7M17 17l2.1 2.1M19.1 4.9 17 7M7 17l-2.1 2.1',
  target: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM12 16.5a4.5 4.5 0 1 0 0-9 4.5 4.5 0 0 0 0 9zM12 13.2a1.2 1.2 0 1 0 0-2.4 1.2 1.2 0 0 0 0 2.4z',
  compass: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM15.5 8.5l-2 5-5 2 2-5 5-2z',
  ruler: 'M15.5 2.5 21.5 8.5 8.5 21.5 2.5 15.5 15.5 2.5zM6 12l2 2M9 9l2 2M12 6l2 2',
  swap: 'M7 4 4 7.5 7 11M4 7.5h11a4 4 0 0 1 0 8H9M17 20l3-3.5-3-3.5',
  shuffle: 'M3 5h4l10 14h4M17 5h4M17 19h4M21 5l-3 3M21 19l-3-3M3 19h4l3-4',
  // 数据/图表
  chart: 'M4 20V10M9.3 20V4M14.7 20v-8M20 20V7',
  pie: 'M12 3a9 9 0 1 0 9 9h-9V3zM14.5 3.5A8 8 0 0 1 20.5 9.5H14.5V3.5z',
  trend: 'M3 17l6-6 4 4 8-8M21 7h-6M21 7v6',
  gauge: 'M12 21a9 9 0 1 1 9-9M12 12l4.5-4.5M21 12h-3M18.4 5.6l-2.1 2.1',
  counter: 'M4 19h16M6 19V9M11 19V5M16 19v-7M20 19v-4',
  cal: 'M4 6h16v15H4V6zM4 10h16M8 3v4M16 3v4',
  layers: 'M12 3 3 8l9 5 9-5-9-5zM3 13l9 5 9-5M3 17l9 5 9-5',
  // 机房/动环
  snow: 'M12 2v20M4.2 6.5l15.6 11M19.8 6.5 4.2 17.5M12 6 9.5 4M12 6l2.5-2M12 18l-2.5 2M12 18l2.5 2',
  battery: 'M2 8h16v8H2V8zM21 11v2M5.5 11v2M9 11v2M12.5 11v2',
  plug: 'M9 2v6M15 2v6M6 8h12v3a6 6 0 0 1-12 0V8zM12 17v5',
  bolt: 'M13 2 4 14h6l-1 8 9-12h-6l1-8z',
  flame: 'M12 22c4 0 7-2.7 7-6.5 0-4.5-4.5-7-5.5-13.5C10 5 8.5 7.5 8.5 10c-1-.5-1.8-1.6-2-3C4.8 8.7 5 11 5 15.5 5 19.3 8 22 12 22z',
  drop: 'M12 3s6 6.4 6 10.5a6 6 0 1 1-12 0C6 9.4 12 3 12 3z',
  temp: 'M10 13.6V5a2 2 0 1 1 4 0v8.6a4.5 4.5 0 1 1-4 0z',
  wind: 'M3 8h11a3 3 0 1 0-3-3M3 12h15a3 3 0 1 1-3 3M3 16h8',
  fan: 'M12 13a1 1 0 1 0 0-2 1 1 0 0 0 0 2zM12 11V4c3 0 4 2 3 4M13.7 12.5l6 2c-1.2 2.8-3.5 2.4-4.8.4M10.3 12.5l-6 2c1.2 2.8 3.5 2.4 4.8.4',
  storm: 'M3 5h18M5 9h14M8 13h9M11 17h5M13 21h3',
  monitor: 'M3 4h18v12H3V4zM8 20h8M12 16v4',
  server: 'M4 5h16v5H4zM4 14h16v5H4zM7.5 7.5h.5M7.5 16.5h.5',
  rack: 'M5 2h14v20H5V2zM8 6h5M8 10h5M8 14h5M17 6v.01M17 10v.01M17 14v.01',
  building: 'M4 21V5a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v16M16 9h2a2 2 0 0 1 2 2v10M2 21h20M7 7h2M11 7h2M7 11h2M11 11h2M7 15h2M11 15h2',
  home: 'M3 11l9-8 9 8M5 9.5V21h14V9.5M9.5 21v-6h5v6',
  globe: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM3.5 9h17M3.5 15h17M12 3c2.5 2.6 3.8 5.7 3.8 9S14.5 18.4 12 21c-2.5-2.6-3.8-5.7-3.8-9S9.5 5.6 12 3z',
  signal: 'M12 13.5a1.5 1.5 0 1 0 0-3 1.5 1.5 0 0 0 0 3zM8.5 8.5a5 5 0 0 0 0 7M15.5 15.5a5 5 0 0 0 0-7M5.6 5.6a9 9 0 0 0 0 12.8M18.4 18.4a9 9 0 0 0 0-12.8',
  wifi: 'M2 8.5a15 15 0 0 1 20 0M5 12a10 10 0 0 1 14 0M8.5 15.5a5 5 0 0 1 7 0M12 19v.01',
  cube: 'M12 2 3 7v10l9 5 9-5V7l-9-5zM3 7l9 5 9-5M12 12v10',
  toolbox: 'M3 8h18v11H3V8zM8 8V5h8v3M3 13h18M10 13v2h4v-2',
  puzzle: 'M10 4a2 2 0 1 1 4 0v1h3a1 1 0 0 1 1 1v3h1a2 2 0 1 1 0 4h-1v3a1 1 0 0 1-1 1h-3v1a2 2 0 1 1-4 0v-1H6a1 1 0 0 1-1-1v-3H4a2 2 0 1 1 0-4h1V6a1 1 0 0 1 1-1h3V4z',
  factory: 'M3 21V10l5 3.5V10l5 3.5V10l6 4v7H3zM6 17v.01M11 17v.01M16 17v.01',
  pager: 'M4 6h16v12H4V6zM8 10h8M8 14h4',
  door: 'M6 2h12v20H6V2zM14.5 12v.01M3 22h18',
  column: 'M6 3h12M8 3v18M16 3v18M6 21h12',
  wall: 'M3 5h18v14H3V5zM3 12h18M9 5v7M15 12v7',
  phone: 'M7 2h10v20H7V2zM10.5 19h3',
  bulb: 'M9 18h6M10 21h4M12 3a6 6 0 0 0-3.5 10.9c.6.5 1 1.2 1 2h5c0-.8.4-1.5 1-2A6 6 0 0 0 12 3z',
  eye: 'M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12zM12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z',
  'eye-off': 'M4 4l16 16M9.9 5.9A9.9 9.9 0 0 1 12 5.5c6 0 9.5 6.5 9.5 6.5a17 17 0 0 1-3.3 4.1M6.4 7.9A16.8 16.8 0 0 0 2.5 12S6 18.5 12 18.5c1.4 0 2.6-.3 3.7-.8M9.9 9.9a3 3 0 0 0 4.2 4.2',
  cpu: 'M7 7h10v10H7V7zM4 10h3M4 14h3M17 10h3M17 14h3M10 4v3M14 4v3M10 17v3M14 17v3',
  image: 'M3 5h18v14H3V5zM8.5 11a1.5 1.5 0 1 0 0-3 1.5 1.5 0 0 0 0 3zM3 16l5-4 4 3 3.5-3L21 16',
  palette: 'M12 21a9 9 0 1 1 9-9c0 1.7-1.3 3-3 3h-2a2 2 0 0 0-1.4 3.4c.4.4.4 1.2-.2 1.7-.8.6-1.7.9-2.4.9zM7.5 12a1 1 0 1 0 0-2 1 1 0 0 0 0 2zM10.5 8a1 1 0 1 0 0-2 1 1 0 0 0 0 2zM15 8.5a1 1 0 1 0 0-2 1 1 0 0 0 0 2z',
  // 教学系统扩展
  trash: 'M4 7h16M9 7V5a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2M6 7l1 13a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1l1-13M10 11v6M14 11v6',
  rocket: 'M12 2.5c3.2 2.2 5 5.9 5 9.5l-2.5 3h-5l-2.5-3c0-3.6 1.8-7.3 5-9.5zM9.5 15 8 19.5l4-2 4 2-1.5-4.5M12 8a1.6 1.6 0 1 0 0 3.2A1.6 1.6 0 0 0 12 8z',
  power: 'M12 3v9M6.8 6.8a8 8 0 1 0 10.4 0',
  chat: 'M4 5h16v11H8.5L4 19.5V5zM8 9h8M8 12h5',
  grad: 'M12 4 2.5 9 12 14l9.5-5L12 4zM6 11.5V16c0 1.7 2.7 3 6 3s6-1.3 6-3v-4.5M21.5 9v5.5',
  cast: 'M3 5h13v10H3V5zM7 19h4M9 15v4M18.5 8a5 5 0 0 1 0 6M21 5.5a8.5 8.5 0 0 1 0 11',
  sliders: 'M4 8h10M18 8h2M4 16h4M12 16h8M16 6v4M8 14v4',
  card: 'M3 6h18v12H3V6zM3 10h18M6 14h4',
  play: 'M8 5l11 7-11 7V5z',
  stop: 'M6 6h12v12H6V6z',
  undo: 'M4 9h11a5 5 0 0 1 0 10h-5M4 9l4-4M4 9l4 4',
}

export default {
  name: 'SvgIcon',
  props: {
    name: { type: String, default: 'grid' },
    size: { type: [Number, String], default: '1em' },
    color: { type: String, default: '' },
    strokeWidth: { type: [Number, String], default: 1.8 },
  },
  computed: {
    d() {
      return ICON_PATHS[this.name] || ICON_PATHS.grid
    },
  },
}
</script>

<style scoped>
.svg-icon {
  display: inline-block;
  vertical-align: -0.14em;
  flex-shrink: 0;
}
</style>
