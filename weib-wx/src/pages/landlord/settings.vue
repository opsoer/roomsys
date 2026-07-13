<template>
  <view class="page-settings">
    <text class="page-title">公寓设置</text>

    <view v-if="loading" class="loading-wrap"><text>加载中...</text></view>
    <view v-else class="settings-form">
      <view class="cover-section">
        <text class="form-label">封面图</text>
        <view class="cover-wrap" @click="pickCover">
          <image v-if="form.cover_image" :src="coverPreview" mode="aspectFill" class="cover-img" />
          <view v-else class="cover-placeholder">
            <text class="cover-icon">📷</text>
            <text class="cover-tip">点击上传封面</text>
          </view>
        </view>
      </view>
      <view class="form-group"><text class="form-label">公寓名称</text><input class="form-input" v-model="form.name" /></view>

      <view class="form-group"><text class="form-label">区域</text>
        <picker mode="selector" :range="districtLabels" @change="e => { form.district = districts[e.detail.value]?.value || ''; form.street = ''; form.village = '' }">
          <view class="picker-val">{{ form.district || '选择区域' }}</view>
        </picker>
      </view>
      <view class="form-group"><text class="form-label">街道</text>
        <picker mode="selector" :range="streetLabels" @change="e => { form.street = currentStreets[e.detail.value]?.value || ''; form.village = '' }">
          <view class="picker-val">{{ form.street || '选择街道' }}</view>
        </picker>
      </view>
      <view class="form-group"><text class="form-label">楼牌号</text><input class="form-input" v-model="form.building_no" /></view>
      <view class="form-group"><text class="form-label">简介</text><textarea class="form-textarea" v-model="form.description" rows="3" /></view>

      <view class="divider" />
      <text class="section-label">房东信息</text>
      <view class="form-group"><text class="form-label">房东姓名</text><input class="form-input" v-model="form.landlord_name" /></view>
      <view v-for="(p, i) in form.landlord_phones" :key="i" class="phone-row">
        <input class="form-input phone-input" v-model="form.landlord_phones[i]" placeholder="房东电话" />
        <button v-if="form.landlord_phones.length > 1" class="remove-btn" @click="form.landlord_phones.splice(i, 1)">×</button>
      </view>
      <button class="add-phone-btn" @click="form.landlord_phones.push('')">+ 添加电话</button>

      <button class="save-btn" :disabled="saving" @click="handleSave">{{ saving ? '保存中...' : '保存设置' }}</button>
    </view>
  </view>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { getBuildingInfo, updateBuildingInfo, buildingUploadCover } from '../../api'
import { mediaUrl } from '../../utils/format'
import shenzhen from '../../utils/shenzhen'

const districts = shenzhen
const districtLabels = computed(() => districts.map(d => d.label))
const currentStreets = computed(() => {
  const d = districts.find(x => x.value === form.value.district)
  return d ? d.streets : []
})
const streetLabels = computed(() => currentStreets.value.map(s => s.label))

const loading = ref(true)
const saving = ref(false)
const coverPreview = ref('')
const form = ref({
  name: '', district: '', street: '', village: '', building_no: '',
  description: '', landlord_name: '', landlord_phones: [''],
  cover_image: ''
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
      cover_image: b.cover_image || '',
    }
    if (form.value.cover_image) coverPreview.value = mediaUrl(form.value.cover_image)
  } catch {} finally { loading.value = false }
}

async function pickCover() {
  try {
    const res = await uni.chooseImage({ count: 1, sizeType: ['compressed'] })
    const tempPath = res.tempFilePaths[0]
    const compressRes = await uni.compressImage({ src: tempPath, quality: 65 })
    const compressedPath = compressRes.tempFilePath
    coverPreview.value = compressedPath
    uni.showLoading({ title: '上传中...' })
    const uploadRes = await buildingUploadCover(compressedPath)
    uni.hideLoading()
    form.value.cover_image = uploadRes.data.file_path || uploadRes.data
    uni.showToast({ title: '封面上传成功', icon: 'success' })
  } catch {
    uni.hideLoading()
    uni.showToast({ title: '上传失败', icon: 'none' })
  }
}

async function handleSave() {
  saving.value = true
  try {
    const data = { ...form.value }
    data.landlords = data.landlord_phones.filter(p => p.trim()).map(p => ({ name: (data.landlord_name || '').trim() || '未知', phone: p.trim() }))
    delete data.landlord_name
    delete data.landlord_phones
    await updateBuildingInfo(data)
    uni.showToast({ title: '保存成功', icon: 'success' })
  } catch { uni.showToast({ title: '保存失败', icon: 'none' }) }
  finally { saving.value = false }
}

onMounted(fetchInfo)
</script>

<style scoped>
.page-settings { padding: 16px; min-height: 100vh; }
.page-title { font-size: 20px; font-weight: 700; margin-bottom: 20px; color: #1a1a2e; display: block; }
.loading-wrap { text-align: center; padding: 60px 0; color: #999; }
.form-group { margin-bottom: 14px; }
.form-label { font-size: 14px; color: #333; display: block; margin-bottom: 4px; font-weight: 500; }
.form-input { width: 100%; height: 40px; border: 1px solid #dcdfe6; border-radius: 8px; padding: 0 12px; font-size: 14px; background: #fff; }
.form-textarea { width: 100%; border: 1px solid #dcdfe6; border-radius: 8px; padding: 10px 12px; font-size: 14px; background: #fff; min-height: 80px; }
.picker-val { height: 40px; line-height: 40px; border: 1px solid #dcdfe6; border-radius: 8px; padding: 0 12px; font-size: 14px; background: #fff; }
.divider { height: 1px; background: #eee; margin: 20px 0; }
.section-label { font-size: 15px; font-weight: 600; color: #333; display: block; margin-bottom: 14px; }
.phone-row { display: flex; gap: 8px; margin-bottom: 8px; }
.phone-input { flex: 1; }
.remove-btn { width: 36px; height: 36px; border-radius: 50%; background: #f56c6c; color: #fff; border: none; font-size: 18px; display: flex; align-items: center; justify-content: center; margin-top: 2px; }
.add-phone-btn { font-size: 13px; color: #1989fa; background: none; border: 1px dashed #1989fa; border-radius: 6px; padding: 8px 16px; margin-bottom: 20px; }
.save-btn { width: 100%; height: 44px; background: #1989fa; color: #fff; border: none; border-radius: 22px; font-size: 16px; font-weight: 600; display: flex; align-items: center; justify-content: center; }
.save-btn[disabled] { opacity: 0.6; }
.cover-section { margin-bottom: 14px; }
.cover-wrap { width: 100%; height: 160px; border-radius: 10px; overflow: hidden; border: 1px dashed #dcdfe6; margin-top: 6px; }
.cover-img { width: 100%; height: 100%; }
.cover-placeholder { width: 100%; height: 100%; display: flex; flex-direction: column; align-items: center; justify-content: center; background: #fafafa; }
.cover-icon { font-size: 40px; }
.cover-tip { font-size: 13px; color: #999; margin-top: 6px; }
</style>
