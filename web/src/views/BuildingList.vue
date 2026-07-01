<template>
  <div class="page-home">
    <van-sticky>
      <div class="recruit-banner">
        <span class="recruit-text">📞 {{ recruitPhone ? '房东入驻热线：' + recruitPhone : '招商入驻 · 诚邀房东合作' }}</span>
        <van-button size="small" round type="warning" @click="$router.push('/apply')" style="font-size:13px;padding:0 16px;">申请入驻</van-button>
      </div>
      <div class="top-bar">
        <div class="top-bar-left" @click="scrollToTop">
          <span class="top-logo">☀️</span>
          <span class="top-title">圳好租</span>
        </div>
        <div class="top-bar-right">
          <van-icon name="manager" size="20" @click="goToDashboard" />
        </div>
      </div>
      <div class="filter-bar">
        <van-dropdown-menu>
          <van-dropdown-item v-model="districtFilter" :options="districtOptions" @change="fetchBuildings" />
        </van-dropdown-menu>
        <span class="filter-count">共 {{ buildings.length }} 栋</span>
      </div>
    </van-sticky>
    
    <van-pull-refresh v-model="refreshing" @refresh="onRefresh">
      <div class="hero-banner">
        <h1 class="hero-title">圳好租 · 深圳公寓</h1>
        <p class="hero-subtitle">直连房东 · 真实房源 · 即租即住</p>
      </div>

      <div v-if="loading" class="skeleton-wrap">
        <div v-for="n in 3" :key="n" class="skeleton-card">
          <van-skeleton title :row="3" />
        </div>
      </div>

      <template v-else-if="buildings.length === 0">
        <van-empty description="暂无可租公寓" />
      </template>

      <div v-else class="building-list">
        <div
          v-for="b in buildings"
          :key="b.id"
          class="building-card"
          @click="$router.push(`/building/${b.id}`)"
        >
          <div class="card-img">
            <img v-if="b.cover_image" :src="mediaUrl(b.cover_image)" :alt="b.name" loading="lazy" @error="e => { e.target.onerror = null; e.target.src = '/default-image.png' }" />
            <div v-else class="card-img-placeholder">
              <van-icon name="home-o" size="48" color="#ccc" />
            </div>
          </div>
          <div class="card-body">
            <h3 class="card-name">{{ b.name }}</h3>
            <p class="card-addr">
              <van-icon name="location-o" size="12" />
              {{ b.district }} {{ b.street }} {{ b.village }} {{ b.building_no }}
            </p>
            <div class="card-tags">
              <van-tag v-if="b.vacant_count" type="success" size="medium">
                {{ b.vacant_count }}间可租
              </van-tag>
              <van-tag v-if="b.expiring_count" type="warning" size="medium">
                {{ b.expiring_count }}间即将到期
              </van-tag>
            </div>
            <div v-if="b.landlords && b.landlords.length" class="card-landlords">
              <div v-for="l in b.landlords" :key="l.id" class="landlord-item">
                <van-icon name="phone-o" size="12" />
                {{ maskName(l.name) }} {{ maskPhone(l.phone) }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="home-footer">
        <p>© 2026 圳好租 · 深圳公寓租赁管理平台</p>
      </div>
    </van-pull-refresh>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { showToast } from 'vant'
import { getBuildings, getDistricts, getRecruitPhone } from '../api'
import { useAuthStore } from '../stores/auth'
import { useUtils } from '../composables/useUtils'

const router = useRouter()
const authStore = useAuthStore()
const { mediaUrl, goToDashboard, maskName, maskPhone } = useUtils()
const buildings = ref([])
const districts = ref([])
const loading = ref(true)
const refreshing = ref(false)
const districtFilter = ref('')
const recruitPhone = ref('')

const districtOptions = computed(() => {
  const opts = [{ text: '全部区域', value: '' }]
  for (const d of districts.value) {
    opts.push({ text: d, value: d })
  }
  return opts
})

function scrollToTop() {
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function fetchBuildings() {
  loading.value = true
  try {
    const params = {}
    if (districtFilter.value) params.district = districtFilter.value
    const res = await getBuildings(params)
    buildings.value = res.data.buildings || []
  } catch (e) {
    showToast('加载失败')
  } finally {
    loading.value = false
  }
}

async function onRefresh() {
  await fetchBuildings()
  refreshing.value = false
  showToast('刷新成功')
}

async function fetchRecruit() {
  try {
    const res = await getRecruitPhone()
    recruitPhone.value = res.data?.phone || ''
  } catch {
    ElMessage.error('获取招商信息失败')
  }
}

onMounted(async () => {
  try {
    const res = await getDistricts()
    districts.value = res.data.districts || []
  } catch {}
  await fetchBuildings()
  await fetchRecruit()
})
</script>

<style scoped>
.page-home {
  min-height: 100vh;
  background: #f5f6fa;
  padding-bottom: 20px;
}
.top-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  background: #fff;
}
.top-bar-left {
  display: flex;
  align-items: center;
  gap: 8px;
}
.top-logo {
  font-size: 24px;
}
.top-title {
  font-size: 18px;
  font-weight: 700;
  background: linear-gradient(135deg, #e6a23c, #f56c6c);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}
.top-bar-right {
  display: flex;
  align-items: center;
  gap: 12px;
  color: #666;
}
.filter-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 16px;
  background: #fff;
  border-bottom: 1px solid #f0f0f0;
}
.filter-count {
  font-size: 12px;
  color: #999;
  white-space: nowrap;
}
:deep(.van-dropdown-menu__bar) {
  box-shadow: none;
}
:deep(.van-dropdown-menu__title) {
  font-size: 14px;
}
.hero-banner {
  background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
  padding: 28px 20px;
  text-align: center;
}
.hero-title {
  font-size: 22px;
  font-weight: 700;
  color: #fff;
  margin-bottom: 8px;
  letter-spacing: 2px;
}
.hero-subtitle {
  font-size: 13px;
  color: rgba(255,255,255,0.6);
}
.skeleton-wrap {
  padding: 12px 12px 0;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}
.skeleton-card {
  background: #fff;
  border-radius: 12px;
  padding: 12px;
}
.building-list {
  padding: 12px 12px 0;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}
.building-card {
  background: #fff;
  border-radius: 12px;
  overflow: hidden;
  cursor: pointer;
  box-shadow: 0 1px 8px rgba(0,0,0,0.06);
}
.card-img {
  height: 140px;
  background: #e9ecef;
  overflow: hidden;
}
.card-img img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.card-img-placeholder {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}
.card-body {
  padding: 10px 12px 12px;
}
.card-name {
  font-size: 15px;
  font-weight: 600;
  color: #1a1a2e;
  margin-bottom: 6px;
}
.card-addr {
  font-size: 13px;
  color: #888;
  margin-bottom: 10px;
  display: flex;
  align-items: center;
  gap: 4px;
}
.card-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 8px;
}
.card-landlords {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.landlord-item {
  font-size: 12px;
  color: #e6a23c;
  background: #fdf6ec;
  padding: 3px 10px;
  border-radius: 4px;
  display: inline-flex;
  align-items: center;
  gap: 3px;
}
.home-footer {
  text-align: center;
  padding: 24px 16px;
  color: #aaa;
  font-size: 12px;
}
.recruit-banner {
  background: linear-gradient(135deg, #e6a23c, #f56c6c);
  padding: 10px 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}
.recruit-text {
  color: #fff;
  font-size: 14px;
  font-weight: 500;
}

</style>