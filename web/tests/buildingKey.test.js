import { describe, it, expect } from 'vitest'
import { nameAbbr, buildingKey, roomKey, parseIdKey } from '../src/utils/buildingKey'

describe('buildingKey 公开页 URL 标识', () => {
  it('nameAbbr 中文取拼音首字母', () => {
    expect(nameAbbr('阳光小院')).toBe('ygxy')
    expect(nameAbbr('福田公寓')).toBe('ftgy')
  })

  it('nameAbbr 混合字母数字，非字母数字丢弃', () => {
    expect(nameAbbr('A座1栋')).toBe('az1d')
    expect(nameAbbr('Building 101!')).toBe('building101')
  })

  it('nameAbbr 空值与超长截断', () => {
    expect(nameAbbr('')).toBe('')
    expect(nameAbbr(null)).toBe('')
    const long = '一二三四五六七八九十甲乙丙丁戊己庚辛'
    expect(nameAbbr(long).length).toBeLessThanOrEqual(12)
  })

  it('buildingKey 有名称时「缩写_id」，无名称时纯数字', () => {
    expect(buildingKey(1, '阳光小院')).toBe('ygxy_1')
    expect(buildingKey(12, '')).toBe('12')
    expect(buildingKey(12)).toBe('12')
    expect(buildingKey(null, '阳光小院')).toBe('')
  })

  it('roomKey 房号做缩写', () => {
    expect(roomKey(8, '205')).toBe('205_8')
    expect(roomKey(12, 'A301')).toBe('a301_12')
    expect(roomKey(8, '')).toBe('8')
    expect(roomKey(null, '205')).toBe('')
  })

  it('parseIdKey 取最后一个下划线后的数字', () => {
    expect(parseIdKey('ygxy_1')).toBe('1')
    expect(parseIdKey('205_8')).toBe('8')
    expect(parseIdKey('1')).toBe('1')
  })

  it('parseIdKey 非法输入返回空串', () => {
    expect(parseIdKey('abc')).toBe('')
    expect(parseIdKey('ygxy_')).toBe('')
    expect(parseIdKey('')).toBe('')
    expect(parseIdKey(null)).toBe('')
    expect(parseIdKey(undefined)).toBe('')
  })
})
