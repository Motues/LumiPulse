<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '../../api/client'
import type { Service } from '../../api/types'
import type { SectionGroup } from '../../types'
import { useToast } from '../../composables/useToast'
import { useI18n } from '../../composables/useI18n'
import DateTimePicker from './DateTimePicker.vue'
import CustomSelect from './CustomSelect.vue'
import SectionNav from './SectionNav.vue'

const { show: toast } = useToast()
const { t } = useI18n()

const settings = ref<Record<string, string>>({})
const services = ref<(Service & { uptime: number; latency: number })[]>([])
const loading = ref(true)
const saving = ref(false)
const testing = ref(false)

// 二级导航：通知设置内容较多，按功能分类，一次只展示一类
const activeNav = ref('smtp')

const navGroups = computed<SectionGroup[]>(() => [
  {
    title: t('admin.notify.nav.groupBasics'),
    items: [
      { id: 'smtp', label: t('admin.notify.nav.smtp'), icon: 'M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z' },
      { id: 'test-email', label: t('admin.notify.nav.testEmail'), icon: 'M12 19l9 2-9-18-9 18 9-2zm0 0v-8' },
      { id: 'quiet', label: t('admin.notify.nav.quiet'), icon: 'M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z' },
      { id: 'escalation', label: t('admin.notify.nav.escalation'), icon: 'M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z' },
    ],
  },
  {
    title: t('admin.notify.nav.groupChannels'),
    items: [
      { id: 'service', label: t('admin.notify.nav.service'), icon: 'M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9' },
      { id: 'webhook', label: t('admin.notify.nav.webhook'), icon: 'M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1' },
    ],
  },
  {
    title: t('admin.notify.nav.groupReports'),
    items: [
      { id: 'sla-report', label: t('admin.notify.nav.slaReport'), icon: 'M9 17V9m4 8V5m4 12v-5M5 21h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v14a2 2 0 002 2z' },
      { id: 'weekly-report', label: t('admin.notify.nav.weeklyReport'), icon: 'M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z' },
      { id: 'template', label: t('admin.notify.nav.template'), icon: 'M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z' },
    ],
  },
])

/** 当前二级导航所属分组标题，用于面包屑 */
const activeGroupTitle = computed(
  () => navGroups.value.find(g => g.items.some(i => i.id === activeNav.value))?.title || '',
)
/** 当前二级导航条目名称，用于面包屑 */
const activeNavLabel = computed(
  () => navGroups.value.flatMap(g => g.items).find(i => i.id === activeNav.value)?.label || '',
)

// Test email
const testTo = ref('')

// Webhook
const testingWebhook = ref(false)

const webhookTypes = computed<{ key: string; label: string; hint: string; placeholder: string }[]>(() => [
  { key: 'generic', label: t('admin.webhook.channelGeneric'), hint: t('admin.webhook.hintGeneric'), placeholder: 'https://example.com/hooks/lumipulse' },
  { key: 'slack', label: t('admin.webhook.channelSlack'), hint: t('admin.webhook.hintSlack'), placeholder: 'https://hooks.slack.com/services/...' },
  { key: 'discord', label: t('admin.webhook.channelDiscord'), hint: t('admin.webhook.hintDiscord'), placeholder: 'https://discord.com/api/webhooks/...' },
  { key: 'telegram', label: t('admin.webhook.channelTelegram'), hint: t('admin.webhook.hintTelegram'), placeholder: 'https://api.telegram.org/bot<token>/sendMessage' },
])

const webhookTypeHint = computed(
  () => webhookTypes.value.find(w => w.key === (settings.value['webhook_type'] || 'generic'))?.hint || '',
)
const webhookTypePlaceholder = computed(
  () => webhookTypes.value.find(w => w.key === (settings.value['webhook_type'] || 'generic'))?.placeholder || '',
)

/** webhook_events 以逗号分隔存储；空值在后端视为「所有事件都订阅」 */
const webhookEvents = computed<string[]>({
  get: () => (settings.value['webhook_events'] || 'down,up,maintenance,cert_expiring').split(',').filter(Boolean),
  set: (val: string[]) => { settings.value['webhook_events'] = val.join(',') },
})

function toggleWebhookEvent(event: string) {
  const current = webhookEvents.value
  webhookEvents.value = current.includes(event)
    ? current.filter(e => e !== event)
    : [...current, event]
}

