<template>
  <div class="page-building" v-if="building">
    <header class="site-header">
      <div class="header-inner">
        <div class="logo" @click="$router.push('/')">
          <span class="logo-icon">☀️</span>
          <span class="logo-text">圳好租</span>
        </div>
        <nav class="nav-links">
          <a class="nav-link" href="/">首页</a>
          <a class="nav-link active" href="#">{{ building.name }}</a>
        </nav>
        <div class="header-actions">
          <el-button size="small" plain @click="goToDashboard">后台管理</el-button>
        </div>
      </div>
    </header>

    <section class="hero-section">
      <div class="hero-bg"></div>
      <div class="hero-content">
        <h1 class="hero-title">{{ building.name }}</h1>
        <p class="hero-subtitle">
          {{ building.district }} {{ building.street }} {{ building.village }} {{ building.building_no }}
        </p>
        <div class="hero-stats">
          <div class="stat-item">
            <span class="stat-number">{{ building.room_count }}</span>
            <span class="stat-label">房源总数</span>
          </div>
          <div class="stat-divider"></div>
          <div class="stat-item">
            <span class="stat-number">{{ building.vacant_count }}</span>
            <span class="stat-label">待出租</span>
          </div>
          <div class="stat-divider"></div>
          <div class="stat-item">
            <span class="stat-number">{{ building.rented_count }}</span>
            <span class="stat-label">已出租</span>
          </div>
          <div class="stat-divider"></div>
          <div class="stat-item">
            <span class="stat-number">{{ building.expiring_count }}</span>
            <span class="stat-label">即将退租</span>
          </div>
        </div>
      </div>
    </section>

    <section class="landlord-banner" v-if="building.landlords && building.landlords.length">
      <div class="landlord-inner">
        <span class="landlord-label">房东</span>
        <span v-for="l in building.landlords" :key="l.id" class="landlord-item">
          {{ l.name }} 📞 {{ l.phone }}
        </span>
      </div>
    </section>

    <section class="desc-section" v-if="building.description">
      <div class="desc-content">{{ building.description }}</div>
    </section>

    <section id="rooms" class="rooms-section">
      <div class="section-header">
        <h2>房源展示</h2>
        <div class="section-actions">
          <el-select v-model="statusFilter" placeholder="筛选状态" clearable style="width: 120px" @change="fetchRooms">
            <el-option label="全部" value="" />
            <el-option label="未出租" value="vacant" />
            <el-option label="已出租" value="rented" />
            <el-option label="即将退租" value="expiring" />
          </el-select>
          <el-select v-model="floorFilter" placeholder="楼层" clearable style="width: 90px" @change="fetchRooms">
            <el-option label="全部" value="" />
            <el-option label="1层" value="1" />
            <el-option label="2层" value="2" />
            <el-option label="3层" value="3" />
          </el-select>
          <el-select v-model="layoutFilter" placeholder="户型" clearable style="width: 120px" @change="fetchRooms">
            <el-option label="全部" value="" />
            <el-option label="单间" value="单间" />
            <el-option label="大单间" value="大单间" />
            <el-option label="一室一厅" value="一室一厅" />
          </el-select>
        </div>
      </div>

      <div v-if="loading" class="loading-wrap">
        <el-icon class="is-loading" :size="36"><Loading /></el-icon>
        <p>加载中...</p>
      </div>

      <div v-else-if="rooms.length === 0" class="empty-wrap">
        <el-empty description="暂无房间数据" />
      </div>

      <div v-else class="room-grid">
        <div v-for="room in displayRooms" :key="room.id" class="room-card" @click="$router.push(`/building/${id}/room/${room.id}`)">
          <div class="room-card-image">
            <img v-if="room.thumbnail" :src="mediaUrl(room.thumbnail)" :alt="room.room_number" />
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
            <p v-if="room.end_date" class="room-card-enddate">退租日期：{{ room.end_date }}</p>
          </div>
        </div>
      </div>
    </section>

    <footer class="site-footer">
      <p>© 2026 圳好租 · 深圳公寓租赁管理平台</p>
      <p style="margin-top: 8px; font-size: 12px;">
        <a href="javascript:;" style="color: #ccc; text-decoration: none;" @click="$router.push('/login')">管理登录</a>
      </p>
    </footer>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Loading, Picture } from '@element-plus/icons-vue'
