const BASE_URL = 'https://your-api-domain.com/api'

function getToken() {
  try {
    return uni.getStorageSync('token') || ''
  } catch {
    return ''
  }
}

function request(method, url, data, opts = {}) {
  return new Promise((resolve, reject) => {
    const token = getToken()
    const header = {
      'X-Requested-With': 'XMLHttpRequest',
    }
    if (token) header['Authorization'] = `Bearer ${token}`
    if (opts.contentType) header['Content-Type'] = opts.contentType

    // Handle FormData for upload
    const isUpload = opts.isUpload
    const reqOpts = {
      url: BASE_URL + url,
      method,
      header,
      timeout: opts.timeout || 30000,
      success: (res) => {
        const data = res.data
        if (data && data.code === 0 && data.data !== undefined && data.data !== null) {
          res.data = data.data
        }
        resolve(res)
      },
      fail: (err) => {
        reject(err)
      },
      complete: () => {}
    }
    if (isUpload) {
      reqOpts.formData = data
      uni.uploadFile(reqOpts)
      return
    }
    if (opts.params) reqOpts.data = opts.params
    if (data) reqOpts.data = data
    uni.request(reqOpts)
  })
}

// 上传文件封装
function uploadFile(url, filePath, formData = {}, onProgress) {
  return new Promise((resolve, reject) => {
    const token = getToken()
    const uploadTask = uni.uploadFile({
      url: BASE_URL + url,
      filePath,
      name: 'file',
      formData,
      header: {
        Authorization: `Bearer ${token}`,
        'X-Requested-With': 'XMLHttpRequest',
      },
      success: (res) => {
        try {
          const data = JSON.parse(res.data)
          if (data && data.code === 0 && data.data !== undefined && data.data !== null) {
            res.data = data.data
          } else {
            res.data = data
          }
        } catch (e) {}
        resolve(res)
      },
      fail: (err) => {
        reject(err)
      },
    })
    if (onProgress) {
      uploadTask.onProgressUpdate(onProgress)
    }
  })
}

// ===== 认证 =====
export function login(username, password) {
  return request('POST', '/auth/login', { username, password })
}

// ===== 公共 - 建筑浏览 =====
export function getBuildings(params) {
  return request('GET', '/buildings', null, { params })
}

export function getBuildingDetail(id) {
  return request('GET', `/buildings/${id}`)
}

export function getBuildingRooms(id, params) {
  return request('GET', `/buildings/${id}/rooms`, null, { params })
}

export function getPublicRoom(buildingId, roomId) {
  return request('GET', `/buildings/${buildingId}/rooms/${roomId}`)
}

// ===== 平台管理员 =====
export function adminCreateBuilding(data) {
  return request('POST', '/admin/buildings', data)
}

export function adminGetBuildings(params) {
  return request('GET', '/admin/buildings', null, { params })
}

export function adminUpdateBuilding(id, data) {
  return request('PUT', `/admin/buildings/${id}`, data)
}

export function adminDeleteBuilding(id) {
  return request('DELETE', `/admin/buildings/${id}`)
}

export function adminUpgradePackage(id, data) {
  return request('PUT', `/admin/buildings/${id}/package`, data)
}

export function adminCreateBuildingAdmin(data) {
  return request('POST', '/admin/auth/create-building-admin', data)
}

export function adminUpdateUser(id, data) {
  return request('PUT', `/admin/auth/users/${id}`, data)
}

export function adminDeleteUser(id) {
  return request('DELETE', `/admin/auth/users/${id}`)
}

// ===== 公寓管理后台 =====
export function getBuildingInfo() {
  return request('GET', '/building/info')
}

export function updateBuildingInfo(data) {
  return request('PUT', '/building/info', data)
}

export function buildingGetRooms(params) {
  return request('GET', '/building/rooms', null, { params })
}

export function buildingCreateRoom(data) {
  return request('POST', '/building/rooms', data)
}

export function buildingGetRoom(id) {
  return request('GET', `/building/rooms/${id}`)
}

export function buildingUpdateRoom(id, data) {
  return request('PUT', `/building/rooms/${id}`, data)
}

export function buildingDeleteRoom(id) {
  return request('DELETE', `/building/rooms/${id}`)
}

export function buildingUpdateRoomStatus(id, data) {
  return request('PUT', `/building/rooms/${id}/status`, data)
}

export function buildingRenewContract(id, data) {
  return request('PUT', `/building/rooms/${id}/contract`, data)
}

export function buildingDeleteMedia(roomId, mediaId) {
  return request('DELETE', `/building/rooms/${roomId}/media/${mediaId}`)
}

export function buildingUploadCover(filePath) {
  return uploadFile('/building/cover', filePath, {})
}

export function buildingGetBills(params) {
  return request('GET', '/building/bills', null, { params })
}

export function buildingCreateBill(data) {
  return request('POST', '/building/bills', data)
}

export function buildingUpdateBill(id, data) {
  return request('PUT', `/building/bills/${id}`, data)
}

export function buildingGetBillStats(month, year) {
  const params = {}
  if (month != null) params.month = month
  if (year != null) params.year = year
  return request('GET', '/building/bills/stats', null, { params })
}

export function buildingGetDividends(page = 1, pageSize = 20) {
  return request('GET', '/building/dividends', null, { params: { page, page_size: pageSize } })
}

export function buildingCalculateDividend(month) {
  return request('GET', '/building/dividends/calculate', null, { params: { month } })
}

export function buildingGetShareholders() {
  return request('GET', '/building/dividends/shareholders')
}

export function buildingCreateShareholder(data) {
  return request('POST', '/building/dividends/shareholders', data)
}

export function buildingUpdateShareholder(id, data) {
  return request('PUT', `/building/dividends/shareholders/${id}`, data)
}

export function buildingDeleteShareholder(id) {
  return request('DELETE', `/building/dividends/shareholders/${id}`)
}

export function buildingGetTasks(status, page = 1, pageSize = 20) {
  return request('GET', '/building/tasks', null, { params: { status, page, page_size: pageSize } })
}

export function buildingProcessTask(id, data) {
  return request('POST', `/building/tasks/${id}/process`, data)
}

export function buildingGetUsers() {
  return request('GET', '/building/auth/users')
}

export function buildingCreateAdmin(data) {
  return request('POST', '/building/auth/create-admin', data)
}

// ===== 招募 =====
export function getRecruitList() {
  return request('GET', '/admin/recruit/list')
}

export function processRecruit(id) {
  return request('PUT', `/admin/recruit/process/${id}`)
}

export function submitRecruit(data) {
  return request('POST', '/recruit/submit', data)
}

export function buildingUploadMedia(roomId, filePath, formData, onProgress) {
  return uploadFile(`/building/rooms/${roomId}/media`, filePath, formData, onProgress)
}

// ===== 统计 =====
export function adminGetStatsOverview() {
  return request('GET', '/admin/stats/overview')
}

export function adminGetBuildingStats(id) {
  return request('GET', `/admin/stats/building/${id}`)
}

export function adminGetPriceReference() {
  return request('GET', '/building/stats/price-reference')
}

export function buildingGetMyStats() {
  return request('GET', '/building/stats/pv')
}

export default { login, getBuildings }
