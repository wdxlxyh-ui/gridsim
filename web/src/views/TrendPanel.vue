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
          <el-select v-if="props.mode === 'realtime'" v-model="localInterval" size="small" style="width: 90px" @change="restartTimer">
            <el-option label="200ms" :value="200" />
            <el-option label="500ms" :value="500" />
            <el-option label="1s" :value="1000" />
            <el-option label="2s" :value="2000" />
            <el-option label="5s" :value="5000" />
          </el-select>
          <el-tag v-if="props.mode === 'history'" size="small" type="info" effect="plain">固定查询</el-tag>
          <el-button v-if="props.mode === 'realtime'" size="small" :type="paused ? 'warning' : 'info'" @click="togglePause">
            {{ paused ? '▶' : '⏸' }}
          </el-button>
          <el-button
            v-if="props.mode === 'realtime'"
            size="small"
            :loading="resetting"
            title="清空当前曲线，并从当前时刻重新采样"
            @click="restartFromNow"
          >从当前开始</el-button>
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
import { readPointsBatch, getPersistenceStatus, getPointHistory } from '../api'

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
  mode: 'realtime' | 'history'
  historyFrom: number | null
  historyTo: number | null
  historyQueryToken: number
}>()

const emit = defineEmits<{
  remove: [panelId: string]
  addTrace: [panelId: string]
  tracesChanged: [panelId: string, traces: TraceConfig[]]
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
let fetchInFlight = false
let restartPending = false
let requestGeneration = 0
const resetting = ref(false)

// Reconcile traces when props.traces changes (template switch / add trace from parent).
// 使用深度监听确保所有属性变化都能触发更新
watch(() => props.traces, (newConfigs) => {
  const wasEmpty = panelTraces.value.length === 0
  const newTraces: Trace[] = []
  
  for (const cfg of newConfigs) {
    const existing = panelTraces.value.find(t => t.instId === cfg.instId && t.ioa === cfg.ioa)
    if (existing) {
      // 更新所有属性，保留已有的数据
      newTraces.push({
        ...existing,
        inst: cfg.inst,
        name: cfg.name,
        unit: cfg.unit,
        alias: cfg.alias,
        colorIdx: cfg.colorIdx,
      })
    } else {
      newTraces.push({ ...cfg, data: [] })
    }
  }
  
  panelTraces.value = newTraces
  
  if (wasEmpty && newTraces.length > 0) {
    // Panel went from empty to having traces — init chart + start polling
    nextTick(async () => {
      initChart()
      if (props.mode === 'realtime') {
        await backfillHistory()
        await fetchAllPoints()
        startPolling()
      }
    })
  } else if (newTraces.length === 0) {
    if (chartInstance) { chartInstance.dispose(); chartInstance = null }
    if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
  } else {
    nextTick(updateChart)
  }
}, { deep: true })

watch(() => props.timeRange, () => {
  if (props.mode === 'realtime') trimData()
  updateChart()
})

watch(() => props.mode, async (mode) => {
  requestGeneration += 1
  if (disposed || panelTraces.value.length === 0) return
  if (mode === 'realtime') {
    paused.value = false
    await backfillHistory()
    await fetchAllPoints()
    startPolling()
  } else {
    if (pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
    // Historical mode is intentionally static: remove any live-buffer values
    // and wait for the explicit time-range query.
    panelTraces.value.forEach(trace => { trace.data = [] })
    updateChart()
  }
})

watch(() => props.historyQueryToken, async (token) => {
  if (token <= 0 || props.mode !== 'history') return
  await queryHistory()
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
      axisLabel: { color: '#64748b', fontSize: 9, formatter: formatAxisTime },
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
  }, { notMerge: false, lazyUpdate: true, replaceMerge: ['series'] })
}

function formatAxisTime(value: string | number): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  const seconds = String(date.getSeconds()).padStart(2, '0')
  return `${hours}:${minutes}:${seconds}`
}

function trimData() {
  const cutoff = Date.now() - props.timeRange * 60 * 1000
  panelTraces.value.forEach(t => {
    if (t.data.length === 0 || t.data[0][0] >= cutoff) return



    const idx = t.data.findIndex(d => d[0] >= cutoff)
    t.data = idx === -1 ? [] : t.data.slice(idx)
  })
}


// Real-time mode may show a short persisted lead-in, then continuously appends
// fresh protocol values. Historical mode only calls this flow when the user
// explicitly presses the query button.
async function loadPersistedHistory(from: number, to: number, generation = requestGeneration) {
  const byInstance = new Map<string, number[]>()
  panelTraces.value.forEach(t => {
    const ioas = byInstance.get(t.instId) || []
    ioas.push(t.ioa)
    byInstance.set(t.instId, ioas)
  })

  for (const [instId, ioas] of byInstance) {
    try {
      const res = await getPointHistory(instId, ioas, from, to)
      // Ignore late responses after switching modes, clearing, or re-querying.
      if (disposed || generation !== requestGeneration) return
      for (const series of res.series) {
        const trace = panelTraces.value.find(t => t.instId === instId && t.ioa === series.ioa)
        if (trace) trace.data = series.samples
      }
    } catch {
      // A stopped instance or unavailable persistence backend must not break the panel.
    }
  }
}

