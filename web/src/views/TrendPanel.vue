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
          <el-tag v-if="props.mode === 'history' && historyWindow" size="small" :type="historyWindow.clamped ? 'warning' : 'success'" effect="plain" :title="historyWindowTitle">
            {{ historyWindow.clamped ? '已截断：' : '实际范围：' }}{{ formatHistoryWindow(historyWindow) }}
          </el-tag>
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
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
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
  pointType?: string
  data: [number, number][]
}

const COLLECTION_POINT_TYPES = new Set(['AI', 'DI', 'PI'])
const CONTROL_POINT_TYPES = new Set(['AO', 'DO'])

function pointValue(point: { point_type: string; value: number; bool_value: boolean; int_value: number }): number {
  if (point.point_type === 'DI' || point.point_type === 'DO') return point.bool_value ? 1 : 0
  if (point.point_type === 'PI') return point.int_value
  return point.value
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
  historyCleanup: { token: number; instanceId: string; ioas: number[] } | null
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
const historyWindow = ref<{ from: number; to: number; clamped: boolean; retentionMinutes: number } | null>(null)
const historyWindowTitle = computed(() => {
  const window = historyWindow.value
  if (!window) return ''
  const prefix = window.clamped ? `请求范围早于保留窗口，已截断为实际范围；当前保留期 ${window.retentionMinutes} 分钟。` : '历史查询实际返回范围。'
  return `${prefix}${formatHistoryWindow(window)}`
})

const chartRef = ref<HTMLElement | null>(null)
let chartInstance: echarts.ECharts | null = null
let resizeObserver: ResizeObserver | null = null
let pollTimer: ReturnType<typeof setInterval> | null = null
let disposed = false
let fetchInFlight = false
let restartPending = false
let requestGeneration = 0
let controlEventFromAt = Date.now()
let persistenceEnabled = false
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
        controlEventFromAt = Date.now()
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
    controlEventFromAt = Date.now()
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

watch(() => props.historyCleanup?.token, () => {
  const cleanup = props.historyCleanup
  if (!cleanup) return
  requestGeneration += 1
  let changed = false
  panelTraces.value.forEach(trace => {
    if (trace.instId === cleanup.instanceId && cleanup.ioas.includes(trace.ioa)) {
      trace.data = []
      changed = true
    }
  })
  if (changed) updateChart()
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

function traceLabel(trace: Trace): string {
  return `${trace.inst} · ${trace.alias || trace.name}`
}

function traceColor(trace: Trace): string {
  return COLORS[trace.colorIdx % COLORS.length]
}

function isBinaryTrace(trace: Trace): boolean {
  return trace.pointType === 'DI' || trace.pointType === 'DO'
}

function isControlTrace(trace: Trace): boolean {
  return trace.pointType === 'AO' || trace.pointType === 'DO'
}

function formatTrendTooltip(params: any[]): string {
  const rows = Array.isArray(params) ? params : [params]
  if (rows.length === 0) return ''
  const axisValue = rows[0]?.axisValue
  const timestamp = typeof axisValue === 'number' ? axisValue : Date.parse(axisValue)
  const heading = Number.isFinite(timestamp) ? new Date(timestamp).toLocaleString() : String(axisValue || '')
  const states = rows.filter(row => !String(row.seriesName || '').startsWith('event:'))
  const events = rows.filter(row => String(row.seriesName || '').startsWith('event:'))
  const lines = [`<div style="margin-bottom:4px;color:#cbd5e1">${heading}</div>`]
  states.forEach(row => {
    const value = Array.isArray(row.value) ? row.value[1] : row.value
    lines.push(`<div>${row.marker || ''}${row.seriesName}: <b>${value}</b></div>`)
  })
  if (events.length > 0) {
    lines.push('<div style="margin-top:4px;color:#fbbf24">控制事件</div>')
    events.forEach(row => {
      const value = Array.isArray(row.value) ? row.value[1] : row.value
      const name = String(row.seriesName).slice('event:'.length)
      lines.push(`<div>${row.marker || ''}${name}: <b>${value}</b></div>`)
    })
  }
  return lines.join('')
}

function updateChart() {
  if (!chartInstance) return

  const numericTraces = panelTraces.value.filter(trace => !isBinaryTrace(trace))
  const binaryTraces = panelTraces.value.filter(isBinaryTrace)
  const hasNumeric = numericTraces.length > 0
  const hasBinary = binaryTraces.length > 0
  const splitTracks = hasNumeric && hasBinary
  const numericAxisIndex = hasNumeric ? 0 : -1
  const binaryAxisIndex = hasNumeric ? 1 : 0
  const legendNames: string[] = []
  const series: any[] = []

  const grids: any[] = splitTracks
    ? [
        { left: 52, right: 16, top: 28, height: '51%' },
        { left: 52, right: 16, top: '72%', bottom: 52 },
      ]
    : [{ left: 52, right: 16, top: 24, bottom: 52 }]
  const xAxis: any[] = []
  const yAxis: any[] = []
  const titles: any[] = []

  if (hasNumeric) {
    xAxis.push({
      type: 'time',
      gridIndex: numericAxisIndex,
      axisLine: { lineStyle: { color: '#334155' } },
      axisLabel: { show: !splitTracks, color: '#64748b', fontSize: 9, formatter: formatAxisTime },
      splitLine: { lineStyle: { color: '#1e293b' } },
    })
    yAxis.push({
      type: 'value',
      gridIndex: numericAxisIndex,
      axisLine: { show: false },
      axisLabel: { color: '#64748b', fontSize: 9, formatter: (value: number) => Number(value).toFixed(1) },
      splitLine: { lineStyle: { color: '#1e293b' } },
      // Leave a small margin around zero so AO=0 event diamonds are visible.
      min: (extent: { min: number; max: number }) => {
        const span = Math.max(1, extent.max - extent.min)
        return extent.min - span * 0.05
      },
      max: (extent: { min: number; max: number }) => {
        const span = Math.max(1, extent.max - extent.min)
        return extent.max + span * 0.05
      },
    })
    if (splitTracks) {
      titles.push({ text: '数值趋势 / AO 阶梯状态', left: 52, top: 3, textStyle: { color: '#94a3b8', fontSize: 10, fontWeight: 'normal' } })
    }
  }

  if (hasBinary) {
    xAxis.push({
      type: 'time',
      gridIndex: binaryAxisIndex,
      axisLine: { lineStyle: { color: '#334155' } },
      axisLabel: { color: '#64748b', fontSize: 9, formatter: formatAxisTime },
      splitLine: { lineStyle: { color: '#1e293b' } },
    })
    yAxis.push({
      type: 'value',
      gridIndex: binaryAxisIndex,
      min: -0.15,
      max: 1.15,
      interval: 1,
      axisLine: { show: false },
      axisLabel: { color: '#64748b', fontSize: 9, formatter: (value: number) => value === 1 ? '1 / true' : value === 0 ? '0 / false' : '' },
      splitLine: { lineStyle: { color: '#1e293b' } },
    })
    if (splitTracks) {
      titles.push({ text: 'DI / DO 状态轨道（DO 圆点为控制事件）', left: 52, top: '65%', textStyle: { color: '#94a3b8', fontSize: 10, fontWeight: 'normal' } })
    }
  }

  panelTraces.value.forEach(trace => {
    const label = traceLabel(trace)
    const color = traceColor(trace)
    const binary = isBinaryTrace(trace)
    const control = isControlTrace(trace)
    const axisIndex = binary ? binaryAxisIndex : numericAxisIndex
    const controlAO = trace.pointType === 'AO'
    const controlDO = trace.pointType === 'DO'
    legendNames.push(label)

    series.push({
      name: label,
      type: 'line',
      xAxisIndex: axisIndex,
      yAxisIndex: axisIndex,
      data: trace.data,
      smooth: false,
      step: control || trace.pointType === 'DI' ? 'end' : false,
      symbol: 'none',
      lineStyle: { color, width: control ? 2 : 1.5 },
      areaStyle: control || binary ? undefined : {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: color + '40' },
          { offset: 1, color: color + '05' },
        ]),
      },
      z: control ? 2 : 1,
    })

    // Every AO/DO persisted write remains visible, including a repeated write of
    // the same value that would not create a visible step transition by itself.
    if (controlAO || controlDO) {
      series.push({
        name: `event:${label}`,
        type: 'scatter',
        xAxisIndex: axisIndex,
        yAxisIndex: axisIndex,
        data: trace.data,
        symbol: controlAO ? 'diamond' : 'circle',
        symbolSize: controlAO ? 10 : 8,
        itemStyle: { color, borderColor: '#e2e8f0', borderWidth: 1 },
        z: 5,
      })
    }
  })

  chartInstance.setOption({
    animation: false,
    title: titles,
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#1a1f2e',
      borderColor: '#334155',
      textStyle: { color: '#e2e8f0', fontSize: 11, fontFamily: 'monospace' },
      formatter: formatTrendTooltip,
    },
    legend: {
      data: legendNames,
      bottom: 0,
      textStyle: { color: '#94a3b8', fontSize: 10 },
    },
    grid: grids,
    xAxis,
    yAxis,
    dataZoom: [
      { type: 'inside', xAxisIndex: xAxis.map((_axis, index) => index), orient: 'horizontal' },
      { type: 'slider', xAxisIndex: xAxis.map((_axis, index) => index), bottom: 22, height: 12, borderColor: '#334155', backgroundColor: '#1e293b',
        fillerColor: '#33415555', textStyle: { color: '#64748b', fontSize: 9 } },
    ],
    series: series.length ? series : [{ type: 'line', data: [] }],
  }, { notMerge: false, lazyUpdate: true, replaceMerge: ['series', 'xAxis', 'yAxis', 'grid', 'title'] })
}

