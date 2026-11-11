<template>
  <el-dialog
    v-model="visible"
    :title="mode === 'edit' ? '修改租控表内容' : '租控表预览'"
    :width="isMobile ? '100%' : '1120px'"
    :fullscreen="isMobile"
    top="3vh"
    :close-on-click-modal="false"
    :close-on-press-escape="!exporting"
    class="rent-control-dialog"
    @closed="handleClosed"
  >
    <div v-loading="loading" class="rent-control-body">
      <div class="layout" :class="{ 'layout-edit': mode === 'edit', 'layout-stack': isMobile }">
        <!-- 编辑表单：修改后右侧预览实时更新 -->
        <div v-if="mode === 'edit'" class="edit-form">
          <div class="edit-tip">
            修改后右侧预览实时更新；内容保存在本机浏览器，下次打开自动带出。
          </div>
          <el-form label-position="top">
            <el-form-item v-for="field in FIELDS" :key="field.key" :label="field.label">
              <el-input
                v-model="content[field.key]"
                type="textarea"
                :rows="field.rows"
                :maxlength="500"
                show-word-limit
                :placeholder="field.placeholder || ''"
              />
            </el-form-item>
          </el-form>
          <el-button size="small" @click="resetContent">恢复默认</el-button>
        </div>

        <!-- 预览：导出图片的内容 -->
        <div class="preview-column">
          <div ref="previewRef" class="poster">
            <div class="poster-title">{{ posterTitle }}</div>

            <div v-if="total === 0" class="poster-empty">暂无可导出房间</div>

            <template v-else>
              <!-- 空房表：只列房号/格局/租金/管理费，不带备注列 -->
              <section v-if="vacant.length" class="poster-block">
                <div class="block-title">
                  <span class="block-name">空房</span>
                  <span class="block-count">{{ vacant.length }} 套</span>
                </div>
                <table class="poster-table table-vacant">
                  <colgroup>
                    <col style="width: 20%">
                    <col style="width: 30%">
                    <col style="width: 25%">
                    <col style="width: 25%">
                  </colgroup>
                  <thead>
                    <tr>
                      <th>房号</th>
                      <th>格局</th>
                      <th>租金</th>
                      <th>管理费</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="(row, idx) in vacantRows" :key="'v' + idx">
                      <td>{{ row.room_number }}</td>
                      <td>{{ row.layout }}</td>
                      <td>{{ formatMoney(row.rent_price) }}</td>
                      <td>{{ formatMoney(row.management_fee) }}</td>
                    </tr>
                  </tbody>
                </table>
              </section>

              <!-- 即将到期表：多一列备注，自动写到期时间 -->
              <section v-if="expiring.length" class="poster-block">
                <div class="block-title">
                  <span class="block-name">即将到期</span>
                  <span class="block-count">{{ expiring.length }} 套</span>
                </div>
                <table class="poster-table table-expiring">
                  <colgroup>
                    <col style="width: 14%">
                    <col style="width: 22%">
                    <col style="width: 16%">
                    <col style="width: 16%">
                    <col style="width: 32%">
                  </colgroup>
                  <thead>
                    <tr>
                      <th>房号</th>
                      <th>格局</th>
                      <th>租金</th>
                      <th>管理费</th>
                      <th>备注</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="(row, idx) in expiringRows" :key="'e' + idx">
                      <td>{{ row.room_number }}</td>
                      <td>{{ row.layout }}</td>
                      <td>{{ formatMoney(row.rent_price) }}</td>
                      <td>{{ formatMoney(row.management_fee) }}</td>
                      <td class="cell-remark">{{ row.remark }}</td>
                    </tr>
                  </tbody>
                </table>
              </section>
            </template>

            <div class="poster-footer">
              <div v-for="sec in visibleSections" :key="sec.key" class="poster-section">
                <span class="section-label">【{{ sec.label }}】</span>
                <span class="section-body">{{ content[sec.key] }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="dialog-footer">
        <template v-if="mode === 'preview'">
          <el-button :disabled="total === 0" @click="mode = 'edit'">修改内容</el-button>
          <el-button type="primary" :loading="exporting" :disabled="total === 0" @click="handleDownload">
            下载图片
          </el-button>
        </template>
        <template v-else>
          <el-button @click="mode = 'preview'">返回预览</el-button>
          <el-button type="primary" :loading="exporting" :disabled="total === 0" @click="handleDownload">
            下载图片
          </el-button>
        </template>
      </div>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import dayjs from 'dayjs'
import { ElMessage } from 'element-plus'
import { toPng } from 'html-to-image'
import { buildingGetRentControl } from '../../api'
import { useMobile } from '../../composables/useMobile'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue'])

const { isMobile } = useMobile()
const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})

