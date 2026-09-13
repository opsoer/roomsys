<template>
  <van-popup :show="show" round position="bottom" :style="{ padding: '20px' }" @update:show="v => emit('update:show', v)">
    <div class="captcha-dialog">
      <div class="captcha-title">安全验证</div>
      <div class="captcha-desc">今日查看次数已达上限，请输入图中数字继续查看</div>
      <div class="captcha-row">
        <img v-if="captchaImg" :src="captchaImg" class="captcha-img" alt="验证码" @click="load" />
        <div v-else class="captcha-loading">加载中…</div>
        <van-icon name="replay" size="20" color="#969799" @click="load" />
      </div>
      <van-field v-model="code" type="digit" maxlength="4" placeholder="请输入图中 4 位数字" clearable @keyup.enter="submit" />
      <van-button type="primary" block round :loading="submitting" style="margin-top: 14px" @click="submit">确 认</van-button>
    </div>
  </van-popup>
</template>

<script setup>
// 通用图片验证码弹窗：弹窗打开时自动取图；确认后调用外部 handler(payload)，
// 返回 true 视为通过并关闭弹窗，false 刷新验证码等待重输（错误提示由全局拦截器 toast）。
import { ref, watch } from 'vue'
import { getCaptcha } from '../../api'

const props = defineProps({
  show: { type: Boolean, default: false },
  handler: { type: Function, required: true },
})
const emit = defineEmits(['update:show'])

const captchaImg = ref('')
const captchaId = ref('')
const code = ref('')
const submitting = ref(false)

watch(() => props.show, (v) => {
  if (v) {
    code.value = ''
    load()
  }
})

async function load() {
  captchaImg.value = ''
  try {
    const res = await getCaptcha()
    captchaId.value = res.data.captcha_id
    captchaImg.value = res.data.image
  } catch (e) {
    /* 取图失败保持空白，用户可点击重试 */
  }
}

async function submit() {
  if (!code.value || code.value.length < 4 || submitting.value) return
  submitting.value = true
  try {
    const ok = await props.handler({ captcha_id: captchaId.value, captcha_code: code.value })
    if (ok) {
      emit('update:show', false)
      code.value = ''
    } else {
      load()
    }
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.captcha-dialog {
  text-align: center;
}
.captcha-title {
  font-size: 17px;
  font-weight: 600;
  color: #323233;
}
.captcha-desc {
  font-size: 13px;
  color: #969799;
  margin: 8px 0 14px;
}
.captcha-row {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  margin-bottom: 12px;
}
.captcha-img {
  height: 80px;
  border-radius: 8px;
  border: 1px solid #ebedf0;
}
.captcha-loading {
  width: 240px;
  height: 80px;
  line-height: 80px;
  border: 1px dashed #ebedf0;
  border-radius: 8px;
  color: #c8c9cc;
  font-size: 13px;
}
</style>