function formatAxisTime(value: string | number): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  const seconds = String(date.getSeconds()).padStart(2, '0')
  return `${hours}:${minutes}:${seconds}`
}

function formatHistoryWindow(window: { from: number; to: number }): string {
  const format = (value: number) => {
    const date = new Date(value)
    const month = String(date.getMonth() + 1).padStart(2, '0')
    const day = String(date.getDate()).padStart(2, '0')
    return `${month}-${day} ${formatAxisTime(value)}`
  }
  return `${format(window.from)} ～ ${format(window.to)}`
}

function trimData() {
  const cutoff = Date.now() - props.timeRange * 60 * 1000
  panelTraces.value.forEach(t => {
    if (t.data.length === 0 || t.data[0][0] >= cutoff) return



    const idx = t.data.findIndex(d => d[0] >= cutoff)
    t.data = idx === -1 ? [] : t.data.slice(idx)
  })
}


// Historical mode always returns the exact user-selected persistence range. Real-time
// mode may use this data as a lead-in, but its post-reset session boundary is handled
// separately so pre-click records never leak into the new live buffer.
function appendSample(trace: Trace, sampleAt: number, value: number): boolean {
  const last = trace.data[trace.data.length - 1]
  if (last && sampleAt < last[0]) return false
  if (last && last[0] === sampleAt) last[1] = value
  else trace.data.push([sampleAt, value])
  return true
}

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
      if (disposed || generation !== requestGeneration) return
      historyWindow.value = {
        from: res.from,
        to: res.to,
        clamped: res.clamped,
        retentionMinutes: res.retention_minutes,
      }
      for (const series of res.series) {
        const trace = panelTraces.value.find(t => t.instId === instId && t.ioa === series.ioa)
        if (!trace) continue
        trace.pointType = series.point_type
        trace.data = series.samples
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
  persistenceEnabled = Boolean(status.enabled)
  if (!persistenceEnabled || disposed || generation !== requestGeneration) return
  const retention = status.retention_minutes || 24 * 60
  const to = Date.now()
  await loadPersistedHistory(to - Math.min(props.timeRange, retention) * 60 * 1000, to, generation)
  if (!disposed && generation === requestGeneration) updateChart()
}

