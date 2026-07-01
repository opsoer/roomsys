import { createApp } from 'vue'
import { createPinia } from 'pinia'
import 'element-plus/dist/index.css'
import 'vant/lib/index.css'
import { ElMessage } from 'element-plus'
import { ArrowLeft, ArrowDown, Close, Coin, Delete, HomeFilled, List, Loading, Money, Picture, Plus, Search, Setting, User, VideoCamera } from '@element-plus/icons-vue'
import App from './App.vue'
import router from './router'
import './style.css'

const app = createApp(App)
app.use(createPinia())

app.config.errorHandler = (err, instance, info) => {
  console.error('全局错误:', err, info)
  if (err.message && !err.message.includes('ResizeObserver')) {
    ElMessage.error('操作失败，请重试')
  }
}

app.component('ArrowLeft', ArrowLeft)
app.component('ArrowDown', ArrowDown)
app.component('Close', Close)
app.component('Coin', Coin)
app.component('Delete', Delete)
app.component('HomeFilled', HomeFilled)
app.component('List', List)
app.component('Loading', Loading)
app.component('Money', Money)
app.component('Picture', Picture)
app.component('Plus', Plus)
app.component('Search', Search)
app.component('Setting', Setting)
app.component('User', User)
app.component('VideoCamera', VideoCamera)
app.use(router)
app.mount('#app')