async function backfillHistory() {
  if (panelTraces.value.length === 0 || disposed) return
  const generation = requestGeneration
  const status = await getPersistenceStatus()
  if (!status.enabled || disposed || generation !== requestGeneration) return
  const retention = status.retention_minutes || 60
  const to = Date.now()
  await loadPersistedHistory(to - Math.min(props.timeRange, retention) * 60 * 1000, to, generation)
  if (!disposed && generation === requestGeneration) updateChart()
}

async function queryHistory() {
  if (disposed || props.historyFrom === null || props.historyTo === null) return
  const generation = ++requestGeneration
  panelTraces.value.forEach(trace => { trace.data = [] })
  await loadPersistedHistory(props.historyFrom, props.historyTo, generation)
  if (!disposed && generation === requestGeneration) updateChart()
}

async function fetchAllPoints() {
  if (props.mode !== 'realtime' || panelTraces.value.length === 0 || paused.value || disposed || fetchInFlight) return
  const generation = requestGeneration
  const sampledAt = Date.now()
  fetchInFlight = true
  try {
    const byInstance = new Map<string, number[]>()
    panelTraces.value.forEach(t => {
      if (!byInstance.has(t.instId)) byInstance.set(t.instId, [])
      byInstance.get(t.instId)!.push(t.ioa)
    })
    for (const [instId, ioas] of byInstance) {
      const res = await readPointsBatch(instId, ioas)
      if (disposed || props.mode !== 'realtime' || generation !== requestGeneration) return
      for (const pt of res.points) {
        const trace = panelTraces.value.find(t => t.instId === instId && t.ioa === pt.ioa)
        if (!trace) continue
        let value = pt.value
        if (pt.point_type === 'DI' || pt.point_type === 'DO') value = pt.bool_value ? 1 : 0
        else if (pt.point_type === 'PI') value = pt.int_value
        const last = trace.data[trace.data.length - 1]
        if (last && last[0] === sampledAt) last[1] = value
        else trace.data.push([sampledAt, value])
      }
    }
    if (disposed || generation !== requestGeneration) return
    lastUpdate.value = new Date(sampledAt).toLocaleTimeString()
    trimData()
    updateChart()
  } catch {
    // The next interval retries transient read failures without overlapping requests.
  } finally {
    fetchInFlight = false
    if (restartPending) {
      restartPending = false
      void fetchAllPoints()
    } else if (resetting.value) {
      resetting.value = false
    }
  }
}

function restartTimer() {
  if (pollTimer) clearInterval(pollTimer)
  if (props.mode === 'realtime' && !paused.value) {
    fetchAllPoints()
    pollTimer = setInterval(fetchAllPoints, localInterval.value)
  }
}

function togglePause() {
  if (props.mode !== 'realtime') return
  paused.value = !paused.value
  if (paused.value) {
    if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
    lastUpdate.value = '已暂停'
  } else {
    pollTimer = setInterval(fetchAllPoints, localInterval.value)
  }
}

function restartFromNow() {
  if (props.mode !== 'realtime') return
  // Invalidate both in-flight history reads and live read responses before clearing.
  requestGeneration += 1
  panelTraces.value.forEach(trace => { trace.data = [] })
  resetting.value = true
  updateChart()
  if (fetchInFlight) {
    restartPending = true
  } else {
    void fetchAllPoints()
  }
}

function removeTrace(i: number) {
  panelTraces.value.splice(i, 1)
  
  // 重新计算所有测点的colorIdx，确保颜色连续
  panelTraces.value.forEach((t, idx) => {
    t.colorIdx = idx
  })
  
  // Notify parent of trace removal so template save reflects the change
  emit('tracesChanged', props.panelId, panelTraces.value.map(t => ({
    instId: t.instId, inst: t.inst, ioa: t.ioa, name: t.name,
    unit: t.unit, alias: t.alias, colorIdx: t.colorIdx,
  })))
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
    // 格式：yyyy-mm-dd hh:mm:ss.000
    const year = d.getFullYear()
    const month = String(d.getMonth() + 1).padStart(2, '0')
    const day = String(d.getDate()).padStart(2, '0')
    const hours = String(d.getHours()).padStart(2, '0')
    const minutes = String(d.getMinutes()).padStart(2, '0')
    const seconds = String(d.getSeconds()).padStart(2, '0')
    const ms = String(d.getMilliseconds()).padStart(3, '0')
    const tStr = `${year}-${month}-${day} ${hours}:${minutes}:${seconds}.${ms}`
    
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
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

function startPolling() {
  if (pollTimer) clearInterval(pollTimer)
  if (props.mode === 'realtime') {
    pollTimer = setInterval(fetchAllPoints, localInterval.value)
  }
}

onMounted(() => {
  // Initialize traces from props
  panelTraces.value = props.traces.map(t => ({ ...t, data: [] }))
  nextTick(async () => {
    if (panelTraces.value.length > 0) {
      initChart()
      if (props.mode === 'realtime') {
        await backfillHistory()
        await fetchAllPoints()
        startPolling()
      }
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
