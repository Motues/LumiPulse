<script setup lang="ts">
import { customFooter, showAdminButton } from '../composables/useSiteConfig'
import { useI18n, type Locale } from '../composables/useI18n'

/**
 * 公开页页脚（首页与事件详情页共用）。
 * 除管理后台入口外，这里也放语言切换：项目本身没有设置页，
 * 把切换入口放在页脚既不打断阅读，也保证任何公开页都能改语言。
 */
const { locale, t, setLocale } = useI18n()

const LANGUAGES: { value: Locale; label: string }[] = [
  { value: 'zh-CN', label: '中文' },
  { value: 'en-US', label: 'English' },
]
</script>

<template>
  <footer style="background-color: var(--bg-color);" class="mt-4">
    <div class="max-w-[1000px] mx-auto px-6 py-8 flex flex-col md:flex-row justify-between items-center gap-4">
      <div class="text-sm">
        <a href="https://github.com/Motues/LumiPulse" target="_blank" rel="noopener noreferrer" class="footer-link transition-opacity">{{ t('home.poweredBy') }}</a>
      </div>
      <div class="flex items-center gap-6 text-sm">
        <span v-if="customFooter" v-html="customFooter" class="footer-link"></span>
        <a v-else-if="showAdminButton" href="/dashboard" class="footer-link transition-opacity">{{ t('common.adminPanel') }}</a>
        <div class="flex items-center gap-2">
          <button
            v-for="lang in LANGUAGES"
            :key="lang.value"
            @click="setLocale(lang.value)"
            class="transition-opacity"
            :class="locale === lang.value ? 'footer-link-active' : 'footer-link'"
          >
            {{ lang.label }}
          </button>
        </div>
      </div>
    </div>
  </footer>
</template>

<style scoped>
.footer-link {
  color: var(--text-color);
  opacity: 0.5;
}
.footer-link:hover {
  opacity: 1;
}
.footer-link-active {
  color: var(--text-color);
  opacity: 0.9;
  font-weight: 500;
}
</style>
