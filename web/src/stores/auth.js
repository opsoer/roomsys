import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('token') || '')
  const username = ref(localStorage.getItem('username') || '')
  const role = ref(localStorage.getItem('role') || '')
  const buildingId = ref(localStorage.getItem('building_id') || '')

  const isLoggedIn = computed(() => !!token.value)
  const isSuperAdmin = computed(() => role.value === 'super_admin')
  const isBuildingAdmin = computed(() => role.value === 'building_admin' || role.value === 'admin')

  function setAuth(data) {
    token.value = data.token || ''
    username.value = data.username || ''
    role.value = data.role || ''
    buildingId.value = data.building_id || ''

    localStorage.setItem('token', token.value)
    localStorage.setItem('username', username.value)
    localStorage.setItem('role', role.value)
    localStorage.setItem('building_id', buildingId.value)
  }

  function login(userData, tokenStr) {
    setAuth({
      token: tokenStr,
      username: userData.username,
      role: userData.role,
      building_id: userData.building_id,
    })
  }

  function logout() {
    token.value = ''
    username.value = ''
    role.value = ''
    buildingId.value = ''

    localStorage.removeItem('token')
    localStorage.removeItem('username')
    localStorage.removeItem('role')
    localStorage.removeItem('building_id')
  }

  function getDashboardPath() {
    if (role.value === 'super_admin') return '/admin/buildings'
    return '/landlord/rooms'
  }

  // 多标签页状态同步：storage 事件只在「其它」标签页触发，
  // 任何标签页写入/移除 localStorage 后，其余标签页的 store 状态随之更新。
  function handleStorageChange(e) {
    if (e.key === 'token') {
      token.value = e.newValue || ''
    } else if (e.key === 'username') {
      username.value = e.newValue || ''
    } else if (e.key === 'role') {
      role.value = e.newValue || ''
    } else if (e.key === 'building_id') {
      buildingId.value = e.newValue || ''
    }
  }

  // storage 事件只在其它标签页触发；store 是应用级单例、永不销毁，监听注册一次即
  // 随应用生命周期存活。注意：不要在 setup store 里调 onUnmounted 注销——处于组件
  // setup 内首次创建 store 时，钩子会挂到该组件上，组件卸载反而误删监听；
  // 在组件外创建则只产生 Vue 警告并空操作。两种结果都不符合需求，保持不注销。
  if (typeof window !== 'undefined') {
    window.addEventListener('storage', handleStorageChange)
  }

  return {
    token,
    username,
    role,
    buildingId,
    isLoggedIn,
    isSuperAdmin,
    isBuildingAdmin,
    setAuth,
    login,
    logout,
    getDashboardPath,
  }
})
