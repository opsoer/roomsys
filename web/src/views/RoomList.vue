<template>
  <div class="page-home">
    <div class="section-header">
      <h2>房源管理</h2>
      <div class="section-actions">
        <el-select v-model="statusFilter" placeholder="筛选状态" clearable style="width: 120px" @change="fetchRooms">
          <el-option label="全部" value="" />
          <el-option label="未出租" value="vacant" />
          <el-option label="已预订" value="reserved" />
          <el-option label="已出租" value="rented" />
          <el-option label="即将退租" value="expiring" />
        </el-select>
        <el-select v-model="floorFilter" placeholder="楼层" clearable style="width: 90px" @change="fetchRooms">
          <el-option label="全部楼层" value="" />
          <el-option v-for="f in floorOptions" :key="f" :label="f + '层'" :value="String(f)" />
        </el-select>
        <el-select v-model="layoutFilter" placeholder="户型" clearable style="width: 120px" @change="fetchRooms">
          <el-option label="全部户型" value="" />
          <el-option v-for="lo in layoutOptions" :key="lo" :label="lo" :value="lo" />
        </el-select>
        <el-button type="primary" @click="openBlankAddDialog">
          <el-icon><Plus /></el-icon> 添加房间
        </el-button>
        <el-button type="success" @click="openCopyCreateDialog">
          <el-icon><CopyDocument /></el-icon> 复用房间创建新房间
        </el-button>
      </div>
    </div>

    <div v-if="loading" class="skeleton-wrap">
      <div v-for="n in 6" :key="n" class="skeleton-item">
        <el-skeleton :rows="3" animated>
          <template #template>
            <el-skeleton-item variant="image" style="height: 120px; border-radius: 8px 8px 0 0;" />
            <div style="padding: 12px;">
              <el-skeleton-item variant="h3" style="width: 60%; margin-bottom: 8px;" />
              <el-skeleton-item variant="text" style="width: 40%; margin-bottom: 6px;" />
              <el-skeleton-item variant="text" style="width: 80%;" />
            </div>
          </template>
        </el-skeleton>
      </div>
    </div>

    <div v-else-if="rooms.length === 0" class="empty-wrap">
      <el-empty description="暂无房间数据" />
    </div>

    <div v-else class="room-grid">
      <div v-for="room in rooms" :key="room.id" class="room-card" @click="$router.push(`/landlord/rooms/${room.id}`)">
        <div class="room-card-image">
            <img v-if="room.thumbnail" :src="mediaUrl(room.thumbnail)" :alt="room.room_number" loading="lazy" @error="e => { e.target.onerror = null; e.target.src = '/default-image.svg' }" />
          <div v-else class="room-card-placeholder">
            <el-icon :size="48" color="#ccc"><Picture /></el-icon>
          </div>
          <span class="room-card-tag" :class="'tag-' + room.status">{{ statusLabel(room.status) }}</span>
        </div>
        <div class="room-card-body">
          <h3 class="room-card-number">{{ room.room_number }}</h3>
          <p class="room-card-info">
            <template v-if="room.floor">{{ room.floor }}层</template>
            <template v-if="room.floor && room.layout"> · </template>
            <template v-if="room.layout">{{ room.layout }}</template>
          </p>
          <div class="room-card-price-row" v-if="room.rent_price || room.deposit_months != null">
            <span v-if="room.rent_price" class="room-card-price">¥{{ room.rent_price }}/月</span>
            <span v-if="mgmtFee(room) != null" class="room-card-mgmt">{{ mgmtFee(room) ? '管理费¥' + mgmtFee(room) + '/月' : '无管理费' }}</span>
            <span v-if="room.deposit_months != null" class="room-card-deposit">{{ ['无押金', '押一', '押二', '押三'][room.deposit_months] }}</span>
          </div>
          <p class="room-card-utilities" v-if="room.electricity_unit_price || room.water_unit_price">
            <template v-if="room.electricity_unit_price">电¥{{ room.electricity_unit_price }}/度</template>
            <template v-if="room.electricity_unit_price && room.water_unit_price"> · </template>
            <template v-if="room.water_unit_price">水¥{{ room.water_unit_price }}/吨</template>
          </p>
          <p v-if="room.end_date && room.status !== 'vacant'" class="room-card-enddate">退租日期：{{ room.end_date }}</p>
        </div>
      </div>
    </div>
    <div v-if="loadingMore" style="text-align: center; padding: 16px; color: #999">
      <el-icon class="is-loading"><Loading /></el-icon> 加载中...
    </div>
    <div v-else-if="roomTotal > 0 && rooms.length >= roomTotal" style="text-align: center; padding: 16px; color: #999; font-size: 13px">
      已全部加载（共 {{ roomTotal }} 间）
    </div>
    <div v-else-if="roomTotal > 0" style="text-align: center; padding: 16px; color: #999; font-size: 13px">
      共 {{ roomTotal }} 间，已显示 {{ rooms.length }} 间
    </div>
    <div ref="sentinel" class="load-more-sentinel"></div>

    <el-dialog v-model="showAddDialog" :title="dialogTitle" width="500px">
      <div v-if="addMode === 'copy' && copyCreateStep === 1">
        <div class="copy-create-tip">
          选择一个已有房间作为模板，下一步会自动带出它的全部信息（户型、价格、描述等），只需填写新房间号即可完成创建。
        </div>
        <el-form label-width="90px">
          <el-form-item label="楼层" required>
            <el-select v-model="copySourceFloor" placeholder="选择楼层" style="width: 100%" @change="copySourceRoomId = null">
              <el-option v-for="f in copySourceFloorOptions" :key="f" :label="f + '层'" :value="f" />
            </el-select>
          </el-form-item>
          <el-form-item label="房间号" required>
            <el-select v-model="copySourceRoomId" placeholder="请先选择楼层" style="width: 100%" :disabled="!copySourceFloor">
              <el-option v-for="r in copySourceRoomOptions" :key="r.id" :label="r.room_number" :value="r.id" />
            </el-select>
          </el-form-item>
        </el-form>
      </div>
      <div v-else-if="addMode === 'copy' && copyCreateStep === 2" class="copy-create-tip">
        已带入房间 <strong>{{ copySourceRoomNumber }}</strong> 的全部信息，确认或修改后填写新房间号即可；照片和视频默认复用该房间。
      </div>
      <el-form v-if="addMode !== 'copy' || copyCreateStep === 2" ref="addFormRef" :model="addForm" label-width="90px">
        <el-form-item v-if="addMode === 'copy'" label="批量创建">
          <el-switch v-model="batchCreate" active-text="以相同信息一次创建多个房间" />
        </el-form-item>
        <el-form-item v-if="!(addMode === 'copy' && batchCreate)" label="房间号" prop="room_number" :rules="[{ required: true, message: '请输入房间号' }]">
          <el-input v-model="addForm.room_number" />
        </el-form-item>
        <el-form-item v-else label="房间号" required>
          <div style="width:100%">
            <el-input v-model="batchRoomNumbersText" type="textarea" :rows="4" placeholder="每行一个房间号，也可用逗号、空格分隔，自动去重" />
            <div v-if="batchRoomNumbers.length" class="batch-hint">将创建 {{ batchRoomNumbers.length }} 个房间，均使用上方表单信息</div>
          </div>
        </el-form-item>
        <el-form-item label="楼层" prop="floor" :rules="[{ required: true, message: '请选择楼层' }]">
          <el-select v-model="addForm.floor" placeholder="选择楼层" style="width: 100%">
            <el-option v-for="f in floorOptions" :key="f" :label="f + '层'" :value="String(f)" />
          </el-select>
        </el-form-item>
        <el-form-item label="户型" prop="layout" :rules="[{ required: true, message: '请选择户型' }]">
          <el-select v-model="addForm.layout" placeholder="选择户型" style="width: 100%">
            <el-option v-for="lo in layoutOptions" :key="lo" :label="lo" :value="lo" />
          </el-select>
        </el-form-item>
        <el-divider>价格设置</el-divider>
        <el-form-item label="租金（月）" prop="rent_price" required :rules="[
          { required: true, message: '请输入月租金' },
          { validator: (_, v) => v > 0, message: '租金必须大于0' }
        ]">
          <el-input :model-value="addForm.rent_price" @update:model-value="v => addForm.rent_price = v === '' ? null : Number(v)" type="number" step="0.01" min="0" placeholder="月租金" clearable />
        </el-form-item>
        <el-form-item label="押金规则" prop="deposit_months" required :rules="[
          { required: true, message: '请选择押金规则' },
          { validator: (_, v) => v >= 0 && v <= 3, message: '押金月数范围为0~3' }
        ]">
          <el-select v-model="addForm.deposit_months" placeholder="选择押金规则" style="width:100%">
            <el-option :value="0" label="无押金" />
            <el-option :value="1" label="押一" />
            <el-option :value="2" label="押二" />
            <el-option :value="3" label="押三" />
          </el-select>
        </el-form-item>
        <el-form-item label="管理费" prop="management_fee" required :rules="[
          { required: true, message: '请输入管理费' },
          { validator: (_, v) => v >= 0, message: '管理费不能为负数' }
        ]">
          <el-input :model-value="addForm.management_fee" @update:model-value="v => addForm.management_fee = v === '' ? null : Number(v)" type="number" step="0.01" min="0" placeholder="每月管理费" clearable />
        </el-form-item>
        <el-form-item label="电费单价" prop="electricity_unit_price" required :rules="[
          { required: true, message: '请输入电费单价' },
          { validator: (_, v) => v >= 0, message: '电费单价不能为负数' }
        ]">
          <el-input :model-value="addForm.electricity_unit_price" @update:model-value="v => addForm.electricity_unit_price = v === '' ? null : Number(v)" type="number" step="0.01" min="0" placeholder="元/度" clearable />
        </el-form-item>
        <el-form-item label="水费单价" prop="water_unit_price" required :rules="[
          { required: true, message: '请输入水费单价' },
          { validator: (_, v) => v >= 0, message: '水费单价不能为负数' }
        ]">
          <el-input :model-value="addForm.water_unit_price" @update:model-value="v => addForm.water_unit_price = v === '' ? null : Number(v)" type="number" step="0.01" min="0" placeholder="元/吨" clearable />
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input v-model="addForm.description" type="textarea" :rows="3" />
        </el-form-item>
        <el-form-item label="复用媒体">
          <el-checkbox v-if="addMode === 'copy'" v-model="copyCreateCopyMedia">
            复用房间 {{ copySourceRoomNumber }} 的照片和视频
          </el-checkbox>
          <div v-else style="display:flex;gap:8px;width:100%">
            <el-select v-model="addCopyFloor" placeholder="选择楼层" clearable style="flex:1" @change="addCopyRoom = ''">
              <el-option v-for="f in addCopyFloorOptions" :key="f" :label="f + '层'" :value="f" />
            </el-select>
            <el-select v-model="addCopyRoom" placeholder="选择房间号" clearable style="flex:1" :disabled="!addCopyFloor">
              <el-option v-for="r in addCopyRoomOptions" :key="r.id" :label="r.room_number" :value="r.id" />
            </el-select>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <template v-if="addMode === 'copy' && copyCreateStep === 1">
          <el-button @click="showAddDialog = false">取消</el-button>
          <el-button type="primary" :loading="copySourceLoading" :disabled="!copySourceRoomId" @click="goCopyCreateStep2">下一步</el-button>
        </template>
        <template v-else>
          <el-button v-if="addMode === 'copy'" @click="copyCreateStep = 1">上一步</el-button>
          <el-button v-else @click="showAddDialog = false">取消</el-button>
          <el-button type="primary" :loading="submitting" @click="handleAdd">
            {{ addMode === 'copy' && batchCreate ? (batchRoomNumbers.length ? `批量创建（${batchRoomNumbers.length}）` : '批量创建') : '确定' }}
          </el-button>
        </template>
      </template>
    </el-dialog>

    <el-dialog v-model="showBatchResult" title="批量创建结果" width="420px">
      <div v-if="batchSuccessCount > 0" class="batch-result-summary ok">
        成功创建 {{ batchSuccessCount }} 个房间
      </div>
      <div v-if="batchFailures.length">
        <div class="batch-result-summary fail">以下 {{ batchFailures.length }} 个房间创建失败：</div>
        <div class="batch-failure-list">
          <div v-for="f in batchFailures" :key="f.room_number" class="batch-failure-item">
            <span class="num">{{ f.room_number }}</span>
            <span class="reason">{{ f.reason }}</span>
          </div>
        </div>
      </div>
      <template #footer>
        <el-button type="primary" @click="showBatchResult = false">知道了</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { buildingGetRooms, buildingCreateRoom, buildingGetRoom } from '../api'
