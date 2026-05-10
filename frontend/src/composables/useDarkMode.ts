import { ref, watch } from 'vue'

const STORAGE_KEY = 'lumipulse-dark-mode'

const isDark = ref(false)

function applyDark(val: boolean) {
  isDark.value = val
  if (val) {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
}

function init() {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved !== null) {
    applyDark(saved === 'true')
  } else {
    applyDark(window.matchMedia('(prefers-color-scheme: dark)').matches)
  }
}

function toggle() {
  applyDark(!isDark.value)
  localStorage.setItem(STORAGE_KEY, String(isDark.value))
}

export function useDarkMode() {
  return { isDark, init, toggle }
}
