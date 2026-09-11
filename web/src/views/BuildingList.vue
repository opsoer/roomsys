<template>
  <div class="page-home">
    <van-sticky>
      <div class="recruit-banner">
        <span class="recruit-text">招商入驻 · 诚邀房东合作</span>
        <van-button size="small" round type="warning" @click="$router.push('/apply')" style="font-size:13px;padding:0 16px;">申请入驻</van-button>
      </div>
      <div class="top-bar">
        <div class="top-bar-left" @click="scrollToTop">
          <span class="top-logo">
            <span class="house-big">🏠</span>
            <span class="house-small">🏠</span>
          </span>
          <span class="top-title">圳好租</span>
        </div>
        <div class="top-bar-right">
          <span class="nav-share" @click="openQrShare">分享</span>
          <span class="login-btn" @click="goToDashboard">{{ authStore.isLoggedIn ? '管理' : '登录' }}</span>
        </div>
      </div>
    </van-sticky>
    <div class="filter-bar">
      <div class="filter-tabs">
        <div class="filter-tab" :class="{ active: locationActive || filterOpen }" @click="openFilter">
          <span>{{ locationText }}</span>
          <van-icon name="arrow-down" size="12" />
        </div>
        <div class="filter-tab" :class="{ active: priceActive }" @click="openFilter">
          <span>{{ priceText }}</span>
          <van-icon name="arrow-down" size="12" />
        </div>
        <div class="filter-tab" :class="{ active: layoutActive }" @click="openFilter">
          <span>{{ layoutText }}</span>
          <van-icon name="arrow-down" size="12" />
        </div>
      </div>
      <div class="filter-bar-right">
        <span class="qr-entry" @click="openLocationQr">
          <van-icon name="qr" size="14" />
          二维码
        </span>
        <span class="filter-count">共 {{ total > 0 ? total : buildings.length }} 栋</span>
      </div>
    </div>

    <van-popup v-model:show="filterOpen" position="bottom" round :style="{ height: '72vh' }">
      <div class="filter-popup">
        <div class="filter-popup-header">
          <span class="fp-btn" @click="resetFilter">重置</span>
          <span class="fp-title">筛选</span>
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
                <div v-for="d in districtOptions" :key="d.value" class="filter-col-item" :class="{ active: stepDistrict?.value === d.value }" @click="stepDistrict = d; stepStreet = null; stepVillage = ''">
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
          <div class="filter-sections">
            <div class="filter-section">
              <div class="filter-section-title">价格（元/月）</div>
              <div class="chip-wrap">
                <span class="chip" :class="{ active: stepMinPrice == null && stepMaxPrice == null }" @click="clearStepPrice">不限</span>
                <span
                  v-for="p in PRICE_PRESETS"
                  :key="p.label"
                  class="chip"
                  :class="{ active: isPresetActive(p) }"
                  @click="togglePreset(p)"
                >{{ p.label }}</span>
              </div>
              <div class="custom-price">
                <span class="cp-label">自定义</span>
                <input
                  class="cp-input"
                  type="number"
                  inputmode="numeric"
                  placeholder="最低价"
                  :value="stepMinPrice == null ? '' : stepMinPrice"
                  @input="onCustomPriceInput('min', $event)"
                />
                <span class="cp-sep">—</span>
                <input
                  class="cp-input"
                  type="number"
                  inputmode="numeric"
                  placeholder="最高价"
                  :value="stepMaxPrice == null ? '' : stepMaxPrice"
                  @input="onCustomPriceInput('max', $event)"
                />
                <span class="cp-unit">元/月</span>
              </div>
            </div>
            <div class="filter-section">
              <div class="filter-section-title">户型</div>
              <div class="chip-wrap">
                <span class="chip" :class="{ active: !stepLayout }" @click="stepLayout = ''">不限</span>
                <span
                  v-for="l in LAYOUT_OPTIONS"
                  :key="l"
                  class="chip"
                  :class="{ active: stepLayout === l }"
                  @click="stepLayout = stepLayout === l ? '' : l"
                >{{ l }}</span>
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
        <van-empty :description="hasAnyFilter ? '没有符合筛选条件的公寓，试试放宽筛选' : '暂无可租公寓'" />
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

        <div v-if="loadingMore" class="load-more-wrap">
          <van-loading size="18" color="#1989fa">加载中...</van-loading>
        </div>
        <div v-else-if="total > 0 && buildings.length >= total" class="load-more-count">
          已全部加载（共 {{ total }} 栋）
        </div>
        <div v-else-if="total > 0" class="load-more-count">
          共 {{ total }} 栋，已显示 {{ buildings.length }} 栋
        </div>
        <div ref="sentinel" class="load-more-sentinel"></div>
      </div>

      <div class="home-footer">
        <p>© 2026 圳好租 · 深圳公寓租赁管理平台</p>
      </div>
    </van-pull-refresh>

    <QrSharePopup v-model:show="qrVisible" :title="qrTitle" :link="qrLink" :data-url="qrDataUrl"
      @copy="copyQrLink" @download="downloadQrCard" />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { showToast } from 'vant'