// 预览表格最少行数：空房表补到 8 行，到期表补到 4 行，方便打印后手写补充
const MIN_VACANT_ROWS = 8
const MIN_EXPIRING_ROWS = 4
const STORAGE_PREFIX = 'rent_control_content_'

// 底部说明字段：顺序即图片上的展示顺序
const FIELDS = [
  { key: 'propertyAddress', label: '物业地址', rows: 3, placeholder: '公寓地址 + 导航地址，自动带出' },
  { key: 'feeStandard', label: '收费标准', rows: 2, placeholder: '如水费、电费单价' },
  { key: 'commissionStandard', label: '佣金标准', rows: 3 },
  { key: 'bookingProcess', label: '定房流程', rows: 4 },
  { key: 'checkinRequirement', label: '入住要求', rows: 2 },
  { key: 'viewAndSign', label: '看房及签合同', rows: 2, placeholder: '联系人及电话' },
  { key: 'commissionContact', label: '结佣请加客服', rows: 2, placeholder: '联系人及电话' },
  { key: 'priceSupport', label: '价格支持/定房', rows: 2, placeholder: '联系人及电话' },
]

const loading = ref(false)
const exporting = ref(false)
const mode = ref('preview')
const building = ref({})
// 两张表分开存放：空房表、即将到期表
const vacant = ref([])
const expiring = ref([])
const content = ref({})
const previewRef = ref(null)
// 是否已完成本机缓存 hydration：避免首次写入默认值时把已保存的内容覆盖掉
let hydrated = false

// 默认文案：物业地址自动用公寓地址 + 导航地址拼接；
// 收费标准按房间实际水电单价推导（各房间不一致时留空，交给用户填写）；
// 其余给一套可编辑的通用文案。
function defaultContent(b, rows = []) {
  return {
    propertyAddress: [b.address, b.directions].filter(Boolean).join('\n'),
    feeStandard: deriveFeeStandard(rows),
    commissionStandard: '一年一佣，半年半佣，押一减半（即佣金=月租金/12*实际签约月数）；转合同后两押一租收齐3天内结佣金。',
    bookingProcess: '1、定金为1个月租金，若预定违约则定金不退；（注意：免租期不得超过2天，现房交定后，五天内转合同，五天不转合同房子重新对外出租且定金不退！）；2、若出现撞单，以先后交定顺序为准。',
    checkinRequirement: '两押一租，禁止养宠物。',
    viewAndSign: '',
    commissionContact: '',
    priceSupport: '',
  }
}

function deriveFeeStandard(rows) {
  const pairs = new Set()
  for (const r of rows) {
    if (r.water_unit_price != null || r.electricity_unit_price != null) {
      pairs.add(`${r.water_unit_price ?? ''}|${r.electricity_unit_price ?? ''}`)
    }
  }
  if (pairs.size !== 1) return ''
  const [water, electricity] = [...pairs][0].split('|')
  const parts = []
  if (water !== '') parts.push(`水费${formatMoney(water)}元/吨`)
  if (electricity !== '') parts.push(`电费${formatMoney(electricity)}元/度`)
  return parts.join('，')
}

async function open() {
  loading.value = true
  try {
    const res = await buildingGetRentControl()
    const data = res.data || {}
    building.value = data.building || {}
    vacant.value = data.vacant || []
    expiring.value = data.expiring || []

    const base = defaultContent(building.value, [...vacant.value, ...expiring.value])
    const saved = localStorage.getItem(STORAGE_PREFIX + (building.value.id || 'default'))
    if (saved) {
      try {
        Object.assign(base, JSON.parse(saved))
      } catch {
        // 缓存损坏时忽略，使用默认文案
      }
    }
    content.value = base
    hydrated = true
    mode.value = 'preview'
  } catch {
    // 错误提示由 axios 拦截器统一处理
  } finally {
    loading.value = false
  }
}

watch(() => props.modelValue, (val) => {
  if (val) open()
})

// 编辑内容实时保存到本机（按公寓区分），不入库
watch(content, (val) => {
  if (!hydrated || !building.value.id) return
  localStorage.setItem(STORAGE_PREFIX + building.value.id, JSON.stringify(val))
}, { deep: true })

function handleClosed() {
  hydrated = false
  exporting.value = false
  mode.value = 'preview'
}

function resetContent() {
  content.value = defaultContent(building.value, [...vacant.value, ...expiring.value])
}

const total = computed(() => vacant.value.length + expiring.value.length)

