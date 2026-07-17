<template>
  <div>
    <el-dialog v-model="showRentDialog" title="设为已出租" width="500px">
      <el-form ref="rentFormRef" :model="rentForm" label-width="100px">
        <el-form-item label="租客姓名" prop="tenant_name" :rules="[{ required: true, message: '请输入租客姓名' }]">
          <el-input v-model="rentForm.tenant_name" />
        </el-form-item>
        <el-form-item label="联系电话" prop="tenant_phone">
          <el-input v-model="rentForm.tenant_phone" />
        </el-form-item>
        <el-form-item label="月租金" prop="rent_price" :rules="[{ required: true, message: '请输入租金' }]">
          <el-input-number v-model="rentForm.rent_price" :min="0" :precision="2" style="width:100%" />
        </el-form-item>
        <el-form-item label="管理费" prop="management_fee" :rules="[{ required: true, message: '请输入管理费' }]">
          <el-input-number v-model="rentForm.management_fee" :min="0" :precision="2" style="width:100%" />
        </el-form-item>
        <el-form-item label="押金" prop="deposit" :rules="[{ required: true, message: '请输入押金金额' }]">
          <el-input-number v-model="rentForm.deposit" :min="0" :precision="2" style="width:100%" />
        </el-form-item>
        <el-form-item label="起租日期" prop="start_date" :rules="[{ required: true, message: '请选择起租日期' }]">
          <el-date-picker v-model="rentForm.start_date" type="date" format="YYYY-MM-DD" value-format="YYYY-MM-DD" style="width:100%" />
        </el-form-item>
        <el-form-item label="结束日期" prop="end_date" :rules="[{ required: true, message: '请选择结束日期' }]">
          <el-date-picker v-model="rentForm.end_date" type="date" format="YYYY-MM-DD" value-format="YYYY-MM-DD" style="width:100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showRentDialog = false">取消</el-button>
        <el-button type="primary" :loading="rentSubmitting" @click="handleRent">确定出租</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showReserveDialog" title="交定金 / 预订" width="500px">
      <el-form ref="reserveFormRef" :model="reserveForm" label-width="100px">
        <el-form-item label="租客姓名" prop="tenant_name" :rules="[{ required: true, message: '请输入租客姓名' }]">
          <el-input v-model="reserveForm.tenant_name" />
        </el-form-item>
        <el-form-item label="联系电话" prop="tenant_phone">
          <el-input v-model="reserveForm.tenant_phone" />
        </el-form-item>
        <el-form-item label="定金金额" prop="earnest_money" :rules="[{ required: true, message: '请输入定金金额' }, { validator: (_, v) => v > 0, message: '定金必须大于0' }]">
          <el-input-number v-model="reserveForm.earnest_money" :min="0" :precision="2" style="width:100%" />
        </el-form-item>
        <el-divider>约定信息（正式签约时可微调）</el-divider>
        <el-form-item label="月租金" prop="rent_price" :rules="[{ required: true, message: '请输入租金' }]">
          <el-input-number v-model="reserveForm.rent_price" :min="0" :precision="2" style="width:100%" />
        </el-form-item>
        <el-form-item label="管理费" prop="management_fee">
          <el-input-number v-model="reserveForm.management_fee" :min="0" :precision="2" style="width:100%" />
        </el-form-item>
        <el-form-item label="押金" prop="deposit" :rules="[{ required: true, message: '请输入押金金额' }]">
          <el-input-number v-model="reserveForm.deposit" :min="0" :precision="2" style="width:100%" />
        </el-form-item>
        <el-form-item label="预计起租" prop="start_date" :rules="[{ required: true, message: '请选择起租日期' }]">
          <el-date-picker v-model="reserveForm.start_date" type="date" format="YYYY-MM-DD" value-format="YYYY-MM-DD" style="width:100%" />
        </el-form-item>
        <el-form-item label="预计结束" prop="end_date" :rules="[{ required: true, message: '请选择结束日期' }]">
          <el-date-picker v-model="reserveForm.end_date" type="date" format="YYYY-MM-DD" value-format="YYYY-MM-DD" style="width:100%" />
        </el-form-item>
      </el-form>
      <div class="reserve-hint">交定金阶段不产生账单，正式签约入住后定金将抵扣押金</div>
      <template #footer>
        <el-button @click="showReserveDialog = false">取消</el-button>
        <el-button type="primary" :loading="reserveSubmitting" @click="handleReserve">确定预订</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showConfirmSignDialog" title="确认签约入住" width="500px">
      <el-form ref="signFormRef" :model="signForm" label-width="100px">
        <el-form-item label="租客姓名" prop="tenant_name" :rules="[{ required: true, message: '请输入租客姓名' }]">
          <el-input v-model="signForm.tenant_name" />
        </el-form-item>
        <el-form-item label="联系电话" prop="tenant_phone">
          <el-input v-model="signForm.tenant_phone" />
        </el-form-item>
        <el-form-item label="月租金" prop="rent_price" :rules="[{ required: true, message: '请输入租金' }]">
          <el-input-number v-model="signForm.rent_price" :min="0" :precision="2" style="width:100%" />
        </el-form-item>
        <el-form-item label="管理费" prop="management_fee">
          <el-input-number v-model="signForm.management_fee" :min="0" :precision="2" style="width:100%" />
        </el-form-item>
        <el-form-item label="押金" prop="deposit" :rules="[{ required: true, message: '请输入押金金额' }]">
          <el-input-number v-model="signForm.deposit" :min="0" :precision="2" style="width:100%" />
        </el-form-item>
        <el-form-item label="起租日期" prop="start_date" :rules="[{ required: true, message: '请选择起租日期' }]">
          <el-date-picker v-model="signForm.start_date" type="date" format="YYYY-MM-DD" value-format="YYYY-MM-DD" style="width:100%" />
        </el-form-item>
        <el-form-item label="结束日期" prop="end_date" :rules="[{ required: true, message: '请选择结束日期' }]">
          <el-date-picker v-model="signForm.end_date" type="date" format="YYYY-MM-DD" value-format="YYYY-MM-DD" style="width:100%" />
        </el-form-item>
      </el-form>
      <div class="reserve-hint">
        已收定金 <strong>{{ Number(earnestMoney).toFixed(2) }}</strong> 元，将抵扣押金；
        实收押金 <strong>{{ Math.max(0, (signForm.deposit || 0) - earnestMoney).toFixed(2) }}</strong> 元
      </div>
      <template #footer>
        <el-button @click="showConfirmSignDialog = false">取消</el-button>
        <el-button type="primary" :loading="signSubmitting" @click="handleConfirmSign">确认签约</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showCancelReserveDialog" title="取消预订" width="450px">
      <div class="vacate-deposit-info">
        <div class="vacate-row">
          <span>已收定金</span>
          <span class="vacate-amount">{{ Number(earnestMoney).toFixed(2) }} 元</span>
        </div>
        <p class="vacate-hint">请填写实际退还给租客的定金金额，系统将按差额自动记账</p>
      </div>
      <el-form ref="cancelFormRef" :model="cancelForm" label-width="100px">
        <el-form-item label="退还定金" prop="refunded_deposit">
          <el-input-number v-model="cancelForm.refunded_deposit" :min="0" :precision="2" style="width:100%" />
        </el-form-item>
      </el-form>
      <div v-if="earnestMoney - cancelForm.refunded_deposit > 0" class="vacate-deduction-note">
        扣留违约金 <strong>{{ (earnestMoney - cancelForm.refunded_deposit).toFixed(2) }}</strong> 元，将创建定金违约收入账单
      </div>
      <div v-else-if="cancelForm.refunded_deposit - earnestMoney > 0" class="vacate-deduction-note">
        多退 <strong>{{ (cancelForm.refunded_deposit - earnestMoney).toFixed(2) }}</strong> 元，将创建定金违约支出账单
      </div>
      <template #footer>
        <el-button @click="showCancelReserveDialog = false">取消</el-button>
        <el-button type="warning" :loading="cancelSubmitting" @click="handleCancelReserve">确定取消预订</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showEndDateDialog" title="修改退租时间" width="420px">
      <el-form ref="endDateFormRef" :model="endDateForm" label-width="100px">
        <el-form-item label="退租日期" prop="end_date" :rules="[{ required: true, message: '请选择退租日期' }]">
          <el-date-picker v-model="endDateForm.end_date" type="date" format="YYYY-MM-DD" value-format="YYYY-MM-DD" placeholder="选择退租日期" style="width:100%" />
        </el-form-item>
        <el-form-item label="退租租金" prop="rent_price" :rules="[{ required: true, message: '请输入退租租金' }]">
          <el-input-number v-model="endDateForm.rent_price" :min="0" :precision="2" style="width:100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showEndDateDialog = false">取消</el-button>
        <el-button type="primary" :loading="endDateSubmitting" @click="handleUpdateEndDate">确定修改</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showVacateDialog" title="设为未出租" width="450px">
      <div>
        <div v-if="currentContract?.deposit" class="vacate-deposit-info">
          <div class="vacate-row">
            <span>原押金</span>
            <span class="vacate-amount">{{ currentContract.deposit.toFixed(2) }} 元</span>
          </div>
          <p class="vacate-hint">如因卫生或家具损坏需扣除部分押金，请填写实际退还金额</p>
        </div>
        <el-form ref="vacateFormRef" :model="vacateForm" label-width="100px">
          <el-form-item label="退还押金" prop="refunded_deposit">
            <el-input-number v-model="vacateForm.refunded_deposit" :min="0" :precision="2" style="width:100%" />
          </el-form-item>
        </el-form>
        <div v-if="vacateForm.refunded_deposit > 0" class="vacate-deduction-note">
          将自动创建 <strong>{{ Number(vacateForm.refunded_deposit).toFixed(2) }}</strong> 元的押金支出账单
        </div>
      </div>
      <template #footer>
        <el-button @click="showVacateDialog = false">取消</el-button>
        <el-button type="primary" :loading="vacateSubmitting" @click="confirmVacate">确定退租</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showEditDialog" title="编辑房间" width="500px">
      <el-form ref="editFormRef" :model="editForm" label-width="90px">
        <el-form-item label="房间号" prop="room_number">
          <el-input v-model="editForm.room_number" />
        </el-form-item>
        <el-form-item label="楼层" prop="floor">
          <el-select v-model="editForm.floor" placeholder="选择楼层" style="width:100%">
            <el-option v-for="f in floorOptions" :key="f" :label="f + '层'" :value="String(f)" />
          </el-select>
        </el-form-item>
        <el-form-item label="户型" prop="layout">
          <el-select v-model="editForm.layout" placeholder="选择户型" style="width:100%">
            <el-option v-for="lo in layoutOptions" :key="lo" :label="lo" :value="lo" />
          </el-select>
        </el-form-item>
        <el-divider>价格设置</el-divider>
        <el-form-item label="租金（月）" prop="rent_price" required
          :rules="[{ required: true, message: '请输入月租金' }, { validator: (_, v) => v > 0, message: '租金必须大于0' }]">
          <el-input :model-value="editForm.rent_price" @update:model-value="v => editForm.rent_price = v === '' ? null : Number(v)" type="number" step="0.01" min="0" placeholder="月租金" clearable />
        </el-form-item>
        <el-form-item label="押金规则" prop="deposit_months" required
          :rules="[{ required: true, message: '请选择押金规则' }, { validator: (_, v) => v >= 0 && v <= 3, message: '押金月数范围为0~3' }]">
          <el-select v-model="editForm.deposit_months" placeholder="选择押金规则" style="width:100%">
            <el-option :value="0" label="无押金" />
            <el-option :value="1" label="押一" />
            <el-option :value="2" label="押二" />
            <el-option :value="3" label="押三" />
          </el-select>
        </el-form-item>
        <el-form-item label="管理费" prop="management_fee" required
          :rules="[{ required: true, message: '请输入管理费' }, { validator: (_, v) => v >= 0, message: '管理费不能为负数' }]">
          <el-input :model-value="editForm.management_fee" @update:model-value="v => editForm.management_fee = v === '' ? null : Number(v)" type="number" step="0.01" min="0" placeholder="每月管理费" clearable />
        </el-form-item>
        <el-form-item label="电费单价" prop="electricity_unit_price" required
          :rules="[{ required: true, message: '请输入电费单价' }, { validator: (_, v) => v >= 0, message: '电费单价不能为负数' }]">
          <el-input :model-value="editForm.electricity_unit_price" @update:model-value="v => editForm.electricity_unit_price = v === '' ? null : Number(v)" type="number" step="0.01" min="0" placeholder="元/度" clearable />
        </el-form-item>
        <el-form-item label="水费单价" prop="water_unit_price" required
          :rules="[{ required: true, message: '请输入水费单价' }, { validator: (_, v) => v >= 0, message: '水费单价不能为负数' }]">
          <el-input :model-value="editForm.water_unit_price" @update:model-value="v => editForm.water_unit_price = v === '' ? null : Number(v)" type="number" step="0.01" min="0" placeholder="元/吨" clearable />
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input v-model="editForm.description" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showEditDialog = false">取消</el-button>
        <el-button type="primary" :loading="editSubmitting" @click="handleEdit">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { buildingUpdateRoomStatus, buildingRenewContract, buildingUpdateRoom } from '../../api'
