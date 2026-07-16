<template>
  <div class="page-home">
    <van-sticky>
      <div class="recruit-banner">
        <span class="recruit-text">招商入驻 · 诚邀房东合作</span>
        <van-button size="small" round type="warning" @click="$router.push('/apply')" style="font-size:13px;padding:0 16px;">申请入驻</van-button>
      </div>
      <div class="top-bar">
        <div class="top-bar-left" @click="scrollToTop">
          <span class="top-logo">☀️</span>
          <span class="top-title">圳好租</span>
        </div>
        <div class="top-bar-right">
          <span class="login-btn" @click="goToDashboard">登录</span>
        </div>
      </div>
    </van-sticky>
    <div class="filter-bar">
      <div class="filter-tab" :class="{ active: filterOpen }" @click="filterOpen = true">
        <span>{{ filterText }}</span>
        <van-icon name="arrow-down" size="12" />
      </div>
      <span class="filter-count">共 {{ buildings.length }} 栋</span>
    </div>

    <van-popup v-model:show="filterOpen" position="bottom" round :style="{ height: '60vh' }">
      <div class="filter-popup">
        <div class="filter-popup-header">
          <span class="fp-btn" @click="resetFilter">重置</span>
          <span class="fp-title">选择位置</span>
          <span class="fp-btn fp-confirm" @click="confirmFilter">确认</span>
        </div>
        <div class="filter-popup-body">
          <div class="filter-cols">
            <div class="filter-col">
              <div class="filter-col-title">区域</div>
              <div class="filter-col-list">
                <div class="filter-col-item all-item" :class="{ active: !stepDistrict }" @click="stepDistrict = null; stepStreet = null; stepVillage = ''">
                  全部深圳市区
                </div>
                <div v-for="d in shenzhen" :key="d.value" class="filter-col-item" :class="{ active: stepDistrict?.value === d.value }" @click="stepDistrict = d; stepStreet = null; stepVillage = ''">
                  {{ d.label }}
                </div>
              </div>
            </div>
            <div class="filter-col">
              <div class="filter-col-title">街道</div>
              <div class="filter-col-list">
                <div class="filter-col-item all-item" :class="{ active: stepDistrict && !stepStreet }" @click="stepStreet = null; stepVillage = ''">
                  全部街道
                </div>
                <div v-for="s in currentStreets" :key="s.value" class="filter-col-item" :class="{ active: stepStreet?.value === s.value }" @click="stepStreet = s; stepVillage = ''">
                  {{ s.label }}
                </div>
              </div>
            </div>
            <div class="filter-col">
              <div class="filter-col-title">村/小区</div>
              <div class="filter-col-list">
                <div class="filter-col-item all-item" :class="{ active: stepStreet && !stepVillage }" @click="stepVillage = ''">
                  全部村/小区
                </div>
                <div v-for="v in currentVillages" :key="v" class="filter-col-item" :class="{ active: stepVillage === v }" @click="stepVillage = v">
                  {{ v }}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </van-popup>
    
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
            <img v-if="b.cover_image" :src="mediaUrl(b.cover_image)" :alt="b.name" loading="lazy" @error="e => { e.target.onerror = null; e.target.src = '/default-image.svg' }" />
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

        <div v-if="buildings.length < total" style="text-align: center; padding: 12px">
          <van-button :loading="loadingMore" size="small" plain @click="loadMore">加载更多</van-button>
        </div>
        <div v-if="total > 0" style="text-align: center; padding: 0 12px 12px; font-size: 12px; color: #999">
          共 {{ total }} 栋，已显示 {{ buildings.length }} 栋
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
import { getBuildings } from '../api'
import { useAuthStore } from '../stores/auth'
import { useUtils } from '../composables/useUtils'
import shenzhen from '../utils/shenzhen'

const router = useRouter()
const authStore = useAuthStore()
const { mediaUrl, goToDashboard, maskName, maskPhone } = useUtils()
const buildings = ref([])
const loading = ref(true)
const refreshing = ref(false)
const filterOpen = ref(false)
const filterDistrict = ref('')
const filterStreet = ref('')
const filterVillage = ref('')
const stepDistrict = ref(null)
const stepStreet = ref(null)
const stepVillage = ref('')
const currentPage = ref(1)
const total = ref(0)
const pageSize = 20
const loadingMore = ref(false)