// 补空行：仅在实际有数据时补，避免出现一张全是空行的表
function padRows(rows, min) {
  if (rows.length === 0) return []
  const out = rows.map((r) => ({ ...r }))
  for (let i = out.length; i < min; i++) {
    out.push({ room_number: '', layout: '', rent_price: null, management_fee: null, remark: '' })
  }
  return out
}

const vacantRows = computed(() => padRows(vacant.value, MIN_VACANT_ROWS))
const expiringRows = computed(() => padRows(expiring.value, MIN_EXPIRING_ROWS))

// 内容为空的段落不出现在图片上，避免留下空框
const visibleSections = computed(() =>
  FIELDS.filter((f) => (content.value[f.key] || '').trim() !== '')
)

const posterTitle = computed(
  () => `${building.value.name || '公寓'}租控表——${dayjs().format('YYYY/MM/DD')}`
)

const fileName = computed(
  () => `${building.value.name || '公寓'}租控表_${dayjs().format('YYYYMMDD')}.png`
)

function formatMoney(v) {
  if (v === null || v === undefined || v === '') return ''
  const n = Number(v)
  if (Number.isNaN(n)) return ''
  return n % 1 === 0 ? String(n) : n.toFixed(2)
}

async function handleDownload() {
  if (!previewRef.value || exporting.value) return
  exporting.value = true
  try {
    const dataUrl = await toPng(previewRef.value, {
      pixelRatio: 2,
      cacheBust: true,
      backgroundColor: '#ffffff',
    })
    const link = document.createElement('a')
    link.download = fileName.value
    link.href = dataUrl
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
  } catch {
    ElMessage.error('生成图片失败，请重试')
  } finally {
    exporting.value = false
  }
}
</script>

<style scoped>
.rent-control-body {
  min-height: 220px;
}

.layout {
  display: flex;
  gap: 20px;
}

.layout-edit .edit-form {
  width: 340px;
  flex-shrink: 0;
}

/* 移动端竖排：编辑表单在上，预览在下 */
.layout-stack {
  flex-direction: column;
}

.layout-stack .edit-form {
  width: 100%;
}

.layout-stack .preview-column {
  max-height: none;
  overflow: auto;
}

.edit-tip {
  font-size: 12px;
  color: #909399;
  line-height: 1.6;
  margin-bottom: 12px;
}

.preview-column {
  flex: 1;
  min-width: 0;
  max-height: 70vh;
  overflow: auto;
}

/* 预览海报：导出图片的内容，样式自包含 */
.poster {
  width: 720px;
  background: #fff;
  border: 1px solid #f0d9cf;
  border-radius: 14px;
  padding: 26px 28px 18px;
  box-shadow: 0 6px 24px rgba(214, 84, 42, 0.08);
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC',
    'Hiragino Sans GB', 'Microsoft YaHei', sans-serif;
  color: #2b2b2b;
  box-sizing: border-box;
}

.poster-title {
  text-align: center;
  font-size: 25px;
  font-weight: 700;
  color: #d6542a;
  letter-spacing: 2px;
  margin-bottom: 16px;
}

.poster-table {
  width: 100%;
  border-collapse: collapse;
  table-layout: fixed;
  font-size: 14px;
}

.poster-table th,
.poster-table td {
  border: 1px solid #f0c9b8;
  padding: 8px 6px;
  text-align: center;
  word-break: break-all;
}

.poster-table th {
  background: #fff3ec;
  color: #c74820;
  font-weight: 600;
}

.poster-table tbody tr:nth-child(even) td {
  background: #fffaf6;
}

.poster-table .cell-remark {
  text-align: left;
}

/* 空房表 / 到期表：同一张图片里的两个独立表格 */
.poster-block + .poster-block {
  margin-top: 20px;
}

.block-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.block-name {
  font-size: 16px;
  font-weight: 700;
  color: #c74820;
  padding-left: 9px;
  border-left: 4px solid #d6542a;
  line-height: 1.15;
}

.block-count {
  font-size: 12px;
  color: #c07a5e;
  background: #fff3ec;
  border-radius: 10px;
  padding: 2px 8px;
}

.poster-empty {
  padding: 30px 0;
  text-align: center;
  color: #909399;
  font-size: 14px;
}

.poster-footer {
  margin-top: 14px;
}

.poster-section {
  padding: 9px 0;
  border-top: 1px dashed #f0c9b8;
  font-size: 13px;
  line-height: 1.75;
}

.poster-section:first-child {
  border-top: none;
}

.section-label {
  color: #d6542a;
  font-weight: 600;
}

.section-body {
  color: #333;
  white-space: pre-wrap;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
</style>
