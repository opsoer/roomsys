<template>
  <div>
    <h3 style="margin-bottom: 20px">财务管理</h3>
    <el-tabs v-model="activeTab" @tab-change="handleTabChange">
      <el-tab-pane label="账单列表" name="list">
        <div style="display: flex; gap: 10px; margin-bottom: 16px; flex-wrap: wrap; align-items: center">
          <el-select v-model="filter.type" placeholder="类型" clearable style="width: 120px" @change="fetchBills">
            <el-option label="全部" value="" />
            <el-option label="收入" value="income" />
            <el-option label="支出" value="expense" />
          </el-select>
          <el-select v-model="filter.subtype" placeholder="子类型" clearable style="width: 140px" @change="fetchBills">
            <el-option label="全部" value="" />
            <el-option label="租金" value="租金" />
            <el-option label="定金" value="定金" />
            <el-option label="押金" value="押金" />
            <el-option label="水电费" value="水电费" />
            <el-option label="物业费" value="物业费" />
            <el-option label="维修费" value="维修费" />
            <el-option label="清洁费" value="清洁费" />
            <el-option label="税费" value="税费" />
            <el-option label="其他" value="其他" />
          </el-select>
          <el-select v-model="filterYear" placeholder="年" style="width: 100px" @change="fetchBills">
            <el-option v-for="y in availableYears" :key="y" :label="y + '年'" :value="y" />
          </el-select>
          <el-select v-model="filterMonth" placeholder="月" style="width: 90px" @change="fetchBills">
            <el-option label="全部" value="" />
            <el-option v-for="m in 12" :key="m" :label="m + '月'" :value="m" />
          </el-select>
          <el-select v-model="filterDay" placeholder="日" style="width: 90px" @change="fetchBills">
            <el-option label="全部" value="" />
            <el-option v-for="d in 31" :key="d" :label="d + '日'" :value="d" />
          </el-select>
          <el-button type="primary" @click="showAddDialog = true">新增账单</el-button>
        </div>

        <div class="desktop-table">
          <el-table :data="bills" border stripe style="width: 100%" v-loading="billLoading">
            <el-table-column prop="bill_no" label="账单编号" width="140" />
            <el-table-column prop="bill_date" label="日期" width="110" />
            <el-table-column prop="type" label="类型" width="80">
              <template #default="{ row }">
                <el-tag :type="row.type === 'income' ? 'success' : 'danger'" size="small">
                  {{ row.type === 'income' ? '收入' : '支出' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="subtype" label="子类型" width="100" />
            <el-table-column prop="amount" label="金额" width="120">
              <template #default="{ row }">
                <span :style="{ color: row.type === 'income' ? '#67c23a' : '#f56c6c', fontWeight: 'bold' }">
                  {{ row.type === 'income' ? '+' : '-' }}{{ row.amount.toFixed(2) }}
                </span>
              </template>
            </el-table-column>
            <el-table-column prop="room" label="关联房间" width="100">
              <template #default="{ row }">{{ row.room?.room_number || '-' }}</template>
            </el-table-column>
            <el-table-column prop="description" label="备注" min-width="150" show-overflow-tooltip />
            <el-table-column label="操作" width="100" fixed="right">
              <template #default="{ row }">
                <el-button size="small" @click="handleEdit(row)">修改</el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>
        <div class="mobile-cards" v-loading="billLoading">
          <div v-for="row in bills" :key="row.id" class="bill-card">
            <div class="bc-head">
              <span class="bc-no">{{ row.bill_no }}</span>
              <el-tag :type="row.type === 'income' ? 'success' : 'danger'" size="small" effect="dark" round>
                {{ row.type === 'income' ? '收入' : '支出' }}
              </el-tag>
            </div>
            <div class="bc-info">
              <span>{{ row.bill_date }}</span>
              <span class="bc-subtype">{{ row.subtype }}</span>
              <span class="bc-room">{{ row.room?.room_number || '-' }}</span>
            </div>
            <div class="bc-body">
              <span :class="['bc-amount', row.type]">
                {{ row.type === 'income' ? '+' : '-' }}{{ row.amount.toFixed(2) }}
              </span>
              <el-button size="small" text @click="handleEdit(row)">修改</el-button>
            </div>
            <div v-if="row.description" class="bc-desc">{{ row.description }}</div>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="月度统计" name="monthly">
        <div style="margin-bottom: 16px">
          <el-date-picker v-model="statsMonth" type="month" format="YYYY-MM" value-format="YYYY-MM" @change="fetchStats" />
        </div>
        <div v-if="stats" style="display: flex; gap: 20px; margin-bottom: 20px">
          <el-card style="flex: 1">
            <div style="color: #67c23a; font-size: 14px">总收入</div>
            <div style="font-size: 28px; font-weight: bold; color: #67c23a">{{ stats.total_income.toFixed(2) }}</div>
          </el-card>
          <el-card style="flex: 1">
            <div style="color: #f56c6c; font-size: 14px">总支出</div>
            <div style="font-size: 28px; font-weight: bold; color: #f56c6c">{{ stats.total_expense.toFixed(2) }}</div>
          </el-card>
          <el-card style="flex: 1">
            <div style="color: #409eff; font-size: 14px">净利润</div>
            <div style="font-size: 28px; font-weight: bold; color: #409eff">{{ stats.net_profit.toFixed(2) }}</div>
          </el-card>
        </div>
        <el-row :gutter="20">
          <el-col :span="8">
            <el-card>
              <h4>收入明细</h4>
              <v-chart v-if="stats?.income_detail?.length" :option="incomePieOption(stats.income_detail)" style="height:240px" autoresize />
              <div v-else style="color: #999; padding: 10px; text-align:center">暂无数据</div>
            </el-card>
          </el-col>
          <el-col :span="8">
            <el-card>
              <h4>支出明细</h4>
              <v-chart v-if="stats?.expense_detail?.length" :option="expensePieOption(stats.expense_detail)" style="height:240px" autoresize />
              <div v-else style="color: #999; padding: 10px; text-align:center">暂无数据</div>
            </el-card>
          </el-col>
          <el-col :span="8">
            <el-card>
              <h4>收支对比</h4>
              <v-chart :option="compareOption(stats)" style="height:240px" autoresize />
            </el-card>
          </el-col>
        </el-row>
      </el-tab-pane>

      <el-tab-pane label="年度统计" name="yearly">
        <div style="margin-bottom: 16px">
          <el-date-picker v-model="statsYear" type="year" format="YYYY" value-format="YYYY" @change="fetchYearStats" />
        </div>
        <div v-if="yearStats" style="display: flex; gap: 20px; margin-bottom: 20px">
          <el-card style="flex: 1">
            <div style="color: #67c23a; font-size: 14px">年度总收入</div>
            <div style="font-size: 28px; font-weight: bold; color: #67c23a">{{ yearStats.total_income.toFixed(2) }}</div>
          </el-card>
          <el-card style="flex: 1">
            <div style="color: #f56c6c; font-size: 14px">年度总支出</div>
            <div style="font-size: 28px; font-weight: bold; color: #f56c6c">{{ yearStats.total_expense.toFixed(2) }}</div>
          </el-card>
          <el-card style="flex: 1">
            <div style="color: #409eff; font-size: 14px">年度净利润</div>
            <div style="font-size: 28px; font-weight: bold; color: #409eff">{{ yearStats.net_profit.toFixed(2) }}</div>
          </el-card>
        </div>
        <el-row :gutter="20">
          <el-col :span="8">
            <el-card>
              <h4>收入明细</h4>
              <v-chart v-if="yearStats?.income_detail?.length" :option="incomePieOption(yearStats.income_detail)" style="height:240px" autoresize />
              <div v-else style="color: #999; padding: 10px; text-align:center">暂无数据</div>
            </el-card>
          </el-col>
          <el-col :span="8">
            <el-card>
              <h4>支出明细</h4>
              <v-chart v-if="yearStats?.expense_detail?.length" :option="expensePieOption(yearStats.expense_detail)" style="height:240px" autoresize />
              <div v-else style="color: #999; padding: 10px; text-align:center">暂无数据</div>
            </el-card>
          </el-col>
          <el-col :span="8">
            <el-card>
              <h4>收支对比</h4>
              <v-chart :option="compareOption(yearStats)" style="height:240px" autoresize />
            </el-card>
          </el-col>
        </el-row>
      </el-tab-pane>

      <el-tab-pane label="收支趋势" name="trend">
        <div style="margin-bottom: 16px">
          <el-select v-model="trendRange" style="width: 140px" @change="fetchTrend">
            <el-option label="近12个月" value="12" />
            <el-option label="近24个月" value="24" />
          </el-select>
        </div>
        <el-card v-loading="trendLoading" style="margin-bottom: 20px">
          <h4 style="margin-bottom: 16px">月度收支趋势</h4>
          <v-chart v-if="trendData.length" :option="trendOption" style="height: 360px" autoresize />
          <div v-else style="text-align:center;padding:40px;color:#999">暂无趋势数据</div>
        </el-card>
        <el-card v-loading="trendLoading">
          <h4 style="margin-bottom: 16px">增长率分析</h4>
          <div class="desktop-table">
            <el-table :data="growthData" border stripe size="small" max-height="300">
              <el-table-column prop="month" label="月份" width="90" />
              <el-table-column label="收入环比" width="110">
                <template #default="{ row }">
                  <span :style="{ color: (row.income_mom||0) >= 0 ? '#67c23a' : '#f56c6c' }">
                    {{ row.income_mom != null ? (row.income_mom >= 0 ? '+' : '') + row.income_mom.toFixed(1) + '%' : '-' }}
                  </span>
                </template>
              </el-table-column>
              <el-table-column label="收入同比" width="110">
                <template #default="{ row }">
                  <span :style="{ color: (row.income_yoy||0) >= 0 ? '#67c23a' : '#f56c6c' }">
                    {{ row.income_yoy != null ? (row.income_yoy >= 0 ? '+' : '') + row.income_yoy.toFixed(1) + '%' : '-' }}
                  </span>
                </template>
              </el-table-column>
              <el-table-column label="支出环比" width="110">
                <template #default="{ row }">
                  <span :style="{ color: (row.expense_mom||0) >= 0 ? '#f56c6c' : '#67c23a' }">
                    {{ row.expense_mom != null ? (row.expense_mom >= 0 ? '+' : '') + row.expense_mom.toFixed(1) + '%' : '-' }}
                  </span>
                </template>
              </el-table-column>
              <el-table-column label="支出同比" width="110">
                <template #default="{ row }">
                  <span :style="{ color: (row.expense_yoy||0) >= 0 ? '#f56c6c' : '#67c23a' }">
                    {{ row.expense_yoy != null ? (row.expense_yoy >= 0 ? '+' : '') + row.expense_yoy.toFixed(1) + '%' : '-' }}
                  </span>
                </template>
              </el-table-column>
            </el-table>
          </div>
          <div class="mobile-cards">
            <div v-for="g in growthData" :key="g.month" class="growth-card">
              <div class="gc-month">{{ g.month }}</div>
              <div class="gc-grid">
                <div class="gc-item">
                  <span class="gc-label">收入环比</span>
                  <span :style="{ color: (g.income_mom||0) >= 0 ? '#67c23a' : '#f56c6c', fontWeight:600 }">
                    {{ g.income_mom != null ? (g.income_mom >= 0 ? '+' : '') + g.income_mom.toFixed(1) + '%' : '-' }}
                  </span>
                </div>
                <div class="gc-item">
                  <span class="gc-label">收入同比</span>
                  <span :style="{ color: (g.income_yoy||0) >= 0 ? '#67c23a' : '#f56c6c', fontWeight:600 }">
                    {{ g.income_yoy != null ? (g.income_yoy >= 0 ? '+' : '') + g.income_yoy.toFixed(1) + '%' : '-' }}
                  </span>
                </div>
                <div class="gc-item">
                  <span class="gc-label">支出环比</span>
                  <span :style="{ color: (g.expense_mom||0) >= 0 ? '#f56c6c' : '#67c23a', fontWeight:600 }">
                    {{ g.expense_mom != null ? (g.expense_mom >= 0 ? '+' : '') + g.expense_mom.toFixed(1) + '%' : '-' }}
                  </span>
                </div>
                <div class="gc-item">
                  <span class="gc-label">支出同比</span>
                  <span :style="{ color: (g.expense_yoy||0) >= 0 ? '#f56c6c' : '#67c23a', fontWeight:600 }">
                    {{ g.expense_yoy != null ? (g.expense_yoy >= 0 ? '+' : '') + g.expense_yoy.toFixed(1) + '%' : '-' }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </el-card>
      </el-tab-pane>

      <el-tab-pane label="分红预测" name="predict">
        <div style="margin-bottom: 16px">
          <el-radio-group v-model="predictMonths" @change="fetchPredict">
            <el-radio-button value="1">未来1个月</el-radio-button>
            <el-radio-button value="3">未来3个月</el-radio-button>
            <el-radio-button value="12">未来12个月</el-radio-button>
          </el-radio-group>
        </div>
        <el-card v-loading="predictLoading">
          <h4 style="margin-bottom: 16px">现金流预测 <span style="font-size:13px;color:#999;font-weight:400">（押金为负债，不参与净利润计算）</span></h4>
          <div class="desktop-table">
            <el-table :data="predictions" border stripe>
              <el-table-column prop="month" label="月份" width="100" />
              <el-table-column prop="rent" label="预计租金收入" width="140">
                <template #default="{ row }"><span style="color:#67c23a;font-weight:600">{{ row.rent.toFixed(2) }}</span></template>
              </el-table-column>
              <el-table-column prop="deposit" label="其中押金(负债)" width="140">
                <template #default="{ row }"><span style="color:#e6a23c">{{ row.deposit.toFixed(2) }}</span></template>
              </el-table-column>
              <el-table-column prop="available" label="可分配净利润" width="140">
                <template #default="{ row }"><span style="color:#409eff;font-weight:600">{{ row.available.toFixed(2) }}</span></template>
              </el-table-column>
            </el-table>
          </div>
          <div class="mobile-cards">
            <div v-for="p in predictions" :key="p.month" class="predict-card">
              <div class="pc-month">{{ p.month }}</div>
              <div class="pc-rows">
                <div class="pc-row"><span class="pc-label">预计租金收入</span><span class="pc-val pc-rent">¥{{ p.rent.toFixed(2) }}</span></div>
                <div class="pc-row"><span class="pc-label">其中押金(负债)</span><span class="pc-val pc-deposit">¥{{ p.deposit.toFixed(2) }}</span></div>
                <div class="pc-row pc-divider"><span class="pc-label">可分配净利润</span><span class="pc-val pc-avail">¥{{ p.available.toFixed(2) }}</span></div>
              </div>
            </div>
          </div>
          <div v-if="predictions.length" style="margin-top:16px;padding:12px;background:#f5f7fa;border-radius:8px;font-size:14px;color:#666">
            预测期可分配总额：<span style="color:#409eff;font-weight:600;font-size:18px">{{ predictTotal.toFixed(2) }}</span> 元
          </div>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="showAddDialog" :title="editingId ? '修改账单金额' : '新增账单'" width="480px">
      <el-form ref="billFormRef" :model="billForm" label-width="90px">
        <template v-if="!editingId">
          <el-form-item label="类型" prop="type" :rules="[{ required: true, message: '请选择类型' }]">
            <el-radio-group v-model="billForm.type">
              <el-radio value="income">收入</el-radio>
              <el-radio value="expense">支出</el-radio>
            </el-radio-group>
          </el-form-item>
          <el-form-item label="子类型" prop="subtype" :rules="[{ required: true, message: '请选择子类型' }]">
            <el-select v-model="billForm.subtype" style="width: 100%">
              <el-option v-for="s in subtypeOptions" :key="s" :label="s" :value="s" />
            </el-select>
          </el-form-item>
          <el-form-item label="金额" prop="amount" :rules="[{ required: true, message: '请输入金额' }]">
            <el-input-number v-model="billForm.amount" :min="0" :precision="2" style="width: 100%" />
          </el-form-item>
          <el-form-item label="账单日期" prop="bill_date" :rules="[{ required: true, message: '请选择日期' }]">
            <el-date-picker v-model="billForm.bill_date" type="date" format="YYYY-MM-DD" value-format="YYYY-MM-DD" style="width: 100%" />
          </el-form-item>
          <el-form-item label="关联房间" prop="room_id">
            <el-select v-model="billForm.room_id" placeholder="不选则不关联" clearable style="width: 100%">
              <el-option v-for="r in allRooms" :key="r.id" :label="r.room_number" :value="r.id" />
            </el-select>
          </el-form-item>
          <el-form-item label="备注" prop="description" :rules="billForm.subtype === '其他' ? [{ required: true, message: '子类型为其他时，备注不能为空' }] : []">
            <el-input v-model="billForm.description" type="textarea" :rows="2" />
          </el-form-item>
        </template>
        <template v-else>
          <el-form-item label="原金额">
            <span style="font-weight:600;color:#f56c6c">{{ billForm._old_amount.toFixed(2) }}</span>
          </el-form-item>
          <el-form-item label="新金额" prop="amount" :rules="[{ required: true, message: '请输入金额' }]">
            <el-input-number v-model="billForm.amount" :min="0" :precision="2" style="width: 100%" />
          </el-form-item>
          <el-form-item label="修改原因" prop="modify_reason"
            :rules="[{ required: true, message: '请填写修改原因' }]">
            <el-input v-model="billForm.modify_reason" type="textarea" :rows="3" placeholder="请填写修改原因" />
          </el-form-item>
          <div style="background:#f5f7fa;padding:10px 14px;border-radius:6px;font-size:13px;color:#666">
            修改后将在备注追加：<br>
            <template v-if="billForm.modify_reason">
              修改原因 {{ billForm.modify_reason }},金额从 {{ billForm._old_amount.toFixed(2) }} 变为 {{ billForm.amount.toFixed(2) }}
            </template>
            <template v-else style="color:#ccc">请填写修改原因</template>
          </div>
        </template>
      </el-form>
      <template #footer>
        <el-button @click="showAddDialog = false">取消</el-button>
        <el-button type="primary" :loading="billSubmitting" @click="handleSaveBill">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { buildingGetBills, buildingCreateBill, buildingUpdateBill, buildingDeleteBill, buildingGetBillStats, buildingGetBillTrend, buildingGetDividendPredict, buildingGetRooms } from '../api'
import { ElMessage, ElMessageBox } from 'element-plus'
import VChart from 'vue-echarts'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart, PieChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
use([CanvasRenderer, LineChart, PieChart, GridComponent, TooltipComponent, LegendComponent])

const activeTab = ref('list')
const bills = ref([])
const billLoading = ref(false)
const showAddDialog = ref(false)
const billSubmitting = ref(false)
const editingId = ref(null)
const billFormRef = ref(null)
const allRooms = ref([])

const filter = reactive({ type: '', subtype: '' })
const now = new Date()
const filterYear = ref(now.getFullYear())
const filterMonth = ref(now.getMonth() + 1)
const filterDay = ref('')
const availableYears = computed(() => {
  const y = now.getFullYear()
  return [y - 10, y - 9, y - 8, y - 7, y - 6, y - 5, y - 4, y - 3, y - 2, y - 1, y, y + 1]
})
const billForm = reactive({
  type: 'income', subtype: '', amount: 0, bill_date: '', room_id: null, description: '', modify_reason: '', _old_amount: 0,
})

const subtypeOptions = computed(() => {
  return billForm.type === 'income'
    ? ['租金', '定金', '押金', '水电费', '其他']
    : ['物业费', '维修费', '清洁费', '税费', '其他']
})

const statsMonth = ref('')
const stats = ref(null)
const statsYear = ref('')
const yearStats = ref(null)

// 趋势图表
const trendRange = ref('12')
const trendLoading = ref(false)
const trendData = ref([])
const growthData = ref([])

const trendOption = computed(() => ({
  tooltip: { trigger: 'axis' },
  legend: { data: ['收入', '支出', '净利润'] },
  grid: { left: 50, right: 20, bottom: 30, top: 40 },
  xAxis: { type: 'category', data: trendData.value.map(d => d.month), axisLabel: { rotate: 45 } },
  yAxis: { type: 'value' },
  series: [
    { name: '收入', type: 'line', data: trendData.value.map(d => d.income), itemStyle: { color: '#67c23a' }, smooth: true },
    { name: '支出', type: 'line', data: trendData.value.map(d => d.expense), itemStyle: { color: '#f56c6c' }, smooth: true },
    { name: '净利润', type: 'line', data: trendData.value.map(d => d.profit), itemStyle: { color: '#409eff' }, smooth: true, lineStyle: { type: 'dashed' } },
  ],
}))

// 分红预测
const predictMonths = ref('3')
const predictLoading = ref(false)
const predictions = ref([])
const predictTotal = computed(() => predictions.value.reduce((s, p) => s + p.available, 0))

// 饼图
function pieOption(data, title, color) {
  return {
    tooltip: { trigger: 'item', formatter: '{b}: {c}元 ({d}%)' },
    series: [{
      type: 'pie', radius: ['30%', '70%'], center: ['50%', '55%'],
      data: (data || []).map(d => ({ name: d.subtype, value: d.total })),
      itemStyle: { borderRadius: 4, borderColor: '#fff', borderWidth: 2 },
      label: { show: true, formatter: '{b}\n{d}%', fontSize: 11 },
    }],
    color: color || ['#67c23a', '#409eff', '#e6a23c', '#f56c6c', '#909399'],
  }
}
function incomePieOption(data) { return pieOption(data, '收入构成', ['#67c23a', '#67CC6A', '#67D67A', '#67E08A', '#67EA9A']) }
function expensePieOption(data) { return pieOption(data, '支出构成', ['#f56c6c', '#f08080', '#e9967a', '#eea2ad', '#f4b4c2']) }
function compareOption(stats) {
  if (!stats) return {}
  return {
    tooltip: { trigger: 'item', formatter: '{b}: {c}元 ({d}%)' },
    series: [{
      type: 'pie', radius: ['30%', '70%'],
      data: [
        { name: '收入', value: stats.total_income || 0, itemStyle: { color: '#67c23a' } },
        { name: '支出', value: stats.total_expense || 0, itemStyle: { color: '#f56c6c' } },
      ],
      label: { show: true, formatter: '{b}\n{d}%', fontSize: 13 },
    }],
  }
}

async function fetchBills() {
  billLoading.value = true
  try {
    const params = {}
    if (filter.type) params.type = filter.type
    if (filter.subtype) params.subtype = filter.subtype
    const y = filterYear.value
    const m = filterMonth.value
    const d = filterDay.value
    if (d) {
      params.start_date = `${y}-${String(m).padStart(2, '0')}-${String(d).padStart(2, '0')}`
      params.end_date = params.start_date
    } else if (m) {
      const lastDay = new Date(y, m, 0).getDate()
      params.start_date = `${y}-${String(m).padStart(2, '0')}-01`
      params.end_date = `${y}-${String(m).padStart(2, '0')}-${String(lastDay).padStart(2, '0')}`
    } else {
      params.start_date = `${y}-01-01`
      params.end_date = `${y}-12-31`
    }
    const res = await buildingGetBills(params)
    bills.value = res.data.bills
  } catch {
    ElMessage.error('获取账单列表失败')
  } finally {
    billLoading.value = false
  }
}

async function fetchStats() {
  if (!statsMonth.value) return
  try {
    const res = await buildingGetBillStats(statsMonth.value)
    stats.value = res.data
  } catch {
    ElMessage.error('获取月度统计失败')
  }
}

async function fetchYearStats() {
  if (!statsYear.value) return
  try {
    const res = await buildingGetBillStats(null, statsYear.value)
    yearStats.value = res.data
  } catch {
    ElMessage.error('获取年度统计失败')
  }
}

async function fetchTrend() {
  trendLoading.value = true
  try {
    const res = await buildingGetBillTrend({ years: trendRange.value })
    trendData.value = res.data.months || []
    growthData.value = res.data.growth || []
  } catch {
    ElMessage.error('获取趋势数据失败')
  } finally {
    trendLoading.value = false
  }
}

async function fetchPredict() {
  predictLoading.value = true
  try {
    const res = await buildingGetDividendPredict({ months: predictMonths.value })
    predictions.value = res.data.predictions || []
  } catch {
    ElMessage.error('获取预测数据失败')
  } finally {
    predictLoading.value = false
  }
}

async function handleSaveBill() {
  const valid = await billFormRef.value.validate().catch(() => false)
  if (!valid) return
  billSubmitting.value = true
  try {
    if (editingId.value) {
      await buildingUpdateBill(editingId.value, billForm)
      ElMessage.success('更新成功')
    } else {
      await buildingCreateBill(billForm)
      ElMessage.success('创建成功')
    }
    showAddDialog.value = false
    resetBillForm()
    await fetchBills()
    await fetchStats()
    await fetchYearStats()
  } catch {
    ElMessage.error(editingId.value ? '更新失败' : '创建失败')
  } finally {
    billSubmitting.value = false
  }
}

function handleEdit(row) {
  editingId.value = row.id
  billForm.type = row.type
  billForm.subtype = row.subtype
  billForm.amount = row.amount
  billForm.bill_date = row.bill_date
  billForm.room_id = row.room_id
  billForm.description = row.description || ''
  billForm.modify_reason = ''
  billForm._old_amount = row.amount
  showAddDialog.value = true
}

async function handleDelete(id) {
  try {
    await ElMessageBox.confirm('确认删除该账单？', '提示')
    await buildingDeleteBill(id)
    ElMessage.success('删除成功')
    await fetchBills()
    await fetchStats()
    await fetchYearStats()
  } catch {
    ElMessage.error('删除失败')
  }
}

function resetBillForm() {
  editingId.value = null
  billForm.type = 'income'
  billForm.subtype = ''
  billForm.amount = 0
  billForm.bill_date = ''
  billForm.room_id = null
  billForm.description = ''
  billForm.modify_reason = ''
  billForm._old_amount = 0
}

function handleTabChange(name) {
  if (name === 'monthly') {
    fetchStats()
  } else if (name === 'yearly') {
    fetchYearStats()
  } else if (name === 'trend') {
    fetchTrend()
  } else if (name === 'predict') {
    fetchPredict()
  }
}

onMounted(async () => {
  await fetchBills()
  try {
    const res = await buildingGetRooms()
    allRooms.value = res.data.rooms
  } catch {
    ElMessage.error('获取房间列表失败')
  }
  const now = new Date()
  statsMonth.value = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
  statsYear.value = String(now.getFullYear())
  await fetchStats()
  await fetchYearStats()
})
</script>

<style scoped>
.desktop-table { display: block; }
.mobile-cards { display: none; }

.bill-card {
  background: #fff;
  border-radius: 10px;
  padding: 12px 14px;
  margin-bottom: 10px;
  border: 1px solid #eee;
  box-shadow: 0 1px 4px rgba(0,0,0,0.04);
}
.bc-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
}
.bc-no {
  font-size: 12px;
  font-weight: 600;
  color: #999;
  letter-spacing: 0.3px;
}
.bc-info {
  display: flex;
  gap: 8px;
  font-size: 12px;
  color: #999;
  margin-bottom: 8px;
}
.bc-subtype, .bc-room {
  color: #666;
}
.bc-body {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.bc-amount {
  font-size: 18px;
  font-weight: 700;
}
.bc-amount.income { color: #67c23a; }
.bc-amount.expense { color: #f56c6c; }
.bc-desc {
  font-size: 12px;
  color: #999;
  margin-top: 6px;
  padding-top: 6px;
  border-top: 1px solid #f5f5f5;
}

.growth-card {
  background: #fff;
  border-radius: 10px;
  padding: 12px 14px;
  margin-bottom: 10px;
  border: 1px solid #eee;
  box-shadow: 0 1px 4px rgba(0,0,0,0.04);
}
.gc-month {
  font-size: 14px;
  font-weight: 600;
  color: #333;
  margin-bottom: 8px;
}
.gc-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}
.gc-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.gc-label {
  font-size: 11px;
  color: #999;
}

.predict-card {
  background: #fff;
  border-radius: 10px;
  padding: 12px 14px;
  margin-bottom: 10px;
  border: 1px solid #eee;
  box-shadow: 0 1px 4px rgba(0,0,0,0.04);
}
.pc-month {
  font-size: 14px;
  font-weight: 600;
  color: #333;
  margin-bottom: 8px;
}
.pc-rows { display: flex; flex-direction: column; gap: 6px; }
.pc-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.pc-label { font-size: 13px; color: #666; }
.pc-val { font-size: 14px; font-weight: 600; }
.pc-rent { color: #67c23a; }
.pc-deposit { color: #e6a23c; }
.pc-avail { color: #409eff; }
.pc-divider {
  padding-top: 6px;
  border-top: 1px solid #f5f5f5;
}

@media (max-width: 768px) {
  .desktop-table { display: none; }
  .mobile-cards { display: block; }
  .el-tabs__item { font-size: 13px; padding: 0 12px; }
  .el-table .el-table__cell { font-size: 12px; }
  .el-table-column--fixed { display: none; }
  .el-row { flex-direction: column; }
  .el-col { width: 100% !important; max-width: 100% !important; flex: 0 0 100% !important; }
}
</style>
