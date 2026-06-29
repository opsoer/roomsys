<template>
  <div v-if="loading" class="loading-wrap">
    <el-icon class="is-loading" :size="36"><Loading /></el-icon>
    <p>加载中...</p>
  </div>
  <div v-else-if="room" class="room-detail-page">
    <div class="detail-back">
      <el-button text @click="goBack">
        <el-icon><ArrowLeft /></el-icon> 返回
      </el-button>
    </div>

    <!-- 房间头图 -->
    <div class="detail-hero" :class="{ 'has-cover': coverImage }">
      <div v-if="coverImage" class="detail-cover">
        <img :src="mediaUrl(coverImage.file_path)" class="cover-img" @click="showFullscreen(mediaUrl(coverImage.file_path))" />
        <el-button v-if="isAdmin" size="small" type="danger" circle class="cover-delete-btn"
          @click.stop="handleDeleteMedia(coverImage.id)">
          <el-icon><Close /></el-icon>
        </el-button>
        <div class="cover-overlay"></div>
      </div>
      <div class="detail-hero-content">
        <div class="hero-tags">
          <el-tag :type="statusTag(room.status)" size="large" effect="dark" class="status-badge">{{ statusLabel(room.status) }}</el-tag>
          <el-tag v-if="room.end_date && (room.status === 'rented' || room.status === 'expiring' || room.status === 'expired')" type="info" effect="plain" class="end-date-badge">
            退租：{{ room.end_date }}
          </el-tag>
        </div>
        <h1 class="detail-title">{{ room.room_number }}</h1>
        <p class="detail-floor">
          <template v-if="room.floor">{{ room.floor }}层</template>
          <template v-if="room.floor && room.layout"> · </template>
          <template v-if="room.layout">{{ room.layout }}</template>
        </p>
        <div v-if="isAdmin" class="hero-actions">
          <el-button type="primary" size="small" @click="showEditDialog = true">编辑</el-button>
          <el-button type="danger" size="small" @click="handleDeleteRoom">删除</el-button>
        </div>
      </div>
    </div>

    <div class="detail-body">
      <div class="detail-main">
        <!-- 描述 -->
        <div v-if="room.description" class="detail-card">
          <h3 class="card-title">房间介绍</h3>
          <p class="card-text">{{ room.description }}</p>
        </div>

        <!-- 照片 -->
        <div v-if="galleryImages.length > 0" class="detail-card">
          <h3 class="card-title">
            照片
            <span class="card-count">{{ galleryImages.length }}张</span>
          </h3>
          <div class="gallery-grid">
            <div v-for="img in galleryImages" :key="img.id" class="gallery-item" :class="{ 'admin-mode': isAdmin }">
              <img :src="mediaUrl(img.file_path)" class="gallery-img" @click="showFullscreen(mediaUrl(img.file_path))" />
              <el-button v-if="isAdmin" size="small" type="danger" circle class="delete-btn"
                @click.stop="handleDeleteMedia(img.id)">
                <el-icon><Close /></el-icon>
              </el-button>
            </div>
          </div>
        </div>

        <!-- 视频 -->
        <div v-if="videos.length > 0" class="detail-card">
          <h3 class="card-title">
            视频
            <span class="card-count">{{ videos.length }}个</span>
          </h3>
          <div class="video-list">
            <div v-for="v in videos" :key="v.id" class="video-item" :class="{ 'admin-mode': isAdmin }">
              <video :src="mediaUrl(v.file_path)" controls class="video-player" preload="metadata"></video>
              <el-button v-if="isAdmin" size="small" type="danger" circle class="delete-btn"
                @click.stop="handleDeleteMedia(v.id)">
                <el-icon><Close /></el-icon>
              </el-button>
            </div>
          </div>
        </div>

        <!-- 空状态 -->
        <div v-if="!room.description && galleryImages.length === 0 && videos.length === 0" class="detail-card empty-section">
          <el-empty description="暂无房间介绍" :image-size="80" />
        </div>
      </div>

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
            <el-button v-if="room.status === 'rented' || room.status === 'expiring'" size="small" text type="primary" @click="openRenewDialog">
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
            <span class="sidebar-label">押金</span>
            <span class="sidebar-val price-warn">{{ currentContract.deposit?.toFixed(2) }} 元</span>
          </div>
        </div>

        <div v-if="isAdmin" class="sidebar-card">
          <h4 class="sidebar-title">状态操作</h4>
          <div class="sidebar-actions">
            <el-button v-if="room.status === 'vacant'" type="success" @click="showRentDialog = true" style="width:100%">
              设为已出租
            </el-button>
            <el-button v-if="room.status === 'rented' || room.status === 'expiring' || room.status === 'expired'" type="warning" @click="handleVacant" style="width:100%">
              设为未出租
            </el-button>
          </div>
        </div>

        <div v-if="isAdmin" class="sidebar-card">
          <h4 class="sidebar-title">上传媒体</h4>
          <div class="upload-actions">
            <el-upload
              :action="`/api/building/rooms/${room.id}/media`"
              :headers="{ Authorization: `Bearer ${token}` }"
              :data="{ category: 'cover' }"
              :on-success="handleUploadSuccess"
              :before-upload="beforeUploadImage"
              :show-file-list="false"
              accept="image/jpeg,image/png,image/gif"
            >
              <el-button type="warning" :icon="Plus" style="width:100%">上传封面</el-button>
            </el-upload>
            <el-upload
              :action="`/api/building/rooms/${room.id}/media`"
              :headers="{ Authorization: `Bearer ${token}` }"
              :data="{ category: 'gallery' }"
              :on-success="handleUploadSuccess"
              :before-upload="beforeUploadImage"
              :show-file-list="false"
              accept="image/jpeg,image/png,image/gif"
              multiple
            >
              <el-button type="primary" :icon="Picture" style="width:100%">上传照片</el-button>
            </el-upload>
            <el-upload
              :action="`/api/building/rooms/${room.id}/media`"
              :headers="{ Authorization: `Bearer ${token}` }"
              :data="{ category: 'video' }"
              :on-success="handleUploadSuccess"
              :before-upload="beforeUploadVideo"
              :show-file-list="false"
              accept="video/mp4,video/quicktime"
            >
              <el-button type="success" :icon="VideoCamera" style="width:100%">上传视频</el-button>
            </el-upload>
          </div>
        </div>
      </div>
    </div>

    <!-- Dialogs (unchanged) -->
    <el-dialog v-model="showRentDialog" title="设为已出租" width="500px">
      <el-form ref="rentFormRef" :model="rentForm" label-width="100px">
        <el-form-item label="租客姓名" prop="tenant_name" :rules="[{ required: true, message: '请输入租客姓名' }]">
          <el-input v-model="rentForm.tenant_name" />
        </el-form-item>
        <el-form-item label="联系电话" prop="tenant_phone">
          <el-input v-model="rentForm.tenant_phone" />
        </el-form-item>
        <el-form-item label="月租金" prop="rent_price" :rules="[{ required: true, message: '请输入租金' }]">
          <el-input-number v-model="rentForm.rent_price" :min="0" :precision="2" style="width:100%" />
        </el-form-item>
        <el-form-item label="押金" prop="deposit" :rules="[{ required: true, message: '请输入押金金额' }]">
          <el-input-number v-model="rentForm.deposit" :min="0" :precision="2" style="width:100%" />
        </el-form-item>
        <el-form-item label="起租日期" prop="start_date" :rules="[{ required: true, message: '请选择起租日期' }]">
          <el-date-picker v-model="rentForm.start_date" type="date" format="YYYY-MM-DD" value-format="YYYY-MM-DD" style="width:100%" />
        </el-form-item>
        <el-form-item label="结束日期" prop="end_date" :rules="[{ required: true, message: '请选择结束日期' }]">
          <el-date-picker v-model="rentForm.end_date" type="date" format="YYYY-MM-DD" value-format="YYYY-MM-DD" style="width:100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showRentDialog = false">取消</el-button>
        <el-button type="primary" :loading="rentSubmitting" @click="handleRent">确定出租</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showEndDateDialog" title="续租" width="420px">
      <el-form ref="endDateFormRef" :model="endDateForm" label-width="90px">
        <el-form-item label="续租租金" prop="rent_price" :rules="[{ required: true, message: '请输入续租租金' }]">
          <el-input-number v-model="endDateForm.rent_price" :min="0" :precision="2" style="width:100%" />
        </el-form-item>
        <el-form-item label="续租结束日" prop="end_date" :rules="[{ required: true, message: '请选择续租结束日' }]">
          <el-date-picker v-model="endDateForm.end_date" type="date" format="YYYY-MM-DD" value-format="YYYY-MM-DD" placeholder="选择续租结束日" style="width:100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showEndDateDialog = false">取消</el-button>
        <el-button type="primary" :loading="endDateSubmitting" @click="handleUpdateEndDate">确定续租</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showVacateDialog" title="设为未出租" width="450px">
      <div>
        <div v-if="currentContract?.deposit" class="vacate-deposit-info">
          <div class="vacate-row">
            <span>原押金</span>
            <span class="vacate-amount">{{ currentContract.deposit.toFixed(2) }} 元</span>
          </div>
          <p class="vacate-hint">如因卫生或家具损坏需扣除部分押金，请填写实际退还金额</p>
        </div>
        <el-form ref="vacateFormRef" :model="vacateForm" label-width="100px">
          <el-form-item label="退还押金" prop="refunded_deposit">
            <el-input-number v-model="vacateForm.refunded_deposit" :min="0" :precision="2" style="width:100%" />
          </el-form-item>
        </el-form>
        <div v-if="vacateDeduction > 0" class="vacate-deduction-note">
          将自动创建 <strong>{{ vacateDeduction.toFixed(2) }}</strong> 元的押金收入账单
        </div>
      </div>
      <template #footer>
        <el-button @click="showVacateDialog = false">取消</el-button>
        <el-button type="primary" :loading="vacateSubmitting" @click="confirmVacate">确定退租</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showEditDialog" title="编辑房间" width="500px">
      <el-form ref="editFormRef" :model="editForm" label-width="90px">
        <el-form-item label="房间号" prop="room_number">
          <el-input v-model="editForm.room_number" />
        </el-form-item>
        <el-form-item label="楼层" prop="floor">
          <el-select v-model="editForm.floor" placeholder="选择楼层" style="width:100%">
            <el-option label="1层" value="1" />
            <el-option label="2层" value="2" />
            <el-option label="3层" value="3" />
          </el-select>
        </el-form-item>
        <el-form-item label="户型" prop="layout">
          <el-select v-model="editForm.layout" placeholder="选择户型" style="width:100%">
            <el-option label="单间" value="单间" />
            <el-option label="大单间" value="大单间" />
            <el-option label="一室一厅" value="一室一厅" />
          </el-select>
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input v-model="editForm.description" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showEditDialog = false">取消</el-button>
        <el-button type="primary" :loading="editSubmitting" @click="handleEdit">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Loading, Plus, Picture, VideoCamera, Close } from '@element-plus/icons-vue'
