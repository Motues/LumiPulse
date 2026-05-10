import { ref } from 'vue'
import { api } from '../api/client'

export const siteName = ref('LumiPulse')
export const siteIcon = ref('')
export const emailEnabled = ref(false)

let loaded = false

export async function loadSiteConfig() {
  if (loaded) return
  try {
    const res = await api.getSiteConfig()
    if (res.data['site_name']) siteName.value = res.data['site_name']
    if (res.data['site_icon']) siteIcon.value = res.data['site_icon']
    emailEnabled.value = res.data['email_enabled'] === 'true'

    // Update document title and favicon
    document.title = siteName.value + ' - 系统状态'
    updateFavicon()
  } catch {
    // use defaults
  } finally {
    loaded = true
  }
}

export function updateSiteTitle(title: string) {
  siteName.value = title
  document.title = title + ' - 系统状态'
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
