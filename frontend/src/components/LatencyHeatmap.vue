<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { api } from '../api/client'
import type { LatencyHeatmapCell } from '../api/types'
import { useI18n } from '../composables/useI18n'

/**
 * 响应时间热力图：行 = 日期，列 = 小时，颜色越红表示该小时的平均响应越慢。
 *
 * 数据来自后端按「日期 × 小时」聚合的接口（SQL 内完成分组），
 * 公开状态页与管理端服务详情共用本组件。
 * 只用于暴露「每天固定时段的性能劣化」这类规律，因此不做逐点曲线。
 */
const props = withDefaults(defineProps<{
  /** 服务的 publicHash（公开接口按 hash 定位） */
  serviceHash: string
  /** 展示天数，默认 7 天 */
  days?: number
}>(), {
  days: 7,
})

const { t, locale } = useI18n()

const loading = ref(true)
const error = ref('')
const from = ref('')
const to = ref('')
const maxAvg = ref(0)
const cells = ref<LatencyHeatmapCell[]>([])

/** 行高与间距：tooltip 的定位依赖这两个值，保持一致 */
const ROW_H = 22
const GAP = 3

const HOURS = Array.from({ length: 24 }, (_, i) => i)

/** 稀疏格子 → Map，键为 `YYYY-MM-DD|H` */
const cellMap = computed(() => {
  const map = new Map<string, LatencyHeatmapCell>()
  for (const c of cells.value) map.set(`${c.day}|${c.hour}`, c)
  return map
})

/** 行：从 from 到 to 的每一天（YYYY-MM-DD） */
const rows = computed(() => {
  if (!from.value || !to.value) return [] as string[]
  const list: string[] = []
  const start = new Date(`${from.value}T00:00:00+08:00`)
  const end = new Date(`${to.value}T00:00:00+08:00`)
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) return list
  for (let d = start.getTime(); d <= end.getTime(); d += 86400000) {
    // 统一按北京时间的日期字符串推进，避免本地时区把日期挤到前一天
    list.push(new Date(d + 8 * 3600 * 1000).toISOString().slice(0, 10))
  }
  return list
})

/** 色阶上限：优先后端给的 maxAvg，兜底用前端自己算的值 */
const scaleMax = computed(() => {
  if (maxAvg.value > 0) return maxAvg.value
  return cells.value.reduce((m, c) => Math.max(m, c.avg), 0)
})

const scaleMin = computed(() => {
  const values = cells.value.filter(c => c.samples > 0).map(c => c.avg)
  return values.length ? Math.min(...values) : 0
})

/** 平均延迟 → 颜色：绿（快）→ 黄 → 橙 → 红（慢） */
function cellColor(avg: number): string {
  const max = scaleMax.value
  if (max <= 0) return 'hsl(145, 55%, 45%)'
  const ratio = Math.max(0, Math.min(1, avg / max))
  const hue = 145 - 145 * ratio
  return `hsl(${hue.toFixed(0)}, 68%, 45%)`
}

function rowLabel(day: string): string {
  return day.slice(5)
}

function rowWeekday(day: string): string {
  const d = new Date(`${day}T12:00:00+08:00`)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleDateString(locale.value, { weekday: 'short', timeZone: 'Asia/Shanghai' })
}

function isEmptyCell(c: LatencyHeatmapCell | undefined): boolean {
  return !c || (c.samples === 0 && c.failures === 0)
}

function isAllFailed(c: LatencyHeatmapCell | undefined): boolean {
  return !!c && c.samples === 0 && c.failures > 0
}

function cellStyle(c: LatencyHeatmapCell | undefined): Record<string, string> {
  if (isAllFailed(c)) {
    // 全部失败：用斜纹区分「慢」和「挂」，颜色不参与色阶
    return {
      backgroundImage: 'repeating-linear-gradient(45deg, #df2d2a 0 4px, #b91c1c 4px 8px)',
    }
  }
  if (isEmptyCell(c)) return {}
  return { backgroundColor: cellColor(c!.avg) }
}

// ---- 悬停提示 ----
const hover = ref<{ row: number; hour: number } | null>(null)

const hoverCell = computed(() => {
  if (!hover.value) return null
  const day = rows.value[hover.value.row]
  if (!day) return null
  return {
    day,
    hour: hover.value.hour,
    cell: cellMap.value.get(`${day}|${hover.value.hour}`),
  }
})

function onCellEnter(row: number, hour: number) {
  hover.value = { row, hour }
}

function tooltipStyle(): Record<string, string> {
  if (!hover.value) return {}
  const { row, hour } = hover.value
  // 用行号/列号直接算位置，不依赖 DOM 测量；父容器只包住日期行，
  // 因此 top 可以直接按行高计算（刻度行在容器之外）
  const left = ((hour + 0.5) / 24) * 100
  // 靠近右边缘时把提示框往左收，避免溢出容器
  const translateX = hour >= 20 ? '-92%' : hour <= 3 ? '-8%' : '-50%'
  if (row === 0) {
    // 第一行上方没有空间，改到格子下方
    return {
      left: `${left}%`,
      top: `${ROW_H + 4}px`,
      transform: `translate(${translateX}, 0)`,
    }
  }
  return {
    left: `${left}%`,
    top: `${row * (ROW_H + GAP)}px`,
    transform: `translate(${translateX}, -100%)`,
  }
}

