<template>
  <el-container style="height: 100vh">
    <el-header class="layout-header">
      <h2 class="layout-logo" @click="$router.push('/')">
        ☀️ 圳好租
      </h2>
      <el-menu
        :default-active="$route.path"
        mode="horizontal"
        :ellipsis="false"
        class="layout-menu"
        router
      >
        <el-menu-item index="/rooms">
          <el-icon><HomeFilled /></el-icon>
          <span class="nav-text">租房管理</span>
        </el-menu-item>
        <el-menu-item v-if="isAdmin" index="/bills">
          <el-icon><Money /></el-icon>
          <span class="nav-text">财务管理</span>
        </el-menu-item>
        <el-menu-item v-if="isAdmin" index="/dividends">
          <el-icon><Coin /></el-icon>
          <span class="nav-text">分红管理</span>
        </el-menu-item>
        <el-menu-item v-if="isAdmin" index="/tasks">
          <el-icon><List /></el-icon>
          <span class="nav-text">代办事项</span>
        </el-menu-item>
        <el-menu-item v-if="isSuperAdmin" index="/users">
          <el-icon><User /></el-icon>
          <span class="nav-text">管理员管理</span>
        </el-menu-item>
      </el-menu>
      <div class="layout-user">
        <template v-if="loggedIn">
          <el-tag v-if="isAdmin" size="small" type="warning">管理员</el-tag>
          <span class="user-text">{{ username }}</span>
          <el-button size="small" @click="logout">退出</el-button>
        </template>
        <template v-else>
          <el-button size="small" type="primary" @click="$router.push('/login')">登录</el-button>
        </template>
      </div>
      <el-button class="mobile-menu-btn" text @click="showMobileMenu = !showMobileMenu">
        <el-icon :size="22"><Menu /></el-icon>
      </el-button>
    </el-header>
    <div v-if="showMobileMenu" class="mobile-menu-overlay" @click="showMobileMenu = false">
      <div class="mobile-menu-panel" @click.stop>
        <div class="mobile-menu-header">
          <h3>☀️ 圳好租</h3>
          <el-button text @click="showMobileMenu = false"><el-icon><Close /></el-icon></el-button>
        </div>
        <el-menu :default-active="$route.path" router @select="showMobileMenu = false">
          <el-menu-item index="/rooms">
            <el-icon><HomeFilled /></el-icon>租房管理
          </el-menu-item>
          <el-menu-item v-if="isAdmin" index="/bills">
            <el-icon><Money /></el-icon>财务管理
          </el-menu-item>
          <el-menu-item v-if="isAdmin" index="/dividends">
            <el-icon><Coin /></el-icon>分红管理
          </el-menu-item>
          <el-menu-item v-if="isAdmin" index="/tasks">
            <el-icon><List /></el-icon>代办事项
          </el-menu-item>
          <el-menu-item v-if="isSuperAdmin" index="/users">
            <el-icon><User /></el-icon>管理员管理
          </el-menu-item>
        </el-menu>
        <div class="mobile-menu-user">
          <template v-if="loggedIn">
            <span>{{ username }}</span>
            <el-button size="small" @click="logout">退出</el-button>
          </template>
          <template v-else>
            <el-button size="small" type="primary" @click="$router.push('/login'); showMobileMenu = false">登录</el-button>
          </template>
        </div>
      </div>
    </div>
    <el-main style="background: #f5f7fa; padding: 20px">
      <router-view v-slot="{ Component }">
        <transition name="slide-fade" mode="out-in">
          <component :is="Component" />
        </transition>
      </router-view>
    </el-main>
  </el-container>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Menu, Close, List } from '@element-plus/icons-vue'

const showMobileMenu = ref(false)

const router = useRouter()
const username = ref(localStorage.getItem('username') || '')
const token = computed(() => localStorage.getItem('token'))
const loggedIn = computed(() => !!token.value)
const isAdmin = computed(() => {
  const role = localStorage.getItem('role')
  return role === 'admin' || role === 'super_admin'
})
const isSuperAdmin = computed(() => localStorage.getItem('role') === 'super_admin')

function logout() {
  localStorage.removeItem('token')
  localStorage.removeItem('user')
  localStorage.removeItem('username')
  localStorage.removeItem('role')
  router.push('/')
}
</script>

<style scoped>
.layout-header {
  background: #fff; color: #333; display: flex; align-items: center; padding: 0 24px;
  border-bottom: 1px solid #eee; box-shadow: 0 1px 4px rgba(0,0,0,0.04);
  position: relative; z-index: 200;
}
.layout-logo {
  margin-right: 40px; font-size: 18px; cursor: pointer;
  background: linear-gradient(135deg, #e6a23c, #f56c6c);
  -webkit-background-clip: text; -webkit-text-fill-color: transparent; background-clip: text;
  white-space: nowrap;
}
.layout-menu {
  background: transparent; border: none; flex: 1; min-width: 0;
}
.layout-menu .el-menu-item { padding: 0 16px; }
.layout-user { display: flex; align-items: center; gap: 10px; white-space: nowrap; }
.user-text { font-size: 14px; }
.mobile-menu-btn { display: none; }
.mobile-menu-overlay {
  display: none; position: fixed; inset: 0; z-index: 500; background: rgba(0,0,0,0.4);
}
.mobile-menu-panel {
  width: 260px; height: 100%; background: #fff; padding: 16px 0;
  display: flex; flex-direction: column;
}
.mobile-menu-header {
  display: flex; align-items: center; justify-content: space-between; padding: 0 16px 12px; border-bottom: 1px solid #eee;
}
.mobile-menu-header h3 { margin: 0; font-size: 16px; }
.mobile-menu-user { padding: 16px; border-top: 1px solid #eee; display: flex; align-items: center; gap: 8px; margin-top: auto; }
@media (max-width: 768px) {
  .layout-logo { font-size: 15px; margin-right: 12px; }
  .layout-menu { display: none !important; }
  .layout-user { display: none !important; }
  .mobile-menu-btn { display: flex; }
  .mobile-menu-overlay { display: block; }
  .nav-text { display: none; }
}
</style>
