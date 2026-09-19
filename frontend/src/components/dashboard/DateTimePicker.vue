<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import CustomSelect from './CustomSelect.vue'
import { useI18n } from '../../composables/useI18n'

/**
 * 自研日期 / 日期时间 / 月份 / 时刻选择器。
 *
 * 不使用浏览器原生的 `input[type=date|datetime-local|month|time]`：原生控件在
 * 各浏览器里外观差异大、无法跟随主题（深色模式下仍是白底），因此在管理端
 * 统一换成这里的日历弹层 + 时/分下拉。
 *
 * 值格式（与原生控件一致，便于直接落库 / 提交）：
 * - `datetime`：`YYYY-MM-DDTHH:mm`
 * - `date`：`YYYY-MM-DD`
 * - `month`：`YYYY-MM`
 * - `time`：`HH:mm`（只有时/分下拉，没有日历）
 * 空字符串表示未选择。
 *
 * 时间口径统一为北京时间（UTC+8），与后端存储和展示保持一致。
 */

type Mode = 'datetime' | 'date' | 'month' | 'time'

const props = withDefaults(defineProps<{
  modelValue: string
  mode?: Mode
  placeholder?: string
  /** 可选下界（与值同格式），早于该值的日期不可选 */
  min?: string
  /** 可选上界（与值同格式），晚于该值的日期不可选 */
  max?: string
  disabled?: boolean
}>(), {
  mode: 'datetime',
})

const { t } = useI18n()

/** 未显式传入 placeholder 时使用本地化默认文案（props 默认值不随语言切换变化，故放在 computed 里） */
const placeholderText = computed(() => props.placeholder || t('admin.datepicker.placeholder'))

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const CST = 'Asia/Shanghai'

interface Parts { y: number; m: number; d: number; hh: number; mm: number }

const pad = (n: number) => String(n).padStart(2, '0')

/** 当前北京时间（说「今天」「此刻」时使用） */
function nowParts(): Parts {
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone: CST,
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', hour12: false,
  }).formatToParts(new Date())
  const get = (t: string) => parts.find(p => p.type === t)?.value || '0'
  return { y: Number(get('year')), m: Number(get('month')), d: Number(get('day')), hh: Number(get('hour')) % 24, mm: Number(get('minute')) }
}

function parseValue(raw: string): Parts | null {
  if (!raw) return null
  const dt = raw.match(/^(\d{4})-(\d{2})-(\d{2})[T ](\d{2}):(\d{2})/)
  if (dt) return { y: +dt[1], m: +dt[2], d: +dt[3], hh: +dt[4], mm: +dt[5] }
  const day = raw.match(/^(\d{4})-(\d{2})-(\d{2})$/)
  if (day) return { y: +day[1], m: +day[2], d: +day[3], hh: 0, mm: 0 }
  const mon = raw.match(/^(\d{4})-(\d{2})$/)
  if (mon) return { y: +mon[1], m: +mon[2], d: 1, hh: 0, mm: 0 }
  const time = raw.match(/^(\d{1,2}):(\d{2})$/)
  if (time) return { ...nowParts(), hh: +time[1] % 24, mm: +time[2] }
  return null
}

function formatValue(p: Parts): string {
  if (props.mode === 'month') return `${p.y}-${pad(p.m)}`
  if (props.mode === 'date') return `${p.y}-${pad(p.m)}-${pad(p.d)}`
  if (props.mode === 'time') return `${pad(p.hh)}:${pad(p.mm)}`
  return `${p.y}-${pad(p.m)}-${pad(p.d)}T${pad(p.hh)}:${pad(p.mm)}`
}

/** 当前值；未选择时以「今天 00:00」为基准，保证点选任意一格都能得到完整时间 */
const current = computed<Parts>(() => {
  const parsed = parseValue(props.modelValue)
  if (parsed) return parsed
  const now = nowParts()
  // 只选时刻时，未设置就默认用当前时刻，而不是 00:00
  if (props.mode === 'time') return now
  return { ...now, hh: 0, mm: 0 }
})

const displayText = computed(() => {
  const parsed = parseValue(props.modelValue)
  if (!parsed) return ''
  if (props.mode === 'time') return formatValue(parsed)
  return props.mode === 'datetime' ? `${parsed.y}-${pad(parsed.m)}-${pad(parsed.d)} ${pad(parsed.hh)}:${pad(parsed.mm)}` : formatValue(parsed)
})

// ---- 弹层状态 ----
const open = ref(false)
const dropUp = ref(false)
const root = ref<HTMLElement | null>(null)

const now = nowParts()
const viewYear = ref(now.y)
const viewMonth = ref(now.m)

const showCalendar = computed(() => props.mode !== 'month' && props.mode !== 'time')

