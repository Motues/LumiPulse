<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import Sidebar from '../components/dashboard/Sidebar.vue'
import TopBar from '../components/dashboard/TopBar.vue'
import Toast from '../components/dashboard/Toast.vue'
import DashboardHome from '../components/dashboard/DashboardHome.vue'
import ServiceManager from '../components/dashboard/ServiceManager.vue'
import IncidentManager from '../components/dashboard/IncidentManager.vue'
import MaintenanceManager from '../components/dashboard/MaintenanceManager.vue'
import SettingsPanel from '../components/dashboard/SettingsPanel.vue'
import LogViewer from '../components/dashboard/LogViewer.vue'
import UserManager from '../components/dashboard/UserManager.vue'
import NotificationManager from '../components/dashboard/NotificationManager.vue'
import ApiKeyManager from '../components/dashboard/ApiKeyManager.vue'

type Section = 'dashboard' | 'services' | 'probes' | 'logs' | 'incidents' | 'maintenances' | 'users' | 'notifications' | 'settings' | 'api-keys'
const activeSection = ref<Section>('dashboard')
const sidebarCollapsed = ref(false)
const isMobile = ref(false)
const mobileSidebarOpen = ref(false)

function checkMobile() {
  isMobile.value = window.innerWidth < 768
  if (isMobile.value) {
    sidebarCollapsed.value = true
  }
}

onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
})

onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
})

const navGroups = [
  {
    title: '',
    items: [
      { id: 'dashboard' as Section, label: '控制台', icon: 'M3 3h18v18H3V3z M3 9h18 M9 21V9' },
    ],
  },
  {
    title: '监控管理',
    items: [
      { id: 'services' as Section, label: '服务管理', icon: 'M5 3h14a2 2 0 012 2v14a2 2 0 01-2 2H5a2 2 0 01-2-2V5a2 2 0 012-2z M8 3v18' },
      { id: 'probes' as Section, label: '探测任务', icon: 'M12 2a10 10 0 0110 10 10 10 0 01-10 10A10 10 0 012 12 10 10 0 0112 2z M12 6a6 6 0 016 6 6 6 0 01-6 6 6 6 0 01-6-6 6 6 0 016-6z' },
      { id: 'logs' as Section, label: '监控日志', icon: 'M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z M14 2v6h6 M16 13H8 M16 17H8 M10 9H8' },
    ],
  },
  {
    title: '事件与维护',
    items: [
      { id: 'incidents' as Section, label: '事件管理', icon: 'M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9' },
      { id: 'maintenances' as Section, label: '维护计划', icon: 'M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z' },
    ],
  },
  {
    title: '系统管理',
    items: [
      { id: 'users' as Section, label: '用户管理', icon: 'M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z' },
      { id: 'notifications' as Section, label: '通知管理', icon: 'M18 8A6 6 0 006 8c0 7-3 9-3 9h18s-3-2-3-9 M13.73 21a2 2 0 01-3.46 0' },
      { id: 'settings' as Section, label: '系统设置', icon: 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z' },
      { id: 'api-keys' as Section, label: 'API密钥', icon: 'M21 2l-2 2m-7.61 7.61a5.5 5.5 0 11-7.778 7.778 5.5 5.5 0 017.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4' },
    ],
  },
]

const pageTitle = computed(() => {
  for (const group of navGroups) {
    const item = group.items.find(i => i.id === activeSection.value)
    if (item) return item.label
  }
  return ''
})

function switchSection(s: string) {
  activeSection.value = s as Section
  if (isMobile.value) mobileSidebarOpen.value = false
}
</script>

<template>
  <div class="flex h-screen overflow-hidden bg-[#f7f8fa] dark:bg-gray-950">
    <!-- Mobile backdrop -->
    <div
      v-if="isMobile && mobileSidebarOpen"
      class="fixed inset-0 bg-black/30 z-30"
      @click="mobileSidebarOpen = false"
    />

    <Sidebar
      :collapsed="sidebarCollapsed"
      :active-section="activeSection"
      :nav-groups="navGroups"
      :mobile-open="isMobile ? mobileSidebarOpen : undefined"
      @toggle="sidebarCollapsed = !sidebarCollapsed; if (isMobile) mobileSidebarOpen = true"
      @select="switchSection"
    />

    <main class="flex-1 flex flex-col h-full overflow-hidden">
      <TopBar
        :title="pageTitle"
        :collapsed="sidebarCollapsed"
        :is-mobile="isMobile"
        @toggle="isMobile ? mobileSidebarOpen = !mobileSidebarOpen : sidebarCollapsed = !sidebarCollapsed"
        @navigate="switchSection"
      />

      <div class="flex-1 overflow-y-auto p-4 md:p-6">
        <DashboardHome
          v-if="activeSection === 'dashboard'"
          @navigate="switchSection"
        />
        <ServiceManager v-else-if="activeSection === 'services'" />
        <IncidentManager v-else-if="activeSection === 'incidents'" />
        <MaintenanceManager v-else-if="activeSection === 'maintenances'" />
        <SettingsPanel v-else-if="activeSection === 'settings'" />

        <!-- Placeholder sections -->
        <div v-else-if="activeSection === 'probes'" class="flex flex-col items-center justify-center py-20 text-gray-400 dark:text-gray-500">
          <svg class="w-16 h-16 mb-4 text-gray-300 dark:text-gray-600" fill="none" stroke="currentColor" stroke-width="1" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 2a10 10 0 0110 10 10 10 0 01-10 10A10 10 0 012 12 10 10 0 0112 2z M12 6a6 6 0 016 6 6 6 0 01-6 6 6 6 0 01-6-6 6 6 0 016-6z" />
          </svg>
          <p class="text-base font-medium">探测任务</p>
          <p class="text-sm mt-1">功能开发中，敬请期待</p>
        </div>

        <LogViewer v-else-if="activeSection === 'logs'" />

        <UserManager v-else-if="activeSection === 'users'" />

        <NotificationManager v-else-if="activeSection === 'notifications'" />

        <ApiKeyManager v-else-if="activeSection === 'api-keys'" />
      </div>
    </main>
    <Toast />
  </div>
</template>
