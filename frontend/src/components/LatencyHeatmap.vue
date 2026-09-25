<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { api } from '../api/client'
import type { LatencyHeatmapCell } from '../api/types'
import { useI18n } from '../composables/useI18n'

/**
 * 响应时间热力图：行 = 日期，列 = 小时，颜色越红表示该小时的平均响应越慢。
 *
 * 数据来自后端按「日期 × 小时」聚合的接口（SQL 内完成分组），
 * 公开状态页与管理端服务详情共用本组件。
 * 只用于暴露「每天固定时段的性能劣化」这类规律，因此不做逐点曲线。
 *
 * 两点展示口径：
 * - 窄屏（移动端）把 24 小时聚合成 8 个 3 小时桶，否则每格只有几个像素宽；
 * - 色阶是「相对 + 绝对下限」：整体很快的服务不会被相对色阶拉成橙色。
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

// ---- 移动端：一天聚合成 8 个 3 小时桶 ----
const MOBILE_WIDTH = 768
const MOBILE_HOURS_PER_BUCKET = 3

const isNarrow = ref(false)

function updateViewport() {
  isNarrow.value = window.innerWidth < MOBILE_WIDTH
}

const hoursPerBucket = computed(() => (isNarrow.value ? MOBILE_HOURS_PER_BUCKET : 1))

/** 列定义：宽屏 24 列（每小时），窄屏 8 列（每 3 小时） */
const columns = computed(() => {
  const step = hoursPerBucket.value
  const list: { key: number; startHour: number; endHour: number; label: string; showLabel: boolean }[] = []
  for (let h = 0; h < 24; h += step) {
    list.push({
      key: h,
      startHour: h,
      endHour: h + step - 1,
      label: String(h),
      // 宽屏 24 列时每 3 小时标一个刻度，避免数字挤在一起；窄屏 8 列全部显示
      showLabel: step > 1 || h % 3 === 0,
    })
  }
  return list
})

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

/** 渲染用的格子：窄屏时把同一桶内的小时按成功样本数加权合并 */
interface HeatmapViewCell {
  day: string
  startHour: number
  endHour: number
  avg: number
  samples: number
  failures: number
  maintenance: boolean
}

const displayRows = computed(() => {
  const step = hoursPerBucket.value
  return rows.value.map(day => {
    const cellsOfDay: HeatmapViewCell[] = []
    for (let h = 0; h < 24; h += step) {
      let samples = 0
      let failures = 0
      let weighted = 0
      let maintenance = false
      for (let hour = h; hour < h + step && hour < 24; hour++) {
        const c = cellMap.value.get(`${day}|${hour}`)
        if (!c) continue
        samples += c.samples
        failures += c.failures
        weighted += c.avg * c.samples
        if (c.maintenance) maintenance = true
      }
      cellsOfDay.push({
        day,
        startHour: h,
        endHour: Math.min(h + step - 1, 23),
        avg: samples > 0 ? Math.round(weighted / samples) : 0,
        samples,
        failures,
        maintenance,
      })
    }
    return { day, cells: cellsOfDay }
  })
})

// 绝对下限：平均延迟低于该值一律按「快」处理（绿色）。
// 否则「全程 100ms」的服务会因为窗口最大值也很低而被相对色阶染成橙色。
const FAST_FLOOR_MS = 200
// 色阶最小跨度：窗口最大值只略高于下限时不至于立刻满色（红）
const MIN_SPAN_MS = 300
const FAST_HUE = 145

/** 色阶上限：按当前展示粒度取实际最大值，窄屏聚合后不会被按小时算的上限压扁 */
const scaleMax = computed(() => {
  let max = 0
  for (const row of displayRows.value) {
    for (const c of row.cells) {
      if (c.samples > 0 && c.avg > max) max = c.avg
    }
  }
  return max > 0 ? max : maxAvg.value
})

const scaleMin = computed(() => {
  let min = 0
  for (const row of displayRows.value) {
    for (const c of row.cells) {
      if (c.samples > 0 && (min === 0 || c.avg < min)) min = c.avg
    }
  }
  return min
})