import { FLOOR_OPTIONS, LAYOUT_OPTIONS } from '../../utils/constants'

const floorOptions = FLOOR_OPTIONS
const layoutOptions = LAYOUT_OPTIONS

const props = defineProps({
  roomId: { type: [String, Number], required: true },
  currentContract: { type: Object, default: null },
  editInitData: { type: Object, default: () => ({}) },
})

const emit = defineEmits(['save-success'])

const showRentDialog = ref(false)
const rentSubmitting = ref(false)
const rentForm = ref({ tenant_name: '', tenant_phone: '', rent_price: 0, management_fee: 0, deposit: 0, start_date: '', end_date: '' })
const rentFormRef = ref(null)

const showReserveDialog = ref(false)
const reserveSubmitting = ref(false)
const reserveForm = ref({ tenant_name: '', tenant_phone: '', earnest_money: 0, rent_price: 0, management_fee: 0, deposit: 0, start_date: '', end_date: '' })
const reserveFormRef = ref(null)

const showConfirmSignDialog = ref(false)
const signSubmitting = ref(false)
const signForm = ref({ tenant_name: '', tenant_phone: '', rent_price: 0, management_fee: 0, deposit: 0, start_date: '', end_date: '' })
const signFormRef = ref(null)

const showCancelReserveDialog = ref(false)
const cancelSubmitting = ref(false)
const cancelForm = ref({ refunded_deposit: 0 })
const cancelFormRef = ref(null)

