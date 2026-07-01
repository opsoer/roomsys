import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('token') || '')
  const refreshToken = ref(localStorage.getItem('refreshToken') || '')
  const username = ref(localStorage.getItem('username') || '')
  const role = ref(localStorage.getItem('role') || '')
  const buildingId = ref(localStorage.getItem('building_id') || '')

  const isLoggedIn = computed(() => !!token.value)
  const isSuperAdmin = computed(() => role.value === 'super_admin')
  const isBuildingAdmin = computed(() => role.value === 'building_admin' || role.value === 'admin')

  function setAuth(data) {
    token.value = data.token || ''
    refreshToken.value = data.refreshToken || ''
    username.value = data.username || ''
    role.value = data.role || ''
    buildingId.value = data.building_id || ''

    localStorage.setItem('token', token.value)
    localStorage.setItem('refreshToken', refreshToken.value)
    localStorage.setItem('username', username.value)
    localStorage.setItem('role', role.value)
    localStorage.setItem('building_id', buildingId.value)
  }

  function login(userData, tokenStr, refreshTokenStr) {
    setAuth({
      token: tokenStr,
      refreshToken: refreshTokenStr,
      username: userData.username,
      role: userData.role,
      building_id: userData.building_id,
    })
  }

  function logout() {
    token.value = ''
    refreshToken.value = ''
    username.value = ''
    role.value = ''
    buildingId.value = ''

    localStorage.removeItem('token')
    localStorage.removeItem('refreshToken')
    localStorage.removeItem('username')
    localStorage.removeItem('role')
    localStorage.removeItem('building_id')
  }

  function getDashboardPath() {
    if (role.value === 'super_admin') return '/admin/buildings'
    return '/landlord/rooms'
  }

  return {
    token,
    refreshToken,
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