/** 月份名（数组下标 0 = 一月），供月历表头与月份网格复用 */
const MONTH_LABELS = computed(() => [
  t('admin.datepicker.monthJan'),
  t('admin.datepicker.monthFeb'),
  t('admin.datepicker.monthMar'),
  t('admin.datepicker.monthApr'),
  t('admin.datepicker.monthMay'),
  t('admin.datepicker.monthJun'),
  t('admin.datepicker.monthJul'),
  t('admin.datepicker.monthAug'),
  t('admin.datepicker.monthSep'),
  t('admin.datepicker.monthOct'),
  t('admin.datepicker.monthNov'),
  t('admin.datepicker.monthDec'),
])

const headerLabel = computed(() =>
  props.mode === 'month'
    ? t('admin.datepicker.yearOnly', { year: viewYear.value })
    : t('admin.datepicker.yearMonth', { year: viewYear.value, month: MONTH_LABELS.value[viewMonth.value - 1] }),
)

const WEEKDAYS = computed(() => [
  t('admin.datepicker.weekday.mon'),
  t('admin.datepicker.weekday.tue'),
  t('admin.datepicker.weekday.wed'),
  t('admin.datepicker.weekday.thu'),
  t('admin.datepicker.weekday.fri'),
  t('admin.datepicker.weekday.sat'),
  t('admin.datepicker.weekday.sun'),
])

const MONTHS = Array.from({ length: 12 }, (_, i) => i + 1)

const HOURS = Array.from({ length: 24 }, (_, i) => ({ label: pad(i), value: pad(i) }))
const MINUTES = Array.from({ length: 60 }, (_, i) => ({ label: pad(i), value: pad(i) }))

interface Cell { key: string; day: number; y: number; m: number }

/** 月历网格：周一为一周起点，前后补齐到整行，保证弹层高度稳定 */
const cells = computed<Cell[]>(() => {
  const y = viewYear.value
  const m = viewMonth.value
  const firstWeekday = (new Date(Date.UTC(y, m - 1, 1)).getUTCDay() + 6) % 7 // 0=周一
  const daysThis = new Date(Date.UTC(y, m, 0)).getUTCDate()
  const prevMonth = m === 1 ? 12 : m - 1
  const prevYear = m === 1 ? y - 1 : y
  const daysPrev = new Date(Date.UTC(prevYear, prevMonth, 0)).getUTCDate()
  const nextMonth = m === 12 ? 1 : m + 1
  const nextYear = m === 12 ? y + 1 : y

  const list: Cell[] = []
  for (let i = firstWeekday - 1; i >= 0; i--) {
    list.push({ key: `p${i}`, day: daysPrev - i, y: prevYear, m: prevMonth })
  }
  for (let d = 1; d <= daysThis; d++) {
    list.push({ key: `c${d}`, day: d, y, m })
  }
  const total = Math.ceil(list.length / 7) * 7
  for (let d = 1; list.length < total; d++) {
    list.push({ key: `n${d}`, day: d, y: nextYear, m: nextMonth })
  }
  return list
})

function isDisabled(p: Parts): boolean {
  const v = formatValue(p)
  if (props.min) {
    const min = parseValue(props.min)
    if (min && v < formatValue(min)) return true
  }
  if (props.max) {
    const max = parseValue(props.max)
    if (max && v > formatValue(max)) return true
  }
  return false
}

function isCurrentCell(c: Cell): boolean {
  return current.value.y === c.y && current.value.m === c.m && current.value.d === c.day
}

function isTodayCell(c: Cell): boolean {
  const t = nowParts()
  return t.y === c.y && t.m === c.m && t.d === c.day
}

function isCellDisabled(c: Cell): boolean {
  return isDisabled({ ...current.value, y: c.y, m: c.m, d: c.day })
}

function stepMonth(offset: number) {
  const d = new Date(Date.UTC(viewYear.value, viewMonth.value - 1 + offset, 1))
  viewYear.value = d.getUTCFullYear()
  viewMonth.value = d.getUTCMonth() + 1
}

function stepYear(offset: number) {
  viewYear.value += offset
}

function commit(p: Parts) {
  emit('update:modelValue', formatValue(p))
}

function pickDay(c: Cell) {
  if (isCellDisabled(c)) return
  commit({ ...current.value, y: c.y, m: c.m, d: c.day })
  if (props.mode === 'date') open.value = false
}

function pickMonth(m: number) {
  if (isDisabled({ ...current.value, y: viewYear.value, m })) return
  commit({ ...current.value, y: viewYear.value, m })
  open.value = false
}

function setHour(value: string) {
  commit({ ...current.value, hh: Number(value) })
}

function setMinute(value: string) {
  commit({ ...current.value, mm: Number(value) })
}

