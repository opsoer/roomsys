<template>
  <div class="detail-hero" :class="{ 'has-cover': coverImage }">
    <div v-if="coverImage" class="detail-cover">
      <img :src="mediaUrl(coverImage.file_path)" class="cover-img" @click="$emit('fullscreen', mediaUrl(coverImage.file_path))" />
      <el-button v-if="isAdmin" size="small" type="danger" circle class="cover-delete-btn"
        @click.stop="$emit('delete-media', coverImage.id)">
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
        <el-button type="primary" size="small" @click="$emit('edit')">编辑</el-button>
        <el-button type="danger" size="small" @click="$emit('delete-room')">删除</el-button>
      </div>
    </div>
  </div>
</template>

<script setup>
defineProps({
  room: { type: Object, required: true },
  coverImage: { type: Object, default: null },
  isAdmin: { type: Boolean, default: false },
})

defineEmits(['fullscreen', 'delete-media', 'edit', 'delete-room'])

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
</script>

<style scoped>
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
.detail-cover:hover .cover-delete-btn { opacity: 1; }

@media (max-width: 768px) {
  .detail-hero { min-height: 200px; border-radius: 12px; margin-bottom: 16px; }
  .detail-hero.has-cover { min-height: 240px; }
  .detail-hero-content { padding: 20px; }
  .detail-title { font-size: 24px; }
  .hero-actions { flex-wrap: wrap; }
  .hero-actions .el-button { flex: 1; font-size: 12px; }
}
</style>
