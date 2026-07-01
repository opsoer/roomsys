<template>
  <div>
    <div style="display: flex; gap: 10px; margin-bottom: 16px; flex-wrap: wrap; align-items: center">
      <el-select v-model="filter.type" placeholder="类型" clearable style="width: 120px" @change="$emit('search')">
        <el-option label="全部" value="" />
        <el-option label="收入" value="income" />
        <el-option label="支出" value="expense" />
      </el-select>
      <el-select v-model="filter.subtype" placeholder="子类型" clearable style="width: 140px" @change="$emit('search')">
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
      <el-select v-model="filterYear" placeholder="年" style="width: 100px" @change="$emit('search')">
        <el-option v-for="y in availableYears" :key="y" :label="y + '年'" :value="y" />
      </el-select>
      <el-select v-model="filterMonth" placeholder="月" style="width: 90px" @change="$emit('search')">
        <el-option label="全部" value="" />
        <el-option v-for="m in 12" :key="m" :label="m + '月'" :value="m" />
      </el-select>
      <el-select v-model="filterDay" placeholder="日" style="width: 90px" @change="$emit('search')">
        <el-option label="全部" value="" />
        <el-option v-for="d in 31" :key="d" :label="d + '日'" :value="d" />
      </el-select>
      <el-button type="primary" @click="$emit('add')">新增账单</el-button>
    </div>

    <div class="desktop-table">
      <el-table :data="bills" border stripe style="width: 100%" v-loading="loading">
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
            <el-button size="small" @click="$emit('edit', row)">修改</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>
    <div class="mobile-cards" v-loading="loading">
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
          <el-button size="small" text @click="$emit('edit', row)">修改</el-button>
        </div>
        <div v-if="row.description" class="bc-desc">{{ row.description }}</div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed } from 'vue'
import dayjs from 'dayjs'

const props = defineProps({
  bills: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false },
})

defineEmits(['search', 'add', 'edit'])

const filter = reactive({ type: '', subtype: '' })
const now = dayjs()
const filterYear = ref(now.year())
const filterMonth = ref(now.month() + 1)
const filterDay = ref('')

const availableYears = computed(() => {
  const y = now.year()
  return [y - 10, y - 9, y - 8, y - 7, y - 6, y - 5, y - 4, y - 3, y - 2, y - 1, y, y + 1]
})

function getFilterParams() {
  const params = {}
  if (filter.type) params.type = filter.type
  if (filter.subtype) params.subtype = filter.subtype
  const y = filterYear.value
  const m = filterMonth.value
  const d = filterDay.value
  if (d) {
    params.start_date = dayjs(`${y}-${String(m).padStart(2, '0')}-${String(d).padStart(2, '0')}`).format('YYYY-MM-DD')
    params.end_date = params.start_date
  } else if (m) {
    const lastDay = dayjs(`${y}-${String(m).padStart(2, '0')}-01`).daysInMonth()
    params.start_date = dayjs(`${y}-${String(m).padStart(2, '0')}-01`).format('YYYY-MM-DD')
    params.end_date = dayjs(`${y}-${String(m).padStart(2, '0')}-${String(lastDay).padStart(2, '0')}`).format('YYYY-MM-DD')
  } else {
    params.start_date = dayjs(`${y}-01-01`).format('YYYY-MM-DD')
    params.end_date = dayjs(`${y}-12-31`).format('YYYY-MM-DD')
  }
  return params
}

defineExpose({ getFilterParams })
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

@media (max-width: 768px) {
  .desktop-table { display: none; }
  .mobile-cards { display: block; }
}
</style>
