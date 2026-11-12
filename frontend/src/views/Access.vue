<template>
  <div>
    <div class="card">
      <h3 style="margin-top:0"><SvgIcon name="door" /> 门禁规则</h3>
      <div class="toolbar">
        <button class="btn btn-primary" @click="openRule">＋ 新增规则</button>
        <button class="btn" @click="load"><SvgIcon name="refresh" /> 刷新</button>
      </div>
      <table>
        <thead><tr><th>ID</th><th>机房</th><th>班级</th><th>星期</th><th>节次</th><th>判定</th><th>备注</th><th style="width:100px">操作</th></tr></thead>
        <tbody>
          <tr v-for="r in rules" :key="r.id">
            <td>{{ r.id }}</td>
            <td>{{ roomName(r.room_id) }}</td>
            <td>{{ r.clazz_id ? ('班级#' + r.clazz_id) : '所有班级' }}</td>
            <td>{{ r.weekday ? ('周' + '一二三四五六日'[r.weekday - 1]) : '不限' }}</td>
            <td>{{ r.period || '不限' }}</td>
            <td><span :class="['tag', r.allow ? 'tag-on' : 'tag-off']">{{ r.allow ? '允许进入' : '禁止进入' }}</span></td>
            <td>{{ r.remark || '-' }}</td>
            <td><button class="btn btn-sm btn-danger" @click="delRule(r)"><SvgIcon name="trash" /> </button></td>
          </tr>
          <tr v-if="!rules.length"><td colspan="8" style="text-align:center;color:var(--muted)">暂无规则（默认放行）</td></tr>
        </tbody>
      </table>
    </div>

    <div class="card">
      <h3 style="margin-top:0"><SvgIcon name="card" /> 卡号绑定</h3>
      <div class="toolbar">
        <input class="input" v-model="bind.sno" placeholder="学号 *" style="width:140px">
        <input class="input" v-model="bind.card_no" placeholder="卡号 *" style="width:160px">
        <button class="btn btn-primary" @click="doBind">绑定</button>
      </div>
    </div>

    <div class="card">
      <h3 style="margin-top:0"><SvgIcon name="card" /> 刷卡记录（最近 200 条）</h3>
      <div class="toolbar">
        <select class="input" v-model="evFilter.room_id" @change="loadEvents">
          <option value="">全部机房</option>
          <option v-for="r in rooms" :key="r.id" :value="r.id">{{ r.name }}</option>
        </select>
        <input class="input" v-model="evFilter.sno" placeholder="按学号过滤" style="width:140px" @keyup.enter="loadEvents">
        <button class="btn" @click="loadEvents">筛选</button>
      </div>
      <table>
        <thead><tr><th>ID</th><th>卡号</th><th>学号</th><th>机房</th><th>设备</th><th>方向</th><th>来源</th><th>判定</th><th>签到</th><th>时间</th></tr></thead>
        <tbody>
          <tr v-for="e in events" :key="e.id">
            <td>{{ e.id }}</td>
            <td><code>{{ e.card_no }}</code></td>
            <td>{{ e.student_sno || '—' }}</td>
            <td>{{ e.room_name || '-' }}</td>
            <td>{{ e.device_name || '-' }}</td>
            <td><span class="tag">{{ e.type === 'in' ? '进入' : '离开' }}</span></td>
            <td>{{ e.source === 'card' ? '一卡通' : '门禁' }}</td>
            <td>
              <span v-if="e.allowed === null" style="color:var(--muted)">—</span>
              <span v-else :class="['tag', e.allowed ? 'tag-on' : 'tag-off']">{{ e.allowed ? '放行' : '拒绝' }}</span>
            </td>
            <td>{{ e.check_in_id ? ('#' + e.check_in_id) : '未签到' }}</td>
            <td>{{ fmt(e.created_at) }}</td>
          </tr>
          <tr v-if="!events.length"><td colspan="10" style="text-align:center;color:var(--muted)">暂无刷卡记录</td></tr>
        </tbody>
      </table>
    </div>

    <!-- 新增规则弹窗 -->
    <div v-if="showRule" class="modal-mask" @click.self="showRule = false">
      <div class="modal">
        <h3 style="margin-top:0">新增门禁规则</h3>
        <div class="form-row"><label>机房 *</label>
          <select class="input" v-model="ruleForm.room_id">
            <option v-for="r in rooms" :key="r.id" :value="r.id">{{ r.name }}</option>
          </select>
        </div>
        <div class="form-row"><label>班级</label><input class="input" v-model="ruleForm.clazz_id" placeholder="班级 ID，留空=所有班级"></div>
        <div class="form-row"><label>星期</label>
          <select class="input" v-model.number="ruleForm.weekday">
            <option :value="0">不限</option>
            <option v-for="(w, i) in '一二三四五六日'" :key="i" :value="i + 1">周{{ w }}</option>
          </select>
        </div>
        <div class="form-row"><label>节次</label><input class="input" v-model="ruleForm.period" placeholder="如 1-2节，留空=不限"></div>
        <div class="form-row"><label>判定</label>
          <select class="input" v-model="ruleForm.allow">
            <option :value="true">允许进入</option>
            <option :value="false">禁止进入</option>
          </select>
        </div>
        <div class="form-row"><label>备注</label><input class="input" v-model="ruleForm.remark"></div>
        <div class="modal-actions">
          <button class="btn" @click="showRule = false">取消</button>
          <button class="btn btn-primary" @click="saveRule">保存</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import api from '../api'

