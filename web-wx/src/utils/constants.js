export const FLOOR_OPTIONS = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15]

export const LAYOUT_OPTIONS = ['单间', '一室一厅', '两室一厅', '三室一厅', '四室及以上']

// 账单子类型的表单下拉选项；后端自动生成的账单还用「押金退还」「定金违约」，
// 调整时注意与列表页的筛选/展示保持一致。
export const SUBTYPE_INCOME = ['租金', '定金', '押金', '水电费', '其他']
export const SUBTYPE_EXPENSE = ['物业费', '维修费', '清洁费', '税费', '其他']

// 房间筛选项。注意不含"已过期(expired)"：过期是 DynamicRoomStatus 的动态派生状态，
// 应按需再补充（value 同后端 utils/time.go 的枚举）。
export const ROOM_STATUS_OPTIONS = [
  { label: '全部', value: '' },
  { label: '未出租', value: 'vacant' },
  { label: '已预订', value: 'reserved' },
  { label: '已出租', value: 'rented' },
  { label: '即将到期', value: 'expiring' },
]
