<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '../composables/useI18n'

/**
 * 服务状态标签（圆点 + 文案）。
 *
 * 维护中的服务统一显示为蓝色「维护中」：维护窗口内的探测失败属于计划内停机，
 * 不应该用故障的红色表达；蓝色同时区别于正常（绿）与异常（黄）。
 */
const props = defineProps<{
  status: string
  /** 该服务是否处于进行中的维护窗口（维护中会覆盖服务自身的状态与颜色） */
  maintenance?: boolean
}>()

const { t } = useI18n()

const statusColors: Record<string, string> = {
  operational: '#34a761',
  degraded: '#fda305',
  outage: '#df2d2a',
}

function statusText(status: string): string {
  switch (status) {
    case 'operational': return t('status.operational')
    case 'degraded': return t('status.degraded')
    case 'outage': return t('status.outage')
    default: return status
  }
}

const label = computed(() => (props.maintenance ? t('status.maintenance') : statusText(props.status)))
const dotColor = computed(() =>
  props.maintenance ? 'var(--maintenance-color)' : (statusColors[props.status] || '#9ca3af')
)
</script>

<template>
  <div
    class="flex items-center gap-1.5 text-sm font-medium"
    :class="{
      'text-[#45ba65] dark:text-[#4ade80]': status === 'operational' && !maintenance,
      'text-[#f9ac05] dark:text-[#fbbf24]': status === 'degraded' && !maintenance,
      'text-[#df2d2a] dark:text-[#f87171]': status === 'outage' && !maintenance,
      'text-[color:var(--maintenance-color)]': maintenance,
    }"
  >
    <div class="w-2 h-2 rounded-full" :style="{ backgroundColor: dotColor }"></div>
    {{ label }}
  </div>
</template>
