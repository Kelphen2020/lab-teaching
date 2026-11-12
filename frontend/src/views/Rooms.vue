<template>
  <div class="card">
    <!-- 新增机房 -->
    <div class="toolbar">
      <input v-model="form.name" placeholder="机房名称" />
      <input v-model="form.building" placeholder="楼栋" style="width:90px" />
      <input v-model="form.floor" placeholder="楼层" style="width:80px" />
      <input v-model.number="form.seat_count" type="number" placeholder="座位数" style="width:90px" />
      <select v-model="form.boot_mode">
        <option value="无盘">无盘</option>
        <option value="本地">本地</option>
        <option value="混合">混合</option>
      </select>
      <button class="btn btn-primary" :disabled="saving" @click="add">{{ saving ? '保存中…' : '+ 新增机房' }}</button>
      <ImportButton endpoint="/rooms" :sample-headers="['name','building','floor','seat_count','net_type','boot_mode','status']"
        :map-row="(cols, h) => ({ name: h.name || cols[0], building: h.building || '', floor: h.floor || '', seat_count: parseInt(h.seat_count) || 60, net_type: h.net_type || '千兆', boot_mode: h.boot_mode || '混合', status: h.status || '启用' })"
        @done="load" />
      <input v-model="keyword" placeholder="搜索名称/楼栋" style="margin-left:auto;width:200px" />
    </div>

    <ListState :state="state" error-text="机房列表加载失败" @retry="load">
      <table>
        <thead>
          <tr><th>ID</th><th>名称</th><th>位置</th><th>座位数</th><th>网络</th><th>启动模式</th><th>状态</th><th>操作</th></tr>
        </thead>
        <tbody>
          <tr v-if="!filtered.length"><td colspan="8" class="empty-cell">没有匹配的机房</td></tr>
          <tr v-for="r in filtered" :key="r.id">
            <td>{{ r.id }}</td>
            <td>{{ r.name }}</td>
            <td>{{ r.building }} {{ r.floor }}</td>
            <td>{{ r.seat_count }}</td>
            <td>{{ r.net_type }}</td>
            <td>{{ r.boot_mode }}</td>
            <td><span class="tag" :class="r.status === '启用' ? 'tag-on' : 'tag-off'">{{ r.status }}</span></td>
            <td class="ops">
              <button class="btn btn-sm" @click="openEdit(r)"><SvgIcon name="pen" /> 编辑</button>
              <button class="btn btn-sm btn-danger" @click="askDel(r)"><SvgIcon name="trash" /> 删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </ListState>

    <!-- 编辑弹窗 -->
    <div v-if="editVisible" class="modal-mask" @click.self="editVisible = false">
      <div class="modal-box">
        <h3>编辑机房 — {{ editForm.name || ('#' + editId) }}</h3>
        <div class="fgrid">
          <label>机房名称<input v-model="editForm.name" /></label>
          <label>楼栋<input v-model="editForm.building" /></label>
          <label>楼层<input v-model="editForm.floor" /></label>
          <label>座位数<input v-model.number="editForm.seat_count" type="number" /></label>
          <label>网络类型
            <select v-model="editForm.net_type">
              <option value="千兆">千兆</option>
              <option value="万兆">万兆</option>
              <option value="百兆">百兆</option>
            </select>
          </label>
          <label>启动模式
            <select v-model="editForm.boot_mode">
              <option value="无盘">无盘</option>
              <option value="本地">本地</option>
              <option value="混合">混合</option>
            </select>
          </label>
          <label>状态
            <select v-model="editForm.status">
              <option value="启用">启用</option>
              <option value="停用">停用</option>
              <option value="维护中">维护中</option>
            </select>
          </label>
        </div>
        <p v-if="editErr" class="err">{{ editErr }}</p>
        <div class="modal-actions">
          <button class="btn" @click="editVisible = false">取消</button>
          <button class="btn btn-primary" :disabled="saving" @click="saveEdit">{{ saving ? '保存中…' : '保存修改' }}</button>
        </div>
      </div>
    </div>

    <!-- 删除二次确认 -->
    <ConfirmDialog
      v-model:visible="delVisible"
      tone="danger"
      title="删除机房"
      :message="delTarget ? `确定删除「${delTarget.name}」吗？该机房下的座位与终端绑定将一并移除，此操作不可恢复。` : ''"
      confirm-text="确认删除"
      @confirm="doDel"
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import api from '../api'
import ListState from '../components/ListState.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import ImportButton from '../components/ImportButton.vue'

