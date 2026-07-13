<template>
  <view v-if="loading" class="loading-wrap"><text>加载中...</text></view>
  <view v-else-if="room" class="page-room-detail">
    <view class="hero-section">
      <image v-if="coverImage" :src="mediaUrl(coverImage.file_path)" mode="aspectFill" class="hero-cover" />
      <view class="hero-overlay" />
      <view class="hero-content">
        <text :class="['status-badge', 'badge-' + room.status]">{{ statusLabel(room.status) }}</text>
        <text class="hero-title">{{ room.room_number }}</text>
        <text class="hero-meta">{{ room.floor }}层 · {{ room.layout }}</text>
        <view class="hero-actions">
          <button class="hero-btn" @click="openEditDialog">编辑</button>
        </view>
      </view>
    </view>

    <!-- Current Contract -->
    <view v-if="currentContract" class="info-card">
      <text class="info-title">当前租约</text>
      <view class="info-row"><text class="il">租客</text><text class="iv">{{ currentContract.tenant?.name || '-' }}</text></view>
      <view class="info-row"><text class="il">电话</text><text class="iv">{{ currentContract.tenant?.phone || '-' }}</text></view>
      <view class="info-row"><text class="il">起租</text><text class="iv">{{ currentContract.start_date }}</text></view>
      <view class="info-row"><text class="il">到期</text><text class="iv">{{ currentContract.end_date || '未设置' }}</text></view>
      <view class="info-row"><text class="il">月租金</text><text class="iv gold">¥{{ currentContract.rent_price?.toFixed(2) }}</text></view>
      <view class="info-row"><text class="il">押金</text><text class="iv warn">¥{{ currentContract.deposit?.toFixed(2) }}</text></view>
    </view>

    <!-- Status Actions -->
    <view class="action-card">
      <button v-if="room.status === 'vacant'" class="action-btn success" @click="openRentDialog">设为已出租</button>
      <button v-if="room.status === 'rented' || room.status === 'expiring'" class="action-btn warning" @click="openVacantDialog">设为未出租</button>
      <button v-if="room.status === 'rented' || room.status === 'expiring'" class="action-btn primary" @click="openRenewDialog">修改退租时间</button>
    </view>

    <!-- Description & Media -->
    <view v-if="room.description" class="info-card">
      <text class="info-title">房间介绍</text>
      <text class="desc-text">{{ room.description }}</text>
    </view>

    <view v-if="galleryImages.length" class="info-card">
      <text class="info-title">照片 ({{ galleryImages.length }})</text>
      <view class="gallery-grid">
        <view v-for="img in galleryImages" :key="img.id" class="gallery-item">
          <image :src="mediaUrl(img.file_path)" mode="aspectFill" @click="previewImage(img.file_path)" />
          <button class="media-del" @click.stop="handleDeleteMedia(img.id)">×</button>
        </view>
      </view>
    </view>

    <view v-if="videos.length" class="info-card">
      <text class="info-title">视频 ({{ videos.length }})</text>
      <view v-for="v in videos" :key="v.id" class="video-wrap">
        <video :src="mediaUrl(v.file_path)" controls />
        <button class="media-del video-del" @click.stop="handleDeleteMedia(v.id)">×</button>
      </view>
    </view>

    <!-- Dialogs (rent, renew, vacant, edit) -->
    <view v-if="showRentDialog" class="overlay" @click="showRentDialog = false">
      <scroll-view scroll-y class="dialog-panel" @click.stop>
        <text class="dialog-title">设为已出租</text>
        <view class="form-group"><text class="form-label">租客姓名</text><input class="form-input" v-model="rentForm.tenant_name" /></view>
        <view class="form-group"><text class="form-label">联系电话</text><input class="form-input" v-model="rentForm.tenant_phone" /></view>
        <view class="form-group"><text class="form-label">月租金</text><input class="form-input" v-model="rentForm.rent_price" type="digit" /></view>
        <view class="form-group"><text class="form-label">押金</text><input class="form-input" v-model="rentForm.deposit" type="digit" /></view>
        <view class="form-group"><text class="form-label">起租</text><picker mode="date" @change="e => rentForm.start_date = e.detail.value"><view class="picker-val">{{ rentForm.start_date || '选择日期' }}</view></picker></view>
        <view class="form-group"><text class="form-label">到期</text><picker mode="date" @change="e => rentForm.end_date = e.detail.value"><view class="picker-val">{{ rentForm.end_date || '选择日期' }}</view></picker></view>
        <view class="dialog-actions">
          <button class="dialog-btn cancel" @click="showRentDialog = false">取消</button>
          <button class="dialog-btn confirm" :disabled="rentSubmitting" @click="handleRent">{{ rentSubmitting ? '提交中...' : '确定出租' }}</button>
        </view>
      </scroll-view>
    </view>

    <view v-if="showVacantDialog" class="overlay" @click="showVacantDialog = false">
      <view class="small-dialog" @click.stop>
        <text class="dialog-title">设为未出租</text>
        <text v-if="currentContract?.deposit" class="deposit-info">原押金：¥{{ currentContract.deposit.toFixed(2) }}</text>
        <view class="form-group"><text class="form-label">退还押金</text><input class="form-input" v-model="vacateForm.refunded_deposit" type="digit" /></view>
        <view class="dialog-actions">
          <button class="dialog-btn cancel" @click="showVacantDialog = false">取消</button>
          <button class="dialog-btn confirm" :disabled="vacateSubmitting" @click="handleVacant">{{ vacateSubmitting ? '提交中...' : '确定退租' }}</button>
        </view>
      </view>
    </view>

    <view v-if="showRenewDialog" class="overlay" @click="showRenewDialog = false">
      <view class="small-dialog" @click.stop>
        <text class="dialog-title">修改退租时间</text>
        <view class="form-group"><text class="form-label">新退租日期</text><picker mode="date" @change="e => renewForm.end_date = e.detail.value"><view class="picker-val">{{ renewForm.end_date || '选择日期' }}</view></picker></view>
        <view class="form-group"><text class="form-label">退租租金</text><input class="form-input" v-model="renewForm.rent_price" type="digit" /></view>
        <view class="dialog-actions">
          <button class="dialog-btn cancel" @click="showRenewDialog = false">取消</button>
          <button class="dialog-btn confirm" :disabled="renewSubmitting" @click="handleRenew">{{ renewSubmitting ? '提交中...' : '确定' }}</button>
        </view>
      </view>
    </view>

    <!-- Media Upload -->
    <view class="info-card">
      <text class="info-title">上传媒体</text>
      <view class="upload-actions">
        <button class="upload-btn" @click="uploadImage">📷 上传图片</button>
        <button class="upload-btn" @click="uploadVideo">🎬 上传视频</button>
      </view>
      <text v-if="uploadProgress > 0 && uploadProgress < 100" class="upload-progress">上传中 {{ uploadProgress }}%</text>
    </view>

    <view v-if="showEditDialog" class="overlay" @click="showEditDialog = false">
      <scroll-view scroll-y class="dialog-panel" @click.stop>
        <text class="dialog-title">编辑房间</text>
        <view class="form-group"><text class="form-label">房间号</text><input class="form-input" v-model="editForm.room_number" /></view>
        <view class="form-group"><text class="form-label">楼层</text><picker mode="selector" :range="FLOOR_OPTIONS.map(f => f+'层')" @change="e => editForm.floor = String(FLOOR_OPTIONS[e.detail.value])"><view class="picker-val">{{ editForm.floor ? editForm.floor+'层' : '选择' }}</view></picker></view>
        <view class="form-group"><text class="form-label">户型</text><picker mode="selector" :range="LAYOUT_OPTIONS" @change="e => editForm.layout = LAYOUT_OPTIONS[e.detail.value]"><view class="picker-val">{{ editForm.layout || '选择' }}</view></picker></view>
        <view class="form-group"><text class="form-label">月租金</text><input class="form-input" v-model="editForm.rent_price" type="digit" /></view>
        <view class="form-group"><text class="form-label">管理费</text><input class="form-input" v-model="editForm.management_fee" type="digit" /></view>
        <view class="form-group"><text class="form-label">电费</text><input class="form-input" v-model="editForm.electricity_unit_price" type="digit" /></view>
        <view class="form-group"><text class="form-label">水费</text><input class="form-input" v-model="editForm.water_unit_price" type="digit" /></view>
        <view class="dialog-actions">
          <button class="dialog-btn cancel" @click="showEditDialog = false">取消</button>
          <button class="dialog-btn confirm" :disabled="editSubmitting" @click="handleEdit">{{ editSubmitting ? '提交中...' : '确定' }}</button>
        </view>
      </scroll-view>
    </view>
  </view>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { buildingGetRoom, buildingUpdateRoomStatus, buildingRenewContract, buildingUpdateRoom, buildingDeleteRoom, buildingUploadMedia, buildingDeleteMedia } from '../../api'
