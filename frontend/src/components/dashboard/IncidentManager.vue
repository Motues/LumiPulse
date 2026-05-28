<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { api } from '../../api/client'
import type { Incident, Service } from '../../api/types'
import IncidentDetail from './IncidentDetail.vue'
import CustomSelect from './CustomSelect.vue'
import { useToast } from '../../composables/useToast'
import { useUnsavedChanges } from '../../composables/useUnsavedChanges'

const incidents = ref<Incident[]>([])
const loading = ref(true)
const page = ref(1)
const totalPage = ref(0)
const limit = 15
const { show: toast } = useToast()

// Detail view
const selectedIncident = ref<Incident | null>(null)

function getServiceName(serviceId: number): string {
  const svc = services.value.find(s => s.id === serviceId)
  return svc ? svc.name : ''
}

function handleDetailUpdated() {
  load().then(() => {
    if (selectedIncident.value) {
      const updated = incidents.value.find(i => i.id === selectedIncident.value!.id)
      if (updated) selectedIncident.value = updated
    }
  })
}

function handleDetailDeleted() {
  selectedIncident.value = null
  load()
}

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
    const res = await api.getAdminIncidents(page.value, limit)
    incidents.value = res.data.incidents
    totalPage.value = res.data.pagination.totalPage
  } catch (e: any) {
    toast(e.message || '加载失败')
  } finally {
    loading.value = false
  }
}

function goPage(p: number) {
  if (p < 1 || p > totalPage.value) return
  page.value = p
  load()
}

async function loadServices() {
  try {
    const res = await api.getAdminServices()
    services.value = res.data
  } catch {
    // silent
  }
}

const { markClean: cleanInc, handleClose: closeInc, restoreFromStorage: restoreInc } = useUnsavedChanges(form as any, 'inc_form')

// Track dirty state for incident update form
const updateDirty = ref(false)
watch(updateForm, (n, o) => { if (o.content !== undefined) updateDirty.value = true }, { deep: true })

function openCreate() {
  editing.value = null
  const defaults = { title: '', impact: 'minor', status: 'investigating', serviceId: 0 }
  form.value = { ...defaults }
  restoreInc()
  cleanInc()
  showForm.value = true
}

function openEdit(inc: Incident) {
  editing.value = inc
  form.value = { title: inc.title, impact: inc.impact, status: inc.status, serviceId: inc.serviceId }
  const svc = services.value.find(s => s.id === inc.serviceId)
  serviceSearch.value = svc ? svc.name : ''
  cleanInc()
  showForm.value = true
}

