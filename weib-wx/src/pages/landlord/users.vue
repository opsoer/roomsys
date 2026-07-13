<template>
  <view class="page-users">
    <view class="header-row">
      <text class="page-title">管理员管理</text>
      <button class="add-btn" @click="openAddDialog">+ 添加管理员</button>
    </view>

    <view v-if="loading" class="loading-wrap"><text>加载中...</text></view>
    <view v-else-if="users.length === 0" class="empty-wrap"><text>暂无管理员</text></view>

    <view v-else class="user-list">
      <view v-for="item in users" :key="item.id" class="user-card">
        <view class="uc-head">
          <text class="uc-name">{{ item.username }}</text>
          <text :class="['uc-role', item.role === 'super_admin' ? 'super' : 'admin']">{{ item.role === 'super_admin' ? '超级管理员' : '普通管理员' }}</text>
        </view>
        <view class="uc-field"><text class="uc-label">ID</text><text class="uc-value">{{ item.id }}</text></view>
        <view class="uc-field"><text class="uc-label">创建时间</text><text class="uc-value">{{ item.created_at }}</text></view>
        <view class="uc-foot">
          <button v-if="currentUserRole === 'super_admin'" @click="openEditDialog(item)">编辑</button>
          <button v-if="currentUserRole === 'super_admin'" class="danger" :disabled="item.role === 'super_admin'" @click="handleDelete(item)">删除</button>
        </view>
      </view>
    </view>

    <!-- Add/Edit Dialog -->
    <view v-if="showDialog" class="overlay" @click="showDialog = false">
      <view class="small-dialog" @click.stop>
        <text class="dialog-title">{{ editingId ? '编辑管理员' : '添加管理员' }}</text>
        <view v-if="!editingId" class="form-group"><text class="form-label">用户名</text><input class="form-input" v-model="form.username" /></view>
        <view class="form-group"><text class="form-label">密码</text><input class="form-input" v-model="form.password" type="text" :placeholder="editingId ? '留空则不修改' : '请输入密码'" /></view>
        <view class="form-group"><text class="form-label">角色</text>
          <view class="radio-group">
            <text :class="['radio', form.role === 'admin' ? 'active' : '']" @click="form.role = 'admin'">普通管理员</text>
            <text :class="['radio', form.role === 'super_admin' ? 'active' : '']" @click="form.role = 'super_admin'">超级管理员</text>
          </view>
        </view>
        <view class="dialog-actions">
          <button class="dialog-btn cancel" @click="showDialog = false">取消</button>
          <button class="dialog-btn confirm" :disabled="submitting" @click="handleSubmit">{{ submitting ? '提交中...' : '确定' }}</button>
        </view>
      </view>
    </view>
  </view>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { buildingGetUsers, buildingCreateAdmin, adminUpdateUser, adminDeleteUser } from '../../api'
import { auth } from '../../store/auth'

const users = ref([])
const loading = ref(false)
const showDialog = ref(false)
const submitting = ref(false)
const editingId = ref(null)
const form = ref({ username: '', password: '', role: 'admin' })
const currentUserRole = auth.role

async function fetchUsers() {
  loading.value = true
  try {
    const res = await buildingGetUsers()
    users.value = res.data.users || []
  } catch { uni.showToast({ title: '获取失败', icon: 'none' }) }
  finally { loading.value = false }
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
  submitting.value = true
  try {
    if (editingId.value) {
      const data = { role: form.value.role }
      if (form.value.password) data.password = form.value.password
      await adminUpdateUser(editingId.value, data)
    } else {
      await buildingCreateAdmin({ username: form.value.username, password: form.value.password })
    }
    uni.showToast({ title: '保存成功', icon: 'success' })
    showDialog.value = false
    await fetchUsers()
  } catch { uni.showToast({ title: '操作失败', icon: 'none' }) }
  finally { submitting.value = false }
}

async function handleDelete(row) {
  uni.showModal({
    title: '确认删除',
    content: `确认删除管理员「${row.username}」？`,
    success: async (res) => {
      if (!res.confirm) return
      try {
        await adminDeleteUser(row.id)
        uni.showToast({ title: '已删除', icon: 'success' })
        await fetchUsers()
      } catch { uni.showToast({ title: '删除失败', icon: 'none' }) }
    }
  })
}

onMounted(fetchUsers)
</script>

<style scoped>
.page-users { padding: 16px; min-height: 100vh; }
.header-row { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.page-title { font-size: 20px; font-weight: 700; color: #1a1a2e; }
.add-btn { background: #1989fa; color: #fff; border: none; border-radius: 8px; padding: 8px 16px; font-size: 14px; }
.loading-wrap, .empty-wrap { text-align: center; padding: 60px 0; color: #999; }
.user-card { background: #fff; border-radius: 10px; padding: 14px; margin-bottom: 12px; border: 1px solid #eee; }
.uc-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
.uc-name { font-size: 15px; font-weight: 600; color: #333; }
.uc-role { font-size: 11px; padding: 2px 10px; border-radius: 10px; color: #fff; }
.uc-role.super { background: #f56c6c; }
.uc-role.admin { background: #409eff; }
.uc-field { display: flex; gap: 8px; margin-bottom: 4px; }
.uc-label { font-size: 12px; color: #999; min-width: 60px; }
.uc-value { font-size: 13px; color: #666; }
.uc-foot { display: flex; gap: 12px; margin-top: 10px; padding-top: 10px; border-top: 1px solid #f5f5f5; }
.uc-foot button { font-size: 13px; color: #1989fa; background: none; border: none; padding: 4px 8px; }
.uc-foot button.danger { color: #f56c6c; }
.overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); z-index: 1000; display: flex; align-items: flex-end; }
.small-dialog { width: 100%; background: #fff; border-radius: 16px 16px 0 0; padding: 20px; }
.dialog-title { font-size: 18px; font-weight: 700; display: block; margin-bottom: 16px; }
.form-group { margin-bottom: 12px; }
.form-label { font-size: 14px; color: #333; display: block; margin-bottom: 4px; }
.form-input { width: 100%; height: 40px; border: 1px solid #dcdfe6; border-radius: 8px; padding: 0 12px; font-size: 14px; background: #fff; }
.radio-group { display: flex; gap: 10px; }
.radio { padding: 8px 20px; border: 1px solid #dcdfe6; border-radius: 8px; font-size: 14px; color: #666; }
.radio.active { border-color: #1989fa; color: #1989fa; background: #ecf5ff; }
.dialog-actions { display: flex; gap: 12px; margin-top: 20px; }
.dialog-btn { flex: 1; height: 44px; border-radius: 22px; display: flex; align-items: center; justify-content: center; font-size: 15px; }
.dialog-btn.cancel { background: #f5f5f5; color: #666; border: none; }
.dialog-btn.confirm { background: #1989fa; color: #fff; border: none; }
.dialog-btn.confirm[disabled] { opacity: 0.6; }
</style>
