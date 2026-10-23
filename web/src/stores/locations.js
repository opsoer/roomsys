// 位置选项统一数据源：静态官方区划（只读基座）+ 数据库自定义条目 + 静态改名映射 + 公寓实际录入位置，
// 合并出一棵完整的位置树，主页筛选与创建/编辑公寓弹窗共用，保证各处选项一致。
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { getBuildingLocations, getLocationOptions, getLocationManage } from '../api'
import shenzhen from '../utils/shenzhen'

export const useLocationStore = defineStore('locations', () => {
  // 公寓实际使用的位置（区域→街道→村/小区），来自公开聚合接口
  const dbLocations = ref([])
  // 超管维护的自定义条目与静态条目改名映射
  const customs = ref([])
  const renames = ref([])
  const loaded = ref(false)

  async function load(force = false) {
    if (loaded.value && !force) return
    const tasks = [
      getBuildingLocations().then(res => { dbLocations.value = res.data.locations || [] }).catch(() => { dbLocations.value = [] }),
      getLocationOptions().then(res => {
        customs.value = res.data.customs || []
        renames.value = res.data.renames || []
      }).catch(() => {
        customs.value = []
        renames.value = []
      }),
    ]
    await Promise.all(tasks)
    loaded.value = true
  }

  // loadManage 超管管理端使用：读取带 id 的全量数据（新增/删除自定义条目需要 id）。
  // 注意与 load() 共用 loaded 缓存标志：若公有端 load() 先执行过，这里不带 force 会
  // 直接命中缓存、拿到的可能是无 id 的 options 数据，管理端入口应保证先走 loadManage
  // （或在切换角色时调用 invalidate()）。
  async function loadManage(force = false) {
    if (loaded.value && !force) return
    const tasks = [
      getBuildingLocations().then(res => { dbLocations.value = res.data.locations || [] }).catch(() => { dbLocations.value = [] }),
      getLocationManage().then(res => {
        customs.value = res.data.customs || []
        renames.value = res.data.renames || []
      }).catch(() => {
        customs.value = []
        renames.value = []
      }),
    ]
    await Promise.all(tasks)
    loaded.value = true
  }

  function invalidate() {
    loaded.value = false
  }

  // 街道归一化：录入可能带或不带「街道」后缀（如「石岩」与「石岩街道」指同一地方）
  const normStreet = (s) => (s || '').replace(/街道$/, '')
  // 村/小区归一化：去掉「社区」后缀，用于改名映射键与全树去重判等
  // （如「沙埔头」与「沙埔头社区」视为同一）。
  const normVillage = (v) => (v || '').replace(/社区$/, '')

  // 改名映射：level + 上下文(区域/街道) + 旧名 → 新名
  const renameMap = computed(() => {
    const map = {}
    for (const r of renames.value) {
      const key = r.level === 1 ? `1:${r.old_name}` : r.level === 2 ? `2:${r.district}|${normStreet(r.old_name)}` : `3:${r.district}|${normStreet(r.street)}|${normVillage(r.old_name)}`
      map[key] = r.new_name
    }
    return map
  })

  function applyRename(level, district, street, name) {
    const key = level === 1 ? `1:${name}` : level === 2 ? `2:${district}|${normStreet(name)}` : `3:${district}|${normStreet(street)}|${normVillage(name)}`
    return renameMap.value[key] || name
  }

  // 合并后的完整位置树，形状与 shenzhen.js 一致：
  // [{ value, label, streets: [{ value, label, villages: [] }] }]
  // 村/小区排序：有房源的（数据库实际录入）在前，其后为自定义 + 静态基座（应用改名映射后）
  const fullTree = computed(() => {
    // 数据库实际位置转 map：区域 → { 街道(归一化) → [村/小区] }，区域/街道保留原始录入名
    const dbMap = {}
    for (const d of dbLocations.value) {
      const streets = {}
      for (const s of d.streets || []) streets[s.name] = s.villages || []
      dbMap[d.name] = streets
    }

    const tree = []
    const districtIndex = {} // 归一化区域名 → tree 下标

    function ensureDistrict(name) {
      const idx = districtIndex[name]
      if (idx !== undefined) return tree[idx]
      const node = { value: name, label: name, streets: [] }
      districtIndex[name] = tree.length
      tree.push(node)
      return node
    }

    // 1) 静态基座（应用改名映射）
    for (const d of shenzhen) {
      const dName = applyRename(1, '', '', d.label)
      const dNode = ensureDistrict(dName)
      const streetIndex = {}
      for (const s of d.streets) {
        const sName = applyRename(2, d.label, '', s.label)
        const sNode = { value: sName, label: sName, villages: [] }
        streetIndex[normStreet(sName)] = sNode
        dNode.streets.push(sNode)
      }
      // 2) 自定义街道并入（与静态同名则并入既有条目）
      for (const c of customs.value) {
        if (c.level === 2 && c.district === dName) {
          const cName = applyRename(2, c.district, '', c.name)
          const existing = streetIndex[normStreet(cName)]
          if (existing) continue
          const sNode = { value: cName, label: cName, villages: [] }
          streetIndex[normStreet(cName)] = sNode
          dNode.streets.push(sNode)
        }
      }
      // 3) 村/小区：数据库实际录入在前，其后自定义，再静态（应用改名），去重
      for (const sNode of dNode.streets) {
        const seen = new Set()
        const villages = []
        const push = (v) => {
          const key = normVillage(v)
          if (key && !seen.has(key)) {
            seen.add(key)
            villages.push(v)
          }
        }
        const dbStreetKey = Object.keys(dbMap[dName] || {}).find(k => normStreet(k) === normStreet(sNode.label))
        if (dbStreetKey !== undefined) for (const v of dbMap[dName][dbStreetKey] || []) push(v)
        for (const c of customs.value) {
          if (c.level === 3 && c.district === dName && normStreet(c.street) === normStreet(sNode.label)) push(c.name)
        }
        // 静态村/小区挂在归一化后同名（含改名来源）的街道上
        const sourceStreet = d.streets.find(s => normStreet(applyRename(2, d.label, '', s.label)) === normStreet(sNode.label))
        for (const v of sourceStreet ? sourceStreet.villages : []) push(applyRename(3, d.label, sNode.label, v))
        sNode.villages = villages
      }
    }

    // 4) 数据库中存在、静态与自定义都没有的区域/街道（自由录入）补进树
    for (const dName of Object.keys(dbMap)) {
      let dNode = tree.find(n => n.label === dName)
      if (!dNode) dNode = ensureDistrict(dName)
      for (const [sName, villages] of Object.entries(dbMap[dName] || {})) {
        let sNode = dNode.streets.find(s => normStreet(s.label) === normStreet(sName))
        if (!sNode) {
          sNode = { value: sName, label: sName, villages: [] }
          dNode.streets.push(sNode)
        }
        const seen = new Set(sNode.villages.map(normVillage))
        for (const v of villages) {
          if (!seen.has(normVillage(v))) {
            seen.add(normVillage(v))
            sNode.villages.unshift(v)
          }
        }
      }
    }

    // 5) 自定义区域（静态没有的）
    for (const c of customs.value) {
      if (c.level === 1) ensureDistrict(applyRename(1, '', '', c.name))
    }

    // 6) 自定义街道补挂：步骤 2 只在静态区域循环内并入，自定义区域（及仅数据库录入的区域）下的街道会漏掉
    for (const c of customs.value) {
      if (c.level !== 2) continue
      const dName = applyRename(1, '', '', c.district)
      const dNode = tree.find(n => n.label === dName) || ensureDistrict(dName)
      const cName = applyRename(2, c.district, '', c.name)
      if (dNode.streets.some(s => normStreet(s.label) === normStreet(cName))) continue
      dNode.streets.push({ value: cName, label: cName, villages: [] })
    }

    // 7) 自定义村/小区补挂：步骤 3 同样只覆盖静态区域，非静态街道下的小区会漏掉
    for (const c of customs.value) {
      if (c.level !== 3) continue
      const dName = applyRename(1, '', '', c.district)
      const dNode = tree.find(n => n.label === dName) || ensureDistrict(dName)
      const sName = applyRename(2, c.district, '', c.street)
      let sNode = dNode.streets.find(s => normStreet(s.label) === normStreet(sName))
      if (!sNode) {
        sNode = { value: sName, label: sName, villages: [] }
        dNode.streets.push(sNode)
      }
      if (!sNode.villages.some(v => normVillage(v) === normVillage(c.name))) sNode.villages.push(c.name)
    }

    return tree
  })

  function streetsOf(districtLabel) {
    return fullTree.value.find(d => d.label === districtLabel)?.streets || []
  }

  function villagesOf(districtLabel, streetLabel) {
    return streetsOf(districtLabel).find(s => s.label === streetLabel)?.villages || []
  }

  return { dbLocations, customs, renames, loaded, load, loadManage, invalidate, fullTree, streetsOf, villagesOf }
})
