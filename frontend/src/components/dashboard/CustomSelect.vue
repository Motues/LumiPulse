<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'

const props = defineProps<{
  modelValue: string
  options: { label: string; value: string }[]
  placeholder?: string
  minWidth?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const open = ref(false)
const container = ref<HTMLElement | null>(null)

const displayText = computed(() => {
  const found = props.options.find(o => o.value === props.modelValue)
  return found ? found.label : (props.placeholder || '请选择')
})

const triggerStyle = computed(() => {
  const s: Record<string, string> = {}
  if (props.minWidth) s.minWidth = props.minWidth
  return s
})

function toggle() {
  open.value = !open.value
}

function select(value: string) {
  emit('update:modelValue', value)
  open.value = false
}

function onClickOutside(e: MouseEvent) {
  if (container.value && !container.value.contains(e.target as Node)) {
    open.value = false
  }
}

onMounted(() => document.addEventListener('click', onClickOutside))
onUnmounted(() => document.removeEventListener('click', onClickOutside))
</script>

<template>
  <div ref="container" class="custom-select">
    <button
      type="button"
      @click="toggle"
      class="select-trigger"
      :style="triggerStyle"
    >
      <span>{{ displayText }}</span>
      <svg class="select-arrow" :class="{ 'rotate': open }" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
        <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
      </svg>
    </button>
    <Transition name="dropdown">
      <div v-if="open" class="select-dropdown">
        <button
          v-for="opt in options"
          :key="opt.value"
          type="button"
          @click="select(opt.value)"
          class="select-option"
          :class="{ active: opt.value === modelValue }"
        >
          {{ opt.label }}
        </button>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.custom-select {
  position: relative;
}
.select-trigger {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border-radius: 8px;
  font-size: 14px;
  cursor: pointer;
  transition: border-color 0.15s;
  border: 1px solid var(--button-border-color);
  background-color: var(--bg-color);
  color: var(--text-color);
}
.select-trigger:hover {
  border-color: var(--text-color);
  opacity: 0.8;
}
.select-arrow {
  width: 14px;
  height: 14px;
  flex-shrink: 0;
  opacity: 0.35;
  transition: transform 0.2s;
}
.select-arrow.rotate {
  transform: rotate(180deg);
}
.select-dropdown {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  right: 0;
  z-index: 50;
  border-radius: 8px;
  padding: 4px;
  max-height: 200px;
  overflow-y: auto;
  scrollbar-width: none;
  background-color: var(--bg-color);
  border: 1px solid var(--button-border-color);
}
.select-dropdown::-webkit-scrollbar {
  display: none;
}
.select-option {
  width: 100%;
  text-align: left;
  padding: 6px 10px;
  border-radius: 6px;
  font-size: 14px;
  cursor: pointer;
  transition: background-color 0.1s;
  border: none;
  background: none;
  color: var(--text-color);
}
.select-option:hover {
  background-color: var(--button-hover-color);
}
.select-option.active {
  color: #10b981;
  font-weight: 500;
}

.dropdown-enter-active,
.dropdown-leave-active {
  transition: opacity 0.15s, transform 0.15s;
}
.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
