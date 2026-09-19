<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '../../api/client'
import type { Maintenance, Service } from '../../api/types'
import { useToast } from '../../composables/useToast'
import { useUnsavedChanges } from '../../composables/useUnsavedChanges'
import CustomSelect from './CustomSelect.vue'
import DateTimePicker from './DateTimePicker.vue'
import { useI18n } from '../../composables/useI18n'

const { t, formatDateTime } = useI18n()

const maintenances = ref<Maintenance[]>([])
const loading = ref(true)
const { show: toast } = useToast()
const showForm = ref(false)
const editing = ref<Maintenance | null>(null)
const form = ref({
  title: '',
  description: '',
  scheduledStart: '',
  scheduledEnd: '',
  status: 'scheduled',
  affectedServices: '',
  recurrence: '',
  recurrenceInterval: 1,
  recurrenceMonthday: 0,
  recurrenceUntil: '',
})

const { markClean: cleanMtn, handleClose: closeMtn, restoreFromStorage: restoreMtn } = useUnsavedChanges(form as any, 'mtn_form')

// Timezone helpers: assume all times are CST (UTC+8)
const CST_OFFSET = '+08:00'

// 表单里的时间统一用自研 DateTimePicker（值格式 "YYYY-MM-DDTHH:mm"），
// 因此回填时必须把后端存储的 ISO 时间转换成 CST 墙上时间。
function toDatetimeLocalValue(iso: string): string {
  if (!iso) return ''
  // 已经是不带时区的 datetime-local 形式，直接归一化（去掉秒/毫秒）
  const plain = iso.match(/^(\d{4}-\d{2}-\d{2})[T ](\d{2}:\d{2})(?::\d{2}(?:\.\d+)?)?$/)
  if (plain) return `${plain[1]}T${plain[2]}`
  // 带时区的时间（Z 或 ±HH:mm），按 CST 还原成墙上时间
  const withTz = /[Zz]$|[+-]\d{2}:?\d{2}$/.test(iso) ? iso : iso + CST_OFFSET
  const d = new Date(withTz)
  if (Number.isNaN(d.getTime())) {
    // 无法解析时退化为截取前 16 个字符，尽量避免把原值丢掉
    return iso.length >= 16 ? iso.slice(0, 16) : ''
  }
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', hour12: false,
  }).formatToParts(d)
  const get = (t: string) => parts.find(p => p.type === t)?.value || ''
  const hour = get('hour') === '24' ? '00' : get('hour')
  return `${get('year')}-${get('month')}-${get('day')}T${hour}:${get('minute')}`
}

function toCSTISO(dt: string): string {
  if (!dt) return ''
  // 统一先归一化成 "YYYY-MM-DDTHH:mm"，再补上秒和 CST 偏移，避免重复拼接时区
  const normalized = toDatetimeLocalValue(dt)
  if (!normalized) return ''
  return `${normalized}:00${CST_OFFSET}`
}

function formatCST(iso: string): string {
  if (!iso) return ''
  // If no timezone info in the string, assume CST
  const s = /[Z+-]/.test(iso) ? iso : iso + CST_OFFSET
  if (Number.isNaN(new Date(s).getTime())) return iso
  return formatDateTime(s)
}

// Service multi-select
const services = ref<(Service & { uptime: number; latency: number })[]>([])
const serviceSearch = ref('')
const showServiceDropdown = ref(false)
const selectedServices = ref<number[]>([])

const filteredServices = computed(() => {
  if (!serviceSearch.value) return services.value
  const q = serviceSearch.value.toLowerCase()
  return services.value.filter(s => s.name.toLowerCase().includes(q))
})

function affectedServiceNames(ids: string): string {
  if (!ids) return '-'
  return ids.split(',').map(id => {
    const svc = services.value.find(s => s.id === Number(id))
    return svc ? svc.name : id
  }).join(', ')
}

function toggleService(svc: { id: number; name: string }) {
  const idx = selectedServices.value.indexOf(svc.id)
  if (idx >= 0) {
    selectedServices.value.splice(idx, 1)
  } else {
    selectedServices.value.push(svc.id)
  }
  serviceSearch.value = ''
}

