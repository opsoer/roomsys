<template>
  <div>
    <h3 style="margin-bottom: 20px">合同查询</h3>
    <div class="toolbar">
      <el-input
        v-model="keyword"
        placeholder="搜索租客姓名 / 电话 / 房间号"
        clearable
        style="width: 260px"
        @keyup.enter="search"
        @clear="search"
      >
        <template #append>
          <el-button :icon="Search" @click="search" />
        </template>
      </el-input>
      <el-select
        v-model="selectedRoomId"
        placeholder="全部房间"
        clearable
        filterable
        style="width: 200px"
        @change="search"
      >
        <el-option v-for="r in rooms" :key="r.id" :label="roomLabel(r)" :value="r.id" />
      </el-select>
      <el-select v-model="statusFilter" style="width: 130px" @change="search">
        <el-option label="全部状态" value="" />
        <el-option v-for="(label, val) in STATUS_LABELS" :key="val" :label="label" :value="val" />
      </el-select>
      <el-tag v-if="selectedRoom" :type="selectedRoom.status === 'vacant' ? 'info' : 'success'" size="large">
        {{ selectedRoom.room_number }}（{{ roomStatusLabel(selectedRoom.status) }}）
      </el-tag>
      <span v-if="!loading && total > 0" class="total-hint">共 {{ total }} 份合同</span>
    </div>

    <div v-if="loading" class="empty-wrap" v-loading="loading" element-loading-text="加载中..." />
    <div v-else-if="contracts.length === 0" class="empty-wrap">
      <el-empty description="没有符合条件的合同" />
    </div>
    <template v-else>
      <div class="desktop-table">
        <el-table :data="contracts" border stripe style="width: 100%">
          <el-table-column label="房间" width="110" fixed>
            <template #default="{ row }">{{ row.room?.room_number || '-' }}</template>
          </el-table-column>
          <el-table-column label="状态" width="90">
            <template #default="{ row }">
              <el-tag :type="STATUS_TAG_TYPES[row.status] || 'info'" size="small" effect="dark" round>
                {{ STATUS_LABELS[row.status] || row.status }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="租客" min-width="110">
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

      <div class="mobile-cards">
        <div v-for="row in contracts" :key="row.id" class="contract-card">
          <div class="cc-head">
            <span class="cc-tenant">{{ row.tenant?.name || '-' }}</span>
            <el-tag :type="STATUS_TAG_TYPES[row.status] || 'info'" size="small" effect="dark" round>
              {{ STATUS_LABELS[row.status] || row.status }}
            </el-tag>
          </div>
          <div class="cc-grid">
            <span class="cc-label">房间</span>
            <span class="cc-value">{{ row.room?.room_number || '-' }}</span>
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

      <div ref="sentinel" class="sentinel">
        <span v-if="loadingMore" class="list-footer">加载中...</span>
        <span v-else-if="contracts.length >= total" class="list-footer">没有更多了</span>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { Search } from '@element-plus/icons-vue'
import { buildingGetRooms, buildingGetContracts } from '../api'
import { ElMessage } from 'element-plus'

const PAGE_SIZE = 20

const STATUS_LABELS = { active: '进行中', reserved: '已预订', ended: '已结束', cancelled: '已取消' }
const STATUS_TAG_TYPES = { active: 'success', reserved: 'warning', ended: 'info', cancelled: 'danger' }

const rooms = ref([])
const selectedRoomId = ref(null)
const statusFilter = ref('')
const keyword = ref('')
const contracts = ref([])
const total = ref(0)
const loading = ref(false)
const loadingMore = ref(false)
const sentinel = ref(null)

let observer = null

function roomLabel(r) {
  return [r.room_number, r.floor ? r.floor + '层' : '', r.layout].filter(Boolean).join(' · ')
}

function roomStatusLabel(status) {
  return { vacant: '未出租', reserved: '已预订', rented: '已出租', expiring: '即将退租' }[status] || status
}

function mgmtFeeLabel(fee) {
  if (fee != null && Number(fee) > 0) return '¥' + Number(fee).toFixed(2) + '/月'
  return '无管理费'
}

async function fetchContracts(append = false) {
  if (!append) {
    loading.value = true
  } else {
    loadingMore.value = true
  }
  try {
    const params = { page_size: PAGE_SIZE }
    if (append && contracts.value.length > 0) {
      const last = contracts.value[contracts.value.length - 1]
      params.last_id = last.id
      params.last_key = last.start_date
    }
    if (selectedRoomId.value) params.room_id = selectedRoomId.value
    if (statusFilter.value) params.status = statusFilter.value
    if (keyword.value.trim()) params.keyword = keyword.value.trim()

    const res = await buildingGetContracts(params)
    const data = res.data.contracts || []
    if (!append) {
      total.value = res.data.total || 0
      contracts.value = data
    } else {
      // 去重兜底：游标分页偶发重复时避免同一合同显示多次
      const existingIds = new Set(contracts.value.map(c => c.id))
      const fresh = data.filter(c => !existingIds.has(c.id))
      if (fresh.length === 0) {
        total.value = Math.max(total.value, contracts.value.length)
      } else {
        contracts.value = [...contracts.value, ...fresh]
      }
    }
  } catch {
    ElMessage.error('获取合同列表失败')
  } finally {
    loading.value = false
    loadingMore.value = false
    nextTick(setupInfiniteScroll)
  }
}

function search() {
  fetchContracts()
}

function loadMore() {
  fetchContracts(true)
}

// 触底自动加载：sentinel 进入视口附近时加载下一页
function setupInfiniteScroll() {
  if (observer) observer.disconnect()
  if (!sentinel.value) return
  observer = new IntersectionObserver((entries) => {
    if (
      entries[0].isIntersecting &&
      !loading.value &&
      !loadingMore.value &&
      total.value > 0 &&
      contracts.value.length < total.value
    ) {
      loadMore()
    }
  }, { rootMargin: '200px 0px' })
  observer.observe(sentinel.value)
}

// 拉取全部房间供选择器使用（page_size 上限 100，超过时用游标分页循环取完）
async function fetchAllRoomsForPicker() {
  const all = []
  let lastId = 0
  let lastKey = ''
  try {
    for (let i = 0; i < 20; i++) {
      const params = { page: 1, page_size: 100 }
      if (lastId) {
        params.last_id = lastId
        params.last_key = lastKey
      }
      const res = await buildingGetRooms(params)
      const list = res?.data?.rooms || []
      all.push(...list)
      if (list.length === 0 || all.length >= (res?.data?.total || 0)) break
      lastId = list[list.length - 1].id
      lastKey = list[list.length - 1].room_number
    }
  } catch { /* 网络异常时保留已取到的部分 */ }
  return all
}

onMounted(async () => {
  rooms.value = await fetchAllRoomsForPicker()
  fetchContracts()
})

onBeforeUnmount(() => {
  if (observer) observer.disconnect()
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
.total-hint {
  color: #999;
  font-size: 13px;
}
.empty-wrap {
  background: #fff;
  border-radius: 8px;
  padding: 24px 0;
  border: 1px solid #eee;
  min-height: 240px;
}
.desktop-table { display: block; }
.mobile-cards { display: none; }
.sentinel {
  display: flex;
  justify-content: center;
  padding: 12px 0 4px;
}
.list-footer {
  color: #999;
  font-size: 13px;
}

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
