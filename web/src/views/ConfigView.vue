<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useApi, type AllConfig, type ModeInfo } from '@/composables/useApi'
import ModeSwitch from '@/components/ModeSwitch.vue'

const api = useApi()
const config = ref<AllConfig | null>(null)
const mode = ref<ModeInfo | null>(null)
const clientConfigText = ref('')
const saved = ref(false)
const modeChanging = ref(false)
const showModeWarning = ref(false)
const pendingMode = ref('')
const configPlaceholder = `# Пример конфигурации TrustTunnel Client
# Заполните и нажмите «Сохранить»

vpn_mode = 1  # 1 = SOCKS5, 2 = TUN

[server]
address = "your-server.example.com"
port = 443

[auth]
login = "your_login"
password = "your_password"`

onMounted(async () => {
  config.value = await api.getConfig()
  if (config.value) {
    clientConfigText.value = config.value.client_config
    mode.value = config.value.mode
  }
})

async function saveConfig() {
  const result = await api.putConfig({
    client_config: clientConfigText.value,
    mode_config: '',
  })
  if (result) {
    saved.value = true
    setTimeout(() => { saved.value = false }, 3000)
  }
}

function onModeChange(newMode: string) {
  pendingMode.value = newMode
  showModeWarning.value = true
}

async function confirmModeChange() {
  showModeWarning.value = false
  modeChanging.value = true
  await api.putMode({
    mode: pendingMode.value,
    tun_idx: mode.value?.tun_idx ?? 0,
    proxy_idx: mode.value?.proxy_idx ?? 0,
  })
  if (mode.value) mode.value.mode = pendingMode.value
  modeChanging.value = false
}
</script>

<template>
  <div class="space-y-6">
    <h1 class="text-2xl font-bold">Настройки</h1>

    <!-- Mode switch -->
    <div class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-6">
      <h2 class="text-lg font-semibold mb-2">Режим работы</h2>
      <p class="text-sm text-gray-500 dark:text-gray-400 mb-4">
        SOCKS5 — проксирование через локальный порт 1080. TUN — полный перехват трафика через виртуальный интерфейс.
      </p>
      <ModeSwitch
        :model-value="mode?.mode ?? 'socks5'"
        @change="onModeChange"
      />
      <div v-if="modeChanging" class="mt-3 text-sm text-brand-600 dark:text-brand-400">Переключение режима...</div>
    </div>

    <!-- Config editor -->
    <div class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-6">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-lg font-semibold">Конфигурация клиента (TOML)</h2>
        <button
          @click="saveConfig"
          :disabled="api.loading.value"
          class="inline-flex items-center px-4 py-2 rounded-lg text-sm font-medium text-white bg-brand-600 hover:bg-brand-700 disabled:opacity-50 transition-colors"
        >
          Сохранить
        </button>
      </div>
      <textarea
        v-model="clientConfigText"
        rows="16"
        class="w-full font-mono text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg p-3 focus:ring-2 focus:ring-brand-500 focus:border-brand-500 resize-y"
        spellcheck="false"
        :placeholder="configPlaceholder"
      />
      <p v-if="saved" class="mt-2 text-sm text-green-600 dark:text-green-400">Конфигурация сохранена</p>
    </div>

    <!-- Health Check settings -->
    <div v-if="mode" class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-6">
      <h2 class="text-lg font-semibold mb-4">Health Check</h2>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4 text-sm">
        <div>
          <label class="block text-xs text-gray-500 dark:text-gray-400 mb-1">Включён</label>
          <p class="font-medium">{{ mode.hc_enabled }}</p>
        </div>
        <div>
          <label class="block text-xs text-gray-500 dark:text-gray-400 mb-1">Интервал (сек)</label>
          <p class="font-medium">{{ mode.hc_interval }}</p>
        </div>
        <div>
          <label class="block text-xs text-gray-500 dark:text-gray-400 mb-1">Порог отказов</label>
          <p class="font-medium">{{ mode.hc_fail_threshold }}</p>
        </div>
        <div>
          <label class="block text-xs text-gray-500 dark:text-gray-400 mb-1">URL проверки</label>
          <p class="font-medium text-xs truncate">{{ mode.hc_target_url }}</p>
        </div>
      </div>
    </div>

    <!-- Mode change warning modal -->
    <div v-if="showModeWarning" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div class="bg-white dark:bg-gray-800 rounded-xl shadow-xl max-w-md w-full mx-4 p-6">
        <h3 class="text-lg font-semibold mb-2">Смена режима</h3>
        <p class="text-sm text-gray-600 dark:text-gray-400 mb-4">
          Переключение на <strong>{{ pendingMode.toUpperCase() }}</strong> приведёт к пересозданию сетевого интерфейса.
          Текущее соединение будет прервано.
        </p>
        <div class="flex justify-end gap-3">
          <button @click="showModeWarning = false" class="px-4 py-2 rounded-lg text-sm border border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700">
            Отмена
          </button>
          <button @click="confirmModeChange" class="px-4 py-2 rounded-lg text-sm font-medium text-white bg-brand-600 hover:bg-brand-700">
            Подтвердить
          </button>
        </div>
      </div>
    </div>

    <p v-if="api.error.value" class="text-red-600 dark:text-red-400 text-sm">{{ api.error.value }}</p>
  </div>
</template>
