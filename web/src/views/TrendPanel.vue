<template>
  <el-card shadow="never" class="panel-card">
    <template #header>
      <div class="panel-header">
        <div class="panel-traces">
          <el-tag
            v-for="(t, i) in panelTraces"
            :key="i"
            :color="COLORS[t.colorIdx % COLORS.length]"
            closable
            :disable-transitions="true"
            size="small"
            style="color: #fff; border: none; margin-right: 6px; margin-bottom: 4px"
            @close="removeTrace(i)"
          >
            {{ t.inst }} · {{ t.alias || t.name || 'IOA:' + t.ioa }}
          </el-tag>
          <el-button size="small" @click="$emit('addTrace', panelId)">+ 添加</el-button>
        </div>
        <div class="panel-controls">
          <el-select v-model="localInterval" size="small" style="width: 90px" @change="restartTimer">
            <el-option label="200ms" :value="200" />
            <el-option label="500ms" :value="500" />
            <el-option label="1s" :value="1000" />
            <el-option label="2s" :value="2000" />
            <el-option label="5s" :value="5000" />
          </el-select>
          <el-button size="small" :type="paused ? 'warning' : 'info'" @click="togglePause">
            {{ paused ? '▶' : '⏸' }}
          </el-button>
          <el-button size="small" @click="clearAllData">🔄</el-button>
          <el-button size="small" type="primary" @click="downloadCSV">📥</el-button>
          <el-button size="small" type="danger" text @click="$emit('remove', panelId)">✕</el-button>
        </div>
      </div>
    </template>

    <div v-if="panelTraces.length === 0" class="panel-empty">
      <span style="font-size: 32px; margin-bottom: 8px">📊</span>
      <span style="color: #64748b">点击「+ 添加」选择测点</span>
    </div>
    <div v-else ref="chartRef" class="panel-chart"></div>
  </el-card>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, nextTick } from 'vue'
import * as echarts from 'echarts'
import { readPointsBatch } from '../api'

const COLORS = ['#14b8a6', '#f59e0b', '#3b82f6', '#a855f7', '#ec4899', '#22d3ee', '#f97316', '#8b5cf6']

interface TraceConfig {
  instId: string; inst: string; ioa: number; name: string; unit: string
  alias: string; colorIdx: number
}

interface Trace {
  instId: string; inst: string; ioa: number; name: string; unit: string
  alias: string; colorIdx: number
  data: [number, number][]
}

const props = defineProps<{
  panelId: string
  traces: TraceConfig[]
  timeRange: number
  pollInterval: number
}>()

const emit = defineEmits<{
  remove: [panelId: string]
  addTrace: [panelId: string]
}>()

const panelTraces = ref<Trace[]>([])
const localInterval = ref(props.pollInterval)
const paused = ref(false)
const lastUpdate = ref('--')

const chartRef = ref<HTMLElement | null>(null)
let chartInstance: echarts.ECharts | null = null
let resizeObserver: ResizeObserver | null = null
let pollTimer: ReturnType<typeof setInterval> | null = null
let disposed = false

// Reconcile traces when props.traces changes (template switch / add trace from parent)
watch(() => props.traces, (newConfigs) => {
  const wasEmpty = panelTraces.value.length === 0
  const newTraces: Trace[] = []
  for (const cfg of newConfigs) {
    const existing = panelTraces.value.find(t => t.instId === cfg.instId && t.ioa === cfg.ioa)
    if (existing) {
      newTraces.push(existing)
    } else {
      newTraces.push({ ...cfg, data: [] })
    }
  }
  panelTraces.value = newTraces
  if (wasEmpty && newTraces.length > 0) {
    // Panel went from empty to having traces — init chart + start polling
    nextTick(() => {
      initChart()
      startPolling()
    })
  } else if (newTraces.length === 0) {
    if (chartInstance) { chartInstance.dispose(); chartInstance = null }
    if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
  } else {
    nextTick(updateChart)
  }
}, { deep: true })

watch(() => props.timeRange, () => {
  trimData()
  updateChart()
})

