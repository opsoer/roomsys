<template>
  <div class="layout-wrap">
    <!-- 桌面端布局 -->
    <el-container v-if="!isMobile" class="desktop-layout">
      <el-header class="layout-header">
        <h2 class="layout-logo" @click="goToBuildingPage">
          🏠 {{ buildingName || '公寓管理' }}
        </h2>
        <el-menu
          :default-active="$route.path"
          mode="horizontal"
          :ellipsis="false"
          class="layout-menu"
          router
        >
          <el-menu-item index="/landlord/rooms">
            <el-icon><HomeFilled /></el-icon>
            <span class="nav-text">租房管理</span>
          </el-menu-item>
          <el-menu-item v-if="isBuildingAdmin" index="/landlord/bills">
            <el-icon><Money /></el-icon>
            <span class="nav-text">财务管理</span>
          </el-menu-item>
          <el-menu-item v-if="isBuildingAdmin" index="/landlord/dividends">
            <el-icon><Coin /></el-icon>
            <span class="nav-text">分红管理</span>
          </el-menu-item>
          <el-menu-item v-if="isBuildingAdmin" index="/landlord/tasks">
            <el-icon><List /></el-icon>
            <span class="nav-text">代办事项</span>
          </el-menu-item>
          <el-menu-item v-if="isBuildingAdmin" index="/landlord/users">
            <el-icon><User /></el-icon>
            <span class="nav-text">管理员管理</span>
          </el-menu-item>
          <el-menu-item index="/landlord/settings">
            <el-icon><Setting /></el-icon>
            <span class="nav-text">公寓设置</span>
          </el-menu-item>
        </el-menu>
        <div class="layout-user">
          <template v-if="loggedIn">
            <el-tag v-if="isBuildingAdmin" size="small" type="warning">管理员</el-tag>
            <span class="user-text">{{ username }}</span>
            <el-button size="small" @click="logout">退出</el-button>
          </template>
        </div>
      </el-header>
      <el-main class="layout-main">
        <router-view v-slot="{ Component }">
          <transition name="slide-fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </el-main>
    </el-container>

    <!-- 移动端布局 -->
    <div v-else class="mobile-layout">
      <div class="mobile-header">
        <van-icon name="bars" size="22" @click="showMobileMenu = true" />
        <h2 class="mobile-title" @click="goToBuildingPage">
          🏠 {{ buildingName || '公寓管理' }}
        </h2>
        <van-icon name="friends-o" size="20" @click="showUserMenu = !showUserMenu" />
      </div>
      <div v-if="showUserMenu" class="mobile-user-dropdown" @click="showUserMenu = false">
        <div class="mobile-user-card" @click.stop>
          <div class="mobile-user-name">{{ username }}</div>
          <van-button round block type="danger" size="small" @click="logout">退出登录</van-button>
        </div>
      </div>
      <div class="mobile-body">
        <router-view v-slot="{ Component }">
          <transition name="slide-fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </div>

      <!-- 侧滑菜单 -->
      <van-overlay :show="showMobileMenu" @click="showMobileMenu = false" z-index="500">
        <div class="mobile-menu-panel" :class="{ 'menu-open': showMobileMenu }" @click.stop>
          <div class="mobile-menu-header">
            <h3>🏠 {{ buildingName || '公寓管理' }}</h3>
            <van-icon name="cross" size="20" @click="showMobileMenu = false" />
          </div>
          <div class="mobile-menu-body">
            <van-cell
              title="租房管理"
              icon="home-o"
              :to="'/landlord/rooms'"
              @click="showMobileMenu = false"
              :class="{ 'menu-active': $route.path.startsWith('/landlord/rooms') }"
            />
            <van-cell
              v-if="isBuildingAdmin"
              title="财务管理"
              icon="gold-coin-o"
              :to="'/landlord/bills'"
              @click="showMobileMenu = false"
              :class="{ 'menu-active': $route.path.startsWith('/landlord/bills') }"
            />
            <van-cell
              v-if="isBuildingAdmin"
              title="分红管理"
              icon="chart-treemap"
              :to="'/landlord/dividends'"
              @click="showMobileMenu = false"
              :class="{ 'menu-active': $route.path.startsWith('/landlord/dividends') }"
            />
            <van-cell
              v-if="isBuildingAdmin"
              title="代办事项"
              icon="todo-o"
              :to="'/landlord/tasks'"
              @click="showMobileMenu = false"
              :class="{ 'menu-active': $route.path.startsWith('/landlord/tasks') }"
            />
            <van-cell
              v-if="isBuildingAdmin"
              title="管理员管理"
              icon="contact"
              :to="'/landlord/users'"
              @click="showMobileMenu = false"
              :class="{ 'menu-active': $route.path.startsWith('/landlord/users') }"
            />
            <van-cell
              title="公寓设置"
              icon="setting-o"
              :to="'/landlord/settings'"
              @click="showMobileMenu = false"
              :class="{ 'menu-active': $route.path.startsWith('/landlord/settings') }"
            />
          </div>
          <div class="mobile-menu-footer">
            <span class="mobile-user">{{ username }}</span>
            <van-tag v-if="isBuildingAdmin" type="warning" size="small">管理员</van-tag>
          </div>
        </div>
      </van-overlay>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { showToast } from 'vant'
