<template>
  <view class="page-tasks">
    <view class="filter-tabs">
      <text :class="['filter-tab', filterStatus === 'pending' ? 'active' : '']" @click="filterStatus='pending'; currentPage=1; fetchTasks()">待处理</text>
      <text :class="['filter-tab', filterStatus === '' ? 'active' : '']" @click="filterStatus=''; currentPage=1; fetchTasks()">全部</text>
      <text :class="['filter-tab', filterStatus === 'completed' ? 'active' : '']" @click="filterStatus='completed'; currentPage=1; fetchTasks()">已完成</text>
    </view>

    <view v-if="loading" class="loading-wrap"><text>加载中...</text></view>
    <view v-else-if="tasks.length === 0" class="empty-wrap"><text>暂无代办事项</text></view>

    <view v-else class="task-list">
      <view v-for="task in tasks" :key="task.id" :class="['task-card', task.status]">
        <view class="task-left">
          <text class="task-status">{{ task.status === 'pending' ? '待处理' : '已完成' }}</text>
        </view>
        <view class="task-body">
          <text class="task-title">{{ task.title }}</text>
          <text class="task-desc">{{ task.description }}<text v-if="task.room">（房间 {{ task.room?.room_number }}）</text></text>
          <text class="task-time">创建于 {{ task.created_at }}</text>
        </view>
        <button v-if="task.status === 'pending' && task.type === 'expired_room'" class="task-btn" @click="openProcessDialog(task)">处理退租</button>
      </view>
    </view>

    <!-- Process Dialog -->
    <view v-if="showProcessDialog" class="overlay" @click="showProcessDialog = false">
      <view class="small-dialog" @click.stop>
        <text class="dialog-title">处理退租</text>
        <view v-if="processingTask" class="process-info">
          <view class="pi-row"><text>房间</text><text class="pi-val">{{ processingTask.room?.room_number || '-' }}</text></view>
          <view class="pi-row"><text>原押金</text><text class="pi-val gold">¥{{ originalDeposit.toFixed(2) }}</text></view>
        </view>
        <view class="form-group"><text class="form-label">退还押金</text><input class="form-input" v-model="processForm.refunded_deposit" type="digit" /></view>
        <text v-if="processForm.refunded_deposit > 0" class="process-note">将自动创建 {{ Number(processForm.refunded_deposit).toFixed(2) }} 元的押金支出账单</text>
        <view class="dialog-actions">
          <button class="dialog-btn cancel" @click="showProcessDialog = false">取消</button>
          <button class="dialog-btn confirm" :disabled="processSubmitting" @click="handleProcessSubmit">{{ processSubmitting ? '提交中...' : '确认退租' }}</button>
        </view>
      </view>
    </view>
  </view>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { buildingGetTasks, buildingProcessTask } from '../../api'

const tasks = ref([])
const loading = ref(false)
const filterStatus = ref('pending')
const currentPage = ref(1)
const total = ref(0)
const pageSize = 20

const showProcessDialog = ref(false)
const processingTask = ref(null)
const processSubmitting = ref(false)
const processForm = ref({ refunded_deposit: 0 })
const originalDeposit = ref(0)

async function fetchTasks() {
  loading.value = true
  try {
    const status = filterStatus.value || undefined
    const res = await buildingGetTasks(status, currentPage.value, pageSize)
    tasks.value = res.data.tasks || []
    total.value = res.data.total || 0
  } catch { uni.showToast({ title: '获取任务失败', icon: 'none' }) }
  finally { loading.value = false }
}

function openProcessDialog(task) {
  processingTask.value = task
  originalDeposit.value = task.deposit || 0
  processForm.value = { refunded_deposit: originalDeposit.value }
  showProcessDialog.value = true
}

async function handleProcessSubmit() {
  processSubmitting.value = true
  try {
    await buildingProcessTask(processingTask.value.id, { refunded_deposit: Number(processForm.value.refunded_deposit) })
    uni.showToast({ title: '退租处理完成', icon: 'success' })
    showProcessDialog.value = false
    await fetchTasks()
  } catch (e) {
    uni.showToast({ title: '处理失败', icon: 'none' })
  }
  finally { processSubmitting.value = false }
}

onMounted(fetchTasks)
</script>

<style scoped>
.page-tasks { padding: 12px; min-height: 100vh; }
.filter-tabs { display: flex; gap: 8px; margin-bottom: 12px; }
.filter-tab { padding: 6px 16px; border-radius: 20px; font-size: 13px; color: #666; background: #f0f0f0; }
.filter-tab.active { background: #e6a23c; color: #fff; font-weight: 600; }
.loading-wrap, .empty-wrap { text-align: center; padding: 60px 0; color: #999; }
.task-card { display: flex; align-items: flex-start; gap: 10px; background: #fff; border-radius: 10px; padding: 12px 14px; margin-bottom: 10px; box-shadow: 0 1px 4px rgba(0,0,0,0.04); }
.task-card.completed { opacity: 0.6; }
.task-left { flex-shrink: 0; }
.task-status { font-size: 11px; padding: 2px 10px; border-radius: 10px; color: #fff; background: #e6a23c; }
.task-card.completed .task-status { background: #67c23a; }
.task-body { flex: 1; }
.task-title { font-size: 14px; font-weight: 600; color: #333; display: block; }
.task-desc { font-size: 13px; color: #999; display: block; margin-top: 4px; }
.task-time { font-size: 12px; color: #bbb; display: block; margin-top: 4px; }
.task-btn { flex-shrink: 0; font-size: 12px; color: #fff; background: #1989fa; border: none; border-radius: 20px; padding: 4px 12px; }
.overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); z-index: 1000; display: flex; align-items: flex-end; }
.small-dialog { width: 100%; background: #fff; border-radius: 16px 16px 0 0; padding: 20px; }
.dialog-title { font-size: 18px; font-weight: 700; display: block; margin-bottom: 16px; }
.process-info { background: #f5f7fa; padding: 10px; border-radius: 8px; margin-bottom: 16px; }
.pi-row { display: flex; justify-content: space-between; margin-bottom: 4px; font-size: 14px; color: #666; }
.pi-val { font-weight: 600; color: #333; }
.pi-val.gold { color: #e6a23c; }
.form-group { margin-bottom: 12px; }
.form-label { font-size: 14px; color: #333; display: block; margin-bottom: 4px; }
.form-input { width: 100%; height: 40px; border: 1px solid #dcdfe6; border-radius: 8px; padding: 0 12px; font-size: 14px; background: #fff; }
.process-note { display: block; background: #fef0f0; padding: 10px; border-radius: 6px; font-size: 13px; color: #f56c6c; }
.dialog-actions { display: flex; gap: 12px; margin-top: 20px; }
.dialog-btn { flex: 1; height: 44px; border-radius: 22px; display: flex; align-items: center; justify-content: center; font-size: 15px; }
.dialog-btn.cancel { background: #f5f5f5; color: #666; border: none; }
.dialog-btn.confirm { background: #1989fa; color: #fff; border: none; }
.dialog-btn.confirm[disabled] { opacity: 0.6; }
</style>