const earnestMoney = computed(() => props.currentContract?.earnest_money || 0)

const showEndDateDialog = ref(false)
const endDateSubmitting = ref(false)
const endDateForm = ref({ end_date: '', rent_price: 0 })
const endDateFormRef = ref(null)

const showVacateDialog = ref(false)
const vacateSubmitting = ref(false)
const vacateForm = ref({ refunded_deposit: 0 })
const vacateFormRef = ref(null)

const vacateDeduction = computed(() => {
  const deposit = props.currentContract?.deposit || 0
  const refunded = vacateForm.value.refunded_deposit
  return Math.max(0, deposit - refunded)
})

const showEditDialog = ref(false)
const editSubmitting = ref(false)
const editForm = ref({})
const editFormRef = ref(null)

function openRent() {
  showRentDialog.value = true
}

function openRenew() {
  endDateForm.value = { end_date: '', rent_price: props.currentContract?.rent_price || 0 }
  showEndDateDialog.value = true
}

function openVacant() {
  const deposit = props.currentContract?.deposit || 0
  vacateForm.value.refunded_deposit = deposit
  showVacateDialog.value = true
}

function openEdit() {
  editForm.value = { ...props.editInitData }
  showEditDialog.value = true
}

function openReserve() {
  reserveForm.value = { tenant_name: '', tenant_phone: '', earnest_money: 0, rent_price: 0, management_fee: 0, deposit: 0, start_date: '', end_date: '' }
  showReserveDialog.value = true
}