import { getBuildings } from '../api'
import { siteHomeUrl, locationFilterUrl, generateSiteQrDataUrl, generateLocationQrDataUrl, downloadQrImage } from '../utils/qr'
import { copyText } from '../utils/format'
import { useAuthStore } from '../stores/auth'
import { useLocationStore } from '../stores/locations'
import { useUtils } from '../composables/useUtils'
import { LAYOUT_OPTIONS } from '../utils/constants'
import QrSharePopup from '../components/common/QrSharePopup.vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const { mediaUrl, goToDashboard, maskName, maskPhone } = useUtils()
const buildings = ref([])
const loading = ref(true)
const refreshing = ref(false)
const filterOpen = ref(false)

// 价格区间预设（元/月），min/max 为 null 表示该侧不限制
const PRICE_PRESETS = [
  { label: '1000以下', min: null, max: 1000 },
  { label: '1000-2000', min: 1000, max: 2000 },
  { label: '2000-3000', min: 2000, max: 3000 },
  { label: '3000-5000', min: 3000, max: 5000 },
  { label: '5000以上', min: 5000, max: null },
]

// 已生效的筛选条件（点「确认」后写入）
const filterDistrict = ref('')
const filterStreet = ref('')
const filterVillage = ref('')
const filterMinPrice = ref(null)
const filterMaxPrice = ref(null)
const filterLayout = ref('')
// 弹窗内待确认的筛选条件
const stepDistrict = ref(null)
const stepStreet = ref(null)
const stepVillage = ref('')
const stepMinPrice = ref(null)
const stepMaxPrice = ref(null)
const stepLayout = ref('')

const total = ref(0)
const pageSize = 20
const loadingMore = ref(false)
const sentinel = ref(null)
let observer = null

const qrVisible = ref(false)
const qrTitle = ref('分享主页')
const qrDataUrl = ref('')
const qrLink = ref('')

function openQrShare() {
  qrTitle.value = '分享主页'
  qrVisible.value = true
  qrDataUrl.value = ''
  qrLink.value = siteHomeUrl()
  generateSiteQrDataUrl().then((url) => {
    qrDataUrl.value = url
  }).catch(() => {
    showToast('二维码生成失败')
  })
}

// 位置二维码：把当前筛选条件（区域/街道/村小区 + 租金 + 户型）生成二维码，打印张贴后租客扫码直达筛选结果
function openLocationQr() {
  const district = filterDistrict.value
  const street = filterStreet.value
  const village = filterVillage.value
  const minPrice = filterMinPrice.value
  const maxPrice = filterMaxPrice.value
  const layout = filterLayout.value
  if (!hasAnyFilter.value) {
    showToast('请先选择筛选条件')
    openFilter()
    return
  }
  qrTitle.value = village || street || district || '房源筛选'
  qrVisible.value = true
  qrDataUrl.value = ''
  qrLink.value = locationFilterUrl({ district, street, village, minPrice, maxPrice, layout })
  generateLocationQrDataUrl({ district, street, village, minPrice, maxPrice, layout, total: total.value }).then((url) => {
    qrDataUrl.value = url
  }).catch(() => {
    showToast('二维码生成失败')
  })
}

