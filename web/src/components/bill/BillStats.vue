<template>
  <div>
    <div style="margin-bottom: 16px">
      <el-date-picker v-model="selectedMonth" :type="mode === 'monthly' ? 'month' : 'year'"
        :format="mode === 'monthly' ? 'YYYY-MM' : 'YYYY'" :value-format="mode === 'monthly' ? 'YYYY-MM' : 'YYYY'"
        @change="fetchData" />
    </div>
    <div v-if="stats" class="summary-cards">
      <el-card>
        <div class="sum-label income-label">{{ mode === 'monthly' ? '总' : '年度总' }}收入</div>
        <div class="sum-value income-value">{{ fmtMoney(stats.total_income) }}</div>
      </el-card>
      <el-card>
        <div class="sum-label expense-label">{{ mode === 'monthly' ? '总' : '年度总' }}支出</div>
        <div class="sum-value expense-value">{{ fmtMoney(stats.total_expense) }}</div>
      </el-card>
      <el-card>
        <div class="sum-label profit-label">{{ mode === 'monthly' ? '净' : '年度净' }}利润</div>
        <div class="sum-value" :class="(stats.net_profit || 0) >= 0 ? 'profit-value' : 'expense-value'">{{ fmtMoney(stats.net_profit) }}</div>
      </el-card>
      <el-card>
        <div class="sum-label count-label">利润率</div>
        <div class="sum-value count-value">{{ profitRate(stats) }}</div>
      </el-card>
      <el-card>
        <div class="sum-label count-label">账单笔数</div>
        <div class="sum-value count-value">{{ stats.bill_count ?? 0 }}</div>
      </el-card>
    </div>
    <el-row :gutter="20">
      <el-col :span="8">
        <el-card>
          <h4>收入明细</h4>
          <v-chart v-if="stats?.income_detail?.length" :option="incomePieOption(stats.income_detail)" style="height:240px" autoresize />
          <div v-else style="color: #999; padding: 10px; text-align:center">暂无数据</div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card>
          <h4>支出明细</h4>
          <v-chart v-if="stats?.expense_detail?.length" :option="expensePieOption(stats.expense_detail)" style="height:240px" autoresize />
          <div v-else style="color: #999; padding: 10px; text-align:center">暂无数据</div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card>
          <h4>收支对比</h4>
          <v-chart v-if="stats" :option="compareOption(stats)" style="height:240px" autoresize />
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import dayjs from 'dayjs'
import { ElMessage } from 'element-plus'
import { buildingGetBillStats } from '../../api'
import VChart from 'vue-echarts'
import '../../utils/echarts'

const props = defineProps({
  mode: { type: String, default: 'monthly' },
})

const now = dayjs()
const selectedMonth = ref(props.mode === 'monthly'
  ? now.format('YYYY-MM')
  : now.format('YYYY'))
const stats = ref(null)

function fmtMoney(n) {
  if (n == null) return '-'
  return '¥' + Number(n).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

function profitRate(stats) {
  const income = stats?.total_income || 0
  const profit = stats?.net_profit || 0
  if (income <= 0) return '-'
  return (profit / income * 100).toFixed(1) + '%'
}

function pieOption(data, color) {
  return {
    tooltip: { trigger: 'item', formatter: '{b}: {c}元 ({d}%)' },
    series: [{
      type: 'pie', radius: ['30%', '70%'], center: ['50%', '55%'],
      data: (data || []).map(d => ({ name: d.subtype, value: d.total })),
      itemStyle: { borderRadius: 4, borderColor: '#fff', borderWidth: 2 },
      label: { show: true, formatter: '{b}\n{d}%', fontSize: 11 },
    }],
    color: color || ['#67c23a', '#409eff', '#e6a23c', '#f56c6c', '#909399'],
  }
}

function incomePieOption(data) { return pieOption(data, ['#67c23a', '#67CC6A', '#67D67A', '#67E08A', '#67EA9A']) }
function expensePieOption(data) { return pieOption(data, ['#f56c6c', '#f08080', '#e9967a', '#eea2ad', '#f4b4c2']) }
// 收入/支出/净利润三根柱对比，比饼图多呈现净利润绝对值
function compareOption(stats) {
  if (!stats) return {}
  return {
    tooltip: { trigger: 'axis', valueFormatter: v => fmtMoney(v) },
    grid: { left: 8, right: 8, top: 30, bottom: 0, containLabel: true },
    xAxis: { type: 'category', data: ['收入', '支出', '净利润'], axisLabel: { fontSize: 12 } },
    yAxis: { type: 'value' },
    series: [{
      type: 'bar',
      barWidth: '40%',
      data: [
        { value: stats.total_income || 0, itemStyle: { color: '#67c23a' } },
        { value: stats.total_expense || 0, itemStyle: { color: '#f56c6c' } },
        { value: stats.net_profit || 0, itemStyle: { color: '#409eff' } },
      ],
      label: { show: true, position: 'top', formatter: p => fmtMoney(p.value), fontSize: 10 },
    }],
  }
}

async function fetchData() {
  if (!selectedMonth.value) return
  try {
    const res = props.mode === 'monthly'
      ? await buildingGetBillStats(selectedMonth.value)
      : await buildingGetBillStats(null, selectedMonth.value)
    stats.value = res.data
  } catch {
    ElMessage.error('获取统计数据失败')
  }
}

onMounted(fetchData)
</script>

<style scoped>
.summary-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
  gap: 14px;
  margin-bottom: 20px;
}
.sum-label { font-size: 14px; }
.sum-value { font-size: 24px; font-weight: bold; margin-top: 6px; }
.income-label { color: #67c23a; }
.income-value { color: #67c23a; }
.expense-label { color: #f56c6c; }
.expense-value { color: #f56c6c; }
.profit-label { color: #409eff; }
.profit-value { color: #409eff; }
.count-label { color: #909399; }
.count-value { color: #606266; }
</style>
