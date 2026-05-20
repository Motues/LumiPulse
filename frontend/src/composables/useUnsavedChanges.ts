import { ref, watch, type Ref } from 'vue'

export function useUnsavedChanges(formRef: Ref<Record<string, any>>, storageKey: string) {
  const isDirty = ref(false)
  const initialSnapshot = ref('')

  function takeSnapshot(): string {
    return JSON.stringify(formRef.value)
  }

  function markClean() {
    initialSnapshot.value = takeSnapshot()
    isDirty.value = false
    localStorage.removeItem(storageKey)
  }

  function handleClose(): boolean {
    if (isDirty.value) {
      // Save to localStorage before closing
      localStorage.setItem(storageKey, JSON.stringify(formRef.value))
      return window.confirm('有未保存的更改，确定要关闭吗？')
    }
    return true
  }

  function restoreFromStorage() {
    const saved = localStorage.getItem(storageKey)
    if (saved) {
      try {
        const parsed = JSON.parse(saved)
        Object.assign(formRef.value, parsed)
      } catch {
        // ignore corrupt data
      }
    }
  }

  function clearSaved() {
    localStorage.removeItem(storageKey)
  }

  watch(
    formRef,
    () => {
      if (!initialSnapshot.value) return
      isDirty.value = takeSnapshot() !== initialSnapshot.value
    },
    { deep: true }
  )

  return { isDirty, markClean, handleClose, restoreFromStorage, clearSaved }
}
