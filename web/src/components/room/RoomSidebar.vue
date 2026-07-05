<template>
  <div class="detail-sidebar">
    <div v-if="landlords.length" class="sidebar-card">
      <h4 class="sidebar-title">联系房东</h4>
      <div v-for="l in landlords" :key="l.id" class="sidebar-row">
        <span class="sidebar-label">{{ l.name }}</span>
        <a :href="'tel:' + l.phone" class="sidebar-phone">{{ l.phone }}</a>
      </div>
    </div>

    <div v-if="currentContract && isAdmin" class="sidebar-card">
      <div class="sidebar-card-header">
        <h4 class="sidebar-title" style="margin:0">当前租约</h4>
        <el-button v-if="room.status === 'rented' || room.status === 'expiring'" size="small" text type="primary" @click="$emit('renew')">
          修改退租时间
        </el-button>
      </div>
      <div class="sidebar-row">
        <span class="sidebar-label">租客</span>
        <span class="sidebar-val">{{ currentContract.tenant?.name || '-' }}</span>
      </div>
      <div class="sidebar-row">
        <span class="sidebar-label">电话</span>
        <span class="sidebar-val">{{ currentContract.tenant?.phone || '-' }}</span>
      </div>
      <div class="sidebar-row">
        <span class="sidebar-label">起租</span>
        <span class="sidebar-val">{{ currentContract.start_date }}</span>
      </div>
      <div class="sidebar-row">
        <span class="sidebar-label">到期</span>
        <span class="sidebar-val">{{ currentContract.end_date || '未设置' }}</span>
      </div>
      <div class="sidebar-row">
        <span class="sidebar-label">月租金</span>
        <span class="sidebar-val price-primary">{{ currentContract.rent_price?.toFixed(2) }} 元</span>
      </div>
      <div class="sidebar-row">
        <span class="sidebar-label">押金</span>
        <span class="sidebar-val price-warn">{{ currentContract.deposit?.toFixed(2) }} 元</span>
      </div>
    </div>

    <div v-if="isAdmin" class="sidebar-card">
      <h4 class="sidebar-title">状态操作</h4>
      <div class="sidebar-actions">
        <el-button v-if="room.status === 'vacant'" type="success" @click="$emit('rent')" style="width:100%">
          设为已出租
        </el-button>
        <el-button v-if="room.status === 'rented' || room.status === 'expiring' || room.status === 'expired'" type="warning" @click="$emit('vacant')" style="width:100%">
          设为未出租
        </el-button>
      </div>
    </div>

    <div v-if="isAdmin" class="sidebar-card">
      <h4 class="sidebar-title">上传媒体</h4>
      <el-progress v-if="uploading" :percentage="uploadProgress" :stroke-width="6" style="margin-bottom:12px" />
      <div class="upload-actions">
        <el-upload
          :http-request="customUpload"
          :data="{ category: 'cover', roomId: room.id }"
          :on-success="handleUploadSuccess"
          :on-error="handleUploadError"
          :before-upload="beforeUploadImage"
          :show-file-list="false"
          accept="image/jpeg,image/png,image/gif"
        >
          <el-button type="warning" :icon="Plus" style="width:100%">上传封面</el-button>
        </el-upload>
        <el-upload
          :http-request="customUpload"
          :data="{ category: 'gallery', roomId: room.id }"
          :on-success="handleUploadSuccess"
          :on-error="handleUploadError"
          :before-upload="beforeUploadImage"
          :show-file-list="false"
          accept="image/jpeg,image/png,image/gif"
          multiple
        >
          <el-button type="primary" :icon="Picture" style="width:100%">上传照片</el-button>
        </el-upload>
        <el-upload
          :http-request="customUpload"
          :data="{ category: 'video', roomId: room.id }"
          :on-success="handleUploadSuccess"
          :on-error="handleUploadError"
          :before-upload="beforeUploadVideo"
          :show-file-list="false"
          accept="video/mp4,video/quicktime"
        >
          <el-button type="success" :icon="VideoCamera" style="width:100%">上传视频</el-button>
        </el-upload>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { buildingUploadMedia } from '../../api'

