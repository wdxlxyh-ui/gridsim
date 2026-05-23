<template>
  <div>
    <el-card shadow="never" style="margin-bottom: 16px">
      <div style="font-size: 13px; color: #94a3b8; line-height: 1.8">
        选取多个实例的测点，实时追踪数据变化趋势。时间轴基于数据真实时间戳。
        <span v-if="traces.length === 0" style="color: var(--el-color-warning)">
          请先添加测点开始监控。
        </span>
      </div>
    </el-card>

    <el-card shadow="never" style="margin-bottom: 16px">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px">
        <span style="font-size: 14px; font-weight: 500">已选测点</span>
        <span style="font-size: 12px; color: var(--el-text-color-secondary)">
          {{ traces.length }} 条曲线
        </span>
      </div>
      <div style="display: flex; flex-wrap: wrap; gap: 8px; align-items: center">
        <el-tag
          v-for="(t, i) in traces"
          :key="i"
          :color="COLORS[t.colorIdx % COLORS.length]"
          closable
          :disable-transitions="true"
          style="color: #fff; border: none"
          @close="removeTrace(i)"
        >
          {{ t.inst }} · {{ t.alias || t.name || 'IOA:' + t.ioa }}
        </el-tag>
        <el-button size="small" @click="dialogVisible = true">+ 添加测点</el-button>
      </div>
    </el-card>

    <el-card shadow="never">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; flex-wrap: wrap; gap: 8px">
        <span style="font-size: 15px; font-weight: 600">📈 实时趋势</span>
        <div style="display: flex; align-items: center; gap: 10px">
          <el-radio-group v-model="timeRange" size="small" @change="trimData">
            <el-radio-button :value="5">5m</el-radio-button>
            <el-radio-button :value="15">15m</el-radio-button>
            <el-radio-button :value="30">30m</el-radio-button>
            <el-radio-button :value="60">1h</el-radio-button>
            <el-radio-button :value="120">2h</el-radio-button>
          </el-radio-group>
          <el-divider direction="vertical" />
          <el-select v-model="pollInterval" size="small" style="width: 100px" @change="restartTimer">
            <el-option label="200ms" :value="200" />
            <el-option label="500ms" :value="500" />
            <el-option label="1s" :value="1000" />
            <el-option label="2s" :value="2000" />
            <el-option label="5s" :value="5000" />
          </el-select>
          <span style="font-size: 12px; color: var(--el-text-color-secondary)">
            {{ lastUpdate }}
          </span>
        </div>
      </div>

      <div v-if="traces.length === 0" style="text-align: center; padding: 80px 20px; color: #475569">
        <div style="font-size: 48px; margin-bottom: 12px">📊</div>
        <div>点击上方「+ 添加测点」选择要跟踪的数据</div>
      </div>

      <div v-else ref="chartRef" style="width: 100%; height: 420px"></div>
    </el-card>

    <el-dialog v-model="dialogVisible" title="添加趋势测点" width="480px" @opened="openDialog">
      <el-form label-width="60px">
        <el-form-item label="实例">
          <el-select v-model="formInst" filterable style="width: 100%" @change="onInstChange">
            <el-option v-for="inst in allInstances" :key="inst.id" :label="inst.name + ' (' + inst.id + ')'" :value="inst.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="测点">
          <el-select v-model="formIoas" filterable multiple style="width: 100%" :disabled="!formInst">
            <el-option v-for="pt in formPoints" :key="pt.ioa" :label="pt.name + ' (IOA:' + pt.ioa + ')'" :value="pt.ioa" />
          </el-select>
        </el-form-item>
        <el-form-item label="别名">
          <el-input v-model="formAlias" placeholder="可选，用于图例显示" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="addTraces">确认添加 ({{ formIoas.length }})</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import * as echarts from 'echarts'
import { listInstances, getPoints, readPointsBatch, type PointSnapshot } from '../api'

