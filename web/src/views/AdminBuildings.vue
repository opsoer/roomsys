<template>
  <div style="max-width: 1200px; margin: 0 auto;">
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; flex-wrap: wrap; gap: 12px;">
      <h2 style="font-size: 22px; font-weight: 700;">
        公寓管理
        <span v-if="total > 0" style="font-size: 13px; font-weight: 400; color: #999; margin-left: 8px;">共 {{ total }} 栋</span>
      </h2>
      <el-button type="primary" @click="openCreate">
        <el-icon><Plus /></el-icon> 创建公寓
      </el-button>
    </div>

    <AdminBuildingList ref="listRef" :buildings="buildings" :loading="loading"
      @search="fetchBuildings" @edit="handleEdit" @upgrade="handleUpgrade"
      @copy-link="copyLoginLink" @create-admin="handleCreateAdmin" @delete="handleDelete"
      @toggle-visibility="handleToggleVisibility" />

    <div v-if="loadingMore" style="text-align: center; padding: 16px; color: #999">
      <el-icon class="is-loading"><Loading /></el-icon> 加载中...
    </div>
    <div v-else-if="total > 0 && buildings.length >= total" style="text-align: center; padding: 16px; color: #999; font-size: 13px">
      已全部加载（共 {{ total }} 栋）
    </div>
    <div v-else-if="total > 0" style="text-align: center; padding: 16px; color: #999; font-size: 13px">
      共 {{ total }} 栋，已显示 {{ buildings.length }} 栋
    </div>
    <div ref="sentinel" style="height: 10px"></div>

    <AdminBuildingDialogs ref="dialogsRef" @save-success="fetchBuildings" />
  </div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import { adminGetBuildings, adminDeleteBuilding, adminUpdateBuilding } from '../api'
import AdminBuildingList from '../components/admin/AdminBuildingList.vue'
import AdminBuildingDialogs from '../components/admin/AdminBuildingDialogs.vue'

const buildings = ref([])
const loading = ref(true)
const loadingMore = ref(false)
const total = ref(0)
const sentinel = ref(null)
let observer = null
const listRef = ref(null)
const dialogsRef = ref(null)

const PAGE_SIZE = 20

async function fetchBuildings(append = false) {
  if (!append) {
    loading.value = true
  } else {
    loadingMore.value = true
  }
  try {
    const filter = listRef.value?.getFilter() || {}
    const params = { page_size: PAGE_SIZE }
    // 游标分页：加载更多时携带上一批最后一条的 id
    if (append && buildings.value.length > 0) {
      params.last_id = buildings.value[buildings.value.length - 1].id
    }
    if (filter.status) params.status = filter.status
    if (filter.keyword) params.keyword = filter.keyword
    const res = await adminGetBuildings(params)
    const data = res.data.buildings || []
    if (!append) total.value = res.data.total || 0
    if (append) {
      buildings.value = [...buildings.value, ...data]
    } else {
      buildings.value = data
    }
  } finally {
    loading.value = false
    loadingMore.value = false
    nextTick(setupInfiniteScroll)
  }
}

function loadMore() {
  fetchBuildings(true)
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
      buildings.value.length < total.value
    ) {
      loadMore()
    }
  }, { rootMargin: '200px 0px' })
  observer.observe(sentinel.value)
}

function openCreate() {
  dialogsRef.value?.openCreate()
}

function handleEdit(row) {
  dialogsRef.value?.openEdit(row)
}

function handleUpgrade(row) {
  dialogsRef.value?.openUpgrade(row)
}

function handleCreateAdmin(row) {
  dialogsRef.value?.openCreateAdmin(row)
}

async function handleDelete(id) {
  try {
    await adminDeleteBuilding(id)
    ElMessage.success('已删除')
    await fetchBuildings()
  } catch {
    ElMessage.error('删除失败')
  }
}

async function handleToggleVisibility(row) {
  const target = row.status === 'hidden' ? 'active' : 'hidden'
  try {
    await adminUpdateBuilding(row.id, { status: target })
    ElMessage.success(target === 'hidden' ? '已设为不可见，首页及所有房间将不再展示' : '已恢复可见')
    await fetchBuildings()
  } catch {
    ElMessage.error('操作失败')
  }
}

function copyLoginLink(row) {
  const url = `${window.location.origin}/login`
  navigator.clipboard.writeText(url).then(() => {
    ElMessage.success('已复制管理员登录页面链接')
  }, () => {
    ElMessage.error('复制失败，请手动复制')
  })
}

onMounted(() => {
  fetchBuildings()
})

onBeforeUnmount(() => {
  if (observer) observer.disconnect()
  observer = null
})
</script>

<style scoped>
@media (max-width: 768px) {
  .el-card { padding: 12px; }
}
</style>
