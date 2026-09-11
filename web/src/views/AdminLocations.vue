<template>
  <div class="loc-manage">
    <el-alert type="info" :closable="false" class="loc-tip">
      <template #title>
        位置选项说明：官方区划（内置，只读）+ 此处新增的自定义条目 + 改名映射，创建/编辑公寓与租客端筛选共用同一份选项。
        改名会<b>同步</b>更新该位置下所有公寓；被公寓使用的位置禁止删除；改名后旧的位置二维码链接会失效，需重新生成。
      </template>
    </el-alert>

    <div class="loc-cols">
      <!-- 区域 -->
      <div class="loc-col">
        <div class="loc-col-head">
          <span>区域</span>
          <el-button size="small" text type="primary" @click="openAdd(1)">+ 新增</el-button>
        </div>
        <div class="loc-col-list">
          <div
            v-for="d in tree" :key="d.label"
            :class="['loc-item', { active: selDistrict === d.label }]"
            @click="selDistrict = d.label; selStreet = ''"
          >
            <span class="loc-name">{{ d.label }}</span>
            <span class="loc-actions">
              <el-button size="small" text type="primary" @click.stop="openRename(1, { displayName: d.label })">改名</el-button>
              <el-button v-if="customId(1, d.label)" size="small" text type="danger" @click.stop="removeCustom(1, d.label)">删除</el-button>
            </span>
          </div>
        </div>
      </div>

      <!-- 街道 -->
      <div class="loc-col">
        <div class="loc-col-head">
          <span>街道{{ selDistrict ? ` · ${selDistrict}` : '' }}</span>
          <el-button size="small" text type="primary" :disabled="!selDistrict" @click="openAdd(2)">+ 新增</el-button>
        </div>
        <div class="loc-col-list">
          <div
            v-for="s in currentStreets" :key="s.label"
            :class="['loc-item', { active: selStreet === s.label }]"
            @click="selStreet = s.label"
          >
            <span class="loc-name">
              {{ s.label }}
              <el-tag v-if="isCustom(2, s.label)" size="small" type="warning" effect="plain">自定义</el-tag>
            </span>
            <el-button size="small" text type="primary" @click.stop="openRename(2, { displayName: s.label })">改名</el-button>
            <el-button v-if="customId(2, s.label)" size="small" text type="danger" @click.stop="removeCustom(2, s.label)">删除</el-button>
          </div>
          <div v-if="!selDistrict" class="loc-empty">请先选择区域</div>
        </div>
      </div>

      <!-- 村/小区 -->
      <div class="loc-col">
        <div class="loc-col-head">
          <span>村/小区{{ selStreet ? ` · ${selStreet}` : '' }}</span>
          <el-button size="small" text type="primary" :disabled="!selStreet" @click="openAdd(3)">+ 新增</el-button>
        </div>
        <div class="loc-col-list">
          <div v-for="v in currentVillages" :key="v" class="loc-item">
            <span class="loc-name">
              {{ v }}
              <el-tag v-if="isCustom(3, v)" size="small" type="warning" effect="plain">自定义</el-tag>
              <el-tag v-else-if="inUse(3, v) > 0" size="small" type="success" effect="plain">{{ inUse(3, v) }}栋</el-tag>
            </span>
            <span class="loc-actions">
              <el-button size="small" text type="primary" @click.stop="openRename(3, { displayName: v })">改名</el-button>
              <el-button v-if="customId(3, v)" size="small" text type="danger" @click.stop="removeCustom(3, v)">删除</el-button>
            </span>
          </div>
          <div v-if="!selStreet" class="loc-empty">请先选择街道</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useLocationStore } from '../stores/locations'
import { adminAddLocationCustom, adminDeleteLocationCustom, adminRenameLocation } from '../api'

const locationStore = useLocationStore()

const tree = computed(() => locationStore.fullTree)
const selDistrict = ref('')
const selStreet = ref('')

const currentStreets = computed(() =>
  selDistrict.value ? locationStore.streetsOf(selDistrict.value) : []
)
const currentVillages = computed(() =>
  (selDistrict.value && selStreet.value) ? locationStore.villagesOf(selDistrict.value, selStreet.value) : []
)

const normStreet = (s) => (s || '').replace(/街道$/, '')

// 自定义条目匹配/请求均使用界面当前显示名：后端改名时已把自定义条目与映射的上下文级联成新名，两者一致

