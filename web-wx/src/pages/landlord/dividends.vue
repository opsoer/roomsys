<template>
  <view class="page-dividends">
    <view class="card-section">
      <text class="section-title">分红计算</text>
      <view class="calc-row">
        <picker mode="date" fields="month" @change="e => calcMonth = e.detail.value">
          <view class="picker-val">{{ calcMonth || '选择月份' }}</view>
        </picker>
        <button class="calc-btn" @click="handleCalculate">查看分红</button>
      </view>
      <view v-if="preview" class="preview-box">
        <view class="pv-row"><text class="pv-label">总收入</text><text class="pv-val">¥{{ preview.total_income?.toFixed(2) }}</text></view>
        <view class="pv-row"><text class="pv-label">总支出</text><text class="pv-val red">¥{{ preview.total_expense?.toFixed(2) }}</text></view>
        <view class="pv-row pv-divider"><text class="pv-label">净利润</text><text class="pv-val gold">¥{{ preview.net_profit?.toFixed(2) }}</text></view>
        <view v-if="preview.results?.length" class="preview-results">
          <view v-for="r in preview.results" :key="r.name" class="pr-row">
            <text class="pr-name">{{ r.name }} ({{ r.share_ratio }}%)</text>
            <text class="pr-amount">¥{{ r.dividend_amount.toFixed(2) }}</text>
          </view>
        </view>
        <text v-else class="no-profit">本月无净利润，不分红</text>
      </view>
    </view>

    <view class="card-section">
      <view class="sh-header"><text class="section-title">股东配置</text><button class="add-btn" @click="openSHDialog">+ 添加</button></view>
      <view v-for="s in shareholders" :key="s.id" class="sh-card">
        <view class="sh-top"><text class="sh-name">{{ s.name }}</text><text class="sh-ratio">{{ s.share_ratio }}%</text></view>
        <view class="sh-actions"><button @click="handleEditSH(s)">编辑</button><button class="danger" @click="handleDeleteSH(s.id)">删除</button></view>
      </view>
    </view>

    <view class="card-section">
      <text class="section-title">历史分红记录</text>
      <view v-for="d in dividends" :key="d.id" class="dh-card">
        <view class="dh-top"><text class="dh-month">{{ d.settle_month }}</text><text class="dh-amount">¥{{ d.dividend_amount?.toFixed(2) }}</text></view>
        <view class="dh-detail">收入 {{ d.total_income?.toFixed(2) }} | 支出 {{ d.total_expense?.toFixed(2) }} | 净利 {{ d.net_profit?.toFixed(2) }}</view>
        <view class="dh-bottom"><text>股东：{{ d.shareholder?.name }}</text><text class="dh-time">{{ d.created_at }}</text></view>
      </view>
    </view>

    <!-- Shareholder Dialog -->
    <view v-if="showSHDialog" class="overlay" @click="showSHDialog = false">
      <view class="small-dialog" @click.stop>
        <text class="dialog-title">{{ editingSHId ? '编辑股东' : '添加股东' }}</text>
        <view class="form-group"><text class="form-label">姓名</text><input class="form-input" v-model="shForm.name" /></view>
        <view class="form-group"><text class="form-label">持股比例(%)</text><input class="form-input" v-model="shForm.share_ratio" type="digit" /></view>
        <view class="dialog-actions">
          <button class="dialog-btn cancel" @click="showSHDialog = false">取消</button>
          <button class="dialog-btn confirm" :disabled="shSubmitting" @click="handleAddSH">{{ shSubmitting ? '提交中...' : '确定' }}</button>
        </view>
      </view>
    </view>
  </view>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { buildingGetDividends, buildingCalculateDividend, buildingGetShareholders, buildingCreateShareholder, buildingUpdateShareholder, buildingDeleteShareholder } from '../../api'

const calcMonth = ref('')
const preview = ref(null)
const dividends = ref([])
const shareholders = ref([])
const showSHDialog = ref(false)
const shSubmitting = ref(false)
const shForm = ref({ name: '', share_ratio: 0 })
const editingSHId = ref(null)

async function handleCalculate() {
  if (!calcMonth.value) { uni.showToast({ title: '请选择月份', icon: 'none' }); return }
  try {
    const res = await buildingCalculateDividend(calcMonth.value)
    preview.value = res.data
  } catch { uni.showToast({ title: '获取失败', icon: 'none' }) }
}

async function fetchDividends() {
  try {
    const res = await buildingGetDividends()
    dividends.value = res.data.dividends || []
  } catch {}
}

async function fetchShareholders() {
  try {
    const res = await buildingGetShareholders()
    shareholders.value = res.data.shareholders || []
  } catch {}
}

function openSHDialog() { editingSHId.value = null; shForm.value = { name: '', share_ratio: 0 }; showSHDialog.value = true }

function handleEditSH(row) {
  editingSHId.value = row.id
  shForm.value = { name: row.name, share_ratio: row.share_ratio }
  showSHDialog.value = true
}