import { ElMessage } from 'element-plus'
import { CopyDocument } from '@element-plus/icons-vue'
import { FLOOR_OPTIONS, LAYOUT_OPTIONS } from '../utils/constants'
import { mediaUrl, statusLabel } from '../utils/format'

const floorOptions = FLOOR_OPTIONS
const layoutOptions = LAYOUT_OPTIONS

const addCopyFloorOptions = computed(() => {
  const floors = new Set()
  for (const r of addCopyAllRooms.value) {
    if (r.floor) floors.add(r.floor)
  }
  return [...floors].sort((a, b) => Number(a) - Number(b))
})

const addCopyRoomOptions = computed(() => {
  if (!addCopyFloor.value) return []
  return addCopyAllRooms.value.filter(r => r.floor === addCopyFloor.value)
})

// 复用房间创建：第一步选择源房间（楼层 → 房间号联动）
const copySourceFloorOptions = computed(() => {
  const floors = new Set()
  for (const r of addCopyAllRooms.value) {
    if (r.floor) floors.add(r.floor)
  }
  return [...floors].sort((a, b) => Number(a) - Number(b))
})

const copySourceRoomOptions = computed(() => {
  if (!copySourceFloor.value) return []
  return addCopyAllRooms.value.filter(r => r.floor === copySourceFloor.value)
})

