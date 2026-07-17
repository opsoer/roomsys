<template>
  <view class="page-room" v-if="room">
    <swiper v-if="allImages.length" class="room-swipe" :indicator-dots="true" indicator-color="rgba(255,255,255,0.4)" indicator-active-color="#fff" autoplay>
      <swiper-item v-for="(img, i) in allImages" :key="i">
        <image :src="mediaUrl(img)" mode="aspectFill" class="swipe-img" @click="previewImage(i)" />
      </swiper-item>
    </swiper>
    <view v-else class="room-img-placeholder">
      <text class="ph-icon">📷</text>
    </view>

    <view class="room-section">
      <view class="room-header">
        <view>
          <text class="room-number">{{ room.room_number }}</text>
          <text class="room-meta">
            <template v-if="room.floor">{{ room.floor }}层</template>
            <template v-if="room.floor && room.layout"> · </template>
            <template v-if="room.layout">{{ room.layout }}</template>
          </text>
        </view>
        <text :class="['status-badge', 'badge-' + room.status]">{{ statusLabel(room.status) }}</text>
      </view>
    </view>

    <view v-if="room.rent_price || room.deposit_months != null || room.management_fee || room.electricity_unit_price || room.water_unit_price" class="room-price-card">
      <view class="price-item" v-if="room.rent_price">
        <text class="price-label">月租金</text>
        <text class="price-value">¥{{ room.rent_price }}</text>
      </view>
      <view class="price-item" v-if="room.deposit_months != null">
        <text class="price-label">押金规则</text>
        <text class="price-value sub">{{ ['无押金', '押一', '押二', '押三'][room.deposit_months] }}</text>
      </view>
      <view class="price-item" v-if="room.management_fee">
        <text class="price-label">管理费</text>
        <text class="price-value sub">¥{{ room.management_fee }}/月</text>
      </view>
      <view class="price-item" v-if="room.electricity_unit_price">
        <text class="price-label">电费单价</text>
        <text class="price-value sub">¥{{ room.electricity_unit_price }}/度</text>
      </view>
      <view class="price-item" v-if="room.water_unit_price">
        <text class="price-label">水费单价</text>
        <text class="price-value sub">¥{{ room.water_unit_price }}/吨</text>
      </view>
    </view>

    <view v-if="building?.landlords?.length" class="room-section landlord-section">
      <text class="section-title">📞 房东信息</text>
      <view v-for="l in building.landlords" :key="l.id" class="landlord-item">
        <view class="landlord-avatar">👤</view>
        <view class="landlord-info">
          <text class="landlord-name">{{ maskName2(l.name) }}</text>
          <text class="landlord-phone-masked">{{ maskPhone2(l.phone) }}</text>
        </view>
        <button class="contact-btn" @click="showContact(l)">获取联系方式</button>
      </view>
    </view>

    <view v-if="room.end_date && room.status !== 'vacant'" class="room-section">
      <text class="section-title">⏰ 退租信息</text>
      <view class="info-row">
        <text class="info-label">预计退租</text>
        <text class="info-val warning">{{ room.end_date }}</text>
      </view>
    </view>

    <view v-if="room.description" class="room-section">
      <text class="section-title">📝 房间介绍</text>
      <text class="desc-text">{{ room.description }}</text>
    </view>

    <view v-if="videos.length" class="room-section">
      <text class="section-title">🎬 视频</text>
      <view v-for="(v, i) in videos" :key="i" class="video-wrap">
        <video :src="mediaUrl(v)" controls class="video-player" />
      </view>
    </view>

    <view v-if="contract" class="room-section">
      <text class="section-title">📋 当前租约</text>
      <view class="info-row"><text class="info-label">起租</text><text class="info-val">{{ contract.start_date }}</text></view>
      <view class="info-row"><text class="info-label">到期</text><text class="info-val">{{ contract.end_date }}</text></view>
    </view>

    <!-- 房东联系方式弹窗 -->
    <view v-if="contactVisible" class="overlay" @click="contactVisible = false">
      <view class="contact-dialog" @click.stop>
        <view class="contact-avatar-wrap">👤</view>
        <text class="contact-name">{{ contactLandlord.name }}</text>
        <text class="contact-phone">{{ contactLandlord.phone }}</text>
        <button class="copy-btn" @click="copyPhone">复制电话号码</button>
        <button class="close-btn" @click="contactVisible = false">关闭</button>
      </view>
    </view>

    <view class="page-footer"><text>© 2026 圳好租</text></view>
  </view>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { getPublicRoom, getBuildingDetail } from '../../api'
import { mediaUrl, statusLabel } from '../../utils/format'

const buildingId = ref('')
const roomId = ref('')
const room = ref(null)
const contract = ref(null)
const building = ref(null)
const allImages = ref([])
const videos = ref([])
const contactVisible = ref(false)
const contactLandlord = ref({ name: '', phone: '' })

function maskName2(name) {
  if (!name) return ''
  return name.charAt(0) + '*'.repeat(name.length - 1)
}

