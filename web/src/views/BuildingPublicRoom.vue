<template>
  <div class="page-room" v-if="room">
    <van-nav-bar
      :title="room.room_number"
      left-arrow
      @click-left="$router.push(`/building/${buildingId}`)"
    >
      <template #right>
        <van-icon name="manager" size="20" @click="goToDashboard" />
      </template>
    </van-nav-bar>

    <!-- 图片轮播 -->
    <van-swipe v-if="allImages.length" class="room-swipe" :autoplay="3000" indicator-color="white">
      <van-swipe-item v-for="(img, i) in allImages" :key="i">
        <van-image
          :src="mediaUrl(img)"
          fit="cover"
          class="swipe-img"
          @click="previewImage(i)"
          @error="e => e.target.style.display='none'"
        />
      </van-swipe-item>
    </van-swipe>

    <div v-else class="room-img-placeholder">
      <van-icon name="photo-o" size="64" color="#ccc" />
    </div>

    <!-- 房间信息 -->
    <div class="room-section">
      <div class="room-header">
        <div>
          <h1 class="room-number">{{ room.room_number }}</h1>
          <p class="room-meta">
            <template v-if="room.floor">{{ room.floor }}层</template>
            <template v-if="room.floor && room.layout"> · </template>
            <template v-if="room.layout">{{ room.layout }}</template>
          </p>
        </div>
        <van-tag :type="statusTagType(room.status)" size="large" round>{{ statusLabel(room.status) }}</van-tag>
      </div>
    </div>

    <!-- 房东信息 -->
    <div v-if="building?.landlords?.length" class="room-section landlord-section">
      <div class="section-title">
        <van-icon name="phone-o" /> 房东信息
      </div>
      <div v-for="l in building.landlords" :key="l.id" class="landlord-item">
        <van-icon name="contact" size="16" color="#e6a23c" />
        <span class="landlord-name">{{ l.name }}</span>
        <a :href="'tel:' + l.phone" class="landlord-phone">{{ l.phone }}</a>
      </div>
    </div>

    <!-- 价格（如有租约） -->
    <div v-if="contract" class="room-price-card">
      <div class="price-item">
        <span class="price-label">月租金</span>
        <span class="price-value">¥{{ contract.rent_price }}</span>
      </div>
      <div class="price-divider"></div>
      <div class="price-item">
        <span class="price-label">押金</span>
        <span class="price-value sub">¥{{ contract.deposit }}</span>
      </div>
    </div>

    <!-- 房间描述 -->
    <div v-if="room.description" class="room-section">
      <div class="section-title">
        <van-icon name="description-o" /> 房间介绍
      </div>
      <p class="desc-text">{{ room.description }}</p>
    </div>

    <!-- 视频 -->
    <div v-if="videos.length" class="room-section">
      <div class="section-title">
        <van-icon name="video-o" /> 视频
      </div>
      <div class="video-grid">
        <div v-for="(v, i) in videos" :key="i" class="video-wrap">
          <video :src="mediaUrl(v)" controls class="video-player" preload="metadata"></video>
        </div>
      </div>
    </div>

    <!-- 租约信息 -->
    <div v-if="contract" class="room-section">
      <div class="section-title">
        <van-icon name="notes-o" /> 当前租约
      </div>
      <div class="contract-grid">
        <div class="contract-row">
          <span class="contract-label"><van-icon name="contact" /> 租客</span>
          <span class="contract-val">{{ contract.tenant?.name }}</span>
        </div>
        <div class="contract-row">
          <span class="contract-label"><van-icon name="phone-o" /> 电话</span>
          <span class="contract-val">{{ contract.tenant?.phone }}</span>
        </div>
        <div class="contract-row">
          <span class="contract-label"><van-icon name="clock-o" /> 起租</span>
          <span class="contract-val">{{ contract.start_date }}</span>
        </div>
        <div class="contract-row">
          <span class="contract-label"><van-icon name="clock-o" /> 到期</span>
          <span class="contract-val">{{ contract.end_date }}</span>
        </div>
      </div>
    </div>

    <div class="page-footer">
      <p>© 2026 圳好租 · 深圳公寓租赁管理平台</p>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showToast, showImagePreview } from 'vant'
import { getPublicRoom, getBuildingDetail } from '../api'