const rooms = ref([])
const loading = ref(true)
const loadingMore = ref(false)
const sentinel = ref(null)
let observer = null
const statusFilter = ref('')
const floorFilter = ref('')
const layoutFilter = ref('')
const showAddDialog = ref(false)
const submitting = ref(false)
const addForm = ref(blankForm())
const addFormRef = ref(null)
const addCopyFloor = ref('')
const addCopyRoom = ref('')
const addCopyAllRooms = ref([])
// 复用房间创建：blank = 普通添加，copy = 以已有房间为模板
const addMode = ref('blank')
const copyCreateStep = ref(1)
const copySourceFloor = ref('')
const copySourceRoomId = ref(null)
const copySourceRoomNumber = ref('')
const copyCreateCopyMedia = ref(true)
const copySourceLoading = ref(false)
// 批量创建：以同一表单信息一次创建多个房间
const batchCreate = ref(false)
const batchRoomNumbersText = ref('')
const showBatchResult = ref(false)
const batchFailures = ref([])
const batchSuccessCount = ref(0)
const roomTotal = ref(0)
const roomPageSize = 20

function blankForm() {
  return { room_number: '', floor: '', layout: '', description: '', rent_price: null, deposit_months: null, management_fee: null, electricity_unit_price: null, water_unit_price: null }
}

