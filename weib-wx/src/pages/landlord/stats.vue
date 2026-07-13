<template>
  <view class="stats-page">
    <text class="page-title">📊 数据看板</text>

    <view class="stats-cards">
      <view class="stat-card gold"><text class="stat-value">{{ formatNum(stats?.pv) }}</text><text class="stat-label">总浏览量</text></view>
      <view class="stat-card blue"><text class="stat-value">{{ formatNum(stats?.uv) }}</text><text class="stat-label">总访客数</text></view>
      <view class="stat-card red"><text class="stat-value">{{ formatNum(stats?.landlord_view) }}</text><text class="stat-label">房东获取</text></view>
      <view class="stat-card green"><text class="stat-value">{{ formatNum(stats?.today_pv) }}</text><text class="stat-label">今日浏览</text></view>
      <view class="stat-card purple"><text class="stat-value">{{ formatNum(stats?.today_uv) }}</text><text class="stat-label">今日访客</text></view>
      <view class="stat-card cyan"><text class="stat-value">{{ stats?.conversion_rate != null ? stats.conversion_rate.toFixed(1) + '%' : '-' }}</text><text class="stat-label">看房转化率</text></view>
    </view>

    <view class="card-section">
      <text class="card-title">房间热度排行</text>
      <view v-if="stats?.room_rank?.length" class="rank-list">
        <view v-for="(r, i) in stats.room_rank" :key="r.room_number" class="rank-item">
          <text class="rank-idx">{{ i + 1 }}</text>
          <view class="rank-info">
            <text class="rank-name">{{ r.room_number }}</text>
            <text class="rank-stats">{{ r.layout }} · 浏览 {{ r.pv }} · <text :class="r.status === 'vacant' ? 'green' : r.status === 'rented' ? 'red' : 'orange'">{{ { vacant: '空置', rented: '已租', expiring: '将到期' }[r.status] || r.status }}</text></text>
          </view>
        </view>
      </view>
      <text v-else class="no-data">暂无数据</text>
    </view>
  </view>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { buildingGetMyStats } from '../../api'

const stats = ref(null)

function formatNum(n) {
  if (n == null) return '-'
  if (n >= 10000) return (n / 10000).toFixed(1) + '万'
  return n.toLocaleString()
}

async function fetchStats() {
  try {
    const res = await buildingGetMyStats()
    stats.value = res.data.stats
  } catch { uni.showToast({ title: '获取统计数据失败', icon: 'none' }) }
}

onMounted(fetchStats)
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
.card-section { background: #fff; border-radius: 12px; padding: 16px; }
.card-title { font-size: 15px; font-weight: 600; color: #333; display: block; margin-bottom: 12px; }
.rank-item { display: flex; align-items: center; gap: 12px; padding: 10px 0; border-bottom: 1px solid #f5f5f5; }
.rank-item:last-child { border-bottom: none; }
.rank-idx { width: 24px; height: 24px; border-radius: 50%; background: #f0f0f0; display: flex; align-items: center; justify-content: center; font-size: 12px; font-weight: 600; color: #666; }
.rank-info { flex: 1; }
.rank-name { font-size: 14px; font-weight: 600; color: #333; display: block; }
.rank-stats { font-size: 12px; color: #999; }
.rank-stats .green { color: #67c23a; }
.rank-stats .red { color: #f56c6c; }
.rank-stats .orange { color: #e6a23c; }
.no-data { display: block; text-align: center; color: #999; padding: 20px; }
</style>