async function copyQrLink() {
  const ok = await copyText(qrLink.value)
  if (ok) {
    showToast({ message: '已复制链接', duration: 1500 })
  } else {
    showToast('复制失败，请点击上方链接手动复制')
  }
}

function downloadQrCard() {
  if (!qrDataUrl.value) {
    showToast('二维码尚未生成，请稍后重试')
    return
  }
  downloadQrImage(qrDataUrl.value, `${qrTitle.value}二维码.png`)
  showToast({ message: '二维码已保存', duration: 1500 })
}

// 位置选项统一来自 locations store（静态官方表 + 超管自定义/改名 + 公寓实际录入，四处共用）
const locationStore = useLocationStore()

const districtOptions = computed(() => locationStore.fullTree)

const currentStreets = computed(() => (stepDistrict.value ? locationStore.streetsOf(stepDistrict.value.label) : []))

const currentVillages = computed(() =>
  (stepDistrict.value && stepStreet.value) ? locationStore.villagesOf(stepDistrict.value.label, stepStreet.value.label) : []
)

const locationActive = computed(() => !!(filterDistrict.value || filterStreet.value || filterVillage.value))
const priceActive = computed(() => filterMinPrice.value != null || filterMaxPrice.value != null)
const layoutActive = computed(() => !!filterLayout.value)
const hasAnyFilter = computed(() => locationActive.value || priceActive.value || layoutActive.value)

const locationText = computed(() => {
  if (filterVillage.value) return `${filterDistrict.value} ${filterStreet.value} ${filterVillage.value}`
  if (filterStreet.value) return `${filterDistrict.value} ${filterStreet.value}`
  if (filterDistrict.value) return filterDistrict.value
  return '位置'
})

const priceText = computed(() => formatPriceRange(filterMinPrice.value, filterMaxPrice.value) || '租金')

const layoutText = computed(() => filterLayout.value || '户型')

function toNum(v) {
  if (v == null || v === '') return null
  const n = Number(v)
  return Number.isFinite(n) ? n : null
}

function formatPriceRange(min, max) {
  const mn = toNum(min)
  const mx = toNum(max)
  if (mn == null && mx == null) return ''
  if (mn != null && mx != null) return `${mn}-${mx}元`
  if (mn != null) return `${mn}元以上`
  return `${mx}元以下`
}

function isPresetActive(p) {
  return toNum(stepMinPrice.value) === toNum(p.min) && toNum(stepMaxPrice.value) === toNum(p.max)
}

function togglePreset(p) {
  if (isPresetActive(p)) {
    clearStepPrice()
    return
  }
  stepMinPrice.value = p.min
  stepMaxPrice.value = p.max
}

function clearStepPrice() {
  stepMinPrice.value = null
  stepMaxPrice.value = null
}

function onCustomPriceInput(which, e) {
  const v = toNum(e.target.value)
  if (v != null && v < 0) {
    e.target.value = ''
    return
  }
  if (which === 'min') stepMinPrice.value = v
  else stepMaxPrice.value = v
}

// 打开弹窗：把已生效的筛选条件同步到弹窗，避免上次未确认的选择残留
function openFilter() {
  stepDistrict.value = filterDistrict.value
    ? (districtOptions.value.find(d => d.label === filterDistrict.value) || { value: filterDistrict.value, label: filterDistrict.value, streets: [] })
    : null
  stepStreet.value = filterStreet.value
    ? (currentStreets.value.find(s => s.label === filterStreet.value) || { value: filterStreet.value, label: filterStreet.value, villages: [] })
    : null
  stepVillage.value = filterVillage.value || ''
  stepMinPrice.value = filterMinPrice.value
  stepMaxPrice.value = filterMaxPrice.value
  stepLayout.value = filterLayout.value || ''
  filterOpen.value = true
}

