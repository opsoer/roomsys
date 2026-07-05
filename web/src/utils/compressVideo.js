import { FFmpeg } from '@ffmpeg/ffmpeg'
import { fetchFile } from '@ffmpeg/util'

let ffmpeg = null
let loadPromise = null

export async function compressVideo(file) {
  if (!file.type.startsWith('video/')) return file
  if (file.size < 5 * 1024 * 1024) return file

  if (!ffmpeg) {
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
    } catch (e) {
      loadPromise = null
      ffmpeg = null
      throw e
    }
  }

  const ext = file.name.match(/\.(\w+)$/)?.[1] || 'mp4'
  const inputName = `input.${ext}`
  const outputName = 'output.mp4'

  ffmpeg.writeFile(inputName, await fetchFile(file))
  await ffmpeg.exec([
    '-i', inputName,
    '-b:v', '2M',
    '-b:a', '128k',
    '-vf', 'scale=-2:720',
    '-c:v', 'libx264',
    '-preset', 'fast',
    '-movflags', '+faststart',
    outputName
  ])

  const data = await ffmpeg.readFile(outputName)
  ffmpeg.deleteFile(inputName)
  ffmpeg.deleteFile(outputName)

  const blob = new Blob([data.buffer], { type: 'video/mp4' })
  if (blob.size >= file.size * 0.9) return file

  return new File([blob], file.name.replace(/\.[^.]+$/, '.mp4'), { type: 'video/mp4' })
}
