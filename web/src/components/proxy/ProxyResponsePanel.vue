<template>
  <div class="response-panel">
    <div class="panel-header">
      <span>响应结果</span>
      <el-button text size="small" @click="$emit('toggle-history')">🕐 历史</el-button>
    </div>
    <div v-if="response" class="response-status">
      <el-tag :type="response.status < 400 ? 'success' : 'danger'">
        <span :class="{ 'glitch-text': response.status >= 400 }">● {{ response.status }} {{ response.status_text }}</span>
      </el-tag>
      <el-tag type="info">⏱ {{ response.time_ms }} ms</el-tag>
      <el-tag type="info">📦 {{ response.size }} B</el-tag>
    </div>
    <div class="response-body">
      <div v-if="response && !response.error" class="json-view" v-html="highlightJson(response.body)" />
      <div v-else-if="response?.error" style="color: #ef4444; padding: 16px;">{{ response.error }}</div>
      <div v-else class="empty-state">
        <div style="font-size: 40px; opacity: 0.3;">📡</div>
        <p style="font-size: 12px; color: #64748b;">点击「发送」发起请求</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { ProxyResponse } from '../../api'

defineProps<{
  response: ProxyResponse | null
}>()

defineEmits<{
  'toggle-history': []
}>()

function highlightJson(body: string) {
  try {
    const pretty = JSON.stringify(JSON.parse(body), null, 2)
    return pretty.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
      .replace(/("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+-]?\d+)?)/g, (m: string) => {
        if (/^"/.test(m)) return /:$/.test(m) ? `<span class="json-key">${m}</span>` : `<span class="json-string">${m}</span>`
        if (/true|false/.test(m)) return `<span class="json-bool">${m}</span>`
        if (/null/.test(m)) return `<span class="json-null">${m}</span>`
        return `<span class="json-number">${m}</span>`
      })
  } catch { return body }
}
</script>

<style scoped>
.response-panel { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.panel-header { padding: 8px 14px; font-size: 11px; font-weight: 600; color: #64748b; text-transform: uppercase; letter-spacing: 0.5px; background: #111827; border-bottom: 1px solid #1e293b; display: flex; align-items: center; justify-content: space-between; }
.response-status { display: flex; gap: 12px; padding: 8px 14px; background: #111827; border-bottom: 1px solid #1e293b; }
.response-body { flex: 1; overflow-y: auto; padding: 14px; }
.json-view { background: #0d1117; border: 1px solid #1e293b; border-radius: 8px; padding: 14px; font-family: 'JetBrains Mono', monospace; font-size: 12px; line-height: 1.7; white-space: pre-wrap; word-break: break-all; color: #e2e8f0; }
.empty-state { display: flex; flex-direction: column; align-items: center; justify-content: center; height: 100%; color: #64748b; gap: 10px; }
</style>
