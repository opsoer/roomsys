<template>
  <el-drawer :model-value="!!bill" title="账单凭证" size="420px" @close="$emit('update:bill', null)">
    <div v-if="bill" class="bev-wrap">
      <div class="bev-head">
        <span class="bev-no">{{ bill.bill_no }}</span>
        <el-tag :type="bill.type === 'income' ? 'success' : 'danger'" size="small">
          {{ bill.type === 'income' ? '收入' : '支出' }} ¥{{ Number(bill.amount).toFixed(2) }}
        </el-tag>
      </div>

      <div v-if="bill.evidences?.length" class="bev-grid">
        <div v-for="ev in bill.evidences" :key="ev.id" class="bev-item">
          <el-image
            v-if="previewable(ev)"
            class="bev-thumb"
            :src="mediaUrl(ev.file_key)"
            :preview-src-list="previewList"
            :initial-index="previewIndex(ev)"
            fit="cover"
            :preview-teleported="true"
          />
          <div v-else class="bev-fallback">
            <a :href="mediaUrl(ev.file_key)" :download="ev.file_name">下载查看</a>
          </div>
          <el-icon class="bev-remove" title="删除凭证" @click="removeOne(ev)"><Close /></el-icon>
        </div>
      </div>
      <el-empty v-else description="暂无凭证" :image-size="64" />

      <el-divider>补充凭证</el-divider>
      <EvidenceUploader v-model="newEvidences" :max="maxNew" />
      <el-button
        type="primary" size="small" style="margin-top: 10px"
        :disabled="!newEvidences.length" :loading="saving"
        @click="saveNew"
      >
        添加 {{ newEvidences.length ? `(${newEvidences.length})` : '' }}
      </el-button>
    </div>
  </el-drawer>
</template>

<script setup>
// 账单凭证查看/补充/删除抽屉：覆盖所有生成路径（含自动账单）的事后补证。
import { ref, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Close } from '@element-plus/icons-vue'
import { buildingAddBillEvidences, buildingDeleteBillEvidence } from '../../api'
import EvidenceUploader from './EvidenceUploader.vue'

const props = defineProps({
  // 当前账单（来自列表行，含预加载的 evidences）；null 表示关闭
  bill: { type: Object, default: null },
})
const emit = defineEmits(['update:bill', 'changed'])

const newEvidences = ref([])
const saving = ref(false)
// 单笔上限 9，扣除已有数量
const maxNew = computed(() => Math.max(0, 9 - (props.bill?.evidences?.length || 0)))

watch(() => props.bill, () => { newEvidences.value = [] })

function previewable(ev) {
  return !/\.hei[cf]$/i.test(ev.file_key || '') && !/\.hei[cf]$/i.test(ev.file_name || '')
}
function mediaUrl(key) {
  return `/api/media/${key}`
}
const previewList = computed(() => (props.bill?.evidences || []).filter(previewable).map(ev => mediaUrl(ev.file_key)))
function previewIndex(ev) {
  const list = props.bill?.evidences || []
  let idx = 0
  for (const item of list) {
    if (item.id === ev.id) break
    if (previewable(item)) idx++
  }
  return idx
}

async function saveNew() {
  if (!newEvidences.value.length) return
  saving.value = true
  try {
    await buildingAddBillEvidences(props.bill.id, newEvidences.value)
    ElMessage.success('凭证已添加')
    emit('changed')
    emit('update:bill', null)
  } finally {
    saving.value = false
  }
}

async function removeOne(ev) {
  try {
    await buildingDeleteBillEvidence(props.bill.id, ev.id)
    ElMessage.success('已删除')
    emit('changed')
    emit('update:bill', null)
  } catch {
    // 失败提示由拦截器统一弹出
  }
}
</script>

<style scoped>
.bev-wrap {
  padding: 0 4px;
}
.bev-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 14px;
}
.bev-no {
  font-size: 13px;
  font-weight: 600;
  color: #666;
}
.bev-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}
.bev-item {
  position: relative;
  width: 96px;
}
.bev-thumb {
  width: 96px;
  height: 96px;
  border-radius: 8px;
  border: 1px solid #ebeef5;
  display: block;
}
.bev-fallback {
  width: 96px;
  height: 96px;
  border-radius: 8px;
  border: 1px dashed #dcdfe6;
  background: #f5f7fa;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
}
.bev-fallback a {
  color: #409eff;
  text-decoration: none;
}
.bev-remove {
  position: absolute;
  top: -7px;
  right: -7px;
  background: #f56c6c;
  color: #fff;
  border-radius: 50%;
  width: 18px;
  height: 18px;
  font-size: 11px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1;
}
</style>
