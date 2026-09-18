<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import type { Incident, ServiceSummary } from '../api/types'
import PublicHeader from '../components/PublicHeader.vue'
import PublicFooter from '../components/PublicFooter.vue'
import PublicSubscribeModal from '../components/PublicSubscribeModal.vue'
import { useI18n } from '../composables/useI18n'

const route = useRoute()
const router = useRouter()
const { t, formatDateTime, formatTime } = useI18n()

const incident = ref<Incident | null>(null)
const serviceNames = ref<string[]>([])
const loading = ref(true)
const error = ref('')
const showSubscribeModal = ref(false)

function impactText(impact: string): string {
  const key = `incident.impact.${impact}`
  const label = t(key)
  return label === key ? impact : label
}

function incidentStatusLabel(status: string): string {
  const key = `incident.status.${status}`
  const label = t(key)
  return label === key ? status : label
}

onMounted(async () => {
  // 公开详情页使用随机 hash 访问，不再暴露自增 ID
  const hash = String(route.params.hash || '').trim()
  if (!hash) {
    error.value = t('home.invalidIncident')
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
      return svc ? svc.name : t('home.serviceLabel', { id })
    })
  } catch (e: any) {
    error.value = e.message || t('home.loadIncidentFailed')
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <PublicHeader :on-subscribe="() => (showSubscribeModal = true)" />

  <main class="max-w-[1000px] mx-auto px-6 py-8">
    <div v-if="loading" class="text-center py-20" style="color: var(--text-color); opacity: 0.4;">{{ t('common.loading') }}</div>
    <div v-else-if="error" class="text-center py-20 text-red-500">{{ error }}</div>

    <template v-else-if="incident">
      <div class="flex items-center mb-6">
        <button @click="router.push('/')" class="flex items-center gap-1.5 text-sm transition-opacity" style="color: var(--text-color); opacity: 0.5;" @mouseenter="($event.currentTarget as HTMLElement).style.opacity = '0.8'" @mouseleave="($event.currentTarget as HTMLElement).style.opacity = '0.5'">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7" /></svg>
          {{ t('common.backToStatus') }}
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
              }">{{ impactText(incident.impact) }}</span>
              <span class="text-sm font-medium px-2 py-0.5 rounded" :class="{
                'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400': incident.status === 'investigating' || incident.status === 'identified',
                'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400': incident.status === 'monitoring',
                'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400': incident.status === 'resolved',
              }">{{ incidentStatusLabel(incident.status) }}</span>
            </div>
          </div>
        </div>
        <div class="grid grid-cols-2 md:grid-cols-3 gap-4 text-sm">
          <div>
            <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">{{ t('home.affectedServices') }}</div>
            <div class="font-medium" style="color: var(--text-color);">
              <span v-for="(name, idx) in serviceNames" :key="idx">{{ name }}<span v-if="idx < serviceNames.length - 1">, </span></span>
            </div>
          </div>
          <div>
            <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">{{ t('home.createdAt') }}</div>
            <div class="font-medium" style="color: var(--text-color);">{{ formatDateTime(incident.createdAt) }}</div>
          </div>
          <div>
            <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">{{ t('home.updatedAt') }}</div>
            <div class="font-medium" style="color: var(--text-color);">{{ formatDateTime(incident.updatedAt) }}</div>
          </div>
        </div>
      </div>

      <!-- Timeline -->
      <div class="rounded-lg p-6" style="border: 1px solid var(--button-border-color);">
        <h2 class="text-lg font-bold mb-6" style="color: var(--text-color);">{{ t('home.incidentTimeline') }}</h2>
        <div v-if="incident.updates && incident.updates.length > 0" class="relative">
          <ul class="space-y-4 relative z-10">
            <li v-for="upd in [...incident.updates].reverse()" :key="upd.id" class="relative pl-4">
              <div class="timeline-dot" :style="{ backgroundColor: upd.status === 'investigating' || upd.status === 'identified' ? '#f97316' : upd.status === 'monitoring' ? '#3b82f6' : '#22c55e', border: '2px solid var(--bg-color)' }" />
              <div class="flex flex-col sm:flex-row sm:justify-between sm:items-start">
                <div class="text-sm">
                  <span class="font-bold" style="color: var(--text-color);">{{ incidentStatusLabel(upd.status) }}</span>
                  <span class="ml-2" style="color: var(--text-color); opacity: 0.5;">{{ upd.content }}</span>
                </div>
                <div class="text-sm mt-0.5 sm:mt-0 sm:ml-4 sm:flex-shrink-0" style="color: var(--text-color); opacity: 0.4;">{{ formatTime(upd.createdAt) }} {{ t('common.cst') }}</div>
              </div>
            </li>
          </ul>
        </div>
        <p v-else class="text-sm" style="color: var(--text-color); opacity: 0.4;">{{ t('home.noUpdates') }}</p>
      </div>
    </template>
  </main>

  <PublicFooter />

  <PublicSubscribeModal
    v-if="showSubscribeModal"
    @close="showSubscribeModal = false"
  />
</template>

<style scoped>
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