/** 「现在」按钮：datetime 取此刻，date 取今天，month 取本月，time 取当前时刻 */
function pickNow() {
  const t = nowParts()
  if (props.mode === 'time') {
    commit({ ...current.value, hh: t.hh, mm: t.mm })
  } else if (props.mode === 'month') {
    commit({ ...current.value, y: t.y, m: t.m })
  } else {
    commit({ ...current.value, y: t.y, m: t.m, d: t.d, hh: t.hh, mm: t.mm })
  }
  open.value = false
}

function clear() {
  emit('update:modelValue', '')
  open.value = false
}

function syncView() {
  const parsed = parseValue(props.modelValue)
  const base = parsed || nowParts()
  viewYear.value = base.y
  viewMonth.value = base.m
}

function computeDropUp() {
  const el = root.value
  if (!el) return
  const rect = el.getBoundingClientRect()
  // 弹层高度：日期模式约 400px（含日历、时间行与页脚），只有时刻时约 150px
  const needed = props.mode === 'time' ? 180 : 400
  dropUp.value = window.innerHeight - rect.bottom < needed && rect.top > needed
}

function toggle() {
  if (props.disabled) return
  if (!open.value) {
    syncView()
    computeDropUp()
  }
  open.value = !open.value
}

function onDocumentClick(e: MouseEvent) {
  if (root.value && !root.value.contains(e.target as Node)) open.value = false
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && open.value) {
    open.value = false
    e.stopPropagation()
  }
}

onMounted(() => {
  document.addEventListener('click', onDocumentClick)
  document.addEventListener('keydown', onKeydown)
})
onUnmounted(() => {
  document.removeEventListener('click', onDocumentClick)
  document.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <div ref="root" class="dtp">
    <button
      type="button"
      class="dtp-trigger"
      :disabled="disabled"
      :class="{ 'is-open': open }"
      @click="toggle"
    >
      <span class="dtp-text" :class="{ 'is-empty': !displayText }">{{ displayText || placeholderText }}</span>
      <svg v-if="mode !== 'time'" class="dtp-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.6">
        <path stroke-linecap="round" stroke-linejoin="round" d="M6.75 3v2.25M17.25 3v2.25M3 18.75V7.5a2.25 2.25 0 012.25-2.25h13.5A2.25 2.25 0 0121 7.5v11.25m-18 0A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75m-18 0v-7.5A2.25 2.25 0 015.25 9h13.5A2.25 2.25 0 0121 11.25v7.5" />
      </svg>
      <svg v-else class="dtp-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.6">
        <path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
    </button>

    <Transition name="dtp-pop">
      <div v-if="open" class="dtp-panel" :class="{ 'is-up': dropUp, 'is-time': mode === 'time' }" @click.stop>
        <div v-if="mode !== 'time'" class="dtp-head">
          <button v-if="mode !== 'month'" type="button" class="dtp-nav" :title="t('admin.datepicker.prevYear')" @click="stepMonth(-12)">&laquo;</button>
          <button v-if="mode !== 'month'" type="button" class="dtp-nav" :title="t('admin.datepicker.prevMonth')" @click="stepMonth(-1)">&lsaquo;</button>
          <button v-else type="button" class="dtp-nav" :title="t('admin.datepicker.prevYear')" @click="stepYear(-1)">&lsaquo;</button>
          <span class="dtp-title">{{ headerLabel }}</span>
          <button v-if="mode !== 'month'" type="button" class="dtp-nav" :title="t('admin.datepicker.nextMonth')" @click="stepMonth(1)">&rsaquo;</button>
          <button v-if="mode !== 'month'" type="button" class="dtp-nav" :title="t('admin.datepicker.nextYear')" @click="stepMonth(12)">&raquo;</button>
          <button v-else type="button" class="dtp-nav" :title="t('admin.datepicker.nextYear')" @click="stepYear(1)">&rsaquo;</button>
        </div>

        <template v-if="showCalendar">
          <div class="dtp-week">
            <span v-for="w in WEEKDAYS" :key="w">{{ w }}</span>
          </div>
          <div class="dtp-grid">
            <button
              v-for="c in cells"
              :key="c.key"
              type="button"
              class="dtp-day"
              :class="{
                'is-out': c.m !== viewMonth,
                'is-today': isTodayCell(c),
                'is-active': isCurrentCell(c),
              }"
              :disabled="isCellDisabled(c)"
              @click="pickDay(c)"
            >{{ c.day }}</button>
          </div>
        </template>

        <div v-else-if="mode === 'month'" class="dtp-months">
          <button
            v-for="m in MONTHS"
            :key="m"
            type="button"
            class="dtp-month"
            :class="{ 'is-active': current.y === viewYear && current.m === m }"
            :disabled="isDisabled({ ...current, y: viewYear, m })"
            @click="pickMonth(m)"
          >{{ MONTH_LABELS[m - 1] }}</button>
        </div>

        <div v-if="mode === 'datetime' || mode === 'time'" class="dtp-time" :class="{ 'is-first': mode === 'time' }">
          <CustomSelect :model-value="pad(current.hh)" :options="HOURS" @update:model-value="setHour" />
          <span class="dtp-colon">:</span>
          <CustomSelect :model-value="pad(current.mm)" :options="MINUTES" @update:model-value="setMinute" />
        </div>

        <div class="dtp-foot">
          <button type="button" class="dtp-link" @click="pickNow">
            {{ mode === 'datetime' || mode === 'time' ? t('admin.datepicker.now') : mode === 'date' ? t('admin.datepicker.today') : t('admin.datepicker.thisMonth') }}
          </button>
          <button type="button" class="dtp-link" @click="clear">{{ t('admin.datepicker.clear') }}</button>
        </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.dtp {
  position: relative;
}
.dtp-trigger {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 8px;
  font-size: 14px;
  cursor: pointer;
  transition: border-color 0.15s;
  border: 1px solid var(--button-border-color);
  background-color: var(--bg-color);
  color: var(--text-color);
}
.dtp-trigger:hover:not(:disabled) {
  border-color: var(--text-color);
  opacity: 0.8;
}
.dtp-trigger:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}
.dtp-text {
  text-align: left;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.dtp-text.is-empty {
  opacity: 0.4;
}
.dtp-icon {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
  opacity: 0.45;
}

.dtp-panel {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  z-index: 60;
  width: 268px;
  padding: 10px;
  border-radius: 10px;
  background-color: var(--bg-color);
  border: 1px solid var(--button-border-color);
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.16);
}
.dtp-panel.is-up {
  top: auto;
  bottom: calc(100% + 6px);
}
.dtp-panel.is-time {
  width: 190px;
}

