<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '../../api/client'
import type { Server, ProbeTask, ServiceDetail } from '../../api/types'
import { useToast } from '../../composables/useToast'
import { useI18n } from '../../composables/useI18n'
import CustomSelect from './CustomSelect.vue'

const { show: toast } = useToast()
const { t } = useI18n()

const servers = ref<Server[]>([])
const probeTasks = ref<ProbeTask[]>([])
const services = ref<ServiceDetail[]>([])
const loading = ref(true)

// Server form
const showServerForm = ref(false)
const editingServer = ref<Server | null>(null)
const serverForm = ref({ name: '', description: '', autoMerge: true, autoMergeThreshold: 1 })

// Probe task form (for both create and edit)
const showProbeForm = ref(false)
const editingProbe = ref<ProbeTask | null>(null)
const probeForm = ref({ serverId: null as number | null, triggerCount: 5, isActive: true })

// Create probe task form (needs serviceId)
const showCreateProbe = ref(false)
const createProbeForm = ref({ serviceId: 0, serverId: null as number | null, triggerCount: 5, isActive: true })

// Service search for probe creation
const serviceSearch = ref('')
const showServiceDropdown = ref(false)

const filteredServices = computed(() => {
  if (!serviceSearch.value) return services.value
  const q = serviceSearch.value.toLowerCase()
  return services.value.filter(s => s.name.toLowerCase().includes(q))
})

// Services that don't have a probe task yet
const availableServices = computed(() => {
  const probeServiceIds = new Set(probeTasks.value.map(t => t.serviceId))
  return services.value.filter(s => !probeServiceIds.has(s.id))
})

// Group probe tasks by server
const tasksByServer = computed(() => {
  const grouped: Record<string, ProbeTask[]> = {}
  grouped['__none__'] = []
  for (const s of servers.value) {
    grouped[String(s.id)] = []
  }
  for (const task of probeTasks.value) {
    const key = task.serverId ? String(task.serverId) : '__none__'
    if (!grouped[key]) grouped[key] = []
    grouped[key].push(task)
  }
  return grouped
})

function getServerName(id?: number): string {
  if (!id) return t('admin.probe.unboundServer')
  const s = servers.value.find(sv => sv.id === id)
  return s ? s.name : t('admin.probe.unknownServer')
}

async function load() {
  loading.value = true
  try {
    const [srvRes, taskRes, svcRes] = await Promise.all([
      api.getServers(),
      api.getProbeTasks(),
      api.getAdminServices(),
    ])
    servers.value = srvRes.data || []
    probeTasks.value = taskRes.data || []
    services.value = svcRes.data || []
  } catch (e: any) {
    toast(e.message || t('admin.probe.loadFailed'))
  } finally {
    loading.value = false
  }
}

// Server CRUD
function openCreateServer() {
  editingServer.value = null
  serverForm.value = { name: '', description: '', autoMerge: true, autoMergeThreshold: 1 }
  showServerForm.value = true
}

function openEditServer(svr: Server) {
  editingServer.value = svr
  serverForm.value = {
    name: svr.name,
    description: svr.description || '',
    autoMerge: svr.autoMerge,
    autoMergeThreshold: svr.autoMergeThreshold || 1,
  }
  showServerForm.value = true
}

async function saveServer() {
  try {
    if (editingServer.value) {
      await api.updateServer(editingServer.value.id, serverForm.value)
    } else {
      await api.createServer(serverForm.value)
    }
    showServerForm.value = false
    toast(editingServer.value ? t('admin.probe.updateSuccess') : t('admin.probe.createSuccess'), 'success')
    load()
  } catch (e: any) {
    toast(e.message || t('admin.probe.saveFailed'))
  }
}

async function removeServer(id: number) {
  if (!confirm(t('admin.probe.deleteServerConfirm'))) return
  try {
    await api.deleteServer(id)
    toast(t('admin.probe.deleteSuccess'), 'success')
    load()
  } catch (e: any) {
    toast(e.message || t('admin.probe.deleteFailed'))
  }
}

// Probe Task CRUD
function openEditProbe(task: ProbeTask) {
  editingProbe.value = task
  probeForm.value = {
    serverId: task.serverId || null,
    triggerCount: task.triggerCount,
    isActive: task.isActive,
  }
  showProbeForm.value = true
}

