import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useLocationStore } from '../src/stores/locations'

describe('Location Store fullTree 合并', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('自定义区域下的自定义街道与村/小区会出现在树中', () => {
    const store = useLocationStore()
    store.customs = [
      { id: 1, level: 1, district: '新区', street: '', name: '新区' },
      { id: 2, level: 2, district: '新区', street: '', name: '新区街道' },
      { id: 3, level: 3, district: '新区', street: '新区街道', name: '幸福小区' },
    ]
    const d = store.fullTree.find(x => x.label === '新区')
    expect(d).toBeTruthy()
    expect(d.streets.map(s => s.label)).toContain('新区街道')
    expect(store.villagesOf('新区', '新区街道')).toContain('幸福小区')
    expect(store.streetsOf('新区')).toHaveLength(1)
  })

  it('静态区域下的自定义街道与村/小区正常并入', () => {
    const store = useLocationStore()
    store.customs = [
      { id: 4, level: 2, district: '福田区', street: '', name: '自定义街道' },
      { id: 5, level: 3, district: '福田区', street: '自定义街道', name: '自定义小区' },
    ]
    expect(store.streetsOf('福田区').map(s => s.label)).toContain('自定义街道')
    expect(store.villagesOf('福田区', '自定义街道')).toContain('自定义小区')
  })

  it('自定义街道与静态街道归一化同名时不重复添加', () => {
    const store = useLocationStore()
    store.customs = [
      { id: 6, level: 2, district: '福田区', street: '', name: '南园' },
    ]
    const matches = store.streetsOf('福田区').filter(s => s.label === '南园街道' || s.label === '南园')
    expect(matches).toHaveLength(1)
  })

  it('自定义村/小区与静态同名时不重复添加', () => {
    const store = useLocationStore()
    store.customs = [
      { id: 7, level: 3, district: '福田区', street: '南园街道', name: '沙埔头社区' },
    ]
    const villages = store.villagesOf('福田区', '南园街道')
    expect(villages.filter(v => v.replace(/社区$/, '') === '沙埔头')).toHaveLength(1)
  })

  it('仅数据库录入的区域下的自定义街道与村/小区会补挂', () => {
    const store = useLocationStore()
    store.dbLocations = [{ name: '飞地区', streets: [{ name: '录入街道', villages: ['录入村'] }] }]
    store.customs = [
      { id: 8, level: 2, district: '飞地区', street: '', name: '自建街道' },
      { id: 9, level: 3, district: '飞地区', street: '录入街道', name: '自建小区' },
    ]
    const labels = store.streetsOf('飞地区').map(s => s.label)
    expect(labels).toContain('录入街道')
    expect(labels).toContain('自建街道')
    expect(store.villagesOf('飞地区', '录入街道')).toContain('自建小区')
    expect(store.villagesOf('飞地区', '录入街道')).toContain('录入村')
  })

  it('自定义区域下的街道不因改名映射丢失', () => {
    const store = useLocationStore()
    store.customs = [
      { id: 10, level: 1, district: '新区', street: '', name: '新区' },
      { id: 11, level: 2, district: '新区', street: '', name: '旧街名' },
    ]
    store.renames = [{ level: 2, district: '新区', street: '', old_name: '旧街名', new_name: '新街名' }]
    expect(store.streetsOf('新区').map(s => s.label)).toContain('新街名')
  })
})
