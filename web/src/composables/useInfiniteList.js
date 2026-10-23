import { ref, nextTick, onBeforeUnmount } from 'vue'

// 无限滚动 + 游标分页列表的通用逻辑（替代各页面复制的 fetch追加/去重兜底/游标参数/触底加载样板）
// - fetcher(params)：发起请求并解构响应，返回 { list, total }（各页 res.data.xxx 的 key 不同，由调用方处理）
// - cursorKey：字符串表示取最后一条的该属性作为 last_id 游标；函数则返回完整游标参数对象（如 last_id + last_key）
// - getExtraParams：每次请求前动态拼接的额外参数（筛选条件等）
// - onError：请求失败提示（不传则错误向外抛出，与原「无 catch」页面行为一致）
// - dedupe：append 时按 id 去重兜底；部分页面原实现无去重，传 false 保持行为一致
// - initialLoading：首屏 loading 初始值（骨架屏页面为 true）
// 约定：
// - sentinel 需由调用方绑定为模板 ref；观察器在每次 fetch 的 finally 中自动重建
//   （重新 observe 会立即以当前相交状态回调一次），一般无需手动调 setupInfiniteScroll
// - total 在整页刷新（append=false）时由响应写入；触底加载期间一般不更新，
//   唯一例外是去重后整批重复数据的兜底分支（见 fetch 内注释），会把 total 拉平为
//   已加载数量终止分页——属可接受的边界，代价是「偶发全重复页」会提前停止分页
export function useInfiniteList({ fetcher, pageSize, cursorKey = 'id', getId = (x) => x.id, getExtraParams, onError, dedupe = true, initialLoading = false }) {
  const items = ref([])
  const total = ref(0)
  const loading = ref(initialLoading)
  const loadingMore = ref(false)
  const sentinel = ref(null)
  let observer = null

  async function fetch(append = false) {
    if (!append) {
      loading.value = true
    } else {
      loadingMore.value = true
    }
    try {
      const params = { ...(getExtraParams ? getExtraParams() : {}), page_size: pageSize }
      // 游标分页：加载更多时携带上一批最后一条的游标参数
      if (append && items.value.length > 0) {
        const last = items.value[items.value.length - 1]
        if (typeof cursorKey === 'function') {
          Object.assign(params, cursorKey(last))
        } else {
          params.last_id = last[cursorKey]
        }
      }
      const { list, total: fetchedTotal } = await fetcher(params)
      if (!append) total.value = fetchedTotal
      if (append && dedupe) {
        // 去重兜底：游标分页偶发重复时避免同一项显示多次
        const existingIds = new Set(items.value.map(getId))
        const fresh = list.filter(x => !existingIds.has(getId(x)))
        if (fresh.length === 0) {
          // 去重后无新数据：把 total 拉平为已加载数量，使哨兵条件 length < total 变为
          // 假而停止触底加载。必须直接赋值（不能用 Math.max）：若服务端 total 偏大且
          // 后续页全是重复数据，Math.max 永远收敛不了，会造成无限重复请求循环。
          total.value = items.value.length
        } else {
          items.value = [...items.value, ...fresh]
        }
      } else if (append) {
        items.value = [...items.value, ...list]
      } else {
        items.value = list
      }
    } catch (e) {
      if (onError) onError(e)
      else throw e
    } finally {
      loading.value = false
      loadingMore.value = false
      nextTick(setupInfiniteScroll)
    }
  }

  function loadMore() {
    fetch(true)
  }

  // 触底自动加载：sentinel 进入视口附近时加载下一页
  function setupInfiniteScroll() {
    if (observer) observer.disconnect()
    if (!sentinel.value) return
    observer = new IntersectionObserver((entries) => {
      if (
        entries[0].isIntersecting &&
        !loading.value &&
        !loadingMore.value &&
        items.value.length < total.value
      ) {
        loadMore()
      }
    }, { rootMargin: '200px 0px' })
    observer.observe(sentinel.value)
  }

  onBeforeUnmount(() => {
    if (observer) observer.disconnect()
    observer = null
  })

  return { items, total, loading, loadingMore, sentinel, fetch, loadMore, setupInfiniteScroll }
}
