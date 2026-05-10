<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '../../api/client'
import type { Incident, Service } from '../../api/types'
import { useToast } from '../../composables/useToast'

const incidents = ref<Incident[]>([])
const loading = ref(true)
const { show: toast } = useToast()

// Incident CRUD
const showForm = ref(false)
const editing = ref<Incident | null>(null)
const form = ref({ title: '', impact: 'minor', status: 'investigating', serviceId: 0 })

// Service search
const services = ref<(Service & { uptime: number; latency: number })[]>([])
const serviceSearch = ref('')
const showServiceDropdown = ref(false)

const filteredServices = computed(() => {
  if (!serviceSearch.value) return services.value
  const q = serviceSearch.value.toLowerCase()
  return services.value.filter(s => s.name.toLowerCase().includes(q))
})

// Incident update
const showUpdate = ref(false)
const updateIncident = ref<Incident | null>(null)
const updateForm = ref({ status: 'investigating', content: '' })

const statusLabel: Record<string, string> = {
  investigating: '调查中',
  identified: '已确认',
  monitoring: '监控中',
  resolved: '已解决',
}

const impactClass = (s: string) => {
  switch (s) {
    case 'critical': return 'text-red-600 bg-red-50 border-red-100 dark:text-red-400 dark:bg-red-900/30 dark:border-red-800'
    case 'major': return 'text-orange-600 bg-orange-50 border-orange-100 dark:text-orange-400 dark:bg-orange-900/30 dark:border-orange-800'
    case 'minor': return 'text-yellow-600 bg-yellow-50 border-yellow-100 dark:text-yellow-400 dark:bg-yellow-900/30 dark:border-yellow-800'
    default: return 'text-gray-600 bg-gray-50 border-gray-100 dark:text-gray-400 dark:bg-gray-800 dark:border-gray-700'
  }
}