import { getBuildingDetail, getBuildingRooms } from '../api'

const route = useRoute()
const router = useRouter()
const id = computed(() => route.params.id)
const building = ref(null)
const rooms = ref([])
const loading = ref(true)
const statusFilter = ref('')
const floorFilter = ref('')
const layoutFilter = ref('')

const displayRooms = computed(() => {
  if (statusFilter.value) return rooms.value.filter(r => r.status === statusFilter.value)
  return rooms.value.filter(r => r.status !== 'rented')
})

function goToDashboard() {
  const token = localStorage.getItem('token')
  const role = localStorage.getItem('role')
  if (token) {
    router.push(role === 'super_admin' ? '/admin/buildings' : '/landlord/rooms')
  } else {
    router.push('/login')
  }
}

function mediaUrl(path) {
  if (!path) return ''
  return `/api/media/${path}`
}

function statusLabel(s) {
  return s === 'vacant' ? '未出租' : s === 'rented' ? '已出租' : '即将退租'
}

async function fetchRooms() {
  loading.value = true
  try {
    const params = {}
    if (statusFilter.value) params.status = statusFilter.value
    if (floorFilter.value) params.floor = floorFilter.value
    if (layoutFilter.value) params.layout = layoutFilter.value
    const res = await getBuildingRooms(id.value, params)
    rooms.value = res.data.rooms || []
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  try {
    const res = await getBuildingDetail(id.value)
    building.value = res.data.building
  } catch {}
  await fetchRooms()
})
</script>