import { buildingGetRoom, buildingUpdateRoom, buildingUpdateRoomStatus, buildingRenewContract, buildingDeleteRoom, buildingDeleteMedia, getBuildingInfo } from '../api'
import { ElMessage, ElMessageBox } from 'element-plus'
import { showImagePreview } from 'vant'

const route = useRoute()
const router = useRouter()
const room = ref(null)
const coverImage = ref(null)
const galleryImages = ref([])
const videos = ref([])
const currentContract = ref(null)
const landlords = ref([])
const loading = ref(true)
const token = localStorage.getItem('token')
const isAdmin = computed(() => {
  const role = localStorage.getItem('role')
  return role === 'admin' || role === 'building_admin' || role === 'super_admin'
})

const showRentDialog = ref(false)
const rentSubmitting = ref(false)
const rentForm = ref({ tenant_name: '', tenant_phone: '', rent_price: 0, deposit: 0, start_date: '', end_date: '' })
const rentFormRef = ref(null)

const showEndDateDialog = ref(false)
const endDateSubmitting = ref(false)
const endDateForm = ref({ end_date: '', rent_price: 0 })
const endDateFormRef = ref(null)

const showVacateDialog = ref(false)
const vacateSubmitting = ref(false)
const vacateForm = ref({ refunded_deposit: 0 })
const vacateFormRef = ref(null)