function initChart() {
  if (!chartRef.value) return
  if (chartInstance) chartInstance.dispose()
  chartInstance = echarts.init(chartRef.value, undefined, { renderer: 'canvas' })
  // B2: ResizeObserver for responsive chart
  if (resizeObserver) resizeObserver.disconnect()
  resizeObserver = new ResizeObserver(() => { chartInstance?.resize() })
  resizeObserver.observe(chartRef.value)
  updateChart()
}

function updateChart() {
  if (!chartInstance) return
  const series = panelTraces.value.map(t => ({
    name: `${t.inst} · ${t.alias || t.name}`,
    type: 'line' as const,
    data: t.data,
    smooth: false,
    symbol: 'none',
    lineStyle: { color: COLORS[t.colorIdx % COLORS.length], width: 1.5 },
    areaStyle: {
      color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
        { offset: 0, color: COLORS[t.colorIdx % COLORS.length] + '40' },
        { offset: 1, color: COLORS[t.colorIdx % COLORS.length] + '05' },
      ]),
    },
  }))

  chartInstance.setOption({
    animation: false,
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#1a1f2e',
      borderColor: '#334155',
      textStyle: { color: '#e2e8f0', fontSize: 11, fontFamily: 'monospace' },
    },
    legend: {
      bottom: 0,
      textStyle: { color: '#94a3b8', fontSize: 10 },
    },
    grid: { left: 45, right: 15, top: 10, bottom: 50 },
    xAxis: {
      type: 'time',
      axisLine: { lineStyle: { color: '#334155' } },
      axisLabel: { color: '#64748b', fontSize: 9 },
      splitLine: { lineStyle: { color: '#1e293b' } },
    },
    yAxis: {
      type: 'value',
      axisLine: { show: false },
      axisLabel: { color: '#64748b', fontSize: 9, formatter: (v: number) => v.toFixed(1) },
      splitLine: { lineStyle: { color: '#1e293b' } },
    },
    dataZoom: [
      { type: 'inside', orient: 'horizontal' },
      { type: 'slider', bottom: 22, height: 12, borderColor: '#334155', backgroundColor: '#1e293b',
        fillerColor: '#33415555', textStyle: { color: '#64748b', fontSize: 9 } },
    ],
    series: series.length ? series : [{ type: 'line', data: [] }],
  }, true)
}

function trimData() {
  const cutoff = Date.now() - props.timeRange * 60 * 1000
  panelTraces.value.forEach(t => {
    if (t.data.length === 0 || t.data[0][0] >= cutoff) return
    const idx = t.data.findIndex(d => d[0] >= cutoff)
    t.data = idx === -1 ? [] : t.data.slice(idx)
  })
}

async function fetchAllPoints() {
  if (panelTraces.value.length === 0 || paused.value || disposed) return
  const byInstance = new Map<string, number[]>()
  panelTraces.value.forEach(t => {
    if (!byInstance.has(t.instId)) byInstance.set(t.instId, [])
    byInstance.get(t.instId)!.push(t.ioa)
  })
  for (const [instId, ioas] of byInstance) {
    try {
      const res = await readPointsBatch(instId, ioas)
      for (const pt of res.points) {
        const trace = panelTraces.value.find(t => t.instId === instId && t.ioa === pt.ioa)
        if (!trace) continue
        const ts = pt.updated_at ? new Date(pt.updated_at).getTime() : Date.now()
        let v = pt.value
        if (pt.point_type === 'DI' || pt.point_type === 'DO') v = pt.bool_value ? 1 : 0
        else if (pt.point_type === 'PI') v = pt.int_value
        // Deduplicate: skip if same timestamp as last point
        const last = trace.data[trace.data.length - 1]
        if (last && last[0] === ts) {
          last[1] = v // update value if changed
        } else {
          trace.data.push([ts, v])
        }
      }
    } catch { /* instance may have stopped */ }
  }
  if (disposed) return
  lastUpdate.value = new Date().toLocaleTimeString()
  trimData()
  updateChart()
}

