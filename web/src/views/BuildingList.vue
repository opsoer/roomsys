<template>
  <div class="page-home">
    <header class="site-header">
      <div class="header-inner">
        <div class="logo">
          <span class="logo-icon">☀️</span>
          <span class="logo-text">圳好租</span>
        </div>
        <nav class="nav-links">
          <a class="nav-link active" href="#">首页</a>
          <a class="nav-link" href="#buildings">公寓列表</a>
        </nav>
        <div class="header-actions">
          <el-button size="small" plain @click="goToDashboard">后台管理</el-button>
        </div>
      </div>
    </header>

    <section class="hero-section">
      <div class="hero-bg"></div>
      <div class="hero-content">
        <h1 class="hero-title">圳好租 · 深圳公寓</h1>
        <p class="hero-subtitle">直连房东 · 真实房源 · 即租即住</p>
        <div class="hero-search">
          <el-select v-model="districtFilter" placeholder="选择区域" clearable style="width: 160px" @change="fetchBuildings">
            <el-option label="全部区域" value="" />
            <el-option v-for="d in districts" :key="d" :label="d" :value="d" />
          </el-select>
          <span class="hero-stats-text">共 {{ buildings.length }} 栋公寓</span>
        </div>
      </div>
    </section>

    <section id="buildings" class="buildings-section">
      <div class="section-header">
        <h2>深圳公寓列表</h2>
      </div>

      <div v-if="loading" class="loading-wrap">
        <el-icon class="is-loading" :size="36"><Loading /></el-icon>
        <p>加载中...</p>
      </div>

      <div v-else-if="buildings.length === 0" class="empty-wrap">
        <el-empty description="暂无可租公寓" />
      </div>

      <div v-else class="building-grid">
        <div v-for="b in buildings" :key="b.id" class="building-card" @click="$router.push(`/building/${b.id}`)">
          <div class="building-card-image">
            <img v-if="b.cover_image" :src="mediaUrl(b.cover_image)" :alt="b.name" />
            <div v-else class="building-card-placeholder">
              <el-icon :size="48" color="#ccc"><OfficeBuilding /></el-icon>
            </div>
          </div>
          <div class="building-card-body">
            <h3 class="building-card-name">{{ b.name }}</h3>
            <p class="building-card-addr">
              {{ b.district }} {{ b.street }} {{ b.village }} {{ b.building_no }}
            </p>
            <div class="building-card-room-stats">
              <span class="room-stat-item stat-vacant">
                <el-icon><Select /></el-icon> {{ b.vacant_count }}间可租
              </span>
              <span class="room-stat-item stat-expiring">
                <el-icon><Warning /></el-icon> {{ b.expiring_count }}间即将到期
              </span>
            </div>
            <div v-if="b.landlords && b.landlords.length" class="building-card-landlords">
              <span v-for="l in b.landlords" :key="l.id" class="landlord-tag">
                📞 {{ l.name }} {{ l.phone }}
              </span>
            </div>
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
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Loading, OfficeBuilding, HomeFilled, Select, Warning } from '@element-plus/icons-vue'
import { getBuildings, getDistricts } from '../api'

const router = useRouter()
const buildings = ref([])
const districts = ref([])
const loading = ref(true)
const districtFilter = ref('')

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

async function fetchBuildings() {
  loading.value = true
  try {
    const params = {}
    if (districtFilter.value) params.district = districtFilter.value
    const res = await getBuildings(params)
    buildings.value = res.data.buildings || []
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  try {
    const res = await getDistricts()
    districts.value = res.data.districts || []
  } catch {}
  await fetchBuildings()
})
</script>