function handleCloseInc() {
  if (closeInc()) showForm.value = false
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
    cleanInc()
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

function handleCloseUpdate() {
  if (updateDirty.value && !confirm('有未保存的更改，确定要关闭吗？')) return
  showUpdate.value = false
}

async function saveUpdate() {
  if (!updateIncident.value) return
  try {
    await api.createIncidentUpdate(updateIncident.value.id, updateForm.value)
    showUpdate.value = false
    updateDirty.value = false
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
    <!-- Detail view -->
    <IncidentDetail
      v-if="selectedIncident"
      :incident="selectedIncident"
      :service-name="getServiceName(selectedIncident.serviceId)"
      @back="selectedIncident = null"
      @updated="handleDetailUpdated"
      @deleted="handleDetailDeleted"
    />

    <!-- List view -->
    <template v-else>
    <div class="flex justify-between items-center mb-4">
      <h2 class="text-lg font-bold" style="color: var(--text-color);">事件管理</h2>
      <button @click="openCreate" class="bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium px-4 py-2 rounded-lg flex items-center gap-1 transition-colors">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" /></svg>
        创建事件
      </button>
    </div>

    <div v-if="loading" class="text-center py-12" style="color: var(--text-color); opacity: 0.4;">加载中...</div>

    <div v-else class="rounded-xl" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
      <div class="overflow-x-auto">
        <table class="w-full text-left text-sm">
        <thead class="text-xs" style="color: var(--text-color); opacity: 0.4; background-color: var(--button-hover-color); opacity: 0.5; border-bottom: 1px solid var(--button-border-color);">
          <tr>
            <th class="px-6 py-3 font-medium">标题</th>
            <th class="px-6 py-3 font-medium">影响</th>
            <th class="px-6 py-3 font-medium">状态</th>
            <th class="px-6 py-3 font-medium">时间</th>
            <th class="px-6 py-3 font-medium text-right">操作</th>
          </tr>
        </thead>
        <tbody class="divide-y">
          <tr v-for="inc in incidents" :key="inc.id" @click="selectedIncident = inc" class="cursor-pointer hover:bg-[var(--button-hover-color)]">
            <td class="px-6 py-4 font-bold" style="color: var(--text-color);">{{ inc.title }}</td>
            <td class="px-6 py-4">
              <span :class="['inline-flex items-center px-2 py-1 rounded text-xs font-medium border', impactClass(inc.impact)]">
                {{ inc.impact === 'critical' ? '严重' : inc.impact === 'major' ? '较大' : '轻微' }}
              </span>
            </td>
            <td class="px-6 py-4" style="color: var(--text-color); opacity: 0.5;">{{ statusLabel[inc.status] || inc.status }}</td>
            <td class="px-6 py-4 text-xs" style="color: var(--text-color); opacity: 0.5;">{{ new Date(inc.createdAt).toLocaleString('zh-CN') }}</td>
            <td class="px-6 py-4 text-right">
              <button @click.stop="openEdit(inc)" class="op-btn op-btn-edit mr-3" title="编辑">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                </svg>
              </button>
              <button @click.stop="openUpdate(inc)" class="op-btn mr-3" title="事件更新">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              </button>
              <button @click.stop="remove(inc.id)" class="op-btn op-btn-delete" title="删除">
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

    <!-- Pagination -->
    <div v-if="totalPage > 1" class="flex items-center justify-center gap-2 mt-4">
      <button
        @click="goPage(page - 1)"
        :disabled="page <= 1"
        class="px-3 py-1.5 text-sm rounded-lg disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
        style="border: 1px solid var(--button-border-color); color: var(--text-color); opacity: 0.6;"
      >
        上一页
      </button>
      <span class="text-sm px-3" style="color: var(--text-color); opacity: 0.5;">
        第 {{ page }} / {{ totalPage }} 页
      </span>
      <button
        @click="goPage(page + 1)"
        :disabled="page >= totalPage"
        class="px-3 py-1.5 text-sm rounded-lg disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
        style="border: 1px solid var(--button-border-color); color: var(--text-color); opacity: 0.6;"
      >
        下一页
      </button>
    </div>
    </template>

    <!-- Incident Form -->
    <div v-if="showForm" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="handleCloseInc">
      <div class="rounded-xl p-4 md:p-6 w-full max-w-lg mx-4" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">{{ editing ? '编辑事件' : '创建事件' }}</h3>
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">标题 *</label>
            <input v-model="form.title" class="w-full px-3 py-2 rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500" style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">影响等级 *</label>
            <CustomSelect v-model="form.impact" :options="[{ label: '轻微', value: 'minor' }, { label: '较大', value: 'major' }, { label: '严重', value: 'critical' }]" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">状态</label>
            <CustomSelect v-model="form.status" :options="[{ label: '调查中', value: 'investigating' }, { label: '已确认', value: 'identified' }, { label: '监控中', value: 'monitoring' }, { label: '已解决', value: 'resolved' }]" />
          </div>
          <div class="relative">
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">服务 *</label>
            <input
              v-model="serviceSearch"
              @focus="showServiceDropdown = true"
              @blur="delayBlur"
              type="text"
              placeholder="搜索服务..."
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
                @mousedown.prevent="selectService(svc)"
                class="svc-opt"
                :class="{ active: form.serviceId === svc.id }"
              >
                {{ svc.name }}
              </div>
              <div v-if="filteredServices.length === 0" class="px-3 py-2 text-sm" style="color: var(--text-color); opacity: 0.4;">
                无匹配服务
              </div>
            </div>
          </div>
        </div>
        <div class="flex justify-end gap-3 mt-6">
          <button @click="handleCloseInc" class="btn-cancel px-4 py-2 text-sm rounded-lg">取消</button>
          <button @click="save" class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium rounded-lg transition-colors">保存</button>
        </div>
      </div>
    </div>

    <!-- Incident Update Form -->
    <div v-if="showUpdate && updateIncident" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="handleCloseUpdate">
      <div class="rounded-xl p-4 md:p-6 w-full max-w-lg mx-4" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">事件更新</h3>
        <div class="text-sm mb-4" style="color: var(--text-color); opacity: 0.5;">更新事件: {{ updateIncident.title }}</div>
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">状态 *</label>
            <CustomSelect v-model="updateForm.status" :options="[{ label: '调查中', value: 'investigating' }, { label: '已确认', value: 'identified' }, { label: '监控中', value: 'monitoring' }, { label: '已解决', value: 'resolved' }]" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">内容 *</label>
            <textarea v-model="updateForm.content" rows="3" class="w-full px-3 py-2 rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500" style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);"></textarea>
          </div>
        </div>
        <div class="flex justify-end gap-3 mt-6">
          <button @click="handleCloseUpdate" class="btn-cancel px-4 py-2 text-sm rounded-lg">取消</button>
          <button @click="saveUpdate" class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium rounded-lg transition-colors">保存</button>
        </div>
      </div>
    </div>
  </div>
</template>
