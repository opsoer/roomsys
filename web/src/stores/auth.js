import { defineStore } from 'pinia'
import { ref, computed, onUnmounted } from 'vue'

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

  if (typeof window !== 'undefined') {
    window.addEventListener('storage', handleStorageChange)
  }

  onUnmounted(() => {
    if (typeof window !== 'undefined') {
      window.removeEventListener('storage', handleStorageChange)
    }
  })

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
