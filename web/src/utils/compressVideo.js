import { FFmpeg } from '@ffmpeg/ffmpeg'
import { fetchFile } from '@ffmpeg/util'

let ffmpeg = null
let loadPromise = null

const CORE_URL = '/api/ffmpeg'

function fetchBlobWithProgress(url, onProgress) {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('GET', url)
    xhr.responseType = 'blob'
    xhr.onprogress = (e) => {
      if (onProgress) onProgress(e.total ? Math.round((e.loaded / e.total) * 100) : -1)
    }
    xhr.onload = () => resolve(URL.createObjectURL(xhr.response))
    xhr.onerror = () => reject(new Error('下载失败'))
    xhr.send()
  })
}

async function getFFmpeg(onProgress) {
  if (ffmpeg) return ffmpeg
  if (loadPromise) return loadPromise

  loadPromise = (async () => {
    try {
      const instance = new FFmpeg()
      await instance.load({
        coreURL: await fetchBlobWithProgress(`${CORE_URL}/ffmpeg-core.js`, onProgress),
        wasmURL: await fetchBlobWithProgress(`${CORE_URL}/ffmpeg-core.wasm`, onProgress),
      })
      ffmpeg = instance
      return instance
    } catch (e) {
      loadPromise = null
      const err = new Error('当前设备不支持视频压缩: ' + (e.message || ''))
      err.code = 'COMPRESS_UNSUPPORTED'
      throw err
    }
  })()

  return loadPromise
}

export async function compressVideo(file, onLoadProgress) {
  if (!file.type.startsWith('video/')) return file
  if (file.size < 5 * 1024 * 1024) return file

  try {
    const ff = await getFFmpeg(onLoadProgress)
    const ext = file.name.match(/\.(\w+)$/)?.[1] || 'mp4'
    const inputName = `input.${ext}`
    const outputName = 'output.mp4'

    ff.writeFile(inputName, await fetchFile(file))
    await ff.exec([
      '-i', inputName,
      '-b:v', '2M',
      '-b:a', '128k',
      '-vf', 'scale=-2:720',
      '-c:v', 'libx264',
      '-preset', 'fast',
      '-movflags', '+faststart',
      outputName
    ])

    const data = await ff.readFile(outputName)
    ff.deleteFile(inputName)
    ff.deleteFile(outputName)

    const blob = new Blob([data.buffer], { type: 'video/mp4' })
    if (blob.size >= file.size * 0.9) return file

    return new File([blob], file.name.replace(/\.[^.]+$/, '.mp4'), { type: 'video/mp4' })
  } catch (e) {
    if (e.code !== 'COMPRESS_UNSUPPORTED') {
      console.warn('Video compression failed:', e)
    }
    throw e
  }
}
