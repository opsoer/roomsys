<script setup>
import { onLaunch } from '@dcloudio/uni-app'
onLaunch(() => {})
</script>
<style>
/* 全局样式 */
page {
  background-color: #f5f6fa;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  -webkit-tap-highlight-color: transparent;
  box-sizing: border-box;
  min-height: 100vh;
}
view, text, scroll-view, swiper {
  box-sizing: border-box;
}
.skeleton-card {
  background: #fff;
  border-radius: 12px;
  padding: 12px;
}
.page-footer {
  text-align: center;
  padding: 24px 16px;
  color: #aaa;
  font-size: 12px;
}
</style>
