<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { api } from '../../api/client'
import type { Incident, IncidentUpdate, Service } from '../../api/types'
import { useToast } from '../../composables/useToast'
import CustomSelect from './CustomSelect.vue'
import { useI18n } from '../../composables/useI18n'

const { t, formatDateTime } = useI18n()

const props = defineProps<{
  incident: Incident
  serviceName: string
  services: Service[]
}>()

const emit = defineEmits<{
  back: []
  updated: []
  deleted: []
  merge: [incident: Incident]
  split: [id: number]
}>()

const { show: toast } = useToast()

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

/** 事件状态下拉选项（随语言切换重新渲染） */
const statusOptions = computed(() => [
  { label: t('incident.status.investigating'), value: 'investigating' },
  { label: t('incident.status.identified'), value: 'identified' },
  { label: t('incident.status.monitoring'), value: 'monitoring' },
  { label: t('incident.status.resolved'), value: 'resolved' },
])

/** 影响等级下拉选项（随语言切换重新渲染） */
const impactOptions = computed(() => [
  { label: t('incident.impact.minor'), value: 'minor' },
  { label: t('incident.impact.major'), value: 'major' },
  { label: t('incident.impact.critical'), value: 'critical' },
])

const impactClass = (s: string) => {
  switch (s) {
    case 'critical': return 'text-red-600 bg-red-50 border-red-100 dark:text-red-400 dark:bg-red-900/30 dark:border-red-800'
    case 'major': return 'text-orange-600 bg-orange-50 border-orange-100 dark:text-orange-400 dark:bg-orange-900/30 dark:border-orange-800'
    case 'minor': return 'text-yellow-600 bg-yellow-50 border-yellow-100 dark:text-yellow-400 dark:bg-yellow-900/30 dark:border-yellow-800'
    default: return 'text-gray-600 bg-gray-50 border-gray-100 dark:text-gray-400 dark:bg-gray-800 dark:border-gray-700'
  }
}

const statusClass = (s: string) => {
  switch (s) {
    case 'investigating': return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
    case 'identified': return 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400'
    case 'monitoring': return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
    case 'resolved': return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400'
    default: return 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-400'
  }
}

const dotClass = (s: string) => {
  switch (s) {
    case 'investigating': return 'bg-orange-500'
    case 'identified': return 'bg-orange-500'
    case 'monitoring': return 'bg-blue-500'
    case 'resolved': return 'bg-emerald-500'
    default: return 'bg-gray-400'
  }
}

const affectedServiceNames = computed(() => {
  if (!props.incident.affectedServices) return [props.serviceName].filter(Boolean)
  const ids = props.incident.affectedServices.split(',').map(Number).filter(Boolean)
  if (ids.length === 0) return [props.serviceName].filter(Boolean)
  return ids.map(id => {
    const svc = props.services.find(s => s.id === id)
    return svc ? svc.name : t('admin.incident.unknownService')
  })
})

// Edit mode
const showEdit = ref(false)
const editForm = ref({
  title: '',
  impact: 'minor' as string,
  rootCause: '',
  resolution: '',
  postmortemUrl: '',
  postmortemPublic: false,
})

function openEdit() {
  editForm.value = {
    title: props.incident.title,
    impact: props.incident.impact,
    rootCause: props.incident.rootCause || '',
    resolution: props.incident.resolution || '',
    postmortemUrl: props.incident.postmortemUrl || '',
    postmortemPublic: !!props.incident.postmortemPublic,
  }
  showEdit.value = true
}

/** 复盘是否已经填写了任何内容 */
const hasPostmortem = computed(() =>
  !!(props.incident.rootCause || props.incident.resolution || props.incident.postmortemUrl)
)

async function saveEdit() {
  try {
    await api.updateIncident(props.incident.id, editForm.value)
    showEdit.value = false
    toast(t('admin.incident.updateSuccess'), 'success')
    emit('updated')
  } catch (e: any) {
    toast(e.message || t('admin.incident.updateFailed'))
  }
}

/**
 * 切换人工确认状态。未确认的活跃事件会被后端按「告警升级」配置再次通知，
 * 确认后即停止升级；取消确认会重置升级计数，重新进入未确认流程。
 */
