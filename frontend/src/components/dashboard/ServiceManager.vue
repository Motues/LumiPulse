<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { api } from '../../api/client'
import type { ServiceDetail } from '../../api/types'
import ServiceDetailComponent from './ServiceDetail.vue'
import { useToast } from '../../composables/useToast'
import { useUnsavedChanges } from '../../composables/useUnsavedChanges'

const props = defineProps<{
  pendingServiceId?: number
}>()

const emit = defineEmits<{
  opened: []
}>()

const { show: toast } = useToast()

const services = ref<ServiceDetail[]>([])
const loading = ref(true)
const showForm = ref(false)
const editing = ref<ServiceDetail | null>(null)
const form = ref({ name: '', url: '', description: '', type: 'http', interval: 60 })
const dragIndex = ref<number | null>(null)
const searchQuery = ref('')

// Detail view
const selectedService = ref<ServiceDetail | null>(null)

const filteredServices = computed(() => {
  const q = searchQuery.value.toLowerCase().trim()
  if (!q) return services.value
  return services.value.filter(s =>
    s.name.toLowerCase().includes(q) ||
    s.url.toLowerCase().includes(q) ||
    (s.description && s.description.toLowerCase().includes(q))
  )
})

const statusClass = (s: string) => {
  switch (s) {
    case 'operational': return 'text-emerald-600 bg-emerald-50 border-emerald-100 dark:text-emerald-400 dark:bg-emerald-900/30 dark:border-emerald-800'
    case 'degraded': return 'text-yellow-600 bg-yellow-50 border-yellow-100 dark:text-yellow-400 dark:bg-yellow-900/30 dark:border-yellow-800'
    case 'outage': return 'text-red-600 bg-red-50 border-red-100 dark:text-red-400 dark:bg-red-900/30 dark:border-red-800'
    default: return 'text-gray-600 bg-gray-50 border-gray-100 dark:text-gray-400 dark:bg-gray-800 dark:border-gray-700'
  }
}

async function load() {
  loading.value = true
  try {
    const res = await api.getAdminServices()
    services.value = res.data
    if (props.pendingServiceId) {
      const svc = services.value.find(s => s.id === props.pendingServiceId)
      if (svc) {
        selectedService.value = svc
        emit('opened')
      }
    }
  } catch (e: any) {
    toast(e.message || '加载失败')
  } finally {
    loading.value = false
  }
}

const { markClean: cleanSvc, handleClose: closeSvc, restoreFromStorage: restoreSvc } = useUnsavedChanges(form as any, 'svc_form')

function openCreate() {
  editing.value = null
  const defaults = { name: '', url: '', description: '', type: 'http', interval: 60 }
  form.value = { ...defaults }
  restoreSvc()
  cleanSvc()
  showForm.value = true
}

function openEdit(svc: ServiceDetail) {
  editing.value = svc
  form.value = {
    name: svc.name,
    url: svc.url,
    description: svc.description || '',
    type: svc.type,
    interval: svc.interval,
  }
  cleanSvc()
  showForm.value = true
}

function handleCloseSvc() {
  if (closeSvc()) showForm.value = false
}

async function save() {
  try {
    if (editing.value) {
      await api.updateService(editing.value.id, form.value)
    } else {
      await api.createService(form.value)
    }
    showForm.value = false
    cleanSvc()
    toast(editing.value ? '更新成功' : '创建成功', 'success')
    load()
  } catch (e: any) {
    toast(e.message || '保存失败')
  }
}

async function remove(id: number) {
  if (!confirm('确定要删除吗？')) return
  try {
    await api.deleteService(id)
    toast('删除成功', 'success')
    load()
  } catch (e: any) {
    toast(e.message || '删除失败')
  }
}

async function moveUp(index: number) {
  if (index <= 0) return
  const tmp = services.value[index]
  services.value[index] = services.value[index - 1]
  services.value[index - 1] = tmp
  await saveOrder()
}

async function moveDown(index: number) {
  if (index >= services.value.length - 1) return
  const tmp = services.value[index]
  services.value[index] = services.value[index + 1]
  services.value[index + 1] = tmp
  await saveOrder()
}

function onDragStart(index: number) {
  dragIndex.value = index
}

function onDragOver(e: DragEvent) {
  e.preventDefault()
}

function onDrop(index: number) {
  if (dragIndex.value === null || dragIndex.value === index) return
  const item = services.value.splice(dragIndex.value, 1)[0]
  services.value.splice(index, 0, item)
  dragIndex.value = null
  saveOrder()
}

async function saveOrder() {
  const order = services.value.map((svc, i) => ({ id: svc.id, sortOrder: i }))
  try {
    await api.reorderServices(order)
    toast('排序已保存', 'success')
  } catch (e: any) {
    toast(e.message || '保存排序失败')
    load()
  }
}

onMounted(load)

watch(() => props.pendingServiceId, (id) => {
  if (id && services.value.length > 0) {
    const svc = services.value.find(s => s.id === id)
    if (svc) {
      selectedService.value = svc
      emit('opened')
    }
  }
})
</script>

