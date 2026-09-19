<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { api } from '../../api/client'
import type { Incident, Service } from '../../api/types'
import IncidentDetail from './IncidentDetail.vue'
import CustomSelect from './CustomSelect.vue'
import { useToast } from '../../composables/useToast'
import { useUnsavedChanges } from '../../composables/useUnsavedChanges'
import { useI18n } from '../../composables/useI18n'

const { t, formatDateTime } = useI18n()

const incidents = ref<Incident[]>([])
const loading = ref(true)
const page = ref(1)
const totalPage = ref(0)
const limit = 15
const { show: toast } = useToast()

// Detail view
const selectedIncident = ref<Incident | null>(null)
const loadingDetail = ref(false)

// Merge / Split
const showMerge = ref(false)
const mergeTarget = ref<Incident | null>(null)
const mergeSourceId = ref('')

const mergeableIncidents = computed(() => {
  if (!mergeTarget.value) return []
  return incidents.value.filter(i => i.id !== mergeTarget.value!.id && i.status !== 'resolved')
})

const mergeOptions = computed(() => {
  return mergeableIncidents.value.map(inc => ({
    label: inc.title,
    value: String(inc.id),
  }))
})

function isMerged(inc: Incident): boolean {
  const ids = inc.affectedServices ? inc.affectedServices.split(',').filter(Boolean).map(Number) : []
  return ids.length > 1
}

function openMerge(inc: Incident) {
  if (inc.status === 'resolved') {
    toast(t('admin.incident.cannotMergeResolved'))
    return
  }
  mergeTarget.value = inc
  mergeSourceId.value = ''
  showMerge.value = true
}

async function saveMerge() {
  if (!mergeTarget.value || !mergeSourceId.value) return
  try {
    await api.mergeIncident(mergeTarget.value.id, Number(mergeSourceId.value))
    showMerge.value = false
    toast(t('admin.incident.mergeSuccess'), 'success')
    load()
  } catch (e: any) {
    toast(e.message || t('admin.incident.mergeFailed'))
  }
}

async function openDetail(inc: Incident) {
  loadingDetail.value = true
  try {
    const res = await api.getAdminIncident(inc.id)
    selectedIncident.value = res.data
  } catch (e: any) {
    // Fallback: use list item if detail fetch fails
    selectedIncident.value = inc
    toast(e.message || t('admin.incident.loadDetailFailed'))
  } finally {
    loadingDetail.value = false
  }
}

function getServiceName(serviceId: number): string {
  const svc = services.value.find(s => s.id === serviceId)
  return svc ? svc.name : ''
}

async function handleDetailUpdated() {
  if (selectedIncident.value) {
    try {
      const res = await api.getAdminIncident(selectedIncident.value.id)
      selectedIncident.value = res.data
    } catch {
      await load()
      if (selectedIncident.value) {
        const updated = incidents.value.find(i => i.id === selectedIncident.value!.id)
        if (updated) selectedIncident.value = updated
      }
    }
  } else {
    await load()
  }
}

function handleDetailDeleted() {
  selectedIncident.value = null
  load()
}

async function handleSplit(id: number) {
  try {
    await api.splitIncident(id)
    toast(t('admin.incident.splitSuccess'), 'success')
    // If viewing a parent incident, re-fetch detail to update children list
    if (selectedIncident.value && selectedIncident.value.id !== id) {
      const res = await api.getAdminIncident(selectedIncident.value.id)
      selectedIncident.value = res.data
    } else {
      selectedIncident.value = null
      await load()
    }
  } catch (e: any) {
    toast(e.message || t('admin.incident.splitFailed'))
  }
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

const statusLabel = computed<Record<string, string>>(() => ({
  investigating: t('incident.status.investigating'),
  identified: t('incident.status.identified'),
  monitoring: t('incident.status.monitoring'),
  resolved: t('incident.status.resolved'),
}))

const impactLabel = computed<Record<string, string>>(() => ({
  critical: t('incident.impact.critical'),
  major: t('incident.impact.major'),
  minor: t('incident.impact.minor'),
}))

/** 影响等级下拉选项（随语言切换重新渲染） */
const impactOptions = computed(() => [
  { label: t('incident.impact.minor'), value: 'minor' },
  { label: t('incident.impact.major'), value: 'major' },
  { label: t('incident.impact.critical'), value: 'critical' },
])

/** 事件状态下拉选项（随语言切换重新渲染） */
const statusOptions = computed(() => [
  { label: t('incident.status.investigating'), value: 'investigating' },
  { label: t('incident.status.identified'), value: 'identified' },
  { label: t('incident.status.monitoring'), value: 'monitoring' },
  { label: t('incident.status.resolved'), value: 'resolved' },
])

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
    toast(e.message || t('admin.incident.loadFailed'))
  } finally {
    loading.value = false
  }
}