async function queryHistory() {
  if (disposed || props.historyFrom === null || props.historyTo === null) return
  const generation = ++requestGeneration
  historyWindow.value = null
  panelTraces.value.forEach(trace => { trace.data = [] })
  await loadPersistedHistory(props.historyFrom, props.historyTo, generation)
  if (!disposed && generation === requestGeneration) updateChart()
}

// AO/DO history is the control audit stream. Re-reading from this live session's
// start is deliberate: the history API has no event cursor and this guarantees a
// delayed persistence write cannot be skipped. appendSample de-duplicates prior rows.
async function fetchControlEvents(generation: number): Promise<number | null> {
  if (!persistenceEnabled) return null

  const byInstance = new Map<string, number[]>()
  panelTraces.value
    .filter(t => !t.pointType || CONTROL_POINT_TYPES.has(t.pointType))
    .forEach(t => {
      const ioas = byInstance.get(t.instId) || []
      ioas.push(t.ioa)
      byInstance.set(t.instId, ioas)
    })

  let latestEventAt: number | null = null
  const to = Date.now()
  for (const [instId, ioas] of byInstance) {
    try {
      const res = await getPointHistory(instId, ioas, controlEventFromAt, to)
      if (disposed || props.mode !== 'realtime' || generation !== requestGeneration) return null
      for (const series of res.series) {
        const trace = panelTraces.value.find(t => t.instId === instId && t.ioa === series.ioa)
        if (!trace) continue
        trace.pointType = series.point_type
        if (!CONTROL_POINT_TYPES.has(series.point_type)) continue
        for (const [sampleAt, value] of series.samples) {
          if (sampleAt < controlEventFromAt) continue
          if (appendSample(trace, sampleAt, value)) {
            latestEventAt = latestEventAt === null ? sampleAt : Math.max(latestEventAt, sampleAt)
          }
        }
      }
    } catch {
      // Leave existing control events visible and retry on the next poll.
    }
  }
  return latestEventAt
}

