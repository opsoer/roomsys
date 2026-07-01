<template>
  <div>
    <div style="margin-bottom: 16px">
      <el-select v-model="trendRange" style="width: 140px" @change="fetchTrend">
        <el-option label="近12个月" value="12" />
        <el-option label="近24个月" value="24" />
      </el-select>
    </div>
    <el-card v-loading="trendLoading" style="margin-bottom: 20px">
      <h4 style="margin-bottom: 16px">月度收支趋势</h4>
      <v-chart v-if="trendData.length" :option="trendOption" style="height: 360px" autoresize />
      <div v-else style="text-align:center;padding:40px;color:#999">暂无趋势数据</div>
    </el-card>
    <el-card v-loading="trendLoading">
      <h4 style="margin-bottom: 16px">增长率分析</h4>
      <div class="desktop-table">
        <el-table :data="growthData" border stripe size="small" max-height="300">
          <el-table-column prop="month" label="月份" width="90" />
          <el-table-column label="收入环比" width="110">
            <template #default="{ row }">
              <span :style="{ color: (row.income_mom||0) >= 0 ? '#67c23a' : '#f56c6c' }">
                {{ row.income_mom != null ? (row.income_mom >= 0 ? '+' : '') + row.income_mom.toFixed(1) + '%' : '-' }}
              </span>
            </template>
          </el-table-column>
          <el-table-column label="收入同比" width="110">
            <template #default="{ row }">
              <span :style="{ color: (row.income_yoy||0) >= 0 ? '#67c23a' : '#f56c6c' }">
                {{ row.income_yoy != null ? (row.income_yoy >= 0 ? '+' : '') + row.income_yoy.toFixed(1) + '%' : '-' }}
              </span>
            </template>
          </el-table-column>
          <el-table-column label="支出环比" width="110">
            <template #default="{ row }">
              <span :style="{ color: (row.expense_mom||0) >= 0 ? '#f56c6c' : '#67c23a' }">
                {{ row.expense_mom != null ? (row.expense_mom >= 0 ? '+' : '') + row.expense_mom.toFixed(1) + '%' : '-' }}
              </span>
            </template>
          </el-table-column>
          <el-table-column label="支出同比" width="110">
            <template #default="{ row }">
              <span :style="{ color: (row.expense_yoy||0) >= 0 ? '#f56c6c' : '#67c23a' }">
                {{ row.expense_yoy != null ? (row.expense_yoy >= 0 ? '+' : '') + row.expense_yoy.toFixed(1) + '%' : '-' }}
              </span>
            </template>
          </el-table-column>
        </el-table>
      </div>
      <div class="mobile-cards">
        <div v-for="g in growthData" :key="g.month" class="growth-card">
          <div class="gc-month">{{ g.month }}</div>
          <div class="gc-grid">
            <div class="gc-item">
              <span class="gc-label">收入环比</span>
              <span :style="{ color: (g.income_mom||0) >= 0 ? '#67c23a' : '#f56c6c', fontWeight:600 }">
                {{ g.income_mom != null ? (g.income_mom >= 0 ? '+' : '') + g.income_mom.toFixed(1) + '%' : '-' }}
              </span>
            </div>
            <div class="gc-item">
              <span class="gc-label">收入同比</span>
              <span :style="{ color: (g.income_yoy||0) >= 0 ? '#67c23a' : '#f56c6c', fontWeight:600 }">
                {{ g.income_yoy != null ? (g.income_yoy >= 0 ? '+' : '') + g.income_yoy.toFixed(1) + '%' : '-' }}
              </span>
            </div>
            <div class="gc-item">
              <span class="gc-label">支出环比</span>
              <span :style="{ color: (g.expense_mom||0) >= 0 ? '#f56c6c' : '#67c23a', fontWeight:600 }">
                {{ g.expense_mom != null ? (g.expense_mom >= 0 ? '+' : '') + g.expense_mom.toFixed(1) + '%' : '-' }}
              </span>
            </div>
            <div class="gc-item">
              <span class="gc-label">支出同比</span>
              <span :style="{ color: (g.expense_yoy||0) >= 0 ? '#f56c6c' : '#67c23a', fontWeight:600 }">
                {{ g.expense_yoy != null ? (g.expense_yoy >= 0 ? '+' : '') + g.expense_yoy.toFixed(1) + '%' : '-' }}
              </span>
            </div>
          </div>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { buildingGetBillTrend } from '../../api'
import VChart from 'vue-echarts'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
use([CanvasRenderer, LineChart, GridComponent, TooltipComponent, LegendComponent])

const trendRange = ref('12')
const trendLoading = ref(false)
const trendData = ref([])
const growthData = ref([])

const trendOption = computed(() => ({
  tooltip: { trigger: 'axis' },
  legend: { data: ['收入', '支出', '净利润'] },
  grid: { left: 50, right: 20, bottom: 30, top: 40 },
  xAxis: { type: 'category', data: trendData.value.map(d => d.month), axisLabel: { rotate: 45 } },
  yAxis: { type: 'value' },
  series: [
    { name: '收入', type: 'line', data: trendData.value.map(d => d.income), itemStyle: { color: '#67c23a' }, smooth: true },
    { name: '支出', type: 'line', data: trendData.value.map(d => d.expense), itemStyle: { color: '#f56c6c' }, smooth: true },
    { name: '净利润', type: 'line', data: trendData.value.map(d => d.profit), itemStyle: { color: '#409eff' }, smooth: true, lineStyle: { type: 'dashed' } },
  ],
}))

async function fetchTrend() {
  trendLoading.value = true
  try {
    const res = await buildingGetBillTrend({ years: trendRange.value })
    trendData.value = res.data.months || []
    growthData.value = res.data.growth || []
  } catch {
    ElMessage.error('获取趋势数据失败')
  } finally {
    trendLoading.value = false
  }
}

onMounted(fetchTrend)
</script>

<style scoped>
.desktop-table { display: block; }
.mobile-cards { display: none; }

.growth-card {
  background: #fff;
  border-radius: 10px;
  padding: 12px 14px;
  margin-bottom: 10px;
  border: 1px solid #eee;
  box-shadow: 0 1px 4px rgba(0,0,0,0.04);
}
.gc-month {
  font-size: 14px;
  font-weight: 600;
  color: #333;
  margin-bottom: 8px;
}
.gc-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}
.gc-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.gc-label {
  font-size: 11px;
  color: #999;
}

@media (max-width: 768px) {
  .desktop-table { display: none; }
  .mobile-cards { display: block; }
}
</style>