.dtp-head {
  display: flex;
  align-items: center;
  gap: 2px;
  margin-bottom: 6px;
}
.dtp-title {
  flex: 1;
  text-align: center;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-color);
}
.dtp-nav {
  width: 26px;
  height: 26px;
  border-radius: 6px;
  font-size: 14px;
  line-height: 1;
  cursor: pointer;
  border: none;
  background: none;
  color: var(--text-color);
  opacity: 0.6;
  transition: background-color 0.12s, opacity 0.12s;
}
.dtp-nav:hover {
  opacity: 1;
  background-color: var(--button-hover-color);
}

.dtp-week,
.dtp-grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
}
.dtp-week span {
  text-align: center;
  font-size: 11px;
  padding: 2px 0 4px;
  color: var(--text-color);
  opacity: 0.4;
}
.dtp-grid {
  gap: 1px;
}
.dtp-day {
  height: 30px;
  border-radius: 6px;
  font-size: 13px;
  cursor: pointer;
  border: none;
  background: none;
  color: var(--text-color);
  font-variant-numeric: tabular-nums;
  transition: background-color 0.12s;
}
.dtp-day:hover:not(:disabled) {
  background-color: var(--button-hover-color);
}
.dtp-day.is-out {
  opacity: 0.3;
}
.dtp-day.is-today {
  color: #10b981;
  font-weight: 600;
}
.dtp-day.is-active {
  background-color: #10b981;
  color: #fff;
  font-weight: 600;
}
.dtp-day.is-active:hover {
  background-color: #0ea472;
}
.dtp-day:disabled,
.dtp-month:disabled {
  opacity: 0.2;
  cursor: not-allowed;
}

.dtp-months {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 4px;
}
.dtp-month {
  padding: 8px 0;
  border-radius: 6px;
  font-size: 13px;
  cursor: pointer;
  border: none;
  background: none;
  color: var(--text-color);
  transition: background-color 0.12s;
}
.dtp-month:hover:not(:disabled) {
  background-color: var(--button-hover-color);
}
.dtp-month.is-active {
  background-color: #10b981;
  color: #fff;
  font-weight: 600;
}

.dtp-time {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--button-border-color);
}
/* 只有时刻时，弹层里没有日历，顶部不需要分割线 */
.dtp-time.is-first {
  margin-top: 0;
  padding-top: 0;
  border-top: none;
}
.dtp-time :deep(.custom-select) {
  flex: 1;
}
.dtp-colon {
  color: var(--text-color);
  opacity: 0.5;
}

.dtp-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--button-border-color);
}
.dtp-link {
  font-size: 12px;
  cursor: pointer;
  border: none;
  background: none;
  color: #10b981;
  padding: 2px 4px;
  border-radius: 4px;
}
.dtp-link:hover {
  background-color: var(--button-hover-color);
}

.dtp-pop-enter-active,
.dtp-pop-leave-active {
  transition: opacity 0.15s, transform 0.15s;
}
.dtp-pop-enter-from,
.dtp-pop-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
