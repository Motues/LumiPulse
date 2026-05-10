<template>
  <aside class="flex w-60 flex-col bg-slate-800 text-white">
    <!-- Logo -->
    <div class="flex h-16 items-center gap-2 border-b border-slate-700 px-6">
      <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-indigo-500 text-sm font-bold overflow-hidden">
        <img v-if="siteIcon" :src="siteIcon" class="w-8 h-8 object-contain" />
        <span v-else>LP</span>
      </div>
      <span class="text-lg font-semibold tracking-wide">{{ siteName }}</span>
    </div>

    <!-- Navigation -->
    <nav class="flex-1 space-y-1 px-3 py-4">
      <router-link
        v-for="item in menuItems"
        :key="item.route"
        :to="item.route"
        class="flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors"
        :class="isActive(item.route)
          ? 'bg-indigo-500/20 text-indigo-300'
          : 'text-slate-300 hover:bg-slate-700/50 hover:text-white'"
      >
        <span class="text-lg">{{ item.icon }}</span>
        <span>{{ item.label }}</span>
      </router-link>
    </nav>

    <!-- Footer -->
    <div class="border-t border-slate-700 px-4 py-3">
      <p class="text-center text-xs text-slate-500">{{ siteName }} v0.1.0</p>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { useRoute } from 'vue-router'
import type { MenuItem } from '@/types'
import { siteName, siteIcon } from '../composables/useSiteConfig'

const route = useRoute()

const menuItems: MenuItem[] = [
  { label: '仪表盘', icon: '📊', route: '/dashboard' },
]

function isActive(path: string): boolean {
  return route.path.startsWith(path)
}
</script>