defineProps({
  room: { type: Object, required: true },
  landlords: { type: Array, default: () => [] },
  currentContract: { type: Object, default: null },
  isAdmin: { type: Boolean, default: false },
})

const emit = defineEmits(['renew', 'rent', 'vacant', 'upload-success'])

const uploading = ref(false)
const uploadProgress = ref(0)

function compressImage(file, maxWidth = 1920, quality = 0.8) {
  return new Promise((resolve, reject) => {
    if (!file.type.startsWith('image/')) {
      resolve(file)
      return
    }
    const img = new Image()
    const url = URL.createObjectURL(file)
    img.onload = () => {
      URL.revokeObjectURL(url)
      let { width, height } = img
      if (width <= maxWidth && height <= maxWidth) {
        resolve(file)
        return
      }
      const canvas = document.createElement('canvas')
      if (width > maxWidth) {
        height = Math.round((maxWidth / width) * height)
        width = maxWidth
      }
      if (height > maxWidth) {
        width = Math.round((maxWidth / height) * width)
        height = maxWidth
      }
      canvas.width = width
      canvas.height = height
      const ctx = canvas.getContext('2d')
      ctx.drawImage(img, 0, 0, width, height)
      canvas.toBlob(blob => {
        const compressed = new File([blob], file.name.replace(/\.[^.]+$/, '.jpg'), { type: 'image/jpeg' })
        resolve(compressed)
      }, 'image/jpeg', quality)
    }
    img.onerror = () => resolve(file)
    img.src = url
  })
}

async function customUpload(options) {
  uploading.value = true
  uploadProgress.value = 0
  try {
    const compressed = await compressImage(options.file)
    const formData = new FormData()
    formData.append('file', compressed)
    for (const key in options.data) {
      formData.append(key, options.data[key])
    }
    const res = await buildingUploadMedia(options.data.roomId || '', formData, (e) => {
      uploadProgress.value = Math.round((e.loaded / e.total) * 100)
    })
    options.onSuccess(res.data, options.file, options.fileList)
  } catch (err) {
    options.onError(err, options.file, options.fileList)
  } finally {
    uploading.value = false
  }
}

function handleUploadError() {
  ElMessage.error('上传失败')
}

function handleUploadSuccess() {
  ElMessage.success('上传成功')
  emit('upload-success')
}

function beforeUploadImage(file) {
  if (!file.type.startsWith('image/')) {
    ElMessage.error('仅支持图片格式')
    return false
  }
  if (file.size > 10 * 1024 * 1024) {
    ElMessage.error('图片最大 10MB')
    return false
  }
  return true
}

function beforeUploadVideo(file) {
  if (!file.type.startsWith('video/')) {
    ElMessage.error('仅支持视频格式')
    return false
  }
  if (file.size > 200 * 1024 * 1024) {
    ElMessage.error('视频最大 200MB')
    return false
  }
  return true
}
</script>

<style scoped>
.detail-sidebar { display: flex; flex-direction: column; gap: 16px; }
.sidebar-card {
  background: #fff;
  border-radius: 12px;
  padding: 20px;
  box-shadow: 0 1px 4px rgba(0,0,0,0.04);
  border: 1px solid #f0f0f0;
}
.sidebar-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}
.sidebar-title {
  font-size: 15px;
  font-weight: 600;
  margin: 0 0 14px;
  color: #1a1a2e;
}
.sidebar-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 0;
}
.sidebar-label { color: #999; font-size: 14px; }
.sidebar-val { font-weight: 500; color: #333; font-size: 14px; }
.sidebar-phone {
  font-weight: 600;
  color: #e6a23c;
  text-decoration: none;
  font-size: 16px;
}
.sidebar-phone:hover { color: #d4880f; }
.price-primary { color: #67c23a; font-weight: 600; }
.price-warn { color: #e6a23c; font-weight: 600; }
.sidebar-actions { display: flex; flex-direction: column; gap: 8px; }
.upload-actions { display: flex; flex-direction: column; gap: 8px; }

@media (max-width: 768px) {
  .sidebar-card { padding: 16px; }
}
</style>