async function toggleAck() {
  const next = !props.incident.acknowledged
  if (!next && !confirm(t('admin.incident.confirmUnack'))) return
  try {
    await api.updateIncident(props.incident.id, { acknowledged: next })
    toast(next ? t('admin.incident.ackSuccess') : t('admin.incident.unackSuccess'), 'success')
    emit('updated')
  } catch (e: any) {
    toast(e.message || t('admin.incident.actionFailed'))
  }
}

/** 确认状态说明文案（含确认人与确认时间） */
const ackDetail = computed(() => {
  if (!props.incident.acknowledged) {
    return props.incident.escalationCount > 0
      ? t('admin.incident.escalatedTimes', { n: props.incident.escalationCount })
      : t('admin.incident.awaitingAck')
  }
  const who = props.incident.acknowledgedBy || t('admin.incident.adminDefault')
  const when = props.incident.acknowledgedAt ? formatDateTime(props.incident.acknowledgedAt) : ''
  return when ? `${who} · ${when}` : who
})

// Add update
const updateForm = ref({ status: 'investigating', content: '' })

async function submitUpdate() {
  if (!updateForm.value.content.trim()) return
  try {
    await api.createIncidentUpdate(props.incident.id, updateForm.value)
    updateForm.value = { status: updateForm.value.status, content: '' }
    toast(t('admin.incident.updateSuccess'), 'success')
    emit('updated')
  } catch (e: any) {
    toast(e.message || t('admin.incident.updateFailed'))
  }
}

// Delete
async function handleDelete() {
  if (!confirm(t('admin.incident.confirmDeleteIncident'))) return
  try {
    await api.deleteIncident(props.incident.id)
    toast(t('admin.incident.deleteSuccess'), 'success')
    emit('deleted')
  } catch (e: any) {
    toast(e.message || t('admin.incident.deleteFailed'))
  }
}

// Timeline update edit/delete
const editingUpdate = ref<IncidentUpdate | null>(null)
const editUpdateForm = ref({ status: 'investigating', content: '' })

function startEditUpdate(update: IncidentUpdate) {
  editingUpdate.value = update
  editUpdateForm.value = { status: update.status, content: update.content }
}

function cancelEditUpdate() {
  editingUpdate.value = null
}

async function saveEditUpdate() {
  if (!editingUpdate.value || !editUpdateForm.value.content.trim()) return
  try {
    await api.updateIncidentUpdate(props.incident.id, editingUpdate.value.id, editUpdateForm.value)
    editingUpdate.value = null
    toast(t('admin.incident.updateSuccess'), 'success')
    emit('updated')
  } catch (e: any) {
    toast(e.message || t('admin.incident.updateFailed'))
  }
}

async function deleteUpdate(updateId: number) {
  if (!confirm(t('admin.incident.confirmDeleteUpdate'))) return
  try {
    await api.deleteIncidentUpdate(props.incident.id, updateId)
    toast(t('admin.incident.deleteSuccess'), 'success')
    emit('updated')
  } catch (e: any) {
    toast(e.message || t('admin.incident.deleteFailed'))
  }
}

// Reset update form status when incident changes
watch(() => props.incident.status, (s) => {
  updateForm.value.status = s
})

const reversedUpdates = computed(() => {
  if (!props.incident.updates) return []
  return [...props.incident.updates].reverse()
})
</script>

