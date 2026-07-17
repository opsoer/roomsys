<template>
  <view class="stats-page">
    <text class="page-title">📊 数据看板</text>

    <view class="stats-cards">
      <view class="stat-card gold"><text class="stat-value">{{ formatNum(overview?.total_pv) }}</text><text class="stat-label">总浏览量</text></view>
      <view class="stat-card blue"><text class="stat-value">{{ formatNum(overview?.total_uv) }}</text><text class="stat-label">总访客数</text></view>
      <view class="stat-card red"><text class="stat-value">{{ formatNum(overview?.total_landlord_view) }}</text><text class="stat-label">房东获取</text></view>
      <view class="stat-card green"><text class="stat-value">{{ formatNum(overview?.today_pv) }}</text><text class="stat-label">今日浏览</text></view>
      <view class="stat-card purple"><text class="stat-value">{{ formatNum(overview?.today_uv) }}</text><text class="stat-label">今日访客</text></view>
      <view class="stat-card cyan"><text class="stat-value">{{ overview?.phone_rate != null ? overview.phone_rate.toFixed(1) + '%' : '-' }}</text><text class="stat-label">获电率</text></view>
      <view class="stat-card orange"><text class="stat-value">{{ overview?.vacancy_rate != null ? overview.vacancy_rate.toFixed(1) + '%' : '-' }}</text><text class="stat-label">空置率</text></view>
    </view>

    <view class="card-section">
      <text class="card-title">楼栋热度排行</text>
      <view v-if="overview?.building_rank?.length" class="rank-list">
        <view v-for="(b, i) in overview.building_rank" :key="b.building_id" class="rank-item" @click="showBuildingDetail(b)">
          <text class="rank-idx">{{ i + 1 }}</text>
          <view class="rank-info">
            <text class="rank-name">{{ b.building_name }}</text>
            <text class="rank-stats">浏览 {{ b.pv }} · 访客 {{ b.uv }} · <text :style="{ color: (b.room_count ? (b.vacant_count/b.room_count*100) : 0) > 15 ? '#f56c6c' : '#67c23a' }">空置 {{ b.room_count ? (b.vacant_count/b.room_count*100).toFixed(1) + '%' : '-' }}</text></text>
          </view>
          <text class="rank-arrow">›</text>
        </view>
      </view>
      <text v-else class="no-data">暂无数据</text>
    </view>

    <view class="card-section">
      <text class="card-title">租金定价参考</text>
      <view v-if="priceRef.length" class="price-table">
        <view class="price-header">
          <text class="p-cell">区域</text><text class="p-cell">户型</text><text class="p-cell">均价</text><text class="p-cell">房源</text>
        </view>
        <view v-for="p in priceRef" :key="p.district + p.layout" class="price-row">
          <text class="p-cell">{{ p.district }}</text>
          <text class="p-cell">{{ p.layout }}</text>
          <text class="p-cell gold">¥{{ p.avg_price }}</text>
          <text class="p-cell">{{ p.count }}</text>
        </view>
      </view>
      <text v-else class="no-data">暂无数据</text>
    </view>
  </view>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { adminGetStatsOverview, adminGetPriceReference, adminGetBuildingStats } from '../../api'

const overview = ref(null)
const priceRef = ref([])

function formatNum(n) {
  if (n == null) return '-'
  if (n >= 10000) return (n / 10000).toFixed(1) + '万'
  return n.toLocaleString()
}

async function fetchOverview() {
  try {
    const res = await adminGetStatsOverview()
    overview.value = res.data.overview
  } catch { uni.showToast({ title: '获取概况失败', icon: 'none' }) }
}

async function fetchPriceRef() {
  try {
    const res = await adminGetPriceReference()
    priceRef.value = res.data.price_reference || []
  } catch {}
}

function showBuildingDetail(row) {
  uni.showModal({
    title: row.building_name,
    content: `浏览量: ${row.pv}\n访客数: ${row.uv}\n房东触达: ${row.landlord_view}次`,
    showCancel: false,
  })
}

onMounted(() => {
  fetchOverview()
  fetchPriceRef()
})
</script>

<style scoped>
.stats-page { padding: 16px; min-height: 100vh; }
.page-title { font-size: 20px; font-weight: 700; margin-bottom: 16px; color: #1a1a2e; display: block; }
.stats-cards { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-bottom: 16px; }
.stat-card { background: #fff; border-radius: 10px; padding: 14px; text-align: center; }
.stat-value { font-size: 22px; font-weight: 700; display: block; }
.stat-label { font-size: 12px; color: #999; margin-top: 4px; display: block; }
.gold .stat-value { color: #e6a23c; }
.blue .stat-value { color: #409eff; }
.red .stat-value { color: #f56c6c; }
.green .stat-value { color: #67c23a; }
.purple .stat-value { color: #722ed1; }
.cyan .stat-value { color: #13c2c2; }
.orange .stat-value { color: #fa8c16; }
.card-section { background: #fff; border-radius: 12px; padding: 16px; margin-bottom: 16px; }
.card-title { font-size: 15px; font-weight: 600; color: #333; display: block; margin-bottom: 12px; }
.rank-item { display: flex; align-items: center; gap: 12px; padding: 10px 0; border-bottom: 1px solid #f5f5f5; }
.rank-item:last-child { border-bottom: none; }
.rank-idx { width: 24px; height: 24px; border-radius: 50%; background: #f0f0f0; display: flex; align-items: center; justify-content: center; font-size: 12px; font-weight: 600; color: #666; }
.rank-info { flex: 1; }
.rank-name { font-size: 14px; font-weight: 600; color: #333; display: block; }
.rank-stats { font-size: 12px; color: #999; }
.rank-arrow { font-size: 18px; color: #ccc; }
.price-table { font-size: 13px; }
.price-header, .price-row { display: flex; padding: 8px 0; border-bottom: 1px solid #f0f0f0; }
.price-header { font-weight: 600; color: #666; background: #f9f9f9; border-radius: 6px 6px 0 0; }
.p-cell { flex: 1; text-align: center; }
.gold { color: #e6a23c; font-weight: 600; }
.no-data { display: block; text-align: center; color: #999; padding: 20px; }
</style>
