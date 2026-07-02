<template>
  <div class="settings-container">
    <h2 class="settings-title">公寓设置</h2>
    <el-card v-loading="loading">
      <el-form :model="form" label-width="100px">
        <el-form-item label="封面图片">
          <div class="cover-section">
            <img v-if="coverUrl" :src="coverUrl" class="cover-image" />
            <div v-else class="cover-placeholder">暂无封面</div>
            <div class="cover-actions">
              <el-upload
                :show-file-list="false"
                :before-upload="beforeUploadCover"
                :http-request="handleUploadCover"
                accept="image/jpeg,image/png,image/gif"
              >
                <el-button type="primary" size="small">上传封面</el-button>
              </el-upload>
              <el-button v-if="form.cover_image" type="danger" size="small" @click="handleRemoveCover">删除封面</el-button>
            </div>
          </div>
        </el-form-item>
        <el-form-item label="公寓名称">
          <el-input v-model="form.name" />
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="区域">
              <el-select v-model="form.district" placeholder="选择区域" filterable allow-create class="full-width" @change="form.street='';form.village=''">
                <el-option v-for="d in districts" :key="d.value" :label="d.label" :value="d.value" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="街道">
              <el-select v-model="form.street" placeholder="选择街道" filterable allow-create class="full-width" @change="form.village=''">
                <el-option v-for="s in currentStreets" :key="s.value" :label="s.label" :value="s.value" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="村/小区">
              <el-select v-model="form.village" placeholder="选择或输入" filterable allow-create class="full-width">
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
        <div v-for="(p, i) in form.landlord_phones" :key="i" class="phone-row">
          <el-input v-model="form.landlord_phones[i]" placeholder="房东电话" class="phone-input" />
          <el-button v-if="form.landlord_phones.length > 1" type="danger" :icon="Delete" circle @click="form.landlord_phones.splice(i,1)" />
        </div>
        <el-button size="small" class="add-phone-btn" @click="form.landlord_phones.push('')">+ 添加电话</el-button>
        <div class="save-section">
          <el-button type="primary" :loading="saving" @click="handleSave">保存设置</el-button>
        </div>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { getBuildingInfo, updateBuildingInfo, buildingUploadCover } from '../api'
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
  cover_image: '',
})

const coverUrl = computed(() => {
  return form.value.cover_image ? `/api/media/${form.value.cover_image}` : ''
})

function beforeUploadCover(file) {
  if (!file.type.startsWith('image/')) {
    ElMessage.error('仅支持图片格式')
    return false
  }
  if (file.size > 10 * 1024 * 1024) {
    ElMessage.error('图片最大 10MB')
    return false
  }
  return true
}

async function handleUploadCover(option) {
  const formData = new FormData()
  formData.append('file', option.file)
  try {
    const res = await buildingUploadCover(formData)
    form.value.cover_image = res.data?.cover_image || ''
    ElMessage.success('封面上传成功')
  } catch {
    ElMessage.error('封面上传失败')
  }
}

async function handleRemoveCover() {
  try {
    await updateBuildingInfo({ cover_image: '' })
    form.value.cover_image = ''
    ElMessage.success('封面已删除')
  } catch {
    ElMessage.error('删除封面失败')
  }
}

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
      cover_image: b.cover_image || '',
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
    data.landlords =     data.landlord_phones.filter(p => p.trim()).map(p => ({ name: (data.landlord_name || '').trim() || '未知', phone: p.trim() }))
    delete data.landlord_name
    delete data.landlord_phones
    await updateBuildingInfo(data)
    ElMessage.success('保存成功')
  } catch {
    ElMessage.error('保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(fetchInfo)
</script>

<style scoped>
.settings-container { max-width: 700px; margin: 0 auto; }
.settings-title { font-size: 20px; font-weight: 700; margin-bottom: 24px; }
.cover-section { display: flex; align-items: flex-start; gap: 12px; }
.cover-image { width: 200px; height: 120px; object-fit: cover; border-radius: 8px; border: 1px solid #dcdfe6; }
.cover-placeholder { width: 200px; height: 120px; border-radius: 8px; border: 1px dashed #dcdfe6; display: flex; align-items: center; justify-content: center; color: #999; font-size: 13px; }
.cover-actions { display: flex; flex-direction: column; gap: 8px; }
.full-width { width: 100%; }
.phone-row { display: flex; gap: 8px; margin-bottom: 8px; margin-left: 100px; }
.phone-input { flex: 1; }
.add-phone-btn { margin-left: 100px; }
.save-section { margin-top: 24px; }

@media (max-width: 768px) {
  .el-card { padding: 12px; }
}
</style>
