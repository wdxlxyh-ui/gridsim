<template>
  <el-card shadow="never" class="panel-card" :class="[`panel-card--${props.panelKind || 'pending'}`, { 'is-paused': paused }]">
    <template #header>
      <div class="panel-header">
        <div class="panel-identity">
          <span class="panel-beacon" :class="{ 'is-history': props.mode === 'history', 'is-paused': paused }"><i></i></span>
          <div class="panel-heading">
            <span class="panel-overline">{{ props.mode === 'realtime' ? 'LIVE TREND' : 'HISTORICAL REPLAY' }}</span>
            <div class="panel-name-row">
              <strong>{{ panelKindShortLabel }}</strong>
              <el-tag size="small" effect="plain" class="panel-kind-tag">{{ panelKindLabel }}</el-tag>
            </div>
          </div>
          <span class="panel-count">{{ visibleTraceCount }} / {{ compatibleTraceCount }} 路可见</span>
        </div>
        <div class="panel-controls">
          <span v-if="props.mode === 'realtime'" class="update-state" :class="{ 'is-paused': paused }">{{ paused ? '已暂停' : `采样 ${lastUpdate}` }}</span>
          <span v-else-if="historyWindow" class="history-state" :class="{ 'is-clamped': historyWindow.clamped }" :title="historyWindowTitle">{{ historyWindow.clamped ? '已截断' : formatHistoryWindow(historyWindow) }}</span>
          <span v-else class="history-state">等待查询</span>
          <el-select v-if="props.mode === 'realtime'" v-model="localInterval" size="small" class="interval-select" @change="restartTimer" aria-label="刷新间隔">
            <el-option label="200 ms" :value="200" />
            <el-option label="500 ms" :value="500" />
            <el-option label="1 秒" :value="1000" />
            <el-option label="2 秒" :value="2000" />
            <el-option label="5 秒" :value="5000" />
          </el-select>
          <el-button v-if="props.mode === 'realtime'" size="small" :type="paused ? 'warning' : 'info'" plain @click="togglePause">{{ paused ? '继续' : '暂停' }}</el-button>
          <el-button v-if="props.mode === 'realtime'" size="small" :loading="resetting" title="清空当前曲线，并从当前时刻重新采样" @click="restartFromNow">重新采样</el-button>
          <el-button size="small" class="export-button" @click="downloadCSV">导出 CSV</el-button>
          <el-button size="small" type="danger" text class="remove-panel-button" title="移除该面板" aria-label="移除该面板" @click="$emit('remove', panelId)">移除</el-button>
        </div>
      </div>
    </template>

    <div v-if="panelTraces.length === 0" class="panel-empty">
      <div class="empty-chart-mark"><span></span><span></span><span></span></div>
      <strong>尚未配置趋势测点</strong>
      <span>添加测点后，系统会按采集与控制类型自动分配曲线。</span>
      <el-button size="small" type="primary" plain @click="$emit('addTrace', panelId)">添加测点</el-button>
    </div>
    <div v-else class="panel-workspace">
      <div class="chart-ruler">
        <span>{{ chartTrackLabel }}</span>
        <span>{{ props.mode === 'realtime' ? '滚轮缩放 · 拖拽浏览 · 悬停查看样本' : '固定历史结果 · 悬停查看样本' }}</span>
      </div>
      <div ref="chartRef" class="panel-chart"></div>
      <div class="trace-dock">
        <div class="trace-dock-heading">
          <div><span class="dock-kicker">TRACE DIRECTORY</span><strong>曲线清单</strong></div>
          <span>{{ compatibleTraceCount }} 路已配置</span>
        </div>
        <div class="trace-list">
          <div v-for="(t, i) in panelTraces" :key="traceKey(t)" class="trace-chip" :class="{ 'is-hidden': isTraceHidden(t), 'is-incompatible': !isTraceCompatibleWithPanel(t) }">
            <button class="trace-visibility" type="button" :title="isTraceHidden(t) ? '显示曲线' : '隐藏曲线'" :aria-pressed="!isTraceHidden(t)" @click="toggleTraceVisibility(t)" @mouseenter="highlightTrace(t)" @mouseleave="downplayTrace">
              <i :style="{ backgroundColor: traceColor(t) }"></i>
            </button>
            <button class="trace-details" type="button" :title="`${t.inst} · ${t.alias || t.name || 'IOA:' + t.ioa}`" @click="toggleTraceVisibility(t)" @mouseenter="highlightTrace(t)" @mouseleave="downplayTrace">
              <span class="trace-name">{{ t.alias || t.name || 'IOA:' + t.ioa }}</span>
              <span class="trace-meta">{{ t.inst }} · {{ t.pointType || '识别中' }}<template v-if="t.unit"> · {{ t.unit }}</template></span>
            </button>
            <button class="trace-remove" type="button" title="移除曲线" :aria-label="`移除 ${t.alias || t.name || '测点'}`" @click="removeTrace(i)">×</button>
          </div>
          <button class="add-trace-button" type="button" @click="$emit('addTrace', panelId)"><span>+</span> 添加测点</button>
        </div>
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import * as echarts from 'echarts'
import { readPointsBatch, getPersistenceStatus, getPointHistory } from '../api'

