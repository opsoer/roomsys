<template>
  <div style="max-width: 1100px; margin: 0 auto;">
    <div style="display: flex; align-items: center; gap: 12px; margin-bottom: 16px">
      <h2 style="font-size: 22px; font-weight: 700; margin: 0">操作审计</h2>
      <el-button size="small" :loading="loading" @click="loadLogs">刷新</el-button>
      <el-select v-model="resource" placeholder="全部资源" size="small" clearable style="width: 140px" @change="loadLogs">
        <el-option v-for="r in resourceOptions" :key="r" :label="r" :value="r" />
      </el-select>
    </div>

    <el-table v-loading="loading" :data="logs" border size="small" style="width: 100%">
      <el-table-column prop="created_at" label="时间" width="170">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column prop="username" label="操作人" width="110">
        <template #default="{ row }">{{ row.username || '系统' }}</template>
      </el-table-column>
      <el-table-column prop="role" label="角色" width="100" />
      <el-table-column prop="action" label="动作" width="90" />
      <el-table-column prop="resource" label="资源" width="100" />
      <el-table-column prop="resource_id" label="对象" width="90" />
      <el-table-column prop="detail" label="说明" min-width="240" show-overflow-tooltip />
      <el-table-column prop="ip" label="IP" width="130" />
    </el-table>

    <div style="display: flex; justify-content: flex-end; margin-top: 12px">
      <el-pagination
        layout="total, prev, pager, next"
        :total="total"
        :page-size="limit"
        :current-page="page"
        @current-change="onPageChange"
      />
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { adminListAuditLogs } from '../api'

const logs = ref([])
const total = ref(0)
const page = ref(1)
const limit = 50
const loading = ref(false)
const resource = ref('')
const resourceOptions = [
  'building', 'landlord', 'room', 'media', 'contract', 'bill',
  'shareholder', 'dividend', 'task', 'recruit', 'user',
  'settings', 'location', 'time', 'job',
]

async function loadLogs() {
  loading.value = true
  try {
    const res = await adminListAuditLogs({
      page: page.value,
      limit,
      resource: resource.value || undefined,
    })
    logs.value = res.data.logs || []
    total.value = res.data.total || 0
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '加载审计记录失败')
  } finally {
    loading.value = false
  }
}

function onPageChange(p) {
  page.value = p
  loadLogs()
}

function formatTime(value) {
  if (!value) return ''
  return String(value).replace('T', ' ').slice(0, 19)
}

onMounted(loadLogs)
</script>
