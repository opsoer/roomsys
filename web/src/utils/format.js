export function mediaUrl(path) {
  if (!path) return ''
  if (path.includes('..') || path.includes('\\')) return ''
  return `/api/media/${path}`
}

// 复制文本到剪贴板：优先 Clipboard API；微信内置浏览器（尤其 HTTP 域名）会禁用 Clipboard API，
// 且 textarea.select() 会唤起 iOS 系统"选中/复制"空白浮层（页面居中的空白框）。
// 因此降级路径采用 copy 事件注入剪贴板（ClipboardJS 同款方案）：不 focus、不依赖选区、复制后立即清理，
// 全程不产生系统选区浮层，兼容微信安卓/iOS。
export async function copyText(text) {
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {
      // 微信等环境会拒绝（Write permission denied），继续走降级方案
    }
  }
  const ta = document.createElement('textarea')
  ta.value = text
  ta.setAttribute('readonly', '')
  ta.style.contain = 'strict'
  ta.style.position = 'fixed'
  ta.style.top = '0'
  ta.style.left = '-9999px'
  ta.style.fontSize = '12pt'
  ta.style.opacity = '0'
  document.body.appendChild(ta)

  const selection = document.getSelection()
  const originalRange = selection.rangeCount > 0 ? selection.getRangeAt(0) : null

  // 通过 copy 事件把文本写入剪贴板，而不是依赖"选中文本"
  let ok = false
  const onCopy = (e) => {
    e.preventDefault()
    e.clipboardData.setData('text/plain', text)
    ok = true
  }
  document.addEventListener('copy', onCopy)
  try {
    // 微信安卓内核需要选区存在才会广播 copy 事件；iOS 上 copy 事件可脱离选区触发。
    // 无论是否产生临时选区，事件处理器都会完成写入，且下方会立即清除选中与浮层。
    ta.select()
    ta.setSelectionRange(0, ta.value.length)
    try {
      document.execCommand('copy')
    } catch {
      // execCommand 在部分环境会抛错，但 copy 事件处理器已写入剪贴板
    }
  } finally {
    document.removeEventListener('copy', onCopy)
    document.body.removeChild(ta)
    // 立即清除选中态，确保 iOS 系统"选中/复制"浮层不残留
    selection.removeAllRanges()
    if (originalRange) selection.addRange(originalRange)
  }
  return ok
}

export function maskName(name) {
  if (!name) return ''
  return name.charAt(0) + '***'
}

export function maskPhone(phone) {
  if (!phone || phone.length < 7) return phone
  return phone.slice(0, 3) + '****' + phone.slice(-4)
}

export function statusLabel(status) {
  const labels = {
    vacant: '未出租',
    reserved: '已预订',
    rented: '已出租',
    expiring: '即将到期',
    expired: '已过期',
  }
  return labels[status] || status
}

export function statusTagType(status) {
  const types = {
    vacant: 'success',
    reserved: 'primary',
    rented: 'danger',
    expiring: 'warning',
    expired: 'danger',
  }
  return types[status] || 'info'
}