/** 周报发送日：值与后端约定一致（1=周一 … 7=周日） */
const weekdayOptions = computed(() => [
  { label: t('admin.notify.weekdayMon'), value: '1' },
  { label: t('admin.notify.weekdayTue'), value: '2' },
  { label: t('admin.notify.weekdayWed'), value: '3' },
  { label: t('admin.notify.weekdayThu'), value: '4' },
  { label: t('admin.notify.weekdayFri'), value: '5' },
  { label: t('admin.notify.weekdaySat'), value: '6' },
  { label: t('admin.notify.weekdaySun'), value: '7' },
])

/**
 * 通知模板：每类通知可分别自定义主题与正文，留空则用内置中英文案。
 * 变量用 {{name}} 形式；邮件与 webhook 共用同一份渲染结果。
 */
const templateGroups = computed<{
  key: string
  title: string
  subjectKey: string
  bodyKey: string
  subjectPlaceholder: string
  bodyPlaceholder: string
  vars: { name: string; desc: string }[]
}[]>(() => [
  {
    key: 'alert',
    title: t('admin.template.groupAlert'),
    subjectKey: 'tpl_alert_subject',
    bodyKey: 'tpl_alert_body',
    subjectPlaceholder: t('admin.template.alertSubjectPh'),
    bodyPlaceholder: t('admin.template.alertBodyPh'),
    vars: [
      { name: 'service', desc: t('admin.template.varService') },
      { name: 'url', desc: t('admin.template.varUrl') },
      { name: 'time', desc: t('admin.template.varEventTime') },
      { name: 'site', desc: t('admin.template.varSite') },
    ],
  },
  {
    key: 'resolved',
    title: t('admin.template.groupResolved'),
    subjectKey: 'tpl_resolved_subject',
    bodyKey: 'tpl_resolved_body',
    subjectPlaceholder: t('admin.template.resolvedSubjectPh'),
    bodyPlaceholder: t('admin.template.resolvedBodyPh'),
    vars: [
      { name: 'service', desc: t('admin.template.varService') },
      { name: 'url', desc: t('admin.template.varUrl') },
      { name: 'duration', desc: t('admin.template.varDuration') },
      { name: 'time', desc: t('admin.template.varRecoveryTime') },
      { name: 'site', desc: t('admin.template.varSite') },
    ],
  },
  {
    key: 'maintenance',
    title: t('admin.template.groupMaintenance'),
    subjectKey: 'tpl_maintenance_subject',
    bodyKey: 'tpl_maintenance_body',
    subjectPlaceholder: t('admin.template.maintenanceSubjectPh'),
    bodyPlaceholder: t('admin.template.maintenanceBodyPh'),
    vars: [
      { name: 'title', desc: t('admin.template.varTitle') },
      { name: 'start', desc: t('admin.template.varStart') },
      { name: 'countdown', desc: t('admin.template.varRemaining') },
      { name: 'site', desc: t('admin.template.varSite') },
    ],
  },
  {
    key: 'cert',
    title: t('admin.template.groupCert'),
    subjectKey: 'tpl_cert_subject',
    bodyKey: 'tpl_cert_body',
    subjectPlaceholder: t('admin.template.certSubjectPh'),
    bodyPlaceholder: t('admin.template.certBodyPh'),
    vars: [
      { name: 'service', desc: t('admin.template.varService') },
      { name: 'expires', desc: t('admin.template.varExpires') },
      { name: 'remaining', desc: t('admin.template.varRemaining') },
      { name: 'site', desc: t('admin.template.varSite') },
    ],
  },
  {
    key: 'escalation',
    title: t('admin.template.groupEscalation'),
    subjectKey: 'tpl_escalation_subject',
    bodyKey: 'tpl_escalation_body',
    subjectPlaceholder: t('admin.template.escalationSubjectPh'),
    bodyPlaceholder: t('admin.template.escalationBodyPh'),
    vars: [
      { name: 'title', desc: t('admin.template.varTitle') },
      { name: 'service', desc: t('admin.template.varService') },
      { name: 'url', desc: t('admin.template.varUrl') },
      { name: 'unhandled', desc: t('admin.template.varUnhandled') },
      { name: 'count', desc: t('admin.template.varCount') },
      { name: 'time', desc: t('admin.template.varNotifyTime') },
      { name: 'site', desc: t('admin.template.varSite') },
    ],
  },
])

/** 折叠状态：默认收起，避免设置页过长 */
const openTemplate = ref('')

/** 变量占位符文案（写成函数，避免在模板里拼接花括号让编译器误判插值） */
function varToken(name: string): string {
  return `{{${name}}}`
}

