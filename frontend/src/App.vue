<script setup lang="ts">
import { onMounted } from 'vue'
import { loadSiteConfig } from './composables/useSiteConfig'
import { useDarkMode } from './composables/useDarkMode'

const { init } = useDarkMode()

onMounted(() => {
  init()
  loadSiteConfig()
})
</script>

<template>
  <router-view />
</template>
