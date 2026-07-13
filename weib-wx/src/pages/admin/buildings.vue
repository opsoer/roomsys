<template>
  <view class="page-admin-buildings">
    <view v-if="!auth.isLoggedIn" class="login-prompt">
      <text>请先登录</text>
      <button @click="uni.navigateTo({ url: '/pages/login/login' })">去登录</button>
    </view>
    <template v-else>
      <view class="header-row">
        <text class="page-title">公寓管理</text>
        <button class="add-btn" @click="openCreate">+ 创建公寓</button>
      </view>

      <!-- 筛选 -->
      <view class="search-bar">
        <input class="search-input" v-model="keyword" placeholder="搜索公寓名或房东电话" @confirm="fetchBuildings" />
        <button class="search-btn" @click="fetchBuildings">搜索</button>
      </view>

      <view v-if="loading" class="loading-wrap"><text>加载中...</text></view>
      <view v-else-if="buildings.length === 0" class="empty-wrap"><text>暂无公寓数据</text></view>

      <view v-else class="building-list">
        <view v-for="b in buildings" :key="b.id" class="building-card">
          <view class="card-top">
            <text class="card-name">{{ b.name }}</text>
            <text :class="['pkg-tag', b.package === 'full' ? 'pkg-full' : 'pkg-basic']">{{ b.package === 'full' ? '全套餐' : '基础套餐' }}</text>
          </view>
          <view class="card-info">
            <text>📍 {{ b.district }} {{ b.street }} {{ b.village }} {{ b.building_no }}</text>
            <text v-if="b.landlords?.length">👤 {{ b.landlords.map(l => l.name + ' ' + l.phone).join(' / ') }}</text>
            <text>🚪 {{ b.room_count }} 间<text v-if="b.vacant_count > 0" class="vacant-hint"> 可租 {{ b.vacant_count }}</text></text>
            <text v-if="b.contract_date">📅 {{ b.contract_date }} → {{ b.expired_at || '未设置' }}</text>
          </view>
          <view class="card-actions">
            <button class="act-btn" @click="handleEdit(b)">编辑</button>
            <button class="act-btn" @click="handleUpgrade(b)">套餐</button>
            <button class="act-btn" @click="handleCreateAdmin(b)">创建管理员</button>
            <button class="act-btn danger" @click="handleDelete(b.id)">删除</button>
          </view>
        </view>
      </view>

      <!-- 创建/编辑弹窗 -->
      <view v-if="showCreateDialog" class="overlay" @click="showCreateDialog = false">
        <scroll-view scroll-y class="dialog-panel" @click.stop>
          <text class="dialog-title">{{ editingId ? '编辑公寓' : '创建公寓' }}</text>
          <view class="form-group"><text class="form-label">名称</text><input class="form-input" v-model="createForm.name" /></view>
          <view class="form-group"><text class="form-label">区域</text>
            <picker mode="selector" :range="districtLabels" @change="e => { createForm.district = districts[e.detail.value]?.value || ''; createForm.street = ''; createForm.village = '' }">
              <view class="picker-val">{{ createForm.district || '选择区域' }}</view>
            </picker>
          </view>
          <view class="form-group"><text class="form-label">街道</text>
            <picker mode="selector" :range="streetLabels" @change="e => { createForm.street = currentStreets[e.detail.value]?.value || ''; createForm.village = '' }">
              <view class="picker-val">{{ createForm.street || '选择街道' }}</view>
            </picker>
          </view>
          <view class="form-group">
            <text class="form-label">楼牌号</text>
            <input class="form-input" v-model="createForm.building_no" />
          </view>
          <view class="dialog-actions">
            <button class="dialog-btn cancel" @click="showCreateDialog = false">取消</button>
            <button class="dialog-btn confirm" :disabled="submitting" @click="handleSave">{{ submitting ? '提交中...' : '确定' }}</button>
          </view>
        </scroll-view>
      </view>
    </template>
  </view>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { adminGetBuildings, adminCreateBuilding, adminUpdateBuilding, adminDeleteBuilding, adminCreateBuildingAdmin } from '../../api'
import { auth } from '../../store/auth'
import shenzhen from '../../utils/shenzhen'

const buildings = ref([])
const loading = ref(true)
const keyword = ref('')
const showCreateDialog = ref(false)
const editingId = ref(null)
const submitting = ref(false)
const createForm = ref({ name: '', district: '', street: '', village: '', building_no: '' })

const districts = shenzhen
const districtLabels = computed(() => districts.map(d => d.label))
const currentStreets = computed(() => {
  const d = districts.find(x => x.value === createForm.value.district)
  return d ? d.streets : []
})
const streetLabels = computed(() => currentStreets.value.map(s => s.label))

