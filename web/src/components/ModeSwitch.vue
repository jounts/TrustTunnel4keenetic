<script setup lang="ts">
import { ref, watch } from 'vue'

const props = defineProps<{
  modelValue: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'change', value: string): void
}>()

const selected = ref(props.modelValue)

watch(() => props.modelValue, (v) => { selected.value = v })

function setMode(mode: string) {
  selected.value = mode
  emit('update:modelValue', mode)
  emit('change', mode)
}
</script>

<template>
  <div class="flex rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
    <button
      @click="setMode('socks5')"
      :class="[
        'flex-1 px-4 py-2.5 text-sm font-medium transition-colors',
        selected === 'socks5'
          ? 'bg-brand-600 text-white'
          : 'bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700'
      ]"
    >
      SOCKS5
    </button>
    <button
      @click="setMode('tun')"
      :class="[
        'flex-1 px-4 py-2.5 text-sm font-medium transition-colors border-l border-gray-200 dark:border-gray-700',
        selected === 'tun'
          ? 'bg-brand-600 text-white'
          : 'bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700'
      ]"
    >
      TUN
    </button>
  </div>
</template>
