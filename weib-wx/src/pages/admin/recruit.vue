<template>
  <view class="page-recruit">
    <view class="header-row">
      <text class="page-title">招商管理</text>
      <text :class="['count-badge', pendingCount > 0 ? 'has-pending' : '']">{{ pendingCount > 0 ? pendingCount + ' 条待处理' : '全部已处理' }}</text>
    </view>

    <view v-if="loading" class="loading-wrap"><text>加载中...</text></view>
    <view v-else-if="submissions.length === 0" class="empty-wrap"><text>暂无入驻申请</text></view>

    <view v-else class="submission-list">
      <view v-for="item in submissions" :key="item.id" :class="['submission-card', item.status]">
        <view class="card-head">
          <text class="card-id">申请 #{{ item.id }}</text>
          <text :class="['status-tag', item.status]">{{ item.status === 'pending' ? '待处理' : '已处理' }}</text>
        </view>
        <view class="card-field">📞 {{ item.phone }}</view>
        <view class="card-field">📍 {{ item.address }}</view>
        <view class="card-foot">
          <text class="card-time">{{ formatTime(item.created_at) }}</text>
          <button v-if="item.status === 'pending'" class="process-btn" @click="handleProcess(item.id)">标记已处理</button>
          <text v-else class="done-text">已完成</text>
        </view>
      </view>
    </view>
  </view>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { getRecruitList, processRecruit } from '../../api'
import dayjs from 'dayjs'

const submissions = ref([])
const loading = ref(false)
const pendingCount = ref(0)

function formatTime(t) {
  if (!t) return '-'
  return dayjs(t).format('YYYY-MM-DD HH:mm')
}

async function fetchSubmissions() {
  loading.value = true
  try {
    const r = await getRecruitList()
    submissions.value = r.data.submissions || []
    pendingCount.value = submissions.value.filter(s => s.status === 'pending').length
  } catch {
    uni.showToast({ title: '获取失败', icon: 'none' })
  } finally {
    loading.value = false
  }
}

async function handleProcess(id) {
  try {
    await processRecruit(id)
    uni.showToast({ title: '已处理', icon: 'success' })
    await fetchSubmissions()
  } catch {
    uni.showToast({ title: '操作失败', icon: 'none' })
  }
}

onMounted(fetchSubmissions)
</script>

<style scoped>
.page-recruit { padding: 16px; min-height: 100vh; }
.header-row { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.page-title { font-size: 20px; font-weight: 700; color: #1a1a2e; }
.count-badge { font-size: 13px; padding: 4px 12px; border-radius: 20px; background: #f0f0f0; color: #999; }
.count-badge.has-pending { background: #fef0f0; color: #f56c6c; }
.loading-wrap, .empty-wrap { text-align: center; padding: 60px 0; color: #999; }
.submission-card { background: #fff; border-radius: 10px; padding: 12px 14px; margin-bottom: 12px; box-shadow: 0 2px 8px rgba(0,0,0,0.06); }
.submission-card.pending { border: 1px solid #fde2e2; }
.submission-card:not(.pending) { border: 1px solid #e8f5e9; }
.card-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
.card-id { font-size: 12px; font-weight: 600; color: #999; }
.status-tag { font-size: 11px; padding: 2px 10px; border-radius: 10px; color: #fff; }
.status-tag.pending { background: #f56c6c; }
.status-tag.processed { background: #67c23a; }
.card-field { font-size: 14px; color: #333; margin-bottom: 4px; }
.card-foot { display: flex; justify-content: space-between; align-items: center; margin-top: 10px; padding-top: 10px; border-top: 1px solid #f5f5f5; }
.card-time { font-size: 12px; color: #bbb; }
.process-btn { font-size: 12px; color: #fff; background: #1989fa; border: none; border-radius: 20px; padding: 4px 14px; }
.done-text { font-size: 12px; color: #67c23a; }
</style>
