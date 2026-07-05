import { FFmpeg } from '@ffmpeg/ffmpeg'
import { fetchFile, toBlobURL } from '@ffmpeg/util'

let ffmpeg = null
let loadPromise = null

const CORE_URL = 'https://unpkg.com/@ffmpeg/core@0.12.10/dist/esm'

async function getFFmpeg() {
  if (ffmpeg) return ffmpeg
  if (loadPromise) return loadPromise

  loadPromise = (async () => {
    try {
      const instance = new FFmpeg()
      await instance.load({
        coreURL: await toBlobURL(`${CORE_URL}/ffmpeg-core.js`, 'text/javascript'),
        wasmURL: await toBlobURL(`${CORE_URL}/ffmpeg-core.wasm`, 'application/wasm'),
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

export async function compressVideo(file) {
  if (!file.type.startsWith('video/')) return file
  if (file.size < 5 * 1024 * 1024) return file

  try {
    const ff = await getFFmpeg()
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
