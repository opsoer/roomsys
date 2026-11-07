<template>
  <div class="evidence-uploader">
    <div class="ev-list">
      <div v-for="(ev, i) in modelValue" :key="ev.file_key" class="ev-item">
        <el-image
          v-if="previewable(ev)"
          class="ev-thumb"
          :src="mediaUrl(ev.file_key)"
          :preview-src-list="previewList"
          :initial-index="previewIndex(i)"
          fit="cover"
          :preview-teleported="true"
        />
        <div v-else class="ev-fallback" title="该格式浏览器无法预览，点击下载查看">
          <span class="ev-ext">{{ extOf(ev) }}</span>
          <a class="ev-dl" :href="mediaUrl(ev.file_key)" :download="ev.file_name">下载查看</a>
        </div>
        <el-icon class="ev-remove" @click="removeAt(i)"><Close /></el-icon>
        <div class="ev-name" :title="ev.file_name">{{ ev.file_name }}</div>
      </div>

      <div v-if="modelValue.length < max" class="ev-add" :class="{ uploading }" @click="pick">
        <el-icon><Plus /></el-icon>
        <div class="ev-add-text">{{ uploading ? '上传中…' : '上传凭证' }}</div>
        <input ref="inputRef" type="file" multiple :accept="acceptStr" hidden @change="onPick" />
      </div>
    </div>
    <div class="ev-tip">仅图片，支持 jpg/png/gif/webp/bmp/tiff/avif/heic/heif；单张 ≤10MB，最多 {{ max }} 张</div>
  </div>
</template>

<script setup>
// 账单凭证截图上传（选填，非阻塞）：选图即传对象存储拿 key，key 由调用方随账单提交。
// 只认图片但尽量多格式；浏览器无法预览的格式（heic 等）降级为下载查看。
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { Close, Plus } from '@element-plus/icons-vue'
import { buildingUploadBillEvidence } from '../../api'

const props = defineProps({
  // 已选凭证列表：[{ file_key, file_name, file_size }]（双向绑定）
  modelValue: { type: Array, default: () => [] },
  max: { type: Number, default: 9 },
})

const emit = defineEmits(['update:modelValue'])

// accept 同时给 MIME 与扩展名：avif/heic/heif 在部分系统上没有注册 MIME
const acceptStr = 'image/*,.jpg,.jpeg,.png,.gif,.webp,.bmp,.tif,.tiff,.avif,.heic,.heif'
const ALLOWED_EXTS = ['jpg', 'jpeg', 'png', 'gif', 'webp', 'bmp', 'tif', 'tiff', 'avif', 'heic', 'heif']
const MAX_SIZE = 10 * 1024 * 1024

const inputRef = ref(null)
const uploading = ref(0)

function extOf(ev) {
  const m = /\.([a-z0-9]+)$/i.exec(ev.file_name || ev.file_key || '')
  return m ? m[1].toUpperCase() : 'IMG'
}

// 浏览器普遍可预览的格式；heic/heif 等交给下载兜底
function previewable(ev) {
  return !/\.hei[cf]$/i.test(ev.file_key || '') && !/\.hei[cf]$/i.test(ev.file_name || '')
}

function mediaUrl(key) {
  return `/api/media/${key}`
}

const previewList = computed(() => props.modelValue.filter(previewable).map(ev => mediaUrl(ev.file_key)))

function previewIndex(i) {
  let idx = 0
  for (let k = 0; k < i; k++) {
    if (previewable(props.modelValue[k])) idx++
  }
  return idx
}

function pick() {
  inputRef.value?.click()
}

function removeAt(i) {
  const next = props.modelValue.slice()
  next.splice(i, 1)
  emit('update:modelValue', next)
}

async function onPick(e) {
  const files = Array.from(e.target.files || [])
  e.target.value = ''
  if (!files.length) return
  const room = props.max - props.modelValue.length
  if (files.length > room) {
    ElMessage.warning(`最多还能再选 ${room} 张`)
  }
  for (const f of files.slice(0, room)) {
    const ext = (/\.([a-z0-9]+)$/i.exec(f.name) || [])[1]?.toLowerCase()
    if (!ext || !ALLOWED_EXTS.includes(ext)) {
      ElMessage.error(`${f.name}：不支持的图片格式`)
      continue
    }
    if (f.size > MAX_SIZE) {
      ElMessage.error(`${f.name}：超过 10MB`)
      continue
    }
    const fd = new FormData()
    fd.append('file', f)
    uploading.value++
    try {
      const res = await buildingUploadBillEvidence(fd)
      emit('update:modelValue', [...props.modelValue, {
        file_key: res.data.file_key,
        file_name: res.data.file_name || f.name,
        file_size: res.data.file_size ?? f.size,
      }])
    } catch {
      // 失败提示由 axios 拦截器统一弹出
    } finally {
      uploading.value--
    }
  }
}
</script>

<style scoped>
.ev-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.ev-item {
  position: relative;
  width: 72px;
}
.ev-thumb {
  width: 72px;
  height: 72px;
  border-radius: 6px;
  border: 1px solid #ebeef5;
  display: block;
}
.ev-fallback {
  width: 72px;
  height: 72px;
  border-radius: 6px;
  border: 1px dashed #dcdfe6;
  background: #f5f7fa;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
}
.ev-ext {
  font-size: 14px;
  font-weight: 700;
  color: #909399;
}
.ev-dl {
  font-size: 11px;
  color: #409eff;
  text-decoration: none;
}
.ev-remove {
  position: absolute;
  top: -6px;
  right: -6px;
  background: #f56c6c;
  color: #fff;
  border-radius: 50%;
  width: 16px;
  height: 16px;
  font-size: 10px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
}
.ev-name {
  font-size: 11px;
  color: #999;
  width: 72px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-top: 2px;
}
.ev-add {
  width: 72px;
  height: 72px;
  border: 1px dashed #c0ccda;
  border-radius: 6px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  color: #8c939d;
  cursor: pointer;
  font-size: 12px;
}
.ev-add:hover {
  border-color: #409eff;
  color: #409eff;
}
.ev-tip {
  font-size: 12px;
  color: #c0c4cc;
  margin-top: 6px;
  line-height: 1.5;
}
</style>
