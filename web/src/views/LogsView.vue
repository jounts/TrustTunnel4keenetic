<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useApi } from '@/composables/useApi'
import { useLogStream } from '@/composables/useLogStream'
import LogViewer from '@/components/LogViewer.vue'

const api = useApi()
const stream = useLogStream()
const liveMode = ref(false)

onMounted(async () => {
  const result = await api.getLogs(500)
  if (result?.lines) {
    stream.lines.value = result.lines
  }
})

function toggleLive() {
  liveMode.value = !liveMode.value
  if (liveMode.value) {
    stream.connect()
  } else {
    stream.disconnect()
  }
}

async function refreshLogs() {
  const result = await api.getLogs(500)
  if (result?.lines) {
    stream.lines.value = result.lines
  }
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <h1 class="text-2xl font-bold">Логи</h1>
      <div class="flex items-center gap-2">
        <button
          @click="stream.togglePause()"
          v-if="liveMode"
          :class="[
            'px-3 py-1.5 rounded-lg text-xs font-medium transition-colors',
            stream.paused.value
              ? 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-400'
              : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
          ]"
        >
          {{ stream.paused.value ? 'Продолжить' : 'Пауза' }}
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
          @click="stream.clear()"
          class="px-3 py-1.5 rounded-lg text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
        >
          Очистить
        </button>
      </div>
    </div>

    <div class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-4">
      <div class="flex items-center gap-2 mb-3 text-xs text-gray-500 dark:text-gray-400">
        <span>Строк: {{ stream.lines.value.length }}</span>
        <span v-if="liveMode && stream.connected.value" class="text-green-500">Подключено</span>
      </div>
      <LogViewer :lines="stream.lines.value" />
    </div>

    <p v-if="api.error.value" class="text-red-600 dark:text-red-400 text-sm">{{ api.error.value }}</p>
  </div>
</template>
