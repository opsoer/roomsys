<template>
  <div class="login-page">
    <div class="login-bg"></div>
    <div class="login-content">
      <div class="login-brand">
        <div class="login-logo">☀️</div>
        <h1 class="login-title">圳好租</h1>
        <p class="login-desc">深圳公寓租赁管理平台</p>
      </div>
      <div class="login-card">
        <van-form @submit="handleLogin">
          <van-field
            v-model="form.username"
            name="username"
            label="用户名"
            placeholder="请输入用户名"
            :rules="[{ required: true, message: '请输入用户名' }]"
            left-icon="contact"
            clearable
          />
          <van-field
            v-model="form.password"
            type="password"
            name="password"
            label="密码"
            placeholder="请输入密码"
            :rules="[{ required: true, message: '请输入密码' }]"
            left-icon="lock"
            clearable
          />
          <div class="login-btn-wrapper">
            <van-button round block type="primary" native-type="submit" :loading="loading">
              登录
            </van-button>
          </div>
        </van-form>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { showToast } from 'vant'
import { login } from '../api'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const authStore = useAuthStore()
const loading = ref(false)
const form = reactive({ username: '', password: '' })

async function handleLogin() {
  loading.value = true
  try {
    const res = await login(form.username, form.password)
    const user = res.data.user
    authStore.login(user, res.data.token, res.data.refresh_token)

    showToast({ message: '登录成功', icon: 'success', duration: 1500 })

    if (user.role === 'super_admin') {
      router.push('/admin/buildings')
    } else {
      router.push('/landlord/rooms')
    }
  } catch (e) {
    const msg = e?.response?.data?.message || e?.response?.data?.error
    if (msg) {
      showToast(msg)
    } else {
      showToast('登录失败，请重试')
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
  position: relative;
  overflow: hidden;
}
.login-bg {
  position: absolute;
  width: 600px;
  height: 600px;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(230,162,60,0.1), transparent 70%);
  top: -200px;
  right: -100px;
}
.login-content {
  position: relative;
  z-index: 1;
  width: 100%;
  padding: 0 24px;
  max-width: 400px;
}
.login-brand {
  text-align: center;
  margin-bottom: 36px;
}
.login-logo {
  font-size: 56px;
  margin-bottom: 12px;
}
.login-title {
  font-size: 32px;
  font-weight: 800;
  color: #fff;
  letter-spacing: 6px;
  margin-bottom: 8px;
}
.login-desc {
  font-size: 14px;
  color: rgba(255,255,255,0.6);
}
.login-card {
  background: #fff;
  border-radius: 16px;
  padding: 24px 0 8px;
  box-shadow: 0 8px 32px rgba(0,0,0,0.2);
}
.login-btn-wrapper {
  margin: 28px 16px 16px;
}
:deep(.van-field) {
  padding: 12px 16px;
}
</style>