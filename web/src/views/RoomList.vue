<template>
  <div class="page-home">
    <div class="section-header">
      <h2>房源管理</h2>
      <div class="section-actions">
        <el-select v-model="statusFilter" placeholder="筛选状态" clearable style="width: 120px" @change="fetchRooms">
          <el-option label="全部" value="" />
          <el-option label="未出租" value="vacant" />
          <el-option label="已出租" value="rented" />
          <el-option label="即将退租" value="expiring" />
        </el-select>
        <el-select v-model="floorFilter" placeholder="楼层" clearable style="width: 90px" @change="fetchRooms">
          <el-option label="全部楼层" value="" />
          <el-option v-for="f in floorOptions" :key="f" :label="f + '层'" :value="String(f)" />
        </el-select>
        <el-select v-model="layoutFilter" placeholder="户型" clearable style="width: 120px" @change="fetchRooms">
          <el-option label="全部户型" value="" />
          <el-option v-for="lo in layoutOptions" :key="lo" :label="lo" :value="lo" />
        </el-select>
        <el-button type="primary" @click="showAddDialog = true">
          <el-icon><Plus /></el-icon> 添加房间
        </el-button>
      </div>
    </div>

    <div v-if="loading" class="skeleton-wrap">
      <div v-for="n in 6" :key="n" class="skeleton-item">
        <el-skeleton :rows="3" animated>
          <template #template>
            <el-skeleton-item variant="image" style="height: 120px; border-radius: 8px 8px 0 0;" />
            <div style="padding: 12px;">
              <el-skeleton-item variant="h3" style="width: 60%; margin-bottom: 8px;" />
              <el-skeleton-item variant="text" style="width: 40%; margin-bottom: 6px;" />
              <el-skeleton-item variant="text" style="width: 80%;" />
            </div>
          </template>
        </el-skeleton>
      </div>
    </div>

    <div v-else-if="rooms.length === 0" class="empty-wrap">
      <el-empty description="暂无房间数据" />
    </div>

    <div v-else class="room-grid">
      <div v-for="room in rooms" :key="room.id" class="room-card" @click="$router.push(`/landlord/rooms/${room.id}`)">
        <div class="room-card-image">
            <img v-if="room.thumbnail" :src="mediaUrl(room.thumbnail)" :alt="room.room_number" loading="lazy" @error="e => { e.target.onerror = null; e.target.src = '/default-image.svg' }" />
          <div v-else class="room-card-placeholder">
            <el-icon :size="48" color="#ccc"><Picture /></el-icon>
          </div>
          <span class="room-card-tag" :class="'tag-' + room.status">{{ statusLabel(room.status) }}</span>
        </div>
        <div class="room-card-body">
          <h3 class="room-card-number">{{ room.room_number }}</h3>
          <p class="room-card-info">
            <template v-if="room.floor">{{ room.floor }}层</template>
            <template v-if="room.floor && room.layout"> · </template>
            <template v-if="room.layout">{{ room.layout }}</template>
          </p>
          <p v-if="room.end_date" class="room-card-enddate">退租日期：{{ room.end_date }}</p>
        </div>
      </div>
    </div>

    <el-dialog v-model="showAddDialog" title="添加房间" width="500px">
      <el-form ref="addFormRef" :model="addForm" label-width="90px">
        <el-form-item label="房间号" prop="room_number" :rules="[{ required: true, message: '请输入房间号' }]">
          <el-input v-model="addForm.room_number" />
        </el-form-item>
        <el-form-item label="楼层" prop="floor" :rules="[{ required: true, message: '请选择楼层' }]">
          <el-select v-model="addForm.floor" placeholder="选择楼层" style="width: 100%">
            <el-option v-for="f in floorOptions" :key="f" :label="f + '层'" :value="String(f)" />
          </el-select>
        </el-form-item>
        <el-form-item label="户型" prop="layout" :rules="[{ required: true, message: '请选择户型' }]">
          <el-select v-model="addForm.layout" placeholder="选择户型" style="width: 100%">
            <el-option v-for="lo in layoutOptions" :key="lo" :label="lo" :value="lo" />
          </el-select>
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input v-model="addForm.description" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddDialog = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleAdd">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { buildingGetRooms, buildingCreateRoom } from '../api'
import { ElMessage } from 'element-plus'
import { FLOOR_OPTIONS, LAYOUT_OPTIONS } from '../utils/constants'
import { mediaUrl, statusLabel } from '../utils/format'

