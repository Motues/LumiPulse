<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { api } from '../../api/client'
import type { ServiceDetail, ServiceFolder } from '../../api/types'
import ServiceDetailComponent from './ServiceDetail.vue'
import { useToast } from '../../composables/useToast'
import { useUnsavedChanges } from '../../composables/useUnsavedChanges'
import { useI18n } from '../../composables/useI18n'
import CustomSelect from './CustomSelect.vue'
import { HOMEPAGE_BLOCKS, parseHomepageBlocks, serializeHomepageBlocks } from '../../utils/homepageBlocks'

const props = defineProps<{
  pendingServiceId?: number
}>()

const emit = defineEmits<{
  opened: []
}>()

const { show: toast } = useToast()
const { t } = useI18n()

const services = ref<ServiceDetail[]>([])
const loading = ref(true)
const showForm = ref(false)
const editing = ref<ServiceDetail | null>(null)
/** 服务分组列表，用于表单里的归属选择 */
const folders = ref<ServiceFolder[]>([])
/** 分组下拉选项：0 = 不分组（未分组服务在首页各自独立展示） */
const folderOptions = computed(() => [
  { label: t('admin.service.folderNone'), value: '0' },
  ...folders.value.map(f => ({ label: f.name, value: String(f.id) })),
])
/**
 * 探测高级匹配字段说明：
 * - httpHeaders / httpBody 属于敏感信息，后端读接口不回显原值，
 *   因此表单里留空提交 = 保持原值（与 SMTP 密码的处理方式一致）。
 * - httpMethod / expectStatus / expectKeyword 后端按「传了就用传上来的值」处理，
 *   清空即回到默认行为（GET / 200-399 / 不做关键字匹配）。
 */
const defaultForm = () => ({
  name: '',
  url: '',
  description: '',
  type: 'http',
  interval: 60,
  timeoutSeconds: 10,
  showOnHomepage: true,
  insecureSkipVerify: false,
  httpMethod: 'GET',
  httpHeaders: '',
  httpBody: '',
  expectStatus: '',
  expectKeyword: '',
  /** 所属服务分组。0 表示未分组（后端按取消分组处理） */
  folderId: 0,
  /**
   * 公开首页「服务详情」展示的内容块（逗号分隔）。
   * 新增服务默认全部展示；显式写全 key 而不是空串，
   * 因为空串在后端会被规范化成「全部不展示」。
   */
  homepageBlocks: serializeHomepageBlocks(HOMEPAGE_BLOCKS),
})
const form = ref(defaultForm())
const showAdvanced = ref(false)

/** 首页展示内容块开关（顺序与 utils/homepageBlocks.ts 一致） */
const homepageBlockOptions = computed(() => [
  { key: 'metrics', label: t('admin.service.blockMetrics') },
  { key: 'interval', label: t('admin.service.blockInterval') },
  { key: 'cert', label: t('admin.service.blockCert') },
  { key: 'latency', label: t('admin.service.blockLatency') },
  { key: 'heatmap', label: t('admin.service.blockHeatmap') },
  { key: 'history', label: t('admin.service.blockHistory') },
])

const selectedHomepageBlocks = computed(() => parseHomepageBlocks(form.value.homepageBlocks))

function toggleHomepageBlock(key: string, event: Event) {
  const checked = (event.target as HTMLInputElement).checked
  const selected = new Set(selectedHomepageBlocks.value)
  if (checked) selected.add(key)
  else selected.delete(key)
  form.value.homepageBlocks = serializeHomepageBlocks(HOMEPAGE_BLOCKS.filter(k => selected.has(k)))
}
/**
 * 请求头输入框的占位提示：编辑态说明留空语义，新增态给一个 JSON 示例。
 * 示例里带引号，放模板属性里会破坏 attribute 解析，因此放脚本里。
 */