async function saveProbe() {
  if (!editingProbe.value) return
  try {
    await api.updateProbeTask(editingProbe.value.id, {
      serverId: probeForm.value.serverId,
      triggerCount: probeForm.value.triggerCount,
      isActive: probeForm.value.isActive,
    })
    showProbeForm.value = false
    toast(t('admin.probe.updateSuccess'), 'success')
    load()
  } catch (e: any) {
    toast(e.message || t('admin.probe.saveFailed'))
  }
}

async function removeProbe(id: number) {
  if (!confirm(t('admin.probe.deleteProbeConfirm'))) return
  try {
    await api.deleteProbeTask(id)
    toast(t('admin.probe.deleteSuccess'), 'success')
    load()
  } catch (e: any) {
    toast(e.message || t('admin.probe.deleteFailed'))
  }
}

async function toggleProbeActive(task: ProbeTask) {
  try {
    await api.updateProbeTask(task.id, {
      serverId: task.serverId,
      triggerCount: task.triggerCount,
      isActive: !task.isActive,
    })
    load()
  } catch (e: any) {
    toast(e.message || t('admin.probe.operationFailed'))
  }
}

// Create Probe Task
function openCreateProbe() {
  createProbeForm.value = { serviceId: 0, serverId: null, triggerCount: 5, isActive: true }
  serviceSearch.value = ''
  showCreateProbe.value = true
}

function selectCreateService(svc: { id: number; name: string }) {
  createProbeForm.value.serviceId = svc.id
  serviceSearch.value = svc.name
  showServiceDropdown.value = false
}

async function saveCreateProbe() {
  if (!createProbeForm.value.serviceId) {
    toast(t('admin.probe.selectServiceRequired'), 'error')
    return
  }
  try {
    await api.createProbeTask({
      serviceId: createProbeForm.value.serviceId,
      serverId: createProbeForm.value.serverId,
      triggerCount: createProbeForm.value.triggerCount,
    })
    showCreateProbe.value = false
    toast(t('admin.probe.createSuccess'), 'success')
    load()
  } catch (e: any) {
    toast(e.message || t('admin.probe.createFailed'))
  }
}

function delayBlur() {
  setTimeout(() => { showServiceDropdown.value = false }, 200)
}

onMounted(load)
</script>

