// 站点模式 store：缓存 /api/config 的部署模式（platform=平台 / private=私享）。
// 私享模式下，公寓列表首页与申请入驻页仅超级管理员可见，
// 普通访客只能通过公寓链接/二维码直达单个公寓页（见 router 守卫）。
import { defineStore } from 'pinia'
import { getSiteConfig } from '../api'

export const useSiteStore = defineStore('site', {
  state: () => ({
    // 部署模式：'platform' | 'private'；请求返回前按平台模式处理（先渲染后纠正）
    mode: 'platform',
    // 私享模式下若可见公寓恰好 1 个，后端下发 { id, name } 作为默认直达目标
    defaultBuilding: null,
    // 是否已完成首次加载
    loaded: false,
  }),
  getters: {
    isPrivate: (s) => s.mode === 'private',
  },
  actions: {
    // load 拉取站点配置（整个页面生命周期只真正请求一次，promise 复用）。
    // 请求失败按平台模式兜底，避免网络抖动把访客卡在白屏。
    load() {
      if (this.loaded) return Promise.resolve()
      if (!this._loadingPromise) {
        this._loadingPromise = getSiteConfig()
          .then((res) => {
            this.mode = res.data?.mode || 'platform'
            this.defaultBuilding = res.data?.default_building || null
          })
          .catch(() => {
            this.mode = 'platform'
            this.defaultBuilding = null
          })
          .finally(() => {
            this.loaded = true
            this._loadingPromise = null
          })
      }
      return this._loadingPromise
    },
  },
})
