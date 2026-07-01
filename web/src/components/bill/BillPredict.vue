<template>
  <div>
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
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { buildingGetDividendPredict } from '../../api'

const predictMonths = ref('3')
const predictLoading = ref(false)
const predictions = ref([])
const predictTotal = computed(() => predictions.value.reduce((s, p) => s + p.available, 0))

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

onMounted(fetchPredict)
</script>

<style scoped>
.desktop-table { display: block; }
.mobile-cards { display: none; }

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
}
</style>