async function handleAddSH() {
  shSubmitting.value = true
  try {
    if (editingSHId.value) {
      await buildingUpdateShareholder(editingSHId.value, shForm.value)
    } else {
      await buildingCreateShareholder(shForm.value)
    }
    uni.showToast({ title: '保存成功', icon: 'success' })
    showSHDialog.value = false
    await fetchShareholders()
  } catch { uni.showToast({ title: '操作失败', icon: 'none' }) }
  finally { shSubmitting.value = false }
}

async function handleDeleteSH(id) {
  uni.showModal({
    title: '确认删除',
    content: '确定删除该股东？',
    success: async (res) => {
      if (!res.confirm) return
      try {
        await buildingDeleteShareholder(id)
        uni.showToast({ title: '已删除', icon: 'success' })
        await fetchShareholders()
      } catch { uni.showToast({ title: '删除失败', icon: 'none' }) }
    }
  })
}

onMounted(() => { fetchDividends(); fetchShareholders() })
</script>

<style scoped>
.page-dividends { padding: 12px; min-height: 100vh; }
.card-section { background: #fff; border-radius: 12px; padding: 16px; margin-bottom: 12px; box-shadow: 0 1px 4px rgba(0,0,0,0.04); }
.section-title { font-size: 15px; font-weight: 600; color: #333; display: block; margin-bottom: 12px; }
.calc-row { display: flex; gap: 10px; align-items: center; }
.picker-val { height: 40px; line-height: 40px; border: 1px solid #dcdfe6; border-radius: 8px; padding: 0 12px; font-size: 14px; background: #fff; min-width: 130px; }
.calc-btn { background: #1989fa; color: #fff; border: none; border-radius: 8px; padding: 8px 16px; font-size: 14px; }
.preview-box { margin-top: 12px; background: #f5f7fa; border-radius: 8px; padding: 12px; }
.pv-row { display: flex; justify-content: space-between; padding: 6px 0; font-size: 14px; }
.pv-divider { border-top: 1px solid #e0e0e0; margin-top: 6px; padding-top: 10px; font-weight: 600; }
.pv-label { color: #666; }
.pv-val { font-weight: 600; color: #333; }
.pv-val.red { color: #f56c6c; }
.pv-val.gold { color: #e6a23c; }
.preview-results { margin-top: 10px; }
.pr-row { display: flex; justify-content: space-between; padding: 6px 0; font-size: 14px; border-top: 1px solid #e0e0e0; }
.pr-name { color: #333; }
.pr-amount { color: #e6a23c; font-weight: 600; }
.no-profit { display: block; text-align: center; color: #999; margin-top: 10px; }
.sh-header { display: flex; justify-content: space-between; align-items: center; }
.add-btn { background: #1989fa; color: #fff; border: none; border-radius: 6px; padding: 6px 14px; font-size: 13px; }
.sh-card { display: flex; justify-content: space-between; align-items: center; padding: 10px 0; border-bottom: 1px solid #f5f5f5; }
.sh-top { flex: 1; }
.sh-name { font-size: 14px; font-weight: 600; color: #333; display: block; }
.sh-ratio { font-size: 13px; color: #409eff; }
.sh-actions { display: flex; gap: 8px; }
.sh-actions button { font-size: 12px; color: #1989fa; background: none; border: none; padding: 4px 8px; }
.sh-actions button.danger { color: #f56c6c; }
.dh-card { border-bottom: 1px solid #f5f5f5; padding: 10px 0; }
.dh-top { display: flex; justify-content: space-between; align-items: center; }
.dh-month { font-size: 14px; font-weight: 600; color: #333; }
.dh-amount { font-size: 15px; font-weight: 700; color: #e6a23c; }
.dh-detail { font-size: 12px; color: #666; margin: 4px 0; }
.dh-bottom { display: flex; justify-content: space-between; font-size: 12px; color: #999; }
.overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); z-index: 1000; display: flex; align-items: flex-end; }
.small-dialog { width: 100%; background: #fff; border-radius: 16px 16px 0 0; padding: 20px; }
.dialog-title { font-size: 18px; font-weight: 700; display: block; margin-bottom: 16px; }
.form-group { margin-bottom: 12px; }
.form-label { font-size: 14px; color: #333; display: block; margin-bottom: 4px; }
.form-input { width: 100%; height: 40px; border: 1px solid #dcdfe6; border-radius: 8px; padding: 0 12px; font-size: 14px; background: #fff; }
.dialog-actions { display: flex; gap: 12px; margin-top: 20px; }
.dialog-btn { flex: 1; height: 44px; border-radius: 22px; display: flex; align-items: center; justify-content: center; font-size: 15px; }
.dialog-btn.cancel { background: #f5f5f5; color: #666; border: none; }
.dialog-btn.confirm { background: #1989fa; color: #fff; border: none; }
.dialog-btn.confirm[disabled] { opacity: 0.6; }
</style>