const rooms = ref([])
const keyword = ref('')
const state = ref('loading') // loading | empty | error | ready
const saving = ref(false)
const form = ref({ name: '', building: '', floor: '', seat_count: 60, boot_mode: '混合' })

// 编辑
const editVisible = ref(false)
const editId = ref(null)
const editForm = ref({})
const editErr = ref('')

// 删除确认
const delVisible = ref(false)
const delTarget = ref(null)

const filtered = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  if (!k) return rooms.value
  return rooms.value.filter(r =>
    (r.name || '').toLowerCase().includes(k) ||
    (r.building || '').toLowerCase().includes(k))
})

async function load() {
  state.value = 'loading'
  try {
    const res = await api.get('/rooms')
    rooms.value = res.data || []
    state.value = rooms.value.length ? 'ready' : 'empty'
  } catch (e) {
    state.value = 'error'
  }
}

async function add() {
  if (!form.value.name) return alert('请输入机房名称')
  saving.value = true
  try {
    await api.post('/rooms', form.value)
    form.value = { name: '', building: '', floor: '', seat_count: 60, boot_mode: '混合' }
    await load()
  } catch (e) {
    alert('新增失败：' + (e.response?.data?.msg || e.message))
  } finally { saving.value = false }
}

function openEdit(r) {
  editId.value = r.id
  editForm.value = {
    name: r.name, building: r.building, floor: r.floor,
    seat_count: r.seat_count, net_type: r.net_type,
    boot_mode: r.boot_mode, status: r.status,
  }
  editErr.value = ''
  editVisible.value = true
}

async function saveEdit() {
  if (!editForm.value.name) { editErr.value = '机房名称不能为空'; return }
  saving.value = true
  editErr.value = ''
  try {
    await api.put('/rooms/' + editId.value, editForm.value)
    editVisible.value = false
    await load()
  } catch (e) {
    editErr.value = '保存失败：' + (e.response?.data?.msg || e.message)
  } finally { saving.value = false }
}

function askDel(r) {
  delTarget.value = r
  delVisible.value = true
}

async function doDel() {
  try {
    await api.delete('/rooms/' + delTarget.value.id)
    delVisible.value = false
    await load()
  } catch (e) {
    alert('删除失败：' + (e.response?.data?.msg || e.message))
    delVisible.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.empty-cell { text-align: center; color: var(--muted); padding: 24px 0 !important; }
.ops { display: flex; gap: 6px; }
.modal-mask {
  position: fixed; inset: 0; z-index: 190;
  background: rgba(15, 25, 50, .45);
  display: flex; align-items: center; justify-content: center;
}
.modal-box {
  background: var(--card); border-radius: var(--radius);
  box-shadow: 0 12px 48px rgba(20, 40, 90, .22);
  padding: 24px; width: 520px; max-width: calc(100vw - 48px);
}
.modal-box h3 { font-size: 15px; margin-bottom: 16px; }
.fgrid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin-bottom: 12px; }
.fgrid label { display: flex; flex-direction: column; gap: 5px; font-size: 12px; color: var(--text2); }
.fgrid input, .fgrid select { padding: 8px 10px; border: 1px solid var(--border2); border-radius: 6px; font-size: 13px; }
.fgrid input:focus, .fgrid select:focus { border-color: var(--accent); outline: none; }
.modal-actions { display: flex; justify-content: flex-end; gap: 10px; margin-top: 8px; }
</style>
