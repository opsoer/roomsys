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
import { ref, computed } from 'vue'
import { Delete } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { adminCreateBuilding, adminUpdateBuilding, adminCreateBuildingAdmin, adminUpgradePackage } from '../../api'
import shenzhen from '../../utils/shenzhen'

const emit = defineEmits(['save-success'])

const districts = shenzhen
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

const createForm = ref({
  name: '', package: 'basic', contract_date: '', district: '', street: '', village: '', building_no: '', description: '',
  landlord_name: '', landlord_phones: [''],
  admin_username: '', admin_password: '',
})
const editForm = ref({ name: '', package: 'basic', contract_date: '', district: '', street: '', village: '', building_no: '', description: '', landlord_name: '', landlord_phones: [''], status: 'active' })
const adminForm = ref({ username: '', password: '' })

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
      const loginUrl = `${window.location.origin}/landlord/login/${newBuilding.id}`
      ElMessage.success(`公寓创建成功！管理员登录链接：${loginUrl}`)
    } else {
      ElMessage.success('公寓创建成功')
    }
    emit('save-success')
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

defineExpose({ openCreate, openEdit, openUpgrade, openCreateAdmin })
</script>
