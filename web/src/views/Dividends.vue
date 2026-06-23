<template>
  <div>
    <h3 style="margin-bottom: 20px">分红管理</h3>

    <el-card style="margin-bottom: 20px">
      <h4 style="margin-bottom: 12px">分红计算</h4>
      <div style="display: flex; gap: 10px; align-items: center">
        <el-date-picker v-model="calcMonth" type="month" format="YYYY-MM" value-format="YYYY-MM" placeholder="选择月份" />
        <el-button type="primary" @click="handleCalculate">查看分红</el-button>
      </div>

      <div v-if="preview" style="margin-top: 16px">
        <el-descriptions :column="3" border size="small">
          <el-descriptions-item label="总收入">{{ preview.total_income.toFixed(2) }}</el-descriptions-item>
          <el-descriptions-item label="总支出">{{ preview.total_expense.toFixed(2) }}</el-descriptions-item>
          <el-descriptions-item label="净利润">{{ preview.net_profit.toFixed(2) }}</el-descriptions-item>
        </el-descriptions>
        <el-table v-if="preview.results?.length" :data="preview.results" border stripe style="margin-top: 12px">
          <el-table-column prop="name" label="股东" />
          <el-table-column prop="share_ratio" label="持股比例(%)" />
          <el-table-column prop="dividend_amount" label="分红金额">
            <template #default="{ row }">{{ row.dividend_amount.toFixed(2) }}</template>
          </el-table-column>
        </el-table>
        <div v-else style="margin-top: 12px; color: #999">本月无净利润，不分红</div>
      </div>
    </el-card>

    <el-card>
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px">
        <h4>股东配置</h4>
        <el-button size="small" type="primary" @click="showShareholderDialog = true">添加股东</el-button>
      </div>
      <el-table :data="shareholders" border stripe>
        <el-table-column prop="name" label="股东姓名" />
        <el-table-column prop="share_ratio" label="持股比例(%)" />
        <el-table-column label="操作" width="160">
          <template #default="{ row }">
            <el-button size="small" type="primary" text @click="handleEditSH(row)">编辑</el-button>
            <el-button size="small" type="danger" text @click="handleDeleteSH(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card style="margin-top: 20px">
      <h4 style="margin-bottom: 12px">历史分红记录</h4>
      <el-table :data="dividends" border stripe>
        <el-table-column prop="settle_month" label="结算月份" width="120" />
        <el-table-column prop="total_income" label="总收入" />
        <el-table-column prop="total_expense" label="总支出" />
        <el-table-column prop="net_profit" label="净利润" />
        <el-table-column prop="shareholder.name" label="股东" />
        <el-table-column prop="dividend_amount" label="分红金额" />
        <el-table-column prop="created_at" label="结算时间" width="180">
          <template #default="{ row }">{{ row.created_at }}</template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="showShareholderDialog" :title="editingSHId ? '编辑股东' : '添加股东'" width="400px">
      <el-form ref="shFormRef" :model="shForm" label-width="90px">
        <el-form-item label="姓名" prop="name" :rules="[{ required: true, message: '请输入姓名' }]">
          <el-input v-model="shForm.name" />
        </el-form-item>
        <el-form-item label="持股比例(%)" prop="share_ratio" :rules="[{ required: true, message: '请输入比例' }]">
          <el-input-number v-model="shForm.share_ratio" :min="0" :max="100" :precision="2" style="width: 100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showShareholderDialog = false">取消</el-button>
        <el-button type="primary" :loading="shSubmitting" @click="handleAddSH">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { buildingGetDividends, buildingCalculateDividend, buildingGetShareholders, buildingCreateShareholder, buildingUpdateShareholder, buildingDeleteShareholder } from '../api'
import { ElMessage, ElMessageBox } from 'element-plus'

const calcMonth = ref('')
const preview = ref(null)
const dividends = ref([])
const shareholders = ref([])
const showShareholderDialog = ref(false)
const shSubmitting = ref(false)
const shForm = ref({ name: '', share_ratio: 0 })
const shFormRef = ref(null)
const editingSHId = ref(null)

async function handleCalculate() {
  if (!calcMonth.value) {
    ElMessage.warning('请选择月份')
    return
  }
  const res = await buildingCalculateDividend(calcMonth.value)
  preview.value = res.data
}

async function fetchDividends() {
  const res = await buildingGetDividends()
  dividends.value = res.data.dividends
}

async function fetchShareholders() {
  const res = await buildingGetShareholders()
  shareholders.value = res.data.shareholders
}

async function handleAddSH() {
  const valid = await shFormRef.value.validate().catch(() => false)
  if (!valid) return
  shSubmitting.value = true
  try {
    if (editingSHId.value) {
      await buildingUpdateShareholder(editingSHId.value, shForm.value)
      ElMessage.success('修改成功')
    } else {
      await buildingCreateShareholder(shForm.value)
      ElMessage.success('添加成功')
    }
    showShareholderDialog.value = false
    editingSHId.value = null
    shForm.value = { name: '', share_ratio: 0 }
    await fetchShareholders()
  } finally {
    shSubmitting.value = false
  }
}

function handleEditSH(row) {
  editingSHId.value = row.id
  shForm.value = { name: row.name, share_ratio: row.share_ratio }
  showShareholderDialog.value = true
}

async function handleDeleteSH(id) {
  try {
    await ElMessageBox.confirm('确认删除该股东？', '提示')
    await buildingDeleteShareholder(id)
    ElMessage.success('删除成功')
    await fetchShareholders()
  } catch {}
}

onMounted(() => {
  fetchDividends()
  fetchShareholders()
})
</script>
