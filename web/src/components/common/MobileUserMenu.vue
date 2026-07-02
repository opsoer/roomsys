<template>
  <div v-if="show" class="mobile-user-dropdown" @click="$emit('close')">
    <div class="mobile-user-card" @click.stop>
      <div class="mobile-user-name">{{ username }}</div>
      <van-button round block type="danger" size="small" @click="handleLogout">退出登录</van-button>
    </div>
  </div>
</template>

<script setup>
import { useRouter } from 'vue-router'
import { showToast } from 'vant'
import { useAuthStore } from '../../stores/auth'

const props = defineProps({
  show: Boolean,
  username: String
})

const emit = defineEmits(['close'])
const router = useRouter()
const authStore = useAuthStore()

function handleLogout() {
  authStore.logout()
  showToast('已退出')
  router.push('/')
}
</script>

<style scoped>
.mobile-user-dropdown {
  position: fixed;
  top: 50px;
  right: 10px;
  z-index: 1000;
  background: rgba(0,0,0,0.5);
  width: 100%;
  height: calc(100% - 50px);
}
.mobile-user-card {
  position: absolute;
  right: 10px;
  top: 0;
  background: #fff;
  border-radius: 10px;
  padding: 16px;
  box-shadow: 0 4px 20px rgba(0,0,0,0.15);
  min-width: 160px;
}
.mobile-user-name {
  font-size: 15px;
  font-weight: 600;
  margin-bottom: 12px;
  color: #333;
}
</style>
