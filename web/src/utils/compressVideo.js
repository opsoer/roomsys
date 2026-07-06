import { FFmpeg } from '@ffmpeg/ffmpeg'
import { fetchFile } from '@ffmpeg/util'

let ffmpeg = null
let loadPromise = null
let compressionQueue = Promise.resolve()

export function compressVideo(file, { timeout = 120000, signal } = {}) {
  if (!file.type.startsWith('video/')) return Promise.resolve(file)
  if (file.size < 5 * 1024 * 1024) return Promise.resolve(file)

  const job = compressionQueue.then(() => doCompress(file, timeout, signal))
  compressionQueue = job.catch(() => {})
  return job
}

function destroyFFmpeg() {
  if (ffmpeg) {
    try { ffmpeg.terminate?.() } catch {}
    ffmpeg = null
    loadPromise = null
  }
}

async function loadFFmpeg(retry = false) {
  if (ffmpeg) return ffmpeg
  if (!loadPromise) {
    loadPromise = (async () => {
      const instance = new FFmpeg()
      await instance.load({
        coreURL: '/api/ffmpeg/ffmpeg-core.js',
        wasmURL: '/api/ffmpeg/ffmpeg-core.wasm',
      })
      ffmpeg = instance
    })()
  }
  try {
    await loadPromise
    return ffmpeg
  } catch (e) {
    loadPromise = null
    ffmpeg = null
    if (!retry) {
      try {
        const { reDownloadFFmpeg } = await import('../api')
        await reDownloadFFmpeg()
        ffmpeg = null
        loadPromise = null
        return await loadFFmpeg(true)
      } catch {}
    }
    throw e
  }
}

function withTimeout(promise, ms) {
  if (ms <= 0) return promise
  let timer
  const timeoutPromise = new Promise((_, reject) => {
    timer = setTimeout(
      () => reject(Object.assign(new Error('压缩超时'), { code: 'COMPRESS_TIMEOUT' })),
      ms,
    )
  })
  const wrapped = Promise.race([promise, timeoutPromise])
  wrapped.finally(() => clearTimeout(timer))
  return wrapped
}

async function doCompress(file, timeout, signal) {
  await loadFFmpeg()

  if (signal?.throwIfAborted) signal.throwIfAborted()

  const uid = `${Date.now()}_${Math.random().toString(36).slice(2, 8)}`
  const ext = file.name.match(/\.(\w+)$/)?.[1] || 'mp4'
  const inputName = `in_${uid}.${ext}`
  const outputName = `out_${uid}.mp4`
  const cleaned = [inputName, outputName]
  let timedOut = false
  let aborted = false

  const abortHandler = () => {
    aborted = true
    destroyFFmpeg()
  }
  signal?.addEventListener?.('abort', abortHandler, { once: true })

  try {
    const fileData = await fetchFile(file)

    if (aborted) throw Object.assign(new Error('已取消'), { code: 'COMPRESS_ABORTED' })

    await withTimeout(ffmpeg.writeFile(inputName, fileData), timeout)

    if (aborted) throw Object.assign(new Error('已取消'), { code: 'COMPRESS_ABORTED' })

    await withTimeout(
      ffmpeg.exec([
        '-i', inputName,
        '-b:v', '2M',
        '-b:a', '128k',
        '-vf', 'scale=-2:720',
        '-c:v', 'libx264',
        '-preset', 'fast',
        '-movflags', '+faststart',
        outputName,
      ]),
      timeout,
    )

    const data = await withTimeout(ffmpeg.readFile(outputName), timeout)
    const blob = new Blob([data.buffer], { type: 'video/mp4' })

    return blob.size >= file.size * 0.9
      ? file
      : new File([blob], file.name.replace(/\.[^.]+$/, '.mp4'), { type: 'video/mp4' })
  } catch (e) {
    if (e.code === 'COMPRESS_TIMEOUT') timedOut = true
    throw e
  } finally {
    signal?.removeEventListener?.('abort', abortHandler)
    if (timedOut || aborted) {
      destroyFFmpeg()
    } else {
      for (const name of cleaned) {
        try { await ffmpeg.deleteFile(name) } catch {}
      }
      destroyFFmpeg()
    }
  }
}
