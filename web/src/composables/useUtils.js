import { useAuthStore } from '../stores/auth'
import { useRouter } from 'vue-router'
import { mediaUrl, maskName, maskPhone, statusLabel, statusTagType } from '../utils/format'

export function useUtils() {
  const authStore = useAuthStore()
  const router = useRouter()

  function goToDashboard() {
    if (authStore.isLoggedIn) {
      router.push(authStore.getDashboardPath())
    } else {
      router.push('/login')
    }
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
