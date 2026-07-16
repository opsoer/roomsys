<template>
  <view class="page-home">
    <view class="recruit-banner" @click="navTo('/pages/apply/apply')">
      <text class="recruit-text">招商入驻 · 诚邀房东合作</text>
      <text class="recruit-btn">申请入驻</text>
    </view>

    <view class="hero-banner">
      <text class="hero-title">圳好租 · 深圳公寓</text>
      <text class="hero-subtitle">直连房东 · 真实房源 · 即租即住</text>
    </view>

    <view class="filter-bar">
      <view class="filter-tab" @click="showFilterPopup = true">
        <text>{{ filterText }}</text>
        <text class="arrow">▼</text>
      </view>
      <text class="filter-count">共 {{ buildings.length }} 栋</text>
    </view>

    <view v-if="showFilterPopup" class="filter-overlay" @click="showFilterPopup = false" />
    <view v-if="showFilterPopup" class="filter-popup">
      <view class="filter-popup-header">
        <text class="fp-btn" @click="resetFilter">重置</text>
        <text class="fp-title">选择位置</text>
        <text class="fp-btn fp-confirm" @click="confirmFilter">确认</text>
      </view>
      <view class="filter-popup-body">
        <view class="filter-cols">
          <view class="filter-col">
            <view class="filter-col-title">区域</view>
            <scroll-view class="filter-col-list" scroll-y>
              <view class="filter-col-item all-item" :class="{ active: !stepDistrict }" @click="stepDistrict = null; stepStreet = null; stepVillage = ''">
                <text>全部深圳市区</text>
              </view>
              <view v-for="d in shenzhen" :key="d.value" class="filter-col-item" :class="{ active: stepDistrict?.value === d.value }" @click="stepDistrict = d; stepStreet = null; stepVillage = ''">
                <text>{{ d.label }}</text>
              </view>
            </scroll-view>
          </view>
          <view class="filter-col">
            <view class="filter-col-title">街道</view>
            <scroll-view class="filter-col-list" scroll-y>
              <view class="filter-col-item all-item" :class="{ active: stepDistrict && !stepStreet }" @click="stepStreet = null; stepVillage = ''">
                <text>全部街道</text>
              </view>
              <view v-for="s in currentStreets" :key="s.value" class="filter-col-item" :class="{ active: stepStreet?.value === s.value }" @click="stepStreet = s; stepVillage = ''">
                <text>{{ s.label }}</text>
              </view>
            </scroll-view>
          </view>
          <view class="filter-col">
            <view class="filter-col-title">村/小区</view>
            <scroll-view class="filter-col-list" scroll-y>
              <view class="filter-col-item all-item" :class="{ active: stepStreet && !stepVillage }" @click="stepVillage = ''">
                <text>全部村/小区</text>
              </view>
              <view v-for="v in currentVillages" :key="v" class="filter-col-item" :class="{ active: stepVillage === v }" @click="stepVillage = v">
                <text>{{ v }}</text>
              </view>
            </scroll-view>
          </view>
        </view>
      </view>
    </view>

    <view v-if="loading" class="skeleton-wrap">
      <view v-for="n in 4" :key="n" class="skeleton-card">
        <view class="sk-img" />
        <view class="sk-line" style="width:60%;margin-top:10px" />
        <view class="sk-line" style="width:80%;margin-top:6px" />
        <view class="sk-line" style="width:40%;margin-top:6px" />
      </view>
    </view>

    <view v-else-if="buildings.length === 0" class="empty-wrap">
      <text class="empty-text">暂无可租公寓</text>
    </view>

    <view v-else class="building-list">
      <view v-for="b in buildings" :key="b.id" class="building-card" @click="navTo('/pages/building/building?id=' + b.id)">
        <view class="card-img">
          <image v-if="b.cover_image" :src="mediaUrl(b.cover_image)" mode="aspectFill" class="card-img-full" />
          <view v-else class="card-img-placeholder">
            <text class="ph-icon">🏠</text>
          </view>
        </view>
        <view class="card-body">
          <text class="card-name">{{ b.name }}</text>
          <view class="card-addr">
            <text class="addr-icon">📍</text>
            <text class="addr-text">{{ b.district }} {{ b.street }} {{ b.village }} {{ b.building_no }}</text>
          </view>
          <view class="card-tags">
            <text v-if="b.vacant_count" class="tag tag-success">{{ b.vacant_count }}间可租</text>
            <text v-if="b.expiring_count" class="tag tag-warning">{{ b.expiring_count }}间即将到期</text>
          </view>
          <view v-if="b.landlords && b.landlords.length" class="card-landlords">
            <view v-for="l in b.landlords" :key="l.id" class="landlord-item">
              <text>📞 {{ maskName(l.name) }} {{ maskPhone(l.phone) }}</text>
            </view>
          </view>
        </view>
      </view>
    </view>

    <view v-if="buildings.length < total" class="load-more-wrap">
      <button class="load-more-btn" :disabled="loadingMore" @click="loadMore">{{ loadingMore ? '加载中...' : '加载更多' }}</button>
    </view>
    <view v-if="total > 0" class="load-more-info">
      <text>共 {{ total }} 栋，已显示 {{ buildings.length }} 栋</text>
    </view>

    <view class="page-footer">
      <text>© 2026 圳好租 · 深圳公寓租赁管理平台</text>
    </view>
  </view>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { onPullDownRefresh } from '@dcloudio/uni-app'
import { getBuildings } from '../../api'
import { mediaUrl, maskName, maskPhone } from '../../utils/format'
import shenzhen from '../../utils/shenzhen'

