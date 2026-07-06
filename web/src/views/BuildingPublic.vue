<template>
  <div class="page-building">
    <template v-if="loadError && !building">
      <van-nav-bar title="加载失败" left-arrow @click-left="$router.push('/')" />
      <div style="text-align:center;padding:80px 20px;">
        <van-icon name="warning-o" size="48" color="#999" />
        <p style="color:#999;margin:16px 0;">数据加载失败，请检查网络后重试</p>
        <van-button type="primary" size="small" @click="retryLoad">重新加载</van-button>
      </div>
    </template>
    <template v-else-if="building">
    <van-nav-bar
      :title="building.name"
      left-arrow
      @click-left="$router.push('/')"
    >
      <template #right>
        <van-icon name="manager" size="20" @click="goToDashboard" />
      </template>
    </van-nav-bar>

    <div class="hero-section">
      <h1 class="hero-title">{{ building.name }}</h1>
      <p class="hero-addr">
        <van-icon name="location-o" />
        {{ building.district }} {{ building.street }} {{ building.village }} {{ building.building_no }}
      </p>
      <div class="hero-stats">
        <div class="stat-item">
          <span class="stat-num">{{ building.room_count }}</span>
          <span class="stat-lbl">房源</span>
        </div>
        <div class="stat-divider"></div>
        <div class="stat-item">
          <span class="stat-num success">{{ building.vacant_count }}</span>
          <span class="stat-lbl">可租</span>
        </div>
        <div class="stat-divider"></div>
        <div class="stat-item">
          <span class="stat-num">{{ building.rented_count }}</span>
          <span class="stat-lbl">已租</span>
        </div>
        <div class="stat-divider"></div>
        <div class="stat-item">
          <span class="stat-num warning">{{ building.expiring_count }}</span>
          <span class="stat-lbl">将到期</span>
        </div>
      </div>
    </div>

    <div v-if="building.landlords && building.landlords.length" class="landlord-bar">
      <van-icon name="phone-o" size="14" color="#e6a23c" />
      <span v-for="(l, i) in building.landlords" :key="l.id">
        <template v-if="i > 0">、</template>
        {{ l.name }} {{ maskPhone(l.phone) }}
      </span>
    </div>

    <div v-if="building.description" class="desc-section">
      <div class="desc-title">公寓简介</div>
      <p class="desc-text">{{ building.description }}</p>
    </div>

    <div class="filter-bar">
      <div class="filter-title">房源展示</div>
      <div class="filter-actions">
        <van-dropdown-menu>
          <van-dropdown-item v-model="statusFilter" :options="statusOptions" @change="onFilterChange" />
          <van-dropdown-item v-model="floorFilter" :options="floorOptions" @change="onFilterChange" />
          <van-dropdown-item v-model="layoutFilter" :options="layoutOptions" @change="onFilterChange" />
        </van-dropdown-menu>
      </div>
    </div>

    <div v-if="loading" class="loading-wrap">
      <van-loading size="30" vertical>加载中...</van-loading>
    </div>

    <template v-else-if="displayRooms.length === 0">
      <van-empty description="暂无符合条件的房间" />
    </template>

    <div v-else class="room-grid">
      <div
        v-for="room in displayRooms"
        :key="room.id"
        class="room-card"
        @click="$router.push(`/building/${id}/room/${room.id}`)"
      >
        <div class="room-card-img">
          <img v-if="room.thumbnail" :src="mediaUrl(room.thumbnail)" :alt="room.room_number" loading="lazy" @error="e => { e.target.onerror = null; e.target.src = '/default-image.svg' }" />
          <div v-else class="room-card-img-placeholder">
            <van-icon name="photo-o" size="36" color="#ccc" />
          </div>
          <van-tag
            :type="statusTagType(room.status)"
            class="room-card-tag"
          >
            {{ statusLabel(room.status) }}
          </van-tag>
        </div>
        <div class="room-card-body">
          <h3 class="room-card-number">{{ room.room_number }}</h3>
          <p class="room-card-info">
            <template v-if="room.floor">{{ room.floor }}层</template>
            <template v-if="room.floor && room.layout"> · </template>
            <template v-if="room.layout">{{ room.layout }}</template>
          </p>
          <div v-if="room.rent_price || room.deposit_months != null" class="room-card-price-row">
            <span v-if="room.rent_price" class="room-card-price">¥{{ room.rent_price }}/月</span>
            <span v-if="room.deposit_months != null" class="room-card-deposit">{{ ['无押金', '押一', '押二', '押三'][room.deposit_months] }}</span>
          </div>
          <p v-if="room.management_fee != null || room.electricity_unit_price || room.water_unit_price" class="room-card-utilities">
            <span v-if="room.management_fee != null">管理费:{{ room.management_fee ? '¥' + room.management_fee : '无' }}</span>
            <template v-if="room.management_fee != null && (room.electricity_unit_price || room.water_unit_price)"> · </template>
            <template v-if="room.electricity_unit_price">电¥{{ room.electricity_unit_price }}/度</template>
            <template v-if="room.electricity_unit_price && room.water_unit_price"> · </template>
            <template v-if="room.water_unit_price">水¥{{ room.water_unit_price }}/吨</template>
          </p>
          <p v-if="room.end_date" class="room-card-enddate">
            <van-icon name="clock-o" size="11" />
            退租：{{ room.end_date }}
          </p>
        </div>
      </div>
    </div>

    <div v-if="rooms.length < totalRooms" style="text-align: center; padding: 12px">
      <van-button :loading="loadingMore" size="small" plain @click="loadMore">加载更多</van-button>
    </div>
    <div v-if="totalRooms > 0" style="text-align: center; padding: 0 12px 12px; font-size: 12px; color: #999">
      共 {{ totalRooms }} 间，已显示 {{ rooms.length }} 间
    </div>
    <div class="page-footer">
      <p>© 2026 圳好租 · 深圳公寓租赁管理平台</p>
    </div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showToast } from 'vant'
