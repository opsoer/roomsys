<template>
  <div>
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
          <img :src="mediaUrl(img.file_path)" class="gallery-img" @click="$emit('fullscreen', mediaUrl(img.file_path))" />
          <el-button v-if="isAdmin" size="small" type="danger" circle class="delete-btn"
            @click.stop="$emit('delete-media', img.id)">
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
            @click.stop="$emit('delete-media', v.id)">
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
</template>

<script setup>
import { Close } from '@element-plus/icons-vue'

defineProps({
  room: { type: Object, required: true },
  galleryImages: { type: Array, default: () => [] },
  videos: { type: Array, default: () => [] },
  isAdmin: { type: Boolean, default: false },
})

defineEmits(['fullscreen', 'delete-media'])

function mediaUrl(path) {
  if (!path) return ''
  return `/api/media/${path}`
}
</script>

<style scoped>
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

.gallery-item .delete-btn,
.video-item .delete-btn {
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
.video-item.admin-mode:hover .delete-btn { opacity: 1; }

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

@media (max-width: 768px) {
  .detail-card { padding: 16px; }
  .gallery-grid { grid-template-columns: repeat(2, 1fr); gap: 8px; }
  .gallery-img { height: 130px; }
  .video-list { grid-template-columns: 1fr; gap: 8px; }
  .video-player { max-height: 200px; }
}
</style>