function maskPhone2(phone) {
  if (!phone) return ''
  if (phone.length <= 3) return phone
  if (phone.length <= 7) return phone.slice(0, 3) + '*'.repeat(phone.length - 3)
  return phone.slice(0, 3) + '*'.repeat(phone.length - 7) + phone.slice(-4)
}

function showContact(l) {
  contactLandlord.value = { name: l.name, phone: l.phone }
  contactVisible.value = true
  // Send landlord view stats
  // uni.request({ url: '/api/stats/landlord-view', method: 'POST', data: { building_id: Number(buildingId.value) } })
}

function copyPhone() {
  uni.setClipboardData({
    data: contactLandlord.value.phone,
    success: () => uni.showToast({ title: '已复制', icon: 'success' })
  })
}

function previewImage(index) {
  uni.previewImage({
    urls: allImages.value.map(img => mediaUrl(img)),
    current: index,
  })
}

onMounted(async () => {
  const pages = getCurrentPages()
  const page = pages[pages.length - 1]
  buildingId.value = page.$page?.options?.bid || ''
  roomId.value = page.$page?.options?.id || ''
  if (!roomId.value) return
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
    uni.showToast({ title: '加载失败', icon: 'none' })
  }
  try {
    const res = await getBuildingDetail(buildingId.value)
    building.value = res.data.building
  } catch {}
})
</script>

<style scoped>
.page-room { min-height: 100vh; background: #f5f6fa; padding-bottom: 20px; }
.room-swipe { height: 320px; }
.swipe-img { width: 100%; height: 320px; }
.room-img-placeholder { height: 200px; background: #e9ecef; display: flex; align-items: center; justify-content: center; }
.ph-icon { font-size: 64px; }
.room-section { background: #fff; border-radius: 12px; margin: 0 12px 12px; padding: 18px 16px; box-shadow: 0 1px 4px rgba(0,0,0,0.04); }
.room-header { display: flex; align-items: flex-start; justify-content: space-between; }
.room-number { font-size: 24px; font-weight: 700; color: #1a1a2e; display: block; }
.room-meta { font-size: 13px; color: #999; display: block; margin-top: 4px; }
.status-badge { font-size: 12px; padding: 4px 12px; border-radius: 20px; color: #fff; font-weight: 500; }
.badge-vacant { background: #67c23a; }
.badge-reserved { background: #409eff; }
.badge-rented { background: #f56c6c; }
.badge-expiring { background: #e6a23c; }
.section-title { font-size: 15px; font-weight: 600; color: #333; display: block; margin-bottom: 12px; }
.info-row { display: flex; justify-content: space-between; padding: 6px 0; }
.info-label { font-size: 14px; color: #888; }
.info-val { font-size: 14px; color: #333; font-weight: 500; }
.info-val.warning { color: #e6a23c; font-weight: 600; }
.desc-text { font-size: 14px; color: #555; line-height: 1.8; display: block; }
.room-price-card {
  background: linear-gradient(135deg, #1a1a2e, #16213e); border-radius: 12px; margin: 0 12px 12px;
  padding: 20px; display: flex; align-items: center; justify-content: space-around; color: #fff; flex-wrap: wrap; gap: 8px;
}
.price-item { text-align: center; }
.price-label { font-size: 12px; color: rgba(255,255,255,0.6); display: block; margin-bottom: 4px; }
.price-value { font-size: 24px; font-weight: 700; color: #e6a23c; }
.price-value.sub { color: rgba(255,255,255,0.8); font-size: 20px; }
.landlord-section { border-left: 3px solid #e6a23c; }
.landlord-item { display: flex; align-items: center; gap: 12px; padding: 10px 0; border-bottom: 1px solid #f5f5f5; }
.landlord-item:last-child { border-bottom: none; }
.landlord-avatar { width: 36px; height: 36px; border-radius: 50%; background: linear-gradient(135deg, #e6a23c, #f0a020); display: flex; align-items: center; justify-content: center; font-size: 20px; }
.landlord-info { flex: 1; display: flex; flex-direction: column; gap: 2px; }
.landlord-name { font-size: 15px; font-weight: 600; color: #333; }
.landlord-phone-masked { font-size: 12px; color: #999; }
.contact-btn { font-size: 12px; color: #1989fa; background: none; border: 1px solid #1989fa; border-radius: 20px; padding: 4px 12px; }
.video-wrap { border-radius: 10px; overflow: hidden; background: #000; margin-bottom: 10px; }
.video-player { width: 100%; max-height: 240px; }
.overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); z-index: 1000; display: flex; align-items: center; justify-content: center; }
.contact-dialog { background: #fff; border-radius: 16px; padding: 32px 24px 20px; width: 80%; text-align: center; }
.contact-avatar-wrap { font-size: 48px; margin-bottom: 12px; }
.contact-name { font-size: 18px; font-weight: 700; color: #1a1a2e; display: block; }
.contact-phone { font-size: 24px; font-weight: 700; color: #333; letter-spacing: 2px; display: block; margin: 8px 0 24px; }
.copy-btn { background: #1989fa; color: #fff; border: none; border-radius: 24px; padding: 12px 0; width: 100%; font-size: 15px; margin-bottom: 8px; }
.close-btn { background: none; color: #999; border: none; font-size: 14px; padding: 8px; }
</style>
