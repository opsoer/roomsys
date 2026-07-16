<template>
  <view class="page-bills">
    <view class="tab-bar">
      <view :class="['tab', activeTab === 'list' ? 'active' : '']" @click="activeTab='list'">账单列表</view>
      <view :class="['tab', activeTab === 'monthly' ? 'active' : '']" @click="activeTab='monthly'">月度统计</view>
      <view :class="['tab', activeTab === 'yearly' ? 'active' : '']" @click="activeTab='yearly'">年度统计</view>
    </view>

    <!-- 账单列表 -->
    <view v-if="activeTab === 'list'">
      <view class="filter-bar">
        <view class="filter-tab" @click="showTypeSheet">
          <text>{{ filterTypeText }}</text><text class="arrow">▼</text>
        </view>
        <button class="add-btn" @click="openAddDialog">+ 新增</button>
      </view>

      <view v-if="billLoading" class="loading-wrap"><text>加载中...</text></view>
      <view v-else-if="bills.length === 0" class="empty-wrap"><text>暂无账单</text></view>

      <view v-else class="bill-list">
        <view v-for="row in bills" :key="row.id" class="bill-card">
          <view class="bc-head">
            <text class="bc-no">{{ row.bill_no }}</text>
            <text :class="['bc-type', row.type]">{{ row.type === 'income' ? '收入' : '支出' }}</text>
          </view>
          <view class="bc-info">
            <text>{{ row.bill_date }}</text>
            <text>{{ row.subtype }}</text>
            <text>{{ row.room?.room_number || '其他' }}</text>
          </view>
          <view class="bc-body">
            <text :class="['bc-amount', row.type]">{{ row.type === 'income' ? '+' : '-' }}{{ row.amount.toFixed(2) }}</text>
            <button class="edit-btn" @click="openEditDialog(row)">修改</button>
          </view>
          <text v-if="row.description" class="bc-desc">{{ row.description }}</text>
        </view>
      </view>

      <view v-if="billTotal > billPageSize" class="pagination">
        <button :disabled="billPage <= 1" @click="billPage--; fetchBills()">上一页</button>
        <text>{{ billPage }} / {{ Math.ceil(billTotal / billPageSize) }}</text>
        <button :disabled="billPage >= Math.ceil(billTotal / billPageSize)" @click="billPage++; fetchBills()">下一页</button>
      </view>
    </view>

    <!-- 月度统计 -->
    <view v-if="activeTab === 'monthly' || activeTab === 'yearly'">
      <view class="date-picker-bar">
        <picker mode="date" fields="month" @change="e => { statMonth = e.detail.value; fetchStats() }">
          <view class="picker-val2">{{ statMonth || '选择月份' }}</view>
        </picker>
      </view>
      <view v-if="stats" class="stats-grid">
        <view class="stat-card green"><text class="stat-label">收入</text><text class="stat-value">¥{{ stats.total_income?.toFixed(2) }}</text></view>
        <view class="stat-card red"><text class="stat-label">支出</text><text class="stat-value">¥{{ stats.total_expense?.toFixed(2) }}</text></view>
        <view class="stat-card blue"><text class="stat-label">净利润</text><text class="stat-value">¥{{ stats.net_profit?.toFixed(2) }}</text></view>
      </view>
      <view v-if="stats?.income_detail?.length" class="detail-section">
        <text class="section-title">收入明细</text>
        <view v-for="d in stats.income_detail" :key="d.subtype" class="detail-row">
          <text>{{ d.subtype }}</text>
          <text class="green">¥{{ d.total.toFixed(2) }}</text>
        </view>
      </view>
      <view v-if="stats?.expense_detail?.length" class="detail-section">
        <text class="section-title">支出明细</text>
        <view v-for="d in stats.expense_detail" :key="d.subtype" class="detail-row">
          <text>{{ d.subtype }}</text>
          <text class="red">¥{{ d.total.toFixed(2) }}</text>
        </view>
      </view>
    </view>

    <!-- Type filter sheet -->
    <view v-if="filterSheetOpen" class="overlay" @click="filterSheetOpen = false">
      <view class="sheet-panel" @click.stop>
        <view v-for="opt in typeFilterOptions" :key="opt.value" :class="['sheet-item', { active: opt.value === filter.type }]" @click="filter.type = opt.value; filterSheetOpen = false; fetchBills()">
          <text>{{ opt.label }}</text>
        </view>
        <view class="sheet-cancel" @click="filterSheetOpen = false">取消</view>
      </view>
    </view>

    <!-- Add/Edit Dialog -->
    <view v-if="showBillDialog" class="overlay" @click="showBillDialog = false">
      <scroll-view scroll-y class="dialog-panel" @click.stop>
        <text class="dialog-title">{{ editingBillId ? '修改账单金额' : '新增账单' }}</text>
        <template v-if="!editingBillId">
          <view class="form-group"><text class="form-label">类型</text><view class="radio-group"><text :class="['radio', billForm.type === 'income' ? 'active' : '']" @click="billForm.type = 'income'">收入</text><text :class="['radio', billForm.type === 'expense' ? 'active' : '']" @click="billForm.type = 'expense'">支出</text></view></view>
          <view class="form-group"><text class="form-label">子类型</text>
            <picker mode="selector" :range="subtypeOptions" @change="e => billForm.subtype = subtypeOptions[e.detail.value]"><view class="picker-val">{{ billForm.subtype || '选择' }}</view></picker>
          </view>
          <view class="form-group"><text class="form-label">金额</text><input class="form-input" v-model="billForm.amount" type="digit" /></view>
          <view class="form-group"><text class="form-label">日期</text><picker mode="date" @change="e => billForm.bill_date = e.detail.value"><view class="picker-val">{{ billForm.bill_date || '选择日期' }}</view></picker></view>
          <view class="form-group"><text class="form-label">备注</text><textarea class="form-textarea" v-model="billForm.description" /></view>
        </template>
        <template v-else>
          <view class="form-group"><text class="form-label">原金额</text><text class="old-amount">¥{{ billForm._old_amount?.toFixed(2) }}</text></view>
          <view class="form-group"><text class="form-label">新金额</text><input class="form-input" v-model="billForm.amount" type="digit" /></view>
        </template>
        <view class="dialog-actions">
          <button class="dialog-btn cancel" @click="showBillDialog = false">取消</button>
          <button class="dialog-btn confirm" :disabled="billSubmitting" @click="handleBillSave">{{ billSubmitting ? '提交中...' : '确定' }}</button>
        </view>
      </scroll-view>
    </view>
  </view>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { buildingGetBills, buildingGetBillStats, buildingCreateBill, buildingUpdateBill } from '../../api'
