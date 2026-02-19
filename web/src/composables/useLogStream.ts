import { ref, onUnmounted } from 'vue'

export function useLogStream() {
  const lines = ref<string[]>([])
  const connected = ref(false)
  const paused = ref(false)
  let eventSource: EventSource | null = null

  function connect() {
    if (eventSource) return

    const creds = localStorage.getItem('tt_auth')
    const url = creds ? `/api/logs/stream?auth=${encodeURIComponent(creds)}` : '/api/logs/stream'

    eventSource = new EventSource(url)
    connected.value = true

    eventSource.onmessage = (event) => {
      if (paused.value) return
      lines.value.push(event.data)
      if (lines.value.length > 5000) {
        lines.value = lines.value.slice(-3000)
      }
    }

    eventSource.onerror = () => {
      connected.value = false
      disconnect()
      setTimeout(connect, 3000)
    }
  }

  function disconnect() {
    if (eventSource) {
      eventSource.close()
      eventSource = null
      connected.value = false
    }
  }

  function clear() {
    lines.value = []
  }

  function togglePause() {
    paused.value = !paused.value
  }

  onUnmounted(disconnect)

  return { lines, connected, paused, connect, disconnect, clear, togglePause }
}
