<template>
  <van-popup v-model:show="visible" round position="center" :style="{ padding: '20px 16px', width: '82%', maxWidth: '360px' }">
    <div style="text-align: center;">
      <div style="font-size: 17px; font-weight: 700; color: #1a1a2e;">{{ title }}</div>
      <div style="font-size: 12px; color: #999; margin: 4px 0 12px;">扫一扫，即刻查看</div>
      <img v-if="dataUrl" :src="dataUrl" alt="二维码" style="width: 100%; border-radius: 10px;" />
      <el-icon v-else class="is-loading" style="font-size: 36px; color: #999"><Loading /></el-icon>
      <div style="font-size: 12px; color: #999; margin-top: 10px; word-break: break-all;">{{ link }}</div>
      <div style="display: flex; gap: 10px; margin-top: 14px;">
        <van-button block round @click="handleCopy">复制链接</van-button>
        <van-button type="primary" block round @click="handleDownload">保存图片</van-button>
      </div>
    </div>
  </van-popup>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  show: { type: Boolean, default: false },
  title: { type: String, default: '分享' },
  link: { type: String, default: '' },
  dataUrl: { type: String, default: '' },
})
const emit = defineEmits(['update:show', 'copy', 'download'])

const visible = computed({
  get: () => props.show,
  set: (v) => emit('update:show', v),
})

function handleCopy() {
  emit('copy', props.link)
}

function handleDownload() {
  emit('download', props.dataUrl)
}
</script>