import { SUBTYPE_INCOME, SUBTYPE_EXPENSE } from '../../utils/constants'

const activeTab = ref('list')
const bills = ref([])
const billLoading = ref(false)
const billPage = ref(1)
const billTotal = ref(0)
const billPageSize = 20

const filter = reactive({ type: '' })
const filterSheetOpen = ref(false)
const typeFilterOptions = [{ label: '全部', value: '' }, { label: '收入', value: 'income' }, { label: '支出', value: 'expense' }]
const filterTypeText = computed(() => typeFilterOptions.find(o => o.value === filter.type)?.label || '全部')

// Stats
const statMonth = ref('')
const stats = ref(null)

// Dialog
const showBillDialog = ref(false)
const editingBillId = ref(null)
const billSubmitting = ref(false)
const billForm = ref({ type: 'income', subtype: '', amount: 0, bill_date: '', description: '', _old_amount: 0 })
const subtypeOptions = computed(() => billForm.value.type === 'income' ? SUBTYPE_INCOME : SUBTYPE_EXPENSE)

async function fetchBills() {
  billLoading.value = true
  try {
    const params = { page: billPage.value, page_size: billPageSize }
    if (filter.type) params.type = filter.type
    const res = await buildingGetBills(params)
    bills.value = res.data.bills || []
    billTotal.value = res.data.total || 0
  } catch { uni.showToast({ title: '获取失败', icon: 'none' }) }
  finally { billLoading.value = false }
}

async function fetchStats() {
  if (!statMonth.value) return
  try {
    const res = await buildingGetBillStats(statMonth.value)
    stats.value = res.data
  } catch { uni.showToast({ title: '获取统计失败', icon: 'none' }) }
}

function showTypeSheet() { filterSheetOpen.value = true }

function openAddDialog() {
  editingBillId.value = null
  billForm.value = { type: 'income', subtype: '', amount: 0, bill_date: '', description: '', _old_amount: 0 }
  showBillDialog.value = true
}

function openEditDialog(row) {
  editingBillId.value = row.id
  billForm.value = { type: row.type, subtype: row.subtype, amount: row.amount, bill_date: row.bill_date, description: row.description || '', _old_amount: row.amount }
  showBillDialog.value = true
}

async function handleBillSave() {
  billSubmitting.value = true
  try {
    if (editingBillId.value) {
      await buildingUpdateBill(editingBillId.value, billForm.value)
    } else {
      await buildingCreateBill(billForm.value)
    }
    uni.showToast({ title: '保存成功', icon: 'success' })
    showBillDialog.value = false
    await fetchBills()
  } catch { uni.showToast({ title: '操作失败', icon: 'none' }) }
  finally { billSubmitting.value = false }
}

onMounted(fetchBills)
</script>