import { getBuildingDetail, getBuildingRooms } from '../api'
import { mediaUrl, statusLabel, statusTagType, maskPhone } from '../utils/format'
import { useAuthStore } from '../stores/auth'

const route = useRoute()
const router = useRouter()
const id = computed(() => route.params.id)
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

const statusOptions = [
  { text: '全部', value: '' },
  { text: '未出租', value: 'vacant' },
  { text: '已出租', value: 'rented' },
  { text: '即将退租', value: 'expiring' },
]

const floorOptions = computed(() => {
  const opts = [{ text: '全部楼层', value: '' }]
  const floors = new Set(rooms.value.map(r => r.floor).filter(Boolean))
  for (const f of [...floors].sort((a, b) => a - b)) {
    opts.push({ text: `${f}层`, value: String(f) })
  }
  return opts
})

const layoutOptions = computed(() => {
  const opts = [{ text: '全部户型', value: '' }]
  const layouts = new Set(rooms.value.map(r => r.layout).filter(Boolean))
  for (const l of layouts) {
    opts.push({ text: l, value: l })
  }
  return opts
})

const displayRooms = computed(() => rooms.value)

function goToDashboard() {
  const authStore = useAuthStore()
  const token = authStore.token
  const role = authStore.role
  if (token) {
    router.push(role === 'super_admin' ? '/admin/buildings' : '/landlord/rooms')
  } else {
    router.push('/login')
  }
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
    showToast('加载失败')
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

function loadMore() {
  currentPage.value++
  fetchRooms(true)
}

function onFilterChange() {
  fetchRooms(false)
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
    showToast('加载失败，请重试')
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  try {
    const res = await getBuildingDetail(id.value)
    building.value = res.data.building
    loadError.value = false
  } catch {
    loadError.value = true
    showToast('加载失败')
  }
  await fetchRooms()
})
</script>