const buildings = ref([])
const loading = ref(true)
const showFilterPopup = ref(false)
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
  showFilterPopup.value = false
  fetchBuildings()
}

function confirmFilter() {
  filterDistrict.value = stepDistrict.value ? stepDistrict.value.label : ''
  filterStreet.value = stepStreet.value ? stepStreet.value.label : ''
  filterVillage.value = stepVillage.value || ''
  showFilterPopup.value = false
  fetchBuildings()
}

function navTo(url) {
  uni.navigateTo({ url })
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
    uni.showToast({ title: '加载失败', icon: 'none' })
  } finally {
    loading.value = false
    loadingMore.value = false
    uni.stopPullDownRefresh()
  }
}

function loadMore() {
  currentPage.value++
  fetchBuildings(true)
}

onPullDownRefresh(() => {
  fetchBuildings()
})

onMounted(() => {
  fetchBuildings()
})
</script>

<style scoped>
.page-home { min-height: 100vh; background: #f5f6fa; padding-bottom: 20px; }
.recruit-banner {
  background: linear-gradient(135deg, #e6a23c, #f56c6c);
  padding: 10px 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}
.recruit-text { color: #fff; font-size: 14px; font-weight: 500; }
.recruit-btn { color: #fff; font-size: 12px; padding: 4px 16px; border: 1px solid rgba(255,255,255,0.6); border-radius: 20px; }
.hero-banner {
  background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
  padding: 28px 20px;
  text-align: center;
}
.hero-title { font-size: 22px; font-weight: 700; color: #fff; letter-spacing: 2px; }
.hero-subtitle { font-size: 13px; color: rgba(255,255,255,0.6); margin-top: 8px; display: block; }
.filter-bar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 10px 16px; background: #fff; border-bottom: 1px solid #f0f0f0;
}
.filter-tab {
  display: flex; align-items: center; gap: 4px;
  padding: 6px 14px; border: 1px solid #e8e8e8; border-radius: 16px;
  font-size: 13px; color: #666;
}
.arrow { font-size: 10px; }
.filter-count { font-size: 12px; color: #999; }
.skeleton-wrap { padding: 12px; display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
.sk-img { height: 120px; background: #e9ecef; border-radius: 8px 8px 0 0; }
.sk-line { height: 14px; background: #e9ecef; border-radius: 4px; }
.empty-wrap { text-align: center; padding: 60px 0; }
.empty-text { color: #999; font-size: 14px; }
.building-list { padding: 12px 12px 0; display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
.building-card {
  background: #fff; border-radius: 12px; overflow: hidden;
  box-shadow: 0 1px 8px rgba(0,0,0,0.06);
}
.card-img { height: 140px; background: #e9ecef; overflow: hidden; }
.card-img-full { width: 100%; height: 100%; }
.card-img-placeholder { height: 100%; display: flex; align-items: center; justify-content: center; }
.ph-icon { font-size: 48px; }
.card-body { padding: 10px 12px 12px; }
.card-name { font-size: 15px; font-weight: 600; color: #1a1a2e; }
.card-addr { display: flex; align-items: center; gap: 3px; margin-top: 6px; }
.addr-icon { font-size: 12px; }
.addr-text { font-size: 13px; color: #888; }
.card-tags { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 8px; }
.tag {
  font-size: 11px; padding: 2px 8px; border-radius: 4px; color: #fff; font-weight: 500;
}
.tag-success { background: #67c23a; }
.tag-warning { background: #e6a23c; }
.tag-danger { background: #f56c6c; }
.card-landlords { margin-top: 6px; }
.landlord-item { font-size: 12px; color: #e6a23c; background: #fdf6ec; padding: 3px 10px; border-radius: 4px; display: inline-block; margin: 2px; }
.filter-overlay {
  position: fixed; top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,0.5); z-index: 999;
}
.filter-popup {
  position: fixed; left: 0; right: 0; bottom: 0;
  background: #fff; border-radius: 16px 16px 0 0;
  height: 60vh; z-index: 1000;
  display: flex; flex-direction: column;
}
.filter-popup-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 16px; border-bottom: 1px solid #f0f0f0; flex-shrink: 0;
}
.fp-btn { font-size: 14px; color: #999; }
.fp-confirm { color: #1989fa; font-weight: 600; }
.fp-title { font-size: 16px; font-weight: 600; color: #1a1a2e; }
.filter-popup-body { flex: 1; overflow: hidden; }
.filter-cols { display: flex; height: 100%; }
.filter-col { flex: 1; display: flex; flex-direction: column; border-right: 1px solid #f0f0f0; }
.filter-col:last-child { border-right: none; }
.filter-col-title {
  font-size: 12px; color: #999; text-align: center;
  padding: 10px 0; border-bottom: 1px solid #f0f0f0; flex-shrink: 0;
}
.filter-col-list { flex: 1; }
.filter-col-item {
  padding: 12px 8px; font-size: 13px; color: #333; text-align: center;
  border-bottom: 1px solid #f5f5f5; line-height: 1.3;
}
.filter-col-item.active { color: #fff; background: #1989fa; font-weight: 600; }
.all-item { color: #e6a23c; }
.load-more-wrap { text-align: center; padding: 12px; }
.load-more-btn { font-size: 13px; padding: 6px 24px; border: 1px solid #1989fa; color: #1989fa; border-radius: 20px; background: #fff; }
.load-more-info { text-align: center; padding: 0 12px 12px; font-size: 12px; color: #999; }
</style>
