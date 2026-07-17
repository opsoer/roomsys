<template>
  <view class="login-page">
    <view class="login-bg" />
    <view class="login-content">
      <view class="login-brand">
        <text class="login-logo">
          <text class="house-big">🏠</text>
          <text class="house-small">🏠</text>
        </text>
        <text class="login-title">圳好租</text>
        <text class="login-desc">深圳公寓租赁管理平台</text>
      </view>
      <view class="login-card">
        <view class="form-group">
          <text class="form-label">用户名</text>
          <input class="form-input" v-model="form.username" placeholder="请输入用户名" />
        </view>
        <view class="form-group">
          <text class="form-label">密码</text>
          <input class="form-input" v-model="form.password" type="password" placeholder="请输入密码" />
        </view>
        <button class="login-btn" :disabled="loading" @click="handleLogin">
          <text v-if="loading">登录中...</text>
          <text v-else>登录</text>
        </button>
      </view>
    </view>
  </view>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { login } from '../../api'
import { auth } from '../../store/auth'

const loading = ref(false)
const form = reactive({ username: '', password: '' })

async function handleLogin() {
  if (!form.username || !form.password) {
    uni.showToast({ title: '请填写完整', icon: 'none' })
    return
  }
  loading.value = true
  try {
    const res = await login(form.username, form.password)
    const user = res.data.user
    auth.login(user, res.data.token, res.data.refresh_token)
    uni.showToast({ title: '登录成功', icon: 'success' })
    if (user.role === 'super_admin') {
      uni.redirectTo({ url: '/pages/admin/buildings' })
    } else {
      uni.redirectTo({ url: '/pages/landlord/rooms' })
    }
  } catch (e) {
    const msg = e?.response?.data?.message || e?.response?.data?.error
    uni.showToast({ title: msg || '登录失败', icon: 'none' })
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh; display: flex; align-items: center; justify-content: center;
  background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
  position: relative; overflow: hidden;
}
.login-bg {
  position: absolute; width: 600rpx; height: 600rpx; border-radius: 50%;
  background: radial-gradient(circle, rgba(230,162,60,0.1), transparent 70%);
  top: -200rpx; right: -100rpx;
}
.login-content { position: relative; z-index: 1; width: 100%; padding: 0 24px; }
.login-brand { text-align: center; margin-bottom: 36px; }
.login-logo { display: flex; align-items: flex-end; justify-content: center; position: relative; margin-bottom: 12px; }
.login-logo .house-big { position: relative; z-index: 1; font-size: 56px; margin-right: -10px; }
.login-logo .house-small { position: relative; z-index: 0; font-size: 42px; transform: translateY(4px); }
.login-title { font-size: 32px; font-weight: 800; color: #fff; letter-spacing: 6px; display: block; }
.login-desc { font-size: 14px; color: rgba(255,255,255,0.6); display: block; margin-top: 8px; }
.login-card {
  background: #fff; border-radius: 16px; padding: 24px 20px 20px; box-shadow: 0 8px 32px rgba(0,0,0,0.2);
}
.form-group { margin-bottom: 16px; }
.form-label { font-size: 14px; color: #333; font-weight: 500; display: block; margin-bottom: 6px; }
.form-input {
  width: 100%; height: 44px; border: 1px solid #dcdfe6; border-radius: 8px;
  padding: 0 12px; font-size: 15px; background: #fff;
}
.login-btn {
  width: 100%; height: 44px; background: linear-gradient(135deg, #1989fa, #3d9bf5);
  color: #fff; border: none; border-radius: 22px; font-size: 16px; font-weight: 600;
  margin-top: 24px; display: flex; align-items: center; justify-content: center;
}
.login-btn[disabled] { opacity: 0.6; }
</style>
