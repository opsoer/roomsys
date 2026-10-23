// 简易状态管理 - 基于 uni-app storage。
// STORAGE_KEYS 是全部凭证键的定义，统一从这里导出供清理复用，
// 勿在其他文件手工列举 key。
export const STORAGE_KEYS = ['token', 'refreshToken', 'username', 'role', 'building_id']

function getVal(key) {
  try { return uni.getStorageSync(key) || '' } catch { return '' }
}
function setVal(key, val) {
  try { uni.setStorageSync(key, val || '') } catch {}
}
function removeVal(key) {
  try { uni.removeStorageSync(key) } catch {}
}

export const auth = {
  get token() { return getVal('token') },
  set token(v) { setVal('token', v) },

  get refreshToken() { return getVal('refreshToken') },
  set refreshToken(v) { setVal('refreshToken', v) },

  get username() { return getVal('username') },
  set username(v) { setVal('username', v) },

  get role() { return getVal('role') },
  set role(v) { setVal('role', v) },

  get buildingId() { return getVal('building_id') },
  set buildingId(v) { setVal('building_id', v) },

  get isLoggedIn() { return !!this.token },

  get isSuperAdmin() { return this.role === 'super_admin' },

  get isBuildingAdmin() {
    return this.role === 'building_admin' || this.role === 'admin'
  },

  login(userData, tokenStr, refreshTokenStr) {
    this.token = tokenStr
    this.refreshToken = refreshTokenStr || ''
    this.username = userData.username || ''
    this.role = userData.role || ''
    this.buildingId = userData.building_id || ''
  },

  logout() {
    STORAGE_KEYS.forEach(k => removeVal(k))
  },

  getDashboardPath() {
    if (this.role === 'super_admin') return '/pages/admin/buildings'
    return '/pages/landlord/rooms'
  }
}
