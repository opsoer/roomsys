<template>
  <div>
    <h3 style="margin-bottom: 20px">财务管理</h3>
    <el-tabs v-model="activeTab" @tab-change="handleTabChange">
      <el-tab-pane label="账单列表" name="list">
        <BillList ref="billListRef" :bills="bills" :loading="billLoading"
          :rooms="allRooms"
          @search="fetchBills" @add="openAddDialog" @edit="handleEdit" @export="handleExport" />
      </el-tab-pane>

      <el-tab-pane label="月度统计" name="monthly">
        <BillStats mode="monthly" />
      </el-tab-pane>

      <el-tab-pane label="年度统计" name="yearly">
        <BillStats mode="yearly" />
      </el-tab-pane>

      <el-tab-pane label="收支趋势" name="trend">
        <BillTrend />
      </el-tab-pane>

      <el-tab-pane label="分红预测" name="predict">
        <BillPredict />
      </el-tab-pane>
    </el-tabs>

    <div v-if="billLoadingMore" style="text-align: center; padding: 16px; color: #999">
      <el-icon class="is-loading"><Loading /></el-icon> 加载中...
    </div>
    <div v-else-if="billTotal > 0 && bills.length >= billTotal" style="text-align: center; padding: 16px; color: #999; font-size: 13px">
      已全部加载（共 {{ billTotal }} 条）
    </div>
    <div v-else-if="billTotal > 0" style="text-align: center; padding: 16px; color: #999; font-size: 13px">
      共 {{ billTotal }} 条，已显示 {{ bills.length }} 条
    </div>
    <div ref="sentinel" style="height: 10px"></div>
    <BillDialog ref="billDialogRef" :all-rooms="allRooms" @save-success="handleSaveSuccess" />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { buildingGetBills, buildingGetRooms, buildingExportBills } from '../api'
import { ElMessage } from 'element-plus'
import { useInfiniteList } from '../composables/useInfiniteList'
import BillList from '../components/bill/BillList.vue'
import BillStats from '../components/bill/BillStats.vue'
import BillTrend from '../components/bill/BillTrend.vue'
import BillPredict from '../components/bill/BillPredict.vue'
import BillDialog from '../components/bill/BillDialog.vue'

const activeTab = ref('list')
const allRooms = ref([])
const billListRef = ref(null)
const billDialogRef = ref(null)
const billPageSize = 20

// 无限滚动 + 游标分页（last_id + last_key 账单日期）；原实现 append 不去重，保持行为一致
const { items: bills, total: billTotal, loading: billLoading, loadingMore: billLoadingMore, sentinel, fetch: fetchBills } = useInfiniteList({
  fetcher: async (params) => {
    const res = await buildingGetBills(params)
    return { list: res.data.bills || [], total: res.data.total || 0 }
  },
  pageSize: billPageSize,
  cursorKey: (last) => ({ last_id: last.id, last_key: last.bill_date }),
  getExtraParams: () => billListRef.value?.getFilterParams() || {},
  onError: () => ElMessage.error('获取账单列表失败'),
  dedupe: false,
})

function openAddDialog() {
  billDialogRef.value?.open()
}

// 按当前筛选条件导出账单 CSV
async function handleExport() {
  try {
    const params = billListRef.value?.getFilterParams() || {}
    const res = await buildingExportBills(params)
    const blob = new Blob([res.data], { type: 'text/csv;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `bills-${params.start_date || ''}_${params.end_date || ''}.csv`
    a.click()
    URL.revokeObjectURL(url)
  } catch {
    ElMessage.error('导出失败')
  }
}

function handleEdit(row) {
  billDialogRef.value?.openEdit(row)
}

async function handleSaveSuccess() {
  await fetchBills()
}

function handleTabChange(name) {
  // Stats/Trend/Predict components handle their own data fetching
}

onMounted(async () => {
  await fetchBills()
  try {
    const res = await buildingGetRooms()
    allRooms.value = res.data.rooms
  } catch {
    ElMessage.error('获取房间列表失败')
  }
})
</script>
