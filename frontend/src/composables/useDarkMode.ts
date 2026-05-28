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
}

function applyMode() {
  if (themeMode.value === 'system') {
    applyDark(window.matchMedia('(prefers-color-scheme: dark)').matches)
  } else {
    applyDark(themeMode.value === 'dark')
  }
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
