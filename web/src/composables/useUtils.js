import { useAuthStore } from '../stores/auth'
import { useRouter } from 'vue-router'

export function useUtils() {
  const authStore = useAuthStore()
  const router = useRouter()

  function mediaUrl(path) {
    if (!path) return ''
    if (path.includes('..') || path.includes('\\')) return ''
    return `/api/media/${path}`
  }

  function goToDashboard() {
    if (authStore.isLoggedIn) {
      router.push(authStore.getDashboardPath())
    } else {
      router.push('/login')
    }
  }

  function maskName(name) {
    if (!name) return ''
    return name.charAt(0) + '***'
  }

  function maskPhone(phone) {
    if (!phone || phone.length < 7) return phone
    return phone.slice(0, 3) + '****' + phone.slice(-4)
  }

  function statusLabel(status) {
    const labels = {
      vacant: '未出租',
      rented: '已出租',
      expiring: '即将到期',
      expired: '已过期',
    }
    return labels[status] || status
  }

  function statusTagType(status) {
    const types = {
      vacant: 'success',
      rented: 'primary',
      expiring: 'warning',
      expired: 'danger',
    }
    return types[status] || 'info'
  }

  return {
    mediaUrl,
    goToDashboard,
    maskName,
    maskPhone,
    statusLabel,
    statusTagType,
  }
}
