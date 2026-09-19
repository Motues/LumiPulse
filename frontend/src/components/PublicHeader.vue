<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { siteName, siteIcon, subEnabledAny } from '../composables/useSiteConfig'
import { useDarkMode } from '../composables/useDarkMode'
import { useI18n } from '../composables/useI18n'

/**
 * 公开页顶部导航（首页与事件详情页共用），含主题切换。
 * 两处原本各有一份重复实现，抽出来同时解决 i18n 与后续维护问题。
 * 语言切换只在页脚提供，顶部不再保留入口，避免同一功能出现两个入口。
 */
defineProps<{
  onSubscribe: () => void
}>()

const { themeMode, setMode } = useDarkMode()
const { t } = useI18n()

const root = ref<HTMLElement | null>(null)
const showThemeMenu = ref(false)

function toggleThemeMenu() {
  showThemeMenu.value = !showThemeMenu.value
}

function onDocumentClick(e: MouseEvent) {
  if (root.value && !root.value.contains(e.target as Node)) {
    showThemeMenu.value = false
  }
}

onMounted(() => document.addEventListener('click', onDocumentClick))
onUnmounted(() => document.removeEventListener('click', onDocumentClick))

function pickTheme(mode: 'light' | 'dark' | 'system') {
  setMode(mode)
  showThemeMenu.value = false
}
</script>

<template>
  <nav ref="root" class="mt-4" style="background-color: var(--bg-color);">
    <div class="max-w-[1000px] mx-auto px-6 h-16 flex items-center justify-between">
      <div class="flex items-center gap-2">
        <img v-if="siteIcon" :src="siteIcon" class="w-6 h-6 object-contain" />
        <img v-else src="/assets/logo.svg" class="w-6 h-6 object-contain" />
        <span class="text-xl font-bold tracking-tight" style="color: var(--text-color);">{{ siteName }}</span>
      </div>
      <div class="flex items-center gap-3">
        <a v-if="subEnabledAny" href="#" @click.prevent="onSubscribe" class="header-btn px-4 py-2 text-sm font-medium rounded-lg">{{ t('header.subscribe') }}</a>

        <!-- 主题切换 -->
        <div class="relative">
          <button
            @click.stop="toggleThemeMenu"
            class="header-btn px-2 py-2 rounded-lg"
            :title="t('header.switchTheme')"
          >
            <svg v-if="themeMode === 'light'" class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
            </svg>
            <svg v-else-if="themeMode === 'dark'" class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M20.985 12.486a9 9 0 1 1-9.473-9.472c.405-.022.617.46.402.803a6 6 0 0 0 8.268 8.268c.344-.215.825-.004.803.401" />
            </svg>
            <svg v-else class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 17.25v1.007a3 3 0 01-.879 2.122L7.5 21h9l-.621-.621A3 3 0 0115 18.257V17.25m6-12V15a2.25 2.25 0 01-2.25 2.25H5.25A2.25 2.25 0 013 15V5.25A2.25 2.25 0 015.25 3h13.5A2.25 2.25 0 0121 5.25z" />
            </svg>
          </button>
          <div
            v-if="showThemeMenu"
            class="absolute right-0 top-full mt-2 rounded-lg shadow-lg py-1.5 px-1.5 z-50 flex flex-col gap-0.5"
            :style="{ backgroundColor: 'var(--bg-color)', border: '1px solid var(--button-border-color)', width: '140px' }"
            @click.stop
          >
            <button
              @click="pickTheme('light')"
              class="w-full text-left px-3 py-2 text-sm flex items-center gap-2 transition-colors rounded-md"
              :style="{ color: 'var(--text-color)', backgroundColor: themeMode === 'light' ? 'var(--button-hover-color)' : 'transparent' }"
            >
              <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" /></svg>
              {{ t('header.theme.light') }}
            </button>
            <button
              @click="pickTheme('dark')"
              class="w-full text-left px-3 py-2 text-sm flex items-center gap-2 transition-colors rounded-md"
              :style="{ color: 'var(--text-color)', backgroundColor: themeMode === 'dark' ? 'var(--button-hover-color)' : 'transparent' }"
            >
              <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20.985 12.486a9 9 0 1 1-9.473-9.472c.405-.022.617.46.402.803a6 6 0 0 0 8.268 8.268c.344-.215.825-.004.803.401" /></svg>
              {{ t('header.theme.dark') }}
            </button>
            <button
              @click="pickTheme('system')"
              class="w-full text-left px-3 py-2 text-sm flex items-center gap-2 transition-colors rounded-md"
              :style="{ color: 'var(--text-color)', backgroundColor: themeMode === 'system' ? 'var(--button-hover-color)' : 'transparent' }"
            >
              <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M9 17.25v1.007a3 3 0 01-.879 2.122L7.5 21h9l-.621-.621A3 3 0 0115 18.257V17.25m6-12V15a2.25 2.25 0 01-2.25 2.25H5.25A2.25 2.25 0 013 15V5.25A2.25 2.25 0 015.25 3h13.5A2.25 2.25 0 0121 5.25z" /></svg>
              {{ t('header.theme.system') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </nav>
</template>
