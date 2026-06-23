<template>
  <div>
    <h3 style="margin-bottom: 20px">管理员管理</h3>
    <el-card>
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px">
        <h4>管理员列表</h4>
        <el-button type="primary" @click="openAddDialog">添加管理员</el-button>
      </div>
      <el-table :data="users" border stripe v-loading="loading">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="username" label="用户名" />
        <el-table-column prop="role" label="角色" width="140">
          <template #default="{ row }">
            <el-tag :type="row.role === 'super_admin' ? 'danger' : 'primary'" size="small">
              {{ row.role === 'super_admin' ? '超级管理员' : '普通管理员' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" />
        <el-table-column label="操作" width="160">
          <template #default="{ row }">
            <el-button size="small" type="primary" text @click="openEditDialog(row)">编辑</el-button>
            <el-button size="small" type="danger" text :disabled="row.role === 'super_admin'" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="showDialog" :title="editingId ? '编辑管理员' : '添加管理员'" width="400px">
      <el-form ref="formRef" :model="form" label-width="90px">
        <el-form-item v-if="!editingId" label="用户名" prop="username" :rules="[{ required: true, message: '请输入用户名' }]">
          <el-input v-model="form.username" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="form.password" type="password" show-password :placeholder="editingId ? '留空则不修改' : '请输入密码'" />
        </el-form-item>
        <el-form-item label="角色" prop="role">
          <el-radio-group v-model="form.role">
            <el-radio value="admin">普通管理员</el-radio>
            <el-radio value="super_admin">超级管理员</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showDialog = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">确定</el-button>
      </template>
    </el-dialog>
    <el-card style="margin-top: 20px">
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
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { buildingCreateAdmin, buildingGetUsers, adminUpdateUser, adminDeleteUser, buildingGetSystemTime, buildingSetSystemTime } from '../api'
import { ElMessage, ElMessageBox } from 'element-plus'

const users = ref([])
const loading = ref(false)
const showDialog = ref(false)
const submitting = ref(false)
const editingId = ref(null)
const form = ref({ username: '', password: '', role: 'admin' })
const formRef = ref(null)

const simulatedTime = ref('')
const timeLoading = ref(false)
const offsetDays = ref(0)
const offsetHours = ref(0)

async function refreshTime() {
  try {
    const res = await buildingGetSystemTime()
    simulatedTime.value = res.data.simulated_time
  } catch {}
}

async function handleSetTime() {
  timeLoading.value = true
  try {
    const totalSeconds = offsetDays.value * 86400 + offsetHours.value * 3600
    await buildingSetSystemTime(totalSeconds)
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
    await buildingSetSystemTime(0)
    ElMessage.success('已重置时间')
    await refreshTime()
  } finally {
    timeLoading.value = false
  }
}

async function fetchUsers() {
  loading.value = true
  try {
    const res = await buildingGetUsers()
    users.value = res.data.users
  } finally {
    loading.value = false
  }
}

function openAddDialog() {
  editingId.value = null
  form.value = { username: '', password: '', role: 'admin' }
  showDialog.value = true
}

function openEditDialog(row) {
  editingId.value = row.id
  form.value = { username: row.username, password: '', role: row.role }
  showDialog.value = true
}

async function handleSubmit() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return
  submitting.value = true
  try {
    if (editingId.value) {
      const data = { role: form.value.role }
      if (form.value.password) data.password = form.value.password
      await adminUpdateUser(editingId.value, data)
      ElMessage.success('修改成功')
    } else {
      await buildingCreateAdmin({ username: form.value.username, password: form.value.password })
      ElMessage.success('管理员创建成功')
    }
    showDialog.value = false
    await fetchUsers()
  } finally {
    submitting.value = false
  }
}

async function handleDelete(row) {
  try {
    await ElMessageBox.confirm(`确认删除管理员「${row.username}」？`, '提示')
    await adminDeleteUser(row.id)
    ElMessage.success('删除成功')
    await fetchUsers()
  } catch {}
}

onMounted(() => {
  fetchUsers()
  refreshTime()
})
</script>
