<template>
  <div class="dashboard">
    <!-- 概览统计卡片 -->
    <div class="stat-grid">
      <router-link to="/rooms" class="stat-card">
        <div class="stat-num">{{ ov.room_count }}</div>
        <div class="stat-label">机房总数</div>
      </router-link>
      <router-link to="/rooms" class="stat-card">
        <div class="stat-num">{{ ov.device_count }}</div>
        <div class="stat-label">设备总数</div>
        <div class="stat-sub">在用 {{ ov.device_in_use }} / 故障 {{ ov.device_fault }}</div>
      </router-link>
      <router-link to="/agents" class="stat-card">
        <div class="stat-num online">{{ ov.agent_online }}</div>
        <div class="stat-label">Agent 在线</div>
      </router-link>
      <router-link to="/settings" class="stat-card">
        <div class="stat-num">{{ ov.user_count }}</div>
        <div class="stat-label">用户数</div>
      </router-link>
      <router-link to="/exam" class="stat-card">
        <div class="stat-num">{{ ov.exam_total }}</div>
        <div class="stat-label">考试总数</div>
        <div class="stat-sub">进行中 {{ ov.exam_running }} / 已结束 {{ ov.exam_finished }}</div>
      </router-link>
      <router-link to="/exam" class="stat-card">
        <div class="stat-num">{{ ov.question_count }}</div>
        <div class="stat-label">题库</div>
        <div class="stat-sub">试卷 {{ ov.paper_count }}</div>
      </router-link>
      <router-link to="/exam" class="stat-card">
        <div class="stat-num warn">{{ ov.violation_count }}</div>
        <div class="stat-label">违纪记录</div>
      </router-link>
    </div>

    <!-- 房间利用率 -->
    <div class="card" v-if="roomStats.length">
      <router-link to="/rooms" class="card-title card-title-link">房间利用率 ↗</router-link>
      <table class="data-table">
        <thead>
          <tr><th>房间</th><th>座位数</th><th>在线终端</th><th>利用率</th><th>状态</th></tr>
        </thead>
        <tbody>
          <tr v-for="r in roomStats" :key="r.room_id" class="clickable-row" @click="goRooms(r.room_id)">
            <td>{{ r.room_name }}</td>
            <td>{{ r.seats }}</td>
            <td>{{ r.online }}</td>
            <td>
              <div class="rate-bar">
                <div class="rate-fill" :style="{ width: rateNum(r.rate) + '%' }"></div>
                <span class="rate-text">{{ r.rate }}</span>
              </div>
            </td>
            <td>
              <span class="badge" :class="r.online > 0 ? 'badge-green' : 'badge-gray'">
                {{ r.online > 0 ? '活跃' : '空闲' }}
              </span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 最近操作日志 -->
    <div class="card">
      <router-link to="/audits" class="card-title card-title-link">最近操作日志 ↗</router-link>
      <table class="data-table" v-if="audits.length">
        <thead>
          <tr><th>时间</th><th>操作</th><th>目标</th><th>IP</th><th>详情</th></tr>
        </thead>
        <tbody>
          <tr v-for="a in audits" :key="a.id" class="clickable-row" @click="$router.push('/audits')">
            <td class="td-time">{{ fmtTime(a.created_at) }}</td>
            <td><span class="method-tag" :class="'m-' + a.action.split(' ')[0]">{{ a.action }}</span></td>
            <td class="td-path">{{ a.target }}</td>
            <td>{{ a.ip }}</td>
            <td class="td-detail">{{ a.detail }}</td>
          </tr>
        </tbody>
      </table>
      <p v-else class="empty-hint">暂无操作记录</p>
    </div>

    <!-- 最近违纪 -->
    <div class="card" v-if="violations.length">
      <router-link to="/exam" class="card-title card-title-link">最近违纪告警 ↗</router-link>
      <table class="data-table">
        <thead>
          <tr><th>时间</th><th>考试ID</th><th>学生</th><th>类型</th><th>详情</th></tr>
        </thead>
        <tbody>
          <tr v-for="v in violations" :key="v.id" class="clickable-row" @click="$router.push('/exam')">
            <td class="td-time">{{ fmtTime(v.created_at) }}</td>
            <td>{{ v.exam_id }}</td>
            <td>{{ v.student_name || v.student_id }}</td>
            <td><span class="badge badge-red">{{ v.type }}</span></td>
            <td class="td-detail">{{ v.detail }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'

const router = useRouter()

const ov = ref({
  room_count: 0, device_count: 0, agent_online: 0, agent_db_online: 0,
  user_count: 0, question_count: 0, paper_count: 0,
  exam_total: 0, exam_running: 0, exam_finished: 0,
  violation_count: 0, device_in_use: 0, device_fault: 0
})
const roomStats = ref([])
const audits = ref([])
const violations = ref([])

onMounted(async () => {
  try {
    const res = await api.get('/stats/dashboard')
    const d = res.data
    if (d.overview) ov.value = d.overview
    if (d.room_stats) roomStats.value = d.room_stats
    if (d.recent_audits) audits.value = d.recent_audits
    if (d.recent_violations) violations.value = d.recent_violations
  } catch (e) {
    console.error('Dashboard load failed:', e)
  }
})

function rateNum(rateStr) {
  return parseFloat(rateStr) || 0
}

function fmtTime(t) {
  if (!t) return '-'
  const d = new Date(t)
  return d.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

function goRooms(roomId) {
  router.push({ path: '/rooms', query: { room_id: roomId } })
}
</script>

<style scoped>
.dashboard { padding: 0; }

/* 统计卡片 */
.stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  gap: 16px;
  margin-bottom: 24px;
}
.stat-card {
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 20px 16px;
  text-align: center;
  text-decoration: none;
  color: inherit;
  display: block;
  transition: border-color .2s, box-shadow .2s;
}
.stat-card:hover {
  border-color: var(--accent);
  box-shadow: 0 4px 14px rgba(0,102,255,.12);
}
.stat-num {
  font-size: 32px;
  font-weight: 700;
  color: var(--text);
  line-height: 1.2;
}
.stat-num.online { color: var(--green); }
.stat-num.warn { color: var(--red); }
.stat-label {
  font-size: 13px;
  color: var(--text2);
  margin-top: 6px;
}
.stat-sub {
  font-size: 11px;
  color: var(--muted);
  margin-top: 4px;
}

/* 卡片 */
.card {
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 20px;
  margin-bottom: 20px;
}
.card-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text);
  margin: 0 0 16px 0;
}
.card-title-link {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  text-decoration: none;
  color: var(--text);
  transition: color .2s;
}
.card-title-link:hover { color: var(--accent); }

