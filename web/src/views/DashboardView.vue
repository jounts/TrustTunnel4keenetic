<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useApi, type ServiceStatus, type SystemInfo } from '@/composables/useApi'
import StatusCard from '@/components/StatusCard.vue'
import ServiceControls from '@/components/ServiceControls.vue'

const api = useApi()
const status = ref<ServiceStatus | null>(null)
const system = ref<SystemInfo | null>(null)
let interval: ReturnType<typeof setInterval> | null = null

async function refresh() {
  const s = await api.getStatus()
  if (s) status.value = s
}

async function handleAction(action: string) {
  await api.serviceAction(action)
  setTimeout(refresh, 1500)
}

onMounted(async () => {
  await refresh()
  system.value = await api.getSystem()
  interval = setInterval(refresh, 5000)
})

onUnmounted(() => {
  if (interval) clearInterval(interval)
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <h1 class="text-2xl font-bold">Dashboard</h1>
      <button
        @click="refresh"
        class="p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
        title="Обновить"
      >
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
        </svg>
      </button>
    </div>

    <StatusCard :status="status" />

    <div class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-6">
      <h2 class="text-lg font-semibold mb-4">Управление</h2>
      <ServiceControls :running="status?.running ?? false" @action="handleAction" />
    </div>

    <div v-if="system" class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-6">
      <h2 class="text-lg font-semibold mb-4">Система</h2>
      <div class="grid grid-cols-2 sm:grid-cols-4 gap-4 text-sm">
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">Модель</p>
          <p class="font-medium">{{ system.model }}</p>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">Прошивка</p>
          <p class="font-medium">{{ system.firmware }}</p>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">Архитектура</p>
          <p class="font-medium font-mono">{{ system.architecture }}</p>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">Хост</p>
          <p class="font-medium">{{ system.hostname }}</p>
        </div>
      </div>
    </div>

    <p v-if="api.error.value" class="text-red-600 dark:text-red-400 text-sm">{{ api.error.value }}</p>
  </div>
</template>
