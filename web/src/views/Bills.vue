<template>
  <div>
    <h3 style="margin-bottom: 20px">财务管理</h3>
    <el-tabs v-model="activeTab" @tab-change="handleTabChange">
      <el-tab-pane label="账单列表" name="list">
        <BillList ref="billListRef" :bills="bills" :loading="billLoading"
          :rooms="allRooms"
          @search="fetchBills" @add="openAddDialog" @edit="handleEdit" />
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

    <div v-if="billTotal > billPageSize" style="display: flex; justify-content: center; margin-top: 16px">
      <el-pagination background layout="prev, pager, next" :total="billTotal" :page-size="billPageSize" :current-page="billPage" @current-change="onBillPageChange" />
    </div>
    <BillDialog ref="billDialogRef" :all-rooms="allRooms" @save-success="handleSaveSuccess" />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { buildingGetBills, buildingGetRooms } from '../api'
import { ElMessage } from 'element-plus'
import BillList from '../components/bill/BillList.vue'
import BillStats from '../components/bill/BillStats.vue'
import BillTrend from '../components/bill/BillTrend.vue'
import BillPredict from '../components/bill/BillPredict.vue'
import BillDialog from '../components/bill/BillDialog.vue'

const activeTab = ref('list')
const bills = ref([])
const billLoading = ref(false)
const allRooms = ref([])
const billListRef = ref(null)
const billDialogRef = ref(null)
const billPage = ref(1)
const billTotal = ref(0)
const billPageSize = 20

async function fetchBills() {
  billLoading.value = true
  try {
    const params = billListRef.value?.getFilterParams() || {}
    params.page = billPage.value
    params.page_size = billPageSize
    const res = await buildingGetBills(params)
    bills.value = res.data.bills
    billTotal.value = res.data.total || 0
  } catch {
    ElMessage.error('获取账单列表失败')
  } finally {
    billLoading.value = false
  }
}

function onBillPageChange(page) {
  billPage.value = page
  fetchBills()
}

function openAddDialog() {
  billDialogRef.value?.open()
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