const headersPlaceholder = computed(() =>
  editing.value
    ? t('admin.service.formHeadersPlaceholderKeep')
    : t('admin.service.formHeadersPlaceholderNew')
)
/** 关键字示例同样带引号，放脚本里避免破坏模板属性解析 */
const keywordPlaceholder = computed(() => t('admin.service.formKeywordPlaceholder'))
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
    case 'operational': return 'text-emerald-600 bg-emerald-50 border-emerald-100 dark:text-emerald-400 dark:bg-emerald-500/15 dark:border-emerald-800'
    case 'degraded': return 'text-yellow-600 bg-yellow-50 border-yellow-100 dark:text-yellow-400 dark:bg-yellow-900/30 dark:border-yellow-800'
    case 'outage': return 'text-red-600 bg-red-50 border-red-100 dark:text-red-400 dark:bg-red-900/30 dark:border-red-800'
    default: return ''
  }
}

async function load() {
  loading.value = true
  try {
    const [res, folderRes] = await Promise.all([
      api.getAdminServices(),
      // 分组用于表单里的归属选择；失败时降级为「无分组可选」，不影响服务列表
      api.getServiceFolders().catch(() => ({ data: [] as ServiceFolder[] })),
    ])
    services.value = res.data
    folders.value = folderRes.data
    if (props.pendingServiceId) {
      const svc = services.value.find(s => s.id === props.pendingServiceId)
      if (svc) {
        selectedService.value = svc
        emit('opened')
      }
    }
  } catch (e: any) {
    toast(e.message || t('admin.service.loadFailed'))
  } finally {
    loading.value = false
  }
}

const { markClean: cleanSvc, handleClose: closeSvc, restoreFromStorage: restoreSvc } = useUnsavedChanges(form as any, 'svc_form')

function openCreate() {
  editing.value = null
  form.value = defaultForm()
  showAdvanced.value = false
  restoreSvc()
  cleanSvc()
  showForm.value = true
}

function openEdit(svc: ServiceDetail) {
  editing.value = svc
  form.value = {
    ...defaultForm(),
    name: svc.name,
    url: svc.url,
    description: svc.description || '',
    type: svc.type,
    interval: svc.interval,
    timeoutSeconds: svc.timeoutSeconds || 10,
    showOnHomepage: svc.showOnHomepage,
    insecureSkipVerify: !!svc.insecureSkipVerify,
    httpMethod: svc.httpMethod || 'GET',
    // 后端不回显敏感值，这里保持空串表示「保持不变」
    httpHeaders: '',
    httpBody: '',
    expectStatus: svc.expectStatus || '',
    expectKeyword: svc.expectKeyword || '',
    folderId: svc.folderId || 0,
    // 归一化后再回填：后端把「全部不展示」存成 none，直接回填空串会被重新解释成「全部展示」
    homepageBlocks: serializeHomepageBlocks(parseHomepageBlocks(svc.homepageBlocks)),
  }
  // 已配置过高级匹配时默认展开，避免看不出服务被特殊配置过
  showAdvanced.value = !!(svc.expectStatus || svc.expectKeyword || (svc.httpMethod && svc.httpMethod !== 'GET'))
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
    toast(editing.value ? t('admin.service.updateSuccess') : t('admin.service.createSuccess'), 'success')
    load()
  } catch (e: any) {
    toast(e.message || t('admin.service.saveFailed'))
  }
}

