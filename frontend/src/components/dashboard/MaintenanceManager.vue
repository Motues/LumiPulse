<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '../../api/client'
import type { Maintenance, Service } from '../../api/types'
import { useToast } from '../../composables/useToast'

const maintenances = ref<Maintenance[]>([])
const loading = ref(true)
const { show: toast } = useToast()
const showForm = ref(false)
const editing = ref<Maintenance | null>(null)
const form = ref({ title: '', description: '', scheduledStart: '', scheduledEnd: '', status: 'scheduled', affectedServices: '' })

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
  form.value = { title: '', description: '', scheduledStart: '', scheduledEnd: '', status: 'scheduled', affectedServices: '' }
  selectedServices.value = []
  serviceSearch.value = ''
  showForm.value = true
}

function openEdit(m: Maintenance) {
  editing.value = m
  form.value = {
    title: m.title,
    description: m.description || '',
    scheduledStart: m.scheduledStart,
    scheduledEnd: m.scheduledEnd,
    status: m.status,
    affectedServices: m.affectedServices || '',
  }
  selectedServices.value = m.affectedServices ? m.affectedServices.split(',').map(Number) : []
  serviceSearch.value = ''
  showForm.value = true
}

async function save() {
  form.value.affectedServices = selectedServices.value.join(',')
  try {
    if (editing.value) {
      await api.updateMaintenance(editing.value.id, form.value)
    } else {
      await api.createMaintenance(form.value)
    }
    showForm.value = false
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
      <h2 class="text-lg font-bold text-gray-900 dark:text-gray-100">维护计划</h2>
      <button @click="openCreate" class="bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium px-4 py-2 rounded-lg flex items-center gap-1 transition-colors">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" /></svg>
        创建维护
      </button>
    </div>

    <div v-if="loading" class="text-center py-12 text-gray-400 dark:text-gray-500">加载中...</div>

    <div v-else class="bg-white dark:bg-gray-900 rounded-xl border border-gray-100 dark:border-gray-800 shadow-sm">
      <div class="overflow-x-auto">
        <table class="w-full text-left text-sm">
        <thead class="text-xs text-gray-400 dark:text-gray-500 bg-gray-50/50 dark:bg-gray-800/50 border-b border-gray-100 dark:border-gray-800">
          <tr>
            <th class="px-6 py-3 font-medium">标题</th>
            <th class="px-6 py-3 font-medium">状态</th>
            <th class="px-6 py-3 font-medium">开始时间</th>
            <th class="px-6 py-3 font-medium">结束时间</th>
            <th class="px-6 py-3 font-medium">受影响服务</th>
            <th class="px-6 py-3 font-medium text-right">操作</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-50 dark:divide-gray-800">
          <tr v-for="m in maintenances" :key="m.id" class="hover:bg-gray-50/50 dark:hover:bg-gray-800/50">
            <td class="px-6 py-4 font-bold text-gray-900 dark:text-gray-100">{{ m.title }}</td>
            <td class="px-6 py-4">
              <span :class="['inline-flex items-center px-2 py-1 rounded text-xs font-medium border', statusClass(m.status)]">
                {{ statusLabel[m.status] || m.status }}
              </span>
            </td>
            <td class="px-6 py-4 text-xs text-gray-500 dark:text-gray-400">{{ new Date(m.scheduledStart).toLocaleString('zh-CN') }}</td>
            <td class="px-6 py-4 text-xs text-gray-500 dark:text-gray-400">{{ new Date(m.scheduledEnd).toLocaleString('zh-CN') }}</td>
            <td class="px-6 py-4 text-xs text-gray-500 dark:text-gray-400 max-w-40 truncate">{{ affectedServiceNames(m.affectedServices) }}</td>
            <td class="px-6 py-4 text-right">
              <button @click="openEdit(m)" class="text-gray-400 hover:text-blue-500 dark:text-gray-500 dark:hover:text-blue-400 transition-colors mr-3" title="编辑">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                </svg>
              </button>
              <button @click="remove(m.id)" class="text-gray-400 hover:text-red-500 dark:text-gray-500 dark:hover:text-red-400 transition-colors" title="删除">
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
    <div v-if="showForm" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="showForm = false">
      <div class="bg-white dark:bg-gray-900 rounded-xl p-4 md:p-6 w-full max-w-lg mx-4 shadow-xl">
        <h3 class="text-lg font-bold text-gray-900 dark:text-gray-100 mb-4">{{ editing ? '编辑维护' : '创建维护' }}</h3>
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">标题 *</label>
            <input v-model="form.title" class="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 dark:bg-gray-800 dark:text-gray-100" />
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">描述</label>
            <textarea v-model="form.description" rows="2" class="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 dark:bg-gray-800 dark:text-gray-100"></textarea>
          </div>
          <div class="grid grid-cols-2 gap-4">
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">开始时间 *</label>
              <input v-model="form.scheduledStart" type="datetime-local" class="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 dark:bg-gray-800 dark:text-gray-100" />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">结束时间 *</label>
              <input v-model="form.scheduledEnd" type="datetime-local" class="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 dark:bg-gray-800 dark:text-gray-100" />
            </div>
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">状态</label>
            <select v-model="form.status" class="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 dark:bg-gray-800 dark:text-gray-100">
              <option value="scheduled">计划中</option>
              <option value="in_progress">进行中</option>
              <option value="completed">已完成</option>
              <option value="cancelled">已取消</option>
            </select>
          </div>
          <div class="relative">
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">受影响服务</label>
            <div
              class="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg text-sm cursor-text focus-within:border-emerald-500 min-h-[38px] flex flex-wrap gap-1 dark:bg-gray-800"
              @click="showServiceDropdown = !showServiceDropdown"
            >
              <span
                v-for="id in selectedServices" :key="id"
                class="inline-flex items-center gap-1 px-2 py-0.5 bg-emerald-50 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-400 rounded text-xs"
              >
                {{ services.find(s => s.id === id)?.name || id }}
              </span>
              <input
                v-model="serviceSearch"
                @focus="showServiceDropdown = true"
                @blur="delayBlur"
                type="text"
                placeholder="搜索服务..."
                class="border-0 outline-none text-sm flex-1 min-w-[80px] bg-transparent dark:text-gray-100 dark:placeholder-gray-500"
              />
            </div>
            <div
              v-if="showServiceDropdown"
              class="absolute z-10 mt-1 w-full bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg max-h-48 overflow-y-auto"
            >
              <div
                v-for="svc in filteredServices" :key="svc.id"
                @mousedown.prevent="toggleService(svc)"
                class="flex items-center justify-between px-3 py-2 text-sm cursor-pointer hover:bg-emerald-50 dark:hover:bg-emerald-900/30 dark:text-gray-200"
              >
                <span>{{ svc.name }}</span>
                <span v-if="selectedServices.includes(svc.id)" class="text-emerald-500 dark:text-emerald-400">✓</span>
              </div>
              <div v-if="filteredServices.length === 0" class="px-3 py-2 text-sm text-gray-400 dark:text-gray-500">
                无匹配服务
              </div>
            </div>
          </div>
        </div>
        <div class="flex justify-end gap-3 mt-6">
          <button @click="showForm = false" class="px-4 py-2 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200">取消</button>
          <button @click="save" class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium rounded-lg transition-colors">保存</button>
        </div>
      </div>
    </div>
  </div>
</template>
