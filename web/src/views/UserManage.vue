<template>
  <div>
    <h3 style="margin-bottom: 20px">管理员管理</h3>
    <el-card>
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px">
        <h4>管理员列表</h4>
        <el-button type="primary" @click="openAddDialog">添加管理员</el-button>
      </div>
      <div class="desktop-table">
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
              <el-button v-if="currentUserRole === 'super_admin'" size="small" type="primary" text @click="openEditDialog(row)">编辑</el-button>
              <el-button v-if="currentUserRole === 'super_admin'" size="small" type="danger" text :disabled="row.role === 'super_admin'" @click="handleDelete(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>
      <div class="mobile-cards" v-loading="loading">
        <div v-if="loading" class="skeleton-list">
          <div v-for="n in 3" :key="n" class="skeleton-item">
            <el-skeleton :rows="2" animated>
              <template #template>
                <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:10px;">
                  <el-skeleton-item variant="h3" style="width:30%;" />
                  <el-skeleton-item variant="rect" style="width:80px;height:24px;border-radius:12px;" />
                </div>
                <el-skeleton-item variant="text" style="width:50%;margin-bottom:4px;" />
                <el-skeleton-item variant="text" style="width:40%;" />
              </template>
            </el-skeleton>
          </div>
        </div>
        <template v-else>
        <div v-for="item in users" :key="item.id" class="user-card">
          <div class="uc-head">
            <span class="uc-name">{{ item.username }}</span>
            <el-tag :type="item.role === 'super_admin' ? 'danger' : 'primary'" size="small" effect="dark" round>
              {{ item.role === 'super_admin' ? '超级管理员' : '普通管理员' }}
            </el-tag>
          </div>
          <div class="uc-field">
            <span class="uc-label">ID</span>
            <span class="uc-value">{{ item.id }}</span>
          </div>
          <div class="uc-field">
            <span class="uc-label">创建时间</span>
            <span class="uc-value">{{ item.created_at }}</span>
          </div>
          <div class="uc-foot">
            <el-button v-if="currentUserRole === 'super_admin'" size="small" type="primary" text @click="openEditDialog(item)">编辑</el-button>
            <el-button v-if="currentUserRole === 'super_admin'" size="small" type="danger" text :disabled="item.role === 'super_admin'" @click="handleDelete(item)">删除</el-button>
          </div>
        </div>
        <div v-if="!loading && users.length === 0" class="empty-text">暂无管理员</div>
        </template>
      </div>
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
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { buildingCreateAdmin, buildingGetUsers, adminUpdateUser, adminDeleteUser } from '../api'
import { ElMessage, ElMessageBox } from 'element-plus'

const users = ref([])
const loading = ref(false)
const showDialog = ref(false)
const submitting = ref(false)
const editingId = ref(null)
const form = ref({ username: '', password: '', role: 'admin' })
const formRef = ref(null)
const currentUserRole = localStorage.getItem('role')

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
  } catch {
    ElMessage.error(editingId.value ? '修改失败' : '创建失败')
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
  } catch (e) {
    if (e !== 'cancel' && e !== 'close') {
      ElMessage.error('删除失败')
    }
  }
}

onMounted(() => {
  fetchUsers()
})
</script>

<style scoped>
.desktop-table {
  display: block;
}
.mobile-cards {
  display: none;
}
.user-card {
  background: #fff;
  border-radius: 10px;
  padding: 14px;
  margin-bottom: 12px;
  border: 1px solid #eee;
  box-shadow: 0 1px 4px rgba(0,0,0,0.04);
}
.uc-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}
.uc-name {
  font-size: 15px;
  font-weight: 600;
  color: #333;
}
.uc-field {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}
.uc-label {
  font-size: 12px;
  color: #999;
  min-width: 56px;
}
.uc-value {
  font-size: 13px;
  color: #666;
}
.uc-foot {
  display: flex;
  gap: 12px;
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid #f5f5f5;
}
.empty-text {
  text-align: center;
  padding: 32px 0;
  color: #999;
  font-size: 14px;
}
.skeleton-list { padding: 12px 0; }
.skeleton-item { padding: 14px; margin-bottom: 12px; background: #fff; border-radius: 10px; border: 1px solid #eee; }

@media (max-width: 768px) {
  .desktop-table {
    display: none;
  }
  .mobile-cards {
    display: block;
  }
  .el-card { padding: 12px; }
}
</style>