async function fetchAllPoints(force = false) {
  if (props.mode !== 'realtime' || panelTraces.value.length === 0 || (!force && paused.value) || disposed || fetchInFlight) return
  const generation = requestGeneration
  let latestSampleAt: number | null = null
  fetchInFlight = true
  try {
    const byInstance = new Map<string, number[]>()
    panelTraces.value.forEach(t => {
      const ioas = byInstance.get(t.instId) || []
      ioas.push(t.ioa)
      byInstance.set(t.instId, ioas)
    })

    for (const [instId, ioas] of byInstance) {
      const res = await readPointsBatch(instId, ioas)
      if (disposed || props.mode !== 'realtime' || generation !== requestGeneration) return
      const refreshedAt = Date.parse(res.refreshed_at)
      const observedAt = Math.max(
        Number.isFinite(refreshedAt) && refreshedAt > 0 ? refreshedAt : Date.now(),
        controlEventFromAt,
      )
      for (const pt of res.points) {
        const trace = panelTraces.value.find(t => t.instId === instId && t.ioa === pt.ioa)
        if (!trace) continue
        trace.pointType = pt.point_type

        // AI/DI/PI are sampling trends. Use the server observation time for every
        // polling cycle, never their previous Store mutation time.
        if (!COLLECTION_POINT_TYPES.has(pt.point_type)) continue
        if (appendSample(trace, observedAt, pointValue(pt))) {
          latestSampleAt = latestSampleAt === null ? observedAt : Math.max(latestSampleAt, observedAt)
        }
      }
    }

    const latestControlEventAt = await fetchControlEvents(generation)
    if (disposed || generation !== requestGeneration) return
    if (latestControlEventAt !== null) {
      latestSampleAt = latestSampleAt === null
        ? latestControlEventAt
        : Math.max(latestSampleAt, latestControlEventAt)
    }
    if (latestSampleAt !== null) lastUpdate.value = new Date(latestSampleAt).toLocaleTimeString()
    trimData()
    updateChart()
  } catch {
    // The next interval retries transient read failures without overlapping requests.
  } finally {
    fetchInFlight = false
    if (restartPending) {
      restartPending = false
      void fetchAllPoints(true)
    } else if (resetting.value) {
      resetting.value = false
    }
  }
}

function restartTimer() {
  if (pollTimer) clearInterval(pollTimer)
  if (props.mode === 'realtime' && !paused.value) {
    void fetchAllPoints()
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
    void fetchAllPoints()
    pollTimer = setInterval(fetchAllPoints, localInterval.value)
  }
}

function restartFromNow() {
  if (props.mode !== 'realtime') return
  // Invalidate both in-flight history reads and live read responses before clearing.
  requestGeneration += 1
  controlEventFromAt = Date.now()
  panelTraces.value.forEach(trace => { trace.data = [] })
  resetting.value = true
  updateChart()
  if (fetchInFlight) {
    restartPending = true
  } else {
    void fetchAllPoints(true)
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
        controlEventFromAt = Date.now()
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
  height: 340px;
}
</style>
