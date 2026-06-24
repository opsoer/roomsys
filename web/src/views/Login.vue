<template>
  <div style="display: flex; justify-content: center; align-items: center; height: 100vh; background: #f5f7fa">
    <el-card style="width: 400px; max-width: 92vw; padding: 20px">
      <h2 style="text-align: center; margin-bottom: 10px; font-size: 24px">🏠 圳好租</h2>
      <p style="text-align: center; color: #999; margin-bottom: 30px; font-size: 14px">深圳公寓租赁管理平台</p>
      <el-form ref="formRef" :model="form" :rules="rules" label-width="0">
        <el-form-item prop="username">
          <el-input v-model="form.username" placeholder="用户名" size="large" :prefix-icon="UserIcon" />
        </el-form-item>
        <el-form-item prop="password">
          <el-input v-model="form.password" type="password" placeholder="密码" size="large" :prefix-icon="LockIcon" show-password @keyup.enter="handleLogin" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" size="large" style="width: 100%" :loading="loading" @click="handleLogin">
            登录
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { User as UserIcon, Lock as LockIcon } from '@element-plus/icons-vue'
import { login } from '../api'
import { ElMessage } from 'element-plus'

const router = useRouter()
const loading = ref(false)
const formRef = ref(null)
const form = reactive({ username: '', password: '' })
const rules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
}

async function handleLogin() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return
  loading.value = true
  try {
    const res = await login(form.username, form.password)
    const user = res.data.user
    localStorage.setItem('token', res.data.token)
    localStorage.setItem('username', user.username)
    localStorage.setItem('role', user.role)
    localStorage.setItem('building_id', String(user.building_id || ''))

    if (user.role === 'super_admin') {
      router.push('/admin/buildings')
    } else {
      router.push('/landlord/rooms')
    }
  } catch {
  } finally {
    loading.value = false
  }
}
</script>