// 解析批量房间号：换行/逗号/顿号/分号/空格均可分隔，自动去重
const batchRoomNumbers = computed(() => {
  const parts = batchRoomNumbersText.value.split(/[\n,，、;；\s]+/).map(s => s.trim()).filter(Boolean)
  return [...new Set(parts)]
})

function resetDialogFormState() {
  addForm.value = blankForm()
  addCopyFloor.value = ''
  addCopyRoom.value = ''
  copyCreateStep.value = 1
  copySourceFloor.value = ''
  copySourceRoomId.value = null
  copySourceRoomNumber.value = ''
  copyCreateCopyMedia.value = true
  batchCreate.value = false
  batchRoomNumbersText.value = ''
}

const dialogTitle = computed(() => {
  if (addMode.value === 'copy') {
    return copyCreateStep.value === 1 ? '复用房间创建新房间 · 选择源房间' : '复用房间创建新房间 · 填写新房间信息'
  }
  return '添加房间'
})

async function fetchRooms(append = false) {
  if (!append) {
    loading.value = true
  } else {
    loadingMore.value = true
  }
  try {
    const params = { page_size: roomPageSize }
    if (append && rooms.value.length > 0) {
      params.last_id = rooms.value[rooms.value.length - 1].id
      params.last_key = rooms.value[rooms.value.length - 1].room_number
    }
    if (statusFilter.value) params.status = statusFilter.value
    if (floorFilter.value) params.floor = floorFilter.value
    if (layoutFilter.value) params.layout = layoutFilter.value
    const res = await buildingGetRooms(params)
    const data = res.data.rooms || []
    if (!append) roomTotal.value = res.data.total || 0
    if (append) {
      // 去重兜底：游标分页偶发重复时避免同一房间显示多次
      const existingIds = new Set(rooms.value.map(r => r.id))
      const fresh = data.filter(r => !existingIds.has(r.id))
      if (fresh.length === 0) {
        // 没有新数据：把 total 收敛为当前已加载数量，终止触底加载，避免无匹配房间时反复请求刷屏
        roomTotal.value = Math.max(roomTotal.value, rooms.value.length)
      } else {
        rooms.value = [...rooms.value, ...fresh]
      }
    } else {
      rooms.value = data
    }
  } catch {
    ElMessage.error('获取房间列表失败')
  } finally {
    loading.value = false
    loadingMore.value = false
    nextTick(setupInfiniteScroll)
  }
}

