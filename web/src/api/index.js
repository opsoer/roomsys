import axios from 'axios'
import { ElMessage } from 'element-plus'
import router from '../router'
import { useAuthStore } from '../stores/auth'

const api = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

const uploadApi = axios.create({
  baseURL: '/api',
  timeout: 300000,
})

api.interceptors.request.use(config => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  config.headers['X-Requested-With'] = 'XMLHttpRequest'
  return config
})

uploadApi.interceptors.request.use(config => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  config.headers['X-Requested-With'] = 'XMLHttpRequest'
  return config
})

api.interceptors.response.use(
  res => {
    if (res.data && res.data.code === 0 && res.data.data !== undefined && res.data.data !== null) {
      res.data = res.data.data
    }
    return res
  },
  err => {
    const data = err.response?.data
    const msg = data?.message || data?.error
    if (err.response?.status === 401) {
      const isLoginPage = window.location.pathname === '/login'
      if (msg) ElMessage.error(msg)
      if (!isLoginPage) {
        const authStore = useAuthStore()
        authStore.logout()
        router.push('/login')
      }
    } else if (data?.code === 1006) {
      return Promise.reject(err)
    } else if (msg) {
      ElMessage.error(msg)
    } else {
      ElMessage.error('请求失败')
    }
    return Promise.reject(err)
  },
)

uploadApi.interceptors.response.use(
  res => {
    if (res.data && res.data.code === 0 && res.data.data !== undefined && res.data.data !== null) {
      res.data = res.data.data
    }
    return res
  },
  err => {
    const msg = err.response?.data?.message || err.response?.data?.error
    if (msg) {
      ElMessage.error(msg)
    } else {
      ElMessage.error('上传失败')
    }
    return Promise.reject(err)
  },
)

// ===== 认证 =====
export function login(username, password) {
  return api.post('/auth/login', { username, password })
}

// ===== 公共 - 建筑浏览 =====
export function getBuildings(params) {
  return api.get('/buildings', { params })
}

export function getBuildingDetail(id) {
  return api.get(`/buildings/${id}`)
}

export function getBuildingRooms(id, params) {
  return api.get(`/buildings/${id}/rooms`, { params })
}

export function getPublicRoom(buildingId, roomId) {
  return api.get(`/buildings/${buildingId}/rooms/${roomId}`)
}

export function getPublicRoomContract(buildingId, roomId) {
  return api.get(`/buildings/${buildingId}/rooms/${roomId}/contract`)
}

export function getDistricts() {
  return api.get('/buildings/districts')
}

// ===== 平台管理员 =====
export function adminCreateBuilding(data) {
  return api.post('/admin/buildings', data)
}

export function adminGetBuildings(params) {
  return api.get('/admin/buildings', { params })
}

export function adminUpdateBuilding(id, data) {
  return api.put(`/admin/buildings/${id}`, data)
}

export function adminDeleteBuilding(id) {
  return api.delete(`/admin/buildings/${id}`)
}

export function adminUpgradePackage(id, data) {
  return api.put(`/admin/buildings/${id}/package`, data)
}

export function adminCreateBuildingAdmin(data) {
  return api.post('/admin/auth/create-building-admin', data)
}

export function adminGetSystemTime() {
  return api.get('/admin/system/time')
}

export function adminSetSystemTime(offsetSeconds) {
  return api.post('/admin/system/time', { offset_seconds: offsetSeconds })
}

export function adminUpdateUser(id, data) {
  return api.put(`/admin/auth/users/${id}`, data)
}

export function adminDeleteUser(id) {
  return api.delete(`/admin/auth/users/${id}`)
}

// ===== 公寓管理后台 =====
export function getBuildingInfo() {
  return api.get('/building/info')
}

export function updateBuildingInfo(data) {
  return api.put('/building/info', data)
}

export function buildingGetRooms(params) {
  return api.get('/building/rooms', { params })
}

export function buildingCreateRoom(data) {
  return api.post('/building/rooms', data)
}

export function buildingGetRoom(id) {
  return api.get(`/building/rooms/${id}`)
}

export function buildingUpdateRoom(id, data) {
  return api.put(`/building/rooms/${id}`, data)
}

export function buildingDeleteRoom(id) {
  return api.delete(`/building/rooms/${id}`)
}

export function buildingUpdateRoomStatus(id, data) {
  return api.put(`/building/rooms/${id}/status`, data)
}

export function buildingRenewContract(id, data) {
  return api.put(`/building/rooms/${id}/contract`, data)
}

export function buildingDeleteMedia(roomId, mediaId) {
  return api.delete(`/building/rooms/${roomId}/media/${mediaId}`)
}

export function buildingUploadCover(formData) {
  return uploadApi.post('/building/cover', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}

export function buildingGetBills(params) {
  return api.get('/building/bills', { params })
}

export function buildingCreateBill(data) {
  return api.post('/building/bills', data)
}

export function buildingUpdateBill(id, data) {
  return api.put(`/building/bills/${id}`, data)
}

export function buildingDeleteBill(id) {
  return api.delete(`/building/bills/${id}`)
}

export function buildingGetBillStats(month, year) {
  const params = {}
  if (month != null) params.month = month
  if (year != null) params.year = year
  return api.get('/building/bills/stats', { params })
}

export function buildingGetBillTrend(params) {
  return api.get('/building/bills/trend', { params })
}

export function buildingGetDividendPredict(params) {
  return api.get('/building/dividends/predict', { params })
}

export function buildingGetDividends(page = 1, pageSize = 20) {
  return api.get('/building/dividends', { params: { page, page_size: pageSize } })
}

export function buildingCalculateDividend(month) {
  return api.get('/building/dividends/calculate', { params: { month } })
}

export function buildingGetShareholders() {
  return api.get('/building/dividends/shareholders')
}

export function buildingCreateShareholder(data) {
  return api.post('/building/dividends/shareholders', data)
}

export function buildingUpdateShareholder(id, data) {
  return api.put(`/building/dividends/shareholders/${id}`, data)
}

export function buildingDeleteShareholder(id) {
  return api.delete(`/building/dividends/shareholders/${id}`)
}

export function buildingGetTasks(status, page = 1, pageSize = 20) {
  return api.get('/building/tasks', { params: { status, page, page_size: pageSize } })
}

export function buildingProcessTask(id, data) {
  return api.post(`/building/tasks/${id}/process`, data)
}

export function buildingGetUsers() {
  return api.get('/building/auth/users')
}

export function buildingCreateAdmin(data) {
  return api.post('/building/auth/create-admin', data)
}

// ===== 招募 =====
export function getRecruitList() {
  return api.get('/admin/recruit/list')
}

export function processRecruit(id) {
  return api.put(`/admin/recruit/process/${id}`)
}

export function submitRecruit(data) {
  return api.post('/recruit/submit', data)
}

export function getUnprocessedRecruitCount() {
  return api.get('/admin/recruit/unprocessed-count')
}

export function buildingUploadMedia(roomId, formData, onProgress) {
  return uploadApi.post(`/building/rooms/${roomId}/media`, formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
    onUploadProgress: onProgress,
  })
}

export function getUploadToken(params) {
  return api.get('/building/media/upload-token', { params })
}

export function confirmMediaUpload(roomId, data) {
  return uploadApi.post(`/building/rooms/${roomId}/media/confirm`, data, {
    headers: { 'Content-Type': 'application/json' },
  })
}

export function getConfig() {
  return api.get('/config')
}

export default api
