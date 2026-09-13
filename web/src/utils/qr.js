// 品牌化二维码：Canvas 合成干净的二维码卡片（标题 + 二维码 + 信息行）并下载

export function siteHomeUrl() {
  return `${window.location.origin}/`
}

// 是否微信内置浏览器（canAutoDownloadImage 内部使用）
function isWechat() {
  return /MicroMessenger/i.test(navigator.userAgent)
}

// 是否移动端设备（含微信）——此类环境 <a download> 无法保存图片到本地，需引导长按保存
export function canAutoDownloadImage() {
  return !isWechat() && !/Android|iPhone|iPad|iPod|Mobile/i.test(navigator.userAgent)
}

export function buildingHomeUrl(buildingId) {
  return `${window.location.origin}/building/${buildingId}`
}

export function roomHomeUrl(buildingId, roomId) {
  return `${window.location.origin}/building/${buildingId}/room/${roomId}`
}

// 从公寓对象提取信息行（有值才显示）
export function buildingInfoLines(b) {
  const lines = []
  const address = [b?.district, b?.street, b?.village, b?.building_no].filter(Boolean).join(' ')
  if (address) lines.push({ label: '位置', value: address })
  if (b?.room_count !== null) lines.push({ label: '房源', value: `${b.room_count}间 · 可租${b.vacant_count ?? 0}间` })
  return lines
}

function roundRectPath(ctx, x, y, w, h, r) {
  ctx.beginPath()
  ctx.moveTo(x + r, y)
  ctx.arcTo(x + w, y, x + w, y + h, r)
  ctx.arcTo(x + w, y + h, x, y + h, r)
  ctx.arcTo(x, y + h, x, y, r)
  ctx.arcTo(x, y, x + w, y, r)
  ctx.closePath()
}

function truncateText(ctx, text, maxWidth) {
  if (!text) return ''
  if (ctx.measureText(text).width <= maxWidth) return text
  let t = text
  while (t.length > 1 && ctx.measureText(t + '…').width > maxWidth) {
    t = t.slice(0, -1)
  }
  return t + '…'
}

// 按宽度折行（中文逐字断行），最多 maxRows 行，仍未放下则末行加省略号
function wrapText(ctx, text, maxWidth, maxRows = 2) {
  if (!text) return ['']
  if (ctx.measureText(text).width <= maxWidth) return [text]
  const rows = []
  let rest = text
  while (rows.length < maxRows - 1) {
    let cur = ''
    let i = 0
    while (i < rest.length && ctx.measureText(cur + rest[i]).width <= maxWidth) {
      cur += rest[i]
      i++
    }
    if (!cur) break
    rows.push(cur)
    rest = rest.slice(i)
  }
  if (rest) {
    let fit = ''
    for (const ch of rest) {
      if (ctx.measureText(fit + ch).width > maxWidth) break
      fit += ch
    }
    let trimmed = fit
    while (trimmed.length > 1 && ctx.measureText(trimmed + '…').width > maxWidth) {
      trimmed = trimmed.slice(0, -1)
    }
    rows.push(fit.length < rest.length ? trimmed + '…' : fit)
  }
  return rows
}

const FONT = '"PingFang SC","Microsoft YaHei",sans-serif'

// 生成一张竖版二维码卡片（620 宽 PNG dataURL）
// opts: { text, title, lines: [{label, value}], footer }
export async function generateBrandedQrDataUrl({
  text,
  title = '',
  lines = [],
  footer = '扫码在线看房 · 一键联系房东',
}) {
  const { default: QRCode } = await import('qrcode')
  const W = 620
  const qrSize = 400
  const qrTop = 128
  const infoTop = qrTop + qrSize + 46
  const rowH = 58
  // 预折行：value 过长的信息行占多行，卡片高度按总行数计算
  const measureCtx = document.createElement('canvas').getContext('2d')
  measureCtx.font = `26px ${FONT}`
  const valueMaxW = W - 70 - 110 - 40
  const lineRows = lines.map(line => wrapText(measureCtx, line.value, valueMaxW, 2))
  const totalRows = lineRows.reduce((n, rows) => n + rows.length, 0)
  const H = infoTop + totalRows * rowH + 80
  const canvas = document.createElement('canvas')
  canvas.width = W
  canvas.height = H
  const ctx = canvas.getContext('2d')
  const cx = W / 2

  // 背景：白色圆角卡片
  roundRectPath(ctx, 0, 0, W, H, 24)
  ctx.fillStyle = '#ffffff'
  ctx.fill()

  // 标题
  ctx.font = `32px ${FONT}`
  ctx.fillStyle = '#1a1a2e'
  ctx.textAlign = 'center'
  ctx.textBaseline = 'middle'
  ctx.fillText(truncateText(ctx, title, W - 120), cx, 66)

  // 二维码白底（细边框）
  const qrX = (W - qrSize) / 2
  roundRectPath(ctx, qrX, qrTop, qrSize, qrSize, 12)
  ctx.fillStyle = '#ffffff'
  ctx.fill()
  ctx.strokeStyle = '#e6e6e6'
  ctx.lineWidth = 2
  ctx.stroke()

  const qrData = await QRCode.toDataURL(text, { width: 560, margin: 0, errorCorrectionLevel: 'H' })
  const img = new Image()
  img.src = qrData
  await new Promise((resolve, reject) => {
    img.onload = resolve
    img.onerror = reject
  })
  const pad = 18
  ctx.drawImage(img, qrX + pad, qrTop + pad, qrSize - pad * 2, qrSize - pad * 2)

  // 二维码中央品牌：白底圆角块 + 橙黄色「圳好租」
  const logoSize = 100
  const logoX = cx - logoSize / 2
  const logoY = qrTop + qrSize / 2 - logoSize / 2
  ctx.save()
  ctx.shadowColor = 'rgba(0,0,0,0.12)'
  ctx.shadowBlur = 8
  roundRectPath(ctx, logoX, logoY, logoSize, logoSize, 14)
  ctx.fillStyle = '#ffffff'
  ctx.fill()
  ctx.restore()
  ctx.font = `28px ${FONT}`
  ctx.fillStyle = '#e6a23c'
  ctx.textAlign = 'center'
  ctx.textBaseline = 'middle'
  ctx.fillText('圳好租', cx, logoY + logoSize / 2 + 1)

  // 分隔线
  ctx.strokeStyle = '#f0f0f0'
  ctx.lineWidth = 1
  ctx.beginPath()
  ctx.moveTo(80, infoTop - 22)
  ctx.lineTo(W - 80, infoTop - 22)
  ctx.stroke()

  // 信息行：label（灰，整块垂直居中） + value（深色，过长自动折两行），左对齐
  ctx.textAlign = 'left'
  const labelX = 70
  const labelW = 110
  const valueX = labelX + labelW
  let rowIndex = 0
  lines.forEach((line, i) => {
    const rows = lineRows[i]
    const y0 = infoTop + 12 + rowIndex * rowH
    ctx.font = `26px ${FONT}`
    ctx.fillStyle = '#333333'
    rows.forEach((r, ri) => {
      ctx.fillText(r, valueX, y0 + ri * rowH + rowH / 2)
    })
    ctx.font = `24px ${FONT}`
    ctx.fillStyle = '#9a9a9a'
    ctx.fillText(line.label, labelX, y0 + (rows.length * rowH) / 2)
    rowIndex += rows.length
  })

  // 底部
  ctx.font = `20px ${FONT}`
  ctx.fillStyle = '#a8a8a8'
  ctx.textAlign = 'center'
  ctx.fillText(footer, cx, H - 36)

  return canvas.toDataURL('image/png')
}

