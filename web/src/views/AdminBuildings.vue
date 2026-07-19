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
      <div style="margin-top: 12px">
        <el-radio-group v-model="timeMode" size="small">
          <el-radio-button value="offset">偏移模式</el-radio-button>
          <el-radio-button value="absolute">指定时间模式</el-radio-button>
        </el-radio-group>
      </div>
      <div v-if="timeMode === 'offset'" style="display: flex; gap: 12px; flex-wrap: wrap; align-items: center; margin-top: 12px">
        <el-input-number v-model="offsetYears" :min="-100" :max="100" size="small" style="width: 100px" controls-position="right" />
        <span>年</span>
        <el-input-number v-model="offsetMonths" :min="-1200" :max="1200" size="small" style="width: 100px" controls-position="right" />
        <span>月</span>
        <el-input-number v-model="offsetDays" :min="-36500" :max="36500" size="small" style="width: 100px" controls-position="right" />
        <span>日</span>
        <el-input-number v-model="offsetHours" :min="-876000" :max="876000" size="small" style="width: 100px" controls-position="right" />
        <span>时</span>
        <el-input-number v-model="offsetMinutes" :min="-52560000" :max="52560000" size="small" style="width: 100px" controls-position="right" />
        <span>分</span>
        <el-button type="primary" size="small" :loading="timeLoading" @click="handleSetOffset">应用偏移</el-button>
      </div>
      <div v-else style="display: flex; gap: 12px; flex-wrap: wrap; align-items: center; margin-top: 12px">
        <span>目标时间：</span>
        <el-date-picker v-model="targetDate" type="datetime" placeholder="选择日期和时间"
          format="YYYY-MM-DD HH:mm:ss" value-format="YYYY-MM-DD HH:mm:ss"
          :disabled-date="() => false" size="small" style="width: 220px" />
        <el-button type="primary" size="small" :loading="timeLoading" @click="handleSetAbsolute">指定时间</el-button>
      </div>
      <div style="display: flex; gap: 12px; flex-wrap: wrap; align-items: center; margin-top: 8px">
        <el-button size="small" @click="handleResetTime">重置（恢复当前真实时间）</el-button>
        <el-button type="warning" size="small" :loading="runTasksLoading" @click="handleRunTasks">手动执行全部定时任务</el-button>
      </div>
    </el-card>

    <AdminBuildingDialogs ref="dialogsRef" @save-success="fetchBuildings" />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { adminGetBuildings, adminDeleteBuilding, adminGetSystemTime, adminSetSystemTime, adminRunTasks } from '../api'
import AdminBuildingList from '../components/admin/AdminBuildingList.vue'
import AdminBuildingDialogs from '../components/admin/AdminBuildingDialogs.vue'

const buildings = ref([])
const loading = ref(true)
const listRef = ref(null)
const dialogsRef = ref(null)

const simulatedTime = ref('')
const timeLoading = ref(false)
const runTasksLoading = ref(false)
const timeMode = ref('offset')
const offsetYears = ref(0)
const offsetMonths = ref(0)
const offsetDays = ref(0)
const offsetHours = ref(0)
const offsetMinutes = ref(0)
const targetDate = ref('')

const TOTAL_SECONDS = {
  year: 365 * 86400,
  month: 30 * 86400,
  day: 86400,
  hour: 3600,
  minute: 60,
}

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
  const url = `${window.location.origin}/login`
  navigator.clipboard.writeText(url).then(() => {
    ElMessage.success('已复制管理员登录页面链接')
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

async function handleSetOffset() {
  timeLoading.value = true
  try {
    const totalSeconds =
      offsetYears.value * TOTAL_SECONDS.year +
      offsetMonths.value * TOTAL_SECONDS.month +
      offsetDays.value * TOTAL_SECONDS.day +
      offsetHours.value * TOTAL_SECONDS.hour +
      offsetMinutes.value * TOTAL_SECONDS.minute
    await adminSetSystemTime({ offset_seconds: totalSeconds })
    ElMessage.success('时间偏移已设置')
    await refreshTime()
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '设置失败')
  } finally {
    timeLoading.value = false
  }
}

async function handleSetAbsolute() {
  if (!targetDate.value) {
    ElMessage.warning('请先选择目标时间')
    return
  }
  timeLoading.value = true
  try {
    await adminSetSystemTime({ target_time: targetDate.value })
    ElMessage.success('时间已指定')
    await refreshTime()
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '设置失败')
  } finally {
    timeLoading.value = false
  }
}

async function handleResetTime() {
  offsetYears.value = 0
  offsetMonths.value = 0
  offsetDays.value = 0
  offsetHours.value = 0
  offsetMinutes.value = 0
  targetDate.value = ''
  timeLoading.value = true
  try {
    await adminSetSystemTime({ offset_seconds: 0 })
    ElMessage.success('已重置时间')
    await refreshTime()
  } finally {
    timeLoading.value = false
  }
}

async function handleRunTasks() {
  runTasksLoading.value = true
  try {
    await adminRunTasks()
    ElMessage.success('已手动执行全部定时任务')
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '执行失败')
  } finally {
    runTasksLoading.value = false
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