/**
 * 切换事件的人工确认状态。
 * 未确认且长时间无人处理的活跃事件会被后端再次升级通知（见 checker/escalation.go），
 * 确认后即停止升级；取消确认会把升级计数重置，重新进入未确认流程。
 */
async function toggleAck(inc: Incident) {
  const next = !inc.acknowledged
  if (!next && !confirm(t('admin.incident.confirmUnack'))) return
  try {
    await api.updateIncident(inc.id, { acknowledged: next })
    toast(next ? t('admin.incident.ackSuccess') : t('admin.incident.unackSuccess'), 'success')
    // 详情页打开时同步刷新，避免两处状态不一致
    if (selectedIncident.value && selectedIncident.value.id === inc.id) {
      await openDetail(inc)
    }
    load()
  } catch (e: any) {
    toast(e.message || t('admin.incident.actionFailed'))
  }
}

/** 确认信息的展示文案 */
function ackText(inc: Incident): string {
  if (!inc.acknowledged) {
    return inc.escalationCount > 0
      ? t('admin.incident.unacknowledgedEscalated', { n: inc.escalationCount })
      : t('admin.incident.unacknowledged')
  }
  const detail = [inc.acknowledgedBy || '', inc.acknowledgedAt ? formatDateTime(inc.acknowledgedAt) : '']
    .filter(Boolean)
    .join(' ')
  return detail ? t('admin.incident.acknowledgedAt', { detail }) : t('admin.incident.acknowledged')
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
    toast(editing.value ? t('admin.incident.updateSuccess') : t('admin.incident.createSuccess'), 'success')
    load()
  } catch (e: any) {
    toast(e.message || t('admin.incident.saveFailed'))
  }
}

async function remove(id: number) {
  if (!confirm(t('admin.incident.confirmDelete'))) return
  try {
    await api.deleteIncident(id)
    toast(t('admin.incident.deleteSuccess'), 'success')
    load()
  } catch (e: any) {
    toast(e.message || t('admin.incident.deleteFailed'))
  }
}

function openUpdate(inc: Incident) {
  updateIncident.value = inc
  updateForm.value = { status: inc.status, content: '' }
  showUpdate.value = true
}

function handleCloseUpdate() {
  if (updateDirty.value && !confirm(t('admin.incident.confirmDiscardChanges'))) return
  showUpdate.value = false
}

