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
import { ref, onMounted, onBeforeUnmount, nextTick } from 'vue'
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
const billLoadingMore = ref(false)
const sentinel = ref(null)
let observer = null
const allRooms = ref([])
const billListRef = ref(null)
const billDialogRef = ref(null)
const billTotal = ref(0)
const billPageSize = 20

async function fetchBills(append = false) {
  if (!append) {
    billLoading.value = true
  } else {
    billLoadingMore.value = true
  }
  try {
    const params = billListRef.value?.getFilterParams() || {}
    params.page_size = billPageSize
    if (append && bills.value.length > 0) {
      params.last_id = bills.value[bills.value.length - 1].id
      params.last_key = bills.value[bills.value.length - 1].bill_date
    }
    const res = await buildingGetBills(params)
    const data = res.data.bills || []
    if (!append) billTotal.value = res.data.total || 0
    if (append) {
      bills.value = [...bills.value, ...data]
    } else {
      bills.value = data
    }
  } catch {
    ElMessage.error('获取账单列表失败')
  } finally {
    billLoading.value = false
    billLoadingMore.value = false
    nextTick(setupInfiniteScroll)
  }
}

function loadMore() {
  fetchBills(true)
}

// 触底自动加载：sentinel 进入视口附近时加载下一页
function setupInfiniteScroll() {
  if (observer) observer.disconnect()
  if (!sentinel.value) return
  observer = new IntersectionObserver((entries) => {
    if (
      entries[0].isIntersecting &&
      !billLoading.value &&
      !billLoadingMore.value &&
      bills.value.length < billTotal.value
    ) {
      loadMore()
    }
  }, { rootMargin: '200px 0px' })
  observer.observe(sentinel.value)
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

onBeforeUnmount(() => {
  if (observer) observer.disconnect()
  observer = null
})
</script>
