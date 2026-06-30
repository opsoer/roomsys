import { createApp } from 'vue'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import zhCn from 'element-plus/dist/locale/zh-cn.mjs'
import { ArrowLeft, ArrowDown, Close, Coin, Delete, HomeFilled, List, Loading, Money, Picture, Plus, Search, Setting, User, VideoCamera } from '@element-plus/icons-vue'
import Vant from 'vant'
import 'vant/lib/index.css'
import App from './App.vue'
import router from './router'
import './style.css'

const app = createApp(App)
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
app.use(ElementPlus, { locale: zhCn })
app.use(Vant)
app.use(router)
app.mount('#app')