function toParamNum(v) {
  if (v === null || v === undefined || v === '') return null
  const n = Number(v)
  return Number.isFinite(n) ? n : null
}

// 筛选查询参数（村/小区 villages 为数组，链接中逗号分隔，主页解析后按多选筛选）
function filterQueryParams({ district = '', street = '', villages = [], minPrice = null, maxPrice = null, layout = '' } = {}) {
  const params = new URLSearchParams()
  if (district) params.set('district', district)
  if (street) params.set('street', street)
  if (villages.length) params.set('village', villages.join(','))
  const min = toParamNum(minPrice)
  const max = toParamNum(maxPrice)
  if (min !== null) params.set('min_price', String(min))
  if (max !== null) params.set('max_price', String(max))
  if (layout) params.set('layout', layout)
  return params
}

// 筛选结果页链接：主页 + 位置/租金/户型查询参数，扫码后主页自动按这些条件筛选
export function locationFilterUrl(opts = {}) {
  const qs = filterQueryParams(opts).toString()
  return `${window.location.origin}/${qs ? `?${qs}` : ''}`
}

// 二维码卡片上展示的链接文本：URLSearchParams 会把中文编码成 %XX，重新拼接为可读形式
function filterLinkText(opts) {
  const qs = [...filterQueryParams(opts).entries()].map(([k, v]) => `${k}=${v}`).join('&')
  return `${window.location.host}/${qs ? `?${qs}` : ''}`
}

// 生成村/小区（或街道/区域）位置二维码卡片，只展示所选位置与链接
export async function generateLocationQrDataUrl({ district, street, villages = [], minPrice, maxPrice, layout, title }) {
  const vs = (villages || []).filter(Boolean)
  const locText = [district, street].filter(Boolean).join(' ')
  const address = vs.length ? `${locText}${locText ? ' ' : ''}${vs.join('、')}` : locText
  const lines = []
  if (address) lines.push({ label: '位置', value: address })
  lines.push({ label: '链接', value: filterLinkText({ district, street, villages: vs, minPrice, maxPrice, layout }) })
  return generateBrandedQrDataUrl({
    text: locationFilterUrl({ district, street, villages: vs, minPrice, maxPrice, layout }),
    title: title || street || district || '房源筛选',
    lines,
    footer: '扫码查看符合条件的在租公寓',
  })
}

export function downloadQrImage(dataUrl, filename) {
  const a = document.createElement('a')
  a.href = dataUrl
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
}

// 生成房间二维码卡片（楼层/房号/户型/位置/租金，有值才显示）
export async function generateRoomQrDataUrl({ text, buildingName, address, floor, roomNumber, layout, price }) {
  const lines = []
  if (floor) lines.push({ label: '楼层', value: `${floor}层` })
  if (roomNumber) lines.push({ label: '房号', value: `${roomNumber}房` })
  if (layout) lines.push({ label: '户型', value: layout })
  if (address) lines.push({ label: '位置', value: address })
  if (price !== null && price !== '') lines.push({ label: '租金', value: `¥${price}/月` })
  return generateBrandedQrDataUrl({
    text,
    title: buildingName || '房间详情',
    lines,
    footer: '扫码在线看房 · 一键联系房东',
  })
}

// 生成网站主页二维码卡片
export async function generateSiteQrDataUrl() {
  return generateBrandedQrDataUrl({
    text: siteHomeUrl(),
    title: '圳好租',
    lines: [{ label: '平台', value: '圳好租 · 深圳公寓租赁平台' }],
    footer: '想租房，就来圳好租',
  })
}
