<template>
  <div>
    <h3 style="margin-bottom: 20px">合同查询</h3>
    <div class="toolbar">
      <el-select
        v-model="selectedRoomId"
        placeholder="选择房间"
        filterable
        clearable
        style="width: 240px"
        @change="handleRoomChange"
      >
        <el-option v-for="r in rooms" :key="r.id" :label="roomLabel(r)" :value="r.id" />
      </el-select>
      <el-tag v-if="selectedRoom" :type="selectedRoom.status === 'vacant' ? 'info' : 'success'" size="large">
        {{ selectedRoom.room_number }}（{{ roomStatusLabel(selectedRoom.status) }}）
      </el-tag>
    </div>

    <div v-if="selectedRoomId === null" class="empty-wrap">
      <el-empty description="请选择房间查看历史合同" />
    </div>
    <div v-else-if="loading" class="empty-wrap">
      <el-empty description="加载中..." />
    </div>
    <div v-else-if="contracts.length === 0" class="empty-wrap">
      <el-empty description="该房间暂无历史合同" />
    </div>
    <template v-else>
      <div class="desktop-table">
        <el-table :data="contracts" border stripe style="width: 100%" v-loading="loading">
          <el-table-column label="状态" width="90">
            <template #default="{ row }">
              <el-tag :type="row.status === 'active' ? 'success' : 'info'" size="small" effect="dark" round>
                {{ contractStatusLabel(row.status) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="租客" min-width="120">
            <template #default="{ row }">{{ row.tenant?.name || '-' }}</template>
          </el-table-column>
          <el-table-column label="电话" width="140">
            <template #default="{ row }">{{ row.tenant?.phone || '-' }}</template>
          </el-table-column>
          <el-table-column label="起租" width="110">
            <template #default="{ row }">{{ row.start_date }}</template>
          </el-table-column>
          <el-table-column label="到期" width="110">
            <template #default="{ row }">{{ row.end_date || '—' }}</template>
          </el-table-column>
          <el-table-column label="月租金" width="120">
            <template #default="{ row }">
              <span style="font-weight: bold">¥{{ Number(row.rent_price).toFixed(2) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="管理费" width="120">
            <template #default="{ row }">
              <span>{{ mgmtFeeLabel(row.management_fee) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="押金" width="120">
            <template #default="{ row }">
              <span>¥{{ Number(row.deposit).toFixed(2) }}</span>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <div class="mobile-cards" v-loading="loading">
        <div v-for="row in contracts" :key="row.id" class="contract-card">
          <div class="cc-head">
            <span class="cc-tenant">{{ row.tenant?.name || '-' }}</span>
            <el-tag :type="row.status === 'active' ? 'success' : 'info'" size="small" effect="dark" round>
              {{ contractStatusLabel(row.status) }}
            </el-tag>
          </div>
          <div class="cc-grid">
            <span class="cc-label">电话</span>
            <span class="cc-value">{{ row.tenant?.phone || '-' }}</span>
            <span class="cc-label">起租</span>
            <span class="cc-value">{{ row.start_date }}</span>
            <span class="cc-label">到期</span>
            <span class="cc-value">{{ row.end_date || '—' }}</span>
            <span class="cc-label">月租金</span>
            <span class="cc-value">¥{{ Number(row.rent_price).toFixed(2) }}</span>
            <span class="cc-label">管理费</span>
            <span class="cc-value">{{ mgmtFeeLabel(row.management_fee) }}</span>
            <span class="cc-label">押金</span>
            <span class="cc-value">¥{{ Number(row.deposit).toFixed(2) }}</span>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { buildingGetRooms, buildingGetRoomContracts } from '../api'
import { ElMessage } from 'element-plus'

const rooms = ref([])
const selectedRoomId = ref(null)
const selectedRoom = ref(null)
const contracts = ref([])
const loading = ref(false)

function roomLabel(r) {
  return [r.room_number, r.floor ? r.floor + '层' : '', r.layout].filter(Boolean).join(' · ')
}

function roomStatusLabel(status) {
  return { vacant: '未出租', reserved: '已预订', rented: '已出租', expiring: '即将退租' }[status] || status
}

function contractStatusLabel(status) {
  return status === 'active' ? '进行中' : '已结束'
}

function mgmtFeeLabel(fee) {
  if (fee != null && Number(fee) > 0) return '¥' + Number(fee).toFixed(2) + '/月'
  return '无管理费'
}

async function fetchContracts() {
  if (selectedRoomId.value === null) {
    contracts.value = []
    selectedRoom.value = null
    return
  }
  loading.value = true
  try {
    selectedRoom.value = rooms.value.find(r => r.id === selectedRoomId.value) || null
    const res = await buildingGetRoomContracts(selectedRoomId.value)
    contracts.value = res.data.contracts || []
  } catch {
    ElMessage.error('获取合同列表失败')
    contracts.value = []
  } finally {
    loading.value = false
  }
}

function handleRoomChange(val) {
  selectedRoomId.value = val ?? null
  fetchContracts()
}

onMounted(async () => {
  try {
    const res = await buildingGetRooms()
    rooms.value = res.data.rooms || []
  } catch {
    ElMessage.error('获取房间列表失败')
    return
  }
  if (rooms.value.length > 0) {
    selectedRoomId.value = rooms.value[0].id
    await fetchContracts()
  }
})
</script>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}
.empty-wrap {
  background: #fff;
  border-radius: 8px;
  padding: 24px 0;
  border: 1px solid #eee;
}
.desktop-table { display: block; }
.mobile-cards { display: none; }

.contract-card {
  background: #fff;
  border-radius: 10px;
  padding: 12px 14px;
  margin-bottom: 10px;
  border: 1px solid #eee;
  box-shadow: 0 1px 4px rgba(0,0,0,0.04);
}
.cc-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}
.cc-tenant {
  font-size: 15px;
  font-weight: bold;
}
.cc-grid {
  display: grid;
  grid-template-columns: 64px 1fr;
  row-gap: 6px;
  font-size: 13px;
}
.cc-label { color: #999; }
.cc-value { color: #333; }

@media (max-width: 768px) {
  .desktop-table { display: none; }
  .mobile-cards { display: block; }
}
</style>