function loadMore() {
  fetchRooms(true)
}

// 触底自动加载：sentinel 进入视口附近时加载下一页
function setupInfiniteScroll() {
  if (observer) observer.disconnect()
  if (!sentinel.value) return
  observer = new IntersectionObserver((entries) => {
    if (
      entries[0].isIntersecting &&
      !loading.value &&
      !loadingMore.value &&
      rooms.value.length < roomTotal.value
    ) {
      loadMore()
    }
  }, { rootMargin: '200px 0px' })
  observer.observe(sentinel.value)
}

function mgmtFee(room) {
  return room.contract_management_fee != null ? room.contract_management_fee : room.management_fee
}

// 拉取全部房间供选择器使用（page_size 上限 100，超过时用游标分页循环取完）
async function fetchAllRoomsForPicker() {
  const all = []
  let lastId = 0
  let lastKey = ''
  try {
    for (let i = 0; i < 20; i++) {
      const params = { page: 1, page_size: 100 }
      if (lastId) {
        params.last_id = lastId
        params.last_key = lastKey
      }
      const res = await buildingGetRooms(params)
      const list = res?.data?.rooms || []
      all.push(...list)
      if (list.length === 0 || all.length >= (res?.data?.total || 0)) break
      lastId = list[list.length - 1].id
      lastKey = list[list.length - 1].room_number
    }
  } catch { /* 网络异常时保留已取到的部分 */ }
  return all
}

async function openBlankAddDialog() {
  addMode.value = 'blank'
  addCopyFloor.value = ''
  addCopyRoom.value = ''
  showAddDialog.value = true
  addCopyAllRooms.value = await fetchAllRoomsForPicker()
}

