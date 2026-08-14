<template>
  <view class="back-top" @click="scrollToTop">
    <text class="back-top-icon">↑</text>
    <text class="back-top-text">顶部</text>
  </view>
</template>

<script setup>
function scrollToTop() {
  uni.pageScrollTo({ scrollTop: 0, duration: 300 })
}
</script>

<style scoped>
.back-top {
  position: fixed;
  right: 16px;
  bottom: 40px;
  z-index: 999;
  width: 52px;
  height: 52px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.96);
  border: 1px solid #e8e8e8;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.12);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}
.back-top-icon {
  font-size: 18px;
  line-height: 1;
  color: #e6a23c;
  font-weight: 700;
}
.back-top-text {
  font-size: 11px;
  color: #666;
}
</style>