import { FLOOR_OPTIONS, LAYOUT_OPTIONS } from '../../utils/constants'
import { mediaUrl, statusLabel } from '../../utils/format'

const roomId = ref('')
const room = ref(null)
const coverImage = ref(null)
const galleryImages = ref([])
const videos = ref([])
const currentContract = ref(null)
const loading = ref(true)

// Dialogs
const showRentDialog = ref(false)
const rentSubmitting = ref(false)
const rentForm = ref({ tenant_name: '', tenant_phone: '', rent_price: 0, deposit: 0, start_date: '', end_date: '' })

const showVacantDialog = ref(false)
const vacateSubmitting = ref(false)
const vacateForm = ref({ refunded_deposit: 0 })

const showRenewDialog = ref(false)
const renewSubmitting = ref(false)
const renewForm = ref({ end_date: '', rent_price: 0 })

const showEditDialog = ref(false)
const editSubmitting = ref(false)
const editForm = ref({})
const uploadProgress = ref(0)

async function uploadImage() {
  try {
    const res = await uni.chooseImage({ count: 10, sizeType: ['compressed'] })
    for (const path of res.tempFilePaths) {
      const compressRes = await uni.compressImage({ src: path, quality: 65 })
      uploadProgress.value = 0
      await buildingUploadMedia(roomId.value, compressRes.tempFilePath, { category: 'gallery' }, (e) => {
        uploadProgress.value = e.progress
      })
    }
    uploadProgress.value = 0
    uni.showToast({ title: '上传成功', icon: 'success' })
    await fetchRoom()
  } catch {
    uploadProgress.value = 0
    uni.showToast({ title: '上传失败', icon: 'none' })
  }
}

