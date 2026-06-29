<template>
  <div style="max-width: 1200px; margin: 0 auto;">
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; flex-wrap: wrap; gap: 12px;">
      <h2 style="font-size: 22px; font-weight: 700;">公寓管理</h2>
      <el-button type="primary" @click="showCreate = true">
        <el-icon><Plus /></el-icon> 创建公寓
      </el-button>
    </div>

    <el-card shadow="never" style="margin-bottom: 16px; background: #fafafa;">
      <div style="display: flex; gap: 12px; flex-wrap: wrap; align-items: center;">
        <el-select v-model="filterStatus" placeholder="公寓状态" clearable style="width:130px" @change="fetchBuildings">
          <el-option label="正常" value="normal" />
          <el-option label="即将到期" value="expiring" />
          <el-option label="已到期" value="expired" />
        </el-select>
        <el-input v-model="keyword" placeholder="搜索公寓名或房东电话" clearable style="width:260px" @keyup.enter="fetchBuildings" @clear="fetchBuildings">
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <el-button @click="fetchBuildings">搜索</el-button>
      </div>
    </el-card>

    <div v-loading="loading">
      <div v-if="buildings.length === 0 && !loading" style="text-align:center;padding:60px 0;color:#999;">
        暂无公寓数据
      </div>
      <el-card v-for="b in buildings" :key="b.id" shadow="hover" style="margin-bottom: 12px;">
        <div style="display: flex; flex-wrap: wrap; gap: 12px; align-items: flex-start;">
          <div style="flex: 1; min-width: 240px;">
            <div style="display: flex; align-items: center; gap: 10px; margin-bottom: 8px;">
              <span style="font-size: 18px; font-weight: 600;">{{ b.name }}</span>
              <el-tag v-if="b.building_status === 'normal'" size="small" type="success">正常</el-tag>
              <el-tag v-else-if="b.building_status === 'expiring'" size="small" type="warning">即将到期</el-tag>
              <el-tag v-else size="small" type="danger">已到期</el-tag>
              <el-tag :type="b.package === 'full' ? 'primary' : 'info'" size="small" effect="plain">{{ b.package === 'full' ? '全套餐' : '基础套餐' }}</el-tag>
            </div>
            <div style="font-size: 13px; color: #555; line-height: 1.8;">
              <div v-if="b.district">
                <span style="color:#999;">📍</span>
                {{ b.district }} {{ b.street }} {{ b.village }} {{ b.building_no }}
              </div>
              <div v-if="b.landlords?.length">
                <span style="color:#999;">👤</span>
                <template v-for="(l, i) in b.landlords" :key="l.id">
                  {{ l.name }} {{ l.phone }}<span v-if="i < b.landlords.length - 1"> / </span>
                </template>
              </div>
              <div>
                <span style="color:#999;">🚪</span>
                房间 {{ b.room_count }} 间
                <el-tag v-if="b.vacant_count > 0" size="small" type="success" style="margin-left:6px;">可租 {{ b.vacant_count }}</el-tag>
                <span v-else style="margin-left:6px;color:#999;">已满</span>
              </div>
              <div v-if="b.contract_date">
                <span style="color:#999;">📅</span>
                签约 {{ b.contract_date }} → {{ b.expired_at || '未设置' }}
              </div>
            </div>
          </div>
          <div style="display: flex; flex-wrap: wrap; gap: 6px; align-items: flex-start;">
            <el-button size="small" @click="editBuilding(b)">编辑</el-button>
            <el-button size="small" @click="showUpgradeDialog(b)">升级套餐</el-button>
            <el-button size="small" @click="copyLoginLink(b)">复制登录链接</el-button>
            <el-button size="small" @click="createAdminForBuilding(b)">创建管理员</el-button>
            <el-popconfirm title="确定删除?" @confirm="handleDelete(b.id)">
              <template #reference>
                <el-button size="small" type="danger">删除</el-button>
              </template>
            </el-popconfirm>
          </div>
        </div>
      </el-card>
    </div>

    <el-card shadow="never" style="margin-top: 20px">
      <h4>测试：系统时间模拟</h4>
      <div style="display: flex; gap: 12px; flex-wrap: wrap; align-items: center; margin-top: 12px">
        <span>当前模拟时间：<strong>{{ simulatedTime }}</strong></span>
        <el-button size="small" @click="refreshTime">刷新时间</el-button>
      </div>
      <div style="display: flex; gap: 12px; flex-wrap: wrap; align-items: center; margin-top: 12px">
        <span>偏移量：</span>
        <el-input-number v-model="offsetDays" :min="-365" :max="365" size="small" style="width: 100px" controls-position="right" />
        <span>天</span>
        <el-input-number v-model="offsetHours" :min="-23" :max="23" size="small" style="width: 80px" controls-position="right" />
        <span>小时</span>
        <el-button type="primary" size="small" :loading="timeLoading" @click="handleSetTime">应用偏移</el-button>
        <el-button size="small" @click="handleResetTime">重置</el-button>
      </div>
    </el-card>

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

    <!-- 升级套餐弹窗 -->
    <el-dialog v-model="showUpgrade" title="升级套餐" width="420px">
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
import { ref, computed, onMounted } from 'vue'
import { Plus, Delete, Search } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { adminGetBuildings, adminCreateBuilding, adminUpdateBuilding, adminDeleteBuilding, adminCreateBuildingAdmin, adminGetSystemTime, adminSetSystemTime, adminUpgradePackage } from '../api'
import shenzhen from '../utils/shenzhen'