const route = useRoute()
const router = useRouter()
const buildingId = computed(() => route.params.bid)
const roomId = computed(() => route.params.id)
const room = ref(null)
const contract = ref(null)
const building = ref(null)
const allImages = ref([])
const videos = ref([])

function goToDashboard() {
  const token = localStorage.getItem('token')
  const role = localStorage.getItem('role')
  if (token) {
    router.push(role === 'super_admin' ? '/admin/buildings' : '/landlord/rooms')
  } else {
    router.push('/login')
  }
}

function mediaUrl(path) {
  if (!path) return ''
  return `/api/media/${path}`
}

function previewImage(index) {
  showImagePreview({
    images: allImages.value.map(img => mediaUrl(img)),
    startPosition: index,
    closeable: true,
  })
}

function statusTagType(s) {
  return s === 'vacant' ? 'success' : s === 'rented' ? 'danger' : 'warning'
}

function statusLabel(s) {
  return s === 'vacant' ? '未出租' : s === 'rented' ? '已出租' : '即将退租'
}

onMounted(async () => {
  try {
    const res = await getPublicRoom(buildingId.value, roomId.value)
    room.value = res.data.room
    contract.value = res.data.room.current_contract || null
    const media = res.data.room.media || []
    const images = []
    const vids = []
    for (const m of media) {
      if (m.type === 'image' && m.file_path) images.push(m.file_path)
      else if (m.type === 'video' && m.file_path) vids.push(m.file_path)
    }
    allImages.value = images
    videos.value = vids
  } catch {
    showToast('加载失败')
  }
  try {
    const res = await getBuildingDetail(buildingId.value)
    building.value = res.data.building
  } catch {}
})
</script>

<style scoped>
.page-room {
  min-height: 100vh;
  background: #f5f6fa;
  padding-bottom: 20px;
}
.room-swipe {
  height: 320px;
}
.swipe-img {
  width: 100%;
  height: 320px;
}
.room-img-placeholder {
  height: 200px;
  background: #e9ecef;
  display: flex;
  align-items: center;
  justify-content: center;
}
.room-section {
  background: #fff;
  border-radius: 12px;
  margin: 0 12px 12px;
  padding: 18px 16px;
  box-shadow: 0 1px 4px rgba(0,0,0,0.04);
}
.room-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
}
.room-number {
  font-size: 24px;
  font-weight: 700;
  color: #1a1a2e;
  margin-bottom: 4px;
}
.room-meta {
  font-size: 13px;
  color: #999;
}
.section-title {
  font-size: 15px;
  font-weight: 600;
  color: #333;
  margin-bottom: 12px;
  display: flex;
  align-items: center;
  gap: 6px;
}
.desc-text {
  font-size: 14px;
  color: #555;
  line-height: 1.8;
}
.room-price-card {
  background: linear-gradient(135deg, #1a1a2e, #16213e);
  border-radius: 12px;
  margin: 0 12px 12px;
  padding: 20px;
  display: flex;
  align-items: center;
  justify-content: space-around;
  color: #fff;
}
.price-item {
  text-align: center;
}
.price-label {
  font-size: 12px;
  color: rgba(255,255,255,0.6);
  display: block;
  margin-bottom: 4px;
}
.price-value {
  font-size: 24px;
  font-weight: 700;
  color: #e6a23c;
}
.price-value.sub {
  color: rgba(255,255,255,0.8);
  font-size: 20px;
}
.price-divider {
  width: 1px;
  height: 36px;
  background: rgba(255,255,255,0.15);
}
.video-grid {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.video-wrap {
  border-radius: 10px;
  overflow: hidden;
  background: #000;
}
.video-player {
  width: 100%;
  max-height: 240px;
  display: block;
}
.contract-grid {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.contract-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 0;
}
.contract-label {
  font-size: 14px;
  color: #888;
  display: flex;
  align-items: center;
  gap: 4px;
}
.contract-val {
  font-size: 14px;
  color: #333;
  font-weight: 500;
}
.landlord-section {
  border-left: 3px solid #e6a23c;
}
.landlord-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
}
.landlord-name {
  font-size: 14px;
  font-weight: 500;
  color: #333;
  min-width: 48px;
}
.landlord-phone {
  font-size: 14px;
  color: #e6a23c;
  text-decoration: none;
  font-weight: 500;
}
.page-footer {
  text-align: center;
  padding: 24px 16px;
  color: #aaa;
  font-size: 12px;
}
</style>