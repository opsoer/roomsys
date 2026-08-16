<template>
  <div class="detail-sidebar">
    <div v-if="landlords.length" class="sidebar-card">
      <h4 class="sidebar-title">联系房东</h4>
      <div v-for="l in landlords" :key="l.id" class="sidebar-row">
        <span class="sidebar-label">{{ l.name }}</span>
        <el-button text type="primary" size="small" @click="showContact(l)">获取联系方式</el-button>
      </div>
    </div>

    <el-dialog v-model="contactVisible" title="房东联系方式" width="360px" align-center>
      <div style="text-align:center;padding:12px 0">
        <div style="font-size:16px;font-weight:600;margin-bottom:16px">{{ contactLandlord.name }}</div>
        <div style="font-size:22px;font-weight:700;color:#333;letter-spacing:2px;margin-bottom:16px">{{ contactLandlord.phone }}</div>
        <el-button type="primary" @click="copyPhone">复制电话号码</el-button>
      </div>
      <template #footer>
        <el-button @click="contactVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <div v-if="currentContract && isAdmin" class="sidebar-card">
      <div class="sidebar-card-header">
        <h4 class="sidebar-title" style="margin:0">{{ room.status === 'reserved' ? '预订信息（已交定金）' : '当前租约' }}</h4>
        <el-button v-if="room.status === 'rented' || room.status === 'expiring'" size="small" text type="primary" @click="$emit('renew')">
          续租
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
        <span class="sidebar-label">管理费</span>
        <span class="sidebar-val">{{ mgmtFeeLabel(currentContract.management_fee) }}</span>
      </div>
      <div class="sidebar-row">
        <span class="sidebar-label">押金</span>
        <span class="sidebar-val price-warn">{{ currentContract.deposit?.toFixed(2) }} 元</span>
      </div>
      <div v-if="room.status === 'reserved'" class="sidebar-row">
        <span class="sidebar-label">定金</span>
        <span class="sidebar-val price-warn">{{ currentContract.earnest_money?.toFixed(2) }} 元</span>
      </div>
    </div>

    <div v-if="futureReservation && isAdmin && room.status !== 'reserved'" class="sidebar-card" style="border-color: #409eff">
      <div class="sidebar-card-header">
        <h4 class="sidebar-title" style="margin:0;color:#409eff">下一租客 · 已交定金预订</h4>
      </div>
      <div class="sidebar-row">
        <span class="sidebar-label">租客</span>
        <span class="sidebar-val">{{ futureReservation.tenant?.name || '-' }}</span>
      </div>
      <div class="sidebar-row">
        <span class="sidebar-label">电话</span>
        <span class="sidebar-val">{{ futureReservation.tenant?.phone || '-' }}</span>
      </div>
      <div class="sidebar-row">
        <span class="sidebar-label">预计起租</span>
        <span class="sidebar-val">{{ futureReservation.start_date }}</span>
      </div>
      <div class="sidebar-row">
        <span class="sidebar-label">预计结束</span>
        <span class="sidebar-val">{{ futureReservation.end_date || '未设置' }}</span>
      </div>
      <div class="sidebar-row">
        <span class="sidebar-label">月租金</span>
        <span class="sidebar-val price-primary">{{ futureReservation.rent_price?.toFixed(2) }} 元</span>
      </div>
      <div class="sidebar-row">
        <span class="sidebar-label">管理费</span>
        <span class="sidebar-val">{{ mgmtFeeLabel(futureReservation.management_fee) }}</span>
      </div>
      <div class="sidebar-row">
        <span class="sidebar-label">押金</span>
        <span class="sidebar-val price-warn">{{ futureReservation.deposit?.toFixed(2) }} 元</span>
      </div>
      <div class="sidebar-row">
        <span class="sidebar-label">定金</span>
        <span class="sidebar-val price-warn">{{ futureReservation.earnest_money?.toFixed(2) }} 元</span>
      </div>
    </div>

    <div v-if="isAdmin" class="sidebar-card">
      <h4 class="sidebar-title">状态操作</h4>
      <div class="sidebar-actions">
        <el-button v-if="room.status === 'vacant'" type="success" @click="$emit('rent')">
          签合同出租
        </el-button>
        <el-button v-if="room.status === 'vacant'" type="primary" plain @click="$emit('reserve')">
          交定金预订
        </el-button>
        <el-button v-if="room.status === 'reserved'" type="success" @click="$emit('confirm-sign')">
          确认签约（收齐押租金）
        </el-button>
        <el-button v-if="room.status === 'reserved'" type="warning" plain @click="$emit('cancel-reserve')">
          取消预订（退/扣定金）
        </el-button>
        <el-button v-if="(room.status === 'rented' || room.status === 'expiring') && !futureReservation" type="primary" plain @click="$emit('reserve')">
          交定金预订（未来）
        </el-button>
        <el-button v-if="room.status === 'rented' || room.status === 'expiring' || room.status === 'expired'" type="warning" @click="$emit('vacant')">
          办理退租
        </el-button>
      </div>
    </div>

    <div v-if="isAdmin" class="sidebar-card">
      <h4 class="sidebar-title">上传媒体</h4>
      <div v-if="uploading" style="margin-bottom:12px">
        <el-progress :percentage="uploadProgress" :stroke-width="6" />
      </div>
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
          :disabled="imageCount >= 10"
          multiple
        >
          <el-button type="primary" :icon="Picture" style="width:100%" :disabled="imageCount >= 10">
            上传照片{{ imageCount >= 10 ? '（已满）' : `（${imageCount}/10）` }}
          </el-button>
        </el-upload>
        <el-upload
          :http-request="customUpload"
          :data="{ category: 'video', roomId: room.id }"
          :on-success="handleUploadSuccess"
          :on-error="handleUploadError"
          :before-upload="beforeUploadVideo"
          :show-file-list="false"
          accept="video/mp4,video/quicktime"
          :disabled="videoCount >= 2"
        >
          <el-button type="success" :icon="VideoCamera" style="width:100%" :disabled="videoCount >= 2">
            上传视频{{ videoCount >= 2 ? '（已满）' : `（${videoCount}/2）` }}
          </el-button>
        </el-upload>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus, Picture, VideoCamera } from '@element-plus/icons-vue'