<template>
  <div>
    <div class="flex justify-between items-center mb-4">
      <h2 class="text-lg font-bold" style="color: var(--text-color);">{{ t('admin.probe.title') }}</h2>
      <div class="flex gap-2">
        <button @click="openCreateProbe" class="bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium px-4 py-2 rounded-lg flex items-center gap-1 transition-colors">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" /></svg>
          {{ t('admin.probe.create') }}
        </button>
        <button @click="openCreateServer" class="border text-sm font-medium px-4 py-2 rounded-lg flex items-center gap-1 transition-colors hover:bg-[var(--button-hover-color)]" style="border-color: var(--button-border-color); color: var(--text-color);">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" /></svg>
          {{ t('admin.probe.addServer') }}
        </button>
      </div>
    </div>

    <div v-if="loading" class="text-center py-12" style="color: var(--text-color); opacity: 0.4;">{{ t('common.loading') }}</div>

    <div v-else class="space-y-6">
      <!-- Servers and their probe tasks -->
      <div v-for="svr in servers" :key="svr.id" class="rounded-xl" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <!-- Server header -->
        <div class="flex items-center justify-between px-6 py-3" style="border-bottom: 1px solid var(--button-border-color);">
          <div class="flex items-center gap-2">
            <svg class="w-5 h-5" style="color: var(--text-color); opacity: 0.5;" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" /></svg>
            <h3 class="font-bold" style="color: var(--text-color);">{{ svr.name }}</h3>
            <span v-if="svr.description" class="text-xs" style="color: var(--text-color); opacity: 0.4;">{{ svr.description }}</span>
            <span v-if="svr.autoMerge" class="text-xs px-1.5 py-0.5 rounded" style="background-color: var(--button-hover-color); color: var(--text-color); opacity: 0.5;">{{ t('admin.probe.autoMerge') }}</span>
          </div>
          <div class="flex items-center gap-1">
            <button @click="openEditServer(svr)" class="op-btn op-btn-edit p-1.5 rounded-lg" :title="t('admin.probe.editServer')">
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" /></svg>
            </button>
            <button @click="removeServer(svr.id)" class="op-btn op-btn-delete p-1.5 rounded-lg" :title="t('admin.probe.deleteServer')">
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg>
            </button>
          </div>
        </div>
        <!-- Probe tasks for this server -->
        <div v-if="tasksByServer[String(svr.id)]?.length > 0">
          <table class="w-full text-left text-sm">
            <thead class="text-xs" style="color: var(--text-color); opacity: 0.4; border-bottom: 1px solid var(--button-border-color);">
              <tr>
                <th class="px-6 py-2 font-medium">{{ t('admin.probe.colService') }}</th>
                <th class="px-6 py-2 font-medium">{{ t('admin.probe.colThreshold') }}</th>
                <th class="px-6 py-2 font-medium">{{ t('admin.probe.colStatus') }}</th>
                <th class="px-6 py-2 font-medium text-right">{{ t('admin.probe.colActions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y">
              <tr v-for="task in tasksByServer[String(svr.id)]" :key="task.id" class="hover:bg-[var(--button-hover-color)]">
                <td class="px-6 py-3 font-medium" style="color: var(--text-color);">{{ task.serviceName }}</td>
                <td class="px-6 py-3" style="color: var(--text-color); opacity: 0.5;">{{ t('admin.probe.consecutive', { n: task.triggerCount }) }}</td>
                <td class="px-6 py-3">
                  <button @click="toggleProbeActive(task)" :class="['inline-flex items-center px-2 py-0.5 rounded text-xs font-medium border cursor-pointer transition-colors', task.isActive ? 'text-emerald-600 bg-emerald-50 border-emerald-100 dark:text-emerald-400 dark:bg-emerald-500/15 dark:border-emerald-800' : 'text-gray-500 bg-gray-50 border-gray-100 dark:text-gray-400 dark:bg-gray-800 dark:border-gray-700']">
                    {{ task.isActive ? t('admin.probe.enabled') : t('admin.probe.disabled') }}
                  </button>
                </td>
                <td class="px-6 py-3 text-right">
                  <button @click="openEditProbe(task)" class="op-btn op-btn-edit mr-2" :title="t('admin.probe.edit')">
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" /></svg>
                  </button>
                  <button @click="removeProbe(task.id)" class="op-btn op-btn-delete" :title="t('admin.probe.delete')">
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg>
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="px-6 py-4 text-sm text-center" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.probe.noTasks') }}</div>
      </div>

      <!-- Unbound probe tasks (no server) -->
      <div v-if="tasksByServer['__none__']?.length > 0" class="rounded-xl" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <div class="flex items-center gap-2 px-6 py-3" style="border-bottom: 1px solid var(--button-border-color);">
          <svg class="w-5 h-5" style="color: var(--text-color); opacity: 0.5;" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" /></svg>
          <h3 class="font-bold" style="color: var(--text-color);">{{ t('admin.probe.unboundServer') }}</h3>
        </div>
        <table class="w-full text-left text-sm">
          <thead class="text-xs" style="color: var(--text-color); opacity: 0.4; border-bottom: 1px solid var(--button-border-color);">
            <tr>
              <th class="px-6 py-2 font-medium">{{ t('admin.probe.colService') }}</th>
              <th class="px-6 py-2 font-medium">{{ t('admin.probe.colThreshold') }}</th>
              <th class="px-6 py-2 font-medium">{{ t('admin.probe.colStatus') }}</th>
              <th class="px-6 py-2 font-medium text-right">{{ t('admin.probe.colActions') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y">
            <tr v-for="task in tasksByServer['__none__']" :key="task.id" class="hover:bg-[var(--button-hover-color)]">
              <td class="px-6 py-3 font-medium" style="color: var(--text-color);">{{ task.serviceName }}</td>
              <td class="px-6 py-3" style="color: var(--text-color); opacity: 0.5;">{{ t('admin.probe.consecutive', { n: task.triggerCount }) }}</td>
              <td class="px-6 py-3">
                <button @click="toggleProbeActive(task)" :class="['inline-flex items-center px-2 py-0.5 rounded text-xs font-medium border cursor-pointer transition-colors', task.isActive ? 'text-emerald-600 bg-emerald-50 border-emerald-100 dark:text-emerald-400 dark:bg-emerald-500/15 dark:border-emerald-800' : 'text-gray-500 bg-gray-50 border-gray-100 dark:text-gray-400 dark:bg-gray-800 dark:border-gray-700']">
                  {{ task.isActive ? t('admin.probe.enabled') : t('admin.probe.disabled') }}
                </button>
              </td>
              <td class="px-6 py-3 text-right">
                <button @click="openEditProbe(task)" class="op-btn op-btn-edit mr-2" :title="t('admin.probe.edit')">
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" /></svg>
                </button>
                <button @click="removeProbe(task.id)" class="op-btn op-btn-delete" :title="t('admin.probe.delete')">
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg>
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Empty state -->
      <div v-if="servers.length === 0 && probeTasks.length === 0" class="text-center py-12" style="color: var(--text-color); opacity: 0.4;">
        <p>{{ t('admin.probe.emptyTitle') }}</p>
        <p class="text-sm mt-1">{{ t('admin.probe.emptyHint') }}</p>
      </div>
    </div>

    <!-- Server Form Modal -->
    <div v-if="showServerForm" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="showServerForm = false">
      <div class="rounded-xl p-4 md:p-6 w-full max-w-lg mx-4" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">{{ editingServer ? t('admin.probe.editServer') : t('admin.probe.addServer') }}</h3>
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.probe.formName') }}</label>
            <input v-model="serverForm.name" class="input-field w-full px-3 py-2 rounded-lg text-sm" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.probe.formDescription') }}</label>
            <input v-model="serverForm.description" class="input-field w-full px-3 py-2 rounded-lg text-sm" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-2" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.probe.autoMerge') }}</label>
            <div class="flex items-center gap-3">
              <button
                @click="serverForm.autoMerge = !serverForm.autoMerge"
                :class="['relative inline-flex h-5 w-9 items-center rounded-full transition-colors', serverForm.autoMerge ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-gray-600']"
              >
                <span :class="['inline-block h-3.5 w-3.5 transform rounded-full bg-white transition-transform', serverForm.autoMerge ? 'translate-x-[18px]' : 'translate-x-0.5']" />
              </button>
              <span class="text-sm" style="color: var(--text-color); opacity: 0.5;">{{ t('admin.probe.autoMergeHint') }}</span>
            </div>
          </div>
          <div v-if="serverForm.autoMerge">
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.probe.mergeThreshold') }}</label>
            <input v-model.number="serverForm.autoMergeThreshold" type="number" min="1" class="input-field w-full px-3 py-2 rounded-lg text-sm" />
            <p class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.probe.mergeThresholdHint') }}</p>
          </div>
        </div>
        <div class="flex justify-end gap-3 mt-6">
          <button @click="showServerForm = false" class="btn-cancel px-4 py-2 text-sm rounded-lg">{{ t('admin.probe.cancel') }}</button>
          <button @click="saveServer" class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium rounded-lg transition-colors">{{ t('admin.probe.save') }}</button>
        </div>
      </div>
    </div>

    <!-- Create Probe Task Modal -->
    <div v-if="showCreateProbe" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="showCreateProbe = false">
      <div class="rounded-xl p-4 md:p-6 w-full max-w-lg mx-4" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">{{ t('admin.probe.create') }}</h3>
        <div class="space-y-4">
          <div class="relative">
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.probe.selectService') }}</label>
            <input
              v-model="serviceSearch"
              @focus="showServiceDropdown = true"
              @blur="delayBlur"
              type="text"
              :placeholder="t('admin.probe.searchService')"
              class="w-full px-3 py-2 rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500"
              style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);"
            />
            <div
              v-if="showServiceDropdown"
              class="absolute z-10 mt-1 w-full rounded-lg shadow-lg max-h-48 overflow-y-auto thin-scroll p-1"
              style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);"
            >
              <div
                v-for="svc in filteredServices"
                :key="svc.id"
                @mousedown.prevent="selectCreateService(svc)"
                class="svc-opt"
                :class="{ active: createProbeForm.serviceId === svc.id }"
              >
                {{ svc.name }}
              </div>
              <div v-if="filteredServices.length === 0" class="px-3 py-2 text-sm" style="color: var(--text-color); opacity: 0.4;">
                {{ availableServices.length === 0 ? t('admin.probe.allServicesHaveTasks') : t('admin.probe.noMatchingService') }}
              </div>
            </div>
            <div v-if="availableServices.length === 0 && services.length > 0" class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">
              {{ t('admin.probe.allServicesHaveTasks') }}
            </div>
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.probe.bindServerOptional') }}</label>
            <CustomSelect
              :model-value="createProbeForm.serverId?.toString() || ''"
              @update:model-value="(v: string) => createProbeForm.serverId = v ? Number(v) : null"
              :options="[{ label: t('admin.probe.notBound'), value: '' }, ...servers.map(s => ({ label: s.name, value: String(s.id) }))]"
            />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.probe.triggerThreshold') }}</label>
            <input v-model.number="createProbeForm.triggerCount" type="number" min="1" class="input-field w-full px-3 py-2 rounded-lg text-sm" />
          </div>
          <div class="flex items-center gap-2">
            <label class="text-sm font-medium" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.probe.enableProbe') }}</label>
            <button
              @click="createProbeForm.isActive = !createProbeForm.isActive"
              :class="['relative inline-flex h-5 w-9 items-center rounded-full transition-colors', createProbeForm.isActive ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-gray-600']"
            >
              <span :class="['inline-block h-3.5 w-3.5 transform rounded-full bg-white transition-transform', createProbeForm.isActive ? 'translate-x-[18px]' : 'translate-x-0.5']" />
            </button>
          </div>
        </div>
        <div class="flex justify-end gap-3 mt-6">
          <button @click="showCreateProbe = false" class="btn-cancel px-4 py-2 text-sm rounded-lg">{{ t('admin.probe.cancel') }}</button>
          <button @click="saveCreateProbe" :disabled="!createProbeForm.serviceId" class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium rounded-lg transition-colors disabled:opacity-40">{{ t('admin.probe.createAction') }}</button>
        </div>
      </div>
    </div>

    <!-- Probe Task Edit Modal -->
    <div v-if="showProbeForm && editingProbe" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="showProbeForm = false">
      <div class="rounded-xl p-4 md:p-6 w-full max-w-lg mx-4" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">{{ t('admin.probe.editProbeTitle') }}</h3>
        <div class="text-sm mb-4" style="color: var(--text-color); opacity: 0.5;">{{ t('admin.probe.serviceLabel', { name: editingProbe.serviceName }) }}</div>
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.probe.bindServer') }}</label>
            <CustomSelect
              :model-value="probeForm.serverId?.toString() || ''"
              @update:model-value="(v: string) => probeForm.serverId = v ? Number(v) : null"
              :options="[{ label: t('admin.probe.notBound'), value: '' }, ...servers.map(s => ({ label: s.name, value: String(s.id) }))]"
            />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.probe.triggerThreshold') }}</label>
            <input v-model.number="probeForm.triggerCount" type="number" min="1" class="input-field w-full px-3 py-2 rounded-lg text-sm" />
          </div>
          <div class="flex items-center gap-2">
            <label class="text-sm font-medium" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.probe.enableProbe') }}</label>
            <button
              @click="probeForm.isActive = !probeForm.isActive"
              :class="['relative inline-flex h-5 w-9 items-center rounded-full transition-colors', probeForm.isActive ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-gray-600']"
            >
              <span :class="['inline-block h-3.5 w-3.5 transform rounded-full bg-white transition-transform', probeForm.isActive ? 'translate-x-[18px]' : 'translate-x-0.5']" />
            </button>
          </div>
        </div>
        <div class="flex justify-end gap-3 mt-6">
          <button @click="showProbeForm = false" class="btn-cancel px-4 py-2 text-sm rounded-lg">{{ t('admin.probe.cancel') }}</button>
          <button @click="saveProbe" class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium rounded-lg transition-colors">{{ t('admin.probe.save') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>
