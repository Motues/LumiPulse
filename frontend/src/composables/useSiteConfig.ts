import { ref, computed } from 'vue'
import { api } from '../api/client'
import { onLocaleChange, t } from './useI18n'

export const siteName = ref('LumiPulse')
export const siteIcon = ref('')
export const emailEnabled = ref(false)
export const showAdminButton = ref(true)
export const customFooter = ref('')
export const subEnableEmail = ref(false)
export const subEnableRss = ref(false)
export const subEnableAtom = ref(false)

export const subEnabledAny = computed(() =>
  subEnableEmail.value || subEnableRss.value || subEnableAtom.value
)

let loaded = false

/** 页面标题：`<站点名> - <系统状态>`，语言切换时同步 */
function pageTitle(): string {
  return `${siteName.value} - ${t('site.statusSuffix')}`
}

/** 分享预览描述，语言切换时同步 */
function shareDescription(): string {
  return t('site.description', { name: siteName.value })
}

export async function loadSiteConfig() {
  if (loaded) return
  try {
    const res = await api.getSiteConfig()
    if (res.data['site_name']) siteName.value = res.data['site_name']
    if (res.data['site_icon']) siteIcon.value = res.data['site_icon']
    emailEnabled.value = res.data['email_enabled'] === 'true'
    showAdminButton.value = res.data['show_admin_footer_button'] !== 'false'
    if (res.data['custom_footer']) customFooter.value = res.data['custom_footer']
    subEnableEmail.value = res.data['sub_enable_email'] === 'true'
    subEnableRss.value = res.data['sub_enable_rss'] === 'true'
    subEnableAtom.value = res.data['sub_enable_atom'] === 'true'

    // Update document title and favicon
    document.title = pageTitle()
    updateFavicon()
    // 分享卡片元信息同步为真实站点名（爬虫不执行 JS，仅对浏览器端分享预览生效）
    updateShareMeta()
  } catch {
    // use defaults
  } finally {
    loaded = true
  }
}

export function updateSiteTitle(title: string) {
  siteName.value = title
  document.title = pageTitle()
  updateShareMeta()
}

export function updateSiteIcon(dataUrl: string) {
  siteIcon.value = dataUrl
  updateFavicon()
}

function updateFavicon() {
  if (!siteIcon.value) return
  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'icon'
    document.head.appendChild(link)
  }
  link.href = siteIcon.value
}

// 语言切换后标题与分享卡片也要跟着切换
onLocaleChange(() => {
  document.title = pageTitle()
  updateShareMeta()
})

// updateShareMeta 把 index.html 中的静态兜底文案替换为真实站点名，
// 让浏览器内触发的分享预览与页面标题保持一致。
function updateShareMeta() {
  const title = pageTitle()
  const description = shareDescription()
  const setMeta = (selector: string, content: string) => {
    const el = document.querySelector<HTMLMetaElement>(selector)
    if (el) el.content = content
  }
  setMeta('meta[property="og:site_name"]', siteName.value)
  setMeta('meta[property="og:title"]', title)
  setMeta('meta[property="og:description"]', description)
  setMeta('meta[name="twitter:title"]', title)
  setMeta('meta[name="twitter:description"]', description)
  const desc = document.querySelector<HTMLMetaElement>('meta[name="description"]')
  if (desc) desc.content = description
}