import { buildingUploadMedia, getUploadToken, confirmMediaUpload } from '../../api'

const props = defineProps({
  room: { type: Object, required: true },
  landlords: { type: Array, default: () => [] },
  currentContract: { type: Object, default: null },
  futureReservation: { type: Object, default: null },
  isAdmin: { type: Boolean, default: false },
})

function mgmtFeeLabel(fee) {
  if (fee !== null && Number(fee) > 0) return Number(fee).toFixed(2) + ' 元/月'
  return '无管理费'
}

const emit = defineEmits(['renew', 'rent', 'vacant', 'reserve', 'confirm-sign', 'cancel-reserve', 'upload-success'])

const imageCount = computed(() => {
  const media = props.room?.media || []
  return media.filter(m => m.type === 'image').length
})
const videoCount = computed(() => {
  const media = props.room?.media || []
  return media.filter(m => m.type === 'video').length
})

const uploading = ref(false)
const uploadProgress = ref(0)

const contactVisible = ref(false)
const contactLandlord = ref({ name: '', phone: '' })

function showContact(landlord) {
  contactLandlord.value = { name: landlord.name, phone: landlord.phone }
  contactVisible.value = true
}

function copyPhone() {
  if (navigator.clipboard?.writeText) {
    navigator.clipboard.writeText(contactLandlord.value.phone)
  } else {
    const ta = document.createElement('textarea')
    ta.value = contactLandlord.value.phone
    ta.style.position = 'fixed'
    ta.style.opacity = '0'
    document.body.appendChild(ta)
    ta.select()
    document.execCommand('copy')
    document.body.removeChild(ta)
  }
  ElMessage.success('已复制到剪贴板')
  contactVisible.value = false
}