const COLORS = ['#14b8a6', '#f59e0b', '#3b82f6', '#a855f7', '#ec4899', '#22d3ee', '#f97316', '#8b5cf6']

type PointType = 'AI' | 'DI' | 'PI' | 'AO' | 'DO'

interface TraceConfig {
  instId: string; inst: string; ioa: number; name: string; unit: string
  alias: string; colorIdx: number
  pointType?: PointType
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

type PanelKind = 'collection' | 'ao-control' | 'do-control'

const PANEL_KIND_LABEL: Record<PanelKind, string> = {
  collection: '采集趋势：AI / DI / PI',
  'ao-control': 'AO · 设定值与写入记录',
  'do-control': 'DO · 开关状态与写入记录',
}

const props = defineProps<{
  panelId: string
  panelKind?: PanelKind
  traces: TraceConfig[]
  timeRange: number
  pollInterval: number
  mode: 'realtime' | 'history'
  historyFrom: number | null
  historyTo: number | null
  historyQueryToken: number
  historyCleanup: { token: number; instanceId: string; ioas: number[] } | null
}>()

const panelKindLabel = computed(() => props.panelKind ? PANEL_KIND_LABEL[props.panelKind] : '正在识别测点类型')

const emit = defineEmits<{
  remove: [panelId: string]
  addTrace: [panelId: string]
  tracesChanged: [panelId: string, traces: TraceConfig[]]
}>()

const panelTraces = ref<Trace[]>([])
const hiddenTraceKeys = ref(new Set<string>())
const localInterval = ref(props.pollInterval)
const paused = ref(false)
const lastUpdate = ref('--')
const historyWindow = ref<{ from: number; to: number; clamped: boolean; retentionMinutes: number } | null>(null)

const panelKindShortLabel = computed(() => {
  if (props.panelKind === 'ao-control') return 'AO 指令记录'
  if (props.panelKind === 'do-control') return 'DO 指令记录'
  if (props.panelKind === 'collection') return '采集趋势'
  return '趋势面板'
})
const chartTrackLabel = computed(() => {
  if (props.panelKind === 'ao-control') return '阶梯线：当前设定值 · 菱形：每次写入'
  if (props.panelKind === 'do-control') return '阶梯线：当前开关状态 · 圆点：每次写入'
  return 'AI/PI 数值趋势 · DI 状态轨道'
})
const compatibleTraceCount = computed(() => panelTraces.value.filter(isTraceCompatibleWithPanel).length)
const visibleTraceCount = computed(() => panelTraces.value.filter(trace => isTraceCompatibleWithPanel(trace) && !isTraceHidden(trace)).length)

function traceKey(trace: Pick<Trace, 'instId' | 'ioa'>): string {
  return `${trace.instId}:${trace.ioa}`
}

function isTraceCompatibleWithPanel(trace: Trace): boolean {
  return Boolean(props.panelKind && traceBelongsToPanel(trace, props.panelKind))
}

function isTraceHidden(trace: Trace): boolean {
  return hiddenTraceKeys.value.has(traceKey(trace))
}

function toggleTraceVisibility(trace: Trace) {
  const next = new Set(hiddenTraceKeys.value)
  const key = traceKey(trace)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  hiddenTraceKeys.value = next
  updateChart()
}

function highlightTrace(trace: Trace) {
  if (isTraceHidden(trace)) return
  chartInstance?.dispatchAction({ type: 'highlight', seriesName: traceLabel(trace) })
}

function downplayTrace() {
  chartInstance?.dispatchAction({ type: 'downplay' })
}

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
let chartInteractionActive = false
let redrawPending = false
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
        pointType: cfg.pointType || existing.pointType,
      })
    } else {
      newTraces.push({ ...cfg, data: [] })
    }
  }
  
  panelTraces.value = newTraces
  const activeTraceKeys = new Set(newTraces.map(traceKey))
  hiddenTraceKeys.value = new Set([...hiddenTraceKeys.value].filter(key => activeTraceKeys.has(key)))
  
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

