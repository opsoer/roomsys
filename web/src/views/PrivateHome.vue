<template>
  <div class="private-home">
    <div class="card">
      <div class="logo">🏠 圳好租</div>
      <h1>私享访问</h1>
      <p class="tip">本站为邀请制访问，请通过房东分享的链接或二维码查看公寓房源</p>
      <el-button type="primary" plain size="small" @click="$router.push('/login')">管理登录</el-button>
    </div>
    <p class="copyright">© {{ year }} 圳好租 · 公寓租赁管理平台</p>
  </div>
</template>

<script setup>
const year = new Date().getFullYear()
</script>

<style scoped>
.private-home {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: linear-gradient(180deg, #f7f8fa 0%, #eef1f5 100%);
  padding: 24px;
}
.card {
  background: #fff;
  border-radius: 16px;
  padding: 48px 40px;
  text-align: center;
  box-shadow: 0 8px 30px rgba(0, 0, 0, 0.06);
  max-width: 420px;
}
.logo {
  font-size: 15px;
  color: #999;
  margin-bottom: 20px;
}
h1 {
  font-size: 22px;
  font-weight: 700;
  color: #333;
  margin: 0 0 12px;
}
.tip {
  font-size: 14px;
  color: #888;
  line-height: 1.8;
  margin: 0 0 24px;
}
.copyright {
  margin-top: 32px;
  font-size: 12px;
  color: #bbb;
}
</style>