const vacateDeduction = computed(() => {
  const deposit = currentContract.value?.deposit || 0
  const refunded = vacateForm.value.refunded_deposit
  return Math.max(0, deposit - refunded)
})

const showEditDialog = ref(false)
const editSubmitting = ref(false)
const editForm = ref({})
const editFormRef = ref(null)

function showFullscreen(url) {
  const allImages = []
  if (coverImage.value) allImages.push(mediaUrl(coverImage.value.file_path))
  for (const img of galleryImages.value) {
    const u = mediaUrl(img.file_path)
    if (!allImages.includes(u)) allImages.push(u)
  }
  const startIndex = allImages.indexOf(url)
  showImagePreview({
    images: allImages,
    startPosition: Math.max(0, startIndex),
    closeable: true,
  })
}

function goBack() {
  router.push('/landlord/rooms')
}

function mediaUrl(path) {
  if (!path) return ''
  return `/api/media/${path}`
}

function statusTag(s) {
  return s === 'vacant' ? 'success' : s === 'rented' ? 'danger' : s === 'expiring' ? 'warning' : 'info'
}

function statusLabel(s) {
  return s === 'vacant' ? '未出租' : s === 'rented' ? '已出租' : s === 'expiring' ? '即将退租' : '待处理退租'
}

async function fetchRoom() {
  loading.value = true
  try {
    const res = await buildingGetRoom(route.params.id)
    room.value = res.data.room
    const media = res.data.room.media || []
    coverImage.value = media.find(m => m.type === 'image' && m.category === 'cover') || null
    galleryImages.value = media.filter(m => m.type === 'image' && m.category !== 'cover')
    if (coverImage.value) {
      galleryImages.value = [coverImage.value, ...galleryImages.value.filter(m => m.id !== coverImage.value.id)]
    }
    videos.value = media.filter(m => m.type === 'video')
    currentContract.value = res.data.room.current_contract || null
    editForm.value = {
      room_number: room.value.room_number,
      floor: room.value.floor,
      layout: room.value.layout,
      description: room.value.description,
    }
  } finally {
    loading.value = false
  }
}

