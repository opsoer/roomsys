<template>
  <view class="page-rooms">
    <view class="header-row">
      <text class="page-title">房源管理</text>
      <button class="add-btn" @click="openAddDialog">+ 添加房间</button>
    </view>

    <view class="filter-bar">
      <view class="filter-tab" @click="showFilter('status')">
        <text>{{ statusText }}</text><text class="arrow">▼</text>
      </view>
      <view class="filter-tab" @click="showFilter('floor')">
        <text>{{ floorText }}</text><text class="arrow">▼</text>
      </view>
    </view>

    <view v-if="loading" class="skeleton-wrap">
      <view v-for="n in 4" :key="n" class="skeleton-card">
        <view class="sk-img" /><view class="sk-line" style="width:50%;margin-top:8px" /><view class="sk-line" style="width:70%;margin-top:4px" />
      </view>
    </view>

    <view v-else-if="rooms.length === 0" class="empty-wrap"><text>暂无房间数据</text></view>

    <view v-else class="room-list">
      <view v-for="room in rooms" :key="room.id" class="room-card" @click="navTo('/pages/landlord/room-detail?id=' + room.id)">
        <view class="room-card-img">
          <image v-if="room.thumbnail" :src="mediaUrl(room.thumbnail)" mode="aspectFill" />
          <view v-else class="rc-placeholder">📷</view>
          <text :class="['rc-tag', 'tag-' + room.status]">{{ statusLabel(room.status) }}</text>
        </view>
        <view class="room-card-body">
          <text class="rc-number">{{ room.room_number }}</text>
          <text class="rc-info">{{ room.floor }}层 · {{ room.layout }}</text>
          <view v-if="room.rent_price" class="rc-price">¥{{ room.rent_price }}/月</view>
          <text v-if="room.end_date && room.status !== 'vacant'" class="rc-enddate">退租：{{ room.end_date }}</text>
        </view>
      </view>
    </view>

    <view v-if="total > pageSize" class="pagination">
      <button :disabled="page <= 1" @click="page--; fetchRooms()">上一页</button>
      <text>{{ page }} / {{ Math.ceil(total / pageSize) }}</text>
      <button :disabled="page >= Math.ceil(total / pageSize)" @click="page++; fetchRooms()">下一页</button>
    </view>

    <!-- Add Room Dialog -->
    <view v-if="showAddDialog" class="overlay" @click="showAddDialog = false">
      <scroll-view scroll-y class="dialog-panel" @click.stop>
        <text class="dialog-title">添加房间</text>
        <view class="form-group"><text class="form-label">房间号</text><input class="form-input" v-model="addForm.room_number" /></view>
        <view class="form-group"><text class="form-label">楼层</text>
          <picker mode="selector" :range="floorLabels" @change="e => addForm.floor = String(FLOOR_OPTIONS[e.detail.value])">
            <view class="picker-val">{{ addForm.floor ? addForm.floor + '层' : '选择楼层' }}</view>
          </picker>
        </view>
        <view class="form-group"><text class="form-label">户型</text>
          <picker mode="selector" :range="layoutLabels" @change="e => addForm.layout = LAYOUT_OPTIONS[e.detail.value]">
            <view class="picker-val">{{ addForm.layout || '选择户型' }}</view>
          </picker>
        </view>
        <view class="form-group"><text class="form-label">月租金</text><input class="form-input" v-model="addForm.rent_price" type="digit" /></view>
        <view class="form-group"><text class="form-label">押金规则</text>
          <picker mode="selector" :range="['无押金', '押一', '押二', '押三']" @change="e => addForm.deposit_months = e.detail.value">
            <view class="picker-val">{{ addForm.deposit_months != null ? ['无押金', '押一', '押二', '押三'][addForm.deposit_months] : '选择' }}</view>
          </picker>
        </view>
        <view class="form-group"><text class="form-label">管理费</text><input class="form-input" v-model="addForm.management_fee" type="digit" /></view>
        <view class="form-group"><text class="form-label">电费单价</text><input class="form-input" v-model="addForm.electricity_unit_price" type="digit" placeholder="元/度" /></view>
        <view class="form-group"><text class="form-label">水费单价</text><input class="form-input" v-model="addForm.water_unit_price" type="digit" placeholder="元/吨" /></view>
        <view class="dialog-actions">
          <button class="dialog-btn cancel" @click="showAddDialog = false">取消</button>
          <button class="dialog-btn confirm" :disabled="submitting" @click="handleAdd">{{ submitting ? '提交中...' : '确定' }}</button>
        </view>
      </scroll-view>
    </view>

    <!-- Filter Sheet -->
    <view v-if="filterOpen" class="overlay" @click="filterOpen = false">
      <view class="sheet-panel" @click.stop>
        <scroll-view scroll-y class="sheet-list">
          <view v-for="opt in filterOptions" :key="opt.value" class="sheet-item" :class="{ active: opt.value === filterValue }" @click="onFilterSelect(opt.value)">
            <text>{{ opt.label }}</text>
          </view>
        </scroll-view>
        <view class="sheet-cancel" @click="filterOpen = false">取消</view>
      </view>
    </view>
  </view>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { buildingGetRooms, buildingCreateRoom } from '../../api'
