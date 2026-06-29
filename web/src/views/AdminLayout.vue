<template>
  <div>
    <!-- 桌面端 -->
    <el-container v-if="!isMobile" class="desktop-layout" style="height: 100vh">
      <el-header class="layout-header" style="background: #1a1a2e; color: #fff; display: flex; align-items: center; padding: 0 24px;">
        <h2 style="font-size: 16px; margin: 0; cursor: pointer; white-space: nowrap;" @click="$router.push('/')">🏠 圳好租 · 平台管理</h2>
        <el-menu :default-active="$route.path" mode="horizontal" :ellipsis="false" style="background: transparent; border: none; flex: 1; margin-left: 40px;" router>
          <el-menu-item index="/admin/buildings" style="color: rgba(255,255,255,0.8);">公寓管理</el-menu-item>
          <el-menu-item index="/admin/recruit" style="color: rgba(255,255,255,0.8);">
            <span>招商</span>
            <span v-if="recruitCount" class="recruit-badge">{{ recruitCount }}</span>
          </el-menu-item>
        </el-menu>
        <div class="layout-user" style="display: flex; align-items: center; gap: 10px;">
          <span style="font-size: 13px; opacity: 0.8;">{{ username }}</span>
          <el-button size="small" @click="logout">退出</el-button>
        </div>
      </el-header>
      <el-main style="background: #f5f7fa; padding: 24px;">
        <router-view />
      </el-main>
    </el-container>

    <!-- 移动端 -->
    <div v-else class="mobile-layout">
      <div class="mobile-header">
        <van-icon name="arrow-left" size="20" @click="$router.push('/')" />
        <h2 class="mobile-title">平台管理</h2>
        <van-icon name="friends-o" size="20" @click="showUserMenu = !showUserMenu" />
      </div>
      <div v-if="showUserMenu" class="mobile-user-dropdown" @click="showUserMenu = false">
        <div class="mobile-user-card" @click.stop>
          <div class="mobile-user-name">{{ username }}</div>
          <van-button round block type="danger" size="small" @click="logout">退出登录</van-button>
        </div>
      </div>
      <div class="mobile-tabs">
        <div :class="['mobile-tab', { active: $route.path === '/admin/buildings' }]" @click="$router.push('/admin/buildings')">公寓管理</div>
        <div :class="['mobile-tab', { active: $route.path === '/admin/recruit' }]" @click="$router.push('/admin/recruit')">
          招商
          <span v-if="recruitCount" class="recruit-badge-mobile">{{ recruitCount }}</span>
        </div>
      </div>
      <div class="mobile-body">
        <router-view />
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { showToast } from 'vant'
import api from '../api'

const router = useRouter()
const username = ref(localStorage.getItem('username') || '')
const showUserMenu = ref(false)
const isMobile = ref(window.innerWidth <= 768)
function handleResize() {
  isMobile.value = window.innerWidth <= 768
}
const recruitCount = ref(0)
let timer = null

async function fetchRecruitCount() {
  const token = localStorage.getItem('token')
  if (!token) return
  try {
    const res = await api.get('/admin/recruit/unprocessed-count')
    recruitCount.value = res.data.count || 0
  } catch {
    ElMessage.error('获取招商信息失败')
  }
}

function logout() {
  localStorage.removeItem('token')
  localStorage.removeItem('username')
  localStorage.removeItem('role')
  localStorage.removeItem('building_id')
  localStorage.removeItem('user')
  showToast('已退出')
  router.push('/')
}

onMounted(() => {
  window.addEventListener('resize', handleResize)
  fetchRecruitCount()
  timer = setInterval(fetchRecruitCount, 30000)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.desktop-layout { display: flex; }
.mobile-layout { display: flex; flex-direction: column; min-height: 100vh; }

.mobile-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  background: #1a1a2e;
  color: #fff;
  position: sticky;
  top: 0;
  z-index: 100;
}
.mobile-title {
  font-size: 16px;
  font-weight: 600;
}
.mobile-tabs {
  display: flex;
  background: #fff;
  border-bottom: 1px solid #f0f0f0;
  position: sticky;
  top: 48px;
  z-index: 99;
}
.mobile-tab {
  flex: 1;
  text-align: center;
  padding: 12px 0;
  font-size: 14px;
  color: #666;
  cursor: pointer;
  border-bottom: 2px solid transparent;
  transition: all 0.2s;
}
.mobile-tab.active {
  color: #e6a23c;
  border-bottom-color: #e6a23c;
  font-weight: 600;
}
.recruit-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: #f56c6c;
  color: #fff;
  font-size: 11px;
  min-width: 18px;
  height: 18px;
  border-radius: 9px;
  padding: 0 5px;
  margin-left: 4px;
  line-height: 1;
}
.recruit-badge-mobile {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: #f56c6c;
  color: #fff;
  font-size: 10px;
  min-width: 16px;
  height: 16px;
  border-radius: 8px;
  padding: 0 4px;
  margin-left: 3px;
  line-height: 1;
  vertical-align: top;
}
.mobile-body {
  min-height: calc(100vh - 48px);
  background: #f5f7fa;
  overflow-y: auto;
}
.mobile-user-dropdown {
  position: fixed;
  inset: 0;
  z-index: 500;
  background: rgba(0,0,0,0.3);
}
.mobile-user-card {
  position: absolute;
  right: 12px;
  top: 56px;
  background: #fff;
  border-radius: 10px;
  padding: 16px;
  min-width: 180px;
  box-shadow: 0 4px 16px rgba(0,0,0,0.15);
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.mobile-user-name {
  font-size: 14px;
  color: #333;
  font-weight: 500;
  text-align: center;
}


</style>