function resetFilter() {
  stepDistrict.value = null
  stepStreet.value = null
  stepVillage.value = ''
  stepMinPrice.value = null
  stepMaxPrice.value = null
  stepLayout.value = ''
  filterDistrict.value = ''
  filterStreet.value = ''
  filterVillage.value = ''
  filterMinPrice.value = null
  filterMaxPrice.value = null
  filterLayout.value = ''
  filterOpen.value = false
  fetchBuildings()
}

function confirmFilter() {
  const min = toNum(stepMinPrice.value)
  const max = toNum(stepMaxPrice.value)
  if (min != null && max != null && min > max) {
    showToast('最低价格不能高于最高价格')
    return
  }
  filterDistrict.value = stepDistrict.value ? stepDistrict.value.label : ''
  filterStreet.value = stepStreet.value ? stepStreet.value.label : ''
  filterVillage.value = stepVillage.value || ''
  filterMinPrice.value = min
  filterMaxPrice.value = max
  filterLayout.value = stepLayout.value || ''
  filterOpen.value = false
  fetchBuildings()
}

function scrollToTop() {
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function fetchBuildings(append = false) {
  if (!append) {
    loading.value = true
  } else {
    loadingMore.value = true
  }
  try {
    const params = { page_size: pageSize }
    // 游标分页：加载更多时携带上一批最后一条的 id
    if (append && buildings.value.length > 0) {
      params.last_id = buildings.value[buildings.value.length - 1].id
    }
    if (filterDistrict.value) params.district = filterDistrict.value
    if (filterStreet.value) params.street = filterStreet.value
    if (filterVillage.value) params.village = filterVillage.value
    if (filterMinPrice.value != null) params.min_price = filterMinPrice.value
    if (filterMaxPrice.value != null) params.max_price = filterMaxPrice.value
    if (filterLayout.value) params.layout = filterLayout.value
    const res = await getBuildings(params)
    const data = res.data.buildings || []
    if (!append) total.value = res.data.total || 0
    if (append) {
      // 去重兜底：游标分页偶发重复时避免同一公寓显示多次
      const existingIds = new Set(buildings.value.map(b => b.id))
      const fresh = data.filter(b => !existingIds.has(b.id))
      if (fresh.length === 0) {
        // 没有新数据：把 total 收敛为当前已加载数量，终止触底加载，避免反复请求
        total.value = Math.max(total.value, buildings.value.length)
      } else {
        buildings.value = [...buildings.value, ...fresh]
      }
    } else {
      buildings.value = data
    }
  } catch (e) {
    showToast('加载失败')
  } finally {
    loading.value = false
    loadingMore.value = false
    nextTick(setupInfiniteScroll)
  }
}

function loadMore() {
  fetchBuildings(true)
}

// 触底自动加载：sentinel 元素进入视口附近时加载下一页（类似小红书等 App 的滑动更新）
function setupInfiniteScroll() {
  if (observer) observer.disconnect()
  if (!sentinel.value) return
  observer = new IntersectionObserver((entries) => {
    if (
      entries[0].isIntersecting &&
      !loading.value &&
      !loadingMore.value &&
      buildings.value.length < total.value
    ) {
      loadMore()
    }
  }, { rootMargin: '200px 0px' })
  observer.observe(sentinel.value)
}

async function onRefresh() {
  locationStore.load(true)
  await fetchBuildings()
  refreshing.value = false
  showToast('刷新成功')
}

// 扫码/带链接进入：?district=xx&street=xx&village=xx&min_price=1&max_price=2&layout=xx 直接作为已确认的筛选条件
function applyQueryFilter() {
  const pick = (v) => (typeof v === 'string' && v.trim() ? v.trim() : '')
  filterDistrict.value = pick(route.query.district)
  filterStreet.value = pick(route.query.street)
  filterVillage.value = pick(route.query.village)
  filterMinPrice.value = toNum(route.query.min_price)
  filterMaxPrice.value = toNum(route.query.max_price)
  filterLayout.value = pick(route.query.layout)
}

onMounted(async () => {
  locationStore.load()
  applyQueryFilter()
  await fetchBuildings()
})

onBeforeUnmount(() => {
  if (observer) observer.disconnect()
  observer = null
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
  display: flex;
  align-items: flex-end;
  position: relative;
}
.house-big {
  position: relative;
  z-index: 1;
  font-size: 28px;
  margin-right: -6px;
}
.house-small {
  position: relative;
  z-index: 0;
  font-size: 20px;
  transform: translateY(2px);
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
.nav-share {
  color: #e6a23c;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
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
  gap: 10px;
  padding: 10px 16px;
  background: #fff;
  border-bottom: 1px solid #f0f0f0;
}
.filter-tabs {
  display: flex;
  align-items: center;
  gap: 8px;
  overflow-x: auto;
  scrollbar-width: none;
  -ms-overflow-style: none;
}
.filter-tabs::-webkit-scrollbar { display: none; }
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
.filter-bar-right {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}
.qr-entry {
  display: flex;
  align-items: center;
  gap: 3px;
  font-size: 12px;
  color: #1989fa;
  cursor: pointer;
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
  padding: 16px 16px 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}
.skeleton-card {
  background: #fff;
  border-radius: 12px;
  padding: 12px;
}
.building-list {
  padding: 16px 16px 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}
.load-more-wrap {
  grid-column: 1 / -1;
  text-align: center;
  padding: 12px 0;
}
.load-more-sentinel {
  grid-column: 1 / -1;
  height: 10px;
}
.load-more-count {
  grid-column: 1 / -1;
  text-align: center;
  padding: 0 12px 12px;
  font-size: 12px;
  color: #999;
}
.building-card {
  background: #fff;
  border-radius: 12px;
  overflow: hidden;
  cursor: pointer;
  box-shadow: 0 1px 8px rgba(0,0,0,0.06);
}
.card-img {
  height: 160px;
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
  height: calc(72vh - 53px);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.fp-btn { font-size: 14px; color: #999; cursor: pointer; }
.fp-confirm { color: #1989fa; font-weight: 600; }
.fp-title { font-size: 16px; font-weight: 600; color: #1a1a2e; }
.filter-cols { display: flex; flex: 1; min-height: 0; }
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
.filter-sections {
  flex-shrink: 0;
  max-height: 46%;
  overflow-y: auto;
  border-top: 1px solid #f0f0f0;
  padding: 2px 12px 12px;
  background: #fff;
}
.filter-section + .filter-section { border-top: 1px dashed #f0f0f0; }
.filter-section-title {
  font-size: 12px; color: #999; padding: 10px 0 8px;
}
.chip-wrap { display: flex; flex-wrap: wrap; gap: 8px; }
.chip {
  padding: 5px 12px; font-size: 12px; color: #333;
  background: #f5f6fa; border: 1px solid transparent; border-radius: 14px;
  cursor: pointer; white-space: nowrap;
}
.chip.active {
  color: #1989fa; background: #e8f3ff; border-color: #1989fa; font-weight: 600;
}
.custom-price {
  display: flex; align-items: center; gap: 6px; margin-top: 10px;
}
.cp-label { font-size: 12px; color: #666; flex-shrink: 0; }
.cp-input {
  width: 72px; padding: 4px 8px; font-size: 12px; color: #333;
  border: 1px solid #e5e6eb; border-radius: 6px; outline: none; background: #fff;
}
.cp-input:focus { border-color: #1989fa; }
.cp-input::-webkit-outer-spin-button,
.cp-input::-webkit-inner-spin-button { -webkit-appearance: none; margin: 0; }
.cp-sep { color: #999; font-size: 12px; }
.cp-unit { font-size: 12px; color: #999; }
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

@media (max-width: 768px) {
  .skeleton-wrap {
    grid-template-columns: repeat(2, 1fr);
    gap: 10px;
    padding: 12px 12px 0;
  }
  .building-list {
    grid-template-columns: repeat(2, 1fr);
    gap: 10px;
    padding: 12px 12px 0;
  }
  .card-img { height: 140px; }
}

</style>