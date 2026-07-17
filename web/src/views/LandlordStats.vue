<template>
  <div class="stats-page">
    <h2 class="page-title">📊 数据看板</h2>

    <div class="stats-cards">
      <el-card shadow="hover" class="stat-card stat-gold" style="cursor:pointer" @click="selectMetric('pv')">
        <div class="stat-value">{{ formatNum(stats?.pv) }}</div>
        <div class="stat-label">总浏览量</div>
      </el-card>
      <el-card shadow="hover" class="stat-card stat-blue" style="cursor:pointer" @click="selectMetric('uv')">
        <div class="stat-value">{{ formatNum(stats?.uv) }}</div>
        <div class="stat-label">总访客数</div>
      </el-card>
      <el-card shadow="hover" class="stat-card stat-red" style="cursor:pointer" @click="selectMetric('landlord_view')">
        <div class="stat-value">{{ formatNum(stats?.landlord_view) }}</div>
        <div class="stat-label">房东信息获取</div>
      </el-card>
      <el-card shadow="hover" class="stat-card stat-green" style="cursor:pointer" @click="selectMetric('pv')">
        <div class="stat-value">{{ formatNum(stats?.today_pv) }}</div>
        <div class="stat-label">今日浏览</div>
      </el-card>
      <el-card shadow="hover" class="stat-card stat-purple" style="cursor:pointer" @click="selectMetric('uv')">
        <div class="stat-value">{{ formatNum(stats?.today_uv) }}</div>
        <div class="stat-label">今日访客</div>
      </el-card>
      <el-card shadow="hover" class="stat-card stat-cyan" style="cursor:pointer" @click="selectMetric('phone_rate')">
        <div class="stat-value">{{ stats?.phone_rate != null ? stats.phone_rate.toFixed(1) + '%' : '-' }}</div>
        <div class="stat-label">获电率</div>
      </el-card>
    </div>

    <div class="chart-section">
      <el-card v-loading="trendLoading">
        <div class="card-header-row">
          <h4>{{ metricLabel }}趋势</h4>
          <el-select v-model="trendDays" size="small" style="width:120px" @change="fetchTrend">
            <el-option label="近7天" value="7" />
            <el-option label="近30天" value="30" />
            <el-option label="近90天" value="90" />
          </el-select>
        </div>
        <v-chart v-if="trendData.length" :option="trendOption" style="height:320px" autoresize />
        <div v-else style="text-align:center;padding:40px;color:#999">暂无趋势数据</div>
      </el-card>
    </div>

    <el-card>
      <div class="card-header-row">
        <h4>房间热度排行</h4>
      </div>
      <div class="desktop-table">
        <el-table :data="stats?.room_rank || []" border stripe size="small">
          <el-table-column type="index" label="#" width="50" />
          <el-table-column prop="room_number" label="房间号" width="100" />
          <el-table-column prop="layout" label="户型" width="100" />
          <el-table-column prop="pv" label="浏览" width="80" sortable />
          <el-table-column prop="status" label="状态" width="80">
            <template #default="{ row }">
              <el-tag :type="row.status === 'vacant' ? 'success' : row.status === 'rented' ? 'danger' : 'warning'" size="small">
                {{ { vacant: '空置', rented: '已租', expiring: '将到期' }[row.status] || row.status }}
              </el-tag>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { buildingGetMyStats, buildingGetMyTrend } from '../api'
import VChart from 'vue-echarts'
import '../utils/echarts'

const stats = ref(null)
const trendLoading = ref(false)
const trendDays = ref('30')
const trendData = ref([])
const selectedMetric = ref('pv')

const metricConfig = {
  pv: { label: '总浏览量', color: '#e6a23c' },
  uv: { label: '总访客数', color: '#409eff' },
  landlord_view: { label: '房东获取', color: '#f56c6c' },
  phone_rate: { label: '获电率', color: '#13c2c2' },
}

const metricLabel = computed(() => metricConfig[selectedMetric.value]?.label || '')
const metricColor = computed(() => metricConfig[selectedMetric.value]?.color || '#409eff')

function formatNum(n) {
  if (n == null) return '-'
  if (n >= 10000) return (n / 10000).toFixed(1) + '万'
  return n.toLocaleString()
}

function formatDate(dateStr) {
  const d = new Date(dateStr)
  return (d.getMonth() + 1) + '.' + d.getDate()
}

function selectMetric(metric) {
  selectedMetric.value = metric
}

const trendOption = computed(() => ({
  tooltip: { trigger: 'axis', valueFormatter: v => metricLabel.value === '获电率' ? v.toFixed(1) + '%' : v },
  grid: { left: 50, right: 20, bottom: 30, top: 40 },
  xAxis: { type: 'category', data: trendData.value.map(d => formatDate(d.date)), axisLabel: { fontSize: 11 } },
  yAxis: { type: 'value', minInterval: metricLabel.value === '获电率' ? undefined : 1, axisLabel: { formatter: metricLabel.value === '获电率' ? '{value}%' : '{value}' } },
  series: [{
    name: metricLabel.value,
    type: 'line',
    data: trendData.value.map(d => selectedMetric.value === 'phone_rate' ? (d.landlord_view && d.pv ? (d.landlord_view / d.pv * 100) : 0) : d[selectedMetric.value] || 0),
    itemStyle: { color: metricColor.value },
    smooth: true,
    areaStyle: { color: metricColor.value + '1a' },
  }],
}))

async function fetchStats() {
  try {
    const res = await buildingGetMyStats()
    stats.value = res.data.stats
  } catch { ElMessage.error('获取统计数据失败') }
}

async function fetchTrend() {
  trendLoading.value = true
  try {
    const res = await buildingGetMyTrend(Number(trendDays.value))
    trendData.value = res.data.trend || []
  } catch { ElMessage.error('获取趋势失败') }
  finally { trendLoading.value = false }
}

onMounted(() => {
  fetchStats()
  fetchTrend()
})
</script>

<style scoped>
.stats-page { max-width: 1000px; margin: 0 auto; }
.page-title { font-size: 20px; margin-bottom: 20px; color: #1a1a2e; }
.stats-cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(155px, 1fr)); gap: 14px; margin-bottom: 20px; }
.stat-card { text-align: center; cursor: default; }
.stat-value { font-size: 26px; font-weight: 700; line-height: 1.2; }
.stat-label { font-size: 13px; color: #999; margin-top: 4px; }
.stat-gold .stat-value { color: #e6a23c; }
.stat-blue .stat-value { color: #409eff; }
.stat-green .stat-value { color: #67c23a; }
.stat-purple .stat-value { color: #722ed1; }
.stat-cyan .stat-value { color: #13c2c2; }
.stat-red .stat-value { color: #f56c6c; }
.chart-section { margin-bottom: 20px; }
.card-header-row { display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; }
.card-header-row h4 { margin: 0; font-size: 15px; }
</style>
