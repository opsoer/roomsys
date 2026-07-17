<template>
  <view class="page-building">
    <view v-if="loadError && !building" class="error-wrap">
      <text class="error-icon">⚠️</text>
      <text class="error-text">数据加载失败</text>
      <button class="retry-btn" @click="retryLoad">重新加载</button>
    </view>

    <template v-else-if="building">
      <view class="hero-section">
        <text class="hero-title">{{ building.name }}</text>
        <view class="hero-addr">
          <text>📍 {{ building.district }} {{ building.street }} {{ building.village }} {{ building.building_no }}</text>
        </view>
        <view class="hero-stats">
          <view class="stat-item">
            <text class="stat-num">{{ building.room_count }}</text>
            <text class="stat-lbl">房源</text>
          </view>
          <view class="stat-divider" />
          <view class="stat-item">
            <text class="stat-num success">{{ building.vacant_count }}</text>
            <text class="stat-lbl">可租</text>
          </view>
          <view class="stat-divider" />
          <view class="stat-item">
            <text class="stat-num">{{ building.rented_count }}</text>
            <text class="stat-lbl">已租</text>
          </view>
          <view class="stat-divider" />
          <view class="stat-item">
            <text class="stat-num warning">{{ building.expiring_count }}</text>
            <text class="stat-lbl">将到期</text>
          </view>
        </view>
      </view>

      <view v-if="building.landlords && building.landlords.length" class="landlord-bar">
        <text>📞</text>
        <text v-for="(l, i) in building.landlords" :key="l.id">
          <text v-if="i > 0">、</text>
          {{ l.name }} {{ maskPhone(l.phone) }}
        </text>
      </view>

      <view v-if="building.description" class="desc-section">
        <text class="desc-title">公寓简介</text>
        <text class="desc-text">{{ building.description }}</text>
      </view>

      <view class="filter-bar">
        <view class="filter-tab" @click="showSheet('status')">
          <text>{{ statusText }}</text><text class="arrow">▼</text>
        </view>
        <view class="filter-tab" @click="showSheet('floor')">
          <text>{{ floorText }}</text><text class="arrow">▼</text>
        </view>
        <view class="filter-tab" @click="showSheet('layout')">
          <text>{{ layoutText }}</text><text class="arrow">▼</text>
        </view>
      </view>

      <view v-if="loading" class="loading-wrap">
        <text>加载中...</text>
      </view>

      <view v-else-if="displayRooms.length === 0" class="empty-wrap">
        <text class="empty-text">暂无符合条件的房间</text>
      </view>

      <view v-else class="room-grid">
        <view v-for="room in displayRooms" :key="room.id" class="room-card"
          @click="navTo('/pages/building/room?bid=' + id + '&id=' + room.id)">
          <view class="room-card-img">
            <image v-if="room.thumbnail" :src="mediaUrl(room.thumbnail)" mode="aspectFill" class="rc-img" />
            <view v-else class="rc-img-placeholder">
              <text class="ph-icon">📷</text>
            </view>
            <text :class="['rc-tag', 'tag-' + room.status]">{{ statusLabel(room.status) }}</text>
          </view>
          <view class="room-card-body">
            <text class="rc-number">{{ room.room_number }}</text>
            <text class="rc-info">
              <template v-if="room.floor">{{ room.floor }}层</template>
              <template v-if="room.floor && room.layout"> · </template>
              <template v-if="room.layout">{{ room.layout }}</template>
            </text>
            <view v-if="room.rent_price || room.deposit_months != null" class="rc-price-row">
              <text v-if="room.rent_price" class="rc-price">¥{{ room.rent_price }}/月</text>
              <text v-if="room.deposit_months != null" class="rc-deposit">{{ ['无押金', '押一', '押二', '押三'][room.deposit_months] }}</text>
            </view>
            <view v-if="room.management_fee != null || room.electricity_unit_price || room.water_unit_price" class="rc-utilities">
              <text v-if="room.management_fee != null">管理费:{{ room.management_fee ? '¥' + room.management_fee : '无' }}</text>
              <text v-if="room.electricity_unit_price"> 电¥{{ room.electricity_unit_price }}/度</text>
              <text v-if="room.water_unit_price"> 水¥{{ room.water_unit_price }}/吨</text>
            </view>
            <text v-if="room.end_date && room.status !== 'vacant'" class="rc-enddate">退租：{{ room.end_date }}</text>
          </view>
        </view>
      </view>

      <view v-if="rooms.length < totalRooms" class="load-more-wrap">
        <button class="load-more-btn" :disabled="loadingMore" @click="loadMore">{{ loadingMore ? '加载中...' : '加载更多' }}</button>
      </view>
      <view v-if="totalRooms > 0" class="count-hint">共 {{ totalRooms }} 间，已显示 {{ rooms.length }} 间</view>
    </template>

    <!-- ActionSheet 模拟 -->
    <view v-if="sheetOpen" class="overlay" @click="sheetOpen = false">
      <view class="sheet-panel" @click.stop>
        <scroll-view scroll-y class="sheet-list">
          <view v-for="act in sheetActions" :key="act.value" class="sheet-item" :class="{ active: act.value === sheetSelectedValue }" @click="onSheetSelect(act)">
            <text>{{ act.text }}</text>
            <text v-if="act.value === sheetSelectedValue" class="check">✓</text>
          </view>
        </scroll-view>
        <view class="sheet-cancel" @click="sheetOpen = false">取消</view>
      </view>
    </view>

    <view class="page-footer">
      <text>© 2026 圳好租</text>
    </view>
  </view>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { getBuildingDetail, getBuildingRooms } from '../../api'
