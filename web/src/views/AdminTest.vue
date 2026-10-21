<template>
  <div style="max-width: 900px; margin: 0 auto;">
    <h2 style="font-size: 22px; font-weight: 700; margin-bottom: 20px;">测试工具</h2>

    <el-card shadow="never" style="margin-bottom: 20px">
      <h4>系统时间模拟</h4>
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

    <el-card shadow="never" style="margin-top: 20px">
      <div style="display: flex; align-items: center; gap: 12px">
        <h4 style="margin: 0">定时任务</h4>
        <el-button size="small" :loading="jobsLoading" @click="loadJobs">刷新</el-button>
      </div>
      <div v-if="jobs.length === 0" style="margin-top: 12px; font-size: 13px; color: #999">
        暂无任务数据，请点击刷新。
      </div>
      <div v-for="job in jobs" :key="job.name"
        style="display: flex; gap: 12px; flex-wrap: wrap; align-items: center; padding: 10px 0; border-bottom: 1px solid var(--el-border-color-lighter)">
        <div style="width: 150px; font-weight: 600">{{ job.title }}</div>
        <el-tag size="small" :type="job.enabled ? 'success' : 'info'" disable-transitions>
          {{ job.enabled ? '已启用' : '已停用' }}
        </el-tag>
        <el-tag size="small" type="warning" disable-transitions>{{ job.schedule }}</el-tag>
        <div style="flex: 1; min-width: 240px; font-size: 12px; color: #666">
          <template v-if="job.last_run_at">
            上次：{{ job.last_run_at }}（{{ job.last_ms }}ms，{{ triggerLabel(job.trigger) }}）<br>
            <span :style="{ color: isJobFailure(job.last_result) ? '#F56C6C' : '#666' }">{{ job.last_result }}</span>
          </template>
          <span v-else>尚未执行过</span>
        </div>
        <el-button size="small" type="primary" :loading="!!jobRunning[job.name]"
          @click="handleRunJob(job)">执行</el-button>
      </div>
      <div style="margin-top: 10px; font-size: 12px; color: #999">
        调度周期来自服务端 config.yaml 的 scheduler 段，修改后需重启生效；此处手动执行不受「已停用」限制。
      </div>
    </el-card>

    <el-card shadow="never">
      <h4>更多测试功能（预留）</h4>
      <div style="margin-top: 12px; font-size: 13px; color: #999">
        后续测试功能将统一放在本页面管理。
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { adminGetSystemTime, adminSetSystemTime, adminRunTasks, adminListSystemJobs, adminRunSystemJob } from '../api'

const simulatedTime = ref('')
const timeLoading = ref(false)
const runTasksLoading = ref(false)
const jobs = ref([])
const jobsLoading = ref(false)
const jobRunning = reactive({})
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
    const res = await adminRunTasks()
    const results = res.data.results || []
    ElMessageBox.alert(
      results.map(r => `<div style="padding:4px 0">${r}</div>`).join(''),
      '定时任务执行结果',
      { dangerouslyUseHTMLString: true, confirmButtonText: '确定' }
    )
    await loadJobs()
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '执行失败')
  } finally {
    runTasksLoading.value = false
  }
}

async function loadJobs() {
  jobsLoading.value = true
  try {
    const res = await adminListSystemJobs()
    jobs.value = res.data.jobs || []
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '加载定时任务失败')
  } finally {
    jobsLoading.value = false
  }
}

async function handleRunJob(job) {
  jobRunning[job.name] = true
  try {
    const res = await adminRunSystemJob(job.name)
    await ElMessageBox.alert(res.data.result || '执行完成', `【${job.title}】执行结果`, {
      confirmButtonText: '确定',
    })
    await loadJobs()
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '执行失败')
  } finally {
    jobRunning[job.name] = false
  }
}

function triggerLabel(trigger) {
  if (trigger === 'manual') return '手动'
  if (trigger === 'startup') return '启动'
  return '调度'
}

function isJobFailure(result) {
  if (!result) return false
  return result.includes('失败') || result.includes('panic')
}

onMounted(() => {
  refreshTime()
  loadJobs()
})

</script>