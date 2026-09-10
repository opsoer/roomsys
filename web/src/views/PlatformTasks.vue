<template>
  <div>
    <div class="todo-header">
      <h3>代办事项</h3>
      <el-badge :value="pendingCount" :hidden="!pendingCount" class="recruit-badge-wrap">
        <el-tag :type="pendingCount ? 'danger' : 'success'">
          {{ pendingCount ? pendingCount + ' 条待处理' : '全部已处理' }}
        </el-tag>
      </el-badge>
    </div>

    <div class="filter-bar">
      <el-radio-group v-model="status" size="small" @change="handleFilter">
        <el-radio-button label="">全部</el-radio-button>
        <el-radio-button label="pending">待处理</el-radio-button>
        <el-radio-button label="completed">已处理</el-radio-button>
      </el-radio-group>
    </div>

    <div class="desktop-table">
      <el-table :data="tasks" v-loading="loading" empty-text="暂无待办事项" style="width:100%">
        <el-table-column label="类型" width="130">
          <template #default="{ row }">
            <el-tag :type="typeTag(row.type)" size="small">{{ typeText(row.type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="title" label="内容" min-width="220" />
        <el-table-column label="关联公寓" min-width="200">
          <template #default="{ row }">
            <template v-if="row.building">
              <div style="font-weight: 500">{{ row.building.name }}</div>
              <div v-if="buildingAddress(row.building)" style="font-size: 12px; color: #999">
                📍 {{ buildingAddress(row.building) }}
              </div>
              <div v-if="landlordText(row.building)" style="font-size: 12px; color: #999">
                👤 房东：{{ landlordText(row.building) }}
              </div>
            </template>
            <span v-else>—</span>
          </template>
        </el-table-column>
        <el-table-column prop="due_date" label="到期日" width="110">
          <template #default="{ row }">
            {{ row.due_date || '—' }}
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="170">
          <template #default="{ row }">
            {{ formatTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'pending' ? 'danger' : 'success'" size="small">
              {{ row.status === 'pending' ? '待处理' : '已处理' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150">
          <template #default="{ row }">
            <el-button v-if="row.status === 'pending' && isExpiryTask(row)" size="small" type="warning" @click="openRenew(row)">
              续约
            </el-button>
            <el-button v-else-if="row.status === 'pending'" size="small" type="primary" @click="handleProcess(row.id)">
              处理
            </el-button>
            <span v-else style="color:#999;font-size:13px;">已完成</span>
          </template>
        </el-table-column>
      </el-table>
      <div v-if="total > pageSize" class="pagination-wrap">
        <el-pagination
          background
          layout="prev, pager, next"
          :total="total"
          :page-size="pageSize"
          :current-page="page"
          @current-change="handlePage"
        />
      </div>
    </div>

    <div class="mobile-cards" v-loading="loading">
      <div v-for="item in tasks" :key="item.id" :class="['todo-card', item.status]">
        <div class="card-indicator"></div>
        <div class="card-body">
          <div class="card-head">
            <el-tag :type="typeTag(item.type)" size="small">{{ typeText(item.type) }}</el-tag>
            <el-tag :type="item.status === 'pending' ? 'danger' : 'success'" size="small" effect="dark" round>
              {{ item.status === 'pending' ? '待处理' : '已处理' }}
            </el-tag>
          </div>
          <div class="card-title">{{ item.title }}</div>
          <div v-if="item.building" class="card-field">
            <span class="field-icon">🏢</span>
            <span class="field-value">{{ item.building.name }}</span>
          </div>
          <div v-if="item.building && buildingAddress(item.building)" class="card-field">
            <span class="field-icon">📍</span>
            <span class="field-value">{{ buildingAddress(item.building) }}</span>
          </div>
          <div v-if="item.building && landlordText(item.building)" class="card-field">
            <span class="field-icon">👤</span>
            <span class="field-value">房东：{{ landlordText(item.building) }}</span>
          </div>
          <div v-if="item.due_date" class="card-field">
            <span class="field-icon">📅</span>
            <span class="field-value">{{ item.due_date }}</span>
          </div>
          <div class="card-foot">
            <span class="card-time">🕐 {{ formatTime(item.created_at) }}</span>
            <el-button v-if="item.status === 'pending' && isExpiryTask(item)" size="small" type="warning" round @click="openRenew(item)">
              续约
            </el-button>
            <el-button v-else-if="item.status === 'pending'" size="small" type="primary" round @click="handleProcess(item.id)">
              处理
            </el-button>
            <el-tag v-else type="success" size="small" effect="plain" round>已完成</el-tag>
          </div>
        </div>
      </div>
      <div v-if="!loading && tasks.length === 0" class="empty-text">暂无待办事项</div>
    </div>

    <el-dialog v-model="renewVisible" title="公寓续约" width="420px" align-center>
      <div v-if="renewTask" style="margin-bottom: 16px">
        <div style="background: #f5f7fa; padding: 12px; border-radius: 8px; margin-bottom: 16px">
          <div style="display: flex; justify-content: space-between; margin-bottom: 8px">
            <span style="color: #999">公寓</span>
            <span style="font-weight: 600">{{ renewTask.building?.name || '-' }}</span>
          </div>
          <div style="display: flex; justify-content: space-between">
            <span style="color: #999">当前到期日</span>
            <span style="font-weight: 600; color: #e6a23c">{{ renewTask.due_date || '-' }}</span>
          </div>
        </div>
        <el-form ref="renewFormRef" :model="renewForm" label-width="100px">
          <el-form-item label="新到期日" prop="expired_at"
            :rules="[{ required: true, message: '请选择新的到期日期' }]">
            <el-date-picker v-model="renewForm.expired_at" type="date" value-format="YYYY-MM-DD"
              :disabled-date="disablePastDate" placeholder="选择新的到期日期" style="width: 100%" />
          </el-form-item>
        </el-form>
        <div style="font-size: 12px; color: #999">
          续约后公寓立即恢复展示；新到期日超过30天，相关待办将自动变为已完成
        </div>
      </div>
      <template #footer>
        <el-button @click="handleDismissRenew">仅标记已处理</el-button>
        <el-button type="primary" :loading="renewSubmitting" @click="handleRenewSubmit">确认续约</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import dayjs from 'dayjs'
import { adminGetPlatformTasks, adminProcessPlatformTask, adminGetPlatformTaskCount, adminRenewBuilding } from '../api'

const tasks = ref([])
const loading = ref(false)
const pendingCount = ref(0)
const status = ref('')
const page = ref(1)
const pageSize = 20
const total = ref(0)

const renewVisible = ref(false)
const renewTask = ref(null)
const renewFormRef = ref(null)
const renewForm = ref({ expired_at: '' })
const renewSubmitting = ref(false)

function typeText(type) {
  return { recruit: '招商申请', building_expiring: '公寓即将到期', building_expired: '公寓已到期' }[type] || type
}

function typeTag(type) {
  if (type === 'recruit') return 'warning'
  if (type === 'building_expired') return 'danger'
  return 'primary'
}

function isExpiryTask(task) {
  return task.type === 'building_expiring' || task.type === 'building_expired'
}

function buildingAddress(b) {
  return [b.district, b.street, b.village, b.building_no].filter(Boolean).join(' ')
}

function landlordText(b) {
  const list = b.landlords || []
  return list.map(l => [l.name, l.phone].filter(Boolean).join(' ')).filter(Boolean).join('、')
}

function disablePastDate(d) {
  return dayjs(d).isBefore(dayjs().startOf('day'))
}

function openRenew(task) {
  renewTask.value = task
  renewForm.value = { expired_at: task.due_date || '' }
  renewVisible.value = true
}

async function fetchTasks() {
  loading.value = true
  try {
    const r = await adminGetPlatformTasks(status.value, { page: page.value, page_size: pageSize })
    tasks.value = r.data.tasks || []
    total.value = r.data.total || 0
    if (status.value === '' || status.value === 'pending') {
      const c = await adminGetPlatformTaskCount()
      pendingCount.value = c.data.count || 0
    } else {
      pendingCount.value = 0
    }
  } catch {
    ElMessage.error('获取待办列表失败')
  } finally {
    loading.value = false
  }
}

function handleFilter() {
  page.value = 1
  fetchTasks()
}

function handlePage(p) {
  page.value = p
  fetchTasks()
}

async function handleProcess(id) {
  try {
    await adminProcessPlatformTask(id)
    ElMessage.success('已处理')
    window.dispatchEvent(new CustomEvent('tasks-changed'))
    await fetchTasks()
  } catch {
    ElMessage.error('操作失败')
  }
}

async function handleRenewSubmit() {
  const valid = await renewFormRef.value.validate().catch(() => false)
  if (!valid) return
  renewSubmitting.value = true
  try {
    await adminRenewBuilding(renewTask.value.building_id, { expired_at: renewForm.value.expired_at })
    ElMessage.success('续约成功，公寓已恢复展示')
    renewVisible.value = false
    window.dispatchEvent(new CustomEvent('tasks-changed'))
    await fetchTasks()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '续约失败')
  } finally {
    renewSubmitting.value = false
  }
}

async function handleDismissRenew() {
  if (!renewTask.value) return
  renewVisible.value = false
  await handleProcess(renewTask.value.id)
}

function formatTime(t) {
  if (!t) return '-'
  return dayjs(t).format('YYYY-MM-DD HH:mm:ss')
}

onMounted(() => {
  fetchTasks()
})
</script>

<style scoped>
.recruit-badge-wrap {
  line-height: 1;
}
.todo-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}
.filter-bar {
  margin-bottom: 14px;
}
.desktop-table {
  display: block;
  background: #fff;
  border-radius: 8px;
  padding: 12px;
}
.mobile-cards {
  display: none;
}
.pagination-wrap {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
}
.todo-card {
  display: flex;
  background: #fff;
  border-radius: 10px;
  margin-bottom: 12px;
  box-shadow: 0 2px 8px rgba(0,0,0,0.06);
  overflow: hidden;
}
.todo-card.pending {
  border: 1px solid #fde2e2;
}
.todo-card:not(.pending) {
  border: 1px solid #e8f5e9;
}
.card-indicator {
  width: 4px;
  flex-shrink: 0;
}
.todo-card.pending .card-indicator {
  background: linear-gradient(180deg, #f56c6c, #f89898);
}
.todo-card:not(.pending) .card-indicator {
  background: linear-gradient(180deg, #67c23a, #95d97a);
}
.card-body {
  flex: 1;
  padding: 12px 14px;
}
.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}
.card-title {
  font-size: 15px;
  color: #333;
  font-weight: 600;
  line-height: 1.5;
  margin-bottom: 8px;
}
.card-field {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin-bottom: 6px;
}
.field-icon {
  font-size: 14px;
  line-height: 1.5;
  width: 18px;
  text-align: center;
  flex-shrink: 0;
}
.field-value {
  font-size: 14px;
  color: #666;
  line-height: 1.5;
  word-break: break-all;
}
.card-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid #f5f5f5;
}
.card-time {
  font-size: 12px;
  color: #bbb;
}
.empty-text {
  text-align: center;
  padding: 32px 0;
  color: #999;
  font-size: 14px;
}

@media (max-width: 768px) {
  .desktop-table {
    display: none;
  }
  .mobile-cards {
    display: block;
  }
}
</style>
