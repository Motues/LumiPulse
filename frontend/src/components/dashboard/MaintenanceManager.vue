<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '../../api/client'
import type { Maintenance, Service } from '../../api/types'
import { useToast } from '../../composables/useToast'
import { useUnsavedChanges } from '../../composables/useUnsavedChanges'
import CustomSelect from './CustomSelect.vue'

const maintenances = ref<Maintenance[]>([])
const loading = ref(true)
const { show: toast } = useToast()
const showForm = ref(false)
const editing = ref<Maintenance | null>(null)
const form = ref({ title: '', description: '', scheduledStart: '', scheduledEnd: '', status: 'scheduled', affectedServices: '' })

const { markClean: cleanMtn, handleClose: closeMtn, restoreFromStorage: restoreMtn } = useUnsavedChanges(form as any, 'mtn_form')

// Timezone helpers: assume all times are CST (UTC+8)
const CST_OFFSET = '+08:00'

// datetime-local 输入框只接受 "YYYY-MM-DDTHH:mm" 形式（不允许带时区/秒），
// 因此回填表单时必须把后端存储的 ISO 时间转换成 CST 墙上时间。
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
  // 统一先归一化成 datetime-local，再补上秒和 CST 偏移，避免重复拼接时区
  const normalized = toDatetimeLocalValue(dt)
  if (!normalized) return ''
  return `${normalized}:00${CST_OFFSET}`
}

function formatCST(iso: string): string {
  if (!iso) return ''
  // If no timezone info in the string, assume CST
  const s = /[Z+-]/.test(iso) ? iso : iso + CST_OFFSET
  const d = new Date(s)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
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

const statusLabel: Record<string, string> = {
  scheduled: '计划中',
  in_progress: '进行中',
  completed: '已完成',
  cancelled: '已取消',
}

async function load() {
  loading.value = true
  try {
    const res = await api.getAdminMaintenances()
    maintenances.value = res.data
  } catch (e: any) {
    toast(e.message || '加载失败')
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
  const defaults = { title: '', description: '', scheduledStart: '', scheduledEnd: '', status: 'scheduled', affectedServices: '' }
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
  }
  try {
    if (editing.value) {
      await api.updateMaintenance(editing.value.id, payload)
    } else {
      await api.createMaintenance(payload)
    }
    showForm.value = false
    cleanMtn()
    toast(editing.value ? '更新成功' : '创建成功', 'success')
    load()
  } catch (e: any) {
    toast(e.message || '保存失败')
  }
}

async function remove(id: number) {
  if (!confirm('确定要删除吗？')) return
  try {
    await api.deleteMaintenance(id)
    toast('删除成功', 'success')
    load()
  } catch (e: any) {
    toast(e.message || '删除失败')
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
      <h2 class="text-lg font-bold" style="color: var(--text-color);">维护计划</h2>
      <button @click="openCreate" class="bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium px-4 py-2 rounded-lg flex items-center gap-1 transition-colors">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" /></svg>
        创建维护
      </button>
    </div>

    <div v-if="loading" class="text-center py-12" style="color: var(--text-color); opacity: 0.4;">加载中...</div>

    <div v-else class="rounded-xl" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
      <div class="overflow-x-auto">
        <table class="w-full text-left text-sm">
        <thead class="text-xs" style="color: var(--text-color); opacity: 0.4; background-color: var(--button-hover-color); opacity: 0.5; border-bottom: 1px solid var(--button-border-color);">
          <tr>
            <th class="px-6 py-3 font-medium">标题</th>
            <th class="px-6 py-3 font-medium">状态</th>
            <th class="px-6 py-3 font-medium">开始时间</th>
            <th class="px-6 py-3 font-medium">结束时间</th>
            <th class="px-6 py-3 font-medium">受影响服务</th>
            <th class="px-6 py-3 font-medium text-right">操作</th>
          </tr>
        </thead>
        <tbody class="divide-y">
          <tr v-for="m in maintenances" :key="m.id">
            <td class="px-6 py-4 font-bold" style="color: var(--text-color);">{{ m.title }}</td>
            <td class="px-6 py-4">
              <span :class="['inline-flex items-center px-2 py-1 rounded text-xs font-medium border', statusClass(m.status)]">
                {{ statusLabel[m.status] || m.status }}
              </span>
            </td>
            <td class="px-6 py-4 text-xs" style="color: var(--text-color); opacity: 0.5;">{{ formatCST(m.scheduledStart) }}</td>
            <td class="px-6 py-4 text-xs" style="color: var(--text-color); opacity: 0.5;">{{ formatCST(m.scheduledEnd) }}</td>
            <td class="px-6 py-4 text-xs max-w-40 truncate" style="color: var(--text-color); opacity: 0.5;">{{ affectedServiceNames(m.affectedServices) }}</td>
            <td class="px-6 py-4 text-right">
              <button @click="openEdit(m)" class="op-btn op-btn-edit mr-3" title="编辑">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                </svg>
              </button>
              <button @click="remove(m.id)" class="op-btn op-btn-delete" title="删除">
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
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">{{ editing ? '编辑维护' : '创建维护' }}</h3>
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">标题 *</label>
            <input v-model="form.title" class="w-full px-3 py-2 rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500" style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">描述</label>
            <textarea v-model="form.description" rows="2" class="w-full px-3 py-2 rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500" style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);"></textarea>
          </div>
          <div class="grid grid-cols-2 gap-4">
            <div>
              <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">开始时间 *</label>
              <input v-model="form.scheduledStart" type="datetime-local" class="w-full px-3 py-2 rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500" style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);" />
            </div>
            <div>
              <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">结束时间 *</label>
              <input v-model="form.scheduledEnd" type="datetime-local" class="w-full px-3 py-2 rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500" style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);" />
            </div>
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">状态</label>
            <CustomSelect v-model="form.status" :options="[{ label: '计划中', value: 'scheduled' }, { label: '进行中', value: 'in_progress' }, { label: '已完成', value: 'completed' }, { label: '已取消', value: 'cancelled' }]" />
          </div>
          <div class="relative">
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">受影响服务</label>
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
                placeholder="搜索服务..."
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
                无匹配服务
              </div>
            </div>
          </div>
        </div>
        <div class="flex justify-end gap-3 mt-6">
          <button @click="handleCloseMtn" class="btn-cancel px-4 py-2 text-sm rounded-lg">取消</button>
          <button @click="save" class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium rounded-lg transition-colors">保存</button>
        </div>
      </div>
    </div>
  </div>
</template>
