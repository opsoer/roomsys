<template>
  <view class="page-apply">
    <view class="apply-hero">
      <text class="apply-hero-title">成为房东</text>
      <text class="apply-hero-sub">把您的公寓放到平台出租，轻松管理</text>
    </view>

    <view class="apply-form">
      <view class="form-group">
        <text class="form-label">联系电话</text>
        <input class="form-input" v-model="form.phone" placeholder="输入您的手机号" type="number" maxlength="11" />
      </view>
      <view class="form-group">
        <text class="form-label">公寓地址</text>
        <textarea class="form-textarea" v-model="form.address" placeholder="输入公寓详细地址" rows="3" />
      </view>
      <button class="submit-btn" :disabled="submitting" @click="handleSubmit">
        <text v-if="submitting">提交中...</text>
        <text v-else>提交申请</text>
      </button>
      <text v-if="submitted" class="apply-success">提交成功！我们会尽快联系您</text>
    </view>
  </view>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { submitRecruit } from '../../api'

const form = reactive({ phone: '', address: '' })
const submitting = ref(false)
const submitted = ref(false)

async function handleSubmit() {
  if (!form.phone || !form.address) {
    uni.showToast({ title: '请填写完整', icon: 'none' })
    return
  }
  if (!/^1[3-9]\d{9}$/.test(form.phone)) {
    uni.showToast({ title: '请输入正确的手机号', icon: 'none' })
    return
  }
  submitting.value = true
  try {
    await submitRecruit({ phone: form.phone, address: form.address })
    submitted.value = true
    form.phone = ''
    form.address = ''
    uni.showToast({ title: '提交成功', icon: 'success' })
  } catch {
    uni.showToast({ title: '提交失败', icon: 'none' })
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.page-apply { min-height: 100vh; background: #f5f6fa; }
.apply-hero {
  background: linear-gradient(135deg, #1a1a2e, #16213e);
  padding: 32px 20px; text-align: center;
}
.apply-hero-title { font-size: 24px; font-weight: 700; color: #fff; display: block; }
.apply-hero-sub { font-size: 14px; color: rgba(255,255,255,0.6); display: block; margin-top: 8px; }
.apply-form { padding: 20px 16px; }
.form-group { margin-bottom: 16px; }
.form-label { font-size: 14px; color: #333; font-weight: 500; display: block; margin-bottom: 6px; }
.form-input {
  width: 100%; height: 44px; border: 1px solid #dcdfe6; border-radius: 8px;
  padding: 0 12px; font-size: 15px; background: #fff;
}
.form-textarea {
  width: 100%; border: 1px solid #dcdfe6; border-radius: 8px;
  padding: 10px 12px; font-size: 15px; background: #fff; min-height: 80px;
}
.submit-btn {
  width: 100%; height: 44px; background: #1989fa; color: #fff;
  border: none; border-radius: 22px; font-size: 16px; font-weight: 600;
  display: flex; align-items: center; justify-content: center; margin-top: 8px;
}
.submit-btn[disabled] { opacity: 0.6; }
.apply-success { display: block; text-align: center; color: #67c23a; font-size: 14px; margin-top: 12px; }
</style>
