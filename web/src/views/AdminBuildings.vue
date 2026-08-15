<template>
  <div style="max-width: 1200px; margin: 0 auto;">
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; flex-wrap: wrap; gap: 12px;">
      <h2 style="font-size: 22px; font-weight: 700;">
        公寓管理
        <span v-if="total > 0" style="font-size: 13px; font-weight: 400; color: #999; margin-left: 8px;">共 {{ total }} 栋</span>
      </h2>
      <div style="display: flex; gap: 12px; flex-wrap: wrap;">
        <el-button type="primary" plain @click="openSiteQr">网站主页二维码</el-button>
        <el-button type="primary" @click="openCreate">
          <el-icon><Plus /></el-icon> 创建公寓
        </el-button>
      </div>
    </div>

    <AdminBuildingList ref="listRef" :buildings="buildings" :loading="loading"
      @search="fetchBuildings" @edit="handleEdit" @upgrade="handleUpgrade"
      @copy-home-link="copyHomeLink" @download-qr="downloadQr" @create-admin="handleCreateAdmin" @delete="handleDelete"
      @toggle-visibility="handleToggleVisibility" @renew="handleRenew" @history="handleHistory" />

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

    <el-dialog v-model="siteQrVisible" title="网站主页二维码" width="380px" align-center>
      <div style="text-align: center;">
        <img v-if="siteQrDataUrl" :src="siteQrDataUrl" alt="网站主页二维码"
          style="width: 240px; border: 1px solid #f0f0f0; border-radius: 8px;" />
        <el-icon v-else class="is-loading" style="font-size: 40px; color: #999"><Loading /></el-icon>
        <div style="font-size: 13px; color: #999; margin-top: 12px; word-break: break-all;">
          扫码访问网站主页：<br />{{ siteHomeLink }}
        </div>
      </div>
      <template #footer>
        <el-button @click="copySiteLink">复制主页链接</el-button>
        <el-button type="primary" @click="downloadSiteQrCard">下载二维码</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="buildingQrVisible" title="公寓二维码" width="380px" align-center>
      <div style="text-align: center;">
        <img v-if="buildingQrDataUrl" :src="buildingQrDataUrl" alt="公寓二维码"
          style="width: 240px; border: 1px solid #f0f0f0; border-radius: 8px;" />
        <el-icon v-else class="is-loading" style="font-size: 40px; color: #999"><Loading /></el-icon>
        <div style="font-size: 13px; color: #999; margin-top: 12px; word-break: break-all;">
          扫码访问公寓主页：<br />{{ buildingQrLink }}
        </div>
      </div>
      <template #footer>
        <el-button @click="copyBuildingQrLink">复制主页链接</el-button>
        <el-button type="primary" @click="downloadBuildingQrCard">下载二维码</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import { adminGetBuildings, adminDeleteBuilding, adminUpdateBuilding } from '../api'
import { buildingHomeUrl, siteHomeUrl, generateBrandedQrDataUrl, generateSiteQrDataUrl, downloadQrImage, buildingInfoLines } from '../utils/qr'
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

const siteQrVisible = ref(false)
const siteQrDataUrl = ref('')
const siteHomeLink = siteHomeUrl()

const buildingQrVisible = ref(false)
const buildingQrDataUrl = ref('')
const buildingQrLink = ref('')
const buildingQrBuilding = ref(null)

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

function handleRenew(row) {
  dialogsRef.value?.openRenew(row)
}

function handleHistory(row) {
  dialogsRef.value?.openHistory(row)
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

function copyHomeLink(row) {
  const url = buildingHomeUrl(row.id)
  navigator.clipboard.writeText(url).then(() => {
    ElMessage.success('已复制公寓主页链接')
  }, () => {
    ElMessage.error('复制失败，请手动复制')
  })
}

async function downloadQr(row) {
  openBuildingQr(row)
}

function buildingQrTitle() {
  return buildingQrBuilding.value?.name || '公寓主页'
}

function openBuildingQr(row) {
  buildingQrVisible.value = true
  buildingQrDataUrl.value = ''
  buildingQrBuilding.value = row
  buildingQrLink.value = buildingHomeUrl(row.id)
  generateBrandedQrDataUrl({
    text: buildingQrLink.value,
    title: row.name || '公寓主页',
    lines: buildingInfoLines(row),
  }).then((url) => {
    buildingQrDataUrl.value = url
  }).catch(() => {
    ElMessage.error('二维码生成失败')
  })
}

function copyBuildingQrLink() {
  navigator.clipboard.writeText(buildingQrLink.value).then(() => {
    ElMessage.success('已复制公寓主页链接')
  }, () => {
    ElMessage.error('复制失败，请手动复制')
  })
}

function downloadBuildingQrCard() {
  if (!buildingQrDataUrl.value) {
    ElMessage.error('二维码尚未生成，请稍后重试')
    return
  }
  downloadQrImage(buildingQrDataUrl.value, `公寓二维码_${buildingQrTitle()}.png`)
  ElMessage.success('二维码已下载')
}

function openSiteQr() {
  siteQrVisible.value = true
  if (!siteQrDataUrl.value) {
    generateSiteQrDataUrl().then((url) => {
      siteQrDataUrl.value = url
    }).catch(() => {
      ElMessage.error('二维码生成失败')
    })
  }
}

async function downloadSiteQrCard() {
  if (!siteQrDataUrl.value) {
    ElMessage.error('二维码尚未生成，请稍后重试')
    return
  }
  downloadQrImage(siteQrDataUrl.value, '网站主页二维码.png')
  ElMessage.success('二维码已下载')
}

function copySiteLink() {
  navigator.clipboard.writeText(siteHomeLink).then(() => {
    ElMessage.success('已复制网站主页链接')
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