const buildings = ref([])
const loading = ref(true)
const submitting = ref(false)
const showCreate = ref(false)
const showEdit = ref(false)
const showCreateAdmin = ref(false)
const showUpgrade = ref(false)
const upgradeBuildingName = ref('')
const upgradeBuildingId = ref(0)
const upgradeForm = ref({ package: 'full' })
const upgradeSubmitting = ref(false)
const selectedBuildingName = ref('')
const selectedBuildingId = ref(0)
const adminSubmitting = ref(false)
const createFormRef = ref(null)

const filterStatus = ref('')
const keyword = ref('')

const simulatedTime = ref('')
const timeLoading = ref(false)
const offsetDays = ref(0)
const offsetHours = ref(0)
const createForm = ref({
  name: '', package: 'basic', contract_date: '', district: '', street: '', village: '', building_no: '', description: '',
  landlord_name: '', landlord_phones: [''],
  admin_username: '', admin_password: '',
})
const editForm = ref({ name: '', package: 'basic', contract_date: '', district: '', street: '', village: '', building_no: '', description: '', landlord_name: '', landlord_phones: [''], status: 'active' })
const adminForm = ref({ username: '', password: '' })

const districts = shenzhen

function findStreet(district) {
  const d = districts.find(x => x.value === district)
  return d ? d.streets : []
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

async function fetchBuildings() {
  loading.value = true
  try {
    const params = {}
    if (filterStatus.value) params.status = filterStatus.value
    if (keyword.value) params.keyword = keyword.value
    const res = await adminGetBuildings(params)
    buildings.value = res.data.buildings || []
  } finally {
    loading.value = false
  }
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
    createForm.value = { name: '', package: 'basic', contract_date: '', district: '', street: '', village: '', building_no: '', description: '', landlord_name: '', landlord_phones: [''], admin_username: '', admin_password: '' }
    await fetchBuildings()
    const newId = createdId || ((await adminGetBuildings()).data.buildings || []).sort((a, b) => b.id - a.id)[0]?.id
    const newBuilding = newId ? { id: newId } : null
    if (newBuilding) {
      const loginUrl = `${window.location.origin}/landlord/login/${newBuilding.id}`
      ElMessage.success(`公寓创建成功！管理员登录链接：${loginUrl}`)
    } else {
      ElMessage.success('公寓创建成功')
    }
  } finally {
    submitting.value = false
  }
}

function editBuilding(row) {
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
    await fetchBuildings()
  } finally {
    submitting.value = false
  }
}

async function handleDelete(id) {
  await adminDeleteBuilding(id)
  ElMessage.success('已删除')
  await fetchBuildings()
}

function copyLoginLink(row) {
  const url = `${window.location.origin}/landlord/login/${row.id}`
  navigator.clipboard.writeText(url).then(() => {
    ElMessage.success('已复制管理员登录链接')
  }, () => {
    ElMessage.error('复制失败，请手动复制')
  })
}

function createAdminForBuilding(row) {
  selectedBuildingId.value = row.id
  selectedBuildingName.value = row.name
  adminForm.value = { username: '', password: '' }
  showCreateAdmin.value = true
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
    const loginUrl = `${window.location.origin}/landlord/login/${selectedBuildingId.value}`
    ElMessage.success(`管理员创建成功！登录链接：${loginUrl}`)
    showCreateAdmin.value = false
  } finally {
    adminSubmitting.value = false
  }
}

function showUpgradeDialog(row) {
  upgradeBuildingId.value = row.id
  upgradeBuildingName.value = row.name
  upgradeForm.value = { package: row.package === 'full' ? 'basic' : 'full' }
  showUpgrade.value = true
}

async function handleUpgrade() {
  upgradeSubmitting.value = true
  try {
    await adminUpgradePackage(upgradeBuildingId.value, { package: upgradeForm.value.package })
    ElMessage.success('套餐变更成功')
    showUpgrade.value = false
    await fetchBuildings()
  } finally {
    upgradeSubmitting.value = false
  }
}

async function refreshTime() {
  try {
    const res = await adminGetSystemTime()
    simulatedTime.value = res.data.simulated_time
  } catch {}
}

async function handleSetTime() {
  timeLoading.value = true
  try {
    const totalSeconds = offsetDays.value * 86400 + offsetHours.value * 3600
    await adminSetSystemTime(totalSeconds)
    ElMessage.success('时间偏移已设置')
    await refreshTime()
  } catch {
    ElMessage.error('设置失败')
  } finally {
    timeLoading.value = false
  }
}

async function handleResetTime() {
  offsetDays.value = 0
  offsetHours.value = 0
  timeLoading.value = true
  try {
    await adminSetSystemTime(0)
    ElMessage.success('已重置时间')
    await refreshTime()
  } finally {
    timeLoading.value = false
  }
}

onMounted(() => {
  fetchBuildings()
  refreshTime()
})
</script>

<style scoped>
@media (max-width: 768px) {
  .el-card { padding: 12px; }
  [style*="display: flex; justify-content: space-between;"] { flex-wrap: wrap; gap: 8px; }
}
</style>