/** 平均延迟 → 颜色：绿（快）→ 黄 → 橙 → 红（慢），带绝对下限 */
function cellColor(avg: number): string {
  const max = scaleMax.value
  if (max <= 0) return `hsl(${FAST_HUE}, 55%, 45%)`
  const span = Math.max(max - FAST_FLOOR_MS, MIN_SPAN_MS)
  const ratio = Math.max(0, Math.min(1, (avg - FAST_FLOOR_MS) / span))
  const hue = FAST_HUE - FAST_HUE * ratio
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

function isEmptyCell(c: HeatmapViewCell): boolean {
  return c.samples === 0 && c.failures === 0
}

function isAllFailed(c: HeatmapViewCell): boolean {
  return c.samples === 0 && c.failures > 0
}

function cellStyle(c: HeatmapViewCell): Record<string, string> {
  if (isAllFailed(c)) {
    // 全部失败：用斜纹区分「慢」和「挂」，颜色不参与色阶。
    // 维护窗口内用蓝色斜纹（计划内停机），否则红色（真实故障）。
    const stripe = c.maintenance
      ? ['var(--maintenance-color, #3b82f6)', 'var(--maintenance-soft-color, #60a5fa)']
      : ['#df2d2a', '#b91c1c']
    return {
      backgroundImage: `repeating-linear-gradient(45deg, ${stripe[0]} 0 4px, ${stripe[1]} 4px 8px)`,
    }
  }
  if (isEmptyCell(c)) return {}
  return { backgroundColor: cellColor(c.avg) }
}

/** 该桶内出现过失败探测时右上角的小点：维护中为蓝色，否则红色 */
function failureDotColor(c: HeatmapViewCell): string {
  return c.maintenance ? 'var(--maintenance-color, #3b82f6)' : '#7f1d1d'
}

// ---- 悬停提示 ----
const hover = ref<{ row: number; col: number } | null>(null)

const hoverCell = computed(() => {
  if (!hover.value) return null
  const row = displayRows.value[hover.value.row]
  const cell = row?.cells[hover.value.col]
  if (!row || !cell) return null
  return { day: row.day, cell }
})

function onCellEnter(row: number, col: number) {
  hover.value = { row, col }
}

function rangeLabel(cell: HeatmapViewCell): string {
  const start = String(cell.startHour).padStart(2, '0')
  if (cell.startHour === cell.endHour) return `${start}:00`
  return `${start}:00-${String(cell.endHour).padStart(2, '0')}:59`
}

function tooltipStyle(): Record<string, string> {
  if (!hover.value) return {}
  const { row, col } = hover.value
  const total = columns.value.length || 24
  // 用行号/列号直接算位置，不依赖 DOM 测量；父容器只包住日期行，
  // 因此 top 可以直接按行高计算（刻度行在容器之外）
  const left = ((col + 0.5) / total) * 100
  // 靠近右边缘时把提示框往左收，避免溢出容器
  const translateX = col >= total - 4 ? '-92%' : col <= 3 ? '-8%' : '-50%'
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

onMounted(() => {
  updateViewport()
  window.addEventListener('resize', updateViewport)
  load()
})

onUnmounted(() => {
  window.removeEventListener('resize', updateViewport)
})

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
      {{ isNarrow ? t('service.heatmapHintNarrow') : t('service.heatmapHint') }}
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
        <div class="flex-1 grid" :style="{ gridTemplateColumns: `repeat(${columns.length}, minmax(0, 1fr))`, gap: `${GAP}px` }">
          <span
            v-for="col in columns"
            :key="'h' + col.key"
            class="text-center"
            style="font-size: 9px; color: var(--text-color); opacity: 0.4;"
          >{{ col.showLabel ? col.label : '' }}</span>
        </div>
      </div>

      <!-- 日期行容器：提示框以它为定位参照 -->
      <div class="relative">
        <div
          v-for="(row, ri) in displayRows"
          :key="row.day"
          class="flex items-center gap-3"
          :style="{ marginBottom: `${GAP}px`, height: `${ROW_H}px` }"
        >
          <span class="flex-shrink-0 text-xs" style="width: 64px; color: var(--text-color); opacity: 0.5;">
            {{ rowLabel(row.day) }}
            <span style="opacity: 0.6;">{{ rowWeekday(row.day) }}</span>
          </span>
          <div class="flex-1 grid" :style="{ gridTemplateColumns: `repeat(${columns.length}, minmax(0, 1fr))`, gap: `${GAP}px` }">
            <div
              v-for="(cell, ci) in row.cells"
              :key="`${row.day}-${cell.startHour}`"
              class="relative rounded-[3px] cursor-default"
              :style="[
                { height: `${ROW_H}px`, border: isEmptyCell(cell) ? '1px dashed var(--button-border-color)' : 'none' },
                cellStyle(cell),
              ]"
              @mouseenter="onCellEnter(ri, ci)"
            >
              <span
                v-if="cell.failures > 0"
                class="absolute rounded-full"
                :style="{ top: '1px', right: '1px', width: '3px', height: '3px', background: failureDotColor(cell) }"
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
            {{ hoverCell.day }} {{ rangeLabel(hoverCell.cell) }}
            <span v-if="hoverCell.cell.maintenance" class="ml-1 text-[color:var(--maintenance-color)]">· {{ t('service.heatmapMaintenance') }}</span>
          </div>
          <template v-if="hoverCell.cell.samples > 0">
            <div class="text-xs mt-1" style="color: var(--text-color); opacity: 0.7;">
              {{ t('service.heatmapAvg') }}: <span class="font-semibold">{{ hoverCell.cell.avg }}ms</span>
            </div>
            <div class="text-xs" style="color: var(--text-color); opacity: 0.55;">
              {{ t('service.heatmapSamples') }}: {{ hoverCell.cell.samples }}
            </div>
          </template>
          <div v-else-if="isAllFailed(hoverCell.cell)" class="text-xs mt-1 font-medium" :class="hoverCell.cell.maintenance ? 'text-[color:var(--maintenance-color)]' : 'text-red-500'">
            {{ hoverCell.cell.maintenance ? t('service.heatmapAllFailedMaintenance') : t('service.heatmapAllFailed') }}
          </div>
          <div v-else class="text-xs mt-1" style="color: var(--text-color); opacity: 0.5;">
            {{ t('service.heatmapNoData') }}
          </div>
          <div v-if="hoverCell.cell.failures > 0" class="text-xs mt-0.5" :class="hoverCell.cell.maintenance ? 'text-[color:var(--maintenance-color)]' : 'text-[#df2d2a] dark:text-[#f87171]'">
            {{ t('service.heatmapFailures') }}: {{ hoverCell.cell.failures }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
