import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import ContractForm from '../src/components/room/ContractForm.vue'

// 模拟用户真实打字链路：键入 → update:modelValue → toNumber → model 更新 → validate
// 回归场景：签合同出租弹窗逐字键入 1500/100/1500 后点「确定出租」必须能提交。

const mountForm = (form, mode = 'rent') =>
  mount(ContractForm, {
    props: { mode, modelValue: form },
    global: { plugins: [ElementPlus] },
    attachTo: document.body,
  })

const typeInto = async (wrapper, label, value) => {
  const items = wrapper.findAll('.el-form-item')
  const item = items.find(i => i.find('.el-form-item__label').text() === label)
  expect(item, `找不到字段 ${label}`).toBeTruthy()
  const input = item.find('input')
  await input.setValue(value)
  await input.trigger('input')
  await wrapper.vm.$nextTick()
}

describe('ContractForm 真实输入链路', () => {
  it('签合同出租：逐字段键入后 model 全部为数字且校验通过', async () => {
    const form = {
      tenant_name: '', tenant_phone: '', rent_price: null,
      management_fee: null, deposit: null, start_date: '2026-11-07', end_date: '2026-11-24',
    }
    const wrapper = mountForm(form, 'rent')
    await wrapper.vm.$nextTick()
    await typeInto(wrapper, '租客姓名', 'zz')
    await typeInto(wrapper, '月租金', '1500')
    await typeInto(wrapper, '管理费', '100')
    await typeInto(wrapper, '押金', '1500')
    expect(form.rent_price).toBe(1500)
    expect(form.management_fee).toBe(100)
    expect(form.deposit).toBe(1500)
    await expect(wrapper.vm.validate()).resolves.toBe(true)
    wrapper.unmount()
  })

  it('交定金预订：含定金字段键入后校验通过', async () => {
    const form = {
      tenant_name: '', tenant_phone: '', earnest_money: null, rent_price: null,
      management_fee: null, deposit: null, start_date: '2026-11-07', end_date: '2026-11-30',
    }
    const wrapper = mountForm(form, 'reserve')
    await wrapper.vm.$nextTick()
    await typeInto(wrapper, '租客姓名', 'zz')
    await typeInto(wrapper, '定金金额', '1')
    await typeInto(wrapper, '月租金', '1')
    await typeInto(wrapper, '管理费', '1')
    await typeInto(wrapper, '押金', '1')
    expect(form.earnest_money).toBe(1)
    expect(form.rent_price).toBe(1)
    await expect(wrapper.vm.validate()).resolves.toBe(true)
    wrapper.unmount()
  })

  it('打字中途清空金额 → model 归 null → 校验拦回必填', async () => {
    const form = {
      tenant_name: 'zz', tenant_phone: '', rent_price: 1500,
      management_fee: 100, deposit: 1500, start_date: '2026-11-07', end_date: '2026-11-24',
    }
    const wrapper = mountForm(form, 'rent')
    await wrapper.vm.$nextTick()
    await typeInto(wrapper, '月租金', '')
    expect(form.rent_price).toBeNull()
    await expect(wrapper.vm.validate()).rejects.toMatchObject({
      rent_price: [{ message: '请输入租金' }],
    })
    wrapper.unmount()
  })
})