<style scoped>
.page-home { min-height: 100vh; background: #f0f2f5; }
.site-header {
  position: fixed; top: 0; left: 0; right: 0; z-index: 100;
  background: rgba(255,255,255,0.92); backdrop-filter: blur(12px);
  border-bottom: 1px solid rgba(0,0,0,0.06);
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
.hero-bg::before {
  content: ''; position: absolute; width: 600px; height: 600px; border-radius: 50%;
  background: radial-gradient(circle,rgba(230,162,60,0.12),transparent 70%);
  top: -200px; right: -100px; animation: float 6s ease-in-out infinite;
}
@keyframes float {
  0%,100% { transform: translate(0,0); }
  50% { transform: translate(30px,-30px); }
}
.hero-content { position: relative; z-index: 1; text-align: center; padding: 60px 24px; }
.hero-title { font-size: 48px; font-weight: 800; color: #fff; margin-bottom: 12px; letter-spacing: 4px; }
.hero-subtitle { font-size: 18px; color: rgba(255,255,255,0.7); margin-bottom: 32px; }
.hero-search { display: inline-flex; align-items: center; gap: 16px; }
.hero-stats-text { color: rgba(255,255,255,0.6); font-size: 14px; }
.buildings-section { max-width: 1200px; margin: 0 auto; padding: 60px 24px; }
.section-header { margin-bottom: 32px; }
.section-header h2 { font-size: 24px; font-weight: 700; color: #1a1a2e; padding-left: 16px; border-left: 4px solid #e6a23c; }
.loading-wrap { text-align: center; padding: 80px 0; color: #999; }
.loading-wrap p { margin-top: 12px; }
.empty-wrap { padding: 60px 0; }
.building-grid { display: grid; grid-template-columns: repeat(auto-fill,minmax(340px,1fr)); gap: 24px; }
.building-card {
  background: #fff; border-radius: 12px; overflow: hidden; cursor: pointer;
  transition: all 0.35s cubic-bezier(0.4,0,0.2,1); box-shadow: 0 2px 12px rgba(0,0,0,0.06);
}
.building-card:hover { transform: translateY(-6px); box-shadow: 0 12px 32px rgba(0,0,0,0.12); }
.building-card-image { height: 200px; background: #e9ecef; overflow: hidden; }
.building-card-image img { width: 100%; height: 100%; object-fit: cover; }
.building-card-placeholder { height: 100%; display: flex; align-items: center; justify-content: center; }
.building-card-body { padding: 20px; }
.building-card-name { font-size: 18px; font-weight: 600; color: #1a1a2e; margin-bottom: 8px; }
.building-card-addr { font-size: 13px; color: #888; margin-bottom: 12px; }
.building-card-room-stats { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 12px; }
.room-stat-item { display: inline-flex; align-items: center; gap: 2px; font-size: 12px; color: #666; background: #f5f7fa; padding: 2px 8px; border-radius: 4px; }
.room-stat-item.stat-vacant { color: #67c23a; background: #f0f9eb; }
.room-stat-item.stat-rented { color: #409eff; background: #ecf5ff; }
.room-stat-item.stat-expiring { color: #e6a23c; background: #fdf6ec; }
.room-stat-item.stat-expired { color: #909399; background: #f4f4f5; }
.building-card-landlords { display: flex; flex-wrap: wrap; gap: 6px; }
.landlord-tag { font-size: 12px; color: #e6a23c; background: #fdf6ec; padding: 2px 8px; border-radius: 4px; }
.site-footer { text-align: center; padding: 32px 24px; color: #aaa; font-size: 13px; border-top: 1px solid #eee; background: #fff; }
@media (max-width: 768px) {
  .header-inner { padding: 0 16px; gap: 16px; }
  .logo-text { font-size: 16px; }
  .nav-links { display: none; }
  .hero-title { font-size: 28px; }
  .hero-subtitle { font-size: 14px; }
  .hero-search { flex-direction: column; }
  .building-grid { grid-template-columns: repeat(2,1fr); gap: 12px; }
  .building-card-image { height: 130px; }
  .building-card-body { padding: 12px; }
  .building-card-name { font-size: 15px; }
  .building-card-room-stats { gap: 4px; }
  .room-stat-item { font-size: 11px; padding: 1px 5px; }
  .landlord-tag { font-size: 11px; }
}
</style>
