// 品牌化二维码：Canvas 合成干净的二维码卡片（标题 + 二维码 + 信息行）并下载

export function siteHomeUrl() {
  return `${window.location.origin}/`
}

// 是否微信内置浏览器
export function isWechat() {
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
  const H = infoTop + lines.length * rowH + 80
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

  // 信息行：label（灰） + value（深色），左对齐
  ctx.textAlign = 'left'
  const labelX = 70
  const labelW = 110
  const valueX = labelX + labelW
  const valueMaxW = W - valueX - 40
  lines.forEach((line, i) => {
    const y = infoTop + 12 + i * rowH + rowH / 2
    ctx.font = `24px ${FONT}`
    ctx.fillStyle = '#9a9a9a'
    ctx.fillText(line.label, labelX, y)
    ctx.font = `26px ${FONT}`
    ctx.fillStyle = '#333333'
    ctx.fillText(truncateText(ctx, line.value, valueMaxW), valueX, y)
  })

  // 底部
  ctx.font = `20px ${FONT}`
  ctx.fillStyle = '#a8a8a8'
  ctx.textAlign = 'center'
  ctx.fillText(footer, cx, H - 36)

  return canvas.toDataURL('image/png')
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

// 生成并下载公寓二维码卡片，返回主页链接（供调用方提示）
export async function downloadBuildingQr(buildingId, buildingName, b = {}) {
  const url = buildingHomeUrl(buildingId)
  const dataUrl = await generateBrandedQrDataUrl({
    text: url,
    title: buildingName || '公寓主页',
    lines: buildingInfoLines(b),
  })
  downloadQrImage(dataUrl, `公寓二维码_${buildingName || '公寓'}.png`)
  return url
}

// 生成并下载网站主页二维码卡片，返回主页链接
export async function generateSiteQrDataUrl() {
  return generateBrandedQrDataUrl({
    text: siteHomeUrl(),
    title: '圳好租',
    lines: [{ label: '平台', value: '圳好租 · 深圳公寓租赁平台' }],
    footer: '想租房，就来圳好租',
  })
}

export async function downloadSiteQr() {
  const dataUrl = await generateSiteQrDataUrl()
  downloadQrImage(dataUrl, '网站主页二维码.png')
  return siteHomeUrl()
}