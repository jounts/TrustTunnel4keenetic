<script setup lang="ts">
import type { ServiceStatus } from '@/composables/useApi'

defineProps<{
  status: ServiceStatus | null
}>()

function formatUptime(seconds: number): string {
  if (!seconds || seconds <= 0) return '—'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const parts: string[] = []
  if (d > 0) parts.push(`${d}д`)
  if (h > 0) parts.push(`${h}ч`)
  parts.push(`${m}м`)
  return parts.join(' ')
}
</script>

<template>
  <div v-if="status" class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-6">
    <div class="flex items-center justify-between mb-4">
      <h2 class="text-lg font-semibold">Статус</h2>
      <span
        :class="[
          'inline-flex items-center px-3 py-1 rounded-full text-xs font-medium',
          status.running
            ? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400'
            : 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400'
        ]"
      >
        <span class="w-2 h-2 rounded-full mr-1.5" :class="status.running ? 'bg-green-500' : 'bg-red-500'" />
        {{ status.running ? 'Работает' : 'Остановлен' }}
      </span>
    </div>

    <div class="grid grid-cols-2 sm:grid-cols-4 gap-4">
      <div>
        <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">Режим</p>
        <p class="text-sm font-medium uppercase">{{ status.mode || '—' }}</p>
      </div>
      <div>
        <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">PID</p>
        <p class="text-sm font-medium font-mono">{{ status.pid || '—' }}</p>
      </div>
      <div>
        <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">Uptime</p>
        <p class="text-sm font-medium">{{ formatUptime(status.uptime_seconds) }}</p>
      </div>
      <div>
        <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">Health Check</p>
        <span
          :class="[
            'inline-flex items-center px-2 py-0.5 rounded text-xs font-medium',
            status.health_check === 'ok'
              ? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400'
              : status.health_check === 'fail'
                ? 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400'
                : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400'
          ]"
        >
          {{ status.health_check }}
        </span>
      </div>
    </div>

    <div class="mt-4 flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
      <span>Версия клиента: <strong class="text-gray-700 dark:text-gray-300">{{ status.client_version }}</strong></span>
      <span v-if="status.watchdog_alive" class="ml-2 text-green-600 dark:text-green-400">Watchdog OK</span>
    </div>
  </div>

  <div v-else class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-6 animate-pulse">
    <div class="h-4 bg-gray-200 dark:bg-gray-700 rounded w-1/4 mb-4"></div>
    <div class="grid grid-cols-4 gap-4">
      <div class="h-8 bg-gray-200 dark:bg-gray-700 rounded"></div>
      <div class="h-8 bg-gray-200 dark:bg-gray-700 rounded"></div>
      <div class="h-8 bg-gray-200 dark:bg-gray-700 rounded"></div>
      <div class="h-8 bg-gray-200 dark:bg-gray-700 rounded"></div>
    </div>
  </div>
</template>