function delayBlur() {
  setTimeout(() => { showServiceDropdown.value = false }, 200)
}

const statusClass = (s: string) => {
  switch (s) {
    case 'scheduled': return 'text-blue-600 bg-blue-50 border-blue-100 dark:text-blue-400 dark:bg-blue-900/30 dark:border-blue-800'
    case 'in_progress': return 'text-amber-600 bg-amber-50 border-amber-100 dark:text-amber-400 dark:bg-amber-900/30 dark:border-amber-800'
    case 'completed': return 'text-gray-500 bg-gray-50 border-gray-100 dark:text-gray-400 dark:bg-gray-800 dark:border-gray-700'
    case 'cancelled': return 'text-red-600 bg-red-50 border-red-100 dark:text-red-400 dark:bg-red-900/30 dark:border-red-800'
    default: return 'text-gray-600 bg-gray-50 border-gray-100 dark:text-gray-400 dark:bg-gray-800 dark:border-gray-700'
  }
}

const statusLabel = computed<Record<string, string>>(() => ({
  scheduled: t('admin.maintenance.status.scheduled'),
  in_progress: t('admin.maintenance.status.inProgress'),
  completed: t('admin.maintenance.status.completed'),
  cancelled: t('admin.maintenance.status.cancelled'),
}))

/** 维护状态下拉选项（随语言切换重新渲染） */
const statusOptions = computed(() => [
  { label: t('admin.maintenance.status.scheduled'), value: 'scheduled' },
  { label: t('admin.maintenance.status.inProgress'), value: 'in_progress' },
  { label: t('admin.maintenance.status.completed'), value: 'completed' },
  { label: t('admin.maintenance.status.cancelled'), value: 'cancelled' },
])

// ---- 周期维护 ----

const recurrenceOptions = computed(() => [
  { label: t('admin.maintenance.recurrence.none'), value: '' },
  { label: t('admin.maintenance.recurrence.daily'), value: 'daily' },
  { label: t('admin.maintenance.recurrence.weekly'), value: 'weekly' },
  { label: t('admin.maintenance.recurrence.monthly'), value: 'monthly' },
])

const WEEKDAY_LABELS = computed(() => [
  '',
  t('admin.maintenance.weekday.mon'),
  t('admin.maintenance.weekday.tue'),
  t('admin.maintenance.weekday.wed'),
  t('admin.maintenance.weekday.thu'),
  t('admin.maintenance.weekday.fri'),
  t('admin.maintenance.weekday.sat'),
  t('admin.maintenance.weekday.sun'),
])

/** 把重复规则描述成一句话，列表里作为标签展示 */
function recurrenceLabel(m: Maintenance): string {
  const interval = m.recurrenceInterval || 1
  switch (m.recurrence) {
    case 'daily':
      return interval === 1 ? t('admin.maintenance.recurrence.daily') : t('admin.maintenance.recurrence.everyNDays', { n: interval })
    case 'weekly': {
      // 重复日期由开始时间的星期决定，这里直接由 scheduledStart 推导，避免回显不一致
      const weekday = weekdayOf(m.scheduledStart)
      const day = weekday ? WEEKDAY_LABELS.value[weekday] : ''
      return interval === 1
        ? t('admin.maintenance.recurrence.weeklyOn', { day })
        : t('admin.maintenance.recurrence.everyNWeeksOn', { n: interval, day })
    }
    case 'monthly':
      return interval === 1 ? t('admin.maintenance.recurrence.monthly') : t('admin.maintenance.recurrence.everyNMonths', { n: interval })
    default:
      return ''
  }
}

/** 从 ISO / datetime-local 串里取星期（1=周一 … 7=周日） */
function weekdayOf(iso: string): number {
  if (!iso) return 0
  const s = /[Zz]$|[+-]\d{2}:?\d{2}$/.test(iso) ? iso : iso + CST_OFFSET
  const d = new Date(s)
  if (Number.isNaN(d.getTime())) return 0
  const day = d.getDay() // 0=周日
  return day === 0 ? 7 : day
}