async function handleDeleteMedia(mediaId) {
  uni.showModal({
    title: '确认删除',
    content: '确定删除该媒体文件？',
    success: async (res) => {
      if (!res.confirm) return
      try {
        await buildingDeleteMedia(roomId.value, mediaId)
        uni.showToast({ title: '已删除', icon: 'success' })
        await fetchRoom()
      } catch { uni.showToast({ title: '删除失败', icon: 'none' }) }
    }
  })
}

async function uploadVideo() {
  try {
    const res = await uni.chooseVideo({ sourceType: ['album', 'camera'], maxDuration: 60 })
    uploadProgress.value = 0
    await buildingUploadMedia(roomId.value, res.tempFilePath, { category: 'video' }, (e) => {
      uploadProgress.value = e.progress
    })
    uploadProgress.value = 0
    uni.showToast({ title: '上传成功', icon: 'success' })
    await fetchRoom()
  } catch {
    uploadProgress.value = 0
    uni.showToast({ title: '上传失败', icon: 'none' })
  }
}

function previewImage(path) {
  uni.previewImage({ urls: [mediaUrl(path)] })
}

async function fetchRoom() {
  loading.value = true
  try {
    const res = await buildingGetRoom(roomId.value)
    room.value = res.data.room
    const media = res.data.room.media || []
    coverImage.value = media.find(m => m.type === 'image' && m.category === 'cover') || null
    galleryImages.value = media.filter(m => m.type === 'image' && m.category !== 'cover')
    if (coverImage.value) galleryImages.value = [coverImage.value, ...galleryImages.value]
    videos.value = media.filter(m => m.type === 'video')
    currentContract.value = res.data.room.current_contract || null
    editForm.value = {
      room_number: room.value.room_number, floor: room.value.floor, layout: room.value.layout,
      rent_price: room.value.rent_price ?? null, deposit_months: room.value.deposit_months ?? null,
      management_fee: room.value.management_fee ?? null,
      electricity_unit_price: room.value.electricity_unit_price ?? null,
      water_unit_price: room.value.water_unit_price ?? null,
    }
  } catch {
    uni.showToast({ title: '获取房间信息失败', icon: 'none' })
  } finally {
    loading.value = false
  }
}