async function openCopyCreateDialog() {
  addMode.value = 'copy'
  resetDialogFormState()
  showAddDialog.value = true
  addCopyAllRooms.value = await fetchAllRoomsForPicker()
}

// 第二步：取源房间详情，全部带入表单（房间号留空），复用媒体默认勾选
async function goCopyCreateStep2() {
  if (!copySourceRoomId.value) return
  copySourceLoading.value = true
  try {
    const res = await buildingGetRoom(copySourceRoomId.value)
    const room = res?.data?.room
    if (!room) throw new Error('房间不存在')
    copySourceRoomNumber.value = room.room_number || ''
    addForm.value = {
      room_number: '',
      floor: room.floor || '',
      layout: room.layout || '',
      description: room.description || '',
      rent_price: room.rent_price ?? null,
      deposit_months: room.deposit_months ?? null,
      management_fee: room.management_fee ?? null,
      electricity_unit_price: room.electricity_unit_price ?? null,
      water_unit_price: room.water_unit_price ?? null,
    }
    copyCreateCopyMedia.value = true
    copyCreateStep.value = 2
  } catch {
    ElMessage.error('获取房间信息失败')
  } finally {
    copySourceLoading.value = false
  }
}

async function handleAdd() {
  if (addMode.value === 'copy' && batchCreate.value) {
    await handleBatchAdd()
    return
  }
  const valid = await addFormRef.value.validate().catch(() => false)
  if (!valid) return
  submitting.value = true
  try {
    const payload = { ...addForm.value }
    if (addMode.value === 'copy') {
      if (copyCreateCopyMedia.value) payload.copy_from_room_id = copySourceRoomId.value
    } else if (addCopyRoom.value) {
      payload.copy_from_room_id = addCopyRoom.value
    }
    await buildingCreateRoom(payload)
    ElMessage.success('添加成功')
    showAddDialog.value = false
    resetDialogFormState()
    await fetchRooms()
  } catch {
    ElMessage.error('添加房间失败')
  } finally {
    submitting.value = false
  }
}

// 批量创建：逐个房间调用创建接口，允许部分成功；
// 有失败时弹窗列出失败房间号和原因；全部失败时保留创建弹窗便于修改后重试
async function handleBatchAdd() {
  const valid = await addFormRef.value.validate().catch(() => false)
  if (!valid) return
  const numbers = batchRoomNumbers.value
  if (numbers.length === 0) {
    ElMessage.warning('请输入至少一个房间号')
    return
  }
  const tooLong = numbers.filter(n => n.length > 20)
  if (tooLong.length > 0) {
    ElMessage.warning(`房间号不能超过20个字符：${tooLong.join('、')}`)
    return
  }
  submitting.value = true
  const failures = []
  let successCount = 0
  try {
    for (const num of numbers) {
      const payload = { ...addForm.value, room_number: num }
      if (copyCreateCopyMedia.value) payload.copy_from_room_id = copySourceRoomId.value
      try {
        await buildingCreateRoom(payload, { silent: true })
        successCount++
      } catch (err) {
        failures.push({
          room_number: num,
          reason: err?.response?.data?.message || err?.response?.data?.error || '创建失败',
        })
      }
    }
  } finally {
    submitting.value = false
  }
  if (successCount > 0) await fetchRooms()
  if (failures.length === 0) {
    ElMessage.success(`批量创建成功，共 ${successCount} 个房间`)
    showAddDialog.value = false
    resetDialogFormState()
  } else {
    batchFailures.value = failures
    batchSuccessCount.value = successCount
    if (successCount > 0) {
      showAddDialog.value = false
      resetDialogFormState()
    }
    showBatchResult.value = true
  }
}

onMounted(fetchRooms)
onBeforeUnmount(() => {
  if (observer) observer.disconnect()
  observer = null
})
</script>