async function remove(id: number) {
  if (!confirm(t('admin.service.deleteConfirm'))) return
  try {
    await api.deleteService(id)
    toast(t('admin.service.deleteSuccess'), 'success')
    load()
  } catch (e: any) {
    toast(e.message || t('admin.service.deleteFailed'))
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
    toast(t('admin.service.orderSaved'), 'success')
  } catch (e: any) {
    toast(e.message || t('admin.service.orderSaveFailed'))
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
      <h2 class="text-lg font-bold" style="color: var(--text-color);">{{ t('admin.service.title') }}</h2>
      <button @click="openCreate" class="bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium px-4 py-2 rounded-lg flex items-center gap-1 transition-colors">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" /></svg>
        {{ t('admin.service.add') }}
      </button>
    </div>

    <!-- Search -->
    <div v-if="!loading" class="mb-4">
      <input
        v-model="searchQuery"
        type="text"
        :placeholder="t('admin.service.searchPlaceholder')"
        class="w-full px-4 py-2 border rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500"
        style="background-color: var(--bg-color); color: var(--text-color); border-color: var(--button-border-color);"
      />
    </div>

    <div v-if="loading" class="text-center py-12" style="color: var(--text-color); opacity: 0.4;">{{ t('common.loading') }}</div>

    <div v-else class="rounded-xl" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
      <div class="overflow-x-auto">
        <table class="w-full text-left text-sm">
        <thead class="text-xs border-b" style="color: var(--text-color); background-color: var(--button-hover-color); opacity: 0.5; border-color: var(--button-border-color);">
          <tr>
            <th class="px-2 py-3 w-6"></th>
            <th class="px-6 py-3 font-medium">{{ t('admin.service.colName') }}</th>
            <th class="px-6 py-3 font-medium">{{ t('admin.service.colUrl') }}</th>
            <th class="px-6 py-3 font-medium">{{ t('admin.service.type') }}</th>
            <th class="px-6 py-3 font-medium">{{ t('admin.service.status') }}</th>
            <th class="px-6 py-3 font-medium">{{ t('admin.service.uptime') }}</th>
            <th class="px-6 py-3 font-medium">{{ t('admin.service.latency') }}</th>
            <th class="px-6 py-3 font-medium text-right">{{ t('admin.service.colActions') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y">
          <tr
            v-for="(svc, idx) in filteredServices" :key="svc.id"
            :draggable="true"
            @dragstart="onDragStart(idx)"
            @dragover="onDragOver"
            @drop="onDrop(idx)"
            class="hover:bg-[var(--button-hover-color)]"
            :class="{ 'opacity-50': dragIndex === idx }"
          >
            <td class="px-2 py-4 cursor-grab" style="color: var(--text-color); opacity: 0.3;">
              <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 24 24"><path d="M8 6h2v2H8V6zm6 0h2v2h-2V6zM8 11h2v2H8v-2zm6 0h2v2h-2v-2zm-6 5h2v2H8v-2zm6 0h2v2h-2v-2z"/></svg>
            </td>
            <td class="px-6 py-4 font-bold cursor-pointer hover:text-emerald-600" style="color: var(--text-color);" @click="selectedService = svc">{{ svc.name }}</td>
            <td class="px-6 py-4 max-w-[200px] truncate" style="color: var(--text-color); opacity: 0.5;">{{ svc.url }}</td>
            <td class="px-6 py-4 " style="color: var(--text-color); opacity: 0.5;">{{ svc.type }}</td>
            <td class="px-6 py-4">
              <span :class="['inline-flex items-center gap-1.5 px-2 py-1 rounded text-xs font-medium border', statusClass(svc.status)]" :style="!['operational','degraded','outage'].includes(svc.status) ? 'color: var(--text-color); opacity: 0.6; background-color: var(--bg-color); border-color: var(--button-border-color);' : ''">
                {{ svc.status === 'operational' ? t('admin.service.statusOperational') : svc.status === 'degraded' ? t('admin.service.statusDegraded') : t('admin.service.statusOutage') }}
              </span>
            </td>
            <td class="px-6 py-4 " style="color: var(--text-color); opacity: 0.5;">{{ svc.uptime.toFixed(2) }}%</td>
            <td class="px-6 py-4 " style="color: var(--text-color); opacity: 0.5;">{{ svc.latency }}ms</td>
            <td class="px-6 py-4 text-right whitespace-nowrap">
              <button @click="moveUp(idx)" :disabled="idx === 0" class="op-btn mr-1 disabled:opacity-30 disabled:cursor-not-allowed" :title="t('admin.service.moveUp')">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M5 15l7-7 7 7" /></svg>
              </button>
              <button @click="moveDown(idx)" :disabled="idx === services.length - 1" class="op-btn mr-2 disabled:opacity-30 disabled:cursor-not-allowed" :title="t('admin.service.moveDown')">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" /></svg>
              </button>
              <button @click="openEdit(svc)" class="op-btn op-btn-edit mr-2" :title="t('admin.service.edit')">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                </svg>
              </button>
              <button @click="remove(svc.id)" class="op-btn op-btn-delete" :title="t('admin.service.delete')">
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
      <div class="rounded-xl p-4 md:p-6 w-full max-w-lg mx-4 max-h-[90vh] overflow-y-auto thin-scroll" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">{{ editing ? t('admin.service.editTitle') : t('admin.service.add') }}</h3>
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.service.formName') }}</label>
            <input v-model="form.name" class="input-field w-full px-3 py-2 rounded-lg text-sm" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.service.formUrl') }}</label>
            <input v-model="form.url" class="input-field w-full px-3 py-2 rounded-lg text-sm" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.service.formDescription') }}</label>
            <input v-model="form.description" class="input-field w-full px-3 py-2 rounded-lg text-sm" />
          </div>
          <div class="grid grid-cols-2 sm:grid-cols-3 gap-4">
            <div>
              <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.service.type') }}</label>
              <CustomSelect v-model="form.type" :options="[{ label: 'HTTP', value: 'http' }, { label: 'TCP', value: 'tcp' }, { label: 'Ping', value: 'ping' }]" />
            </div>
            <div>
              <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.service.formInterval') }}</label>
              <input v-model.number="form.interval" type="number" class="input-field w-full px-3 py-2 rounded-lg text-sm" />
            </div>
            <div>
              <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.service.formTimeout') }}</label>
              <input v-model.number="form.timeoutSeconds" type="number" min="1" max="300" class="input-field w-full px-3 py-2 rounded-lg text-sm" />
            </div>
          </div>
          <p class="text-xs -mt-2" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.service.formTimeoutHint') }}</p>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.service.folderLabel') }}</label>
            <CustomSelect
              :model-value="String(form.folderId)"
              @update:model-value="(v: string) => form.folderId = Number(v)"
              :options="folderOptions"
            />
            <p class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.service.folderHint') }}</p>
          </div>

          <!-- 公开首页展示内容：只影响公开页面，管理后台始终展示全部 -->
          <div class="pt-2" style="border-top: 1px solid var(--button-border-color);">
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.service.formHomepageBlocks') }}</label>
            <div class="grid grid-cols-2 gap-x-4 gap-y-2 text-sm" style="color: var(--text-color);">
              <label
                v-for="block in homepageBlockOptions"
                :key="block.key"
                class="inline-flex items-center gap-2 cursor-pointer"
              >
                <input
                  type="checkbox"
                  :checked="selectedHomepageBlocks.has(block.key)"
                  @change="toggleHomepageBlock(block.key, $event)"
                  class="accent-emerald-500"
                />
                {{ block.label }}
              </label>
            </div>
            <p class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.service.formHomepageBlocksHint') }}</p>
          </div>

          <div class="flex items-center justify-between pt-2">
            <span class="text-sm font-medium" style="color: var(--text-color);">{{ t('admin.service.formShowOnHomepage') }}</span>
            <label class="relative inline-flex items-center cursor-pointer">
              <input type="checkbox" v-model="form.showOnHomepage" class="sr-only" />
              <span
                class="flex items-center rounded-full transition-colors duration-200"
                :class="form.showOnHomepage ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-gray-600'"
                style="width: 40px; height: 22px; flex-shrink: 0;"
              >
                <span
                  class="bg-white rounded-full shadow transition-transform duration-200"
                  :class="form.showOnHomepage ? 'translate-x-[19px]' : 'translate-x-[3px]'"
                  style="width: 16px; height: 16px;"
                ></span>
              </span>
            </label>
          </div>
          <div class="flex items-center justify-between pt-2">
            <div>
              <div class="text-sm font-medium" style="color: var(--text-color);">{{ t('admin.service.formInsecureSkipVerify') }}</div>
              <div class="text-xs mt-0.5" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.service.formInsecureSkipVerifyHint') }}</div>
            </div>
            <label class="relative inline-flex items-center cursor-pointer flex-shrink-0">
              <input type="checkbox" v-model="form.insecureSkipVerify" class="sr-only" />
              <span
                class="flex items-center rounded-full transition-colors duration-200"
                :class="form.insecureSkipVerify ? 'bg-amber-500' : 'bg-gray-300 dark:bg-gray-600'"
                style="width: 40px; height: 22px; flex-shrink: 0;"
              >
                <span
                  class="bg-white rounded-full shadow transition-transform duration-200"
                  :class="form.insecureSkipVerify ? 'translate-x-[19px]' : 'translate-x-[3px]'"
                  style="width: 16px; height: 16px;"
                ></span>
              </span>
            </label>
          </div>

          <!-- HTTP 探测高级匹配 -->
          <div v-if="form.type === 'http'" class="pt-2" style="border-top: 1px solid var(--button-border-color);">
            <button
              type="button"
              @click="showAdvanced = !showAdvanced"
              class="w-full flex items-center justify-between text-sm font-medium"
              style="color: var(--text-color);"
            >
              <span>{{ t('admin.service.formAdvanced') }}</span>
              <svg class="w-4 h-4 transition-transform" :class="showAdvanced ? 'rotate-180' : ''" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2" style="opacity: 0.4;">
                <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
              </svg>
            </button>
            <p class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">
              {{ t('admin.service.formAdvancedHint') }}
            </p>

            <div v-if="showAdvanced" class="space-y-4 mt-4">
              <div class="grid grid-cols-2 gap-4">
                <div>
                  <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.service.formHttpMethod') }}</label>
                  <CustomSelect
                    v-model="form.httpMethod"
                    :options="['GET', 'HEAD', 'POST', 'PUT', 'PATCH', 'DELETE', 'OPTIONS'].map(m => ({ label: m, value: m }))"
                  />
                </div>
                <div>
                  <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.service.formExpectStatus') }}</label>
                  <input v-model="form.expectStatus" placeholder="200-399" class="input-field w-full px-3 py-2 rounded-lg text-sm" />
                </div>
              </div>
              <p class="text-xs -mt-2" style="color: var(--text-color); opacity: 0.4;">
                {{ t('admin.service.formExpectStatusHint') }}
              </p>

              <div>
                <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.service.formExpectKeyword') }}</label>
                <input v-model="form.expectKeyword" :placeholder="keywordPlaceholder" class="input-field w-full px-3 py-2 rounded-lg text-sm" />
                <p class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">
                  {{ t('admin.service.formExpectKeywordHint') }}
                </p>
              </div>

              <div>
                <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.service.formHttpHeaders') }}</label>
                <textarea
                  v-model="form.httpHeaders"
                  rows="3"
                  :placeholder="headersPlaceholder"
                  class="input-field w-full px-3 py-2 rounded-lg text-sm font-mono"
                ></textarea>
                <p class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">
                  {{ t('admin.service.formHttpHeadersHint') }}
                </p>
              </div>

              <div>
                <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.service.formHttpBody') }}</label>
                <textarea
                  v-model="form.httpBody"
                  rows="3"
                  :placeholder="editing ? t('admin.service.formBodyPlaceholderKeep') : t('admin.service.formBodyPlaceholderNew')"
                  class="input-field w-full px-3 py-2 rounded-lg text-sm font-mono"
                ></textarea>
                <p class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">
                  {{ t('admin.service.formHttpBodyHint') }}
                </p>
              </div>
            </div>
          </div>
        </div>
        <div class="flex justify-end gap-3 mt-6">
          <button @click="handleCloseSvc" class="btn-cancel px-4 py-2 text-sm rounded-lg">{{ t('admin.service.cancel') }}</button>
          <button @click="save" class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium rounded-lg transition-colors">{{ t('admin.service.save') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>
