<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import type { Incident, ServiceSummary } from '../api/types'
import { siteName, siteIcon, emailEnabled, showAdminButton, customFooter, subEnabledAny, subEnableEmail, subEnableRss, subEnableAtom } from '../composables/useSiteConfig'
import { useDarkMode } from '../composables/useDarkMode'

const route = useRoute()
const router = useRouter()
const { isDark, themeMode, setMode } = useDarkMode()

const incident = ref<Incident | null>(null)
const serviceNames = ref<string[]>([])
const loading = ref(true)
const error = ref('')

const showThemeMenu = ref(false)
const showSubscribeModal = ref(false)
const subscribeTab = ref<'rss' | 'atom' | 'email'>('rss')
const showServiceSelect = ref(false)
const subscribeEmail = ref('')
const subscribing = ref(false)
const subscribeMsg = ref('')
const subscribeMsgType = ref('success')
const selectedServices = ref<number[]>([])
const servicesList = ref<ServiceSummary[]>([])

async function openSubscribe() {
  subscribeEmail.value = ''
  subscribeMsg.value = ''
  selectedServices.value = []
  showServiceSelect.value = false
  // Pick first available tab
  subscribeTab.value = subEnableEmail.value ? 'email' : subEnableRss.value ? 'rss' : 'atom'
  // Load services for selection
  try {
    const res = await api.getPublicServices()
    servicesList.value = res.data || []
  } catch {
    servicesList.value = []
  }
  showSubscribeModal.value = true
}

async function handleSubscribe() {
  const email = subscribeEmail.value.trim()
  if (!email) return
  subscribing.value = true
  subscribeMsg.value = ''
  try {
    await api.subscribe(email, selectedServices.value.length > 0 ? selectedServices.value : undefined)
    subscribeMsg.value = '订阅成功！我们将通过邮件通知您服务状态变更。'
    subscribeMsgType.value = 'success'
    subscribeEmail.value = ''
    selectedServices.value = []
  } catch (e: any) {
    subscribeMsg.value = e.message || '订阅失败，请稍后重试'
    subscribeMsgType.value = 'error'
  } finally {
    subscribing.value = false
  }
}

function closeSubscribeModal() {
  showSubscribeModal.value = false
  subscribeMsg.value = ''
}

function toggleService(id: number) {
  const idx = selectedServices.value.indexOf(id)
  if (idx >= 0) {
    selectedServices.value.splice(idx, 1)
  } else {
    selectedServices.value.push(id)
  }
}

function getFeedUrl(type: 'rss' | 'atom') {
  return `${window.location.protocol}//${window.location.host}/api/v1/feed/${type}`
}

function copyFeedUrl(type: 'rss' | 'atom') {
  navigator.clipboard.writeText(getFeedUrl(type)).then(() => {
    subscribeMsg.value = '链接已复制到剪贴板'
    subscribeMsgType.value = 'success'
    setTimeout(() => { subscribeMsg.value = '' }, 2000)
  })
}

function closeThemeMenu() {
  showThemeMenu.value = false
}

const impactText: Record<string, string> = {
  minor: '轻微',
  major: '重大',
  critical: '严重',
}

const incidentStatusLabel: Record<string, string> = {
  investigating: '调查中',
  identified: '已确认',
  monitoring: '监控中',
  resolved: '已解决',
}

const dateTimeOpts: Intl.DateTimeFormatOptions = { timeZone: 'Asia/Shanghai' }

function formatDate(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleDateString('zh-CN', { ...dateTimeOpts, year: 'numeric', month: 'long', day: 'numeric' })
}

function formatTime(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleTimeString('zh-CN', { ...dateTimeOpts, hour: '2-digit', minute: '2-digit' })
}

function formatDateTime(iso: string): string {
  return `${formatDate(iso)} ${formatTime(iso)}`
}

