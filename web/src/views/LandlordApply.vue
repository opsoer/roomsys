<template>
  <div class="page-apply">
    <van-nav-bar title="房东入驻" left-arrow @click-left="$router.push('/')" />

    <div class="apply-hero">
      <h1 class="apply-hero-title">成为房东</h1>
      <p class="apply-hero-sub">把您的公寓放到平台出租，轻松管理</p>
    </div>

    <div class="apply-form">
      <van-form @submit="handleSubmit">
        <van-cell-group inset>
          <van-field
            v-model="form.phone"
            name="phone"
            label="联系电话"
            placeholder="输入您的手机号"
            :rules="[{ required: true, message: '请输入联系电话' }, { pattern: /^1\d{10}$/, message: '请输入正确的手机号' }]"
          />
          <van-field
            v-model="form.address"
            name="address"
            label="公寓地址"
            type="textarea"
            rows="3"
            placeholder="输入您的公寓详细地址（区域、街道、门牌号）"
            :rules="[{ required: true, message: '请输入公寓地址' }]"
          />
        </van-cell-group>
        <div style="margin: 24px 16px">
          <van-button round block type="primary" native-type="submit" :loading="submitting">
            提交申请
          </van-button>
        </div>
      </van-form>
      <p v-if="submitted" class="apply-success">提交成功！我们会尽快联系您</p>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { showToast } from 'vant'
import api from '../api'

const form = reactive({ phone: '', address: '' })
const submitting = ref(false)
const submitted = ref(false)

async function handleSubmit() {
  submitting.value = true
  try {
    await api.post('/recruit/submit', { phone: form.phone, address: form.address })
    submitted.value = true
    form.phone = ''
    form.address = ''
    showToast('提交成功')
  } catch {
    showToast('提交失败，请重试')
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.page-apply {
  min-height: 100vh;
  background: #f5f6fa;
}
.apply-hero {
  background: linear-gradient(135deg, #1a1a2e, #16213e);
  padding: 32px 20px;
  text-align: center;
}
.apply-hero-title {
  font-size: 24px;
  font-weight: 700;
  color: #fff;
  margin-bottom: 8px;
}
.apply-hero-sub {
  font-size: 14px;
  color: rgba(255,255,255,0.6);
}
.apply-form {
  margin-top: 16px;
}
.apply-success {
  text-align: center;
  color: #67c23a;
  font-size: 14px;
  margin-top: 8px;
}
</style>