function openConfirmSign() {
  const ct = props.currentContract || {}
  signForm.value = {
    tenant_name: ct.tenant?.name || '',
    tenant_phone: ct.tenant?.phone || '',
    rent_price: ct.rent_price || 0,
    management_fee: ct.management_fee || 0,
    deposit: ct.deposit || 0,
    start_date: ct.start_date || '',
    end_date: ct.end_date || '',
  }
  showConfirmSignDialog.value = true
}

function openCancelReserve() {
  cancelForm.value = { refunded_deposit: 0 }
  showCancelReserveDialog.value = true
}

async function handleReserve() {
  const valid = await reserveFormRef.value.validate().catch(() => false)
  if (!valid) return
  if (reserveForm.value.start_date && reserveForm.value.end_date && reserveForm.value.end_date <= reserveForm.value.start_date) {
    ElMessage.error('结束日期必须大于起租日期')
    return
  }
  reserveSubmitting.value = true
  try {
    await buildingUpdateRoomStatus(props.roomId, { status: 'reserved', ...reserveForm.value })
    ElMessage.success('预订成功')
    showReserveDialog.value = false
    emit('save-success')
  } finally {
    reserveSubmitting.value = false
  }
}

async function handleConfirmSign() {
  const valid = await signFormRef.value.validate().catch(() => false)
  if (!valid) return
  if (signForm.value.start_date && signForm.value.end_date && signForm.value.end_date <= signForm.value.start_date) {
    ElMessage.error('结束日期必须大于起租日期')
    return
  }
  signSubmitting.value = true
  try {
    await buildingUpdateRoomStatus(props.roomId, { status: 'rented', ...signForm.value })
    ElMessage.success('已确认签约')
    showConfirmSignDialog.value = false
    emit('save-success')
  } finally {
    signSubmitting.value = false
  }
}