const rules = ref([])
const events = ref([])
const rooms = ref([])
const showRule = ref(false)
const ruleForm = ref({ room_id: '', clazz_id: '', weekday: 0, period: '', allow: true, remark: '' })
const bind = ref({ sno: '', card_no: '' })
const evFilter = reactive({ room_id: '', sno: '' })

async function load() {
  const [rs, rs2] = await Promise.all([api.get('/access/rules'), api.get('/rooms')])
  rules.value = rs.data || []
  rooms.value = rs2.data || []
  loadEvents()
}

async function loadEvents() {
  const p = new URLSearchParams()
  if (evFilter.room_id) p.set('room_id', evFilter.room_id)
  if (evFilter.sno) p.set('sno', evFilter.sno)
  const res = await api.get('/access/card-events?' + p)
  events.value = res.data || []
}

function roomName(id) {
  const r = rooms.value.find(x => x.id === id)
  return r ? r.name : ('机房#' + id)
}

function openRule() {
  ruleForm.value = { room_id: rooms.value[0]?.id || '', clazz_id: '', weekday: 0, period: '', allow: true, remark: '' }
  showRule.value = true
}

async function saveRule() {
  if (!ruleForm.value.room_id) { alert('请选择机房'); return }
  const body = { ...ruleForm.value }
  if (!body.clazz_id) body.clazz_id = null
  else body.clazz_id = Number(body.clazz_id)
  try {
    await api.post('/access/rules', body)
    showRule.value = false
    load()
  } catch (e) { alert(e.response?.data?.msg || '保存失败') }
}

async function delRule(r) {
  if (!confirm('删除该门禁规则？')) return
  try { await api.delete('/access/rules/' + r.id); load() }
  catch (e) { alert(e.response?.data?.msg || '删除失败') }
}

async function doBind() {
  if (!bind.value.sno || !bind.value.card_no) { alert('学号与卡号必填'); return }
  try {
    const res = await api.put('/access/bind', bind.value)
    alert('绑定成功：' + (res.data?.student_name || bind.value.sno))
    bind.value = { sno: '', card_no: '' }
  } catch (e) { alert(e.response?.data?.msg || '绑定失败') }
}

function fmt(t) {
  if (!t) return '-'
  return new Date(t).toLocaleString('zh-CN')
}

onMounted(load)
</script>

<style scoped>
.modal-mask { position: fixed; inset: 0; background: rgba(0,0,0,.45); display: flex; align-items: center; justify-content: center; z-index: 100; }
.modal { background: var(--card); border-radius: 8px; padding: 24px; width: 480px; box-shadow: 0 8px 32px rgba(0,0,0,.2); }
.form-row { margin-bottom: 12px; }
.form-row label { display: inline-block; width: 70px; color: var(--text2); }
.input { border: 1px solid var(--border2); border-radius: 4px; padding: 6px 8px; }
.modal-actions { text-align: right; margin-top: 16px; }
code { background: var(--bg2); padding: 1px 6px; border-radius: 3px; }
</style>