async function load() {
  if (!props.serviceHash) {
    cells.value = []
    loading.value = false
    return
  }
  loading.value = true
  error.value = ''
  try {
    const res = await api.getServiceLatencyHeatmap(props.serviceHash, props.days)
    from.value = res.data.from
    to.value = res.data.to
    maxAvg.value = res.data.maxAvg
    cells.value = res.data.cells || []
  } catch (e: any) {
    error.value = e.message || t('common.loadFailed')
    cells.value = []
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch(() => [props.serviceHash, props.days], load)
</script>

<template>
  <div>
    <div class="flex flex-wrap items-center justify-between gap-2 mb-3">
      <h3 class="font-bold" style="color: var(--text-color);">{{ t('service.heatmap') }}</h3>
      <div class="flex items-center gap-2 text-xs" style="color: var(--text-color); opacity: 0.5;">
        <span>{{ t('service.heatmapFast') }}</span>
        <span
          class="inline-block h-2.5 rounded-full"
          style="width: 84px; background: linear-gradient(to right, hsl(145,68%,45%), hsl(87,68%,45%), hsl(44,68%,45%), hsl(0,68%,45%));"
        />
        <span>{{ t('service.heatmapSlow') }}</span>
        <span v-if="scaleMax > 0">({{ scaleMin }}~{{ scaleMax }}ms)</span>
      </div>
    </div>
    <p class="text-xs mb-3" style="color: var(--text-color); opacity: 0.45;">
      {{ t('service.heatmapHint') }}
    </p>

    <div v-if="loading" class="text-center py-10 text-sm" style="color: var(--text-color); opacity: 0.4;">
      {{ t('common.loading') }}
    </div>
    <div v-else-if="error" class="text-center py-10 text-sm text-red-500">{{ error }}</div>
    <div v-else-if="cells.length === 0" class="text-center py-10 text-sm" style="color: var(--text-color); opacity: 0.4;">
      {{ t('service.noData') }}
    </div>

    <div v-else @mouseleave="hover = null">
      <!-- 小时刻度 -->
      <div class="flex items-center gap-3 mb-1">
        <span class="flex-shrink-0" style="width: 64px" />
        <div class="flex-1 grid" :style="{ gridTemplateColumns: 'repeat(24, minmax(0, 1fr))', gap: `${GAP}px` }">
          <span
            v-for="h in HOURS"
            :key="'h' + h"
            class="text-center"
            style="font-size: 9px; color: var(--text-color); opacity: 0.4;"
          >{{ h % 3 === 0 ? h : '' }}</span>
        </div>
      </div>

      <!-- 日期行容器：提示框以它为定位参照 -->
      <div class="relative">
        <div
          v-for="(day, ri) in rows"
          :key="day"
          class="flex items-center gap-3"
          :style="{ marginBottom: `${GAP}px`, height: `${ROW_H}px` }"
        >
          <span class="flex-shrink-0 text-xs" style="width: 64px; color: var(--text-color); opacity: 0.5;">
            {{ rowLabel(day) }}
            <span style="opacity: 0.6;">{{ rowWeekday(day) }}</span>
          </span>
          <div class="flex-1 grid" :style="{ gridTemplateColumns: 'repeat(24, minmax(0, 1fr))', gap: `${GAP}px` }">
            <div
              v-for="h in HOURS"
              :key="`${day}-${h}`"
              class="relative rounded-[3px] cursor-default"
              :style="[
                { height: `${ROW_H}px`, border: isEmptyCell(cellMap.get(`${day}|${h}`)) ? '1px dashed var(--button-border-color)' : 'none' },
                cellStyle(cellMap.get(`${day}|${h}`)),
              ]"
              @mouseenter="onCellEnter(ri, h)"
            >
              <span
                v-if="(cellMap.get(`${day}|${h}`)?.failures ?? 0) > 0"
                class="absolute rounded-full"
                style="top: 1px; right: 1px; width: 3px; height: 3px; background: #7f1d1d;"
              />
            </div>
          </div>
        </div>

        <!-- 悬停提示 -->
        <div
          v-if="hoverCell"
          class="pointer-events-none absolute z-20 rounded-lg px-3 py-2 shadow-lg whitespace-nowrap"
          :style="[tooltipStyle(), { backgroundColor: 'var(--bg-color)', border: '1px solid var(--button-border-color)' }]"
        >
          <div class="text-xs font-medium" style="color: var(--text-color);">
            {{ hoverCell.day }} {{ String(hoverCell.hour).padStart(2, '0') }}:00
          </div>
          <template v-if="hoverCell.cell && hoverCell.cell.samples > 0">
            <div class="text-xs mt-1" style="color: var(--text-color); opacity: 0.7;">
              {{ t('service.heatmapAvg') }}: <span class="font-semibold">{{ hoverCell.cell.avg }}ms</span>
            </div>
            <div class="text-xs" style="color: var(--text-color); opacity: 0.55;">
              {{ t('service.heatmapSamples') }}: {{ hoverCell.cell.samples }}
            </div>
          </template>
          <div v-else-if="isAllFailed(hoverCell.cell)" class="text-xs mt-1 text-red-500 font-medium">
            {{ t('service.heatmapAllFailed') }}
          </div>
          <div v-else class="text-xs mt-1" style="color: var(--text-color); opacity: 0.5;">
            {{ t('service.heatmapNoData') }}
          </div>
          <div v-if="hoverCell.cell && hoverCell.cell.failures > 0" class="text-xs mt-0.5 text-[#df2d2a] dark:text-[#f87171]">
            {{ t('service.heatmapFailures') }}: {{ hoverCell.cell.failures }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