function openRentDialog() { rentForm.value = { tenant_name: '', tenant_phone: '', rent_price: 0, deposit: 0, start_date: '', end_date: '' }; showRentDialog.value = true }
function openVacantDialog() { vacateForm.value.refunded_deposit = currentContract.value?.deposit || 0; showVacantDialog.value = true }
function openRenewDialog() { renewForm.value = { end_date: '', rent_price: currentContract.value?.rent_price || 0 }; showRenewDialog.value = true }
function openEditDialog() { showEditDialog.value = true }

async function handleRent() {
  rentSubmitting.value = true
  try {
    await buildingUpdateRoomStatus(roomId.value, { status: 'rented', ...rentForm.value })
    uni.showToast({ title: '出租成功', icon: 'success' })
    showRentDialog.value = false
    await fetchRoom()
  } catch { uni.showToast({ title: '操作失败', icon: 'none' }) }
  finally { rentSubmitting.value = false }
}

async function handleVacant() {
  vacateSubmitting.value = true
  try {
    await buildingUpdateRoomStatus(roomId.value, { status: 'vacant', refunded_deposit: vacateForm.value.refunded_deposit })
    uni.showToast({ title: '已设为未出租', icon: 'success' })
    showVacantDialog.value = false
    await fetchRoom()
  } catch { uni.showToast({ title: '操作失败', icon: 'none' }) }
  finally { vacateSubmitting.value = false }
}

async function handleRenew() {
  renewSubmitting.value = true
  try {
    await buildingRenewContract(roomId.value, { end_date: renewForm.value.end_date, rent_price: renewForm.value.rent_price })
    uni.showToast({ title: '修改成功', icon: 'success' })
    showRenewDialog.value = false
    await fetchRoom()
  } catch { uni.showToast({ title: '操作失败', icon: 'none' }) }
  finally { renewSubmitting.value = false }
}

async function handleEdit() {
  editSubmitting.value = true
  try {
    await buildingUpdateRoom(roomId.value, editForm.value)
    uni.showToast({ title: '编辑成功', icon: 'success' })
    showEditDialog.value = false
    await fetchRoom()
  } catch { uni.showToast({ title: '操作失败', icon: 'none' }) }
  finally { editSubmitting.value = false }
}

onMounted(() => {
  const pages = getCurrentPages()
  const page = pages[pages.length - 1]
  roomId.value = page.$page?.options?.id || ''
  if (roomId.value) fetchRoom()
})
</script>

