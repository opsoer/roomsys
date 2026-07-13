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
      <view class="filter-tab" @click="showDistrictPicker = true">
        <text>{{ districtText }}</text>
        <text class="arrow">▼</text>
      </view>
      <text class="filter-count">共 {{ buildings.length }} 栋</text>
    </view>

    <picker mode="selector" :range="districtOptions" :value="districtIndex" @change="onDistrictChange" @cancel="showDistrictPicker = false" range-key="text">
      <view />
    </picker>

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
const districtFilter = ref('')
const showDistrictPicker = ref(false)

const districtOptions = computed(() => {
  const opts = [{ text: '全部区域', value: '' }]
  for (const d of shenzhen) {
    opts.push({ text: d.value, value: d.value })
  }
  return opts
})

const districtIndex = computed(() => {
  return districtOptions.value.findIndex(o => o.value === districtFilter.value)
})

const districtText = computed(() => {
  const opt = districtOptions.value.find(o => o.value === districtFilter.value)
  return opt ? opt.text : '区域'
})

function onDistrictChange(e) {
  const idx = e.detail.value
  districtFilter.value = districtOptions.value[idx]?.value || ''
  fetchBuildings()
}

function navTo(url) {
  uni.navigateTo({ url })
}

async function fetchBuildings() {
  loading.value = true
  try {
    const params = {}
    if (districtFilter.value) params.district = districtFilter.value
    const res = await getBuildings(params)
    buildings.value = res.data.buildings || []
  } catch (e) {
    uni.showToast({ title: '加载失败', icon: 'none' })
  } finally {
    loading.value = false
    uni.stopPullDownRefresh()
  }
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
</style>
