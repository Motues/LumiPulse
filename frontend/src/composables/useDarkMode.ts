import { ref } from 'vue'

const STORAGE_KEY = 'lumipulse-theme-mode'

const isDark = ref(false)
const themeMode = ref<'system' | 'light' | 'dark'>('system')

let mediaQuery: MediaQueryList | null = null

function applyDark(val: boolean) {
  isDark.value = val
  if (val) {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
  applyThemeColor(val)
}

function applyMode() {
  if (themeMode.value === 'system') {
    applyDark(window.matchMedia('(prefers-color-scheme: dark)').matches)
  } else {
    applyDark(themeMode.value === 'dark')
  }
}

// 浏览器 UI 主题色。index.html 里已有一份随系统偏好切换的声明，
// 这里按用户显式选择覆盖它（media 需去掉，否则会与显式选择打架）。
const THEME_COLORS = { light: '#ffffff', dark: '#0a0a0a' } as const

function applyThemeColor(val: boolean) {
  document
    .querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]')
    .forEach(el => el.remove())
  const meta = document.createElement('meta')
  meta.name = 'theme-color'
  meta.content = THEME_COLORS[val ? 'dark' : 'light']
  document.head.appendChild(meta)
}

function onSystemChange(e: MediaQueryListEvent) {
  if (themeMode.value === 'system') {
    applyDark(e.matches)
  }
}

function init() {
  const saved = localStorage.getItem(STORAGE_KEY)

  // Migrate old boolean values
  if (saved === 'true') {
    themeMode.value = 'dark'
  } else if (saved === 'false') {
    themeMode.value = 'light'
  } else if (saved === 'system' || saved === 'light' || saved === 'dark') {
    themeMode.value = saved
  } else {
    themeMode.value = 'system'
  }

  // Listen for system preference changes
  mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
  mediaQuery.addEventListener('change', onSystemChange)

  applyMode()
}

function setMode(mode: 'system' | 'light' | 'dark') {
  themeMode.value = mode
  localStorage.setItem(STORAGE_KEY, mode)
  applyMode()
}

function toggle() {
  // Cycle: light -> dark -> system -> light
  if (themeMode.value === 'light') setMode('dark')
  else if (themeMode.value === 'dark') setMode('system')
  else setMode('light')
}

export function useDarkMode() {
  return { isDark, themeMode, init, toggle, setMode }
}
