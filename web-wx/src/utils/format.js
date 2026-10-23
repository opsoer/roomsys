// 注意：默认值为占位，部署前需通过 VITE_API_BASE 环境变量配置真实域名
const API_BASE = process.env.VITE_API_BASE || 'https://your-api-domain.com'

export function mediaUrl(path) {
  if (!path) return ''
  if (path.includes('..') || path.includes('\\')) return ''
  return `${API_BASE}/api/media/${path}`
}

export function maskName(name) {
  if (!name) return ''
  return name.charAt(0) + '***'
}

export function maskPhone(phone) {
  if (!phone || phone.length < 7) return phone
  return phone.slice(0, 3) + '****' + phone.slice(-4)
}

export function statusLabel(status) {
  // expiring = 后端 DynamicRoomStatus：rented 且租约 30 天内到期（含今天），
  // 含义是"该续约"，并非"即将退租"；与 web 端 utils/format.js 口径一致。
  const labels = {
    vacant: '未出租',
    reserved: '已预订',
    rented: '已出租',
    expiring: '即将到期',
    expired: '已过期',
  }
  return labels[status] || status
}

export function statusTagType(status) {
  const types = {
    vacant: 'success',
    reserved: 'primary',
    rented: 'danger',
    expiring: 'warning',
    expired: 'danger',
  }
  return types[status] || 'info'
}