function handleUploadSuccess() {
  ElMessage.success('上传成功')
  fetchRoom()
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

async function handleRent() {
  const valid = await rentFormRef.value.validate().catch(() => false)
  if (!valid) return
  if (rentForm.value.start_date && rentForm.value.end_date && rentForm.value.end_date <= rentForm.value.start_date) {
    ElMessage.error('退租日期必须大于起租日期')
    return
  }
  rentSubmitting.value = true
  try {
    await buildingUpdateRoomStatus(route.params.id, { status: 'rented', ...rentForm.value })
    ElMessage.success('出租成功')
    showRentDialog.value = false
    rentForm.value = { tenant_name: '', tenant_phone: '', rent_price: 0, deposit: 0, start_date: '', end_date: '' }
    await fetchRoom()
  } finally {
    rentSubmitting.value = false
  }
}

async function handleVacant() {
  const deposit = currentContract.value?.deposit || 0
  vacateForm.value.refunded_deposit = deposit
  showVacateDialog.value = true
}

async function confirmVacate() {
  vacateSubmitting.value = true
  try {
    await buildingUpdateRoomStatus(route.params.id, { status: 'vacant', refunded_deposit: vacateForm.value.refunded_deposit })
    ElMessage.success('已设为未出租')
    showVacateDialog.value = false
    await fetchRoom()
  } finally {
    vacateSubmitting.value = false
  }
}

function openRenewDialog() {
  endDateForm.value = { end_date: '', rent_price: currentContract.value?.rent_price || 0 }
  showEndDateDialog.value = true
}

async function handleUpdateEndDate() {
  const valid = await endDateFormRef.value.validate().catch(() => false)
  if (!valid) return
  endDateSubmitting.value = true
  try {
    await buildingRenewContract(route.params.id, { end_date: endDateForm.value.end_date, rent_price: endDateForm.value.rent_price })
    ElMessage.success('续租成功')
    showEndDateDialog.value = false
    endDateForm.value = { end_date: '', rent_price: 0 }
    await fetchRoom()
  } finally {
    endDateSubmitting.value = false
  }
}

async function handleDeleteRoom() {
  try {
    await ElMessageBox.confirm('确认删除该房间及其所有媒体文件？此操作不可恢复。', '删除房间', { confirmButtonText: '确认删除', cancelButtonText: '取消', type: 'warning' })
    await buildingDeleteRoom(route.params.id)
    ElMessage.success('房间已删除')
    goBack()
  } catch {}
}

async function handleDeleteMedia(mediaId) {
  try {
    await ElMessageBox.confirm('确认删除该文件？', '提示')
    await buildingDeleteMedia(route.params.id, mediaId)
    ElMessage.success('已删除')
    await fetchRoom()
  } catch {}
}

async function handleEdit() {
  const valid = await editFormRef.value.validate().catch(() => false)
  if (!valid) return
  editSubmitting.value = true
  try {
    await buildingUpdateRoom(route.params.id, editForm.value)
    ElMessage.success('编辑成功')
    showEditDialog.value = false
    await fetchRoom()
  } finally {
    editSubmitting.value = false
  }
}

async function fetchBuildingInfo() {
  try {
    const res = await getBuildingInfo()
    landlords.value = res.data.landlords || []
  } catch {}
}

onMounted(() => {
  fetchRoom()
  fetchBuildingInfo()
})
</script>

<style scoped>
.loading-wrap {
  text-align: center;
  padding: 80px 0;
  color: #999;
}
.room-detail-page {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 24px 40px;
}
.detail-back {
  margin-bottom: 12px;
}
.detail-back .el-button { font-size: 14px; }

/* Hero */
.detail-hero {
  position: relative;
  min-height: 260px;
  border-radius: 16px;
  overflow: hidden;
  margin-bottom: 28px;
  background: linear-gradient(135deg, #1a1a2e, #16213e);
  display: flex;
  align-items: flex-end;
}
.detail-hero.has-cover { min-height: 340px; }
.detail-cover { position: absolute; inset: 0; }
.cover-img { width: 100%; height: 100%; object-fit: cover; }
.cover-overlay {
  position: absolute;
  inset: 0;
  background: linear-gradient(to top, rgba(0,0,0,0.75) 0%, rgba(0,0,0,0.25) 60%, rgba(0,0,0,0.1) 100%);
}
.detail-hero-content {
  position: relative;
  z-index: 1;
  padding: 32px;
  width: 100%;
}
.hero-tags { display: flex; gap: 8px; margin-bottom: 12px; }
.status-badge { font-size: 14px; letter-spacing: 0.5px; }
.end-date-badge {
  font-size: 13px;
  background: rgba(255,255,255,0.12) !important;
  color: rgba(255,255,255,0.8) !important;
  border: 1px solid rgba(255,255,255,0.15) !important;
}
.detail-title {
  font-size: 36px;
  font-weight: 700;
  color: #fff;
  margin: 0 0 6px;
  text-shadow: 0 2px 8px rgba(0,0,0,0.3);
}
.detail-floor { font-size: 15px; color: rgba(255,255,255,0.7); margin: 0 0 12px; }
.hero-actions { display: flex; gap: 8px; }

/* Body */
.detail-body {
  display: grid;
  grid-template-columns: 1fr 320px;
  gap: 28px;
  align-items: start;
}
.detail-main { min-width: 0; }

/* Cards */
.detail-card {
  background: #fff;
  border-radius: 12px;
  padding: 24px;
  margin-bottom: 20px;
  box-shadow: 0 1px 4px rgba(0,0,0,0.04);
  border: 1px solid #f0f0f0;
}
.card-title {
  font-size: 17px;
  font-weight: 600;
  margin: 0 0 16px;
  color: #1a1a2e;
  padding-bottom: 10px;
  border-bottom: 2px solid #f5f5f5;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.card-count {
  font-size: 13px;
  font-weight: 400;
  color: #999;
}
.card-text {
  font-size: 15px;
  line-height: 1.8;
  color: #555;
  margin: 0;
  white-space: pre-wrap;
}
.empty-section { text-align: center; padding: 40px; }

/* Gallery */
.gallery-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 12px;
}
.gallery-item { position: relative; border-radius: 10px; overflow: hidden; }
.gallery-img {
  width: 100%;
  height: 180px;
  object-fit: cover;
  cursor: pointer;
  display: block;
  transition: transform 0.3s;
}
.gallery-img:hover { transform: scale(1.05); }

/* Delete buttons */
.gallery-item .delete-btn,
.video-item .delete-btn,
.cover-delete-btn {
  position: absolute;
  top: 6px;
  right: 6px;
  width: 24px;
  height: 24px;
  min-height: auto;
  padding: 0;
  opacity: 0;
  transition: opacity 0.2s;
  z-index: 2;
}
.gallery-item.admin-mode:hover .delete-btn,
.video-item.admin-mode:hover .delete-btn,
.detail-cover:hover .cover-delete-btn { opacity: 1; }

/* Video */
.video-list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 12px;
}
.video-item { position: relative; border-radius: 10px; overflow: hidden; background: #000; }
.video-player {
  width: 100%;
  max-height: 240px;
  display: block;
}

/* Sidebar */
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

/* Vacate dialog */
.vacate-deposit-info {
  background: #f5f7fa;
  padding: 12px;
  border-radius: 8px;
  margin-bottom: 16px;
}
.vacate-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.vacate-amount {
  font-weight: 600;
  color: #e6a23c;
  font-size: 16px;
}
.vacate-hint {
  font-size: 13px;
  color: #999;
  margin: 8px 0 0;
}
.vacate-deduction-note {
  background: #fef0f0;
  padding: 10px 12px;
  border-radius: 6px;
  font-size: 13px;
  color: #f56c6c;
}

@media (max-width: 768px) {
  .room-detail-page { padding: 0 12px 24px; }
  .detail-hero { min-height: 200px; border-radius: 12px; margin-bottom: 16px; }
  .detail-hero.has-cover { min-height: 240px; }
  .detail-hero-content { padding: 20px; }
  .detail-title { font-size: 24px; }
  .detail-body { grid-template-columns: 1fr; }
  .detail-card { padding: 16px; }
  .gallery-grid { grid-template-columns: repeat(2, 1fr); gap: 8px; }
  .gallery-img { height: 130px; }
  .video-list { grid-template-columns: 1fr; gap: 8px; }
  .video-player { max-height: 200px; }
  .sidebar-card { padding: 16px; }
  .hero-actions { flex-wrap: wrap; }
  .hero-actions .el-button { flex: 1; font-size: 12px; }
}
</style>