<template>
  <div>
    <!-- Header -->
    <div class="flex items-center justify-between mb-4">
      <button @click="emit('back')" class="flex items-center gap-1.5 text-sm transition-colors" style="color: var(--text-color); opacity: 0.5;">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7" /></svg>
        {{ t('admin.incident.backToList') }}
      </button>
      <div class="flex items-center gap-2">
        <button
          v-if="incident.status !== 'resolved'"
          @click="toggleAck"
          class="inline-flex items-center gap-1.5 text-sm font-medium px-3 py-1.5 rounded-lg border transition-colors"
          :class="incident.acknowledged
            ? 'text-emerald-600 bg-emerald-50 border-emerald-100 dark:text-emerald-400 dark:bg-emerald-500/15 dark:border-emerald-800'
            : 'text-amber-600 bg-amber-50 border-amber-100 dark:text-amber-400 dark:bg-amber-900/30 dark:border-amber-800'"
          :title="incident.acknowledged ? t('admin.incident.ackTooltipUnack') : t('admin.incident.ackTooltipConfirm')"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
          </svg>
          {{ incident.acknowledged ? t('admin.incident.acknowledged') : t('admin.incident.ackAction') }}
        </button>
        <button @click="openEdit" class="op-btn op-btn-edit p-1.5 rounded-lg" :title="t('admin.incident.edit')">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" /></svg>
        </button>
        <button @click="emit('merge', incident)" v-if="incident.status !== 'resolved'" class="op-btn p-1.5 rounded-lg" :title="t('admin.incident.merge')">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 4v12m0 0l-3-3m3 3l3-3M4 20h16" /></svg>
        </button>
        <button @click="handleDelete" class="op-btn op-btn-delete p-1.5 rounded-lg" :title="t('admin.incident.delete')">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg>
        </button>
      </div>
    </div>

    <!-- Info card -->
    <div class="rounded-xl p-5 mb-6" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
      <div class="flex items-start justify-between mb-3">
        <h2 class="text-lg font-bold" style="color: var(--text-color);">{{ incident.title }}</h2>
        <span :class="['inline-flex items-center px-2 py-1 rounded text-xs font-medium border', impactClass(incident.impact)]">
          {{ impactLabel[incident.impact] || incident.impact }}
        </span>
      </div>
      <div class="grid grid-cols-2 md:grid-cols-5 gap-4 text-sm">
        <div>
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.incident.fieldStatus') }}</div>
          <span :class="['inline-flex items-center gap-1.5 px-2 py-1 rounded text-xs font-medium', statusClass(incident.status)]">
            <span :class="['w-1.5 h-1.5 rounded-full', dotClass(incident.status)]" />
            {{ statusLabel[incident.status] || incident.status }}
          </span>
        </div>
        <div>
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.incident.fieldAck') }}</div>
          <span
            class="inline-flex items-center gap-1.5 px-2 py-1 rounded text-xs font-medium"
            :class="incident.acknowledged
              ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400'
              : 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400'"
          >
            <span class="w-1.5 h-1.5 rounded-full" :class="incident.acknowledged ? 'bg-emerald-500' : 'bg-amber-500'" />
            {{ incident.acknowledged ? t('admin.incident.acknowledged') : t('admin.incident.unacknowledged') }}
          </span>
          <div class="mt-1 text-xs" style="color: var(--text-color); opacity: 0.45;">{{ ackDetail }}</div>
        </div>
        <div>
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.incident.fieldAffectedServices') }}</div>
          <div class="font-medium" style="color: var(--text-color);">
            <template v-if="affectedServiceNames.length > 0">
              <span v-for="(name, idx) in affectedServiceNames" :key="idx">{{ name }}<span v-if="idx < affectedServiceNames.length - 1">, </span></span>
            </template>
            <template v-else>-</template>
          </div>
        </div>
        <div>
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.incident.fieldCreatedAt') }}</div>
          <div style="color: var(--text-color); opacity: 0.7;">{{ formatDateTime(incident.createdAt) }}</div>
        </div>
        <div>
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.incident.fieldUpdatedAt') }}</div>
          <div style="color: var(--text-color); opacity: 0.7;">{{ formatDateTime(incident.updatedAt) }}</div>
        </div>
      </div>
    </div>

    <!-- Children (sub-events) -->
    <div v-if="incident.children && incident.children.length > 0" class="rounded-xl p-5 mb-6" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
      <div class="flex items-center justify-between mb-3">
        <h3 class="font-bold" style="color: var(--text-color);">{{ t('admin.incident.childrenTitle', { n: incident.children.length }) }}</h3>
      </div>
      <div class="space-y-2">
        <div
          v-for="child in incident.children"
          :key="child.id"
          class="group flex items-center justify-between px-4 py-2.5 rounded-lg transition-colors hover:bg-[var(--button-hover-color)]"
        >
          <div class="flex items-center gap-3">
            <span class="text-sm font-medium" style="color: var(--text-color);">{{ child.title }}</span>
            <span :class="['inline-flex items-center px-2 py-0.5 rounded text-xs font-medium border', impactClass(child.impact)]">
              {{ impactLabel[child.impact] || child.impact }}
            </span>
            <span class="text-xs px-2 py-0.5 rounded" :class="statusClass(child.status)">
              {{ statusLabel[child.status] || child.status }}
            </span>
          </div>
          <button
            @click="emit('split', child.id)"
            class="op-btn p-1.5 rounded-lg opacity-0 group-hover:opacity-100 transition-all"
            :title="t('admin.incident.splitOut')"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 20V8m0 0l3 3m-3-3L9 11M4 4h16" />
            </svg>
          </button>
        </div>
      </div>
    </div>

    <!-- Timeline -->
    <div class="rounded-xl p-5" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
      <h3 class="font-bold mb-4" style="color: var(--text-color);">{{ t('admin.incident.timeline') }}</h3>

      <div v-if="reversedUpdates.length > 0" class="relative">
        <div class="absolute left-[11px] top-2 bottom-2 w-0.5" style="background-color: var(--button-border-color);"></div>
        <div v-for="update in reversedUpdates" :key="update.id" class="group relative pl-8 pb-6 last:pb-0">
          <div :class="['absolute left-0 top-1 w-[22px] h-[22px] rounded-full border-2 flex items-center justify-center', dotClass(update.status)]" style="border-color: var(--bg-color);">
            <svg class="w-3 h-3 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="3"><path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" /></svg>
          </div>
          <!-- Normal display -->
          <template v-if="editingUpdate?.id !== update.id">
            <div class="flex items-center gap-2 mb-1">
              <span :class="['px-2 py-0.5 rounded text-xs font-medium', statusClass(update.status)]">
                {{ statusLabel[update.status] || update.status }}
              </span>
              <span class="text-xs" style="color: var(--text-color); opacity: 0.4;">{{ formatDateTime(update.createdAt) }}</span>
              <div class="flex items-center gap-1 ml-auto opacity-0 group-hover:opacity-100 transition-opacity">
                <button @click="startEditUpdate(update)" class="op-btn op-btn-edit p-0.5" :title="t('admin.incident.edit')">
                  <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" /></svg>
                </button>
                <button @click="deleteUpdate(update.id)" class="op-btn op-btn-delete p-0.5" :title="t('admin.incident.delete')">
                  <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg>
                </button>
              </div>
            </div>
            <div class="text-sm" style="color: var(--text-color); opacity: 0.7;">{{ update.content }}</div>
          </template>
          <!-- Inline edit form -->
          <template v-else>
            <div class="space-y-2 -mt-1">
              <div class="flex gap-2">
                <CustomSelect v-model="editUpdateForm.status" :options="statusOptions" />
                <input v-model="editUpdateForm.content" class="flex-1 px-2 py-1 rounded text-xs focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500" style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);" />
              </div>
              <div class="flex gap-2">
                <button @click="saveEditUpdate" class="px-2 py-1 bg-emerald-500 hover:bg-emerald-600 text-white text-xs font-medium rounded transition-colors">{{ t('admin.incident.save') }}</button>
                <button @click="cancelEditUpdate" class="btn-cancel px-2 py-1 text-xs rounded">{{ t('admin.incident.cancel') }}</button>
              </div>
            </div>
          </template>
        </div>
      </div>
      <div v-else class="text-sm py-4 text-center" style="color: var(--text-color); opacity: 0.4;">{{ t('home.noUpdates') }}</div>

      <!-- Add update form -->
      <div class="mt-6 pt-4" style="border-top: 1px solid var(--button-border-color);">
        <div class="text-sm font-medium mb-3" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.incident.addUpdate') }}</div>
        <div class="flex gap-3">
          <CustomSelect v-model="updateForm.status" :options="statusOptions" />
          <input
            v-model="updateForm.content"
            @keyup.enter="submitUpdate"
            :placeholder="t('admin.incident.updateContentPlaceholder')"
            class="flex-1 px-3 py-2 rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500"
            style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);"
          />
          <button @click="submitUpdate" class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium rounded-lg transition-colors flex-shrink-0">
            {{ t('admin.incident.submit') }}
          </button>
        </div>
      </div>
    </div>

    <!-- 事后复盘（管理端始终可见，是否对外由编辑弹窗里的开关决定） -->
    <div v-if="hasPostmortem" class="rounded-xl p-5 mt-6" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
      <div class="flex items-center justify-between mb-3">
        <h3 class="text-base font-bold" style="color: var(--text-color);">{{ t('incident.postmortem') }}</h3>
        <span
          class="text-xs px-2 py-0.5 rounded font-medium"
          :class="incident.postmortemPublic
            ? 'bg-emerald-50 text-emerald-600 dark:bg-emerald-500/15 dark:text-emerald-400'
            : 'bg-gray-100 text-gray-500 dark:bg-gray-700 dark:text-gray-400'"
        >{{ incident.postmortemPublic ? t('admin.incident.postmortemPublic') : t('admin.incident.postmortemInternal') }}</span>
      </div>
      <div class="space-y-3 text-sm">
        <div v-if="incident.rootCause">
          <div class="mb-1 text-xs" style="color: var(--text-color); opacity: 0.4;">{{ t('incident.rootCause') }}</div>
          <p class="whitespace-pre-wrap" style="color: var(--text-color); opacity: 0.8;">{{ incident.rootCause }}</p>
        </div>
        <div v-if="incident.resolution">
          <div class="mb-1 text-xs" style="color: var(--text-color); opacity: 0.4;">{{ t('incident.resolution') }}</div>
          <p class="whitespace-pre-wrap" style="color: var(--text-color); opacity: 0.8;">{{ incident.resolution }}</p>
        </div>
        <div v-if="incident.postmortemUrl">
          <div class="mb-1 text-xs" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.incident.postmortemUrlLabel') }}</div>
          <a :href="incident.postmortemUrl" target="_blank" rel="noopener noreferrer" class="text-emerald-600 dark:text-emerald-400 hover:opacity-80 break-all">{{ incident.postmortemUrl }}</a>
        </div>
      </div>
    </div>

    <!-- Edit modal -->
    <div v-if="showEdit" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="showEdit = false">
      <div class="rounded-xl p-4 md:p-6 w-full max-w-lg mx-4 max-h-[90vh] overflow-y-auto" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">{{ t('admin.incident.editIncident') }}</h3>
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.incident.labelTitle') }}</label>
            <input v-model="editForm.title" class="w-full px-3 py-2 rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500" style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.incident.labelImpact') }}</label>
            <CustomSelect v-model="editForm.impact" :options="impactOptions" />
          </div>

          <!-- 事后复盘：内容留空则公开页面不展示 -->
          <div class="rounded-lg p-3 space-y-3" style="border: 1px solid var(--button-border-color);">
            <div class="text-sm font-medium" style="color: var(--text-color);">{{ t('incident.postmortem') }}</div>
            <div>
              <label class="block text-xs font-medium mb-1" style="color: var(--text-color); opacity: 0.6;">{{ t('incident.rootCause') }}</label>
              <textarea
                v-model="editForm.rootCause"
                rows="2"
                :placeholder="t('admin.incident.rootCausePlaceholder')"
                class="w-full px-3 py-2 rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500"
                style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);"
              ></textarea>
            </div>
            <div>
              <label class="block text-xs font-medium mb-1" style="color: var(--text-color); opacity: 0.6;">{{ t('incident.resolution') }}</label>
              <textarea
                v-model="editForm.resolution"
                rows="2"
                :placeholder="t('admin.incident.resolutionPlaceholder')"
                class="w-full px-3 py-2 rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500"
                style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);"
              ></textarea>
            </div>
            <div>
              <label class="block text-xs font-medium mb-1" style="color: var(--text-color); opacity: 0.6;">{{ t('admin.incident.postmortemUrl') }}</label>
              <input
                v-model="editForm.postmortemUrl"
                type="url"
                placeholder="https://..."
                class="w-full px-3 py-2 rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500"
                style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);"
              />
            </div>
            <div class="flex items-center justify-between">
              <div>
                <div class="text-xs font-medium" style="color: var(--text-color);">{{ t('admin.incident.postmortemPublicToggle') }}</div>
                <div class="text-xs mt-0.5" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.incident.postmortemPublicHint') }}</div>
              </div>
              <button
                type="button"
                @click="editForm.postmortemPublic = !editForm.postmortemPublic"
                class="relative inline-flex h-6 w-11 items-center rounded-full flex-shrink-0 transition-colors"
                :class="editForm.postmortemPublic ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-gray-600'"
              >
                <span
                  class="inline-block h-4 w-4 transform rounded-full bg-white transition"
                  :class="editForm.postmortemPublic ? 'translate-x-[22px]' : 'translate-x-[3px]'"
                />
              </button>
            </div>
          </div>
        </div>
        <div class="flex justify-end gap-3 mt-6">
          <button @click="showEdit = false" class="btn-cancel px-4 py-2 text-sm rounded-lg">{{ t('admin.incident.cancel') }}</button>
          <button @click="saveEdit" class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium rounded-lg transition-colors">{{ t('admin.incident.save') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>
