<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '../../api/client'
import { version } from '../../../package.json'
import { siteName, siteIcon } from '../../composables/useSiteConfig'
import { useDarkMode } from '../../composables/useDarkMode'

const { isDark, toggle: toggleDark } = useDarkMode()

const props = defineProps<{
  collapsed: boolean
  activeSection: string
  navGroups: { title: string; items: { id: string; label: string; icon: string }[] }[]
  mobileOpen?: boolean
}>()

const emit = defineEmits<{
  toggle: []
  select: [section: string]
}>()

const isEffectivelyCollapsed = computed(() => props.mobileOpen ? false : props.collapsed)

const hoveredItem = ref<{ label: string; id: string } | null>(null)
const tooltipX = ref(0)
const tooltipY = ref(0)
let hoverTimer: ReturnType<typeof setTimeout> | null = null
const activeIncidentCount = ref(0)

onMounted(async () => {
  try {
    const res = await api.getStats()
    activeIncidentCount.value = res.data.activeIncidents
  } catch {
    // silent
  }
})

function onItemEnter(item: { label: string; id: string }, e: MouseEvent) {
  if (hoverTimer) clearTimeout(hoverTimer)
  const el = e.currentTarget as HTMLElement
  hoverTimer = setTimeout(() => {
    hoveredItem.value = item
    const rect = el.getBoundingClientRect()
    tooltipX.value = rect.right + 8
    tooltipY.value = rect.top + rect.height / 2
  }, 200)
}

function onItemLeave(e: MouseEvent) {
  if (hoverTimer) clearTimeout(hoverTimer)
  // Delay hiding to allow moving to tooltip
  hoverTimer = setTimeout(() => {
    hoveredItem.value = null
  }, 100)
}
</script>

<template>
  <aside
    :class="[
      'bg-white dark:bg-gray-900 border-r border-gray-100 dark:border-gray-800 flex flex-col flex-shrink-0 transition-all duration-200',
      mobileOpen !== undefined ? 'w-64 z-40' : (isEffectivelyCollapsed ? 'w-16 z-20' : 'w-64 z-20'),
      mobileOpen !== undefined
        ? 'fixed left-0 top-0 h-screen'
        : 'relative',
      mobileOpen !== undefined && !mobileOpen ? '-translate-x-full' : 'translate-x-0',
    ]"
  >
    <div class="h-16 flex items-center border-b border-gray-50 dark:border-gray-800" :class="isEffectivelyCollapsed ? 'justify-center px-2' : 'px-6'">
      <img v-if="siteIcon" :src="siteIcon" class="w-7 h-7 flex-shrink-0 object-contain" />
      <img v-else src="/assets/logo.svg" class="w-7 h-7 flex-shrink-0 object-contain" />
      <span
        class="text-xl font-bold tracking-tight text-gray-900 dark:text-gray-100 overflow-hidden whitespace-nowrap transition-all duration-200"
        :class="isEffectivelyCollapsed ? 'max-w-0 opacity-0 ml-0' : 'max-w-[200px] opacity-100 ml-2'"
      >{{ siteName }}</span>
    </div>

    <div class="flex-1 overflow-y-auto no-scrollbar p-3 space-y-4">
      <template v-for="(group, gi) in navGroups" :key="gi">
        <!-- Divider between groups -->
        <hr v-if="gi > 0" class="border-gray-100 dark:border-gray-800 transition-all duration-200" :class="isEffectivelyCollapsed ? 'opacity-0' : 'opacity-100'" />

        <!-- Section title -->
        <div
          v-if="group.title"
          class="text-[10px] font-semibold text-gray-400 dark:text-gray-500 uppercase tracking-wider px-3 overflow-hidden whitespace-nowrap transition-all duration-200"
          :class="isEffectivelyCollapsed ? 'max-h-0 py-0 opacity-0' : 'max-h-8 py-2 opacity-100'"
        >
          {{ group.title }}
        </div>

        <!-- Group items -->
        <div class="space-y-1">
          <div
            v-for="item in group.items" :key="item.id"
            @click="emit('select', item.id)"
            @mouseenter="onItemEnter(item, $event)"
            @mouseleave="onItemLeave"
            class="flex items-center px-3 py-2 rounded-lg text-sm font-medium transition-colors cursor-pointer"
            :class="activeSection === item.id ? 'bg-emerald-50 dark:bg-emerald-900/30 text-emerald-600 dark:text-emerald-400' : 'text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-800'"
          >
            <svg class="w-5 h-5 flex-shrink-0" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" :d="item.icon" />
            </svg>
            <span
              class="overflow-hidden whitespace-nowrap transition-all duration-200"
              :class="isEffectivelyCollapsed ? 'max-w-0 opacity-0 ml-0' : 'max-w-40 opacity-100 ml-3'"
            >{{ item.label }}</span>
            <span
              v-if="item.id === 'incidents' && activeIncidentCount > 0"
              class="ml-auto min-w-[18px] h-[18px] flex items-center justify-center px-1 rounded-full bg-red-500 text-white text-[10px] font-bold leading-none"
              :class="isEffectivelyCollapsed ? 'hidden' : ''"
            >
              {{ activeIncidentCount > 99 ? '99+' : activeIncidentCount }}
            </span>
          </div>
        </div>
      </template>
    </div>

    <div class="p-4 border-t border-gray-100 dark:border-gray-800" :class="isEffectivelyCollapsed ? 'text-center' : ''">
      <div
        class="text-xs text-gray-400 dark:text-gray-500 overflow-hidden whitespace-nowrap transition-all duration-200"
        :class="isEffectivelyCollapsed ? 'max-w-0 opacity-0' : 'max-w-48 opacity-100'"
      >
        <div class="mb-1">版本 v{{ version }}</div>
        <div class="mb-2">
          <a
            href="https://github.com/Motues/LumiPulse"
            target="_blank"
            rel="noopener noreferrer"
            class="hover:text-emerald-500 dark:hover:text-emerald-400 transition-colors"
          >Powered By {{ siteName }}</a>
        </div>
        <button
          @click="toggleDark"
          class="flex items-center gap-1.5 text-gray-400 dark:text-gray-500 hover:text-emerald-500 dark:hover:text-emerald-400 transition-colors"
        >
          <svg v-if="isDark" class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
          </svg>
          <svg v-else class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
          </svg>
          {{ isDark ? '浅色模式' : '深色模式' }}
        </button>
      </div>
    </div>

    <!-- Tooltip for collapsed mode -->
    <div
      v-if="isEffectivelyCollapsed && hoveredItem"
      class="fixed z-50 pointer-events-none flex items-center"
      :style="{ left: tooltipX + 'px', top: tooltipY + 'px', transform: 'translateY(-50%)' }"
    >
      <div class="w-2 h-2 bg-white dark:bg-gray-800 rotate-45 -mr-[3px] border-l border-b border-gray-200 dark:border-gray-700" />
      <div class="bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-200 text-xs font-medium px-3 py-1.5 rounded-md whitespace-nowrap shadow-lg border border-gray-200 dark:border-gray-700">
        {{ hoveredItem.label }}
      </div>
    </div>
  </aside>
</template>
