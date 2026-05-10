<script setup lang="ts">
import { ref } from 'vue'

const props = defineProps<{
  days: [number, number, number][]
  uptime: number
  hideLegend?: boolean
  compact?: boolean
}>()

const GREEN = { r: 69, g: 186, b: 101 }
const YELLOW = { r: 249, g: 172, b: 5 }
const RED = { r: 223, g: 45, b: 42 }

function lerpColor(a: typeof GREEN, b: typeof GREEN, t: number): string {
  const r = Math.round(a.r + (b.r - a.r) * t)
  const g = Math.round(a.g + (b.g - a.g) * t)
  const bl = Math.round(a.b + (b.b - a.b) * t)
  return `#${r.toString(16).padStart(2, '0')}${g.toString(16).padStart(2, '0')}${bl.toString(16).padStart(2, '0')}`
}

function getColorForDowntime(down: number): string {
  if (down === -1) return 'var(--matrix-no-data, #e5e7eb)'
  if (down === 0) return `#${GREEN.r.toString(16).padStart(2, '0')}${GREEN.g.toString(16).padStart(2, '0')}${GREEN.b.toString(16).padStart(2, '0')}`
  if (down < 30) return lerpColor(GREEN, YELLOW, down / 30)
  if (down < 60) return lerpColor(YELLOW, RED, (down - 30) / 30)
  return `#${RED.r.toString(16).padStart(2, '0')}${RED.g.toString(16).padStart(2, '0')}${RED.b.toString(16).padStart(2, '0')}`
}

const statusLabel: Record<number, string> = {
  1: '调查中',
  2: '已确认',
  3: '监控中',
  4: '已解决',
}

const hoveredIndex = ref(-1)
const tooltipX = ref(0)
const tooltipY = ref(0)

function formatDate(index: number, total: number): string {
  // Use CST (UTC+8) for date calculation
  const now = new Date()
  const cst = new Date(now.getTime() + (now.getTimezoneOffset() + 480) * 60000)
  cst.setUTCDate(cst.getUTCDate() - (total - 1 - index))
  return `${cst.getUTCFullYear()}年${cst.getUTCMonth() + 1}月${cst.getUTCDate()}日`
}

function onCellEnter(e: MouseEvent, i: number) {
  hoveredIndex.value = i
  const target = e.currentTarget as HTMLElement
  if (target) {
    const cellRect = target.getBoundingClientRect()
    tooltipX.value = cellRect.left + cellRect.width / 2
    tooltipY.value = (target.parentElement?.getBoundingClientRect().top ?? cellRect.top) - 8
  }
}

function onCellLeave() {
  hoveredIndex.value = -1
}

function timeText(minutes: number): string {
  if (minutes < 60) return `${minutes}分钟`
  const h = Math.floor(minutes / 60)
  const m = minutes % 60
  return m > 0 ? `${h}小时${m}分钟` : `${h}小时`
}
</script>

<template>
  <div class="relative">
    <div :class="['flex gap-[2px] w-full', compact ? 'h-5' : 'h-8']">
      <div
        v-for="(pair, i) in days"
        :key="i"
        :style="{ backgroundColor: getColorForDowntime(pair[1]) }"
        class="flex-1 h-full rounded-[1px] cursor-pointer transition-all duration-100 hover:brightness-110 hover:scale-y-110"
        @mouseenter="onCellEnter($event, i)"
        @mouseleave="onCellLeave"
      />
    </div>

    <!-- Tooltip -->
    <div
      v-if="hoveredIndex >= 0"
      class="fixed z-50 bg-white dark:bg-gray-800 rounded-lg shadow-lg border border-gray-100 dark:border-gray-700 px-4 py-3 pointer-events-none"
      :style="{ left: tooltipX + 'px', top: tooltipY + 'px', transform: 'translateX(-50%) translateY(-100%)', width: '220px' }"
    >
      <!-- Arrow -->
      <div class="absolute left-1/2 -bottom-2 -translate-x-1/2 w-0 h-0 border-l-8 border-r-8 border-t-8 border-transparent border-t-white dark:border-t-gray-800" />

      <div class="text-sm font-bold text-gray-900 dark:text-gray-100 mb-2">
        {{ formatDate(hoveredIndex, days.length) }}
      </div>

      <template v-if="days[hoveredIndex]">
        <template v-if="days[hoveredIndex][0] === -1">
          <div class="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
            <span class="w-2.5 h-2.5 rounded-full inline-block flex-shrink-0" :style="{ backgroundColor: getColorForDowntime(-1) }" />
            无数据
          </div>
        </template>
        <template v-else-if="days[hoveredIndex][1] === 0">
          <div class="flex items-center gap-2 text-sm" :style="{ color: getColorForDowntime(0) }">
            <span class="w-2.5 h-2.5 rounded-full inline-block flex-shrink-0" :style="{ backgroundColor: getColorForDowntime(0) }" />
            服务运行正常
          </div>
        </template>
        <template v-else>
          <div class="flex items-center gap-2 text-sm font-medium" :style="{ color: getColorForDowntime(days[hoveredIndex][1]) }">
            <span class="w-2.5 h-2.5 rounded-full inline-block flex-shrink-0" :style="{ backgroundColor: getColorForDowntime(days[hoveredIndex][1]) }" />
            服务异常 {{ timeText(days[hoveredIndex][1]) }}
          </div>
          <div class="text-xs text-gray-400 dark:text-gray-500 mt-1 ml-4.5">
            当前状态：{{ statusLabel[days[hoveredIndex][2]] || '已解决' }}
          </div>
        </template>
      </template>
    </div>

    <!-- Bottom legend -->
    <div v-if="!hideLegend" class="flex items-center text-xs font-medium mt-1 tracking-tight">
      <span class="flex-shrink-0 text-gray-400 dark:text-gray-500">{{ days.length }} 天前</span>
      <span class="flex-1 mx-2 h-px bg-gray-300 dark:bg-gray-700" />
      <span class="font-semibold flex-shrink-0" :class="uptime >= 99.9 ? 'text-emerald-600 dark:text-emerald-400' : uptime >= 95 ? 'text-gray-500 dark:text-gray-400' : 'text-red-500 dark:text-red-400'">{{ uptime.toFixed(1) }}% 在线率</span>
      <span class="flex-1 mx-2 h-px bg-gray-300 dark:bg-gray-700" />
      <span class="flex-shrink-0 text-gray-400 dark:text-gray-500">今天</span>
    </div>
  </div>
</template>
