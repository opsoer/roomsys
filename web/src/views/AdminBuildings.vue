<template>
  <div style="max-width: 1200px; margin: 0 auto;">
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; flex-wrap: wrap; gap: 12px;">
      <h2 style="font-size: 22px; font-weight: 700;">公寓管理</h2>
      <el-button type="primary" @click="openCreate">
        <el-icon><Plus /></el-icon> 创建公寓
      </el-button>
    </div>

    <AdminBuildingList ref="listRef" :buildings="buildings" :loading="loading"
      @search="fetchBuildings" @edit="handleEdit" @upgrade="handleUpgrade"
      @copy-link="copyLoginLink" @create-admin="handleCreateAdmin" @delete="handleDelete" />

    <el-card shadow="never" style="margin-top: 20px">
      <h4>测试：系统时间模拟</h4>
      <div style="display: flex; gap: 12px; flex-wrap: wrap; align-items: center; margin-top: 12px">
        <span>当前模拟时间：<strong>{{ simulatedTime }}</strong></span>
        <el-button size="small" @click="refreshTime">刷新时间</el-button>
      </div>
      <div style="display: flex; gap: 12px; flex-wrap: wrap; align-items: center; margin-top: 12px">
        <span>偏移量：</span>
        <el-input-number v-model="offsetDays" :min="-365" :max="365" size="small" style="width: 100px" controls-position="right" />
        <span>天</span>
        <el-input-number v-model="offsetHours" :min="-23" :max="23" size="small" style="width: 80px" controls-position="right" />
        <span>小时</span>
        <el-button type="primary" size="small" :loading="timeLoading" @click="handleSetTime">应用偏移</el-button>
        <el-button size="small" @click="handleResetTime">重置</el-button>
      </div>
    </el-card>

    <AdminBuildingDialogs ref="dialogsRef" @save-success="fetchBuildings" />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { Plus } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { adminGetBuildings, adminDeleteBuilding, adminGetSystemTime, adminSetSystemTime } from '../api'
import AdminBuildingList from '../components/admin/AdminBuildingList.vue'
import AdminBuildingDialogs from '../components/admin/AdminBuildingDialogs.vue'

const buildings = ref([])
const loading = ref(true)
const listRef = ref(null)
const dialogsRef = ref(null)

const simulatedTime = ref('')
const timeLoading = ref(false)
const offsetDays = ref(0)
const offsetHours = ref(0)

async function fetchBuildings() {
  loading.value = true
  try {
    const filter = listRef.value?.getFilter() || {}
    const params = {}
    if (filter.status) params.status = filter.status
    if (filter.keyword) params.keyword = filter.keyword
    const res = await adminGetBuildings(params)
    buildings.value = res.data.buildings || []
  } finally {
    loading.value = false
  }
}

function openCreate() {
  dialogsRef.value?.openCreate()
}

function handleEdit(row) {
  dialogsRef.value?.openEdit(row)
}

function handleUpgrade(row) {
  dialogsRef.value?.openUpgrade(row)
}

function handleCreateAdmin(row) {
  dialogsRef.value?.openCreateAdmin(row)
}

async function handleDelete(id) {
  try {
    await adminDeleteBuilding(id)
    ElMessage.success('已删除')
    await fetchBuildings()
  } catch {
    ElMessage.error('删除失败')
  }
}

function copyLoginLink(row) {
  const url = `${window.location.origin}/landlord/login/${row.id}`
  navigator.clipboard.writeText(url).then(() => {
    ElMessage.success('已复制管理员登录链接')
  }, () => {
    ElMessage.error('复制失败，请手动复制')
  })
}

async function refreshTime() {
  try {
    const res = await adminGetSystemTime()
    simulatedTime.value = res.data.simulated_time
  } catch {}
}

async function handleSetTime() {
  timeLoading.value = true
  try {
    const totalSeconds = offsetDays.value * 86400 + offsetHours.value * 3600
    await adminSetSystemTime(totalSeconds)
    ElMessage.success('时间偏移已设置')
    await refreshTime()
  } catch {
    ElMessage.error('设置失败')
  } finally {
    timeLoading.value = false
  }
}

async function handleResetTime() {
  offsetDays.value = 0
  offsetHours.value = 0
  timeLoading.value = true
  try {
    await adminSetSystemTime(0)
    ElMessage.success('已重置时间')
    await refreshTime()
  } finally {
    timeLoading.value = false
  }
}

onMounted(() => {
  fetchBuildings()
  refreshTime()
})
</script>

<style scoped>
@media (max-width: 768px) {
  .el-card { padding: 12px; }
}
</style>