const floorOptions = FLOOR_OPTIONS
const layoutOptions = LAYOUT_OPTIONS

const rooms = ref([])
const loading = ref(true)
const statusFilter = ref('')
const floorFilter = ref('')
const layoutFilter = ref('')
const showAddDialog = ref(false)
const submitting = ref(false)
const addForm = ref({ room_number: '', floor: '', layout: '', description: '' })
const addFormRef = ref(null)

async function fetchRooms() {
  loading.value = true
  try {
    const params = {}
    if (statusFilter.value) params.status = statusFilter.value
    if (floorFilter.value) params.floor = floorFilter.value
    if (layoutFilter.value) params.layout = layoutFilter.value
    const res = await buildingGetRooms(params)
    rooms.value = res.data.rooms || []
  } catch {
    ElMessage.error('获取房间列表失败')
  } finally {
    loading.value = false
  }
}

async function handleAdd() {
  const valid = await addFormRef.value.validate().catch(() => false)
  if (!valid) return
  submitting.value = true
  try {
    await buildingCreateRoom(addForm.value)
    ElMessage.success('添加成功')
    showAddDialog.value = false
    addForm.value = { room_number: '', floor: '', layout: '', description: '' }
    await fetchRooms()
  } catch {
    ElMessage.error('添加房间失败')
  } finally {
    submitting.value = false
  }
}

onMounted(fetchRooms)
</script>

<style scoped>
.page-home { min-height: 100vh; background: transparent; }
.section-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px; }
.section-header h2 { font-size: 20px; font-weight: 700; color: #1a1a2e; }
.section-actions { display: flex; gap: 10px; }
.skeleton-wrap { display: grid; grid-template-columns: repeat(auto-fill,minmax(270px,1fr)); gap: 24px; padding: 12px 0; }
.skeleton-item { background: #fff; border-radius: 12px; overflow: hidden; }
.empty-wrap { padding: 60px 0; }
.room-grid { display: grid; grid-template-columns: repeat(auto-fill,minmax(270px,1fr)); gap: 24px; }
.room-card { background: #fff; border-radius: 12px; overflow: hidden; cursor: pointer; transition: all 0.35s cubic-bezier(0.4,0,0.2,1); box-shadow: 0 2px 12px rgba(0,0,0,0.06); }
.room-card:hover { transform: translateY(-6px); box-shadow: 0 12px 32px rgba(0,0,0,0.12); }
.room-card-image { position: relative; height: 200px; background: #e9ecef; overflow: hidden; }
.room-card-image img { width: 100%; height: 100%; object-fit: cover; transition: transform 0.5s; }
.room-card:hover .room-card-image img { transform: scale(1.08); }
.room-card-placeholder { height: 100%; display: flex; align-items: center; justify-content: center; }
.room-card-tag { position: absolute; top: 12px; left: 12px; padding: 4px 12px; border-radius: 20px; font-size: 12px; font-weight: 600; color: #fff; }
.tag-vacant { background: rgba(103,194,58,0.85); }
.tag-rented { background: rgba(245,108,108,0.85); }
.tag-expiring { background: rgba(230,162,60,0.85); }
.room-card-body { padding: 16px; }
.room-card-number { font-size: 16px; font-weight: 600; color: #1a1a2e; margin-bottom: 6px; }
.room-card-info { font-size: 13px; color: #888; }
.room-card-enddate { margin-top: 6px; font-size: 12px; color: #e6a23c; font-weight: 600; }
@media (max-width: 768px) {
  .section-header { flex-direction: column; align-items: flex-start; gap: 12px; }
  .section-actions { width: 100%; flex-wrap: wrap; }
  .room-grid { grid-template-columns: repeat(2,1fr); gap: 12px; }
  .room-card-image { height: 140px; }
  .room-card-body { padding: 12px; }
  .room-card-number { font-size: 14px; }
}
</style>