import { mediaUrl, statusLabel, maskPhone } from '../../utils/format'
import { auth } from '../../store/auth'

const id = ref('')
const building = ref(null)
const rooms = ref([])
const loading = ref(true)
const loadError = ref(false)
const statusFilter = ref('')
const floorFilter = ref('')
const layoutFilter = ref('')
const currentPage = ref(1)
const totalRooms = ref(0)
const pageSize = 20
const loadingMore = ref(false)

const sheetOpen = ref(false)
const sheetType = ref('')
const sheetSelectedValue = ref('')
const sheetActions = ref([])

const statusOptions = [
  { text: '全部', value: '' },
  { text: '未出租', value: 'vacant' },
  { text: '已预订', value: 'reserved' },
  { text: '已出租', value: 'rented' },
  { text: '即将退租', value: 'expiring' },
]

const floorOptions = computed(() => {
  const opts = [{ text: '全部楼层', value: '' }]
  const floors = [...new Set(rooms.value.map(r => r.floor).filter(Boolean))]
  floors.sort((a, b) => a - b).forEach(f => opts.push({ text: f + '层', value: String(f) }))
  return opts
})

const layoutOptions = computed(() => {
  const opts = [{ text: '全部户型', value: '' }]
  const layouts = [...new Set(rooms.value.map(r => r.layout).filter(Boolean))]
  layouts.forEach(l => opts.push({ text: l, value: l }))
  return opts
})

const displayRooms = computed(() => rooms.value)

const statusText = computed(() => statusOptions.find(o => o.value === statusFilter.value)?.text || '状态')
const floorText = computed(() => floorOptions.value.find(o => o.value === floorFilter.value)?.text || '楼层')
const layoutText = computed(() => layoutOptions.value.find(o => o.value === layoutFilter.value)?.text || '户型')

function showSheet(type) {
  sheetType.value = type
  if (type === 'status') {
    sheetActions.value = statusOptions
    sheetSelectedValue.value = statusFilter.value
  } else if (type === 'floor') {
    sheetActions.value = floorOptions.value
    sheetSelectedValue.value = floorFilter.value
  } else {
    sheetActions.value = layoutOptions.value
    sheetSelectedValue.value = layoutFilter.value
  }
  sheetOpen.value = true
}

function onSheetSelect(action) {
  if (sheetType.value === 'status') statusFilter.value = action.value
  else if (sheetType.value === 'floor') floorFilter.value = action.value
  else if (sheetType.value === 'layout') layoutFilter.value = action.value
  sheetOpen.value = false
  fetchRooms(false)
}

function navTo(url) {
  uni.navigateTo({ url })
}

