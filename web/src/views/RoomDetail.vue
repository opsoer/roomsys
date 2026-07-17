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

    <RoomHero :room="room" :cover-image="coverImage" :is-admin="isAdmin"
      @fullscreen="showFullscreen" @delete-media="handleDeleteMedia"
      @edit="openEditDialog" @delete-room="handleDeleteRoom" />

    <div class="detail-body">
      <div class="detail-main">
        <RoomMedia :room="room" :gallery-images="galleryImages" :videos="videos" :is-admin="isAdmin"
          @fullscreen="showFullscreen" @delete-media="handleDeleteMedia" />
      </div>

      <RoomSidebar :room="room" :landlords="landlords" :current-contract="currentContract" :is-admin="isAdmin"
        @renew="openRenewDialog" @rent="openRentDialog" @vacant="handleVacant"
        @reserve="openReserveDialog" @confirm-sign="openConfirmSignDialog" @cancel-reserve="handleCancelReserve"
        @upload-success="fetchRoom" />
    </div>

    <RoomDialogs ref="dialogsRef" :room-id="route.params.id" :current-contract="currentContract"
      :edit-init-data="editForm" @save-success="fetchRoom" />
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { buildingGetRoom, buildingDeleteRoom, buildingDeleteMedia, getBuildingInfo } from '../api'
import { ElMessage, ElMessageBox } from 'element-plus'
import { showImagePreview } from 'vant'
import RoomHero from '../components/room/RoomHero.vue'
import RoomMedia from '../components/room/RoomMedia.vue'
import RoomSidebar from '../components/room/RoomSidebar.vue'
import RoomDialogs from '../components/room/RoomDialogs.vue'
import { mediaUrl } from '../utils/format'

const route = useRoute()
const router = useRouter()
const room = ref(null)
const coverImage = ref(null)
const galleryImages = ref([])
const videos = ref([])
const currentContract = ref(null)
const landlords = ref([])
const loading = ref(true)
const dialogsRef = ref(null)

const isAdmin = computed(() => {
  const role = localStorage.getItem('role')
  return role === 'admin' || role === 'building_admin' || role === 'super_admin'
})

const editForm = ref({})

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

async function fetchRoom() {
  loading.value = true
  try {
    const res = await buildingGetRoom(route.params.id)
    room.value = res.data.room
    const media = res.data.room.media || []
    coverImage.value = media.find(m => m.type === 'image' && m.category === 'cover') || null
    galleryImages.value = media.filter(m => m.type === 'image' && m.category !== 'cover')
    if (coverImage.value) {
      galleryImages.value = [coverImage.value, ...galleryImages.value]
    }
    videos.value = media.filter(m => m.type === 'video')
    currentContract.value = res.data.room.current_contract || null
    editForm.value = {
      room_number: room.value.room_number,
      floor: room.value.floor,
      layout: room.value.layout,
      description: room.value.description,
      rent_price: room.value.rent_price ?? null,
      deposit_months: room.value.deposit_months ?? null,
      management_fee: room.value.management_fee ?? null,
      electricity_unit_price: room.value.electricity_unit_price ?? null,
      water_unit_price: room.value.water_unit_price ?? null,
    }
  } catch {
    ElMessage.error('获取房间信息失败')
  } finally {
    loading.value = false
  }
}

async function handleDeleteMedia(mediaId) {
  try {
    await ElMessageBox.confirm('确认删除该文件？', '提示')
    await buildingDeleteMedia(route.params.id, mediaId)
    ElMessage.success('已删除')
    await fetchRoom()
  } catch (err) {
    if (err !== 'cancel') ElMessage.error('删除文件失败')
  }
}

async function handleDeleteRoom() {
  try {
    await ElMessageBox.confirm('确认删除该房间及其所有媒体文件？此操作不可恢复。', '删除房间', { confirmButtonText: '确认删除', cancelButtonText: '取消', type: 'warning' })
    await buildingDeleteRoom(route.params.id)
    ElMessage.success('房间已删除')
    goBack()
  } catch (err) {
    if (err !== 'cancel') ElMessage.error('删除房间失败')
  }
}

function openRentDialog() {
  dialogsRef.value?.openRent()
}

function openRenewDialog() {
  dialogsRef.value?.openRenew()
}

function handleVacant() {
  dialogsRef.value?.openVacant()
}

function openReserveDialog() {
  dialogsRef.value?.openReserve()
}

function openConfirmSignDialog() {
  dialogsRef.value?.openConfirmSign()
}

function handleCancelReserve() {
  dialogsRef.value?.openCancelReserve()
}

function openEditDialog() {
  dialogsRef.value?.openEdit()
}

async function fetchBuildingInfo() {
  try {
    const res = await getBuildingInfo()
    landlords.value = res.data.landlords || []
  } catch (err) {
    console.error('获取公寓信息失败', err)
  }
}

onMounted(() => {
  fetchRoom().catch(() => {})
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

.detail-body {
  display: grid;
  grid-template-columns: 1fr 320px;
  gap: 28px;
  align-items: start;
}
.detail-main { min-width: 0; }

@media (max-width: 768px) {
  .room-detail-page { padding: 0 12px 24px; }
  .detail-body { grid-template-columns: 1fr; }
}
</style>