<style scoped>
.page-building {
  min-height: 100vh;
  background: #f5f6fa;
  padding-bottom: 20px;
}
.hero-section {
  background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
  padding: 24px 20px 28px;
  text-align: center;
}
.hero-title {
  font-size: 22px;
  font-weight: 700;
  color: #fff;
  margin-bottom: 8px;
}
.hero-addr {
  font-size: 13px;
  color: rgba(255,255,255,0.6);
  margin-bottom: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
}
.hero-stats {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 24px;
  background: rgba(255,255,255,0.08);
  border-radius: 12px;
  padding: 14px 20px;
  margin: 0 8px;
}
.stat-item {
  text-align: center;
}
.stat-num {
  display: block;
  font-size: 22px;
  font-weight: 700;
  color: #e6a23c;
  line-height: 1.2;
}
.stat-num.success { color: #67c23a; }
.stat-num.warning { color: #e6a23c; }
.stat-lbl {
  font-size: 12px;
  color: rgba(255,255,255,0.5);
  margin-top: 2px;
}
.stat-divider {
  width: 1px;
  height: 28px;
  background: rgba(255,255,255,0.12);
}
.landlord-bar {
  background: #fff;
  margin: -10px 12px 0;
  padding: 10px 16px;
  border-radius: 10px;
  font-size: 12px;
  color: #e6a23c;
  display: flex;
  align-items: center;
  gap: 6px;
  box-shadow: 0 2px 8px rgba(0,0,0,0.04);
  position: relative;
  z-index: 1;
  flex-wrap: wrap;
}
.desc-section {
  margin: 14px 12px 0;
  background: #fff;
  border-radius: 10px;
  padding: 16px;
}
.desc-title {
  font-size: 14px;
  font-weight: 600;
  color: #333;
  margin-bottom: 8px;
}
.desc-text {
  font-size: 13px;
  color: #666;
  line-height: 1.7;
}
.filter-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 12px 4px;
}
.filter-title {
  font-size: 16px;
  font-weight: 600;
  color: #333;
  white-space: nowrap;
  padding-left: 12px;
  border-left: 3px solid #e6a23c;
}
.filter-actions {
  display: flex;
  align-items: center;
}
:deep(.van-dropdown-menu__bar) {
  box-shadow: none;
  background: transparent;
}
:deep(.van-dropdown-menu__title) {
  font-size: 13px;
}
.loading-wrap {
  padding: 60px 0;
  display: flex;
  justify-content: center;
}
.room-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  padding: 8px 12px 12px;
}
.room-card {
  background: #fff;
  border-radius: 10px;
  overflow: hidden;
  cursor: pointer;
  box-shadow: 0 1px 6px rgba(0,0,0,0.05);
}
.room-card-img {
  position: relative;
  height: 110px;
  background: #e9ecef;
  overflow: hidden;
}
.room-card-img img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.room-card-img-placeholder {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}
.room-card-tag {
  position: absolute;
  top: 6px;
  left: 6px;
}
.room-card-body {
  padding: 10px 12px 12px;
}
.room-card-number {
  font-size: 15px;
  font-weight: 600;
  color: #1a1a2e;
  margin-bottom: 4px;
}
.room-card-info {
  font-size: 12px;
  color: #888;
}
.room-card-price-row { margin-top: 4px; display: flex; align-items: center; gap: 6px; }
.room-card-price {
  font-size: 14px;
  color: #e6a23c;
  font-weight: 700;
}
.room-card-deposit { font-size: 11px; color: #909399; background: #f4f4f5; padding: 0 6px; border-radius: 3px; line-height: 18px; }
.room-card-utilities {
  margin-top: 2px;
  font-size: 11px;
  color: #999;
}
.room-card-enddate {
  margin-top: 4px;
  font-size: 11px;
  color: #e6a23c;
  font-weight: 500;
  display: flex;
  align-items: center;
  gap: 3px;
}
.page-footer {
  text-align: center;
  padding: 24px 16px;
  color: #aaa;
  font-size: 12px;
}
</style>