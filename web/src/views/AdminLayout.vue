<template>
  <el-container style="height: 100vh">
    <el-header class="layout-header" style="background: #1a1a2e; color: #fff; display: flex; align-items: center; padding: 0 24px;">
      <h2 style="font-size: 16px; margin: 0; cursor: pointer;" @click="$router.push('/')">☀️ 邻租 · 平台管理</h2>
      <el-menu :default-active="$route.path" mode="horizontal" :ellipsis="false" style="background: transparent; border: none; flex: 1; margin-left: 40px;" router>
        <el-menu-item index="/admin/buildings" style="color: rgba(255,255,255,0.8);">公寓管理</el-menu-item>
      </el-menu>
      <div style="display: flex; align-items: center; gap: 10px;">
        <span style="font-size: 13px; opacity: 0.8;">{{ username }}</span>
        <el-button size="small" @click="logout">退出</el-button>
      </div>
    </el-header>
    <el-main style="background: #f5f7fa; padding: 24px;">
      <router-view />
    </el-main>
  </el-container>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'

const router = useRouter()
const username = ref(localStorage.getItem('username') || '')

function logout() {
  localStorage.removeItem('token')
  localStorage.removeItem('username')
  localStorage.removeItem('role')
  localStorage.removeItem('building_id')
  router.push('/')
}
</script>
