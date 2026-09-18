<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '../api/client'
import type { ServiceSummary } from '../api/types'
import { subEnableEmail, subEnableRss, subEnableAtom } from '../composables/useSiteConfig'
import { useI18n } from '../composables/useI18n'

/**
 * 公开页共用的「订阅更新」弹窗（首页与事件详情页原本各有一份相同实现）。
 * 文案走 i18n，因此两个页面切换语言后都会同步。
 */
const props = defineProps<{
  /** 详情页需要在打开时单独拉取服务列表（首页可直接复用总览数据） */
  services?: ServiceSummary[]
}>()

const emit = defineEmits<{
  close: []
}>()

const { t } = useI18n()

const subscribeTab = ref<'rss' | 'atom' | 'email'>('rss')
const showServiceSelect = ref(false)
const subscribeEmail = ref('')
const subscribing = ref(false)
const subscribeMsg = ref('')
const subscribeMsgType = ref('success')
const selectedServices = ref<number[]>([])
const servicesList = ref<ServiceSummary[]>([])

onMounted(async () => {
  // Pick first available tab
  subscribeTab.value = subEnableEmail.value ? 'email' : subEnableRss.value ? 'rss' : 'atom'

  if (props.services && props.services.length > 0) {
    servicesList.value = props.services
    return
  }
  try {
    const res = await api.getPublicServices()
    servicesList.value = res.data || []
  } catch {
    servicesList.value = []
  }
})

async function handleSubscribe() {
  const email = subscribeEmail.value.trim()
  if (!email) return
  subscribing.value = true
  subscribeMsg.value = ''
  try {
    await api.subscribe(email, selectedServices.value.length > 0 ? selectedServices.value : undefined)
    subscribeMsg.value = t('subscribe.success')
    subscribeMsgType.value = 'success'
    subscribeEmail.value = ''
    selectedServices.value = []
  } catch (e: any) {
    subscribeMsg.value = e.message || t('subscribe.failed')
    subscribeMsgType.value = 'error'
  } finally {
    subscribing.value = false
  }
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
  return `${window.location.protocol}//${window.location.host}/feed/${type}`
}

function copyFeedUrl(type: 'rss' | 'atom') {
  navigator.clipboard.writeText(getFeedUrl(type)).then(() => {
    subscribeMsg.value = t('subscribe.copied')
    subscribeMsgType.value = 'success'
    setTimeout(() => { subscribeMsg.value = '' }, 2000)
  })
}

function close() {
  subscribeMsg.value = ''
  emit('close')
}

const tabs = [
  { key: 'email' as const, label: 'subscribe.tab.email', enabled: () => subEnableEmail.value },
  { key: 'rss' as const, label: 'RSS', enabled: () => subEnableRss.value },
  { key: 'atom' as const, label: 'Atom', enabled: () => subEnableAtom.value },
]
</script>

<template>
  <Teleport to="body">
    <div class="fixed inset-0 z-50 flex items-center justify-center" @click.self="close">
      <div class="absolute inset-0 bg-black/40" />
      <div class="relative rounded-xl shadow-xl p-6 w-full max-w-md mx-4" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <div class="flex items-center justify-between mb-4">
          <h3 class="text-base font-bold" style="color: var(--text-color);">{{ t('subscribe.title') }}</h3>
          <button @click="close" class="p-1 rounded-lg transition-colors hover:bg-[var(--button-hover-color)]" style="color: var(--text-color); opacity: 0.5;">
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
          </button>
        </div>

        <!-- Tabs -->
        <div class="flex border-b mb-4" style="border-color: var(--button-border-color);">
          <button
            v-for="tab in tabs.filter(x => x.enabled())"
            :key="tab.key"
            @click="subscribeTab = tab.key; subscribeMsg = ''"
            class="px-4 py-2 text-sm font-medium transition-colors -mb-px"
            :class="subscribeTab === tab.key ? 'border-b-2 border-emerald-500 text-emerald-600' : 'opacity-50 hover:opacity-80'"
            style="color: subscribeTab === tab.key ? '' : 'var(--text-color)';"
          >{{ tab.label.startsWith('subscribe.') ? t(tab.label) : tab.label }}</button>
        </div>

        <!-- Email Tab -->
        <div v-if="subscribeTab === 'email'">
          <p class="text-sm mb-3" style="color: var(--text-color); opacity: 0.6;">{{ t('subscribe.desc') }}</p>
          <div class="flex gap-2 mb-3">
            <input
              v-model="subscribeEmail"
              type="email"
              :placeholder="t('subscribe.emailPlaceholder')"
              class="flex-1 px-4 py-2 rounded-lg text-sm focus:outline-none focus:border-emerald-500"
              style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);"
            />
            <button
              @click="handleSubscribe"
              :disabled="subscribing || !subscribeEmail"
              class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 disabled:opacity-40 text-white text-sm font-medium rounded-lg transition-colors"
            >
              {{ subscribing ? t('subscribe.submitting') : t('subscribe.submit') }}
            </button>
          </div>

          <!-- Service selection toggle -->
          <div v-if="servicesList.length > 0">
            <button @click="showServiceSelect = !showServiceSelect" type="button" class="text-xs flex items-center gap-1 transition-colors hover:opacity-80" style="color: var(--text-color); opacity: 0.5;">
              <svg class="w-3.5 h-3.5" :class="showServiceSelect ? 'rotate-90' : ''" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" /></svg>
              {{ showServiceSelect ? t('subscribe.collapse') : t('subscribe.selectServices') }}
            </button>
            <div v-if="showServiceSelect" class="mt-2">
              <button
                @click="selectedServices = (selectedServices.length === servicesList.length ? [] : servicesList.map(s => s.id))"
                class="text-xs mb-2 transition-colors"
                style="color: var(--text-color); opacity: 0.5;"
              >{{ selectedServices.length === servicesList.length ? t('subscribe.clearAll') : t('subscribe.selectAll') }}</button>
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
              <p class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">{{ t('subscribe.emptyMeansAll') }}</p>
            </div>
          </div>
        </div>

        <!-- RSS / Atom Tabs -->
        <div v-if="subscribeTab === 'rss' || subscribeTab === 'atom'">
          <p class="text-sm mb-3" style="color: var(--text-color); opacity: 0.6;">{{ t('subscribe.feedDesc') }}</p>
          <div class="flex gap-2">
            <input
              :value="getFeedUrl(subscribeTab)"
              readonly
              class="flex-1 px-3 py-2 rounded-lg text-xs font-mono focus:outline-none"
              style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);"
            />
            <button @click="copyFeedUrl(subscribeTab)" class="px-3 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-xs font-medium rounded-lg transition-colors whitespace-nowrap">
              {{ t('common.copy') }}
            </button>
          </div>
        </div>

        <p v-if="subscribeMsg" :class="['text-xs mt-3', subscribeMsgType === 'success' ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-500']">{{ subscribeMsg }}</p>
        <button
          v-if="subscribeMsgType === 'success' && subscribeTab === 'email'"
          @click="close"
          class="mt-3 w-full px-4 py-2 text-sm font-medium rounded-lg modal-close-btn"
        >
          {{ t('common.close') }}
        </button>
      </div>
    </div>
  </Teleport>
</template>