const currentStreets = computed(() => {
  if (!stepDistrict.value) return []
  const d = shenzhen.find(x => x.value === stepDistrict.value.value)
  return d ? d.streets : []
})

const currentVillages = computed(() => {
  if (!stepStreet.value) return []
  return stepStreet.value.villages || []
})

const filterText = computed(() => {
  if (filterVillage.value) return `${filterDistrict.value} ${filterStreet.value} ${filterVillage.value}`
  if (filterStreet.value) return `${filterDistrict.value} ${filterStreet.value}`
  if (filterDistrict.value) return filterDistrict.value
  return '全部位置'
})

function resetFilter() {
  stepDistrict.value = null
  stepStreet.value = null
  stepVillage.value = ''
  filterDistrict.value = ''
  filterStreet.value = ''
  filterVillage.value = ''
  filterOpen.value = false
  fetchBuildings()
}

function confirmFilter() {
  filterDistrict.value = stepDistrict.value ? stepDistrict.value.label : ''
  filterStreet.value = stepStreet.value ? stepStreet.value.label : ''
  filterVillage.value = stepVillage.value || ''
  filterOpen.value = false
  fetchBuildings()
}

function scrollToTop() {
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function fetchBuildings(append = false) {
  if (!append) {
    loading.value = true
    currentPage.value = 1
  } else {
    loadingMore.value = true
  }
  try {
    const params = { page: currentPage.value, page_size: pageSize }
    if (filterDistrict.value) params.district = filterDistrict.value
    if (filterStreet.value) params.street = filterStreet.value
    if (filterVillage.value) params.village = filterVillage.value
    const res = await getBuildings(params)
    const data = res.data.buildings || []
    total.value = res.data.total || 0
    if (append) {
      buildings.value = [...buildings.value, ...data]
    } else {
      buildings.value = data
    }
  } catch (e) {
    showToast('加载失败')
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

function loadMore() {
  currentPage.value++
  fetchBuildings(true)
}

async function onRefresh() {
  await fetchBuildings()
  refreshing.value = false
  showToast('刷新成功')
}

onMounted(async () => {
  await fetchBuildings()
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
.login-btn {
  color: #1989fa;
  font-size: 14px;
  font-weight: 500;
  padding: 4px 12px;
  border: 1px solid #1989fa;
  border-radius: 14px;
  cursor: pointer;
}
.filter-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  background: #fff;
  border-bottom: 1px solid #f0f0f0;
}
.filter-tab {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 14px;
  border: 1px solid #e8e8e8;
  border-radius: 16px;
  font-size: 13px;
  color: #666;
  cursor: pointer;
  white-space: nowrap;
}
.filter-tab.active {
  border-color: #1989fa;
  color: #1989fa;
}
.filter-count {
  font-size: 12px;
  color: #999;
  white-space: nowrap;
}
.hero-banner {
  background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
  padding: 28px 20px;
  text-align: center;
  position: relative;
  overflow: hidden;
  min-height: 140px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
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
.filter-popup-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 16px; border-bottom: 1px solid #f0f0f0;
}
.filter-popup-body {
  height: calc(60vh - 53px); overflow: hidden;
}
.fp-btn { font-size: 14px; color: #999; cursor: pointer; }
.fp-confirm { color: #1989fa; font-weight: 600; }
.fp-title { font-size: 16px; font-weight: 600; color: #1a1a2e; }
.filter-cols { display: flex; height: 100%; }
.filter-col { flex: 1; display: flex; flex-direction: column; border-right: 1px solid #f0f0f0; }
.filter-col:last-child { border-right: none; }
.filter-col-title {
  font-size: 12px; color: #999; text-align: center;
  padding: 10px 0; border-bottom: 1px solid #f0f0f0; flex-shrink: 0;
}
.filter-col-list { flex: 1; overflow-y: auto; -webkit-overflow-scrolling: touch; }
.filter-col-item {
  padding: 12px 8px; font-size: 13px; color: #333; text-align: center;
  border-bottom: 1px solid #f5f5f5; cursor: pointer; line-height: 1.3;
}
.filter-col-item.active { color: #fff; background: #1989fa; font-weight: 600; }
.all-item { color: #e6a23c; }
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