<style scoped>
.page-room-detail { min-height: 100vh; padding-bottom: 30px; }
.loading-wrap { text-align: center; padding: 80px 0; color: #999; }
.hero-section { position: relative; height: 240px; overflow: hidden; }
.hero-cover { width: 100%; height: 100%; position: absolute; }
.hero-overlay { position: absolute; inset: 0; background: linear-gradient(to top, rgba(0,0,0,0.75) 0%, rgba(0,0,0,0.25) 60%, rgba(0,0,0,0.1) 100%); }
.hero-content { position: absolute; bottom: 0; left: 0; right: 0; padding: 20px; z-index: 1; }
.status-badge { font-size: 12px; padding: 4px 12px; border-radius: 20px; color: #fff; display: inline-block; }
.badge-vacant { background: #67c23a; }
.badge-rented { background: #f56c6c; }
.badge-expiring { background: #e6a23c; }
.hero-title { font-size: 28px; font-weight: 700; color: #fff; display: block; margin-top: 8px; text-shadow: 0 2px 8px rgba(0,0,0,0.3); }
.hero-meta { font-size: 14px; color: rgba(255,255,255,0.7); display: block; margin-top: 4px; }
.hero-actions { display: flex; gap: 8px; margin-top: 10px; }
.hero-btn { font-size: 13px; color: #fff; background: rgba(255,255,255,0.2); border: 1px solid rgba(255,255,255,0.3); border-radius: 20px; padding: 6px 18px; }
.info-card { background: #fff; border-radius: 12px; margin: 12px; padding: 16px; box-shadow: 0 1px 4px rgba(0,0,0,0.04); }
.info-title { font-size: 15px; font-weight: 600; color: #333; display: block; margin-bottom: 12px; }
.info-row { display: flex; justify-content: space-between; padding: 6px 0; }
.il { font-size: 14px; color: #999; }
.iv { font-size: 14px; color: #333; font-weight: 500; }
.iv.gold { color: #e6a23c; font-weight: 600; }
.iv.warn { color: #f56c6c; font-weight: 600; }
.action-card { margin: 12px; display: flex; flex-direction: column; gap: 8px; }
.action-btn { padding: 12px; border: none; border-radius: 8px; font-size: 15px; color: #fff; font-weight: 600; }
.action-btn.success { background: #67c23a; }
.action-btn.warning { background: #e6a23c; }
.action-btn.primary { background: #409eff; }
.desc-text { font-size: 14px; color: #555; line-height: 1.8; display: block; }
.gallery-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
.gallery-item { border-radius: 8px; overflow: hidden; height: 120px; }
.gallery-item image { width: 100%; height: 100%; }
.video-wrap { border-radius: 8px; overflow: hidden; margin-bottom: 8px; }
.video-wrap video { width: 100%; max-height: 200px; }
.overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); z-index: 1000; display: flex; align-items: flex-end; }
.dialog-panel { width: 100%; background: #fff; border-radius: 16px 16px 0 0; padding: 20px; max-height: 80vh; }
.small-dialog { width: 100%; background: #fff; border-radius: 16px 16px 0 0; padding: 20px; }
.dialog-title { font-size: 18px; font-weight: 700; display: block; margin-bottom: 16px; }
.form-group { margin-bottom: 12px; }
.form-label { font-size: 14px; color: #333; display: block; margin-bottom: 4px; }
.form-input { width: 100%; height: 40px; border: 1px solid #dcdfe6; border-radius: 8px; padding: 0 12px; font-size: 14px; background: #fff; }
.picker-val { height: 40px; line-height: 40px; border: 1px solid #dcdfe6; border-radius: 8px; padding: 0 12px; font-size: 14px; color: #333; background: #fff; }
.deposit-info { display: block; background: #f5f7fa; padding: 10px; border-radius: 8px; margin-bottom: 16px; font-size: 14px; color: #e6a23c; }
.dialog-actions { display: flex; gap: 12px; margin-top: 20px; }
.dialog-btn { flex: 1; height: 44px; border-radius: 22px; display: flex; align-items: center; justify-content: center; font-size: 15px; }
.dialog-btn.cancel { background: #f5f5f5; color: #666; border: none; }
.dialog-btn.confirm { background: #1989fa; color: #fff; border: none; }
.dialog-btn.confirm[disabled] { opacity: 0.6; }
.upload-actions { display: flex; gap: 10px; }
.upload-btn { flex: 1; padding: 10px; border: 1px dashed #1989fa; border-radius: 8px; background: #f0f7ff; color: #1989fa; font-size: 14px; text-align: center; }
.upload-progress { display: block; text-align: center; color: #1989fa; font-size: 13px; margin-top: 8px; }
.media-del { position: absolute; top: 4px; right: 4px; width: 22px; height: 22px; border-radius: 50%; background: rgba(0,0,0,0.5); color: #fff; border: none; font-size: 14px; display: flex; align-items: center; justify-content: center; padding: 0; line-height: 1; }
.video-del { top: 8px; right: 8px; }
.gallery-item { position: relative; }
</style>