watch(() => props.panelKind, () => {
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
  const zr = chartInstance.getZr()
  zr.on('mousedown', beginChartInteraction)
  zr.on('mouseup', endChartInteraction)
  zr.on('globalout', endChartInteraction)
  // ResizeObserver keeps the chart responsive without increasing polling frequency.
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

function traceBelongsToPanel(trace: Trace, kind: PanelKind): boolean {
  if (kind === 'collection') return trace.pointType === 'AI' || trace.pointType === 'DI' || trace.pointType === 'PI'
  if (kind === 'ao-control') return trace.pointType === 'AO'
  return trace.pointType === 'DO'
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
    lines.push('<div style="margin-top:4px;color:#fbbf24">写入记录</div>')
    events.forEach(row => {
      const value = Array.isArray(row.value) ? row.value[1] : row.value
      const name = String(row.seriesName).slice('event:'.length)
      lines.push(`<div>${row.marker || ''}${name}: <b>${value}</b></div>`)
    })
  }
  return lines.join('')
}

function beginChartInteraction() {
  if (disposed) return
  chartInteractionActive = true
}

function endChartInteraction() {
  if (!chartInteractionActive) return
  chartInteractionActive = false
  redrawPending = false
  updateChart(true)
}

function updateChart(force = false) {
  if (!chartInstance) return
  if (chartInteractionActive && !force) {
    redrawPending = true
    return
  }

  redrawPending = false

  const kind = props.panelKind
  const traces = kind
    ? panelTraces.value.filter(trace => traceBelongsToPanel(trace, kind) && !isTraceHidden(trace))
    : []
  const numericTraces = kind === 'collection'
    ? traces.filter(trace => trace.pointType === 'AI' || trace.pointType === 'PI')
    : kind === 'ao-control' ? traces : []
  const binaryTraces = kind === 'collection'
    ? traces.filter(trace => trace.pointType === 'DI')
    : kind === 'do-control' ? traces : []
  const hasNumeric = numericTraces.length > 0
  const hasBinary = binaryTraces.length > 0
  const splitTracks = kind === 'collection' && hasNumeric && hasBinary
  const numericAxisIndex = hasNumeric ? 0 : -1
  const binaryAxisIndex = hasNumeric ? 1 : 0
  const series: any[] = []

  const grids: any[] = splitTracks
    ? [
        { left: 58, right: 20, top: 30, height: '47%' },
        { left: 58, right: 20, top: '70%', bottom: 38 },
      ]
    : [{ left: 58, right: 20, top: 30, bottom: 38 }]
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
      titles.push({ text: 'AI / PI 数值趋势', left: 52, top: 3, textStyle: { color: '#94a3b8', fontSize: 10, fontWeight: 'normal' } })
    } else if (kind === 'ao-control') {
      titles.push({ text: 'AO 设定值', left: 52, top: 3, textStyle: { color: '#94a3b8', fontSize: 10, fontWeight: 'normal' } })
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
      titles.push({ text: 'DI 状态', left: 52, top: '65%', textStyle: { color: '#94a3b8', fontSize: 10, fontWeight: 'normal' } })
    } else if (kind === 'do-control') {
      titles.push({ text: 'DO 开关状态', left: 52, top: 3, textStyle: { color: '#94a3b8', fontSize: 10, fontWeight: 'normal' } })
    }
  }

  traces.forEach(trace => {
    const label = traceLabel(trace)
    const color = traceColor(trace)
    const binary = isBinaryTrace(trace)
    const control = isControlTrace(trace)
    const axisIndex = binary ? binaryAxisIndex : numericAxisIndex
    const controlAO = trace.pointType === 'AO'
    const controlDO = trace.pointType === 'DO'

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
      sampling: !control && trace.data.length > 1000 ? 'lttb' : undefined,
      large: !control && trace.data.length > 4000,
      largeThreshold: 4000,
      emphasis: { focus: 'series' },
      areaStyle: control || binary ? undefined : {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: color + '40' },
          { offset: 1, color: color + '05' },
        ]),
      },
      z: control ? 2 : 1,
    })

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
    animationDurationUpdate: 0,
    aria: { enabled: true },
    title: titles,
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'rgba(7, 15, 29, 0.96)',
      borderColor: '#31537d',
      borderWidth: 1,
      padding: [8, 10],
      textStyle: { color: '#e6f0ff', fontSize: 11, fontFamily: 'Consolas, monospace' },
      axisPointer: { type: 'line', lineStyle: { color: '#6ca7e8', type: 'dashed', opacity: 0.7 } },
      formatter: formatTrendTooltip,
    },
    legend: { show: false },
    grid: grids,
    xAxis,
    yAxis,
    dataZoom: [
      { type: 'inside', xAxisIndex: xAxis.map((_axis, index) => index), orient: 'horizontal' },
      { type: 'slider', xAxisIndex: xAxis.map((_axis, index) => index), bottom: 8, height: 12, borderColor: '#334155', backgroundColor: '#1e293b',
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
  const [removed] = panelTraces.value.splice(i, 1)
  if (removed) {
    const next = new Set(hiddenTraceKeys.value)
    next.delete(traceKey(removed))
    hiddenTraceKeys.value = next
  }
  panelTraces.value.forEach((trace, index) => { trace.colorIdx = index })

  emit('tracesChanged', props.panelId, panelTraces.value.map(trace => ({
    instId: trace.instId, inst: trace.inst, ioa: trace.ioa, name: trace.name,
    unit: trace.unit, alias: trace.alias, colorIdx: trace.colorIdx, pointType: trace.pointType as PointType | undefined,
  })))
  if (panelTraces.value.length === 0) {
    if (chartInstance) { chartInstance.dispose(); chartInstance = null }
    if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
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
  --panel-bg: #0c1729;
  --panel-bg-elevated: #101f36;
  --panel-line: rgba(97, 128, 169, 0.28);
  --panel-muted: #8da1bd;
  --panel-text: #e6effd;
  position: relative;
  overflow: hidden;
  background: linear-gradient(145deg, rgba(14, 29, 51, 0.98), rgba(8, 18, 34, 0.98));
  border: 1px solid var(--panel-line);
  box-shadow: 0 12px 32px rgba(1, 7, 20, 0.18), inset 0 1px 0 rgba(164, 199, 255, 0.04);
  transition: border-color 0.22s ease, box-shadow 0.22s ease, transform 0.22s ease;
}
.panel-card::before { position: absolute; top: 0; left: 0; width: 96px; height: 2px; content: ''; background: #4d94ff; box-shadow: 0 0 16px rgba(77, 148, 255, 0.8); }
.panel-card--ao-control::before { background: #e8a43a; box-shadow: 0 0 16px rgba(232, 164, 58, 0.7); }
.panel-card--do-control::before { background: #b18cff; box-shadow: 0 0 16px rgba(177, 140, 255, 0.7); }
.panel-card:hover { border-color: rgba(110, 163, 229, 0.5); box-shadow: 0 16px 38px rgba(1, 7, 20, 0.28), inset 0 1px 0 rgba(164, 199, 255, 0.06); }
.panel-card :deep(.el-card__header) { padding: 12px 14px 11px; border-bottom: 1px solid var(--panel-line); background: rgba(5, 14, 29, 0.24); }
.panel-card :deep(.el-card__body) { padding: 0; }
.panel-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.panel-identity { display: flex; align-items: center; min-width: 0; gap: 10px; }
.panel-beacon { display: grid; flex: 0 0 auto; width: 24px; height: 24px; place-items: center; border: 1px solid rgba(70, 155, 254, 0.35); border-radius: 6px; background: rgba(37, 113, 213, 0.12); }
.panel-beacon i { width: 7px; height: 7px; border-radius: 50%; background: #58d5a7; box-shadow: 0 0 0 3px rgba(88, 213, 167, 0.12), 0 0 11px rgba(88, 213, 167, 0.68); }
.panel-beacon.is-history { border-color: rgba(232, 164, 58, 0.38); background: rgba(232, 164, 58, 0.1); }
.panel-beacon.is-history i, .panel-beacon.is-paused i { background: #e8a43a; box-shadow: 0 0 0 3px rgba(232, 164, 58, 0.12), 0 0 11px rgba(232, 164, 58, 0.62); }
.panel-heading { min-width: 0; }
.panel-overline, .dock-kicker { display: block; color: #718aac; font-family: Consolas, 'Courier New', monospace; font-size: 9px; font-weight: 700; letter-spacing: 0.12em; line-height: 1.2; }
.panel-name-row { display: flex; align-items: center; min-width: 0; gap: 7px; margin-top: 3px; }
.panel-name-row strong { overflow: hidden; color: var(--panel-text); font-size: 13px; font-weight: 650; text-overflow: ellipsis; white-space: nowrap; }
.panel-kind-tag { max-width: 220px; overflow: hidden; border-color: rgba(114, 145, 184, 0.35); color: #9eb6d4; background: rgba(65, 89, 124, 0.14); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.panel-card--ao-control .panel-kind-tag { border-color: rgba(232, 164, 58, 0.35); color: #eebc6d; background: rgba(232, 164, 58, 0.09); }
.panel-card--do-control .panel-kind-tag { border-color: rgba(177, 140, 255, 0.35); color: #c6adff; background: rgba(177, 140, 255, 0.09); }
.panel-count { padding-left: 10px; border-left: 1px solid var(--panel-line); color: #8095b2; font-family: Consolas, 'Courier New', monospace; font-size: 10px; white-space: nowrap; }
.panel-controls { display: flex; align-items: center; justify-content: flex-end; flex-wrap: wrap; gap: 6px; }
.update-state, .history-state { max-width: 156px; overflow: hidden; padding: 4px 7px; border: 1px solid rgba(79, 138, 208, 0.25); border-radius: 4px; color: #8eb8eb; background: rgba(41, 99, 172, 0.1); font-family: Consolas, 'Courier New', monospace; font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.update-state.is-paused, .history-state.is-clamped { color: #efbc6a; border-color: rgba(232, 164, 58, 0.3); background: rgba(232, 164, 58, 0.08); }
.history-state { color: #9eb3cc; border-color: rgba(126, 153, 188, 0.24); background: rgba(70, 91, 122, 0.1); }
.interval-select { width: 82px; }
.panel-controls :deep(.el-button) { border-color: rgba(98, 127, 166, 0.36); color: #b4c5dc; background: rgba(17, 34, 57, 0.72); }
.panel-controls :deep(.el-button:hover) { border-color: #4d94ff; color: #edf5ff; background: rgba(47, 107, 191, 0.18); }
.panel-controls :deep(.el-button--warning) { color: #f0bd6b; }
.panel-controls :deep(.el-button--danger.is-text) { border-color: transparent; color: #b98c99; background: transparent; }
.panel-controls :deep(.el-button--danger.is-text:hover) { color: #ffb1bf; background: rgba(200, 67, 92, 0.1); }
.export-button { color: #9ed3c1 !important; }
.remove-panel-button { padding-left: 2px; padding-right: 2px; }
.panel-workspace { min-width: 0; }
.chart-ruler { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 8px 14px 0; color: #8196b3; font-size: 10px; }
.chart-ruler span:first-child { color: #a8bdd7; font-family: Consolas, 'Courier New', monospace; letter-spacing: 0.02em; }
.panel-chart { width: 100%; height: 336px; }
.trace-dock { padding: 0 12px 12px; }
.trace-dock-heading { display: flex; align-items: end; justify-content: space-between; padding: 9px 2px 7px; border-top: 1px solid var(--panel-line); }
.trace-dock-heading strong { display: block; margin-top: 2px; color: #d6e3f5; font-size: 11px; font-weight: 600; }
.trace-dock-heading > span { color: #7188a7; font-size: 10px; }
.trace-list { display: flex; flex-wrap: wrap; gap: 6px; max-height: 116px; overflow: auto; padding: 1px 2px 2px; }
.trace-chip { display: flex; align-items: stretch; min-width: 0; max-width: min(100%, 260px); border: 1px solid rgba(91, 121, 159, 0.36); border-radius: 5px; background: rgba(17, 33, 55, 0.58); transition: border-color 0.18s ease, opacity 0.18s ease, background 0.18s ease; }
.trace-chip:hover { border-color: rgba(110, 170, 242, 0.66); background: rgba(31, 65, 108, 0.38); }
.trace-chip.is-hidden { opacity: 0.5; }
.trace-chip.is-hidden .trace-name, .trace-chip.is-hidden .trace-meta { text-decoration: line-through; }
.trace-chip.is-incompatible { border-style: dashed; opacity: 0.58; }
.trace-visibility, .trace-details, .trace-remove, .add-trace-button { font: inherit; cursor: pointer; }
.trace-visibility { display: grid; width: 26px; padding: 0; place-items: center; border: 0; border-right: 1px solid rgba(91, 121, 159, 0.26); border-radius: 5px 0 0 5px; background: transparent; }
.trace-visibility i { width: 10px; height: 3px; border-radius: 2px; box-shadow: 0 0 7px currentColor; }
.trace-details { display: grid; min-width: 0; flex: 1; gap: 2px; padding: 5px 6px; border: 0; color: inherit; background: transparent; text-align: left; }
.trace-name, .trace-meta { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.trace-name { color: #cfddf1; font-size: 11px; line-height: 1.25; }
.trace-meta { color: #788eab; font-family: Consolas, 'Courier New', monospace; font-size: 9px; line-height: 1.2; }
.trace-remove { width: 22px; padding: 0; border: 0; border-radius: 0 5px 5px 0; color: #7890ae; background: transparent; font-size: 16px; line-height: 1; transition: color 0.16s ease, background 0.16s ease; }
.trace-remove:hover { color: #ffc1cb; background: rgba(211, 76, 100, 0.15); }
.add-trace-button { display: inline-flex; align-items: center; gap: 5px; min-height: 42px; padding: 0 10px; border: 1px dashed rgba(93, 143, 203, 0.55); border-radius: 5px; color: #90bee9; background: rgba(40, 90, 154, 0.08); font-size: 11px; transition: color 0.18s ease, border-color 0.18s ease, background 0.18s ease; }
.add-trace-button span { font-size: 16px; font-weight: 300; line-height: 1; }
.add-trace-button:hover { border-color: #67a9fa; color: #e6f1ff; background: rgba(49, 115, 202, 0.18); }
.panel-empty { display: flex; min-height: 352px; flex-direction: column; align-items: center; justify-content: center; gap: 8px; padding: 20px; color: #8196b2; font-size: 12px; text-align: center; }
.panel-empty strong { margin-top: 4px; color: #d6e4f6; font-size: 14px; }
.panel-empty :deep(.el-button) { margin-top: 8px; }
.empty-chart-mark { display: flex; align-items: end; gap: 4px; height: 35px; padding: 9px 10px; border: 1px solid rgba(76, 139, 210, 0.3); border-radius: 8px; background: rgba(33, 82, 144, 0.1); }
.empty-chart-mark span { width: 5px; border-radius: 2px 2px 0 0; background: #5e9ef0; box-shadow: 0 0 9px rgba(94, 158, 240, 0.55); }
.empty-chart-mark span:nth-child(1) { height: 11px; }.empty-chart-mark span:nth-child(2) { height: 22px; }.empty-chart-mark span:nth-child(3) { height: 16px; }
@media (max-width: 920px) { .panel-header { align-items: flex-start; flex-direction: column; } .panel-controls { justify-content: flex-start; } .panel-chart { height: 320px; } }
@media (max-width: 560px) { .panel-card :deep(.el-card__header) { padding: 10px; } .panel-identity { align-items: flex-start; } .panel-count { display: none; } .panel-kind-tag { max-width: 155px; } .panel-controls { width: 100%; } .update-state, .history-state { max-width: 130px; } .chart-ruler { align-items: flex-start; flex-direction: column; gap: 3px; padding-left: 10px; } .panel-chart { height: 292px; } .trace-dock { padding: 0 9px 9px; } .trace-chip { max-width: 100%; flex: 1 1 178px; } .trace-list { max-height: 148px; } }
@media (prefers-reduced-motion: reduce) { .panel-card, .trace-chip, .trace-remove, .add-trace-button { transition: none; } .panel-beacon i { box-shadow: none; } }
</style>