<style scoped>
.page-bills { min-height: 100vh; }
.tab-bar { display: flex; background: #fff; border-bottom: 1px solid #f0f0f0; position: sticky; top: 0; z-index: 10; }
.tab { flex: 1; text-align: center; padding: 12px 0; font-size: 14px; color: #666; border-bottom: 2px solid transparent; }
.tab.active { color: #e6a23c; border-bottom-color: #e6a23c; font-weight: 600; }
.filter-bar { display: flex; justify-content: space-between; align-items: center; padding: 10px 12px; }
.filter-tab { display: flex; align-items: center; gap: 4px; padding: 6px 14px; border: 1px solid #e8e8e8; border-radius: 16px; font-size: 13px; color: #666; }
.arrow { font-size: 10px; }
.add-btn { background: #1989fa; color: #fff; border: none; border-radius: 8px; padding: 8px 16px; font-size: 14px; }
.loading-wrap, .empty-wrap { text-align: center; padding: 60px 0; color: #999; }
.bill-card { background: #fff; border-radius: 10px; padding: 12px 14px; margin: 0 12px 10px; box-shadow: 0 1px 4px rgba(0,0,0,0.04); }
.bc-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px; }
.bc-no { font-size: 12px; font-weight: 600; color: #999; }
.bc-type { font-size: 11px; padding: 2px 10px; border-radius: 10px; color: #fff; }
.bc-type.income { background: #67c23a; }
.bc-type.expense { background: #f56c6c; }
.bc-info { display: flex; gap: 8px; font-size: 12px; color: #999; margin-bottom: 8px; }
.bc-body { display: flex; align-items: center; justify-content: space-between; }
.bc-amount { font-size: 18px; font-weight: 700; }
.bc-amount.income { color: #67c23a; }
.bc-amount.expense { color: #f56c6c; }
.edit-btn { font-size: 12px; color: #1989fa; background: none; border: 1px solid #1989fa; border-radius: 4px; padding: 2px 10px; }
.bc-desc { font-size: 12px; color: #999; margin-top: 6px; padding-top: 6px; border-top: 1px solid #f5f5f5; display: block; }
.pagination { display: flex; justify-content: center; align-items: center; gap: 12px; margin: 16px; }
.pagination button { background: #fff; border: 1px solid #dcdfe6; border-radius: 6px; padding: 6px 14px; font-size: 13px; }
.pagination button[disabled] { opacity: 0.4; }
.pagination text { font-size: 13px; color: #666; }
.date-picker-bar { padding: 12px; }
.picker-val2 { height: 40px; line-height: 40px; border: 1px solid #dcdfe6; border-radius: 8px; padding: 0 12px; font-size: 14px; background: #fff; display: inline-block; min-width: 140px; }
.stats-grid { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 10px; padding: 0 12px; margin-bottom: 12px; }
.stat-card { background: #fff; border-radius: 10px; padding: 14px; text-align: center; }
.stat-card.green .stat-value { color: #67c23a; }
.stat-card.red .stat-value { color: #f56c6c; }
.stat-card.blue .stat-value { color: #409eff; }
.stat-label { font-size: 12px; color: #999; display: block; }
.stat-value { font-size: 18px; font-weight: 700; display: block; margin-top: 4px; }
.detail-section { background: #fff; border-radius: 10px; margin: 0 12px 12px; padding: 14px; }
.section-title { font-size: 14px; font-weight: 600; color: #333; display: block; margin-bottom: 8px; }
.detail-row { display: flex; justify-content: space-between; padding: 6px 0; font-size: 14px; border-bottom: 1px solid #f5f5f5; }
.detail-row:last-child { border-bottom: none; }
.detail-row .green { color: #67c23a; font-weight: 600; }
.detail-row .red { color: #f56c6c; font-weight: 600; }
.overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); z-index: 1000; display: flex; align-items: flex-end; }
.sheet-panel { width: 100%; background: #fff; border-radius: 16px 16px 0 0; }
.sheet-item { padding: 14px 20px; font-size: 15px; }
.sheet-item.active { color: #1989fa; font-weight: 600; }
.sheet-cancel { text-align: center; padding: 14px; border-top: 1px solid #f0f0f0; color: #999; }
.dialog-panel { width: 100%; background: #fff; border-radius: 16px 16px 0 0; padding: 20px; max-height: 80vh; }
.dialog-title { font-size: 18px; font-weight: 700; display: block; margin-bottom: 16px; }
.form-group { margin-bottom: 12px; }
.form-label { font-size: 14px; color: #333; display: block; margin-bottom: 4px; }
.form-input { width: 100%; height: 40px; border: 1px solid #dcdfe6; border-radius: 8px; padding: 0 12px; font-size: 14px; background: #fff; }
.form-textarea { width: 100%; border: 1px solid #dcdfe6; border-radius: 8px; padding: 10px 12px; font-size: 14px; background: #fff; min-height: 60px; }
.picker-val { height: 40px; line-height: 40px; border: 1px solid #dcdfe6; border-radius: 8px; padding: 0 12px; font-size: 14px; background: #fff; }
.radio-group { display: flex; gap: 10px; }
.radio { padding: 8px 20px; border: 1px solid #dcdfe6; border-radius: 8px; font-size: 14px; color: #666; }
.radio.active { border-color: #1989fa; color: #1989fa; background: #ecf5ff; }
.old-amount { font-size: 16px; font-weight: 600; color: #f56c6c; }
.dialog-actions { display: flex; gap: 12px; margin-top: 20px; }
.dialog-btn { flex: 1; height: 44px; border-radius: 22px; display: flex; align-items: center; justify-content: center; font-size: 15px; }
.dialog-btn.cancel { background: #f5f5f5; color: #666; border: none; }
.dialog-btn.confirm { background: #1989fa; color: #fff; border: none; }
.dialog-btn.confirm[disabled] { opacity: 0.6; }
</style>
