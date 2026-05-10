<template>
  <header class="flex h-16 items-center justify-between border-b border-gray-200 bg-white px-6">
    <!-- Left: page title -->
    <h1 class="text-lg font-semibold text-gray-800">{{ pageTitle }}</h1>

    <!-- Right: user info -->
    <div class="flex items-center gap-4">
      <button class="relative rounded-lg p-2 text-gray-400 hover:bg-gray-100 hover:text-gray-600 transition-colors">
        <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
            d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
        </svg>
        <span class="absolute right-1.5 top-1.5 h-2 w-2 rounded-full bg-red-500"></span>
      </button>

      <div class="flex items-center gap-3">
        <div class="flex h-8 w-8 items-center justify-center rounded-full bg-indigo-100 text-sm font-medium text-indigo-600">
          {{ userInitials }}
        </div>
        <div class="text-sm">
          <p class="font-medium text-gray-700">管理员</p>
          <p class="text-xs text-gray-400">admin</p>
        </div>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()

const pageTitle = computed(() => {
  const map: Record<string, string> = {
    'Dashboard': '仪表盘',
  }
  const name = route.name as string
  return map[name] || name
})

const userInitials = computed(() => '管')
</script>
