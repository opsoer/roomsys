<template>
  <el-dialog v-model="visible" :title="editingId ? '修改账单金额' : '新增账单'" width="480px">
    <el-form ref="formRef" :model="form" label-width="90px">
      <template v-if="!editingId">
        <el-form-item label="类型" prop="type" :rules="[{ required: true, message: '请选择类型' }]">
          <el-radio-group v-model="form.type">
            <el-radio value="income">收入</el-radio>
            <el-radio value="expense">支出</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="子类型" prop="subtype" :rules="[{ required: true, message: '请选择子类型' }]">
          <el-select v-model="form.subtype" style="width: 100%">
            <el-option v-for="s in subtypeOptions" :key="s" :label="s" :value="s" />
          </el-select>
        </el-form-item>
        <el-form-item label="金额" prop="amount" :rules="[{ required: true, message: '请输入金额' }]">
          <el-input-number v-model="form.amount" :min="0.01" :precision="2" style="width: 100%" />
        </el-form-item>
        <el-form-item label="账单日期" prop="bill_date" :rules="[{ required: true, message: '请选择日期' }]">
          <el-date-picker v-model="form.bill_date" type="date" format="YYYY-MM-DD" value-format="YYYY-MM-DD" style="width: 100%" />
        </el-form-item>
        <el-form-item label="关联房间" prop="room_id">
          <el-select v-model="form.room_id" placeholder="不选则不关联" clearable style="width: 100%">
            <el-option v-for="r in allRooms" :key="r.id" :label="r.room_number" :value="r.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="备注" prop="description" :rules="form.subtype === '其他' ? [{ required: true, message: '子类型为其他时，备注不能为空' }] : []">
          <el-input v-model="form.description" type="textarea" :rows="2" />
        </el-form-item>
      </template>
      <template v-else>
        <el-form-item label="原金额">
          <span style="font-weight:600;color:#f56c6c">{{ form._old_amount.toFixed(2) }}</span>
        </el-form-item>
        <el-form-item label="新金额" prop="amount" :rules="[{ required: true, message: '请输入金额' }]">
          <el-input-number v-model="form.amount" :min="0.01" :precision="2" style="width: 100%" />
        </el-form-item>
        <el-form-item label="修改原因" prop="modify_reason"
          :rules="[{ required: true, message: '请填写修改原因' }]">
          <el-input v-model="form.modify_reason" type="textarea" :rows="3" placeholder="请填写修改原因" />
        </el-form-item>
        <div style="background:#f5f7fa;padding:10px 14px;border-radius:6px;font-size:13px;color:#666">
          修改后将在备注追加：<br>
          <template v-if="form.modify_reason">
            修改原因 {{ form.modify_reason }},金额从 {{ form._old_amount.toFixed(2) }} 变为 {{ form.amount.toFixed(2) }}
          </template>
          <template v-else style="color:#ccc">请填写修改原因</template>
        </div>
      </template>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleSave">确定</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { buildingCreateBill, buildingUpdateBill } from '../../api'

const props = defineProps({
  allRooms: { type: Array, default: () => [] },
})

const emit = defineEmits(['save-success'])

const visible = ref(false)
const submitting = ref(false)
const editingId = ref(null)
const formRef = ref(null)

const form = reactive({
  type: 'income', subtype: '', amount: 0, bill_date: '', room_id: null, description: '', modify_reason: '', _old_amount: 0,
})

const subtypeOptions = computed(() => {
  return form.type === 'income'
    ? ['租金', '定金', '押金', '水电费', '其他']
    : ['物业费', '维修费', '清洁费', '税费', '其他']
})

function open() {
  editingId.value = null
  form.type = 'income'
  form.subtype = ''
  form.amount = 0
  form.bill_date = ''
  form.room_id = null
  form.description = ''
  form.modify_reason = ''
  form._old_amount = 0
  visible.value = true
}

function openEdit(row) {
  editingId.value = row.id
  form.type = row.type
  form.subtype = row.subtype
  form.amount = row.amount
  form.bill_date = row.bill_date
  form.room_id = row.room_id
  form.description = row.description || ''
  form.modify_reason = ''
  form._old_amount = row.amount
  visible.value = true
}

async function handleSave() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return
  submitting.value = true
  try {
    if (editingId.value) {
      await buildingUpdateBill(editingId.value, form)
      ElMessage.success('更新成功')
    } else {
      await buildingCreateBill(form)
      ElMessage.success('创建成功')
    }
    visible.value = false
    emit('save-success')
  } catch {
    ElMessage.error(editingId.value ? '更新失败' : '创建失败')
  } finally {
    submitting.value = false
  }
}

defineExpose({ open, openEdit })
</script>
