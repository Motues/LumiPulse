/**
 * 公开首页「服务详情」内容块开关。
 *
 * 管理后台可以为每个服务单独选择首页要展示哪些内容块（探测频率、证书到期、
 * 延迟曲线、热力图、历史矩阵等），管理后台自身始终展示全部内容。
 *
 * 存储语义（与后端 model.HomepageBlock* 保持一致）：
 * - 空串 / 未配置 → 全部展示（历史数据的默认值，保证老数据不会被藏起来）
 * - `none` → 全部不展示（不能复用空串，否则与「全部展示」冲突）
 * - 其余情况 → 只展示列出的 key
 */

/** 全部可配置的内容块，顺序即管理端开关的展示顺序 */
export const HOMEPAGE_BLOCKS = [
  'metrics',
  'interval',
  'cert',
  'latency',
  'heatmap',
  'history',
] as const

export type HomepageBlock = (typeof HOMEPAGE_BLOCKS)[number]

/** 「全部不展示」的显式标记 */
export const HOMEPAGE_BLOCKS_NONE = 'none'

/** 解析服务上的内容块配置，返回实际要展示的 key 集合 */
export function parseHomepageBlocks(raw?: string | null): Set<string> {
  const value = (raw || '').trim()
  if (value === '') return new Set(HOMEPAGE_BLOCKS)
  if (value === HOMEPAGE_BLOCKS_NONE) return new Set()
  const enabled = new Set<string>()
  for (const part of value.split(',')) {
    const key = part.trim().toLowerCase()
    if ((HOMEPAGE_BLOCKS as readonly string[]).includes(key)) enabled.add(key)
  }
  // 配置里没有任何已知 key（脏数据）时按全部展示处理，避免整页空白
  return enabled.size > 0 ? enabled : new Set(HOMEPAGE_BLOCKS)
}

/** 判断某个内容块是否应该在公开首页展示 */
export function isHomepageBlockEnabled(raw: string | null | undefined, block: HomepageBlock): boolean {
  return parseHomepageBlocks(raw).has(block)
}

/** 把内容块集合序列化为提交给后端的值 */
export function serializeHomepageBlocks(blocks: Iterable<string>): string {
  const enabled = new Set(blocks)
  const keys = HOMEPAGE_BLOCKS.filter(key => enabled.has(key))
  return keys.length > 0 ? keys.join(',') : HOMEPAGE_BLOCKS_NONE
}
