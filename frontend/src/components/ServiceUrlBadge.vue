<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from '../composables/useI18n'

/**
 * 服务名后面的「网址」图标：悬浮显示完整 URL，点击在新标签页打开。
 *
 * 提示框用 fixed 定位（位置按图标的 getBoundingClientRect 计算），
 * 而不是 absolute：服务分组展开区域有 overflow 裁剪（收起动画），
 * absolute 提示框会被裁掉，fixed 不受影响。
 */
const props = defineProps<{ url: string }>()

const { t } = useI18n()

const anchor = ref<{ left: number; top: number } | null>(null)

function onEnter(e: MouseEvent) {
  if (!props.url) return
  const el = e.currentTarget as HTMLElement | null
  if (!el) return
  const rect = el.getBoundingClientRect()
  anchor.value = { left: rect.left + rect.width / 2, top: rect.top - 8 }
}

function onLeave() {
  anchor.value = null
}

function openUrl() {
  if (!props.url) return
  window.open(props.url, '_blank')
}
</script>

<template>
  <span
    v-if="url"
    class="inline-flex items-center ml-1 flex-shrink-0"
    @mouseenter="onEnter"
    @mouseleave="onLeave"
  >
    <svg
      class="w-4 h-4 cursor-pointer transition-colors text-[color:var(--text-color)] opacity-40 hover:text-emerald-500 dark:hover:text-emerald-400"
      fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5"
      :aria-label="t('home.openServiceUrl')"
      @click.stop="openUrl"
    >
      <path stroke-linecap="round" stroke-linejoin="round" d="M11.25 11.25l.041-.02a.75.75 0 011.063.852l-.708 2.836a.75.75 0 001.063.853l.041-.021M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-9-3.75h.008v.008H12V8.25z" />
    </svg>
    <div
      v-if="anchor"
      class="fixed z-50 px-3 py-1.5 text-xs rounded-lg whitespace-nowrap shadow-lg pointer-events-none"
      :style="{
        left: `${anchor.left}px`,
        top: `${anchor.top}px`,
        transform: 'translate(-50%, -100%)',
        backgroundColor: 'var(--button-hover-color)',
        color: 'var(--text-color)',
        border: '1px solid var(--button-border-color)',
      }"
    >
      {{ url }}
      <div
        class="absolute left-1/2 -translate-x-1/2 top-full w-0 h-0 border-l-4 border-r-4 border-t-4 border-transparent"
        :style="{ borderTopColor: 'var(--button-hover-color)' }"
      />
    </div>
  </span>
</template>