async function load() {
  loading.value = true
  try {
    const res = await api.getAdminIncidents(1, 50)
    incidents.value = res.data.incidents
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
  form.value = { title: '', impact: 'minor', status: 'investigating', serviceId: 0 }
  serviceSearch.value = ''
  showForm.value = true
}

function openEdit(inc: Incident) {
  editing.value = inc
  form.value = { title: inc.title, impact: inc.impact, status: inc.status, serviceId: inc.serviceId }
  const svc = services.value.find(s => s.id === inc.serviceId)
  serviceSearch.value = svc ? svc.name : ''
  showForm.value = true
}

function delayBlur() {
  setTimeout(() => { showServiceDropdown.value = false }, 200)
}

function selectService(svc: { id: number; name: string }) {
  form.value.serviceId = svc.id
  serviceSearch.value = svc.name
  showServiceDropdown.value = false
}

async function save() {
  try {
    if (editing.value) {
      await api.updateIncident(editing.value.id, form.value)
    } else {
      await api.createIncident(form.value)
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
    await api.deleteIncident(id)
    toast('删除成功', 'success')
    load()
  } catch (e: any) {
    toast(e.message || '删除失败')
  }
}

function openUpdate(inc: Incident) {
  updateIncident.value = inc
  updateForm.value = { status: inc.status, content: '' }
  showUpdate.value = true
}

async function saveUpdate() {
  if (!updateIncident.value) return
  try {
    await api.createIncidentUpdate(updateIncident.value.id, updateForm.value)
    showUpdate.value = false
    toast('更新成功', 'success')
    load()
  } catch (e: any) {
    toast(e.message || '更新失败')
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
      <h2 class="text-lg font-bold text-gray-900 dark:text-gray-100">事件管理</h2>
      <button @click="openCreate" class="bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium px-4 py-2 rounded-lg flex items-center gap-1 transition-colors">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" /></svg>
        创建事件
      </button>
    </div>

    <div v-if="loading" class="text-center py-12 text-gray-400 dark:text-gray-500">加载中...</div>

    <div v-else class="bg-white dark:bg-gray-900 rounded-xl border border-gray-100 dark:border-gray-800 shadow-sm">
      <div class="overflow-x-auto">
        <table class="w-full text-left text-sm">
        <thead class="text-xs text-gray-400 dark:text-gray-500 bg-gray-50/50 dark:bg-gray-800/50 border-b border-gray-100 dark:border-gray-800">
          <tr>
            <th class="px-6 py-3 font-medium">标题</th>
            <th class="px-6 py-3 font-medium">影响</th>
            <th class="px-6 py-3 font-medium">状态</th>
            <th class="px-6 py-3 font-medium">时间</th>
            <th class="px-6 py-3 font-medium text-right">操作</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-50 dark:divide-gray-800">
          <tr v-for="inc in incidents" :key="inc.id" class="hover:bg-gray-50/50 dark:hover:bg-gray-800/50">
            <td class="px-6 py-4 font-bold text-gray-900 dark:text-gray-100">{{ inc.title }}</td>
            <td class="px-6 py-4">
              <span :class="['inline-flex items-center px-2 py-1 rounded text-xs font-medium border', impactClass(inc.impact)]">
                {{ inc.impact === 'critical' ? '严重' : inc.impact === 'major' ? '较大' : '轻微' }}
              </span>
            </td>
            <td class="px-6 py-4 text-gray-500 dark:text-gray-400">{{ statusLabel[inc.status] || inc.status }}</td>
            <td class="px-6 py-4 text-gray-500 dark:text-gray-400 text-xs">{{ new Date(inc.createdAt).toLocaleString('zh-CN') }}</td>
            <td class="px-6 py-4 text-right">
              <button @click="openEdit(inc)" class="text-gray-400 hover:text-blue-500 dark:text-gray-500 dark:hover:text-blue-400 transition-colors mr-3" title="编辑">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                </svg>
              </button>
              <button @click="openUpdate(inc)" class="text-gray-400 hover:text-green-500 dark:text-gray-500 dark:hover:text-green-400 transition-colors mr-3" title="事件更新">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              </button>
              <button @click="remove(inc.id)" class="text-gray-400 hover:text-red-500 dark:text-gray-500 dark:hover:text-red-400 transition-colors" title="删除">
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

    <!-- Incident Form -->
    <div v-if="showForm" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="showForm = false">
      <div class="bg-white dark:bg-gray-900 rounded-xl p-4 md:p-6 w-full max-w-lg mx-4 shadow-xl">
        <h3 class="text-lg font-bold text-gray-900 dark:text-gray-100 mb-4">{{ editing ? '编辑事件' : '创建事件' }}</h3>
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">标题 *</label>
            <input v-model="form.title" class="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 dark:bg-gray-800 dark:text-gray-100" />
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">影响等级 *</label>
            <select v-model="form.impact" class="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 dark:bg-gray-800 dark:text-gray-100">
              <option value="minor">轻微</option>
              <option value="major">较大</option>
              <option value="critical">严重</option>
            </select>
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">状态</label>
            <select v-model="form.status" class="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 dark:bg-gray-800 dark:text-gray-100">
              <option value="investigating">调查中</option>
              <option value="identified">已确认</option>
              <option value="monitoring">监控中</option>
              <option value="resolved">已解决</option>
            </select>
          </div>
          <div class="relative">
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">服务 *</label>
            <input
              v-model="serviceSearch"
              @focus="showServiceDropdown = true"
              @blur="delayBlur"
              type="text"
              placeholder="搜索服务..."
              class="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 dark:bg-gray-800 dark:text-gray-100"
            />
            <div
              v-if="showServiceDropdown"
              class="absolute z-10 mt-1 w-full bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg max-h-48 overflow-y-auto"
            >
              <div
                v-for="svc in filteredServices"
                :key="svc.id"
                @mousedown.prevent="selectService(svc)"
                class="px-3 py-2 text-sm cursor-pointer hover:bg-emerald-50 dark:hover:bg-emerald-900/30"
                :class="{ 'bg-emerald-50 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-400': form.serviceId === svc.id }"
              >
                {{ svc.name }}
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

    <!-- Incident Update Form -->
    <div v-if="showUpdate && updateIncident" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="showUpdate = false">
      <div class="bg-white dark:bg-gray-900 rounded-xl p-4 md:p-6 w-full max-w-lg mx-4 shadow-xl">
        <h3 class="text-lg font-bold text-gray-900 dark:text-gray-100 mb-4">事件更新</h3>
        <div class="text-sm text-gray-500 dark:text-gray-400 mb-4">更新事件: {{ updateIncident.title }}</div>
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">状态 *</label>
            <select v-model="updateForm.status" class="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 dark:bg-gray-800 dark:text-gray-100">
              <option value="investigating">调查中</option>
              <option value="identified">已确认</option>
              <option value="monitoring">监控中</option>
              <option value="resolved">已解决</option>
            </select>
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">内容 *</label>
            <textarea v-model="updateForm.content" rows="3" class="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 dark:bg-gray-800 dark:text-gray-100"></textarea>
          </div>
        </div>
        <div class="flex justify-end gap-3 mt-6">
          <button @click="showUpdate = false" class="px-4 py-2 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200">取消</button>
          <button @click="saveUpdate" class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium rounded-lg transition-colors">保存</button>
        </div>
      </div>
    </div>
  </div>
</template>
