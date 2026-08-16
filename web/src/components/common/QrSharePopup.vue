<template>
  <van-popup v-model:show="visible" round position="center" :style="{ padding: '20px 16px', width: '82%', maxWidth: '360px' }">
    <div style="text-align: center;">
      <div style="font-size: 17px; font-weight: 700; color: #1a1a2e;">{{ title }}</div>
      <div style="font-size: 12px; color: #999; margin: 4px 0 12px;">扫一扫，即刻查看</div>
      <img v-if="dataUrl" :src="dataUrl" alt="二维码" style="width: 100%; border-radius: 10px;" @click="handleDownload" />
      <el-icon v-else class="is-loading" style="font-size: 36px; color: #999"><Loading /></el-icon>
      <div style="font-size: 12px; color: #999; margin-top: 10px; word-break: break-all;">{{ link }}</div>
      <input
        class="share-link-input"
        type="text"
        readonly
        :value="link"
        @focus="e => e.target.select()"
        @click="e => e.target.select()"
      />
      <div style="display: flex; gap: 10px; margin-top: 14px;">
        <van-button block round @click="handleCopy">复制链接</van-button>
        <van-button type="primary" block round @click="handleDownload">保存图片</van-button>
      </div>
      <div style="font-size: 11px; color: #aaa; margin-top: 10px;">
        提示：长按二维码图片可直接保存到相册；复制失败时可点击上方链接全选复制
      </div>
    </div>
  </van-popup>

  <!-- 移动端/微信无法自动下载，全屏展示二维码供长按保存 -->
  <van-image-preview
    v-model:show="previewVisible"
    :images="previewImages"
    :start-position="0"
    closeable
    @close="previewVisible = false"
  />
</template>

<script setup>
import { ref, computed } from 'vue'
import { showToast } from 'vant'
import { canAutoDownloadImage } from '../../utils/qr'

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

const previewVisible = ref(false)
const previewImages = ref([])

function handleCopy() {
  emit('copy', props.link)
}

async function handleDownload() {
  if (!props.dataUrl) {
    showToast('二维码尚未生成，请稍后重试')
    return
  }
  if (canAutoDownloadImage()) {
    emit('download', props.dataUrl)
  } else {
    // 手机/微信：<a download> 无法保存到相册，改用全屏预览 + 长按保存
    previewImages.value = [props.dataUrl]
    previewVisible.value = true
    showToast({ message: '长按图片可保存到相册', duration: 2000 })
  }
}
</script>

<style scoped>
.share-link-input {
  width: 100%;
  margin-top: 6px;
  padding: 8px 10px;
  font-size: 12px;
  color: #333;
  text-align: center;
  background: #f7f8fa;
  border: 1px solid #f0f0f0;
  border-radius: 6px;
  outline: none;
  -webkit-user-select: text;
  user-select: text;
}
</style>