<template>
  <div>
    <!-- Detail view -->
    <ServiceDetailComponent
      v-if="selectedService"
      :service="selectedService"
      @back="selectedService = null"
    />

    <!-- List view -->
    <template v-else>
    <div class="flex justify-between items-center mb-4">
      <h2 class="text-lg font-bold text-gray-900 dark:text-gray-100">服务管理</h2>
      <button @click="openCreate" class="bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium px-4 py-2 rounded-lg flex items-center gap-1 transition-colors">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" /></svg>
        添加服务
      </button>
    </div>

    <!-- Search -->
    <div v-if="!loading" class="mb-4">
      <input
        v-model="searchQuery"
        type="text"
        placeholder="搜索服务名称或 URL..."
        class="w-full px-4 py-2 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 dark:bg-gray-800 dark:text-gray-100"
      />
    </div>

    <div v-if="loading" class="text-center py-12 text-gray-400 dark:text-gray-500">加载中...</div>

    <div v-else class="bg-white dark:bg-gray-900 rounded-xl border border-gray-100 dark:border-gray-800 shadow-sm">
      <div class="overflow-x-auto">
        <table class="w-full text-left text-sm">
        <thead class="text-xs text-gray-400 dark:text-gray-500 bg-gray-50/50 dark:bg-gray-800/50 border-b border-gray-100 dark:border-gray-800">
          <tr>
            <th class="px-2 py-3 w-6"></th>
            <th class="px-6 py-3 font-medium">名称</th>
            <th class="px-6 py-3 font-medium">URL</th>
            <th class="px-6 py-3 font-medium">类型</th>
            <th class="px-6 py-3 font-medium">状态</th>
            <th class="px-6 py-3 font-medium">在线率</th>
            <th class="px-6 py-3 font-medium">延迟</th>
            <th class="px-6 py-3 font-medium text-right">操作</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-50 dark:divide-gray-800">
          <tr
            v-for="(svc, idx) in filteredServices" :key="svc.id"
            :draggable="true"
            @dragstart="onDragStart(idx)"
            @dragover="onDragOver"
            @drop="onDrop(idx)"
            class="hover:bg-gray-50/50 dark:hover:bg-gray-800/50"
            :class="{ 'opacity-50': dragIndex === idx }"
          >
            <td class="px-2 py-4 cursor-grab text-gray-300 dark:text-gray-600 hover:text-gray-500 dark:hover:text-gray-400">
              <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 24 24"><path d="M8 6h2v2H8V6zm6 0h2v2h-2V6zM8 11h2v2H8v-2zm6 0h2v2h-2v-2zm-6 5h2v2H8v-2zm6 0h2v2h-2v-2z"/></svg>
            </td>
            <td class="px-6 py-4 font-bold text-gray-900 dark:text-gray-100 cursor-pointer hover:text-emerald-600 dark:hover:text-emerald-400" @click="selectedService = svc">{{ svc.name }}</td>
            <td class="px-6 py-4 text-gray-500 dark:text-gray-400 max-w-[200px] truncate">{{ svc.url }}</td>
            <td class="px-6 py-4 text-gray-500 dark:text-gray-400">{{ svc.type }}</td>
            <td class="px-6 py-4">
              <span :class="['inline-flex items-center gap-1.5 px-2 py-1 rounded text-xs font-medium border', statusClass(svc.status)]">
                {{ svc.status === 'operational' ? '正常' : svc.status === 'degraded' ? '性能下降' : '故障' }}
              </span>
            </td>
            <td class="px-6 py-4 text-gray-500 dark:text-gray-400">{{ svc.uptime.toFixed(2) }}%</td>
            <td class="px-6 py-4 text-gray-500 dark:text-gray-400">{{ svc.latency }}ms</td>
            <td class="px-6 py-4 text-right whitespace-nowrap">
              <button @click="moveUp(idx)" :disabled="idx === 0" class="text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300 transition-colors mr-1 disabled:opacity-30 disabled:cursor-not-allowed" title="上移">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M5 15l7-7 7 7" /></svg>
              </button>
              <button @click="moveDown(idx)" :disabled="idx === services.length - 1" class="text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300 transition-colors mr-2 disabled:opacity-30 disabled:cursor-not-allowed" title="下移">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" /></svg>
              </button>
              <button @click="openEdit(svc)" class="text-gray-400 hover:text-blue-500 dark:text-gray-500 dark:hover:text-blue-400 transition-colors mr-2" title="编辑">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                </svg>
              </button>
              <button @click="remove(svc.id)" class="text-gray-400 hover:text-red-500 dark:text-gray-500 dark:hover:text-red-400 transition-colors" title="删除">
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
    </template>

    <!-- Form Modal -->
    <div v-if="showForm" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="handleCloseSvc">
      <div class="bg-white dark:bg-gray-900 rounded-xl p-4 md:p-6 w-full max-w-lg mx-4 shadow-xl">
        <h3 class="text-lg font-bold text-gray-900 dark:text-gray-100 mb-4">{{ editing ? '编辑服务' : '添加服务' }}</h3>
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">名称 *</label>
            <input v-model="form.name" class="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 dark:bg-gray-800 dark:text-gray-100" />
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">URL *</label>
            <input v-model="form.url" class="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 dark:bg-gray-800 dark:text-gray-100" />
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">描述</label>
            <input v-model="form.description" class="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 dark:bg-gray-800 dark:text-gray-100" />
          </div>
          <div class="grid grid-cols-2 gap-4">
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">类型</label>
              <select v-model="form.type" class="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 dark:bg-gray-800 dark:text-gray-100">
                <option value="http">HTTP</option>
                <option value="tcp">TCP</option>
                <option value="ping">Ping</option>
              </select>
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">间隔 (秒)</label>
              <input v-model.number="form.interval" type="number" class="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 dark:bg-gray-800 dark:text-gray-100" />
            </div>
          </div>
        </div>
        <div class="flex justify-end gap-3 mt-6">
          <button @click="handleCloseSvc" class="px-4 py-2 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200">取消</button>
          <button @click="save" class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium rounded-lg transition-colors">保存</button>
        </div>
      </div>
    </div>
  </div>
</template>
