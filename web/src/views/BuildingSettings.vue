<template>
  <div style="max-width: 700px; margin: 0 auto;">
    <h2 style="font-size: 20px; font-weight: 700; margin-bottom: 24px;">公寓设置</h2>
    <el-card v-loading="loading">
      <el-form :model="form" label-width="100px">
        <el-form-item label="公寓名称">
          <el-input v-model="form.name" />
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="区域">
              <el-select v-model="form.district" placeholder="选择区域" filterable allow-create style="width:100%" @change="form.street='';form.village=''">
                <el-option v-for="d in districts" :key="d.value" :label="d.label" :value="d.value" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="街道">
              <el-select v-model="form.street" placeholder="选择街道" filterable allow-create style="width:100%" @change="form.village=''">
                <el-option v-for="s in currentStreets" :key="s.value" :label="s.label" :value="s.value" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="村/小区">
              <el-select v-model="form.village" placeholder="选择或输入" filterable allow-create style="width:100%">
                <el-option v-for="v in currentVillages" :key="v" :label="v" :value="v" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="楼牌号">
              <el-input v-model="form.building_no" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="简介">
          <el-input v-model="form.description" type="textarea" :rows="4" />
        </el-form-item>
        <el-divider>房东信息</el-divider>
        <el-form-item label="房东姓名">
          <el-input v-model="form.landlord_name" placeholder="房东姓名" />
        </el-form-item>
        <div v-for="(p, i) in form.landlord_phones" :key="i" style="display: flex; gap: 8px; margin-bottom: 8px; margin-left:100px;">
          <el-input v-model="form.landlord_phones[i]" placeholder="房东电话" style="flex:1" />
          <el-button v-if="form.landlord_phones.length > 1" type="danger" :icon="Delete" circle @click="form.landlord_phones.splice(i,1)" />
        </div>
        <el-button size="small" style="margin-left:100px" @click="form.landlord_phones.push('')">+ 添加电话</el-button>
        <div style="margin-top: 24px;">
          <el-button type="primary" :loading="saving" @click="handleSave">保存设置</el-button>
        </div>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Delete } from '@element-plus/icons-vue'
import { getBuildingInfo, updateBuildingInfo } from '../api'
import shenzhen from '../utils/shenzhen'

const districts = shenzhen

const currentStreets = computed(() => {
  const d = districts.find(x => x.value === form.value.district)
  return d ? d.streets : []
})

const currentVillages = computed(() => {
  const d = districts.find(x => x.value === form.value.district)
  if (!d) return []
  const s = d.streets.find(x => x.value === form.value.street)
  return s ? s.villages : []
})

const loading = ref(true)
const saving = ref(false)
const form = ref({
  name: '', district: '', street: '', village: '', building_no: '',
  description: '', landlord_name: '', landlord_phones: [''],
})

async function fetchInfo() {
  loading.value = true
  try {
    const res = await getBuildingInfo()
    const b = res.data.building
    const landlords = res.data.landlords || []
    const name = landlords.length > 0 ? landlords[0].name : ''
    const phones = landlords.map(l => l.phone)
    form.value = {
      name: b.name || '', district: b.district || '', street: b.street || '',
      village: b.village || '', building_no: b.building_no || '',
      description: b.description || '',
      landlord_name: name,
      landlord_phones: phones.length > 0 ? phones : [''],
    }
  } catch {
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  saving.value = true
  try {
    const data = { ...form.value }
    data.landlords = data.landlord_phones.filter(p => p.trim()).map(p => ({ name: data.landlord_name, phone: p.trim() }))
    delete data.landlord_name
    delete data.landlord_phones
    await updateBuildingInfo(data)
    ElMessage.success('保存成功')
  } finally {
    saving.value = false
  }
}

onMounted(fetchInfo)
</script>