import { FLOOR_OPTIONS, LAYOUT_OPTIONS, ROOM_STATUS_OPTIONS } from '../../utils/constants'
import { mediaUrl, statusLabel } from '../../utils/format'

const rooms = ref([])
const loading = ref(true)
const page = ref(1)
const total = ref(0)
const pageSize = 20
const statusFilter = ref('')
const floorFilter = ref('')
const showAddDialog = ref(false)
const submitting = ref(false)
const addForm = ref({ room_number: '', floor: '', layout: '', rent_price: null, deposit_months: null, management_fee: null, electricity_unit_price: null, water_unit_price: null })
const filterOpen = ref(false)
const filterType = ref('')
const filterValue = ref('')

const floorLabels = FLOOR_OPTIONS.map(f => f + '层')
const layoutLabels = LAYOUT_OPTIONS

const statusText = computed(() => {
  if (!statusFilter.value) return '全部状态'
  return ROOM_STATUS_OPTIONS.find(o => o.value === statusFilter.value)?.label || '状态'
})
const floorText = computed(() => floorFilter.value ? floorFilter.value + '层' : '全部楼层')
const filterOptions = computed(() => filterType.value === 'status' ? ROOM_STATUS_OPTIONS : [{ label: '全部楼层', value: '' }, ...FLOOR_OPTIONS.map(f => ({ label: f + '层', value: String(f) }))])

function showFilter(type) {
  filterType.value = type
  filterValue.value = type === 'status' ? statusFilter.value : floorFilter.value
  filterOpen.value = true
}
function onFilterSelect(val) {
  if (filterType.value === 'status') statusFilter.value = val
  else floorFilter.value = val
  filterOpen.value = false
  page.value = 1
  fetchRooms()
}

function navTo(url) { uni.navigateTo({ url }) }

async function fetchRooms() {
  loading.value = true
  try {
    const params = { page: page.value, page_size: pageSize }
    if (statusFilter.value) params.status = statusFilter.value
    if (floorFilter.value) params.floor = floorFilter.value
    const res = await buildingGetRooms(params)
    rooms.value = res.data.rooms || []
    total.value = res.data.total || 0
  } catch {
    uni.showToast({ title: '获取房间列表失败', icon: 'none' })
  } finally {
    loading.value = false
  }
}

async function handleAdd() {
  if (!addForm.value.room_number || !addForm.value.floor || !addForm.value.layout) {
    uni.showToast({ title: '请填写完整', icon: 'none' })
    return
  }
  submitting.value = true
  try {
    await buildingCreateRoom(addForm.value)
    uni.showToast({ title: '添加成功', icon: 'success' })
    showAddDialog.value = false
    addForm.value = { room_number: '', floor: '', layout: '', rent_price: null, deposit_months: null, management_fee: null, electricity_unit_price: null, water_unit_price: null }
    await fetchRooms()
  } catch {
    uni.showToast({ title: '添加失败', icon: 'none' })
  } finally {
    submitting.value = false
  }
}

onMounted(fetchRooms)
</script>