// Service notification select
const notifyServiceIds = ref<number[]>([])
const serviceSearch = ref('')
const showServiceDropdown = ref(false)

const filteredServices = computed(() => {
  if (!serviceSearch.value) return services.value
  const q = serviceSearch.value.toLowerCase()
  return services.value.filter(s => s.name.toLowerCase().includes(q))
})

const smtpFields = computed<{ key: string; label: string; type: string; placeholder: string }[]>(() => [
  { key: 'smtp_host', label: t('admin.notify.smtpHostLabel'), type: 'text', placeholder: 'smtp.example.com' },
  { key: 'smtp_port', label: t('admin.notify.smtpPortLabel'), type: 'text', placeholder: '587' },
  { key: 'smtp_user', label: t('admin.notify.smtpUserLabel'), type: 'text', placeholder: 'user@example.com' },
  { key: 'smtp_pass', label: t('admin.notify.smtpPassLabel'), type: 'password', placeholder: t('admin.notify.smtpPassPlaceholder') },
])

/** SMTP 安全连接选项 */
const smtpEncryptionOptions = computed(() => [
  { label: t('admin.notify.smtpEncryptionNone'), value: '' },
  { label: t('admin.notify.smtpEncryptionStarttls'), value: 'starttls' },
  { label: t('admin.notify.smtpEncryptionTls'), value: 'tls' },
])

/** 报告语言选项 */
const reportLanguageOptions = computed(() => [
  { label: t('admin.notify.reportLanguageDefault'), value: '' },
  { label: t('admin.notify.reportLanguageZh'), value: 'zh-CN' },
  { label: t('admin.notify.reportLanguageEn'), value: 'en-US' },
])

