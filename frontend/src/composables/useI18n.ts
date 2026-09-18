import { ref, computed, watch } from 'vue'

/**
 * 轻量 i18n：
 * - 不引入 vue-i18n，`t()` 依赖响应式的 `locale`，语言切换会触发模板重渲染；
 * - 文案按 `{locale: {key: text}}` 组织，缺 key 时回退到 zh-CN，再回退到 key 本身；
 * - `t(key, {n: 3})` 支持 `{n}` 形式的插值。
 */

export type Locale = 'zh-CN' | 'en-US'

export const SUPPORTED_LOCALES: Locale[] = ['zh-CN', 'en-US']

const STORAGE_KEY = 'lumipulse-locale'
const FALLBACK: Locale = 'zh-CN'

export const locale = ref<Locale>(FALLBACK)

const messages: Record<Locale, Record<string, string>> = {
  'zh-CN': {},
  'en-US': {},
}

/** registerMessages 由 locales/*.ts 在模块加载时调用，避免这里反向依赖具体文案文件 */
export function registerMessages(loc: Locale, dict: Record<string, string>) {
  Object.assign(messages[loc], dict)
}

/** normalizeLocale 把浏览器语言（如 zh-Hans-CN、en-GB）收敛到支持的 locale */
export function normalizeLocale(raw: string | null | undefined): Locale | null {
  if (!raw) return null
  const lower = raw.toLowerCase()
  if (lower.startsWith('zh')) return 'zh-CN'
  if (lower.startsWith('en')) return 'en-US'
  return null
}

function detectLocale(): Locale {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    const normalized = normalizeLocale(saved)
    if (normalized) return normalized
  } catch {
    // localStorage 不可用（隐私模式等）时退回浏览器语言
  }
  return normalizeLocale(navigator.language) || FALLBACK
}

/** 语言变化时的统一的副作用挂载点（标题、html lang 等） */
const localeHooks: ((loc: Locale) => void)[] = []

export function onLocaleChange(fn: (loc: Locale) => void) {
  localeHooks.push(fn)
}

export function setLocale(loc: Locale) {
  if (!SUPPORTED_LOCALES.includes(loc)) return
  locale.value = loc
  try {
    localStorage.setItem(STORAGE_KEY, loc)
  } catch {
    // 忽略写入失败，当前会话仍然生效
  }
}

export function initLocale() {
  locale.value = detectLocale()
}

export function t(key: string, params?: Record<string, string | number>): string {
  const dict = messages[locale.value] || {}
  let text = dict[key]
  if (text === undefined) text = messages[FALLBACK][key]
  if (text === undefined) return key
  if (!params) return text
  return text.replace(/\{(\w+)\}/g, (match, name) =>
    params[name] !== undefined ? String(params[name]) : match,
  )
}

/** 是否中文环境，用于日期格式与「CST」等时区标注 */
export const isChinese = computed(() => locale.value === 'zh-CN')

const dateTimeOpts: Intl.DateTimeFormatOptions = { timeZone: 'Asia/Shanghai' }

function intlLocale(): string {
  return locale.value
}

/** 上海时区的日期，例如「2026年5月20日」/「May 20, 2026」 */
export function formatDate(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleDateString(intlLocale(), {
    ...dateTimeOpts,
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  })
}

/** 上海时区的时间，例如「14:05」 */
export function formatTime(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleTimeString(intlLocale(), {
    ...dateTimeOpts,
    hour: '2-digit',
    minute: '2-digit',
  })
}

export function formatDateTime(iso: string): string {
  if (!iso) return ''
  return `${formatDate(iso)} ${formatTime(iso)}`
}

/** 矩阵格子的日期，例如「2026年5月20日」/「May 20」 */
export function formatMatrixDate(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleDateString(intlLocale(), {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  })
}

/** 时长文案，例如「1小时20分钟」/「1h 20m」 */
export function formatDuration(minutes: number): string {
  if (minutes < 60) return t('duration.minutes', { n: minutes })
  const h = Math.floor(minutes / 60)
  const m = minutes % 60
  return m > 0 ? t('duration.hoursMinutes', { h, m }) : t('duration.hoursOnly', { h })
}

// 语言切换后同步 <html lang>，便于浏览器与爬虫识别
watch(locale, loc => {
  document.documentElement.setAttribute('lang', loc)
  localeHooks.forEach(fn => fn(loc))
}, { immediate: true })

/** 供组件使用的组合式函数 */
export function useI18n() {
  return {
    locale,
    isChinese,
    t,
    setLocale,
    formatDate,
    formatTime,
    formatDateTime,
    formatMatrixDate,
    formatDuration,
  }
}