import { getBuildingInfo } from '../api'

const showMobileMenu = ref(false)
const showUserMenu = ref(false)
const router = useRouter()
const buildingName = ref('')

const isMobile = ref(window.innerWidth <= 768)
function handleResize() {
  isMobile.value = window.innerWidth <= 768
}

const username = ref(localStorage.getItem('username') || '')
const token = computed(() => localStorage.getItem('token'))
const loggedIn = computed(() => !!token.value)
const role = computed(() => localStorage.getItem('role'))
const isBuildingAdmin = computed(() => role.value === 'building_admin' || role.value === 'admin' || role.value === 'super_admin')

function goToBuildingPage() {
  const bid = localStorage.getItem('building_id')
  if (bid) {
    const url = router.resolve({ name: 'BuildingPublic', params: { id: bid } }).href
    window.open(url, '_blank')
  } else {
    router.push('/')
  }
}

function logout() {
  localStorage.removeItem('token')
  localStorage.removeItem('username')
  localStorage.removeItem('role')
  localStorage.removeItem('building_id')
  showToast('已退出')
  router.push('/')
}

onMounted(() => {
  window.addEventListener('resize', handleResize)
  ;(async () => {
  const bid = localStorage.getItem('building_id')
  if (!bid || bid === '0' || bid === 'null') {
    const r = localStorage.getItem('role')
    if (r === 'super_admin') {
      ElMessage.info('请先在平台后台创建或选择一个公寓')
      router.push('/admin/buildings')
      return
    }
    buildingName.value = '未关联公寓'
    return
  }
  try {
    const res = await getBuildingInfo()
    buildingName.value = res.data.building?.name || ''
  } catch {
    ElMessage.error('获取公寓信息失败')
  }
  })()
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
})
</script>

<style scoped>
.desktop-layout { display: flex; }
.mobile-layout { display: flex; flex-direction: column; min-height: 100vh; }

.layout-header {
  background: #fff; color: #333; display: flex; align-items: center; padding: 0 24px;
  border-bottom: 1px solid #eee; box-shadow: 0 1px 4px rgba(0,0,0,0.04);
  position: relative; z-index: 200;
}
.layout-logo {
  margin-right: 40px; font-size: 16px; cursor: pointer; white-space: nowrap;
  background: linear-gradient(135deg, #e6a23c, #f56c6c);
  -webkit-background-clip: text; -webkit-text-fill-color: transparent; background-clip: text;
}
.layout-menu { background: transparent; border: none; flex: 1; min-width: 0; }
.layout-menu .el-menu-item { padding: 0 14px; }
.layout-user { display: flex; align-items: center; gap: 10px; white-space: nowrap; }
.user-text { font-size: 14px; }
.layout-main { background: #f5f7fa; padding: 20px; }

/* 移动端布局 */
.mobile-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  background: #fff;
  border-bottom: 1px solid #f0f0f0;
  position: sticky;
  top: 0;
  z-index: 100;
}
.mobile-title {
  font-size: 16px;
  font-weight: 600;
  cursor: pointer;
  background: linear-gradient(135deg, #e6a23c, #f56c6c);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}
.mobile-body {
  min-height: calc(100vh - 50px);
  background: #f5f7fa;
  overflow-y: auto;
}
.mobile-menu-panel {
  width: 280px;
  height: 100%;
  background: #fff;
  display: flex;
  flex-direction: column;
  transition: transform 0.3s ease;
}
.mobile-menu-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px 16px 12px;
  border-bottom: 1px solid #f0f0f0;
}
.mobile-menu-header h3 {
  margin: 0;
  font-size: 16px;
}
.mobile-menu-body {
  flex: 1;
  overflow-y: auto;
  padding: 8px 0;
}
.mobile-menu-body :deep(.van-cell) {
  font-size: 15px;
  padding: 14px 16px;
}
.mobile-menu-body :deep(.van-cell__title) {
  font-size: 15px;
}
.mobile-menu-active,
.mobile-menu-body :deep(.menu-active .van-cell__title) {
  color: #e6a23c;
  font-weight: 600;
}
.mobile-menu-footer {
  padding: 16px;
  border-top: 1px solid #f0f0f0;
  display: flex;
  align-items: center;
  gap: 8px;
}
.mobile-user {
  font-size: 14px;
  color: #666;
}
.mobile-user-dropdown {
  position: fixed;
  inset: 0;
  z-index: 600;
  background: rgba(0,0,0,0.3);
}
.mobile-user-card {
  position: absolute;
  right: 12px;
  top: 56px;
  background: #fff;
  border-radius: 10px;
  padding: 16px;
  min-width: 160px;
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