<template>
  <div>
    <!-- 桌面端 -->
    <el-container v-if="!isMobile" class="desktop-layout desktop-height">
      <el-header class="layout-header header-dark header-flex">
        <h2 class="header-title" @click="$router.push('/')">🏠 圳好租 · 平台管理</h2>
        <el-menu :default-active="$route.path" mode="horizontal" :ellipsis="false" class="header-menu" router>
          <el-menu-item index="/admin/buildings" class="menu-item-light">公寓管理</el-menu-item>
          <el-menu-item index="/admin/stats" class="menu-item-light">数据看板</el-menu-item>
          <el-menu-item index="/admin/recruit" class="menu-item-light">
            <span>招商</span>
            <span v-if="recruitCount" class="recruit-badge">{{ recruitCount }}</span>
          </el-menu-item>
        </el-menu>
        <div class="layout-user user-info">
          <span class="username-text">{{ username }}</span>
          <el-button size="small" @click="authStore.logout(); router.push('/')">退出</el-button>
        </div>
      </el-header>
      <el-main class="main-content">
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
      <MobileUserMenu :show="showUserMenu" :username="username" @close="showUserMenu = false" />
      <div class="mobile-tabs">
        <div :class="['mobile-tab', { active: $route.path === '/admin/buildings' }]" @click="$router.push('/admin/buildings')">公寓管理</div>
        <div :class="['mobile-tab', { active: $route.path === '/admin/stats' }]" @click="$router.push('/admin/stats')">数据看板</div>
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
import { getUnprocessedRecruitCount } from '../api'
import { useAuthStore } from '../stores/auth'
import { useMobile } from '../composables/useMobile'
import MobileUserMenu from '../components/common/MobileUserMenu.vue'

const router = useRouter()
const authStore = useAuthStore()
const { isMobile } = useMobile()
const username = ref(authStore.username)
const showUserMenu = ref(false)
const recruitCount = ref(0)
let timer = null

async function fetchRecruitCount() {
  try {
    const res = await getUnprocessedRecruitCount()
    recruitCount.value = res.data.count || 0
  } catch {
    ElMessage.error('获取招商信息失败')
  }
}

onMounted(() => {
  fetchRecruitCount()
  timer = setInterval(() => {
    if (!document.hidden) fetchRecruitCount()
  }, 30000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.desktop-layout { display: flex; }
.desktop-height { height: 100vh; }
.header-dark { background: #1a1a2e; color: #fff; }
.header-flex { display: flex; align-items: center; padding: 0 24px; }
.header-title { font-size: 16px; margin: 0; cursor: pointer; white-space: nowrap; }
.header-menu { background: transparent; border: none; flex: 1; margin-left: 40px; }
.menu-item-light { color: rgba(255,255,255,0.8); }
.user-info { display: flex; align-items: center; gap: 10px; }
.username-text { font-size: 13px; opacity: 0.8; }
.main-content { background: #f5f7fa; padding: 24px; }

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
</style>