// 自定义条目索引：level + 显示名 → 条目（改名与删除需要 id）
function customEntry(level, name) {
  return locationStore.customs.find(c => {
    if (c.level !== level || normStreet(c.name) !== normStreet(name)) return false
    if (level === 1) return true
    if (level === 2) return c.district === selDistrict.value
    return c.district === selDistrict.value && normStreet(c.street) === normStreet(selStreet.value)
  })
}
function isCustom(level, name) { return !!customEntry(level, name) }
function customId(level, name) { return customEntry(level, name)?.id || 0 }

// 该位置下的公寓数（来自 store 的 dbLocations 聚合，仅提示用，以后端 dry_run 为准）
function inUse(level, name) {
  const d = locationStore.dbLocations.find(x => x.name === selDistrict.value)
  if (!d) return 0
  if (level === 3) {
    const s = (d.streets || []).find(s => normStreet(s.name) === normStreet(selStreet.value))
    return s && (s.villages || []).includes(name) ? 1 : 0
  }
  return 0
}

onMounted(() => {
  locationStore.load(true)
  if (!selDistrict.value && tree.value.length) selDistrict.value = tree.value[0].label
})

async function reload() {
  await locationStore.load(true)
}

async function openAdd(level) {
  const tips = { 1: '新增区域名称', 2: `在「${selDistrict.value}」下新增街道`, 3: `在「${selDistrict.value} / ${selStreet.value}」下新增村/小区` }
  const { value } = await ElMessageBox.prompt(tips[level], '新增位置', { inputValue: '', inputPattern: /\S+/, inputErrorMessage: '名称不能为空' })
  const payload = { level, district: selDistrict.value, street: level === 3 ? selStreet.value : '', name: value.trim() }
  if (level === 1) { payload.district = payload.name }
  await adminAddLocationCustom(payload)
  ElMessage.success('已新增')
  await reload()
}

async function openRename(level, { displayName }) {
  const { value } = await ElMessageBox.prompt(`将「${displayName}」改名为：`, '位置改名', {
    inputValue: displayName, inputPattern: /\S+/, inputErrorMessage: '名称不能为空',
  })
  const newName = value.trim()
  if (newName === displayName) return
  const custom = level === 1 ? null : customEntry(level, displayName)
  const payload = {
    level,
    district: level === 1 ? displayName : selDistrict.value,
    street: level === 3 ? selStreet.value : '',
    old_name: displayName,
    new_name: newName,
    custom_id: custom?.id || 0,
  }
  const { data: dry } = await adminRenameLocation({ ...payload, dry_run: true })
  const affected = dry.affected_buildings || 0
  let tip = `确认将「${displayName}」改名为「${newName}」？`
  if (affected > 0) {
    tip += `\n\n⚠️ 该操作将同步修改 ${affected} 栋公寓的位置信息（立即生效，不可自动撤销）。`
  }
  tip += '\n⚠️ 已打印/分享的位置二维码中编码的是位置文字，改名后旧二维码将无法匹配，需重新生成。'
  await ElMessageBox.confirm(tip, '改名确认', { type: 'warning', confirmButtonText: '确认改名', cancelButtonText: '取消' })
  const { data: res } = await adminRenameLocation(payload)
  ElMessage.success(`改名完成，已同步更新 ${res.affected_buildings || 0} 栋公寓`)
  await reload()
}

async function removeCustom(level, name) {
  const id = customId(level, name)
  if (!id) return
  await ElMessageBox.confirm(`确认删除自定义条目「${name}」？仅当没有公寓使用该位置时允许删除。`, '删除确认', { type: 'warning' })
  await adminDeleteLocationCustom(id)
  ElMessage.success('已删除')
  await reload()
}
</script>

<style scoped>
.loc-manage { max-width: 1100px; }
.loc-tip { margin-bottom: 16px; }
.loc-cols { display: flex; gap: 16px; align-items: flex-start; }
.loc-col {
  flex: 1;
  background: #fff;
  border-radius: 8px;
  border: 1px solid #e4e7ed;
  overflow: hidden;
}
.loc-col-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  font-size: 14px;
  font-weight: 600;
  color: #303133;
  border-bottom: 1px solid #ebeef5;
  background: #fafbfc;
}
.loc-col-list { max-height: 60vh; overflow-y: auto; }
.loc-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  font-size: 13px;
  color: #303133;
  cursor: pointer;
  border-bottom: 1px solid #f5f7fa;
}
.loc-item:hover { background: #f5f7fa; }
.loc-item.active { background: #ecf5ff; color: #409eff; font-weight: 600; }
.loc-name { display: flex; align-items: center; gap: 6px; word-break: break-all; }
.loc-actions { display: flex; flex-shrink: 0; }
.loc-empty { padding: 24px 0; text-align: center; color: #999; font-size: 13px; }
</style>
