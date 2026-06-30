<template>
  <div>
    <div class="phone-section">
      <div class="phone-header" @click="showPhone = !showPhone">
        <h3>招商电话</h3>
        <el-icon :class="{ rotated: showPhone }"><ArrowDown /></el-icon>
      </div>
      <div v-show="showPhone" style="display:flex;gap:10px;max-width:400px;">
        <el-input v-model="phone" placeholder="输入房东入驻热线电话" />
        <el-button type="primary" @click="handleSavePhone" :loading="saving">保存</el-button>
      </div>
      <div v-show="showPhone" style="font-size:12px;color:#999;margin-top:4px;">设置后将在网站首页最前面显示</div>
    </div>

    <el-divider />

    <div class="todo-header">
      <h3>代办事项</h3>
      <el-badge :value="pendingCount" :hidden="!pendingCount" class="recruit-badge-wrap">
        <el-tag :type="pendingCount ? 'danger' : 'success'">
          {{ pendingCount ? pendingCount + ' 条待处理' : '全部已处理' }}
        </el-tag>
      </el-badge>
    </div>

    <div class="desktop-table">
      <el-table :data="submissions" v-loading="loading" empty-text="暂无入驻申请" style="width:100%">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="phone" label="联系电话" width="140" />
        <el-table-column prop="address" label="公寓地址" min-width="200" />
        <el-table-column prop="created_at" label="提交时间" width="180">
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
        <el-table-column label="操作" width="100">
          <template #default="{ row }">
            <el-button v-if="row.status === 'pending'" size="small" type="primary" @click="handleProcess(row.id)">
              标记已处理
            </el-button>
            <span v-else style="color:#999;font-size:13px;">已完成</span>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <div class="mobile-cards" v-loading="loading">
      <div v-for="item in submissions" :key="item.id" :class="['todo-card', item.status]">
        <div class="card-indicator"></div>
        <div class="card-body">
          <div class="card-head">
            <span class="card-label">入驻申请 #{{ item.id }}</span>
            <el-tag :type="item.status === 'pending' ? 'danger' : 'success'" size="small" effect="dark" round>
              {{ item.status === 'pending' ? '待处理' : '已处理' }}
            </el-tag>
          </div>
          <div class="card-field">
            <span class="field-icon">📞</span>
            <span class="field-value">{{ item.phone }}</span>
          </div>
          <div class="card-field card-addr">
            <span class="field-icon">📍</span>
            <span class="field-value">{{ item.address }}</span>
          </div>
          <div class="card-foot">
            <span class="card-time">🕐 {{ formatTime(item.created_at) }}</span>
            <el-button v-if="item.status === 'pending'" size="small" type="primary" round @click="handleProcess(item.id)">
              标记已处理
            </el-button>
            <el-tag v-else type="success" size="small" effect="plain" round>已完成</el-tag>
          </div>
        </div>
      </div>
      <div v-if="!loading && submissions.length === 0" class="empty-text">暂无入驻申请</div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { ArrowDown } from '@element-plus/icons-vue'
import api from '../api'

const showPhone = ref(false)

const phone = ref('')
const saving = ref(false)
const submissions = ref([])
const loading = ref(false)
const pendingCount = ref(0)

async function fetchPhone() {
  try {
    const r = await api.get('/admin/settings/recruit_phone')
    phone.value = r.data?.value || ''
  } catch {
    ElMessage.error('获取招商电话失败')
  }
}

async function handleSavePhone() {
  saving.value = true
  try {
    await api.put('/admin/settings/recruit_phone', { value: phone.value })
    ElMessage.success('保存成功')
  } catch {
    ElMessage.error('保存失败')
  } finally {
    saving.value = false
  }
}

async function fetchSubmissions() {
  loading.value = true
  try {
    const r = await api.get('/admin/recruit/list')
    submissions.value = r.data.submissions || []
    pendingCount.value = submissions.value.filter(s => s.status === 'pending').length
  } catch {
    ElMessage.error('获取入驻申请列表失败')
  } finally {
    loading.value = false
  }
}

async function handleProcess(id) {
  try {
    await api.put(`/admin/recruit/process/${id}`)
    ElMessage.success('已处理')
    await fetchSubmissions()
  } catch {
    ElMessage.error('操作失败')
  }
}

function formatTime(t) {
  if (!t) return '-'
  return new Date(t).toLocaleString('zh-CN', { hour12: false })
}

onMounted(() => {
  fetchPhone()
  fetchSubmissions()
})
</script>

<style scoped>
.recruit-badge-wrap {
  line-height: 1;
}
.phone-header {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  margin-bottom: 12px;
  user-select: none;
}
.phone-header h3 {
  margin: 0;
}
.phone-header .el-icon {
  transition: transform 0.2s;
  font-size: 14px;
  color: #999;
}
.phone-header .rotated {
  transform: rotate(-180deg);
}
.todo-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}
.desktop-table {
  display: block;
}
.mobile-cards {
  display: none;
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
.card-label {
  font-size: 12px;
  font-weight: 600;
  color: #999;
  letter-spacing: 0.5px;
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
  color: #333;
  line-height: 1.5;
  word-break: break-all;
}
.card-addr .field-value {
  color: #666;
  font-size: 13px;
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
  .phone-section h3 {
    font-size: 15px;
  }
  .desktop-table {
    display: none;
  }
  .mobile-cards {
    display: block;
  }
}
</style>
