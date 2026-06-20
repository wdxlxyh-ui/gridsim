import { ref, onUnmounted, type Ref } from 'vue'
import { ElNotification } from 'element-plus'

export interface SSEEvent {
  type: string
  instance: string
  ioa?: number
  value?: unknown
  prev_value?: unknown
  alarm_type?: string
  severity?: string
  progress?: { current: number; total: number; failed?: number; label?: string }
  ts: number
}

export function useSSE(maxEvents = 200) {
  const events = ref<SSEEvent[]>([])
  const connected = ref(false)
  let eventSource: EventSource | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null

  function connect() {
    if (eventSource) return

    try {
      eventSource = new EventSource('/api/v1/events')

      eventSource.onopen = () => {
        connected.value = true
      }

      eventSource.      onmessage = (msg) => {
        try {
          const data = JSON.parse(msg.data) as SSEEvent
          events.value.unshift(data)
          if (events.value.length > maxEvents) {
            events.value = events.value.slice(0, maxEvents)
          }
          if (data.type === 'point.alarm' || data.alarm_type) {
            const name = data.instance ? `[${data.instance}]` : ''
            ElNotification({
              title: `⚠️ 告警 ${name}`,
              message: `${data.alarm_type || '告警'} — IOA:${data.ioa}`,
              type: 'warning',
              duration: 4000,
            })
          }
        } catch { /* ignore parse errors */ }
      }

      eventSource.onerror = () => {
        connected.value = false
        eventSource?.close()
        eventSource = null
        // Auto-reconnect after 5s
        if (!reconnectTimer) {
          reconnectTimer = setTimeout(() => {
            reconnectTimer = null
            connect()
          }, 5000)
        }
      }
    } catch {
      connected.value = false
    }
  }

  function disconnect() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    eventSource?.close()
    eventSource = null
    connected.value = false
  }

  function clearEvents() {
    events.value = []
  }

  onUnmounted(() => {
    disconnect()
  })

  return { events, connected, connect, disconnect, clearEvents }
}
