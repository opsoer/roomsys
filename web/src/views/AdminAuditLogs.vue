<template>
  <div style="max-width: 1200px; margin: 0 auto;">
    <div style="display: flex; align-items: center; gap: 12px; margin-bottom: 16px">
      <h2 style="font-size: 22px; font-weight: 700; margin: 0">操作审计</h2>
      <el-button size="small" :loading="loading" @click="loadLogs">刷新</el-button>
      <el-button size="small" type="danger" plain @click="openCleanup">清理记录</el-button>
    </div>

    <!-- 筛选栏 -->
    <div class="filter-bar">
      <el-select v-model="filters.resource" placeholder="全部资源" size="small" clearable style="width: 130px" @change="onFilterChange">
        <el-option v-for="r in resourceOptions" :key="r.value" :label="r.label" :value="r.value" />
      </el-select>
      <el-select v-model="filters.action" placeholder="全部动作" size="small" clearable style="width: 120px" @change="onFilterChange">
        <el-option v-for="a in actionOptions" :key="a.value" :label="a.label" :value="a.value" />
      </el-select>
      <el-select v-model="filters.role" placeholder="全部角色" size="small" clearable style="width: 130px" @change="onFilterChange">
        <el-option v-for="r in roleOptions" :key="r.value" :label="r.label" :value="r.value" />
      </el-select>
      <el-input
        v-model="filters.username"
        placeholder="按操作人搜索"
        size="small"
        clearable
        style="width: 160px"
        @keyup.enter="onFilterChange"
        @clear="onFilterChange"
      />
      <el-date-picker
        v-model="filters.range"
        type="datetimerange"
        size="small"
        start-placeholder="开始时间"
        end-placeholder="结束时间"
        value-format="YYYY-MM-DD HH:mm:ss"
        :default-time="[new Date(2000, 0, 1, 0, 0, 0), new Date(2000, 0, 1, 23, 59, 59)]"
        style="width: 360px"
        @change="onFilterChange"
      />
      <el-button size="small" type="primary" :loading="loading" @click="onFilterChange">查询</el-button>
      <el-button size="small" @click="resetFilters">重置</el-button>
    </div>

    <el-table v-loading="loading" :data="logs" border size="small" style="width: 100%">
      <el-table-column prop="created_at" label="时间" width="170">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column prop="username" label="操作人" width="110">
        <template #default="{ row }">{{ row.username || '系统' }}</template>
      </el-table-column>
      <el-table-column prop="role" label="角色" width="110">
        <template #default="{ row }">{{ roleLabel(row.role) }}</template>
      </el-table-column>
      <el-table-column prop="action" label="动作" width="96">
        <template #default="{ row }">
          <el-tag :type="actionTagType(row.action)" size="small" effect="plain">{{ actionLabel(row.action) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="resource" label="资源" width="100">
        <template #default="{ row }">{{ resourceLabel(row.resource) }}</template>
      </el-table-column>
      <el-table-column prop="resource_id" label="对象" width="90" />
      <el-table-column prop="detail" label="说明" min-width="240" show-overflow-tooltip />
      <el-table-column prop="ip" label="IP" width="130" />
    </el-table>

    <div style="display: flex; justify-content: flex-end; margin-top: 12px">
      <el-pagination
        layout="total, sizes, prev, pager, next"
        :total="total"
        :page-size="limit"
        :page-sizes="[20, 50, 100, 200]"
        :current-page="page"
        @current-change="onPageChange"
        @size-change="onSizeChange"
      />
    </div>
    <!-- 清理记录弹窗：删除指定时间之前（可叠加条件收窄）的审计记录 -->
    <el-dialog v-model="showCleanup" title="清理审计记录" width="520px" destroy-on-close @open="refreshCleanupCount">
      <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 16px;">
        删除将不可恢复。请选择时间点，删除该时间点<b>之前</b>的记录；可叠加条件缩小范围。本次清理操作本身会留痕。
      </el-alert>
      <el-form :model="cleanupForm" label-width="90px">
        <el-form-item label="删除早于" required>
          <el-date-picker
            v-model="cleanupForm.before_time"
            type="datetime"
            placeholder="选择时间点"
            value-format="YYYY-MM-DD HH:mm:ss"
            style="width: 100%"
            @change="onCleanupFormChange"
          />
        </el-form-item>
        <el-form-item label="资源">
          <el-select v-model="cleanupForm.resource" placeholder="全部资源" clearable style="width: 100%" @change="onCleanupFormChange">
            <el-option v-for="r in resourceOptions" :key="r.value" :label="r.label" :value="r.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="动作">
          <el-select v-model="cleanupForm.action" placeholder="全部动作" clearable style="width: 100%" @change="onCleanupFormChange">
            <el-option v-for="a in actionOptions" :key="a.value" :label="a.label" :value="a.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="角色">
          <el-select v-model="cleanupForm.role" placeholder="全部角色" clearable style="width: 100%" @change="onCleanupFormChange">
            <el-option v-for="r in roleOptions" :key="r.value" :label="r.label" :value="r.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="操作人">
          <el-input v-model="cleanupForm.username" placeholder="留空则不限" clearable @keyup.enter="onCleanupFormChange" @clear="onCleanupFormChange" />
        </el-form-item>
      </el-form>
      <div style="margin-bottom: 4px; font-size: 14px;">
        <template v-if="!cleanupForm.before_time">请先选择时间点</template>
        <template v-else>
          将删除
          <b :style="{ color: '#f56c6c', fontSize: '16px' }">{{ cleanupCount == null ? '…' : cleanupCount }}</b>
          条记录
          <el-button link type="primary" size="small" :loading="cleanupCounting" @click="refreshCleanupCount">重新统计</el-button>
        </template>
      </div>
      <template #footer>
        <el-button @click="showCleanup = false">取消</el-button>
        <el-button
          type="danger"
          :loading="cleanupSubmitting"
          :disabled="!cleanupForm.before_time || cleanupCount == null || cleanupCount === 0"
          @click="handleCleanup"
        >
          确认清理
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import dayjs from 'dayjs'
import { adminListAuditLogs, adminDeleteAuditLogs } from '../api'

const logs = ref([])
const total = ref(0)
const page = ref(1)
const limit = ref(50)
const loading = ref(false)

// 筛选条件（range 为时间范围数组，提交时拆成 start_time / end_time）
const filters = reactive({
  resource: '',
  action: '',
  role: '',
  username: '',
  range: null,
})

// 动作中英文映射（与 audit 包常量一一对应）
const actionMap = {
  create: '新增',
  update: '修改',
  delete: '删除',
  status: '状态变更',
  audit: '审核',
  settle: '结算/发放',
  import: '批量导入',
  trigger: '手动触发',
  config: '配置变更',
  login: '登录',
  logout: '登出',
}
const actionOptions = Object.entries(actionMap).map(([value, label]) => ({ value, label }))
// 动作标签配色：新增绿、删除红、变更橙，其余默认
const actionTagTypeMap = { create: 'success', delete: 'danger', update: 'warning', status: 'warning', config: 'warning' }

// 资源中英文映射（与 audit 包常量一一对应）
const resourceMap = {
  building: '公寓',
  landlord: '房东',
  room: '房间',
  media: '房源图片',
  contract: '租约',
  bill: '账单',
  shareholder: '股东',
  dividend: '分红',
  task: '待办任务',
  recruit: '招募',
  user: '用户',
  settings: '系统设置',
  location: '位置数据',
  time: '模拟时间',
  job: '定时任务',
}
const resourceOptions = Object.entries(resourceMap).map(([value, label]) => ({ value, label }))

// 角色中英文映射（与用户表 role 取值一致；空值多为历史记录或系统操作）
const roleMap = {
  super_admin: '超级管理员',
  building_admin: '公寓管理员',
  admin: '管理员',
  system: '系统',
}
const roleOptions = Object.entries(roleMap).map(([value, label]) => ({ value, label }))

function actionLabel(v) {
  return actionMap[v] || v || '—'
}
function resourceLabel(v) {
  return resourceMap[v] || v || '—'
}
function roleLabel(v) {
  return roleMap[v] || v || '—'
}
function actionTagType(v) {
  return actionTagTypeMap[v] || 'info'
}

async function loadLogs() {
  loading.value = true
  try {
    const [start, end] = filters.range || []
    const res = await adminListAuditLogs({
      page: page.value,
      limit: limit.value,
      resource: filters.resource || undefined,
      action: filters.action || undefined,
      role: filters.role || undefined,
      username: filters.username.trim() || undefined,
      start_time: start || undefined,
      end_time: end || undefined,
    })
    logs.value = res.data.logs || []
    total.value = res.data.total || 0
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '加载审计记录失败')
  } finally {
    loading.value = false
  }
}

// 任一筛选条件变化：回到第 1 页再查询
function onFilterChange() {
  page.value = 1
  loadLogs()
}

function resetFilters() {
  filters.resource = ''
  filters.action = ''
  filters.role = ''
  filters.username = ''
  filters.range = null
  onFilterChange()
}

function onPageChange(p) {
  page.value = p
  loadLogs()
}

function onSizeChange(size) {
  limit.value = size
  page.value = 1
  loadLogs()
}

function formatTime(value) {
  if (!value) return ''
  return String(value).replace('T', ' ').slice(0, 19)
}

// ===== 清理审计记录 =====
const showCleanup = ref(false)
const cleanupCount = ref(null)
const cleanupCounting = ref(false)
const cleanupSubmitting = ref(false)
// 默认把当前列表筛选条件带进来，方便「按当前看到的范围」清理
const cleanupForm = reactive({ before_time: '', resource: '', action: '', role: '', username: '' })

function openCleanup() {
  cleanupForm.before_time = dayjs().subtract(3, 'month').startOf('day').format('YYYY-MM-DD HH:mm:ss')
  cleanupForm.resource = filters.resource
  cleanupForm.action = filters.action
  cleanupForm.role = filters.role
  cleanupForm.username = filters.username
  cleanupCount.value = null
  showCleanup.value = true
}

// 统计将删除的条数（复用列表接口：同条件 + before_time，只取 total）
async function refreshCleanupCount() {
  if (!cleanupForm.before_time) return
  cleanupCounting.value = true
  try {
    const res = await adminListAuditLogs({
      page: 1,
      limit: 1,
      resource: cleanupForm.resource || undefined,
      action: cleanupForm.action || undefined,
      role: cleanupForm.role || undefined,
      username: cleanupForm.username.trim() || undefined,
      before_time: cleanupForm.before_time,
    })
    cleanupCount.value = res.data.total || 0
  } catch {
    cleanupCount.value = null
    ElMessage.error('统计待删除条数失败')
  } finally {
    cleanupCounting.value = false
  }
}

// 弹窗内条件变化后重新统计
function onCleanupFormChange() {
  cleanupCount.value = null
  refreshCleanupCount()
}

async function handleCleanup() {
  const count = cleanupCount.value
  try {
    await ElMessageBox.confirm(
      `将永久删除 ${count} 条审计记录，且不可恢复。是否继续？`,
      '清理确认',
      { confirmButtonText: '确认删除', cancelButtonText: '取消', type: 'warning' }
    )
  } catch {
    return
  }
  cleanupSubmitting.value = true
  try {
    const res = await adminDeleteAuditLogs({
      before_time: cleanupForm.before_time,
      resource: cleanupForm.resource || undefined,
      action: cleanupForm.action || undefined,
      role: cleanupForm.role || undefined,
      username: cleanupForm.username.trim() || undefined,
    })
    ElMessage.success(`已删除 ${res.data?.deleted ?? count} 条审计记录；本次清理操作已留痕，其后新产生的记录会正常显示`)
    showCleanup.value = false
    page.value = 1
    loadLogs()
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '清理失败')
  } finally {
    cleanupSubmitting.value = false
  }
}

onMounted(loadLogs)
</script>

<style scoped>
.filter-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  margin-bottom: 12px;
}
</style>
