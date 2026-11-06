import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import ContractForm from '../src/components/room/ContractForm.vue'

// 回归背景：required 规则叠加 whitespace 后走了字符串校验分支，
// 数字金额（toNumber 转换）被误判必填未填，导致「确定预订」被静默拦下（点击无任何反应）。
// 数字字段必须显式 type:'number'。

const mountForm = (form, mode = 'reserve') =>
  mount(ContractForm, {
    props: { mode, modelValue: form },
    global: { plugins: [ElementPlus] },
    attachTo: document.body,
  })

const filledReserve = () => ({
  tenant_name: 'zz',
  tenant_phone: '13800000000',
  earnest_money: 1,
  rent_price: 1,
  management_fee: 1,
  deposit: 1,
  start_date: '2026-11-04',
  end_date: '2026-11-30',
})

describe('ContractForm 校验规则', () => {
  it('预订表单填入数字金额应通过校验（回归：点确定无反应）', async () => {
    const wrapper = mountForm(filledReserve())
    await expect(wrapper.vm.validate()).resolves.toBe(true)
    wrapper.unmount()
  })

  it('金额为空仍须拦截并提示必填', async () => {
    const form = filledReserve()
    form.rent_price = null
    form.earnest_money = null
    const wrapper = mountForm(form)
    await expect(wrapper.vm.validate()).rejects.toMatchObject({
      rent_price: [{ message: '请输入租金' }],
      earnest_money: [{ message: '请输入定金金额' }],
    })
    wrapper.unmount()
  })

  it('租客姓名纯空格仍须拦截（保留 whitespace 语义）', async () => {
    const form = filledReserve()
    form.tenant_name = '   '
    const wrapper = mountForm(form)
    await expect(wrapper.vm.validate()).rejects.toMatchObject({
      tenant_name: [{ message: '请输入租客姓名' }],
    })
    wrapper.unmount()
  })

  it('负数金额与 0 定金仍须拦截', async () => {
    const form = filledReserve()
    form.rent_price = -5
    form.earnest_money = 0
    const wrapper = mountForm(form)
    await expect(wrapper.vm.validate()).rejects.toMatchObject({
      rent_price: [{ message: '租金不能为负数' }],
      earnest_money: [{ message: '定金必须大于0' }],
    })
    wrapper.unmount()
  })

  it('0 管理费/押金合法（免管理费/无押金场景）', async () => {
    const form = filledReserve()
    form.management_fee = 0
    form.deposit = 0
    const wrapper = mountForm(form)
    await expect(wrapper.vm.validate()).resolves.toBe(true)
    wrapper.unmount()
  })
})
