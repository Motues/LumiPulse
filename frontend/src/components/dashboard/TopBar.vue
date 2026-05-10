<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '../../stores/auth'
import { api } from '../../api/client'

defineProps<{
  title: string
  collapsed: boolean
  isMobile?: boolean
}>()

const emit = defineEmits<{
  toggle: []
  navigate: [section: string]
}>()

const router = useRouter()
const { logout } = useAuth()

const username = ref('Admin')
const isHovering = ref(false)
const activeIncidentCount = ref(0)
let hideTimer: ReturnType<typeof setTimeout> | null = null

function onMouseEnter() {
  if (hideTimer) clearTimeout(hideTimer)
  isHovering.value = true
}

function onMouseLeave() {
  hideTimer = setTimeout(() => {
    isHovering.value = false
  }, 150)
}

onUnmounted(() => {
  if (hideTimer) clearTimeout(hideTimer)
})

function navigateTo(section: string) {
  emit('navigate', section)
}

function handleLogout() {
  logout()
  router.push('/login')
}

onMounted(async () => {
  try {
    const userRes = await api.getCurrentUser()
    username.value = userRes.data.username
  } catch {
    // silent
  }
  try {
    const statsRes = await api.getStats()
    activeIncidentCount.value = statsRes.data.activeIncidents
  } catch {
    // silent
  }
})
</script>

<template>
  <header class="h-16 bg-white dark:bg-gray-900 border-b border-gray-100 dark:border-gray-800 flex items-center justify-between px-6 flex-shrink-0 z-10">
    <div class="flex items-center gap-4">
      <button @click="emit('toggle')" class="text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200">
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
        </svg>
      </button>
      <span class="text-gray-900 dark:text-gray-100 font-medium" :class="isMobile ? 'hidden' : ''">{{ title }}</span>
    </div>
    <div class="flex items-center gap-5">
      <!-- Bell icon with incident badge -->
      <button @click="navigateTo('incidents')" class="relative text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 transition-colors">
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
        </svg>
        <span
          v-if="activeIncidentCount > 0"
          class="absolute -top-1 -right-1 min-w-[14px] h-[14px] flex items-center justify-center rounded-full bg-red-500 text-white text-[8px] font-bold leading-none"
        >
          {{ activeIncidentCount > 99 ? '99+' : activeIncidentCount }}
        </span>
      </button>

      <div class="h-6 w-px bg-gray-200 dark:bg-gray-700" />

      <!-- User avatar with dropdown -->
      <div @mouseenter="onMouseEnter" @mouseleave="onMouseLeave" class="relative">
        <button class="flex items-center gap-3 cursor-pointer">
          <div class="w-8 h-8 rounded-full bg-emerald-100 dark:bg-emerald-900/50 flex items-center justify-center text-emerald-600 dark:text-emerald-400 font-bold text-sm">
            <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
              <path fill-rule="evenodd" d="M10 9a3 3 0 100-6 3 3 0 000 6zm-7 9a7 7 0 1114 0H3z" clip-rule="evenodd" />
            </svg>
          </div>
          <div class="flex-col text-left" :class="isMobile ? 'hidden' : 'flex'">
            <span class="text-sm font-bold text-gray-900 dark:text-gray-100 leading-tight">{{ username }}</span>
            <span class="text-[10px] text-gray-500 dark:text-gray-400">管理员</span>
          </div>
        </button>

        <!-- Dropdown menu -->
        <div
          v-if="isHovering"
          class="absolute right-0 top-full mt-2 w-44 bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 rounded-lg shadow-xl py-1"
        >
          <button @click="navigateTo('users')" class="flex items-center gap-2 w-full px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors text-left">
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z" />
            </svg>
            用户管理
          </button>
          <hr class="border-gray-50 dark:border-gray-700 my-1" />
          <button @click="handleLogout" class="flex items-center gap-2 w-full px-4 py-2 text-sm text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/30 transition-colors text-left">
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
            </svg>
            退出登录
          </button>
        </div>
      </div>
    </div>
  </header>
</template>