<style scoped>
.page-home { min-height: 100vh; background: transparent; }
.copy-create-tip { background: #f0f9eb; border: 1px solid #e1f3d8; color: #529b2e; font-size: 13px; line-height: 1.6; border-radius: 6px; padding: 8px 12px; margin-bottom: 16px; }
.batch-hint { margin-top: 6px; font-size: 12px; color: #909399; }
.batch-result-summary { font-size: 14px; margin-bottom: 10px; }
.batch-result-summary.ok { color: #67c23a; }
.batch-result-summary.fail { color: #f56c6c; }
.batch-failure-list { max-height: 240px; overflow-y: auto; border: 1px solid #fde2e2; border-radius: 6px; }
.batch-failure-item { display: flex; justify-content: space-between; align-items: center; gap: 12px; padding: 8px 12px; font-size: 13px; border-bottom: 1px solid #fde2e2; }
.batch-failure-item:last-child { border-bottom: none; }
.batch-failure-item .num { font-weight: 600; color: #303133; }
.batch-failure-item .reason { color: #f56c6c; text-align: right; }
.section-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px; }
.section-header h2 { font-size: 20px; font-weight: 700; color: #1a1a2e; }
.section-actions { display: flex; gap: 10px; }
.skeleton-wrap { display: grid; grid-template-columns: repeat(auto-fill,minmax(270px,1fr)); gap: 24px; padding: 12px 0; }
.skeleton-item { background: #fff; border-radius: 12px; overflow: hidden; }
.empty-wrap { padding: 60px 0; }
.room-grid { display: grid; grid-template-columns: repeat(auto-fill,minmax(270px,1fr)); gap: 24px; }
.load-more-sentinel { grid-column: 1 / -1; height: 10px; }
.room-card { background: #fff; border-radius: 12px; overflow: hidden; cursor: pointer; transition: all 0.35s cubic-bezier(0.4,0,0.2,1); box-shadow: 0 2px 12px rgba(0,0,0,0.06); }
.room-card:hover { transform: translateY(-6px); box-shadow: 0 12px 32px rgba(0,0,0,0.12); }
.room-card-image { position: relative; height: 200px; background: #e9ecef; overflow: hidden; }
.room-card-image img { width: 100%; height: 100%; object-fit: cover; transition: transform 0.5s; }
.room-card:hover .room-card-image img { transform: scale(1.08); }
.room-card-placeholder { height: 100%; display: flex; align-items: center; justify-content: center; }
.room-card-tag { position: absolute; top: 12px; left: 12px; padding: 4px 12px; border-radius: 20px; font-size: 12px; font-weight: 600; color: #fff; }
.tag-vacant { background: rgba(103,194,58,0.85); }
.tag-reserved { background: rgba(64,158,255,0.85); }
.tag-rented { background: rgba(245,108,108,0.85); }
.tag-expiring { background: rgba(230,162,60,0.85); }
.room-card-body { padding: 16px; }
.room-card-number { font-size: 16px; font-weight: 600; color: #1a1a2e; margin-bottom: 6px; }
.room-card-info { font-size: 13px; color: #888; }
.room-card-price-row { margin-top: 6px; display: flex; align-items: center; gap: 8px; }
.room-card-price { font-size: 16px; color: #e6a23c; font-weight: 700; }
.room-card-mgmt { font-size: 12px; color: #909399; background: #f4f4f5; padding: 0 8px; border-radius: 4px; line-height: 20px; }
.room-card-deposit { font-size: 12px; color: #909399; background: #f4f4f5; padding: 0 8px; border-radius: 4px; line-height: 20px; }
.room-card-utilities { margin-top: 2px; font-size: 12px; color: #999; }
.room-card-enddate { margin-top: 6px; font-size: 12px; color: #e6a23c; font-weight: 600; }
@media (max-width: 768px) {
  .section-header { flex-direction: column; align-items: flex-start; gap: 12px; }
  .section-actions { width: 100%; flex-wrap: wrap; }
  .room-grid { grid-template-columns: repeat(2,1fr); gap: 12px; }
  .room-card-image { height: 140px; }
  .room-card-body { padding: 12px; }
  .room-card-number { font-size: 14px; }
}
</style>