<style scoped>
.page-building { min-height: 100vh; background: #f0f2f5; }
.site-header {
  position: fixed; top: 0; left: 0; right: 0; z-index: 100;
  background: rgba(255,255,255,0.92); backdrop-filter: blur(12px); border-bottom: 1px solid rgba(0,0,0,0.06);
}
.header-inner {
  max-width: 1200px; margin: 0 auto; padding: 0 24px; height: 64px;
  display: flex; align-items: center; gap: 40px;
}
.logo { display: flex; align-items: center; gap: 8px; cursor: pointer; }
.logo-icon { font-size: 24px; }
.logo-text { font-size: 20px; font-weight: 700; background: linear-gradient(135deg,#e6a23c,#f56c6c); -webkit-background-clip: text; -webkit-text-fill-color: transparent; background-clip: text; }
.nav-links { display: flex; gap: 24px; flex: 1; }
.nav-link { color: #555; text-decoration: none; font-size: 15px; }
.nav-link:hover, .nav-link.active { color: #e6a23c; }
.header-actions { display: flex; align-items: center; gap: 10px; }
.hero-section {
  position: relative; min-height: 360px; display: flex; align-items: center;
  justify-content: center; overflow: hidden; margin-top: 64px;
}
.hero-bg {
  position: absolute; inset: 0;
  background: linear-gradient(135deg,#1a1a2e 0%,#16213e 50%,#0f3460 100%);
}
.hero-content { position: relative; z-index: 1; text-align: center; padding: 60px 24px; }
.hero-title { font-size: 42px; font-weight: 800; color: #fff; margin-bottom: 12px; }
.hero-subtitle { font-size: 16px; color: rgba(255,255,255,0.7); margin-bottom: 32px; }
.hero-stats {
  display: inline-flex; align-items: center; gap: 40px;
  background: rgba(255,255,255,0.08); backdrop-filter: blur(8px);
  border: 1px solid rgba(255,255,255,0.1); border-radius: 16px; padding: 20px 40px;
}
.stat-item { text-align: center; }
.stat-number { display: block; font-size: 28px; font-weight: 700; color: #e6a23c; line-height: 1.2; }
.stat-label { font-size: 13px; color: rgba(255,255,255,0.6); margin-top: 4px; }
.stat-divider { width: 1px; height: 36px; background: rgba(255,255,255,0.15); }
.landlord-banner { max-width: 1200px; margin: -30px auto 0; padding: 0 24px; position: relative; z-index: 2; }
.landlord-inner {
  display: inline-flex; align-items: center; gap: 16px; background: #fff; border-radius: 12px;
  padding: 14px 28px; box-shadow: 0 4px 16px rgba(0,0,0,0.06); flex-wrap: wrap;
}
.landlord-label { color: #999; font-size: 13px; }
.landlord-item { font-size: 14px; color: #e6a23c; font-weight: 500; }
.desc-section { max-width: 1200px; margin: 20px auto 0; padding: 0 24px; }
.desc-content { background: #fff; border-radius: 12px; padding: 20px 24px; color: #555; line-height: 1.8; }
.rooms-section { max-width: 1200px; margin: 0 auto; padding: 40px 24px 60px; }
.section-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px; }
.section-header h2 { font-size: 22px; font-weight: 700; color: #1a1a2e; padding-left: 16px; border-left: 4px solid #e6a23c; }
.section-actions { display: flex; gap: 10px; }
.loading-wrap { text-align: center; padding: 80px 0; color: #999; }
.empty-wrap { padding: 60px 0; }
.room-grid { display: grid; grid-template-columns: repeat(auto-fill,minmax(270px,1fr)); gap: 24px; }
.room-card { background: #fff; border-radius: 12px; overflow: hidden; cursor: pointer; transition: all 0.35s cubic-bezier(0.4,0,0.2,1); box-shadow: 0 2px 12px rgba(0,0,0,0.06); }
.room-card:hover { transform: translateY(-6px); box-shadow: 0 12px 32px rgba(0,0,0,0.12); }
.room-card-image { position: relative; height: 200px; background: #e9ecef; overflow: hidden; }
.room-card-image img { width: 100%; height: 100%; object-fit: cover; transition: transform 0.5s; }
.room-card:hover .room-card-image img { transform: scale(1.08); }
.room-card-placeholder { height: 100%; display: flex; align-items: center; justify-content: center; }
.room-card-tag { position: absolute; top: 12px; left: 12px; padding: 4px 12px; border-radius: 20px; font-size: 12px; font-weight: 600; color: #fff; }
.tag-vacant { background: rgba(103,194,58,0.85); }
.tag-rented { background: rgba(245,108,108,0.85); }
.tag-expiring { background: rgba(230,162,60,0.85); }
.room-card-body { padding: 16px; }
.room-card-number { font-size: 16px; font-weight: 600; color: #1a1a2e; margin-bottom: 6px; }
.room-card-info { font-size: 13px; color: #888; }
.room-card-enddate { margin-top: 6px; font-size: 12px; color: #e6a23c; font-weight: 600; }
.site-footer { text-align: center; padding: 32px 24px; color: #aaa; font-size: 13px; border-top: 1px solid #eee; background: #fff; }
@media (max-width: 768px) {
  .hero-title { font-size: 28px; }
  .hero-stats { flex-direction: column; gap: 12px; padding: 16px 24px; }
  .stat-divider { display: none; }
  .room-grid { grid-template-columns: repeat(2,1fr); gap: 12px; }
  .room-card-image { height: 140px; }
  .room-card-body { padding: 12px; }
  .room-card-number { font-size: 14px; }
  .section-header { flex-direction: column; align-items: flex-start; gap: 12px; }
  .section-actions { width: 100%; flex-wrap: wrap; }
}
</style>
