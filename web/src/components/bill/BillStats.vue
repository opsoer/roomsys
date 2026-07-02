<template>
  <div>
    <div style="margin-bottom: 16px">
      <el-date-picker v-model="selectedMonth" :type="mode === 'monthly' ? 'month' : 'year'"
        :format="mode === 'monthly' ? 'YYYY-MM' : 'YYYY'" :value-format="mode === 'monthly' ? 'YYYY-MM' : 'YYYY'"
        @change="fetchData" />
    </div>
    <div v-if="stats" style="display: flex; gap: 20px; margin-bottom: 20px">
      <el-card style="flex: 1">
        <div style="color: #67c23a; font-size: 14px">{{ mode === 'monthly' ? '总' : '年度总' }}收入</div>
        <div style="font-size: 28px; font-weight: bold; color: #67c23a">{{ stats.total_income.toFixed(2) }}</div>
      </el-card>
      <el-card style="flex: 1">
        <div style="color: #f56c6c; font-size: 14px">{{ mode === 'monthly' ? '总' : '年度总' }}支出</div>
        <div style="font-size: 28px; font-weight: bold; color: #f56c6c">{{ stats.total_expense.toFixed(2) }}</div>
      </el-card>
      <el-card style="flex: 1">
        <div style="color: #409eff; font-size: 14px">{{ mode === 'monthly' ? '净' : '年度净' }}利润</div>
        <div style="font-size: 28px; font-weight: bold; color: #409eff">{{ stats.net_profit.toFixed(2) }}</div>
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
          <v-chart :option="compareOption(stats)" style="height:240px" autoresize />
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
function compareOption(stats) {
  if (!stats) return {}
  return {
    tooltip: { trigger: 'item', formatter: '{b}: {c}元 ({d}%)' },
    series: [{
      type: 'pie', radius: ['30%', '70%'],
      data: [
        { name: '收入', value: stats.total_income || 0, itemStyle: { color: '#67c23a' } },
        { name: '支出', value: stats.total_expense || 0, itemStyle: { color: '#f56c6c' } },
      ],
      label: { show: true, formatter: '{b}\n{d}%', fontSize: 13 },
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