/* 表格 */
.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
.data-table th {
  text-align: left;
  padding: 10px 12px;
  background: var(--card2);
  border-bottom: 1px solid var(--border);
  color: var(--text2);
  font-weight: 500;
}
.data-table td {
  padding: 10px 12px;
  border-bottom: 1px solid var(--border);
  color: var(--text);
}
.data-table tr:hover td { background: rgba(0,102,255,.04); }

.td-time { white-space: nowrap; color: var(--muted); font-size: 12px; }
.td-path { font-family: monospace; font-size: 12px; color: var(--text2); }
.td-detail { max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

/* 利用率进度条 */
.rate-bar {
  position: relative;
  height: 20px;
  background: var(--border);
  border-radius: 4px;
  overflow: hidden;
  min-width: 100px;
}
.rate-fill {
  height: 100%;
  background: linear-gradient(90deg, var(--accent), var(--green));
  border-radius: 4px;
  transition: width 0.3s;
}
.rate-text {
  position: absolute;
  right: 6px;
  top: 2px;
  font-size: 11px;
  color: var(--text);
  font-weight: 500;
}

/* 徽章 */
.badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
}
.badge-green { background: rgba(0,179,104,.13); color: var(--green); border: 1px solid rgba(0,179,104,.28); }
.badge-gray { background: var(--card2); color: var(--muted); border: 1px solid var(--border2); }
.badge-red { background: rgba(229,72,77,.10); color: var(--red); border: 1px solid rgba(229,72,77,.28); }

/* HTTP 方法标签 */
.method-tag {
  display: inline-block;
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 11px;
  font-family: monospace;
  font-weight: 600;
}
.m-POST { background: var(--accent-glow); color: var(--accent); }
.m-PUT { background: rgba(245,158,11,.12); color: var(--orange); }
.m-DELETE { background: rgba(229,72,77,.10); color: var(--red); }
.m-AGENT_OFFLINE { background: rgba(139,92,246,.10); color: var(--purple); }

.empty-hint { color: var(--muted); font-size: 13px; text-align: center; padding: 20px 0; }

/* 可点击行 */
.clickable-row { cursor: pointer; }
.clickable-row:hover td { background: rgba(0,102,255,.06); }
</style>
