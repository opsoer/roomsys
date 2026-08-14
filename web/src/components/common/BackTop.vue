<template>
  <div class="back-top" @click="scrollToTop">
    <span class="back-top-icon">↑</span>
    <span class="back-top-text">顶部</span>
  </div>
</template>

<script setup>
// 全局返回顶部按钮：兼容窗口滚动及各后台布局的内部滚动容器
// （房东端 .layout-main / 管理端 .main-content / 移动端 .mobile-body）
function scrollToTop() {
  const candidates = [
    document.querySelector('.main-content'),
    document.querySelector('.layout-main'),
    document.querySelector('.mobile-body'),
  ]
  for (const el of candidates) {
    if (el && el.scrollHeight > el.clientHeight) {
      el.scrollTo({ top: 0, behavior: 'smooth' })
      return
    }
  }
  window.scrollTo({ top: 0, behavior: 'smooth' })
}
</script>

<style scoped>
.back-top {
  position: fixed;
  right: 16px;
  bottom: 30px;
  z-index: 999;
  width: 50px;
  height: 50px;
  border-radius: 50%;
  background: #fff;
  border: 1px solid #e8e8e8;
  box-shadow: 0 2px 12px rgba(0,0,0,0.12);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  cursor: pointer;
}
.back-top-icon {
  font-size: 18px;
  line-height: 1;
  color: #1989fa;
  font-weight: 700;
}
.back-top-text {
  font-size: 11px;
  color: #666;
}
</style>