const COLORS = ['#14b8a6', '#f59e0b', '#3b82f6', '#a855f7', '#ec4899', '#22d3ee', '#f97316', '#8b5cf6']

interface Trace {
  instId: string
  inst: string
  ioa: number
  name: string
  unit: string
  alias: string
  colorIdx: number
  data: [number, number][]
}

const traces = ref<Trace[]>([])
const timeRange = ref(15)
const pollInterval = ref(1000)
const lastUpdate = ref('--')

const dialogVisible = ref(false)
const allInstances = ref<{ id: string; name: string }[]>([])
const formInst = ref('')
const formIoas = ref<number[]>([])
const formAlias = ref('')
const formPoints = ref<PointSnapshot[]>([])

const chartRef = ref<HTMLElement | null>(null)
let chartInstance: echarts.ECharts | null = null
let pollTimer: ReturnType<typeof setInterval> | null = null

function initChart() {
  if (!chartRef.value) return
  if (chartInstance) chartInstance.dispose()
  chartInstance = echarts.init(chartRef.value, undefined, { renderer: 'canvas' })
  updateChart()
}

function updateChart() {
  if (!chartInstance) return
  const series = traces.value.map((t, i) => ({
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
      textStyle: { color: '#e2e8f0', fontSize: 12, fontFamily: 'monospace' },
      formatter: (params: any) => {
        if (!params || params.length === 0) return ''
        const t = new Date(params[0].axisValue).toLocaleTimeString()
        let html = `<div style="color:#94a3b8;margin-bottom:4px;font-size:11px">${t}</div>`
        params.forEach((p: any) => {
          html += `<div style="display:flex;justify-content:space-between;gap:16px;padding:1px 0">
            <span style="display:flex;align-items:center;gap:6px;color:#94a3b8">
              <span style="width:8px;height:2px;background:${p.color};flex-shrink:0"></span>${p.seriesName}
            </span>
            <span style="font-weight:600;color:${p.color}">${typeof p.value[1] === 'number' ? p.value[1].toFixed(2) : p.value[1]}</span>
          </div>`
        })
        return html
      },
    },
    legend: {
      bottom: 0,
      textStyle: { color: '#94a3b8', fontSize: 12 },
      data: series.map(s => s.name),
    },
    grid: { left: 50, right: 20, top: 16, bottom: 60 },
    xAxis: {
      type: 'time',
      axisLine: { lineStyle: { color: '#334155' } },
      axisLabel: { color: '#64748b', fontSize: 10, formatter: (v: number) => {
        const d = new Date(v)
        return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}:${String(d.getSeconds()).padStart(2, '0')}`
      }},
      splitLine: { lineStyle: { color: '#1e293b' } },
    },
    yAxis: {
      type: 'value',
      axisLine: { show: false },
      axisLabel: { color: '#64748b', fontSize: 10, formatter: (v: number) => v.toFixed(1) },
      splitLine: { lineStyle: { color: '#1e293b' } },
    },
    dataZoom: [
      { type: 'inside', orient: 'horizontal' },
      { type: 'slider', bottom: 30, height: 16, borderColor: '#334155', backgroundColor: '#1e293b',
        fillerColor: '#33415555', textStyle: { color: '#64748b', fontSize: 10 } },
    ],
    series,
  }, true)
}

function trimData() {
  const cutoff = Date.now() - timeRange.value * 60 * 1000
  traces.value.forEach(t => {
    while (t.data.length > 0 && t.data[0][0] < cutoff) {
      t.data.shift()
    }
  })
  updateChart()
}

async function fetchAllPoints() {
  if (traces.value.length === 0) return
  const byInstance = new Map<string, number[]>()
  traces.value.forEach(t => {
    if (!byInstance.has(t.instId)) byInstance.set(t.instId, [])
    byInstance.get(t.instId)!.push(t.ioa)
  })

  for (const [instId, ioas] of byInstance) {
    try {
      const res = await readPointsBatch(instId, ioas)
      for (const pt of res.points) {
        const trace = traces.value.find(t => t.instId === instId && t.ioa === pt.ioa)
        if (!trace) continue
        const ts = pt.updated_at ? new Date(pt.updated_at).getTime() : Date.now()
        trace.data.push([ts, pt.value])
      }
    } catch {
      // instance may have stopped
    }
  }
  lastUpdate.value = new Date().toLocaleTimeString()
  trimData()
  updateChart()
}

function restartTimer() {
  if (pollTimer) clearInterval(pollTimer)
  pollTimer = setInterval(fetchAllPoints, pollInterval.value)
}

function addTraces() {
  if (!formInst.value || formIoas.value.length === 0) {
    ElMessage.warning('请选择实例和测点')
    return
  }
  const inst = allInstances.value.find(i => i.id === formInst.value)
  if (!inst) return
  for (const ioa of formIoas.value) {
    if (traces.value.some(t => t.instId === formInst.value && t.ioa === ioa)) continue
    const pt = formPoints.value.find(p => p.ioa === ioa)
    traces.value.push({
      instId: formInst.value,
      inst: inst.name,
      ioa,
      name: pt?.name || '',
      unit: pt?.unit || '',
      alias: formAlias.value,
      colorIdx: traces.value.length,
      data: [],
    })
  }
  formIoas.value = []
  formAlias.value = ''
  dialogVisible.value = false
  fetchAllPoints()
  nextTick(updateChart)
}

function removeTrace(i: number) {
  traces.value.splice(i, 1)
  updateChart()
}

async function openDialog() {
  formInst.value = ''
  formIoas.value = []
  formAlias.value = ''
  formPoints.value = []
  try {
    const list = await listInstances()
    allInstances.value = list.map(s => ({ id: s.id, name: s.name }))
    if (list.length === 0) ElMessage.warning('暂无可用实例')
  } catch {
    ElMessage.error('加载实例列表失败')
  }
}

async function onInstChange() {
  formIoas.value = []
  formPoints.value = []
  if (!formInst.value) return
  try {
    const res = await getPoints(formInst.value)
    formPoints.value = res.points
      .filter(p => p.point_type !== 'AO' && p.point_type !== 'DO')
      .sort((a, b) => a.ioa - b.ioa)
  } catch {
    ElMessage.warning('加载测点失败，请确认实例已启动')
  }
}

function saveToLocal() {
  const save = traces.value.map(t => ({
    instId: t.instId, ioa: t.ioa, alias: t.alias, colorIdx: t.colorIdx,
    inst: t.inst, name: t.name, unit: t.unit,
  }))
  localStorage.setItem('trend_traces', JSON.stringify(save))
}

function loadFromLocal() {
  try {
    const raw = localStorage.getItem('trend_traces')
    if (!raw) return
    const saved = JSON.parse(raw)
    if (!Array.isArray(saved)) return
    saved.forEach((s: any) => {
      traces.value.push({
        instId: s.instId, inst: s.inst, ioa: s.ioa,
        name: s.name || '', unit: s.unit || '',
        alias: s.alias || '', colorIdx: s.colorIdx || 0,
        data: [],
      })
    })
  } catch { /* ignore */ }
}

onMounted(() => {
  loadFromLocal()
  nextTick(() => {
    if (traces.value.length > 0) {
      initChart()
      fetchAllPoints()
      pollTimer = setInterval(fetchAllPoints, pollInterval.value)
    }
  })
})

onUnmounted(() => {
  saveToLocal()
  if (pollTimer) clearInterval(pollTimer)
  if (chartInstance) chartInstance.dispose()
})
</script>

<style scoped>
.el-card { background: #0f172a; border: 1px solid #1e293b; }
.el-card :deep(.el-card__body) { padding: 16px; }
</style>
