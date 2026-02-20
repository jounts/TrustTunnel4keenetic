<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useApi } from '@/composables/useApi'
import { useLogStream } from '@/composables/useLogStream'
import LogViewer from '@/components/LogViewer.vue'

const api = useApi()

const clientStream = useLogStream('/api/logs/stream?source=client')
const managerStream = useLogStream('/api/logs/stream?source=manager')

const liveMode = ref(false)

onMounted(async () => {
  const [clientResult, managerResult] = await Promise.all([
    api.getLogs(500, 'client'),
    api.getLogs(500, 'manager'),
  ])
  if (clientResult?.lines) clientStream.lines.value = clientResult.lines
  if (managerResult?.lines) managerStream.lines.value = managerResult.lines
})

function toggleLive() {
  liveMode.value = !liveMode.value
  if (liveMode.value) {
    clientStream.connect()
    managerStream.connect()
  } else {
    clientStream.disconnect()
    managerStream.disconnect()
  }
}

async function refreshLogs() {
  const [clientResult, managerResult] = await Promise.all([
    api.getLogs(500, 'client'),
    api.getLogs(500, 'manager'),
  ])
  if (clientResult?.lines) clientStream.lines.value = clientResult.lines
  if (managerResult?.lines) managerStream.lines.value = managerResult.lines
}

async function clearLogs() {
  await api.clearLogs()
  clientStream.clear()
  managerStream.clear()
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <h1 class="text-2xl font-bold">Логи</h1>
      <div class="flex items-center gap-2">
        <button
          @click="clientStream.togglePause(); managerStream.togglePause()"
          v-if="liveMode"
          :class="[
            'px-3 py-1.5 rounded-lg text-xs font-medium transition-colors',
            clientStream.paused.value
              ? 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-400'
              : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
          ]"
        >
          {{ clientStream.paused.value ? 'Продолжить' : 'Пауза' }}
        </button>
        <button
          @click="toggleLive"
          :class="[
            'px-3 py-1.5 rounded-lg text-xs font-medium transition-colors',
            liveMode
              ? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400'
              : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
          ]"
        >
          {{ liveMode ? 'Live ON' : 'Live OFF' }}
        </button>
        <button
          v-if="!liveMode"
          @click="refreshLogs"
          class="px-3 py-1.5 rounded-lg text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
        >
          Обновить
        </button>
        <button
          @click="clearLogs"
          class="px-3 py-1.5 rounded-lg text-xs font-medium bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400 hover:bg-red-200 dark:hover:bg-red-900/50 transition-colors"
        >
          Очистить всё
        </button>
      </div>
    </div>

    <div class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-4">
      <div class="flex items-center justify-between mb-3">
        <h2 class="text-sm font-semibold text-gray-700 dark:text-gray-300">Клиент (trusttunnel)</h2>
        <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
          <span>Строк: {{ clientStream.lines.value.length }}</span>
          <span v-if="liveMode && clientStream.connected.value" class="text-green-500">Live</span>
        </div>
      </div>
      <LogViewer :lines="clientStream.lines.value" />
    </div>

    <div class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-4">
      <div class="flex items-center justify-between mb-3">
        <h2 class="text-sm font-semibold text-gray-700 dark:text-gray-300">Менеджер (trusttunnel-manager)</h2>
        <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
          <span>Строк: {{ managerStream.lines.value.length }}</span>
          <span v-if="liveMode && managerStream.connected.value" class="text-green-500">Live</span>
        </div>
      </div>
      <LogViewer :lines="managerStream.lines.value" />
    </div>

    <p v-if="api.error.value" class="text-red-600 dark:text-red-400 text-sm">{{ api.error.value }}</p>
  </div>
</template>
