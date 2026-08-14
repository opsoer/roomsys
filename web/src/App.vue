<template>
  <el-config-provider :locale="zhCn">
    <router-view v-slot="{ Component }">
      <transition name="slide-fade" mode="out-in">
        <component :is="Component" />
      </transition>
    </router-view>
    <BackTop v-show="showBackTop" />
  </el-config-provider>
</template>

<script setup>
import { ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import zhCn from 'element-plus/dist/locale/zh-cn.mjs'
import BackTop from './components/common/BackTop.vue'

const route = useRoute()
const showBackTop = ref(true)

watch(() => route.path, () => {
  showBackTop.value = true
  if (route.path === '/login') showBackTop.value = false
}, { immediate: true })
</script>