onMounted(async () => {
  document.addEventListener('click', closeThemeMenu)
  // 公开详情页使用随机 hash 访问，不再暴露自增 ID
  const hash = String(route.params.hash || '').trim()
  if (!hash) {
    error.value = '无效的事件标识'
    loading.value = false
    return
  }
  try {
    const [incRes, sumRes] = await Promise.all([
      api.getPublicIncident(hash),
      api.getSummary(),
    ])
    incident.value = incRes.data

    // Resolve affected service names
    const allSvcs = sumRes.data.services
    const affectedIds = incRes.data.affectedServices
      ? incRes.data.affectedServices.split(',').map(Number).filter(Boolean)
      : [incRes.data.serviceId]
    serviceNames.value = affectedIds.map(id => {
      const svc = allSvcs.find((s: ServiceSummary) => s.id === id)
      return svc ? svc.name : `服务 #${id}`
    })
  } catch (e: any) {
    error.value = e.message || '加载事件详情失败'
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <nav class="mt-4" style="background-color: var(--bg-color);">
    <div class="max-w-[1000px] mx-auto px-6 h-16 flex items-center justify-between">
      <div class="flex items-center gap-2">
        <img v-if="siteIcon" :src="siteIcon" class="w-6 h-6 object-contain" />
        <img v-else src="/assets/logo.svg" class="w-6 h-6 object-contain" />
        <span class="text-xl font-bold tracking-tight" style="color: var(--text-color);">{{ siteName }}</span>
      </div>
      <div class="flex items-center gap-3">
        <a v-if="subEnabledAny" href="#" @click.prevent="openSubscribe" class="header-btn px-4 py-2 text-sm font-medium rounded-lg">订阅更新</a>
        <div class="relative">
          <button
            @click.stop="showThemeMenu = !showThemeMenu"
            class="header-btn px-2 py-2 rounded-lg"
            title="切换主题"
          >
            <svg v-if="themeMode === 'light'" class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
            </svg>
            <svg v-else-if="themeMode === 'dark'" class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M20.985 12.486a9 9 0 1 1-9.473-9.472c.405-.022.617.46.402.803a6 6 0 0 0 8.268 8.268c.344-.215.825-.004.803.401" />
            </svg>
            <svg v-else class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 17.25v1.007a3 3 0 01-.879 2.122L7.5 21h9l-.621-.621A3 3 0 0115 18.257V17.25m6-12V15a2.25 2.25 0 01-2.25 2.25H5.25A2.25 2.25 0 013 15V5.25A2.25 2.25 0 015.25 3h13.5A2.25 2.25 0 0121 5.25z" />
            </svg>
          </button>
          <div
            v-if="showThemeMenu"
            class="absolute right-0 top-full mt-2 rounded-lg shadow-lg py-1.5 px-1.5 z-50 flex flex-col gap-0.5"
            :style="{ backgroundColor: 'var(--bg-color)', border: '1px solid var(--button-border-color)', width: '140px' }"
            @click.stop
          >
            <button
              @click="setMode('light'); showThemeMenu = false"
              class="w-full text-left px-3 py-2 text-sm flex items-center gap-2 transition-colors rounded-md"
              :style="{ color: 'var(--text-color)', backgroundColor: themeMode === 'light' ? 'var(--button-hover-color)' : 'transparent' }"
            >
              <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" /></svg>
              浅色模式
            </button>
            <button
              @click="setMode('dark'); showThemeMenu = false"
              class="w-full text-left px-3 py-2 text-sm flex items-center gap-2 transition-colors rounded-md"
              :style="{ color: 'var(--text-color)', backgroundColor: themeMode === 'dark' ? 'var(--button-hover-color)' : 'transparent' }"
            >
              <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20.985 12.486a9 9 0 1 1-9.473-9.472c.405-.022.617.46.402.803a6 6 0 0 0 8.268 8.268c.344-.215.825-.004.803.401" /></svg>
              深色模式
            </button>
            <button
              @click="setMode('system'); showThemeMenu = false"
              class="w-full text-left px-3 py-2 text-sm flex items-center gap-2 transition-colors rounded-md"
              :style="{ color: 'var(--text-color)', backgroundColor: themeMode === 'system' ? 'var(--button-hover-color)' : 'transparent' }"
            >
              <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M9 17.25v1.007a3 3 0 01-.879 2.122L7.5 21h9l-.621-.621A3 3 0 0115 18.257V17.25m6-12V15a2.25 2.25 0 01-2.25 2.25H5.25A2.25 2.25 0 013 15V5.25A2.25 2.25 0 015.25 3h13.5A2.25 2.25 0 0121 5.25z" /></svg>
              跟随系统
            </button>
          </div>
        </div>
      </div>
    </div>
  </nav>

  <main class="max-w-[1000px] mx-auto px-6 py-8">
    <div v-if="loading" class="text-center py-20" style="color: var(--text-color); opacity: 0.4;">加载中...</div>
    <div v-else-if="error" class="text-center py-20 text-red-500">{{ error }}</div>

    <template v-else-if="incident">
      <div class="flex items-center mb-6">
        <button @click="router.push('/')" class="flex items-center gap-1.5 text-sm transition-opacity" style="color: var(--text-color); opacity: 0.5;" @mouseenter="($event.currentTarget as HTMLElement).style.opacity = '0.8'" @mouseleave="($event.currentTarget as HTMLElement).style.opacity = '0.5'">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7" /></svg>
          返回状态页
        </button>
      </div>

      <!-- Info card -->
      <div class="rounded-lg p-6 mb-6" style="border: 1px solid var(--button-border-color);">
        <div class="flex flex-col sm:flex-row sm:items-start sm:justify-between mb-4">
          <div>
            <h1 class="text-2xl font-bold" :class="{
              'text-red-500 dark:text-red-400': incident.impact === 'critical',
              'text-orange-500 dark:text-orange-400': incident.impact === 'major',
              'text-yellow-500 dark:text-yellow-400': incident.impact === 'minor',
            }">{{ incident.title }}</h1>
            <div class="flex items-center gap-3 mt-2">
              <span class="text-sm font-medium px-2 py-0.5 rounded" :class="{
                'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400': incident.impact === 'critical',
                'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400': incident.impact === 'major',
                'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400': incident.impact === 'minor',
              }">{{ impactText[incident.impact] || incident.impact }}</span>
              <span class="text-sm font-medium px-2 py-0.5 rounded" :class="{
                'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400': incident.status === 'investigating' || incident.status === 'identified',
                'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400': incident.status === 'monitoring',
                'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400': incident.status === 'resolved',
              }">{{ incidentStatusLabel[incident.status] || incident.status }}</span>
            </div>
          </div>
        </div>
        <div class="grid grid-cols-2 md:grid-cols-3 gap-4 text-sm">
          <div>
            <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">受影响服务</div>
            <div class="font-medium" style="color: var(--text-color);">
              <span v-for="(name, idx) in serviceNames" :key="idx">{{ name }}<span v-if="idx < serviceNames.length - 1">, </span></span>
            </div>
          </div>
          <div>
            <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">创建时间</div>
            <div class="font-medium" style="color: var(--text-color);">{{ formatDateTime(incident.createdAt) }}</div>
          </div>
          <div>
            <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">最后更新</div>
            <div class="font-medium" style="color: var(--text-color);">{{ formatDateTime(incident.updatedAt) }}</div>
          </div>
        </div>
      </div>

      <!-- Timeline -->
      <div class="rounded-lg p-6" style="border: 1px solid var(--button-border-color);">
        <h2 class="text-lg font-bold mb-6" style="color: var(--text-color);">事件时间线</h2>
        <div v-if="incident.updates && incident.updates.length > 0" class="relative">
          <ul class="space-y-4 relative z-10">
            <li v-for="upd in [...incident.updates].reverse()" :key="upd.id" class="relative pl-4">
              <div class="timeline-dot" :style="{ backgroundColor: upd.status === 'investigating' || upd.status === 'identified' ? '#f97316' : upd.status === 'monitoring' ? '#3b82f6' : '#22c55e', border: '2px solid var(--bg-color)' }" />
              <div class="flex flex-col sm:flex-row sm:justify-between sm:items-start">
                <div class="text-sm">
                  <span class="font-bold" style="color: var(--text-color);">{{ incidentStatusLabel[upd.status] || upd.status }}</span>
                  <span class="ml-2" style="color: var(--text-color); opacity: 0.5;">{{ upd.content }}</span>
                </div>
                <div class="text-sm mt-0.5 sm:mt-0 sm:ml-4 sm:flex-shrink-0" style="color: var(--text-color); opacity: 0.4;">{{ formatTime(upd.createdAt) }} CST</div>
              </div>
            </li>
          </ul>
        </div>
        <p v-else class="text-sm" style="color: var(--text-color); opacity: 0.4;">暂无更新记录</p>
      </div>
    </template>
  </main>

  <footer style="background-color: var(--bg-color);" class="mt-4">
    <div class="max-w-[1000px] mx-auto px-6 py-8 flex flex-col md:flex-row justify-between items-center gap-4">
      <div class="text-sm">
        <a href="https://github.com/Motues/LumiPulse" target="_blank" rel="noopener noreferrer" class="footer-link transition-opacity">Powered By LumiPulse</a>
      </div>
      <div class="flex items-center gap-6 text-sm">
        <span v-if="customFooter" v-html="customFooter" class="footer-link"></span>
        <a v-else-if="showAdminButton" href="/dashboard" class="footer-link transition-opacity">管理后台</a>
      </div>
    </div>
  </footer>

  <!-- Subscribe Modal -->
  <Teleport to="body">
    <div v-if="showSubscribeModal" class="fixed inset-0 z-50 flex items-center justify-center" @click.self="closeSubscribeModal">
      <div class="absolute inset-0 bg-black/40" />
      <div class="relative rounded-xl shadow-xl p-6 w-full max-w-md mx-4" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <div class="flex items-center justify-between mb-4">
          <h3 class="text-base font-bold" style="color: var(--text-color);">订阅更新</h3>
          <button @click="closeSubscribeModal" class="p-1 rounded-lg transition-colors hover:bg-[var(--button-hover-color)]" style="color: var(--text-color); opacity: 0.5;">
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
          </button>
        </div>

        <!-- Tabs -->
        <div class="flex border-b mb-4" style="border-color: var(--button-border-color);">
          <button
            v-for="tab in [
              subEnableEmail ? { key: 'email', label: '邮件订阅' } : null,
              subEnableRss ? { key: 'rss', label: 'RSS' } : null,
              subEnableAtom ? { key: 'atom', label: 'Atom' } : null,
            ].filter(Boolean)"
            :key="(tab as any).key"
            @click="subscribeTab = (tab as any).key; subscribeMsg = ''"
            class="px-4 py-2 text-sm font-medium transition-colors -mb-px"
            :class="subscribeTab === (tab as any).key ? 'border-b-2 border-emerald-500 text-emerald-600' : 'opacity-50 hover:opacity-80'"
            style="color: subscribeTab === (tab as any).key ? '' : 'var(--text-color)';"
          >{{ (tab as any).label }}</button>
        </div>

        <!-- Email Tab -->
        <div v-if="subscribeTab === 'email'">
          <p class="text-sm mb-3" style="color: var(--text-color); opacity: 0.6;">获取服务状态变更和事件通知。</p>
            <div class="flex gap-2 mb-3">
              <input
                v-model="subscribeEmail"
                type="email"
                placeholder="输入邮箱地址..."
                class="flex-1 px-4 py-2 rounded-lg text-sm focus:outline-none focus:border-emerald-500"
                style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);"
              />
              <button
                @click="handleSubscribe"
                :disabled="subscribing || !subscribeEmail"
                class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 disabled:opacity-40 text-white text-sm font-medium rounded-lg transition-colors"
              >
                {{ subscribing ? '提交中...' : '订阅' }}
              </button>
            </div>

            <!-- Service selection toggle -->
            <div v-if="servicesList.length > 0">
              <button @click="showServiceSelect = !showServiceSelect" type="button" class="text-xs flex items-center gap-1 transition-colors hover:opacity-80" style="color: var(--text-color); opacity: 0.5;">
                <svg class="w-3.5 h-3.5" :class="showServiceSelect ? 'rotate-90' : ''" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" /></svg>
                {{ showServiceSelect ? '收起' : '选择特定服务' }}
              </button>
              <div v-if="showServiceSelect" class="mt-2">
                <button
                  @click="selectedServices = (selectedServices.length === servicesList.length ? [] : servicesList.map(s => s.id))"
                  class="text-xs mb-2 transition-colors"
                  style="color: var(--text-color); opacity: 0.5;"
                >{{ selectedServices.length === servicesList.length ? '取消全选' : '全选' }}</button>
                <div class="max-h-36 overflow-y-auto space-y-1 thin-scroll p-1">
                  <label
                    v-for="svc in servicesList"
                    :key="svc.id"
                    class="flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm cursor-pointer transition-colors hover:bg-[var(--button-hover-color)]"
                  >
                    <input
                      type="checkbox"
                      :checked="selectedServices.includes(svc.id)"
                      @change="toggleService(svc.id)"
                      class="rounded border-gray-300 text-emerald-500 focus:ring-emerald-500"
                    />
                    <span style="color: var(--text-color);">{{ svc.name }}</span>
                  </label>
                </div>
                <p class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">留空则订阅所有服务</p>
              </div>
            </div>
        </div>

        <!-- RSS Tab -->
        <div v-if="subscribeTab === 'rss'">
          <p class="text-sm mb-3" style="color: var(--text-color); opacity: 0.6;">复制以下链接到 RSS 阅读器订阅状态更新。</p>
          <div class="flex gap-2">
            <input
              :value="getFeedUrl('rss')"
              readonly
              class="flex-1 px-3 py-2 rounded-lg text-xs font-mono focus:outline-none"
              style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);"
            />
            <button @click="copyFeedUrl('rss')" class="px-3 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-xs font-medium rounded-lg transition-colors whitespace-nowrap">
              复制
            </button>
          </div>
        </div>

        <!-- Atom Tab -->
        <div v-if="subscribeTab === 'atom'">
          <p class="text-sm mb-3" style="color: var(--text-color); opacity: 0.6;">复制以下链接到 RSS 阅读器订阅状态更新。</p>
          <div class="flex gap-2">
            <input
              :value="getFeedUrl('atom')"
              readonly
              class="flex-1 px-3 py-2 rounded-lg text-xs font-mono focus:outline-none"
              style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);"
            />
            <button @click="copyFeedUrl('atom')" class="px-3 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-xs font-medium rounded-lg transition-colors whitespace-nowrap">
              复制
            </button>
          </div>
        </div>

        <p v-if="subscribeMsg" :class="['text-xs mt-3', subscribeMsgType === 'success' ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-500']">{{ subscribeMsg }}</p>
        <button
          v-if="subscribeMsgType === 'success' && subscribeTab === 'email'"
          @click="closeSubscribeModal"
          class="mt-3 w-full px-4 py-2 text-sm font-medium rounded-lg modal-close-btn"
        >
          关闭
        </button>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.header-btn {
  color: var(--text-color);
  border: 1px solid var(--button-border-color);
  background-color: var(--bg-color);
  transition: background-color 0.2s;
}
.header-btn:hover {
  background-color: var(--button-hover-color);
}
.footer-link {
  color: var(--text-color);
  opacity: 0.5;
}
.footer-link:hover {
  opacity: 1;
}
.timeline-dot {
  position: absolute;
  left: -5px;
  top: 50%;
  transform: translateY(-50%);
  width: 12px;
  height: 12px;
  border-radius: 50%;
  z-index: 1;
}
</style>