<style scoped>
.page-rooms { padding: 16px; min-height: 100vh; }
.header-row { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; }
.page-title { font-size: 20px; font-weight: 700; color: #1a1a2e; }
.add-btn { background: #1989fa; color: #fff; border: none; border-radius: 8px; padding: 8px 16px; font-size: 14px; }
.filter-bar { display: flex; gap: 8px; margin-bottom: 12px; }
.filter-tab { display: flex; align-items: center; gap: 4px; padding: 6px 14px; border: 1px solid #e8e8e8; border-radius: 16px; font-size: 13px; color: #666; }
.arrow { font-size: 10px; }
.skeleton-wrap { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
.sk-img { height: 120px; background: #e9ecef; border-radius: 8px 8px 0 0; }
.sk-line { height: 14px; background: #e9ecef; border-radius: 4px; margin: 0 10px; }
.empty-wrap { text-align: center; padding: 60px 0; color: #999; }
.room-list { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
.room-card { background: #fff; border-radius: 10px; overflow: hidden; box-shadow: 0 1px 6px rgba(0,0,0,0.05); }
.room-card-img { position: relative; height: 120px; background: #e9ecef; overflow: hidden; }
.room-card-img image { width: 100%; height: 100%; }
.rc-placeholder { height: 100%; display: flex; align-items: center; justify-content: center; font-size: 36px; }
.rc-tag { position: absolute; top: 6px; left: 6px; font-size: 11px; padding: 2px 8px; border-radius: 4px; color: #fff; }
.tag-vacant { background: #67c23a; }
.tag-rented { background: #f56c6c; }
.tag-expiring { background: #e6a23c; }
.room-card-body { padding: 10px 12px 12px; }
.rc-number { font-size: 15px; font-weight: 600; color: #1a1a2e; display: block; }
.rc-info { font-size: 12px; color: #888; display: block; margin-top: 4px; }
.rc-price { font-size: 14px; color: #e6a23c; font-weight: 700; margin-top: 4px; }
.rc-enddate { font-size: 11px; color: #e6a23c; display: block; margin-top: 4px; }
.pagination { display: flex; justify-content: center; align-items: center; gap: 12px; margin-top: 16px; }
.pagination button { background: #fff; border: 1px solid #dcdfe6; border-radius: 6px; padding: 6px 14px; font-size: 13px; }
.pagination button[disabled] { opacity: 0.4; }
.pagination text { font-size: 13px; color: #666; }
.overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); z-index: 1000; display: flex; align-items: flex-end; }
.dialog-panel { width: 100%; background: #fff; border-radius: 16px 16px 0 0; padding: 20px; max-height: 80vh; }
.dialog-title { font-size: 18px; font-weight: 700; display: block; margin-bottom: 16px; }
.form-group { margin-bottom: 12px; }
.form-label { font-size: 14px; color: #333; display: block; margin-bottom: 4px; }
.form-input { width: 100%; height: 40px; border: 1px solid #dcdfe6; border-radius: 8px; padding: 0 12px; font-size: 14px; background: #fff; }
.picker-val { height: 40px; line-height: 40px; border: 1px solid #dcdfe6; border-radius: 8px; padding: 0 12px; font-size: 14px; color: #333; background: #fff; }
.dialog-actions { display: flex; gap: 12px; margin-top: 20px; }
.dialog-btn { flex: 1; height: 44px; border-radius: 22px; display: flex; align-items: center; justify-content: center; font-size: 15px; }
.dialog-btn.cancel { background: #f5f5f5; color: #666; border: none; }
.dialog-btn.confirm { background: #1989fa; color: #fff; border: none; }
.dialog-btn.confirm[disabled] { opacity: 0.6; }
.sheet-panel { width: 100%; background: #fff; border-radius: 16px 16px 0 0; max-height: 60vh; }
.sheet-list { max-height: 50vh; }
.sheet-item { padding: 14px 20px; font-size: 15px; color: #333; }
.sheet-item.active { color: #1989fa; font-weight: 600; }
.sheet-cancel { text-align: center; padding: 14px; border-top: 1px solid #f0f0f0; color: #999; }
</style>
