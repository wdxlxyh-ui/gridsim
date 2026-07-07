<template>
  <el-card shadow="never">
    <template #header>
      <div style="display:flex;justify-content:space-between;align-items:center">
        <span style="font-weight:600">组态图</span>
        <span v-if="running" :style="{color: dashboard.grid_power_kw >= 0 ? '#e6a23c' : '#67c23a', fontWeight: 600, fontSize: '13px'}">
          并网 {{ dashboard.grid_power_kw >= 0 ? '←' : '→' }} {{ Math.abs(dashboard.grid_power_kw).toFixed(1) }} kW
        </span>
      </div>
    </template>
    <div class="topo-svg-wrap">
      <svg :viewBox="`0 0 ${svgW} ${svgH}`" width="100%" xmlns="http://www.w3.org/2000/svg">
        <rect width="100%" height="100%" fill="#f8fafc" rx="8"/>

        <!-- Grid node -->
        <g>
          <rect :x="cx - 55" y="12" width="110" height="40" rx="8" fill="#fee2e2" stroke="#f87171" stroke-width="1.5"/>
          <text :x="cx" y="30" text-anchor="middle" font-size="11" fill="#991b1b" font-weight="600">⚡ 电网</text>
          <text :x="cx" y="44" text-anchor="middle" font-size="10" fill="#dc2626" font-weight="700" font-family="monospace">
            {{ running ? fmtPower(dashboard.grid_power_kw) : '—' }}
          </text>
        </g>

        <!-- Grid to bus line -->
        <line :x1="cx" y1="52" :x2="cx" :y2="busY - 6" stroke="#94a3b8" stroke-width="3" stroke-linecap="round"/>

        <!-- Bus bar -->
        <rect :x="busLeft" :y="busY - 5" :width="busRight - busLeft" height="10" rx="3" fill="#334155"/>
        <text :x="busLeft" :y="busY - 10" font-size="9" fill="#64748b">AC Bus 0.4kV</text>

        <!-- Devices -->
        <g v-for="(dev, idx) in devices" :key="dev.id">
          <!-- Branch line -->
          <line :x1="devX(idx)" :y1="busY + 5" :x2="devX(idx)" :y2="cardY" :stroke="devColor(dev.type)" stroke-width="2" stroke-linecap="round"/>
          <!-- Switch dot -->
          <circle :cx="devX(idx)" :cy="busY + 20" r="5" fill="#dcfce7" stroke="#16a34a" stroke-width="1.5"/>
          <!-- Device card -->
          <rect :x="devX(idx) - 45" :y="cardY" width="90" :height="cardH(dev)" rx="6" :fill="devBg(dev.type)" :stroke="devColor(dev.type)" stroke-width="1.2"/>
          <text :x="devX(idx)" :y="cardY + 16" text-anchor="middle" font-size="13">{{ devIcon(dev.type) }}</text>
          <text :x="devX(idx)" :y="cardY + 30" text-anchor="middle" font-size="9" font-weight="700" fill="#1e293b">{{ devLabel(dev.type) }}</text>
          <text :x="devX(idx)" :y="cardY + 42" text-anchor="middle" font-size="8" fill="#64748b">{{ dev.name }}</text>
          <!-- Power value -->
          <text :x="devX(idx)" :y="cardY + 56" text-anchor="middle" font-size="10" font-weight="700" :fill="devColor(dev.type)" font-family="monospace">
            {{ running ? fmtDevPower(dev) : '—' }}
          </text>
          <!-- SOC for battery -->
          <g v-if="dev.type === 'BESS' && running">
            <rect :x="devX(idx) - 30" :y="cardY + 62" width="60" height="6" rx="3" fill="#e2e8f0"/>
            <rect :x="devX(idx) - 30" :y="cardY + 62" :width="Math.max(0, Math.min(60, getSOC(dev) * 0.6))" height="6" rx="3" fill="#10b981"/>
            <text :x="devX(idx)" :y="cardY + 78" text-anchor="middle" font-size="8" fill="#475569">SOC {{ getSOC(dev).toFixed(0) }}%</text>
          </g>
        </g>
      </svg>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { PyMicrogridDevice, PyMicrogridDashboard } from '../../api'

const props = defineProps<{
  devices: PyMicrogridDevice[]
  dashboard: PyMicrogridDashboard
  running: boolean
}>()

const svgW = computed(() => Math.max(500, props.devices.length * 110 + 60))
const svgH = 220
const cx = computed(() => svgW.value / 2)
const busY = 80
const busLeft = computed(() => cx.value - (props.devices.length * 55))
const busRight = computed(() => cx.value + (props.devices.length * 55))
const cardY = 110

function devX(idx: number) {
  const total = props.devices.length
  const spacing = (busRight.value - busLeft.value) / (total + 1)
  return busLeft.value + spacing * (idx + 1)
}

function devColor(type: string) {
  const map: Record<string, string> = { PV: '#eab308', BESS: '#10b981', EV: '#3b82f6', Load: '#f97316', Meter: '#6366f1' }
  return map[type] || '#64748b'
}
function devBg(type: string) {
  const map: Record<string, string> = { PV: '#fefce8', BESS: '#ecfdf5', EV: '#eff6ff', Load: '#fff7ed', Meter: '#eef2ff' }
  return map[type] || '#f8fafc'
}
function devIcon(type: string) {
  const map: Record<string, string> = { PV: '☀️', BESS: '🔋', EV: '🔌', Load: '💡', Meter: '⚡' }
  return map[type] || '❓'
}
function devLabel(type: string) {
  const map: Record<string, string> = { PV: '光伏', BESS: '储能', EV: '充电桩', Load: '负荷', Meter: '电表' }
  return map[type] || type
}
function cardH(dev: PyMicrogridDevice) { return dev.type === 'BESS' ? 84 : 64 }

function fmtPower(v: number) { return (v >= 0 ? '+' : '') + v.toFixed(1) + ' kW' }

function fmtDevPower(dev: PyMicrogridDevice) {
  const lists: Record<string, any[]> = { PV: props.dashboard.pv || [], BESS: props.dashboard.battery || [], EV: props.dashboard.charger || [], Load: props.dashboard.load || [] }
  const list = lists[dev.type]
  if (!list) {
    if (dev.type === 'Meter') return fmtPower(props.dashboard.grid_power_kw)
    return '—'
  }
  const found = list.find((d: any) => d.id === dev.id)
  return found ? found.power_kw.toFixed(1) + ' kW' : '—'
}

function getSOC(dev: PyMicrogridDevice) {
  const bat = (props.dashboard.battery || []).find((b: any) => b.id === dev.id)
  return bat?.soc ?? 0
}
</script>

<style scoped>
.topo-svg-wrap { overflow-x: auto; }
</style>
