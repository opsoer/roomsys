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
          @error="e => { if (e.target.tagName === 'IMG') e.target.src = '/default-image.svg' }"
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

    <!-- 价格信息 -->
    <div v-if="room.rent_price || room.deposit_months != null || room.management_fee || room.electricity_unit_price || room.water_unit_price" class="room-price-card">
      <div v-if="room.rent_price" class="price-item">
        <span class="price-label">月租金</span>
        <span class="price-value">¥{{ room.rent_price }}</span>
      </div>
      <div v-if="room.deposit_months != null" class="price-item">
        <span class="price-label">押金规则</span>
        <span class="price-value sub">{{ ['无押金', '押一', '押二', '押三'][room.deposit_months] }}</span>
      </div>
      <div v-if="room.management_fee" class="price-item">
        <span class="price-label">管理费</span>
        <span class="price-value sub">¥{{ room.management_fee }}/月</span>
      </div>
      <div v-if="room.electricity_unit_price" class="price-item">
        <span class="price-label">电费单价</span>
        <span class="price-value sub">¥{{ room.electricity_unit_price }}/度</span>
      </div>
      <div v-if="room.water_unit_price" class="price-item">
        <span class="price-label">水费单价</span>
        <span class="price-value sub">¥{{ room.water_unit_price }}/吨</span>
      </div>
    </div>

    <!-- 房东信息 -->
    <div v-if="building?.landlords?.length" class="room-section landlord-section">
      <div class="section-title">
        <van-icon name="phone-o" /> 房东信息
      </div>
      <div v-for="l in building.landlords" :key="l.id" class="landlord-item">
        <div class="landlord-avatar">
          <van-icon name="contact" size="20" color="#fff" />
        </div>
        <div class="landlord-info">
          <span class="landlord-name">{{ maskName(l.name) }}</span>
          <span class="landlord-phone-masked">{{ maskPhone(l.phone) }}</span>
        </div>
        <van-button size="small" type="primary" round @click="showContact(l)">获取联系方式</van-button>
      </div>
    </div>

    <van-dialog v-model:show="contactVisible" close-on-click-overlay>
      <div class="contact-dialog">
        <div class="contact-avatar">
          <van-icon name="contact" size="32" color="#fff" />
        </div>
        <div class="contact-name">{{ contactLandlord.name }}</div>
        <div class="contact-phone">{{ contactLandlord.phone }}</div>
        <van-button type="primary" block round @click="copyPhone">
          <van-icon name="records" style="margin-right:4px" /> 复制电话号码
        </van-button>
      </div>
    </van-dialog>

    <!-- 到期信息（公开显示） -->
    <div v-if="room.end_date" class="room-section">
      <div class="section-title">
        <van-icon name="clock-o" /> 退租信息
      </div>
      <div class="contract-grid">
        <div class="contract-row">
          <span class="contract-label"><van-icon name="clock-o" /> 预计退租</span>
          <span class="contract-val" style="color:#e6a23c;font-weight:600;">{{ room.end_date }}</span>
        </div>
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
import { mediaUrl, statusLabel, statusTagType } from '../utils/format'
import { useAuthStore } from '../stores/auth'

const route = useRoute()
const router = useRouter()
const buildingId = computed(() => route.params.bid)
const roomId = computed(() => route.params.id)
const room = ref(null)
const contract = ref(null)
const building = ref(null)
const allImages = ref([])
const videos = ref([])

const contactVisible = ref(false)
const contactLandlord = ref({ name: '', phone: '' })

function maskName(name) {
  if (!name) return ''
  return name.charAt(0) + '*'.repeat(name.length - 1)
}

function maskPhone(phone) {
  if (!phone) return ''
  if (phone.length <= 3) return phone
  if (phone.length <= 7) return phone.slice(0, 3) + '*'.repeat(phone.length - 3)
  return phone.slice(0, 3) + '*'.repeat(phone.length - 7) + phone.slice(-4)
}

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
  showToast({ message: '已复制到剪贴板', duration: 1500 })
}

function goToDashboard() {
  const authStore = useAuthStore()
  const token = authStore.token
  const role = authStore.role
  if (token) {
    router.push(role === 'super_admin' ? '/admin/buildings' : '/landlord/rooms')
  } else {
    router.push('/login')
  }
}

function previewImage(index) {
  showImagePreview({
    images: allImages.value.map(img => mediaUrl(img)),
    startPosition: index,
    closeable: true,
  })
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
  } catch { showToast('加载房东信息失败') }
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
  flex-wrap: wrap;
  gap: 8px;
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
  gap: 12px;
  padding: 10px 0;
  border-bottom: 1px solid #f5f5f5;
}
.landlord-item:last-child {
  border-bottom: none;
}
.landlord-avatar {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: linear-gradient(135deg, #e6a23c, #f0a020);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.landlord-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.landlord-name {
  font-size: 15px;
  font-weight: 600;
  color: #333;
}
.landlord-phone-masked {
  font-size: 12px;
  color: #999;
  letter-spacing: 1px;
}
.contact-dialog {
  text-align: center;
  padding: 24px 20px 16px;
}
.contact-avatar {
  width: 56px;
  height: 56px;
  border-radius: 50%;
  background: linear-gradient(135deg, #e6a23c, #f0a020);
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0 auto 16px;
}
.contact-name {
  font-size: 18px;
  font-weight: 700;
  color: #1a1a2e;
  margin-bottom: 6px;
}
.contact-phone {
  font-size: 24px;
  font-weight: 700;
  color: #333;
  letter-spacing: 2px;
  margin-bottom: 24px;
}
.page-footer {
  text-align: center;
  padding: 24px 16px;
  color: #aaa;
  font-size: 12px;
}
</style>