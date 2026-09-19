<script setup lang="ts">
import type { SectionGroup } from '../../types'

defineProps<{
  active: string
  groups: SectionGroup[]
}>()

const emit = defineEmits<{
  select: [id: string]
}>()
</script>

<template>
  <nav class="flex flex-col gap-4 md:w-56 flex-shrink-0">
    <div v-for="(group, gi) in groups" :key="gi">
      <div
        v-if="group.title"
        class="text-[10px] font-semibold uppercase tracking-wider px-3 mb-1"
        style="color: var(--text-color); opacity: 0.4;"
      >
        {{ group.title }}
      </div>
      <div class="space-y-1">
        <button
          v-for="item in group.items"
          :key="item.id"
          type="button"
          @click="emit('select', item.id)"
          class="section-nav-item w-full flex items-center px-3 py-2 rounded-lg text-sm font-medium text-left transition-colors"
          :class="active === item.id ? 'bg-emerald-50 dark:bg-emerald-500/15 text-emerald-600 dark:text-emerald-400' : ''"
          :style="active !== item.id ? { color: 'var(--text-color)' } : {}"
        >
          <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" :d="item.icon" />
          </svg>
          <span class="ml-2.5 truncate">{{ item.label }}</span>
        </button>
      </div>
    </div>
  </nav>
</template>

<style scoped>
.section-nav-item:hover {
  background-color: var(--button-hover-color);
}
</style>
