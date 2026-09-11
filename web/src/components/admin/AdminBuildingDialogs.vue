<template>
  <div>
    <!-- 创建公寓 -->
    <el-dialog v-model="showCreate" title="创建公寓" width="600px">
      <el-form ref="createFormRef" :model="createForm" label-width="100px">
        <el-form-item label="公寓名称" prop="name" :rules="[{required:true,message:'请输入'}]">
          <el-input v-model="createForm.name" />
        </el-form-item>
        <el-form-item label="签约日期" prop="contract_date" :rules="[{required:true,message:'请选择签约日期'}]">
          <el-date-picker v-model="createForm.contract_date" type="date" placeholder="选择日期" value-format="YYYY-MM-DD" style="width:100%" />
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="区域" prop="district" :rules="[{required:true,message:'请选择区域'}]">
              <el-select v-model="createForm.district" placeholder="选择区域" filterable allow-create style="width:100%" @change="createForm.street='';createForm.village=''">
                <el-option v-for="d in districts" :key="d.value" :label="d.label" :value="d.value" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="街道" prop="street" :rules="[{required:true,message:'请选择街道'}]">
              <el-select v-model="createForm.street" placeholder="选择街道" filterable allow-create style="width:100%" @change="createForm.village=''">
                <el-option v-for="s in currentStreets" :key="s.value" :label="s.label" :value="s.value" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="村/小区" prop="village" :rules="[{required:true,message:'请选择村/小区'}]">
              <el-select v-model="createForm.village" placeholder="选择或输入" filterable allow-create style="width:100%">
                <el-option v-for="v in currentVillages" :key="v" :label="v" :value="v" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="楼牌号" prop="building_no" :rules="[{required:true,message:'请输入楼牌号'}]">
              <el-input v-model="createForm.building_no" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="套餐" prop="package" :rules="[{required:true,message:'请选择套餐'}]">
          <el-radio-group v-model="createForm.package">
            <el-radio value="basic">基础套餐（仅房间管理）</el-radio>
            <el-radio value="full">全套餐（记账、预测、分红等全部功能）</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="简介" prop="description">
          <el-input v-model="createForm.description" type="textarea" :rows="3" />
        </el-form-item>
        <el-divider>房东信息（必填）</el-divider>
        <el-form-item label="房东姓名" prop="landlord_name" :rules="[{required:true,message:'请输入房东姓名'}]">
          <el-input v-model="createForm.landlord_name" placeholder="房东姓名" />
        </el-form-item>
        <div v-for="(p, i) in createForm.landlord_phones" :key="i" style="display: flex; gap: 8px; margin-bottom: 8px; margin-left:100px;">
          <el-input v-model="createForm.landlord_phones[i]" placeholder="房东电话" style="flex:1" @input="onPhoneInput" />
          <el-button v-if="createForm.landlord_phones.length > 1" type="danger" :icon="Delete" circle @click="createForm.landlord_phones.splice(i,1)" />
        </div>
        <el-button size="small" style="margin-left:100px" @click="createForm.landlord_phones.push('')">+ 添加电话</el-button>
        <el-divider />
        <el-form-item label="管理员账号" prop="admin_username" :rules="[{required:true,message:'请输入管理员账号'}]">
          <el-input v-model="createForm.admin_username" placeholder="默认使用房东电话" />
        </el-form-item>
        <el-form-item label="管理员密码" prop="admin_password" :rules="[{required:true,message:'请输入管理员密码'}]">
          <el-input v-model="createForm.admin_password" type="password" placeholder="公寓管理员的登录密码" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreate = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleCreate">确定创建</el-button>
      </template>
    </el-dialog>

    <!-- 编辑公寓 -->
    <el-dialog v-model="showEdit" title="编辑公寓" width="600px">
      <el-form ref="editFormRef" :model="editForm" label-width="100px">
        <el-form-item label="公寓名称" prop="name">
          <el-input v-model="editForm.name" />
        </el-form-item>
        <el-form-item label="签约日期" prop="contract_date">
          <el-date-picker v-model="editForm.contract_date" type="date" placeholder="选择日期" value-format="YYYY-MM-DD" style="width:100%" />
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="区域" prop="district">
              <el-select v-model="editForm.district" placeholder="选择区域" filterable allow-create style="width:100%" @change="editForm.street='';editForm.village=''">
                <el-option v-for="d in districts" :key="d.value" :label="d.label" :value="d.value" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="街道" prop="street">
              <el-select v-model="editForm.street" placeholder="选择街道" filterable allow-create style="width:100%" @change="editForm.village=''">
                <el-option v-for="s in editStreets" :key="s.value" :label="s.label" :value="s.value" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="村/小区" prop="village">
              <el-select v-model="editForm.village" placeholder="选择或输入" filterable allow-create style="width:100%">
                <el-option v-for="v in editVillages" :key="v" :label="v" :value="v" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="楼牌号" prop="building_no">
              <el-input v-model="editForm.building_no" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="套餐" prop="package">
          <el-radio-group v-model="editForm.package">
            <el-radio value="basic">基础套餐（仅房间管理）</el-radio>
            <el-radio value="full">全套餐（记账、预测、分红等全部功能）</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="简介" prop="description">
          <el-input v-model="editForm.description" type="textarea" :rows="3" />
        </el-form-item>
        <el-divider>房东信息</el-divider>
        <el-form-item label="房东姓名">
          <el-input v-model="editForm.landlord_name" placeholder="房东姓名" />
        </el-form-item>
        <div v-for="(p, i) in editForm.landlord_phones" :key="i" style="display: flex; gap: 8px; margin-bottom: 8px; margin-left:100px;">
          <el-input v-model="editForm.landlord_phones[i]" placeholder="房东电话" style="flex:1" />
          <el-button v-if="editForm.landlord_phones.length > 1" type="danger" :icon="Delete" circle @click="editForm.landlord_phones.splice(i,1)" />
        </div>
        <el-button size="small" style="margin-left:100px" @click="editForm.landlord_phones.push('')">+ 添加电话</el-button>
      </el-form>
      <template #footer>
        <el-button @click="showEdit = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleEdit">确定更新</el-button>
      </template>
    </el-dialog>

    <!-- 修改套餐弹窗 -->
    <el-dialog v-model="showUpgrade" title="修改套餐" width="420px">
      <p style="margin-bottom: 16px; color: #666;">为「{{ upgradeBuildingName }}」变更套餐</p>
      <el-form :model="upgradeForm" label-width="80px">
        <el-form-item label="目标套餐">
          <el-radio-group v-model="upgradeForm.package">
            <el-radio value="basic">基础套餐（仅房间管理）</el-radio>
            <el-radio value="full">全套餐（记账、预测、分红等全部功能）</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showUpgrade = false">取消</el-button>
        <el-button type="primary" :loading="upgradeSubmitting" @click="handleUpgrade">确定变更</el-button>
      </template>
    </el-dialog>

    <!-- 续约弹窗 -->
    <el-dialog v-model="showRenew" title="公寓续约" width="420px">
      <p style="margin-bottom: 16px; color: #666;">为「{{ renewBuildingName }}」补录续约信息。</p>
      <p style="margin-bottom: 12px; color: #999; font-size: 13px;">原到期日：{{ renewOldExpiredAt || '未设置' }}。原状态：{{ renewOldStatus }}</p>
      <el-form :model="renewForm" label-width="80px">
        <el-form-item label="新到期日" required>
          <el-date-picker v-model="renewForm.expired_at" type="date" placeholder="选择新的到期日期"
            :disabled-date="disablePastDate" value-format="YYYY-MM-DD" style="width:100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showRenew = false">取消</el-button>
        <el-button type="primary" :loading="renewSubmitting" :disabled="!renewForm.expired_at" @click="handleRenew">
          确认续约
        </el-button>
      </template>
    </el-dialog>

    <!-- 租约记录弹窗 -->
    <el-dialog v-model="showHistory" :title="`租约记录 · ${historyBuildingName}`" width="640px">
      <p style="margin-bottom: 14px; color: #666; font-size: 13px;">
        入驻、到期与续约完整记录。最新到期日期：<b>{{ latestExpiredAt || '—' }}</b>
      </p>
      <div class="desktop-table" v-loading="historyLoading">
        <el-table :data="records" empty-text="暂无记录" size="small" style="width:100%">
          <el-table-column label="类型" width="90">
            <template #default="{ row }">
              <el-tag :type="row.action === 'join' ? 'success' : 'warning'" size="small">
                {{ row.action === 'join' ? '入驻' : '续约' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="原日期" width="110">
            <template #default="{ row }">{{ row.from_date || '—' }}</template>
          </el-table-column>
          <el-table-column label="新日期" width="110">
            <template #default="{ row }">{{ row.to_date || '—' }}</template>
          </el-table-column>
          <el-table-column prop="note" label="备注" min-width="150" />
          <el-table-column label="操作人" width="100">
            <template #default="{ row }">{{ row.operator || '系统' }}</template>
          </el-table-column>
          <el-table-column label="时间" width="160">
            <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
          </el-table-column>
        </el-table>
      </div>
      <div class="mobile-cards" v-loading="historyLoading">
        <div v-for="r in records" :key="r.id" class="history-card">
          <div class="history-card-head">
            <el-tag :type="r.action === 'join' ? 'success' : 'warning'" size="small" effect="dark">
              {{ r.action === 'join' ? '入驻' : '续约' }}
            </el-tag>
            <span class="history-card-time">{{ formatTime(r.created_at) }}</span>
          </div>
          <div class="history-card-row">
            <span class="hc-label">原日期</span><span class="hc-value">{{ r.from_date || '—' }}</span>
          </div>
          <div class="history-card-row">
            <span class="hc-label">新日期</span><span class="hc-value">{{ r.to_date || '—' }}</span>
          </div>
          <div class="history-card-row">
            <span class="hc-label">备注</span><span class="hc-value">{{ r.note || '—' }}</span>
          </div>
          <div class="history-card-row">
            <span class="hc-label">操作人</span><span class="hc-value">{{ r.operator || '系统' }}</span>
          </div>
        </div>
        <div v-if="!historyLoading && records.length === 0" class="empty-text">暂无记录</div>
      </div>
    </el-dialog>

    <!-- 创建管理员弹窗 -->
    <el-dialog v-model="showCreateAdmin" title="创建公寓管理员" width="400px">
      <p style="margin-bottom: 16px; color: #666;">为「{{ selectedBuildingName }}」创建管理员账号</p>
      <el-form :model="adminForm" label-width="80px">
        <el-form-item label="账号">
          <el-input v-model="adminForm.username" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="adminForm.password" type="password" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateAdmin = false">取消</el-button>
        <el-button type="primary" :loading="adminSubmitting" @click="handleCreateAdmin">确定创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import dayjs from 'dayjs'
import { adminCreateBuilding, adminUpdateBuilding, adminCreateBuildingAdmin, adminUpgradePackage, adminRenewBuilding, adminGetBuildingRenewals } from '../../api'
import { useLocationStore } from '../../stores/locations'

const emit = defineEmits(['save-success'])

// 位置选项与主页筛选共用同一数据源（locations store：静态官方表 + 自定义/改名 + 公寓实际录入）
const locationStore = useLocationStore()
locationStore.load()
const districts = computed(() => locationStore.fullTree)
const submitting = ref(false)
const showCreate = ref(false)
const showEdit = ref(false)
const showCreateAdmin = ref(false)
const showUpgrade = ref(false)
const upgradeBuildingName = ref('')
const upgradeBuildingId = ref(0)
const upgradeForm = ref({ package: 'full' })
const upgradeSubmitting = ref(false)
const showRenew = ref(false)
const renewBuildingName = ref('')
const renewBuildingId = ref(0)
const renewOldExpiredAt = ref('')
const renewOldStatus = ref('')
const renewForm = ref({ expired_at: '' })
const renewSubmitting = ref(false)
const showHistory = ref(false)
const historyBuildingName = ref('')
const historyBuildingExpiredAt = ref('')
const records = ref([])
const historyLoading = ref(false)
const selectedBuildingName = ref('')
const selectedBuildingId = ref(0)
const adminSubmitting = ref(false)
const createFormRef = ref(null)

const createForm = ref({
  name: '', package: 'basic', contract_date: '', district: '', street: '', village: '', building_no: '', description: '',
  landlord_name: '', landlord_phones: [''],
  admin_username: '', admin_password: '',
})
const editForm = ref({ name: '', package: 'basic', contract_date: '', district: '', street: '', village: '', building_no: '', description: '', landlord_name: '', landlord_phones: [''], status: 'active' })
const adminForm = ref({ username: '', password: '' })

function findStreet(district) {
  return districts.value.find(x => x.value === district)?.streets || []
}

const currentStreets = computed(() => findStreet(createForm.value.district))

const currentVillages = computed(() => {
  const streets = findStreet(createForm.value.district)
  const s = streets.find(x => x.value === createForm.value.street)
  return s ? s.villages : []
})

const editStreets = computed(() => findStreet(editForm.value.district))

const editVillages = computed(() => {
  const streets = findStreet(editForm.value.district)
  const s = streets.find(x => x.value === editForm.value.street)
  return s ? s.villages : []
})

function onPhoneInput() {
  const phone = createForm.value.landlord_phones.find(p => p.trim())
  if (phone) {
    createForm.value.admin_username = phone
  }
}

function buildLandlords(name, phones) {
  return phones.filter(p => p.trim()).map(p => ({ name, phone: p.trim() }))
}

function openCreate() {
  createForm.value = { name: '', package: 'basic', contract_date: '', district: '', street: '', village: '', building_no: '', description: '', landlord_name: '', landlord_phones: [''], admin_username: '', admin_password: '' }
  showCreate.value = true
}

function openEdit(row) {
  const landlords = row.landlords || []
  const name = landlords.length > 0 ? landlords[0].name : ''
  const phones = landlords.map(l => l.phone)
  editForm.value = {
    id: row.id, name: row.name, package: row.package || 'basic', contract_date: row.contract_date || '',
    district: row.district, street: row.street,
    village: row.village, building_no: row.building_no, description: row.description,
    status: row.status,
    landlord_name: name,
    landlord_phones: phones.length > 0 ? phones : [''],
  }
  showEdit.value = true
}

function openUpgrade(row) {
  upgradeBuildingId.value = row.id
  upgradeBuildingName.value = row.name
  upgradeForm.value = { package: row.package === 'full' ? 'basic' : 'full' }
  showUpgrade.value = true
}

function disablePastDate(date) {
  return dayjs(date).isBefore(dayjs().startOf('day'))
}

function openRenew(row) {
  renewBuildingId.value = row.id
  renewBuildingName.value = row.name
  renewOldExpiredAt.value = row.expired_at || ''
  renewOldStatus.value = row.status === 'hidden' ? '不可见' : row.status === 'expired' ? '已到期' : '正常'
  renewForm.value = {
    expired_at: dayjs().add(1, 'year').format('YYYY-MM-DD'),
  }
  showRenew.value = true
}

function openCreateAdmin(row) {
  selectedBuildingId.value = row.id
  selectedBuildingName.value = row.name
  adminForm.value = { username: '', password: '' }
  showCreateAdmin.value = true
}

async function handleCreate() {
  const valid = await createFormRef.value.validate().catch(() => false)
  if (!valid) return
  submitting.value = true
  try {
    const data = { ...createForm.value }
    data.landlords = buildLandlords(data.landlord_name, data.landlord_phones)
    delete data.landlord_name
    delete data.landlord_phones
    delete data.admin_username
    delete data.admin_password
    const createRes = await adminCreateBuilding(data)
    const createdId = createRes.data?.building?.id
    if (createForm.value.admin_username && createdId) {
      await adminCreateBuildingAdmin({
        username: createForm.value.admin_username,
        password: createForm.value.admin_password,
        building_id: createdId,
      })
    }
    showCreate.value = false
    const newBuilding = createdId ? { id: createdId } : null
    if (newBuilding) {
      const loginUrl = `${window.location.origin}/login`
      ElMessage.success(`公寓创建成功！管理员登录链接：${loginUrl}`)
    } else {
      ElMessage.success('公寓创建成功')
    }
    emit('save-success')
  } catch (err) {
    const respData = err.response?.data
    if (respData?.code === 1006 && respData?.data?.suggested_name) {
      const suggested = respData.data.suggested_name
      try {
        await ElMessageBox.confirm(
          `公寓名称"${createForm.value.name}"已存在，建议修改为"${suggested}"，是否使用该名称？`,
          '名称重复',
          { confirmButtonText: '使用建议名称', cancelButtonText: '手动修改', type: 'warning' }
        )
        createForm.value.name = suggested
        submitting.value = false
        await handleCreate()
        return
      } catch {
        // User cancelled - let them edit manually
      }
    }
  } finally {
    submitting.value = false
  }
}

async function handleEdit() {
  submitting.value = true
  try {
    const data = { ...editForm.value }
    data.landlords = buildLandlords(data.landlord_name, data.landlord_phones)
    delete data.id
    delete data.landlord_name
    delete data.landlord_phones
    await adminUpdateBuilding(editForm.value.id, data)
    ElMessage.success('更新成功')
    showEdit.value = false
    emit('save-success')
  } finally {
    submitting.value = false
  }
}

async function handleUpgrade() {
  upgradeSubmitting.value = true
  try {
    await adminUpgradePackage(upgradeBuildingId.value, { package: upgradeForm.value.package })
    ElMessage.success('套餐变更成功')
    showUpgrade.value = false
    emit('save-success')
  } finally {
    upgradeSubmitting.value = false
  }
}

async function handleRenew() {
  if (!renewForm.value.expired_at) {
    ElMessage.warning('请选择新的到期日期')
    return
  }
  renewSubmitting.value = true
  try {
    await adminRenewBuilding(renewBuildingId.value, { expired_at: renewForm.value.expired_at })
    ElMessage.success('续约成功，公寓已恢复展示')
    showRenew.value = false
    emit('save-success')
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '续约失败')
  } finally {
    renewSubmitting.value = false
  }
}

async function openHistory(row) {
  historyBuildingName.value = row.name
  historyBuildingExpiredAt.value = row.expired_at || ''
  showHistory.value = true
  historyLoading.value = true
  records.value = []
  try {
    const r = await adminGetBuildingRenewals(row.id)
    records.value = r.data.records || []
  } catch {
    ElMessage.error('获取租约记录失败')
  } finally {
    historyLoading.value = false
  }
}

const latestExpiredAt = computed(() => {
  return records.value.length > 0 ? records.value[0].to_date || '' : historyBuildingExpiredAt.value
})

function formatTime(t) {
  if (!t) return '-'
  return dayjs(t).format('YYYY-MM-DD HH:mm:ss')
}

async function handleCreateAdmin() {
  if (!adminForm.value.username || !adminForm.value.password) {
    ElMessage.warning('请填写完整')
    return
  }
  adminSubmitting.value = true
  try {
    await adminCreateBuildingAdmin({
      username: adminForm.value.username,
      password: adminForm.value.password,
      building_id: selectedBuildingId.value,
    })
    const loginUrl = `${window.location.origin}/login`
    ElMessage.success(`管理员创建成功！登录链接：${loginUrl}`)
    showCreateAdmin.value = false
  } finally {
    adminSubmitting.value = false
  }
}

defineExpose({ openCreate, openEdit, openUpgrade, openRenew, openHistory, openCreateAdmin })
</script>

<style scoped>
.desktop-table { display: block; }
.mobile-cards { display: none; }
.history-card {
  background: #fafafa;
  border: 1px solid #eee;
  border-radius: 10px;
  padding: 12px 14px;
  margin-bottom: 10px;
}
.history-card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}
.history-card-time { font-size: 12px; color: #bbb; }
.history-card-row {
  display: flex;
  gap: 8px;
  font-size: 13px;
  margin-bottom: 4px;
}
.hc-label { color: #999; flex-shrink: 0; width: 52px; }
.hc-value { color: #333; word-break: break-all; }
.empty-text {
  text-align: center;
  padding: 24px 0;
  color: #999;
  font-size: 13px;
}
@media (max-width: 768px) {
  .desktop-table { display: none; }
  .mobile-cards { display: block; }
}
</style>
