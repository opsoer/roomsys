<template>
  <div>
    <h3 style="margin-bottom: 20px">代办事项</h3>
    <el-card>
      <div style="display: flex; gap: 12px; margin-bottom: 16px; align-items: center">
        <el-radio-group v-model="filterStatus" @change="fetchTasks">
          <el-radio-button value="">全部</el-radio-button>
          <el-radio-button value="pending">待处理</el-radio-button>
          <el-radio-button value="completed">已完成</el-radio-button>
        </el-radio-group>
        <span style="color: #999; font-size: 13px">共 {{ tasks.length }} 条</span>
      </div>
      <div v-if="loading" v-loading="loading" style="height: 120px" />
      <div v-else-if="!tasks.length" style="text-align: center; padding: 40px; color: #999">
        <el-icon :size="48" style="color: #ddd"><List /></el-icon>
        <p style="margin-top: 12px">暂无代办事项</p>
      </div>
      <div v-else style="display: flex; flex-direction: column; gap: 12px">
        <div v-for="task in tasks" :key="task.id" class="task-card" :class="{ completed: task.status === 'completed' }">
          <div style="display: flex; align-items: flex-start; gap: 12px; flex: 1">
            <el-tag :type="task.status === 'pending' ? 'warning' : 'success'" size="small" style="margin-top: 2px">
              {{ task.status === 'pending' ? '待处理' : '已完成' }}
            </el-tag>
            <div style="flex: 1">
              <div style="font-weight: 500; margin-bottom: 4px">{{ task.title }}</div>
              <div style="font-size: 13px; color: #999">
                {{ task.description }}
                <span v-if="task.room" style="margin-left: 4px">（房间 {{ task.room.room_number }}）</span>
              </div>
              <div style="font-size: 12px; color: #bbb; margin-top: 4px">创建于 {{ task.created_at }}</div>
            </div>
            <div style="display: flex; gap: 8px; flex-shrink: 0">
              <el-button v-if="task.status === 'pending' && task.type === 'expired_room'" type="primary" size="small" @click="openProcessDialog(task)">
                处理退租
              </el-button>
            </div>
          </div>
        </div>
      </div>
    </el-card>

    <el-dialog v-model="showProcessDialog" title="处理退租" width="420px">
      <div v-if="processingTask" style="margin-bottom: 16px">
        <div style="background: #f5f7fa; padding: 12px; border-radius: 8px; margin-bottom: 16px">
          <div style="display: flex; justify-content: space-between; margin-bottom: 8px">
            <span style="color: #999">房间</span>
            <span style="font-weight: 600">{{ processingTask.room?.room_number || '-' }}</span>
          </div>
          <div style="display: flex; justify-content: space-between">
            <span style="color: #999">原押金</span>
            <span style="font-weight: 600; color: #e6a23c">{{ originalDeposit.toFixed(2) }} 元</span>
          </div>
        </div>
        <el-form ref="processFormRef" :model="processForm" label-width="100px">
          <el-form-item label="退还押金" prop="refunded_deposit"
            :rules="[{ required: true, message: '请填写退还押金金额' }]">
            <el-input-number v-model="processForm.refunded_deposit" :min="0" :max="originalDeposit" :precision="2" style="width: 100%" />
          </el-form-item>
        </el-form>
        <div v-if="deduction > 0" style="background: #fef0f0; padding: 10px 12px; border-radius: 6px; font-size: 13px; color: #f56c6c">
          将自动创建一笔 <strong>{{ deduction.toFixed(2) }}</strong> 元的押金收入账单
        </div>
        <div style="font-size: 12px; color: #999; margin-top: 8px">
          退还押金后将自动创建押金退还支出账单和押金收入账单，房间状态改为未出租
        </div>
      </div>
      <template #footer>
        <el-button @click="showProcessDialog = false">取消</el-button>
        <el-button type="primary" :loading="processSubmitting" @click="handleProcessSubmit">确认退租</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { buildingGetTasks, buildingProcessTask } from '../api'
import { ElMessage } from 'element-plus'
import { List } from '@element-plus/icons-vue'

const tasks = ref([])
const loading = ref(false)
const filterStatus = ref('pending')

const showProcessDialog = ref(false)
const processingTask = ref(null)
const processSubmitting = ref(false)
const processFormRef = ref(null)
const processForm = ref({ refunded_deposit: 0 })
const originalDeposit = ref(0)

const deduction = computed(() => {
  const d = originalDeposit.value - (processForm.value.refunded_deposit || 0)
  return d > 0 ? d : 0
})

async function fetchTasks() {
  loading.value = true
  try {
    const status = filterStatus.value || undefined
    const res = await buildingGetTasks(status)
    tasks.value = res.data.tasks
  } finally {
    loading.value = false
  }
}

function openProcessDialog(task) {
  processingTask.value = task
  originalDeposit.value = task.deposit || 0
  processForm.value = { refunded_deposit: originalDeposit.value }
  showProcessDialog.value = true
}

async function handleProcessSubmit() {
  const valid = await processFormRef.value.validate().catch(() => false)
  if (!valid) return
  processSubmitting.value = true
  try {
    await buildingProcessTask(processingTask.value.id, { refunded_deposit: processForm.value.refunded_deposit })
    ElMessage.success('退租处理完成')
    showProcessDialog.value = false
    await fetchTasks()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '处理失败')
  } finally {
    processSubmitting.value = false
  }
}

onMounted(fetchTasks)
</script>

<style scoped>
.task-card {
  background: #fafafa;
  border: 1px solid #eee;
  border-radius: 8px;
  padding: 14px 16px;
  transition: all 0.2s;
}
.task-card:hover {
  border-color: #409eff;
  background: #f0f7ff;
}
.task-card.completed {
  opacity: 0.6;
  background: #f5f5f5;
}
.task-card.completed:hover {
  border-color: #ddd;
  background: #f5f5f5;
}
</style>