async function fetchRooms(append = false) {
  if (!append) {
    loading.value = true
    currentPage.value = 1
  } else {
    loadingMore.value = true
  }
  try {
    const params = { page: currentPage.value, page_size: pageSize }
    if (statusFilter.value) params.status = statusFilter.value
    if (floorFilter.value) params.floor = floorFilter.value
    if (layoutFilter.value) params.layout = layoutFilter.value
    const res = await getBuildingRooms(id.value, params)
    const data = res.data.rooms || []
    totalRooms.value = res.data.total || 0
    if (append) {
      rooms.value = [...rooms.value, ...data]
    } else {
      rooms.value = data
    }
  } catch {
    uni.showToast({ title: '加载失败', icon: 'none' })
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

function loadMore() {
  currentPage.value++
  fetchRooms(true)
}

async function retryLoad() {
  loadError.value = false
  loading.value = true
  try {
    const res = await getBuildingDetail(id.value)
    building.value = res.data.building
    await fetchRooms()
  } catch {
    loadError.value = true
    uni.showToast({ title: '加载失败', icon: 'none' })
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  const pages = getCurrentPages()
  const page = pages[pages.length - 1]
  id.value = page.$page?.options?.id || ''
  if (!id.value) {
    loadError.value = true
    return
  }
  try {
    const res = await getBuildingDetail(id.value)
    building.value = res.data.building
  } catch {
    loadError.value = true
    uni.showToast({ title: '加载失败', icon: 'none' })
  }
  await fetchRooms()
})
</script>

<style scoped>
.page-building { min-height: 100vh; background: #f5f6fa; padding-bottom: 20px; }
.error-wrap { text-align: center; padding: 80px 20px; }
.error-icon { font-size: 48px; }
.error-text { display: block; color: #999; margin: 16px 0; }
.retry-btn { padding: 8px 24px; background: #1989fa; color: #fff; border: none; border-radius: 8px; font-size: 14px; }
.hero-section {
  background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
  padding: 24px 20px 28px; text-align: center;
}
.hero-title { font-size: 22px; font-weight: 700; color: #fff; }
.hero-addr { font-size: 13px; color: rgba(255,255,255,0.6); margin: 8px 0 20px; }
.hero-stats {
  display: flex; align-items: center; justify-content: center; gap: 24px;
  background: rgba(255,255,255,0.08); border-radius: 12px; padding: 14px 20px; margin: 0 8px;
}
.stat-item { text-align: center; }
.stat-num { display: block; font-size: 22px; font-weight: 700; color: #e6a23c; }
.stat-num.success { color: #67c23a; }
.stat-num.warning { color: #e6a23c; }
.stat-lbl { font-size: 12px; color: rgba(255,255,255,0.5); }
.stat-divider { width: 1px; height: 28px; background: rgba(255,255,255,0.12); }
.landlord-bar {
  background: #fff; margin: -10px 12px 0; padding: 10px 16px; border-radius: 10px;
  font-size: 12px; color: #e6a23c; display: flex; align-items: center; gap: 6px;
  box-shadow: 0 2px 8px rgba(0,0,0,0.04); position: relative; z-index: 1; flex-wrap: wrap;
}
.desc-section { margin: 14px 12px 0; background: #fff; border-radius: 10px; padding: 16px; }
.desc-title { font-size: 14px; font-weight: 600; color: #333; display: block; margin-bottom: 8px; }
.desc-text { font-size: 13px; color: #666; line-height: 1.7; display: block; }
.filter-bar {
  display: flex; align-items: center; gap: 8px; padding: 10px 12px; background: #fff; border-bottom: 1px solid #f0f0f0;
}
.filter-tab {
  display: flex; align-items: center; gap: 4px; padding: 6px 14px;
  border: 1px solid #e8e8e8; border-radius: 16px; font-size: 13px; color: #666;
}
.arrow { font-size: 10px; }
.loading-wrap { padding: 60px 0; text-align: center; color: #999; }
.empty-wrap { text-align: center; padding: 60px 0; }
.empty-text { color: #999; }
.room-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; padding: 8px 12px 12px; }
.room-card { background: #fff; border-radius: 10px; overflow: hidden; box-shadow: 0 1px 6px rgba(0,0,0,0.05); }
.room-card-img { position: relative; height: 110px; background: #e9ecef; overflow: hidden; }
.rc-img { width: 100%; height: 100%; }
.rc-img-placeholder { height: 100%; display: flex; align-items: center; justify-content: center; }
.ph-icon { font-size: 36px; }
.rc-tag { position: absolute; top: 6px; left: 6px; font-size: 11px; padding: 2px 8px; border-radius: 4px; color: #fff; font-weight: 500; }
.tag-vacant { background: #67c23a; }
.tag-reserved { background: #409eff; }
.tag-rented { background: #f56c6c; }
.tag-expiring { background: #e6a23c; }
.tag-expired { background: #909399; }
.room-card-body { padding: 10px 12px 12px; }
.rc-number { font-size: 15px; font-weight: 600; color: #1a1a2e; display: block; }
.rc-info { font-size: 12px; color: #888; display: block; margin-top: 4px; }
.rc-price-row { margin-top: 4px; display: flex; align-items: center; gap: 6px; }
.rc-price { font-size: 14px; color: #e6a23c; font-weight: 700; }
.rc-deposit { font-size: 11px; color: #909399; background: #f4f4f5; padding: 0 6px; border-radius: 3px; line-height: 18px; }
.rc-utilities { margin-top: 2px; font-size: 11px; color: #999; }
.rc-enddate { margin-top: 4px; font-size: 11px; color: #e6a23c; font-weight: 500; display: block; }
.load-more-wrap { text-align: center; padding: 12px; }
.load-more-btn { font-size: 13px; color: #1989fa; background: none; border: 1px solid #1989fa; border-radius: 20px; padding: 6px 20px; }
.count-hint { text-align: center; font-size: 12px; color: #999; padding: 0 12px 12px; }
.overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); z-index: 1000; display: flex; align-items: flex-end; }
.sheet-panel { width: 100%; background: #fff; border-radius: 16px 16px 0 0; max-height: 60vh; }
.sheet-list { padding: 8px 0; max-height: 50vh; }
.sheet-item { display: flex; justify-content: space-between; align-items: center; padding: 14px 20px; font-size: 15px; color: #333; }
.sheet-item.active { color: #1989fa; font-weight: 600; }
.check { color: #1989fa; }
.sheet-cancel { text-align: center; padding: 14px; border-top: 1px solid #f0f0f0; color: #999; font-size: 15px; }
</style>
