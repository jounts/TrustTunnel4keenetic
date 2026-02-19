import { ref } from 'vue'

const BASE = '/api'

function authHeaders(): HeadersInit {
  const creds = localStorage.getItem('tt_auth')
  if (creds) {
    return { Authorization: `Basic ${creds}` }
  }
  return {}
}

export function setAuth(username: string, password: string) {
  localStorage.setItem('tt_auth', btoa(`${username}:${password}`))
}

export function clearAuth() {
  localStorage.removeItem('tt_auth')
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const resp = await fetch(`${BASE}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...authHeaders(),
      ...(options.headers || {}),
    },
  })

  if (resp.status === 401) {
    throw new Error('Unauthorized')
  }

  if (!resp.ok) {
    const body = await resp.json().catch(() => ({ error: resp.statusText }))
    throw new Error(body.error || resp.statusText)
  }

  return resp.json()
}

export interface ServiceStatus {
  running: boolean
  pid: number
  uptime_seconds: number
  mode: string
  watchdog_alive: boolean
  health_check: string
  client_version: string
}

export interface ModeInfo {
  mode: string
  tun_idx: number
  proxy_idx: number
  hc_enabled: string
  hc_interval: number
  hc_fail_threshold: number
  hc_grace_period: number
  hc_target_url: string
  hc_curl_timeout: number
  hc_socks5_proxy: string
}

export interface AllConfig {
  client_config: string
  mode_config: string
  mode: ModeInfo
}

export interface UpdateInfo {
  client_current_version: string
  client_latest_version: string
  client_update_available: boolean
  manager_current_version: string
  manager_latest_version: string
  manager_update_available: boolean
}

export interface SystemInfo {
  model: string
  firmware: string
  architecture: string
  hostname: string
  uptime: string
}

export function useApi() {
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function call<T>(fn: () => Promise<T>): Promise<T | null> {
    loading.value = true
    error.value = null
    try {
      return await fn()
    } catch (e: any) {
      error.value = e.message
      return null
    } finally {
      loading.value = false
    }
  }

  return {
    loading,
    error,
    getStatus: () => call(() => request<ServiceStatus>('/status')),
    serviceAction: (action: string) => call(() => request<any>(`/service/${action}`, { method: 'POST' })),
    getConfig: () => call(() => request<AllConfig>('/config')),
    putConfig: (data: { client_config: string; mode_config: string }) =>
      call(() => request<any>('/config', { method: 'PUT', body: JSON.stringify(data) })),
    getMode: () => call(() => request<ModeInfo>('/mode')),
    putMode: (data: { mode: string; tun_idx: number; proxy_idx: number }) =>
      call(() => request<any>('/mode', { method: 'PUT', body: JSON.stringify(data) })),
    getLogs: (lines = 100) => call(() => request<{ lines: string[]; count: number }>(`/logs?lines=${lines}`)),
    checkUpdate: () => call(() => request<UpdateInfo>('/update/check')),
    installUpdate: () => call(() => request<any>('/update/install', { method: 'POST' })),
    getSystem: () => call(() => request<SystemInfo>('/system')),
  }
}