async function saveUpdate() {
  if (!updateIncident.value) return
  try {
    await api.createIncidentUpdate(updateIncident.value.id, updateForm.value)
    showUpdate.value = false
    updateDirty.value = false
    toast(t('admin.incident.updateSuccess'), 'success')
    load()
  } catch (e: any) {
    toast(e.message || t('admin.incident.updateFailed'))
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
    <div v-if="loadingDetail" class="text-center py-12" style="color: var(--text-color); opacity: 0.4;">{{ t('common.loading') }}</div>
    <IncidentDetail
      v-else-if="selectedIncident"
      :incident="selectedIncident"
      :service-name="getServiceName(selectedIncident.serviceId)"
      :services="services"
      @back="selectedIncident = null"
      @updated="handleDetailUpdated"
      @deleted="handleDetailDeleted"
      @merge="openMerge"
      @split="handleSplit"
    />

    <!-- List view -->
    <template v-else>
    <div class="flex justify-between items-center mb-4">
      <h2 class="text-lg font-bold" style="color: var(--text-color);">{{ t('admin.incident.listTitle') }}</h2>
      <button @click="openCreate" class="bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium px-4 py-2 rounded-lg flex items-center gap-1 transition-colors">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" /></svg>
        {{ t('admin.incident.create') }}
      </button>
    </div>

    <div v-if="loading" class="text-center py-12" style="color: var(--text-color); opacity: 0.4;">{{ t('common.loading') }}</div>

    <div v-else class="rounded-xl" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
      <div class="overflow-x-auto">
        <table class="w-full text-left text-sm">
        <thead class="text-xs" style="color: var(--text-color); opacity: 0.4; background-color: var(--button-hover-color); opacity: 0.5; border-bottom: 1px solid var(--button-border-color);">
          <tr>
            <th class="px-6 py-3 font-medium">{{ t('admin.incident.colTitle') }}</th>
            <th class="px-6 py-3 font-medium">{{ t('admin.incident.colImpact') }}</th>
            <th class="px-6 py-3 font-medium">{{ t('admin.incident.colAck') }}</th>
            <th class="px-6 py-3 font-medium">{{ t('admin.incident.colStatus') }}</th>
            <th class="px-6 py-3 font-medium">{{ t('admin.incident.colTime') }}</th>
            <th class="px-6 py-3 font-medium text-right">{{ t('admin.incident.colActions') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y">
          <tr v-for="inc in incidents" :key="inc.id" @click="openDetail(inc)" class="cursor-pointer hover:bg-[var(--button-hover-color)]">
            <td class="px-6 py-4 font-bold" style="color: var(--text-color);">{{ inc.title }}</td>
            <td class="px-6 py-4">
              <span :class="['inline-flex items-center px-2 py-1 rounded text-xs font-medium border', impactClass(inc.impact)]">
                {{ impactLabel[inc.impact] || inc.impact }}
              </span>
            </td>
            <td class="px-6 py-4" @click.stop>
              <button
                @click="toggleAck(inc)"
                :disabled="inc.status === 'resolved'"
                :title="inc.status === 'resolved' ? t('admin.incident.ackTooltipResolved') : (inc.acknowledged ? t('admin.incident.ackTooltipUnack') : t('admin.incident.ackTooltipAck'))"
                class="inline-flex items-center gap-1 px-2 py-1 rounded text-xs font-medium border cursor-pointer transition-colors disabled:cursor-not-allowed disabled:opacity-40"
                :class="inc.acknowledged
                  ? 'text-emerald-600 bg-emerald-50 border-emerald-100 dark:text-emerald-400 dark:bg-emerald-500/15 dark:border-emerald-800'
                  : 'text-amber-600 bg-amber-50 border-amber-100 dark:text-amber-400 dark:bg-amber-900/30 dark:border-amber-800'"
                :style="inc.status === 'resolved' ? 'color: var(--text-color); background-color: var(--bg-color); border-color: var(--button-border-color);' : ''"
              >
                <span class="w-1.5 h-1.5 rounded-full" :class="inc.acknowledged ? 'bg-emerald-500' : 'bg-amber-500'" />
                {{ ackText(inc) }}
              </button>
            </td>
            <td class="px-6 py-4" style="color: var(--text-color); opacity: 0.5;">{{ statusLabel[inc.status] || inc.status }}</td>
            <td class="px-6 py-4 text-xs" style="color: var(--text-color); opacity: 0.5;">{{ formatDateTime(inc.createdAt) }}</td>
            <td class="px-6 py-4 text-right">
              <button @click.stop="openEdit(inc)" class="op-btn op-btn-edit mr-3" :title="t('admin.incident.edit')">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                </svg>
              </button>
              <button @click.stop="openUpdate(inc)" class="op-btn mr-3" :title="t('admin.incident.incidentUpdate')">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              </button>
              <button @click.stop="remove(inc.id)" class="op-btn op-btn-delete" :title="t('admin.incident.delete')">
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
        {{ t('admin.incident.prevPage') }}
      </button>
      <span class="text-sm px-3" style="color: var(--text-color); opacity: 0.5;">
        {{ t('admin.incident.pageOf', { page, total: totalPage }) }}
      </span>      <button
        @click="goPage(page + 1)"
        :disabled="page >= totalPage"
        class="px-3 py-1.5 text-sm rounded-lg disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
        style="border: 1px solid var(--button-border-color); color: var(--text-color); opacity: 0.6;"
      >
        {{ t('admin.incident.nextPage') }}
      </button>
    </div>
    </template>

    <!-- Incident Form -->
    <div v-if="showForm" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="handleCloseInc">
      <div class="rounded-xl p-4 md:p-6 w-full max-w-lg mx-4" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">{{ editing ? t('admin.incident.editIncident') : t('admin.incident.create') }}</h3>
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.incident.labelTitle') }}</label>
            <input v-model="form.title" class="w-full px-3 py-2 rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500" style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.incident.labelImpact') }}</label>
            <CustomSelect v-model="form.impact" :options="impactOptions" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.incident.labelStatus') }}</label>
            <CustomSelect v-model="form.status" :options="statusOptions" />
          </div>
          <div class="relative">
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.incident.labelService') }}</label>
            <input
              v-model="serviceSearch"
              @focus="showServiceDropdown = true"
              @blur="delayBlur"
              type="text"
              :placeholder="t('admin.incident.searchServicePlaceholder')"
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
                {{ t('admin.incident.noMatchingService') }}
              </div>
            </div>
          </div>
        </div>
        <div class="flex justify-end gap-3 mt-6">
          <button @click="handleCloseInc" class="btn-cancel px-4 py-2 text-sm rounded-lg">{{ t('admin.incident.cancel') }}</button>
          <button @click="save" class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium rounded-lg transition-colors">{{ t('admin.incident.save') }}</button>
        </div>
      </div>
    </div>

    <!-- Incident Update Form -->
    <div v-if="showUpdate && updateIncident" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="handleCloseUpdate">
      <div class="rounded-xl p-4 md:p-6 w-full max-w-lg mx-4" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">{{ t('admin.incident.incidentUpdate') }}</h3>
        <div class="text-sm mb-4" style="color: var(--text-color); opacity: 0.5;">{{ t('admin.incident.updateEventFor', { title: updateIncident.title }) }}</div>
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.incident.labelStatusRequired') }}</label>
            <CustomSelect v-model="updateForm.status" :options="statusOptions" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.incident.labelContent') }}</label>
            <textarea v-model="updateForm.content" rows="3" class="w-full px-3 py-2 rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500" style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);"></textarea>
          </div>
        </div>
        <div class="flex justify-end gap-3 mt-6">
          <button @click="handleCloseUpdate" class="btn-cancel px-4 py-2 text-sm rounded-lg">{{ t('admin.incident.cancel') }}</button>
          <button @click="saveUpdate" class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium rounded-lg transition-colors">{{ t('admin.incident.save') }}</button>
        </div>
      </div>
    </div>

    <!-- Merge Modal -->
    <div v-if="showMerge && mergeTarget" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="showMerge = false">
      <div class="rounded-xl p-4 md:p-6 w-full max-w-lg mx-4" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">{{ t('admin.incident.mergeTitle') }}</h3>
        <div class="text-sm mb-4" style="color: var(--text-color); opacity: 0.5;">
          {{ t('admin.incident.mergeHint', { title: mergeTarget.title }) }}
        </div>
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.incident.labelSelectIncident') }}</label>
            <CustomSelect v-model="mergeSourceId" :options="mergeOptions" :placeholder="t('admin.incident.selectPlaceholder')" />
            <div v-if="mergeableIncidents.length === 0" class="text-sm mt-2" style="color: var(--text-color); opacity: 0.4;">
              {{ t('admin.incident.noMergeableIncident') }}
            </div>
          </div>
        </div>
        <div class="flex justify-end gap-3 mt-6">
          <button @click="showMerge = false" class="btn-cancel px-4 py-2 text-sm rounded-lg">{{ t('admin.incident.cancel') }}</button>
          <button @click="saveMerge" :disabled="!mergeSourceId || mergeableIncidents.length === 0" class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium rounded-lg transition-colors disabled:opacity-40">{{ t('admin.incident.merge') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>
