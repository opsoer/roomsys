// 公开端「查看房东完整电话」的共享逻辑：
// - 会话级缓存：同一公寓在公寓页/房间页之间共享已取到的完整号码，避免重复消耗每日额度；
// - 服务端规则：每个 IP 每日前 reveal_free_per_day 次直接放行，之后需图片验证码（业务码 1007）。
import { reactive } from 'vue'
import { revealBuildingPhones } from '../api'

// buildingId -> landlords 数组（含完整号码）
const cache = reactive({})

export function revealedLandlords(buildingId) {
  return cache[buildingId] || null
}

export function cacheLandlords(buildingId, landlords) {
  cache[buildingId] = landlords
}

// 请求完整号码。需要验证码时抛出错误（err.response.data.code === 1007），由调用方弹出验证码弹窗。
export async function revealLandlords(buildingId, payload) {
  const res = await revealBuildingPhones(buildingId, payload)
  cache[buildingId] = res.data.landlords
  return cache[buildingId]
}