async function fetchBuildings() {
  loading.value = true
  try {
    const params = {}
    if (keyword.value) params.keyword = keyword.value
    const res = await adminGetBuildings(params)
    buildings.value = res.data.buildings || []
  } catch {
    uni.showToast({ title: '获取失败', icon: 'none' })
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingId.value = null
  createForm.value = { name: '', district: '', street: '', village: '', building_no: '' }
  showCreateDialog.value = true
}

function handleEdit(row) {
  editingId.value = row.id
  createForm.value = {
    name: row.name,
    district: row.district || '',
    street: row.street || '',
    village: row.village || '',
    building_no: row.building_no || '',
  }
  showCreateDialog.value = true
}

async function handleSave() {
  if (!createForm.value.name) {
    uni.showToast({ title: '请填写名称', icon: 'none' })
    return
  }
  submitting.value = true
  try {
    if (editingId.value) {
      await adminUpdateBuilding(editingId.value, createForm.value)
      uni.showToast({ title: '更新成功', icon: 'success' })
    } else {
      await adminCreateBuilding(createForm.value)
      uni.showToast({ title: '创建成功', icon: 'success' })
    }
    showCreateDialog.value = false
    await fetchBuildings()
  } catch {
    uni.showToast({ title: '操作失败', icon: 'none' })
  } finally {
    submitting.value = false
  }
}

function handleUpgrade(row) {
  uni.showActionSheet({
    itemList: ['升级为全套餐', '降级为基础套餐'],
    success: async (res) => {
      try {
        const { adminUpgradePackage } = await import('../../api')
        await adminUpgradePackage(row.id, { package: res.tapIndex === 0 ? 'full' : 'basic' })
        uni.showToast({ title: '套餐变更成功', icon: 'success' })
        await fetchBuildings()
      } catch {
        uni.showToast({ title: '操作失败', icon: 'none' })
      }
    }
  })
}

function handleCreateAdmin(row) {
  uni.showModal({
    title: '创建管理员',
    content: `为「${row.name}」创建管理员？`,
    success: async (res) => {
      if (!res.confirm) return
      try {
        await adminCreateBuildingAdmin({ building_id: row.id, username: row.name + '_admin', password: '123456' })
        uni.showToast({ title: '管理员创建成功', icon: 'success' })
      } catch {
        uni.showToast({ title: '创建失败', icon: 'none' })
      }
    }
  })
}

async function handleDelete(id) {
  uni.showModal({
    title: '确认删除',
    content: '确定删除该公寓？',
    success: async (res) => {
      if (!res.confirm) return
      try {
        await adminDeleteBuilding(id)
        uni.showToast({ title: '已删除', icon: 'success' })
        await fetchBuildings()
      } catch {
        uni.showToast({ title: '删除失败', icon: 'none' })
      }
    }
  })
}

onMounted(() => {
  if (auth.isLoggedIn) fetchBuildings()
})
</script>

<style scoped>
.page-admin-buildings { padding: 16px; min-height: 100vh; }
.login-prompt { text-align: center; padding: 80px 0; color: #999; }
.login-prompt button { margin-top: 12px; padding: 8px 24px; background: #1989fa; color: #fff; border: none; border-radius: 8px; }
.header-row { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.page-title { font-size: 20px; font-weight: 700; color: #1a1a2e; }
.add-btn { background: #1989fa; color: #fff; border: none; border-radius: 8px; padding: 8px 16px; font-size: 14px; }
.search-bar { display: flex; gap: 8px; margin-bottom: 16px; }
.search-input { flex: 1; height: 40px; border: 1px solid #dcdfe6; border-radius: 8px; padding: 0 12px; font-size: 14px; background: #fff; }
.search-btn { background: #1989fa; color: #fff; border: none; border-radius: 8px; padding: 0 16px; font-size: 14px; }
.loading-wrap, .empty-wrap { text-align: center; padding: 60px 0; color: #999; }
.building-card { background: #fff; border-radius: 12px; padding: 14px; margin-bottom: 12px; box-shadow: 0 1px 6px rgba(0,0,0,0.05); }
.card-top { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
.card-name { font-size: 16px; font-weight: 600; color: #1a1a2e; }
.pkg-tag { font-size: 11px; padding: 2px 8px; border-radius: 4px; color: #fff; }
.pkg-full { background: #409eff; }
.pkg-basic { background: #909399; }
.card-info { font-size: 13px; color: #555; line-height: 1.8; }
.vacant-hint { color: #67c23a; font-weight: 500; }
.card-actions { display: flex; gap: 6px; margin-top: 10px; flex-wrap: wrap; }
.act-btn { font-size: 12px; padding: 4px 12px; border: 1px solid #dcdfe6; border-radius: 6px; background: #fff; color: #333; }
.act-btn.danger { color: #f56c6c; border-color: #f56c6c; }
.overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); z-index: 1000; display: flex; align-items: flex-end; }
.dialog-panel { width: 100%; background: #fff; border-radius: 16px 16px 0 0; padding: 20px; max-height: 80vh; }
.dialog-title { font-size: 18px; font-weight: 700; color: #1a1a2e; display: block; margin-bottom: 16px; }
.form-group { margin-bottom: 14px; }
.form-label { font-size: 14px; color: #333; font-weight: 500; display: block; margin-bottom: 4px; }
.form-input { width: 100%; height: 40px; border: 1px solid #dcdfe6; border-radius: 8px; padding: 0 12px; font-size: 14px; background: #fff; }
.picker-val { height: 40px; line-height: 40px; border: 1px solid #dcdfe6; border-radius: 8px; padding: 0 12px; font-size: 14px; color: #333; background: #fff; }
.dialog-actions { display: flex; gap: 12px; margin-top: 20px; }
.dialog-btn { flex: 1; height: 44px; border-radius: 22px; font-size: 15px; font-weight: 600; display: flex; align-items: center; justify-content: center; }
.dialog-btn.cancel { background: #f5f5f5; color: #666; border: none; }
.dialog-btn.confirm { background: #1989fa; color: #fff; border: none; }
.dialog-btn.confirm[disabled] { opacity: 0.6; }
</style>
