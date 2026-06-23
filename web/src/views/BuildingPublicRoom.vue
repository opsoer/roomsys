<template>
  <div class="page-room" v-if="room">
    <header class="site-header">
      <div class="header-inner">
        <div class="logo" @click="$router.push('/')">
          <span class="logo-icon">☀️</span>
          <span class="logo-text">邻租</span>
        </div>
        <nav class="nav-links">
          <a class="nav-link" href="/">首页</a>
          <a class="nav-link" :href="`/building/${buildingId}`">返回公寓</a>
          <a class="nav-link active">{{ room.room_number }}</a>
        </nav>
      </div>
    </header>

    <div class="room-detail" style="max-width: 1000px; margin: 100px auto 40px; padding: 0 24px;">
      <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 32px;">
        <div>
          <el-image v-if="coverImage" :src="mediaUrl(coverImage)" style="width: 100%; border-radius: 12px;" fit="cover" />
          <div v-else style="height: 400px; background: #e9ecef; border-radius: 12px; display: flex; align-items: center; justify-content: center; color: #ccc;">
            <el-icon :size="64"><Picture /></el-icon>
          </div>
          <div v-if="galleryImages.length" style="display: flex; gap: 8px; margin-top: 12px; flex-wrap: wrap;">
            <el-image v-for="(img, i) in galleryImages" :key="i" :src="mediaUrl(img)" style="width: 80px; height: 80px; border-radius: 8px; cursor: pointer;" fit="cover" />
          </div>
        </div>
        <div>
          <h1 style="font-size: 28px; font-weight: 700; margin-bottom: 8px;">{{ room.room_number }}</h1>
          <div style="display: flex; gap: 12px; margin-bottom: 20px;">
            <el-tag :type="statusTag(room.status)" size="large">{{ statusLabel(room.status) }}</el-tag>
            <el-tag>{{ room.floor }}层</el-tag>
            <el-tag>{{ room.layout }}</el-tag>
          </div>
          <p v-if="room.description" style="color: #555; line-height: 1.8; margin-bottom: 24px;">{{ room.description }}</p>

          <el-card v-if="contract" style="margin-bottom: 20px;">
            <template #header><strong>当前租约信息</strong></template>
            <p>租客：{{ contract.tenant?.name }}</p>
            <p>起租：{{ contract.start_date }} 至 {{ contract.end_date }}</p>
            <p>月租金：¥{{ contract.rent_price }}</p>
          </el-card>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { Picture } from '@element-plus/icons-vue'
import { getPublicRoom, getPublicRoomContract } from '../api'

const route = useRoute()
const buildingId = computed(() => route.params.bid)
const roomId = computed(() => route.params.id)
const room = ref(null)
const contract = ref(null)
const coverImage = ref('')
const galleryImages = ref([])

function mediaUrl(path) {
  if (!path) return ''
  return `/api/media/${path}`
}

function statusTag(s) {
  return s === 'vacant' ? 'success' : s === 'rented' ? 'danger' : 'warning'
}

function statusLabel(s) {
  return s === 'vacant' ? '未出租' : s === 'rented' ? '已出租' : '即将退租'
}

onMounted(async () => {
  try {
    const res = await getPublicRoom(buildingId.value, roomId.value)
    room.value = res.data.room
    const media = res.data.room.media || []
    for (const m of media) {
      if (m.category === 'cover') coverImage.value = m.file_path
      else if (m.type === 'image') galleryImages.value.push(m.file_path)
    }
  } catch {}
  try {
    const res = await getPublicRoomContract(buildingId.value, roomId.value)
    contract.value = res.data.contract
  } catch {}
})
</script>

<style scoped>
.page-room { min-height: 100vh; background: #f5f7fa; }
.site-header {
  position: fixed; top: 0; left: 0; right: 0; z-index: 100;
  background: rgba(255,255,255,0.92); backdrop-filter: blur(12px); border-bottom: 1px solid rgba(0,0,0,0.06);
}
.header-inner {
  max-width: 1200px; margin: 0 auto; padding: 0 24px; height: 64px;
  display: flex; align-items: center; gap: 40px;
}
.logo { display: flex; align-items: center; gap: 8px; cursor: pointer; }
.logo-icon { font-size: 24px; }
.logo-text { font-size: 20px; font-weight: 700; background: linear-gradient(135deg,#e6a23c,#f56c6c); -webkit-background-clip: text; -webkit-text-fill-color: transparent; background-clip: text; }
.nav-links { display: flex; gap: 24px; flex: 1; }
.nav-link { color: #555; text-decoration: none; font-size: 15px; }
.nav-link:hover, .nav-link.active { color: #e6a23c; }
</style>