/** 重复截止日期的展示文案 */
function recurrenceUntilLabel(m: Maintenance): string {
  return m.recurrenceUntil ? t('admin.maintenance.recurrence.until', { date: m.recurrenceUntil }) : ''
}

async function load() {
  loading.value = true
  try {
    const res = await api.getAdminMaintenances()
    maintenances.value = res.data
  } catch (e: any) {
    toast(e.message || t('admin.maintenance.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function loadServices() {
  try {
    const res = await api.getAdminServices()
    services.value = res.data
  } catch {
    // silent
  }
}

function openCreate() {
  editing.value = null
  const defaults = {
    title: '',
    description: '',
    scheduledStart: '',
    scheduledEnd: '',
    status: 'scheduled',
    affectedServices: '',
    recurrence: '',
    recurrenceInterval: 1,
    recurrenceMonthday: 0,
    recurrenceUntil: '',
  }
  form.value = { ...defaults }
  selectedServices.value = []
  serviceSearch.value = ''
  restoreMtn()
  cleanMtn()
  showForm.value = true
}

function openEdit(m: Maintenance) {
  editing.value = m
  form.value = {
    title: m.title,
    description: m.description || '',
    // datetime-local 只能接收 "YYYY-MM-DDTHH:mm"，必须把 ISO 时间按 CST 回填，
    // 否则输入框会被浏览器判定为非法值而清空，保存时时间就丢了
    scheduledStart: toDatetimeLocalValue(m.scheduledStart),
    scheduledEnd: toDatetimeLocalValue(m.scheduledEnd),
    status: m.status,
    affectedServices: m.affectedServices || '',
    recurrence: m.recurrence || '',
    recurrenceInterval: m.recurrenceInterval || 1,
    recurrenceMonthday: m.recurrenceMonthday || 0,
    recurrenceUntil: m.recurrenceUntil || '',
  }
  selectedServices.value = m.affectedServices ? m.affectedServices.split(',').map(Number) : []
  serviceSearch.value = ''
  cleanMtn()
  showForm.value = true
}

function handleCloseMtn() {
  if (closeMtn()) showForm.value = false
}

async function save() {
  form.value.affectedServices = selectedServices.value.join(',')
  // Convert local datetime to explicit CST time for storage
  const payload = {
    ...form.value,
    scheduledStart: toCSTISO(form.value.scheduledStart),
    scheduledEnd: toCSTISO(form.value.scheduledEnd),
    // 一次性维护窗口：清空周期参数，避免残留脏数据
    recurrenceInterval: form.value.recurrence ? Number(form.value.recurrenceInterval) || 1 : 0,
    recurrenceMonthday: form.value.recurrence === 'monthly' ? Number(form.value.recurrenceMonthday) || 0 : 0,
    recurrenceUntil: form.value.recurrence ? form.value.recurrenceUntil : '',
  }
  try {
    if (editing.value) {
      await api.updateMaintenance(editing.value.id, payload)
    } else {
      await api.createMaintenance(payload)
    }
    showForm.value = false
    cleanMtn()
    toast(editing.value ? t('admin.maintenance.updateSuccess') : t('admin.maintenance.createSuccess'), 'success')
    load()
  } catch (e: any) {
    toast(e.message || t('admin.maintenance.saveFailed'))
  }
}

async function remove(id: number) {
  if (!confirm(t('admin.maintenance.confirmDelete'))) return
  try {
    await api.deleteMaintenance(id)
    toast(t('admin.maintenance.deleteSuccess'), 'success')
    load()
  } catch (e: any) {
    toast(e.message || t('admin.maintenance.deleteFailed'))
  }
}

onMounted(() => {
  load()
  loadServices()
})
</script>

<template>
  <div>
    <div class="flex justify-between items-center mb-4">
      <h2 class="text-lg font-bold" style="color: var(--text-color);">{{ t('admin.maintenance.listTitle') }}</h2>
      <button @click="openCreate" class="bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium px-4 py-2 rounded-lg flex items-center gap-1 transition-colors">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" /></svg>
        {{ t('admin.maintenance.create') }}
      </button>
    </div>

    <div v-if="loading" class="text-center py-12" style="color: var(--text-color); opacity: 0.4;">{{ t('common.loading') }}</div>

    <div v-else class="rounded-xl" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
      <div class="overflow-x-auto">
        <table class="w-full text-left text-sm">
        <thead class="text-xs" style="color: var(--text-color); opacity: 0.4; background-color: var(--button-hover-color); opacity: 0.5; border-bottom: 1px solid var(--button-border-color);">
          <tr>
            <th class="px-6 py-3 font-medium">{{ t('admin.maintenance.colTitle') }}</th>
            <th class="px-6 py-3 font-medium">{{ t('admin.maintenance.colStatus') }}</th>
            <th class="px-6 py-3 font-medium">{{ t('admin.maintenance.colStart') }}</th>
            <th class="px-6 py-3 font-medium">{{ t('admin.maintenance.colEnd') }}</th>
            <th class="px-6 py-3 font-medium">{{ t('admin.maintenance.colAffectedServices') }}</th>
            <th class="px-6 py-3 font-medium text-right">{{ t('admin.maintenance.colActions') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y">
          <tr v-for="m in maintenances" :key="m.id">
            <td class="px-6 py-4 font-bold" style="color: var(--text-color);">
              {{ m.title }}
              <div v-if="m.recurrence" class="mt-1 flex items-center gap-1.5 font-normal">
                <span class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium bg-violet-50 dark:bg-violet-500/15 text-violet-600 dark:text-violet-400">
                  <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                  </svg>
                  {{ recurrenceLabel(m) }}
                </span>
                <span v-if="m.recurrenceUntil" class="text-[10px]" style="color: var(--text-color); opacity: 0.4;">
                  {{ recurrenceUntilLabel(m) }}
                </span>
              </div>
            </td>
            <td class="px-6 py-4">
              <span :class="['inline-flex items-center px-2 py-1 rounded text-xs font-medium border', statusClass(m.status)]">
                {{ statusLabel[m.status] || m.status }}
              </span>
            </td>
            <td class="px-6 py-4 text-xs" style="color: var(--text-color); opacity: 0.5;">{{ formatCST(m.scheduledStart) }}</td>
            <td class="px-6 py-4 text-xs" style="color: var(--text-color); opacity: 0.5;">{{ formatCST(m.scheduledEnd) }}</td>
            <td class="px-6 py-4 text-xs max-w-40 truncate" style="color: var(--text-color); opacity: 0.5;">{{ affectedServiceNames(m.affectedServices) }}</td>
            <td class="px-6 py-4 text-right">
              <button @click="openEdit(m)" class="op-btn op-btn-edit mr-3" :title="t('admin.maintenance.edit')">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                </svg>
              </button>
              <button @click="remove(m.id)" class="op-btn op-btn-delete" :title="t('admin.maintenance.delete')">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
              </button>
            </td>
          </tr>
        </tbody>
      </table>
        </div>
    </div>

    <!-- Form Modal -->
    <div v-if="showForm" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="handleCloseMtn">
      <div class="rounded-xl p-4 md:p-6 w-full max-w-lg mx-4" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">{{ editing ? t('admin.maintenance.editMaintenance') : t('admin.maintenance.create') }}</h3>
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.maintenance.labelTitle') }}</label>
            <input v-model="form.title" class="w-full px-3 py-2 rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500" style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.maintenance.labelDescription') }}</label>
            <textarea v-model="form.description" rows="2" class="w-full px-3 py-2 rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500" style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);"></textarea>
          </div>
          <div class="grid grid-cols-2 gap-4">
            <div>
              <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.maintenance.labelStart') }}</label>
              <DateTimePicker v-model="form.scheduledStart" :placeholder="t('admin.maintenance.startPlaceholder')" />
            </div>
            <div>
              <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.maintenance.labelEnd') }}</label>
              <DateTimePicker v-model="form.scheduledEnd" :placeholder="t('admin.maintenance.endPlaceholder')" />
            </div>
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.maintenance.labelStatus') }}</label>
            <CustomSelect v-model="form.status" :options="statusOptions" />
          </div>

          <!-- 周期重复：窗口结束后由服务端推进到下一个窗口 -->
          <div class="rounded-lg p-3 space-y-3" style="border: 1px solid var(--button-border-color);">
            <div>
              <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.maintenance.labelRecurrence') }}</label>
              <CustomSelect v-model="form.recurrence" :options="recurrenceOptions" />
            </div>

            <template v-if="form.recurrence">
              <div class="grid grid-cols-2 gap-4">
                <div>
                  <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">
                    {{ form.recurrence === 'daily' ? t('admin.maintenance.intervalDays') : form.recurrence === 'weekly' ? t('admin.maintenance.intervalWeeks') : t('admin.maintenance.intervalMonths') }}
                  </label>
                  <input
                    v-model.number="form.recurrenceInterval"
                    type="number"
                    min="1"
                    max="365"
                    class="w-full px-3 py-2 rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500"
                    style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);"
                  />
                </div>
                <div>
                  <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.maintenance.labelRecurrenceUntil') }}</label>
                  <DateTimePicker v-model="form.recurrenceUntil" mode="date" :placeholder="t('admin.maintenance.recurrenceUntilPlaceholder')" />
                </div>
              </div>
              <div v-if="form.recurrence === 'monthly'">
                <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.maintenance.labelMonthday') }}</label>
                <input
                  v-model.number="form.recurrenceMonthday"
                  type="number"
                  min="0"
                  max="31"
                  :placeholder="t('admin.maintenance.monthdayPlaceholder')"
                  class="w-full px-3 py-2 rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500"
                  style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);"
                />
              </div>
              <p class="text-xs" style="color: var(--text-color); opacity: 0.45;">
                {{
                  form.recurrence === 'weekly'
                    ? t('admin.maintenance.hintWeekly')
                    : t('admin.maintenance.hintAdvance')
                }}
                {{ t('admin.maintenance.hintUntilEmpty') }}
              </p>
            </template>
          </div>
          <div class="relative">
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.maintenance.labelAffectedServices') }}</label>
            <div
              class="w-full px-3 py-2 rounded-lg text-sm cursor-text focus-within:border-emerald-500 min-h-[38px] flex flex-wrap gap-1"
              style="border: 1px solid var(--button-border-color); background-color: var(--bg-color);"
              @click="showServiceDropdown = !showServiceDropdown"
            >
              <span
                v-for="id in selectedServices" :key="id"
                class="inline-flex items-center gap-1 px-2 py-0.5 bg-emerald-50 dark:bg-emerald-500/15 text-emerald-700 dark:text-emerald-400 rounded text-xs"
              >
                {{ services.find(s => s.id === id)?.name || id }}
              </span>
              <input
                v-model="serviceSearch"
                @focus="showServiceDropdown = true"
                @blur="delayBlur"
                type="text"
                :placeholder="t('admin.maintenance.searchServicePlaceholder')"
                class="border-0 outline-none text-sm flex-1 min-w-[80px] bg-transparent dark:placeholder-gray-500"
                style="color: var(--text-color);"
              />
            </div>
            <div
              v-if="showServiceDropdown"
              class="absolute z-10 mt-1 w-full rounded-lg shadow-lg max-h-48 overflow-y-auto thin-scroll p-1"
              style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);"
            >
              <div
                v-for="svc in filteredServices" :key="svc.id"
                @mousedown.prevent="toggleService(svc)"
                class="svc-opt flex items-center justify-between"
                :class="{ active: selectedServices.includes(svc.id) }"
              >
                <span>{{ svc.name }}</span>
                <span v-if="selectedServices.includes(svc.id)" class="text-emerald-500">&#10003;</span>
              </div>
              <div v-if="filteredServices.length === 0" class="px-3 py-2 text-sm" style="color: var(--text-color); opacity: 0.4;">
                {{ t('admin.maintenance.noMatchingService') }}
              </div>
            </div>
          </div>
        </div>
        <div class="flex justify-end gap-3 mt-6">
          <button @click="handleCloseMtn" class="btn-cancel px-4 py-2 text-sm rounded-lg">{{ t('admin.maintenance.cancel') }}</button>
          <button @click="save" class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium rounded-lg transition-colors">{{ t('admin.maintenance.save') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>
