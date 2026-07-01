<template>
  <div class="not-found">
    <div class="not-found-content">
      <h1>404</h1>
      <p>页面不存在</p>
      <button @click="$router.push('/')">返回首页</button>
    </div>
  </div>
</template>

<style scoped>
.not-found {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background: #f5f6fa;
}
.not-found-content {
  text-align: center;
}
.not-found-content h1 {
  font-size: 72px;
  color: #e6a23c;
  margin-bottom: 16px;
}
.not-found-content p {
  font-size: 18px;
  color: #666;
  margin-bottom: 24px;
}
.not-found-content button {
  padding: 10px 24px;
  background: #e6a23c;
  color: #fff;
  border: none;
  border-radius: 8px;
  font-size: 16px;
  cursor: pointer;
}
.not-found-content button:hover {
  background: #d4910a;
}
</style>