function restartTimer() {
  if (pollTimer) clearInterval(pollTimer)
  if (!paused.value) {
    fetchAllPoints()
    pollTimer = setInterval(fetchAllPoints, localInterval.value)
  }
}

function togglePause() {
  paused.value = !paused.value
  if (paused.value) {
    if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
    lastUpdate.value = '已暂停'
  } else {
    pollTimer = setInterval(fetchAllPoints, localInterval.value)
  }
}

function clearAllData() {
  panelTraces.value.forEach(t => { t.data = [] })
  updateChart()
}

function removeTrace(i: number) {
  panelTraces.value.splice(i, 1)
  if (panelTraces.value.length === 0) {
    if (chartInstance) { chartInstance.dispose(); chartInstance = null }
  } else {
    updateChart()
  }
}

function downloadCSV() {
  const MAX_POINTS = 50000
  const tsSet = new Set<number>()
  // O2: Build lookup maps for O(1) access instead of O(n) find per timestamp
  const traceMaps = panelTraces.value.map(t => {
    const map = new Map<number, number>()
    const slice = t.data.length > MAX_POINTS ? t.data.slice(-MAX_POINTS) : t.data
    slice.forEach(d => { tsSet.add(d[0]); map.set(d[0], d[1]) })
    return map
  })
  const timestamps = Array.from(tsSet).sort((a, b) => a - b)

  const header = ['时间']
  panelTraces.value.forEach(t => header.push(`${t.inst}·${t.alias || t.name}`))

  const rows: string[][] = []
  timestamps.forEach(ts => {
    const d = new Date(ts)
    const tStr = `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,'0')}-${String(d.getDate()).padStart(2,'0')} ${String(d.getHours()).padStart(2,'0')}:${String(d.getMinutes()).padStart(2,'0')}:${String(d.getSeconds()).padStart(2,'0')}.${String(d.getMilliseconds()).padStart(3,'0')}`
    const row = [tStr]
    traceMaps.forEach(map => {
      const v = map.get(ts)
      row.push(v !== undefined ? String(v) : '')
    })
    rows.push(row)
  })

  const csvLines = [header.join(',')]
  rows.forEach(r => csvLines.push(r.join(',')))
  const csvContent = '\uFEFF' + csvLines.join('\n')

  const now = new Date()
  const filename = `trend_${now.getFullYear()}${String(now.getMonth()+1).padStart(2,'0')}${String(now.getDate()).padStart(2,'0')}_${String(now.getHours()).padStart(2,'0')}${String(now.getMinutes()).padStart(2,'0')}${String(now.getSeconds()).padStart(2,'0')}.csv`
  const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url; a.download = filename
  document.body.appendChild(a); a.click()
  document.body.removeChild(a); URL.revokeObjectURL(url)
}

function startPolling() {
  if (pollTimer) clearInterval(pollTimer)
  pollTimer = setInterval(fetchAllPoints, localInterval.value)
}

onMounted(() => {
  // Initialize traces from props
  panelTraces.value = props.traces.map(t => ({ ...t, data: [] }))
  nextTick(() => {
    if (panelTraces.value.length > 0) {
      initChart()
      fetchAllPoints()
      startPolling()
    }
  })
})

onUnmounted(() => {
  disposed = true
  if (pollTimer) clearInterval(pollTimer)
  if (resizeObserver) resizeObserver.disconnect()
  if (chartInstance) chartInstance.dispose()
})
</script>

<style scoped>
.panel-card {
  background: #0f172a;
  border: 1px solid #1e293b;
  display: flex;
  flex-direction: column;
}
.panel-card :deep(.el-card__header) {
  padding: 10px 14px;
  border-bottom: 1px solid #1e293b;
}
.panel-card :deep(.el-card__body) {
  padding: 8px 10px 14px;
  flex: 1;
  min-height: 0;
}
.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  flex-wrap: wrap;
  gap: 8px;
}
.panel-traces {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  align-items: center;
  flex: 1;
  min-width: 0;
}
.panel-controls {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}
.panel-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 250px;
  color: #475569;
  font-size: 13px;
}
.panel-chart {
  width: 100%;
  height: 280px;
}
</style>
