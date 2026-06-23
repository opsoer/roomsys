import axios from 'axios'
import { ElMessage } from 'element-plus'
import router from '../router'

const api = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

api.interceptors.request.use(config => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

api.interceptors.response.use(
  res => res,
  err => {
    if (err.response?.status === 401) {
      localStorage.removeItem('token')
      localStorage.removeItem('user')
      localStorage.removeItem('username')
      localStorage.removeItem('role')
      localStorage.removeItem('building_id')
      const path = router.currentRoute?.value?.path || ''
      if (path.startsWith('/landlord')) {
        router.push('/landlord/login')
      } else {
        router.push('/login')
      }
      ElMessage.error('登录已过期，请重新登录')
    } else if (err.response?.data?.error) {
      ElMessage.error(err.response.data.error)
    } else {
      ElMessage.error('请求失败')
    }
    return Promise.reject(err)
  },
)

// ===== 认证 =====
export function login(username, password) {
  return api.post('/auth/login', { username, password })
}

export function getMe() {
  return api.get('/auth/me')
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

export function adminGetBuildings() {
  return api.get('/admin/buildings')
}

export function adminUpdateBuilding(id, data) {
  return api.put(`/admin/buildings/${id}`, data)
}

export function adminDeleteBuilding(id) {
  return api.delete(`/admin/buildings/${id}`)
}

export function adminCreateAdmin(data) {
  return api.post('/admin/auth/create-admin', data)
}

export function adminCreateBuildingAdmin(data) {
  return api.post('/admin/auth/create-building-admin', data)
}

export function adminListUsers() {
  return api.get('/admin/auth/users')
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

export function getBuildingStats(params) {
  return api.get('/building/stats', { params })
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

export function buildingUpdateContractEndDate(id, data) {
  return api.put(`/building/rooms/${id}/contract`, data)
}

export function buildingGetRoomContract(id) {
  return api.get(`/building/rooms/${id}/contract`)
}

export function buildingUploadMedia(roomId, file) {
  const fd = new FormData()
  fd.append('file', file)
  return api.post(`/building/rooms/${roomId}/media`, fd, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}

export function buildingDeleteMedia(roomId, mediaId) {
  return api.delete(`/building/rooms/${roomId}/media/${mediaId}`)
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
  return api.get('/building/bills/stats', { params: { month, year } })
}

export function buildingGetBillTrend(params) {
  return api.get('/building/bills/trend', { params })
}

export function buildingGetDividendPredict(params) {
  return api.get('/building/dividends/predict', { params })
}

export function buildingGetDividends() {
  return api.get('/building/dividends')
}

export function buildingCalculateDividend(month) {
  return api.get('/building/dividends/calculate', { params: { month } })
}

export function buildingSettleDividend(month) {
  return api.post('/building/dividends/settle', { month })
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

export function buildingGetTasks(status) {
  return api.get('/building/tasks', { params: { status } })
}

export function buildingProcessTask(id, data) {
  return api.post(`/building/tasks/${id}/process`, data)
}

export function buildingCompleteTask(id) {
  return api.put(`/building/tasks/${id}/complete`)
}

export function buildingDeleteTask(id) {
  return api.delete(`/building/tasks/${id}`)
}

export function buildingGetSystemTime() {
  return api.get('/building/system/time')
}

export function buildingSetSystemTime(offsetSeconds) {
  return api.post('/building/system/time', { offset_seconds: offsetSeconds })
}

export function buildingGetUsers() {
  return api.get('/building/auth/users')
}

export function buildingCreateAdmin(data) {
  return api.post('/building/auth/create-admin', data)
}

export default api