async function load() {
  loading.value = true
  try {
    const [settingsRes, servicesRes] = await Promise.all([
      api.getSettings(),
      api.getAdminServices(),
    ])
    settings.value = settingsRes.data
    services.value = servicesRes.data

    // Parse notify_services
    const raw = settingsRes.data['notify_services'] || ''
    notifyServiceIds.value = raw ? raw.split(',').map(Number) : []
  } catch (e: any) {
    toast(e.message || t('admin.notify.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function save() {
  // Serialize notify_services before saving
  settings.value['notify_services'] = notifyServiceIds.value.join(',')

  saving.value = true
  try {
    await api.updateSettings(settings.value)
    toast(t('admin.notify.saveSuccess'), 'success')
  } catch (e: any) {
    toast(e.message || t('admin.notify.saveFailed'))
  } finally {
    saving.value = false
  }
}

function toggleNotifyService(svc: { id: number }) {
  const idx = notifyServiceIds.value.indexOf(svc.id)
  if (idx >= 0) {
    notifyServiceIds.value.splice(idx, 1)
  } else {
    notifyServiceIds.value.push(svc.id)
  }
  serviceSearch.value = ''
}

function delayBlur() {
  setTimeout(() => { showServiceDropdown.value = false }, 200)
}

async function testEmail() {
  if (!testTo.value) return
  testing.value = true
  try {
    await api.testEmail(testTo.value)
    toast(t('admin.notify.testEmailSuccess'), 'success')
  } catch (e: any) {
    toast(e.message || t('admin.notify.sendFailed'))
  } finally {
    testing.value = false
  }
}

async function testWebhook() {
  testingWebhook.value = true
  try {
    await api.testWebhook()
    toast(t('admin.notify.testWebhookSuccess'), 'success')
  } catch (e: any) {
    toast(e.message || t('admin.notify.sendFailed'))
  } finally {
    testingWebhook.value = false
  }
}

onMounted(load)
</script>

<template>
  <div>

    <div v-if="loading" class="text-center py-12" style="color: var(--text-color); opacity: 0.4;">{{ t('common.loading') }}</div>

    <div v-else class="flex flex-col gap-4 md:flex-row md:gap-6 md:items-start">
      <SectionNav :active="activeNav" :groups="navGroups" @select="activeNav = $event" />

      <div class="flex-1 min-w-0">
        <div class="text-xs mb-4" style="color: var(--text-color); opacity: 0.4;">
          {{ activeGroupTitle }} / {{ activeNavLabel }}
        </div>
        <!-- 下面 9 个面板用 v-if / v-else-if 链式互斥：同一时刻只有一个分支被渲染。
             不要把它们拆成各自独立的 v-if —— 相邻兄弟节点上的独立 v-if 不会编译进
             同一个 block，切换时旧面板的 DOM 会被复用，表现为「点了二级标题没内容」。 -->
        <div>
      <!-- SMTP Config -->
      <div v-if="activeNav === 'smtp'" class="rounded-xl p-6" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">{{ t('admin.notify.smtpTitle') }}</h3>
        <div class="space-y-4 max-w-md">
          <div v-for="field in smtpFields" :key="field.key">
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ field.label }}</label>
            <input
              v-model="settings[field.key]"
              :type="field.type"
              :placeholder="field.placeholder"
              class="w-full px-3 py-2 rounded-lg text-sm input-field"
            />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.notify.smtpEncryptionLabel') }}</label>
            <CustomSelect v-model="settings['smtp_encryption']" :options="smtpEncryptionOptions" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.notify.emailEnabled') }}</label>
            <button
              type="button"
              @click="settings['email_enabled'] = settings['email_enabled'] === 'true' ? 'false' : 'true'"
              class="relative inline-flex h-6 w-11 items-center rounded-full flex-shrink-0 transition-colors"
              :class="settings['email_enabled'] === 'true' ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-gray-600'"
            >
              <span
                class="inline-block h-4 w-4 transform rounded-full bg-white transition"
                :class="settings['email_enabled'] === 'true' ? 'translate-x-[22px]' : 'translate-x-[3px]'"
              />
            </button>
          </div>
          <button
            @click="save"
            :disabled="saving"
            class="px-6 py-2 bg-emerald-500 hover:bg-emerald-600 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
            :style="saving ? { opacity: 0.3 } : {}"
          >
            {{ saving ? t('admin.notify.saving') : t('admin.notify.saveConfig') }}
          </button>
        </div>
      </div>

      <!-- Service Notification Config -->
      <div v-else-if="activeNav === 'service'" class="rounded-xl p-6" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">{{ t('admin.notify.serviceTitle') }}</h3>
        <p class="text-sm mb-4" style="color: var(--text-color); opacity: 0.5;">{{ t('admin.notify.serviceDesc') }}</p>

        <div class="space-y-4 max-w-md">
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.notify.serviceMonitorLabel') }}</label>
            <div class="relative">
              <div class="w-full px-3 py-2 border rounded-lg text-sm cursor-text focus-within:border-emerald-500 min-h-[38px] flex flex-wrap gap-1" style="border-color: var(--button-border-color); background-color: var(--bg-color);">
                <span
                  v-for="id in notifyServiceIds" :key="id"
                  class="inline-flex items-center gap-1 px-2 py-0.5 bg-emerald-50 dark:bg-emerald-500/15 text-emerald-700 dark:text-emerald-400 rounded text-xs"
                >
                  {{ services.find(s => s.id === id)?.name || id }}
                  <button @click="toggleNotifyService({ id })" class="ml-0.5 hover:text-emerald-900 dark:hover:text-emerald-300">&times;</button>
                </span>
                <input
                  v-model="serviceSearch"
                  @focus="showServiceDropdown = true"
                  @blur="delayBlur"
                  type="text"
                  :placeholder="t('admin.notify.serviceSearchPlaceholder')"
                  class="border-0 outline-none text-sm flex-1 min-w-[80px] bg-transparent"
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
                  @mousedown.prevent="toggleNotifyService(svc)"
                  class="svc-opt flex items-center justify-between"
                  :class="{ active: notifyServiceIds.includes(svc.id) }"
                >
                  <span>{{ svc.name }}</span>
                  <span v-if="notifyServiceIds.includes(svc.id)" class="text-emerald-500">✓</span>
                </div>
                <div v-if="filteredServices.length === 0" class="px-3 py-2 text-sm" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.notify.serviceNoMatch') }}</div>
              </div>
            </div>
          </div>

          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.notify.notifyEmailsLabel') }}</label>
            <p class="text-xs mb-1" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.notify.notifyEmailsHint') }}</p>
            <input
              v-model="settings['notify_emails']"
              type="text"
              placeholder="admin@example.com,backup@example.com"
              class="w-full px-3 py-2 rounded-lg text-sm input-field"
            />
          </div>

          <div>
            <button
              @click="save"
              :disabled="saving"
              class="px-6 py-2 bg-emerald-500 hover:bg-emerald-600 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
              :style="saving ? { opacity: 0.3 } : {}"
            >
              {{ saving ? t('admin.notify.saving') : t('admin.notify.saveConfig') }}
            </button>
          </div>
        </div>
      </div>

      <!-- 告警静默与免打扰 -->
      <div v-else-if="activeNav === 'quiet'" class="rounded-xl p-6" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">{{ t('admin.notify.quietTitle') }}</h3>
        <p class="text-sm mb-4" style="color: var(--text-color); opacity: 0.5;">
          {{ t('admin.notify.quietDesc') }}
        </p>

        <div class="space-y-4 max-w-md">
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.notify.cooldownLabel') }}</label>
            <input
              v-model="settings['alert_cooldown_minutes']"
              type="number"
              min="0"
              max="1440"
              :placeholder="t('admin.notify.cooldownPlaceholder')"
              class="w-full px-3 py-2 rounded-lg text-sm input-field"
            />
            <p class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">
              {{ t('admin.notify.cooldownHint') }}
            </p>
          </div>

          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.notify.quietHoursLabel') }}</label>
            <div class="flex items-center gap-2">
              <div class="flex-1">
                <DateTimePicker v-model="settings['quiet_hours_start']" mode="time" :placeholder="t('admin.notify.quietHoursStart')" />
              </div>
              <span class="text-sm" style="color: var(--text-color); opacity: 0.5;">{{ t('admin.notify.quietHoursTo') }}</span>
              <div class="flex-1">
                <DateTimePicker v-model="settings['quiet_hours_end']" mode="time" :placeholder="t('admin.notify.quietHoursEnd')" />
              </div>
            </div>
            <p class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">
              {{ t('admin.notify.quietHoursHint1') }}
              <span class="font-medium">critical</span> {{ t('admin.notify.quietHoursHint2') }}
            </p>
          </div>

          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.notify.siteUrlLabel') }}</label>
            <p class="text-xs mb-1" style="color: var(--text-color); opacity: 0.4;">
              {{ t('admin.notify.siteUrlHint') }}
            </p>
            <input
              v-model="settings['site_url']"
              type="text"
              placeholder="https://status.example.com"
              class="w-full px-3 py-2 rounded-lg text-sm input-field"
            />
          </div>

          <div>
            <button
              @click="save"
              :disabled="saving"
              class="px-6 py-2 bg-emerald-500 hover:bg-emerald-600 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
              :style="saving ? { opacity: 0.3 } : {}"
            >
              {{ saving ? t('admin.notify.saving') : t('admin.notify.saveConfig') }}
            </button>
          </div>
        </div>
      </div>

      <!-- 告警升级 -->
      <div v-else-if="activeNav === 'escalation'" class="rounded-xl p-6" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">{{ t('admin.notify.escalationTitle') }}</h3>
        <p class="text-sm mb-4" style="color: var(--text-color); opacity: 0.5;">
          {{ t('admin.notify.escalationDesc') }}
        </p>

        <div class="space-y-4 max-w-md">
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.notify.escalationEnabled') }}</label>
            <button
              type="button"
              @click="settings['alert_escalation_enabled'] = settings['alert_escalation_enabled'] === 'true' ? 'false' : 'true'"
              class="relative inline-flex h-6 w-11 items-center rounded-full flex-shrink-0 transition-colors"
              :class="settings['alert_escalation_enabled'] === 'true' ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-gray-600'"
            >
              <span
                class="inline-block h-4 w-4 transform rounded-full bg-white transition"
                :class="settings['alert_escalation_enabled'] === 'true' ? 'translate-x-[22px]' : 'translate-x-[3px]'"
              />
            </button>
          </div>

          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.notify.escalationMinutesLabel') }}</label>
            <input
              v-model="settings['alert_escalation_minutes']"
              type="number"
              min="1"
              max="10080"
              placeholder="15"
              class="w-full px-3 py-2 rounded-lg text-sm input-field"
            />
            <p class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">
              {{ t('admin.notify.escalationMinutesHint') }}
            </p>
          </div>

          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.notify.escalationRepeatLabel') }}</label>
            <input
              v-model="settings['alert_escalation_repeat_minutes']"
              type="number"
              min="0"
              max="10080"
              :placeholder="t('admin.notify.escalationRepeatPlaceholder')"
              class="w-full px-3 py-2 rounded-lg text-sm input-field"
            />
            <p class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">
              {{ t('admin.notify.escalationRepeatHint1') }}
              <span class="font-medium">critical</span> {{ t('admin.notify.escalationRepeatHint2') }}
            </p>
          </div>

          <button
            @click="save"
            :disabled="saving"
            class="px-6 py-2 bg-emerald-500 hover:bg-emerald-600 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
            :style="saving ? { opacity: 0.3 } : {}"
          >
            {{ saving ? t('admin.notify.saving') : t('admin.notify.saveConfig') }}
          </button>
        </div>
      </div>

      <!-- Test Email -->
      <div v-else-if="activeNav === 'test-email'" class="rounded-xl p-6" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">{{ t('admin.notify.testEmailTitle') }}</h3>
        <p class="text-sm mb-4" style="color: var(--text-color); opacity: 0.5;">{{ t('admin.notify.testEmailDesc') }}</p>
        <div class="flex items-end gap-3 max-w-md">
          <div class="flex-1">
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.notify.testEmailRecipient') }}</label>
            <input v-model="testTo" type="email" placeholder="test@example.com" class="w-full px-3 py-2 rounded-lg text-sm input-field" />
          </div>
          <button
            @click="testEmail"
            :disabled="!testTo || testing"
            class="px-6 py-2 bg-blue-500 hover:bg-blue-600 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors whitespace-nowrap"
            :style="(!testTo || testing) ? { opacity: 0.3 } : {}"
          >
            {{ testing ? t('admin.notify.testEmailSending') : t('admin.notify.testEmailSend') }}
          </button>
        </div>
      </div>

      <!-- Webhook Config -->
      <div v-else-if="activeNav === 'webhook'" class="rounded-xl p-6" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">{{ t('admin.notify.webhookTitle') }}</h3>
        <p class="text-sm mb-4" style="color: var(--text-color); opacity: 0.5;">
          {{ t('admin.notify.webhookDesc') }}
        </p>

        <div class="space-y-4 max-w-md">
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.notify.webhookEnabled') }}</label>
            <button
              type="button"
              @click="settings['webhook_enabled'] = settings['webhook_enabled'] === 'true' ? 'false' : 'true'"
              class="relative inline-flex h-6 w-11 items-center rounded-full flex-shrink-0 transition-colors"
              :class="settings['webhook_enabled'] === 'true' ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-gray-600'"
            >
              <span
                class="inline-block h-4 w-4 transform rounded-full bg-white transition"
                :class="settings['webhook_enabled'] === 'true' ? 'translate-x-[22px]' : 'translate-x-[3px]'"
              />
            </button>
          </div>

          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.notify.webhookTypeLabel') }}</label>
            <CustomSelect
              v-model="settings['webhook_type']"
              :options="webhookTypes.map(w => ({ label: w.label, value: w.key }))"
            />
            <p class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">{{ webhookTypeHint }}</p>
          </div>

          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.notify.webhookUrlLabel') }}</label>
            <input
              v-model="settings['webhook_url']"
              type="text"
              :placeholder="webhookTypePlaceholder"
              class="w-full px-3 py-2 rounded-lg text-sm input-field"
            />
          </div>

          <div v-if="(settings['webhook_type'] || 'generic') === 'telegram'">
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">Telegram chat_id</label>
            <input
              v-model="settings['webhook_telegram_chat_id']"
              type="text"
              placeholder="-1001234567890"
              class="w-full px-3 py-2 rounded-lg text-sm input-field"
            />
          </div>

          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.notify.webhookEventsLabel') }}</label>
            <div class="flex flex-wrap items-center gap-4 text-sm" style="color: var(--text-color);">
              <label class="inline-flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  :checked="webhookEvents.includes('down')"
                  @change="toggleWebhookEvent('down')"
                  class="accent-emerald-500"
                />
                {{ t('admin.notify.webhookEventDown') }}
              </label>
              <label class="inline-flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  :checked="webhookEvents.includes('up')"
                  @change="toggleWebhookEvent('up')"
                  class="accent-emerald-500"
                />
                {{ t('admin.notify.webhookEventUp') }}
              </label>
              <label class="inline-flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  :checked="webhookEvents.includes('maintenance')"
                  @change="toggleWebhookEvent('maintenance')"
                  class="accent-emerald-500"
                />
                {{ t('admin.notify.webhookEventMaintenance') }}
              </label>
              <label class="inline-flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  :checked="webhookEvents.includes('cert_expiring')"
                  @change="toggleWebhookEvent('cert_expiring')"
                  class="accent-emerald-500"
                />
                {{ t('admin.notify.webhookEventCert') }}
              </label>
            </div>
          </div>

          <div v-if="(settings['webhook_type'] || 'generic') === 'generic'">
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.notify.webhookSecretLabel') }}</label>
            <p class="text-xs mb-1" style="color: var(--text-color); opacity: 0.4;">
              {{ t('admin.notify.webhookSecretHint') }}
            </p>
            <input
              v-model="settings['webhook_secret']"
              type="password"
              :placeholder="t('admin.notify.webhookSecretPlaceholder')"
              class="w-full px-3 py-2 rounded-lg text-sm input-field"
            />
          </div>

          <div class="flex items-center gap-3">
            <button
              @click="save"
              :disabled="saving"
              class="px-6 py-2 bg-emerald-500 hover:bg-emerald-600 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
              :style="saving ? { opacity: 0.3 } : {}"
            >
              {{ saving ? t('admin.notify.saving') : t('admin.notify.saveConfig') }}
            </button>
            <button
              @click="testWebhook"
              :disabled="testingWebhook"
              class="px-6 py-2 bg-blue-500 hover:bg-blue-600 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
              :style="testingWebhook ? { opacity: 0.3 } : {}"
            >
              {{ testingWebhook ? t('admin.notify.testEmailSending') : t('admin.notify.testEmailSend') }}
            </button>
          </div>
          <p class="text-xs" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.notify.webhookTestHint') }}</p>
        </div>
      </div>

      <!-- Monthly SLA Report Email -->
      <div v-else-if="activeNav === 'sla-report'" class="rounded-xl p-6" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">{{ t('admin.notify.slaTitle') }}</h3>
        <p class="text-sm mb-4" style="color: var(--text-color); opacity: 0.5;">
          {{ t('admin.notify.slaDesc') }}
        </p>

        <div class="space-y-4 max-w-md">
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.notify.slaEnabled') }}</label>
            <button
              type="button"
              @click="settings['sla_report_enabled'] = settings['sla_report_enabled'] === 'true' ? 'false' : 'true'"
              class="relative inline-flex h-6 w-11 items-center rounded-full flex-shrink-0 transition-colors"
              :class="settings['sla_report_enabled'] === 'true' ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-gray-600'"
            >
              <span
                class="inline-block h-4 w-4 transform rounded-full bg-white transition"
                :class="settings['sla_report_enabled'] === 'true' ? 'translate-x-[22px]' : 'translate-x-[3px]'"
              />
            </button>
          </div>

          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.notify.recipientsLabel') }}</label>
            <p class="text-xs mb-1" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.notify.reportEmailsHint') }}</p>
            <input
              v-model="settings['sla_report_emails']"
              type="text"
              placeholder="admin@example.com"
              class="w-full px-3 py-2 rounded-lg text-sm input-field"
            />
          </div>

          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.notify.reportLanguageLabel') }}</label>
            <CustomSelect
              v-model="settings['sla_report_language']"
              :options="reportLanguageOptions"
            />
          </div>

          <div v-if="settings['last_sla_report_month']" class="text-xs" style="color: var(--text-color); opacity: 0.4;">
            {{ t('admin.notify.slaLastMonth', { month: settings['last_sla_report_month'] }) }}
          </div>

          <button
            @click="save"
            :disabled="saving"
            class="px-6 py-2 bg-emerald-500 hover:bg-emerald-600 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
            :style="saving ? { opacity: 0.3 } : {}"
          >
            {{ saving ? t('admin.notify.saving') : t('admin.notify.saveConfig') }}
          </button>
        </div>
      </div>

      <!-- 周报摘要邮件 -->
      <div v-else-if="activeNav === 'weekly-report'" class="rounded-xl p-6" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">{{ t('admin.notify.weeklyTitle') }}</h3>
        <p class="text-sm mb-4" style="color: var(--text-color); opacity: 0.5;">
          {{ t('admin.notify.weeklyDesc') }}
        </p>

        <div class="space-y-4 max-w-md">
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.notify.weeklyEnabled') }}</label>
            <button
              type="button"
              @click="settings['weekly_report_enabled'] = settings['weekly_report_enabled'] === 'true' ? 'false' : 'true'"
              class="relative inline-flex h-6 w-11 items-center rounded-full flex-shrink-0 transition-colors"
              :class="settings['weekly_report_enabled'] === 'true' ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-gray-600'"
            >
              <span
                class="inline-block h-4 w-4 transform rounded-full bg-white transition"
                :class="settings['weekly_report_enabled'] === 'true' ? 'translate-x-[22px]' : 'translate-x-[3px]'"
              />
            </button>
          </div>

          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.notify.weeklySendDayLabel') }}</label>
            <CustomSelect
              v-model="settings['weekly_report_weekday']"
              :options="weekdayOptions"
            />
            <p class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">
              {{ t('admin.notify.weeklySendDayHint') }}
            </p>
          </div>

          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.notify.recipientsLabel') }}</label>
            <p class="text-xs mb-1" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.notify.reportEmailsHint') }}</p>
            <input
              v-model="settings['weekly_report_emails']"
              type="text"
              placeholder="admin@example.com"
              class="w-full px-3 py-2 rounded-lg text-sm input-field"
            />
          </div>

          <div v-if="settings['weekly_report_last_sent']" class="text-xs" style="color: var(--text-color); opacity: 0.4;">
            {{ t('admin.notify.weeklyLastSent', { time: settings['weekly_report_last_sent'] }) }}
          </div>

          <p class="text-xs" style="color: var(--text-color); opacity: 0.4;">
            {{ t('admin.notify.weeklyLanguageNote') }}
          </p>

          <button
            @click="save"
            :disabled="saving"
            class="px-6 py-2 bg-emerald-500 hover:bg-emerald-600 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
            :style="saving ? { opacity: 0.3 } : {}"
          >
            {{ saving ? t('admin.notify.saving') : t('admin.notify.saveConfig') }}
          </button>
        </div>
      </div>
      <!-- 通知模板 -->
      <div v-else class="rounded-xl p-6" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-1" style="color: var(--text-color);">{{ t('admin.template.title') }}</h3>
        <p class="text-sm mb-4" style="color: var(--text-color); opacity: 0.5;">
          {{ t('admin.template.desc') }}
        </p>

        <div class="space-y-3 max-w-2xl">
          <div
            v-for="g in templateGroups"
            :key="g.key"
            class="rounded-lg"
            style="border: 1px solid var(--button-border-color);"
          >
            <button
              type="button"
              @click="openTemplate = openTemplate === g.key ? '' : g.key"
              class="w-full flex items-center justify-between px-4 py-3 text-left"
            >
              <span class="text-sm font-medium" style="color: var(--text-color);">{{ g.title }}</span>
              <span class="flex items-center gap-2">
                <span
                  v-if="settings[g.subjectKey] || settings[g.bodyKey]"
                  class="text-[10px] px-1.5 py-0.5 rounded bg-emerald-50 text-emerald-600 dark:bg-emerald-500/15 dark:text-emerald-400"
                >{{ t('admin.template.customized') }}</span>
                <svg
                  class="w-4 h-4 transition-transform"
                  :class="openTemplate === g.key ? 'rotate-180' : ''"
                  style="color: var(--text-color); opacity: 0.4;"
                  fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"
                ><path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" /></svg>
              </span>
            </button>

            <div v-if="openTemplate === g.key" class="px-4 pb-4 space-y-3" style="border-top: 1px solid var(--button-border-color);">
              <div class="pt-3">
                <label class="block text-xs font-medium mb-1" style="color: var(--text-color); opacity: 0.6;">{{ t('admin.template.subjectLabel') }}</label>
                <input
                  v-model="settings[g.subjectKey]"
                  type="text"
                  :placeholder="g.subjectPlaceholder"
                  class="w-full px-3 py-2 rounded-lg text-sm input-field"
                />
              </div>
              <div>
                <label class="block text-xs font-medium mb-1" style="color: var(--text-color); opacity: 0.6;">{{ t('admin.template.bodyLabel') }}</label>
                <textarea
                  v-model="settings[g.bodyKey]"
                  rows="4"
                  :placeholder="g.bodyPlaceholder"
                  class="w-full px-3 py-2 rounded-lg text-sm input-field font-mono"
                ></textarea>
              </div>
              <p class="text-xs" style="color: var(--text-color); opacity: 0.45;">
                {{ t('admin.template.varsLabel') }}
                <span
                  v-for="(v, vi) in g.vars"
                  :key="v.name"
                  class="font-mono"
                >{{ varToken(v.name) }}（{{ v.desc }}）<span v-if="vi < g.vars.length - 1">{{ t('admin.template.varSeparator') }}</span></span>
              </p>
            </div>
          </div>

          <button
            @click="save"
            :disabled="saving"
            class="px-6 py-2 bg-emerald-500 hover:bg-emerald-600 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
            :style="saving ? { opacity: 0.3 } : {}"
          >
            {{ saving ? t('admin.notify.saving') : t('admin.notify.saveConfig') }}
          </button>
        </div>
      </div>
        </div>
      </div>
    </div>
  </div>
</template>
