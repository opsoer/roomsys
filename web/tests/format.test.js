import { describe, it, expect } from 'vitest'
import { mediaUrl, maskName, maskPhone, statusLabel, statusTagType } from '../src/utils/format'

describe('mediaUrl', () => {
  it('returns empty for null/undefined/empty', () => {
    expect(mediaUrl(null)).toBe('')
    expect(mediaUrl(undefined)).toBe('')
    expect(mediaUrl('')).toBe('')
  })

  it('blocks path traversal', () => {
    expect(mediaUrl('../secret')).toBe('')
    expect(mediaUrl('..\\windows')).toBe('')
    expect(mediaUrl('path/../../etc')).toBe('')
  })

  it('returns correct URL for valid path', () => {
    expect(mediaUrl('uploads/1.jpg')).toBe('/api/media/uploads/1.jpg')
    expect(mediaUrl('test.png')).toBe('/api/media/test.png')
  })
})

describe('maskName', () => {
  it('returns empty for null/undefined/empty', () => {
    expect(maskName(null)).toBe('')
    expect(maskName(undefined)).toBe('')
    expect(maskName('')).toBe('')
  })

  it('masks name correctly', () => {
    expect(maskName('张')).toBe('张***')
    expect(maskName('张三')).toBe('张***')
    expect(maskName('欧阳锋')).toBe('欧***')
  })
})

describe('maskPhone', () => {
  it('returns original for short numbers', () => {
    expect(maskPhone(null)).toBe(null)
    expect(maskPhone('123')).toBe('123')
    expect(maskPhone('123456')).toBe('123456')
  })

  it('masks phone correctly', () => {
    expect(maskPhone('13812345678')).toBe('138****5678')
    expect(maskPhone('07551234567')).toBe('075****4567')
  })
})

describe('statusLabel', () => {
  it('returns Chinese label for known status', () => {
    expect(statusLabel('vacant')).toBe('未出租')
    expect(statusLabel('rented')).toBe('已出租')
    expect(statusLabel('expiring')).toBe('即将到期')
    expect(statusLabel('expired')).toBe('已过期')
  })

  it('returns original for unknown status', () => {
    expect(statusLabel('unknown')).toBe('unknown')
    expect(statusLabel('')).toBe('')
  })
})

describe('statusTagType', () => {
  it('returns correct Element Plus tag type', () => {
    expect(statusTagType('vacant')).toBe('success')
    expect(statusTagType('rented')).toBe('danger')
    expect(statusTagType('expiring')).toBe('warning')
    expect(statusTagType('expired')).toBe('danger')
  })

  it('returns info for unknown status', () => {
    expect(statusTagType('unknown')).toBe('info')
    expect(statusTagType('')).toBe('info')
  })
})