async function handleCancelReserve() {
  cancelSubmitting.value = true
  try {
    await buildingUpdateRoomStatus(props.roomId, { status: 'vacant', refunded_deposit: cancelForm.value.refunded_deposit })
    ElMessage.success('已取消预订')
    showCancelReserveDialog.value = false
    emit('save-success')
  } finally {
    cancelSubmitting.value = false
  }
}

async function handleRent() {
  const valid = await rentFormRef.value.validate().catch(() => false)
  if (!valid) return
  if (rentForm.value.start_date && rentForm.value.end_date && rentForm.value.end_date <= rentForm.value.start_date) {
    ElMessage.error('退租日期必须大于起租日期')
    return
  }
  rentSubmitting.value = true
  try {
    await buildingUpdateRoomStatus(props.roomId, { status: 'rented', ...rentForm.value })
    ElMessage.success('出租成功')
    showRentDialog.value = false
    rentForm.value = { tenant_name: '', tenant_phone: '', rent_price: 0, management_fee: 0, deposit: 0, start_date: '', end_date: '' }
    emit('save-success')
  } finally {
    rentSubmitting.value = false
  }
}

async function handleUpdateEndDate() {
  const valid = await endDateFormRef.value.validate().catch(() => false)
  if (!valid) return
  endDateSubmitting.value = true
  try {
    await buildingRenewContract(props.roomId, { end_date: endDateForm.value.end_date, rent_price: endDateForm.value.rent_price })
    ElMessage.success('续租成功')
    showEndDateDialog.value = false
    endDateForm.value = { end_date: '', rent_price: 0 }
    emit('save-success')
  } finally {
    endDateSubmitting.value = false
  }
}

async function confirmVacate() {
  vacateSubmitting.value = true
  try {
    await buildingUpdateRoomStatus(props.roomId, { status: 'vacant', refunded_deposit: vacateForm.value.refunded_deposit })
    ElMessage.success('已设为未出租')
    showVacateDialog.value = false
    emit('save-success')
  } finally {
    vacateSubmitting.value = false
  }
}

async function handleEdit() {
  const valid = await editFormRef.value.validate().catch(() => false)
  if (!valid) return
  editSubmitting.value = true
  try {
    await buildingUpdateRoom(props.roomId, editForm.value)
    ElMessage.success('编辑成功')
    showEditDialog.value = false
    emit('save-success')
  } finally {
    editSubmitting.value = false
  }
}

defineExpose({ openRent, openRenew, openVacant, openEdit, openReserve, openConfirmSign, openCancelReserve })
</script>

<style scoped>
.vacate-deposit-info {
  background: #f5f7fa;
  padding: 12px;
  border-radius: 8px;
  margin-bottom: 16px;
}
.vacate-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.vacate-amount {
  font-weight: 600;
  color: #e6a23c;
  font-size: 16px;
}
.vacate-hint {
  font-size: 13px;
  color: #999;
  margin: 8px 0 0;
}
.vacate-deduction-note {
  background: #fef0f0;
  padding: 10px 12px;
  border-radius: 6px;
  font-size: 13px;
  color: #f56c6c;
}
.reserve-hint {
  background: #ecf5ff;
  padding: 10px 12px;
  border-radius: 6px;
  font-size: 13px;
  color: #409eff;
  margin-top: 8px;
}
</style>
