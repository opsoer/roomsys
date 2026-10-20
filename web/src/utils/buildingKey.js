// 公开页 URL 标识工具：格式为「名称缩写_数字id」（如 ygxy_1、205_8）。
// 缩写部分纯装饰（提升链接可读性，分享/复制时能一眼认出是哪栋公寓），
// 解析时只认最后一个下划线之后的数字 id，因此缩写缺失、过时或乱写都不影响访问。
import { pinyin } from 'pinyin-pro'

const MAX_ABBR = 12

// 名称 → 缩写：中文取拼音首字母，字母数字原样保留（转小写），其余字符丢弃
export function nameAbbr(name) {
  if (!name) return ''
  const chars = pinyin(String(name), { pattern: 'first', toneType: 'none', type: 'array' })
  let abbr = ''
  for (const ch of chars) {
    const c = String(ch).toLowerCase()
    if ((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) abbr += c
    if (abbr.length >= MAX_ABBR) break
  }
  return abbr
}

// 数字 id → 公开页标识：有名称时「缩写_id」，否则纯数字
export function buildingKey(id, name = '') {
  if (id === null || id === undefined || id === '') return ''
  const abbr = nameAbbr(name)
  return abbr ? `${abbr}_${id}` : String(id)
}

// 房间标识：房号做缩写（如 205_8、a301_12）
export function roomKey(roomId, roomNumber = '') {
  if (roomId === null || roomId === undefined || roomId === '') return ''
  const num = String(roomNumber).toLowerCase().replace(/[^a-z0-9]/g, '').slice(0, MAX_ABBR)
  return num ? `${num}_${roomId}` : String(roomId)
}

// 标识 → 数字 id（字符串形式）：取最后一个下划线之后的数字；无下划线则整体解析；非法返回 ''
export function parseIdKey(key) {
  if (key === null || key === undefined) return ''
  let s = String(key)
  const i = s.lastIndexOf('_')
  if (i >= 0) s = s.slice(i + 1)
  return /^\d+$/.test(s) ? s : ''
}