function compressImage(file, maxWidth = 1600, quality = 0.65) {
  return new Promise((resolve, reject) => {
    if (!file.type.startsWith('image/')) { resolve(file); return }
    const img = new Image()
    const url = URL.createObjectURL(file)
    img.onload = () => {
      URL.revokeObjectURL(url)
      let { width, height } = img
      if (width <= maxWidth && height <= maxWidth) { resolve(file); return }
      const canvas = document.createElement('canvas')
      if (width > maxWidth) { height = Math.round((maxWidth / width) * height); width = maxWidth }
      if (height > maxWidth) { width = Math.round((maxWidth / height) * width); height = maxWidth }
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
  if (uploading.value) {
    ElMessage.warning('正在上传中，请等待当前任务完成')
    return
  }
  uploading.value = true
  uploadProgress.value = 0
  try {
    if (options.file.type.startsWith('video/')) {
      await directUploadVideo(options)
      return
    }
    const file = await compressImage(options.file)
    const formData = new FormData()
    formData.append('file', file)
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

// 视频走七牛直传：浏览器直接上传原件到七牛，不占服务器带宽，上传后有真实进度
async function directUploadVideo(options) {
  const file = options.file
  const ext = (file.name.match(/\.(\w+)$/) || [])[1] || 'mp4'
  const tokRes = await getUploadToken({
    room_id: options.data.roomId,
    category: 'video',
    ext,
    file_size: file.size,
  })
  const { token, key, upload_url } = tokRes.data
  await uploadDirect(upload_url, token, key, file, (p) => {
    uploadProgress.value = p
  })
  const res = await confirmMediaUpload(options.data.roomId, {
    key,
    type: 'video',
    category: 'video',
    file_name: file.name,
    file_size: file.size,
  })
  options.onSuccess(res.data, options.file, options.fileList)
  ElMessage.success('视频上传成功，正在后台转码压缩，完成后自动更新')
}

function uploadDirect(url, token, key, file, onProgress) {
  return new Promise((resolve, reject) => {
    const fd = new FormData()
    fd.append('token', token)
    fd.append('key', key)
    fd.append('file', file)
    const xhr = new XMLHttpRequest()
    xhr.open('POST', url)
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) onProgress(Math.round((e.loaded / e.total) * 100))
    }
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) resolve(xhr.responseText)
      else reject(new Error(`上传失败(${xhr.status})`))
    }
    xhr.onerror = () => reject(new Error('网络错误'))
    xhr.send(fd)
  })
}

function handleUploadError() {
  ElMessage.error('上传失败')
}

function handleUploadSuccess() {
  ElMessage.success('上传成功')
  emit('upload-success')
}

function beforeUploadImage(file) {
  if (!file.type.startsWith('image/')) { ElMessage.error('仅支持图片格式'); return false }
  if (file.size > 10 * 1024 * 1024) { ElMessage.error('图片最大 10MB'); return false }
  if (imageCount.value >= 10) { ElMessage.error('每个房间最多允许10张照片'); return false }
  return true
}

function beforeUploadVideo(file) {
  if (!file.type.startsWith('video/')) { ElMessage.error('仅支持视频格式'); return false }
  if (file.size > 200 * 1024 * 1024) { ElMessage.error('视频最大 200MB'); return false }
  if (videoCount.value >= 2) { ElMessage.error('每个房间最多允许2个视频'); return false }
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
.price-primary { color: #67c23a; font-weight: 600; }
.price-warn { color: #e6a23c; font-weight: 600; }
.sidebar-actions { display: flex; flex-direction: column; gap: 8px; }
.sidebar-actions .el-button { width: 100%; margin-left: 0 !important; }
.upload-actions { display: flex; flex-direction: column; gap: 8px; }

@media (max-width: 768px) {
  .sidebar-card { padding: 16px; }
}
</style>
