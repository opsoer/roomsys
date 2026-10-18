<template>
  <el-form ref="formRef" :model="form" label-width="100px">
    <template v-for="f in fields" :key="f.prop || f.divider">
      <el-divider v-if="f.divider">{{ f.divider }}</el-divider>
      <el-form-item v-else :label="f.label" :prop="f.prop" :rules="rules[f.prop]">
        <el-input v-if="f.type === 'text'" v-model="form[f.prop]" />
        <el-input v-else-if="f.type === 'number'" :model-value="form[f.prop]" type="number" step="0.01" min="0" :placeholder="f.label" style="width:100%" @update:model-value="v => form[f.prop] = toNumber(v)" />
        <el-date-picker v-else v-model="form[f.prop]" type="date" format="YYYY-MM-DD" value-format="YYYY-MM-DD" style="width:100%" />
      </el-form-item>
    </template>
    <div v-if="mode === 'rent' && historicalStart" class="rent-historical-hint">
      起租日期早于本月：系统只补记本月租金账单，之前月份的账单不再补录。
      <el-checkbox v-if="form.deposit > 0" v-model="form.record_deposit_bill">
        补记押金收入账单（勾选后押金计入本月收入；历史押金通常无需勾选）
      </el-checkbox>
    </div>
  </el-form>
</template>

<script setup>
import { ref, computed } from 'vue'

const props = defineProps({
  // 流转类型：rent 签合同出租 / reserve 交定金预订 / sign 确认签约
  mode: { type: String, default: 'sign' },
  // 起租日期早于本月（仅 rent 模式展示历史起租提示）
  historicalStart: { type: Boolean, default: false },
})
// 表单字段对象由父级 v-model 绑定（三种流转提交到不同接口，数据仍由父级持有）
const form = defineModel({ type: Object, required: true })

const formRef = ref(null)

// 三种流转共用的字段校验规则（文案与原三个弹窗保持一致）
const required = (message) => [{ required: true, message }]
const rules = {
  tenant_name: required('请输入租客姓名'),
  rent_price: required('请输入租金'),
  management_fee: required('请输入管理费'),
  deposit: required('请输入押金金额'),
  start_date: required('请选择起租日期'),
  end_date: required('请选择结束日期'),
  earnest_money: [...required('请输入定金金额'), { validator: (_, v) => v > 0, message: '定金必须大于0' }],
}

// 字段配置：reserve 模式多「定金金额」+ 分隔线，起租/结束为「预计」文案
const fields = computed(() => {
  const isReserve = props.mode === 'reserve'
  const list = [
    { prop: 'tenant_name', label: '租客姓名', type: 'text' },
    { prop: 'tenant_phone', label: '联系电话', type: 'text' },
  ]
  if (isReserve) {
    list.push({ prop: 'earnest_money', label: '定金金额', type: 'number' })
    list.push({ divider: '约定信息（正式签约时可微调）' })
  }
  list.push(
    { prop: 'rent_price', label: '月租金', type: 'number' },
    { prop: 'management_fee', label: '管理费', type: 'number' },
    { prop: 'deposit', label: '押金', type: 'number' },
    { prop: 'start_date', label: isReserve ? '预计起租' : '起租日期', type: 'date' },
    { prop: 'end_date', label: isReserve ? '预计结束' : '结束日期', type: 'date' },
  )
  return list
})

// 数字输入：空串置 null，其余转数字（与原三个弹窗写法一致）
function toNumber(v) {
  return v === '' ? null : Number(v)
}

// 供父级提交前调用，行为与原 el-form.validate 一致
function validate() {
  return formRef.value.validate()
}

defineExpose({ validate })
</script>

<style scoped>
.rent-historical-hint {
  background: #fdf6ec;
  border: 1px solid #faecd8;
  padding: 10px 12px;
  border-radius: 6px;
  font-size: 13px;
  color: #e6a23c;
  line-height: 1.6;
  margin: 0 0 12px 12px;
}
.rent-historical-hint .el-checkbox {
  display: flex;
  margin-top: 4px;
  white-space: normal;
}
</style>
