<template>
  <el-card shadow="never" class="section-card">
    <template #header>
      <div style="display:flex;justify-content:space-between;align-items:center">
        <span style="font-weight:600">组态图</span>
        <span v-if="running" class="grid-power-badge" :class="gridPowerClass">
          并网 {{ gridPower >= 0 ? '←' : '→' }} {{ Math.abs(gridPower).toFixed(1) }} kW
        </span>
      </div>
    </template>
    <div class="topology-wrap" ref="wrapRef">
      <svg :viewBox="`0 0 ${svgW} ${svgH}`" width="100%" xmlns="http://www.w3.org/2000/svg" class="topology-svg">
        <defs>
          <!-- Flow animation markers -->
          <marker id="arrow-down" viewBox="0 0 6 6" refX="3" refY="3" markerWidth="4" markerHeight="4" orient="auto">
            <path d="M0,0 L6,3 L0,6 Z" fill="#e6a23c"/>
          </marker>
          <marker id="arrow-up" viewBox="0 0 6 6" refX="3" refY="3" markerWidth="4" markerHeight="4" orient="auto">
            <path d="M0,0 L6,3 L0,6 Z" fill="#67c23a"/>
          </marker>
          <!-- Gradients -->
          <linearGradient id="grad-grid" x1="0%" y1="0%" x2="0%" y2="100%">
            <stop offset="0%" stop-color="#fee2e2"/><stop offset="100%" stop-color="#fecaca"/>
          </linearGradient>
          <linearGradient id="grad-meter" x1="0%" y1="0%" x2="0%" y2="100%">
            <stop offset="0%" stop-color="#fef9c3"/><stop offset="100%" stop-color="#fde68a"/>
          </linearGradient>
          <linearGradient id="grad-bus" x1="0%" y1="0%" x2="100%" y2="0%">
            <stop offset="0%" stop-color="#64748b"/><stop offset="50%" stop-color="#334155"/><stop offset="100%" stop-color="#64748b"/>
          </linearGradient>
        </defs>

        <!-- Background -->
        <rect width="100%" height="100%" fill="#f8fafc" rx="8"/>

        <!-- ═══ Grid (电网) ═══ -->
        <g class="grid-node">
          <rect :x="cx - 60" y="16" width="120" height="44" rx="8" fill="url(#grad-grid)" stroke="#f87171" stroke-width="1.5"/>
          <text :x="cx" y="34" text-anchor="middle" font-size="11" fill="#991b1b" font-weight="600">⚡ 电网</text>
          <text :x="cx" y="50" text-anchor="middle" font-size="10" fill="#dc2626" font-weight="700" font-family="monospace">
            {{ running ? (gridPower >= 0 ? '+' : '') + gridPower.toFixed(1) + ' kW' : '—' }}
          </text>
        </g>

        <!-- Grid → Meter connection -->
        <line :x1="cx" y1="60" :x2="cx" y2="82" stroke="#94a3b8" stroke-width="3" stroke-linecap="round"
          :class="running ? gridFlowClass : ''"/>

        <!-- ═══ 关口表 (Meter) ═══ -->
        <g class="meter-node">
          <rect :x="cx - 60" y="82" width="120" height="40" rx="8" fill="url(#grad-meter)" stroke="#d97706" stroke-width="1.5"/>
          <text :x="cx" y="100" text-anchor="middle" font-size="11" fill="#92400e" font-weight="600">关口表</text>
          <text :x="cx" y="114" text-anchor="middle" font-size="9" fill="#a16207">{{ gridMeterCapacity }} kW</text>
        </g>

        <!-- Meter → Bus connection -->
        <line :x1="cx" y1="122" :x2="cx" :y2="BUS_Y - 8" stroke="#94a3b8" stroke-width="3" stroke-linecap="round"
          :class="running ? gridFlowClass : ''"/>

        <!-- ═══ Bus bar (母线) ═══ -->
        <rect :x="busLeft - 4" :y="BUS_Y - 6" :width="busRight - busLeft + 8" height="12" rx="3" fill="url(#grad-bus)" opacity="0.9"/>
        <text :x="busLeft - 8" :y="BUS_Y - 12" font-size="10" fill="#475569" font-weight="600" text-anchor="start">
          {{ busName }}
        </text>
        <text :x="busRight + 8" :y="BUS_Y - 12" font-size="9" fill="#94a3b8" text-anchor="end">
          {{ busVoltage }} kV
        </text>

        <!-- ═══ Devices ═══ -->
        <g v-for="(dev, idx) in devices" :key="dev.id" class="device-group">
          <!-- Branch line: bus → switch -->
          <line :x1="devX(idx)" :y1="BUS_Y + 6" :x2="devX(idx)" :y2="swY - SW_R"
            :stroke="devColor(dev)" stroke-width="2.5" stroke-linecap="round"
            :class="running ? devFlowClass(dev) : ''"/>

          <!-- Switch (breaker) -->
          <g :transform="`translate(${devX(idx)}, ${swY})`" style="cursor:pointer"
            :data-dev-id="dev.id" data-action="toggle-switch">
            <circle :r="SW_R" :fill="dev.switch.closed ? '#dcfce7' : '#fee2e2'"
              :stroke="dev.switch.closed ? '#16a34a' : '#dc2626'" stroke-width="2"/>
            <line v-if="dev.switch.closed" x1="-6" y1="0" x2="6" y2="0" stroke="#16a34a" stroke-width="2.5" stroke-linecap="round"/>
            <g v-else>
              <line x1="-5" y1="-5" x2="5" y2="5" stroke="#dc2626" stroke-width="2" stroke-linecap="round"/>
              <line x1="-5" y1="5" x2="5" y2="-5" stroke="#dc2626" stroke-width="2" stroke-linecap="round"/>
            </g>
          </g>
          <text :x="devX(idx)" :y="swY + SW_R + 11" text-anchor="middle" font-size="8" fill="#94a3b8">
            {{ dev.switch.name || 'QF' + (idx + 1) }}
          </text>

          <!-- Branch line: switch → device box -->
          <line :x1="devX(idx)" :y1="swY + SW_R" :x2="devX(idx)" :y2="boxTop"
            :stroke="dev.switch.closed ? devColor(dev) : '#d1d5db'" stroke-width="2.5" stroke-linecap="round"
            :class="running && dev.switch.closed ? devFlowClass(dev) : ''"/>

          <!-- Device card -->
          <g :transform="`translate(${devX(idx) - CARD_W/2}, ${boxTop})`">
            <!-- Card background -->
            <rect width="100" :height="cardHeight(dev)" rx="8"
              :fill="dev.switch.closed ? devBgColor(dev) : '#f1f5f9'"
              :stroke="dev.switch.closed ? devColor(dev) : '#d1d5db'" stroke-width="1.5"
              :opacity="dev.switch.closed ? 1 : 0.6"/>

            <!-- Device icon -->
            <text x="50" y="18" text-anchor="middle" font-size="14">{{ devIcon(dev.type) }}</text>

            <!-- Device type + name -->
            <text x="50" y="32" text-anchor="middle" font-size="10" font-weight="700"
              :fill="dev.switch.closed ? '#1e293b' : '#94a3b8'">{{ devTypeLabel(dev.type) }}</text>
            <text x="50" y="44" text-anchor="middle" font-size="9"
              :fill="dev.switch.closed ? '#475569' : '#94a3b8'">{{ dev.name }}</text>

            <!-- Power value -->
            <rect x="10" y="50" width="80" height="18" rx="4"
              :fill="dev.switch.closed ? devColor(dev) : '#e2e8f0'" opacity="0.15"/>
            <text x="50" y="63" text-anchor="middle" font-size="11" font-weight="700" font-family="monospace"
              :fill="dev.switch.closed ? devColor(dev) : '#94a3b8'">
              {{ dev.switch.closed ? devPower(dev) + ' kW' : '已断开' }}
            </text>

            <!-- Control mode badge -->
            <rect v-if="dev.switch.closed" x="10" y="72" width="80" height="14" rx="3"
              :fill="dev.control_mode === 'local' ? '#fef3c7' : '#dbeafe'"/>
            <text v-if="dev.switch.closed" x="50" y="83" text-anchor="middle" font-size="8" font-weight="600"
              :fill="dev.control_mode === 'local' ? '#92400e' : '#1d4ed8'">
              {{ dev.control_mode === 'local' ? '本地(策略)' : '远方(AO)' }}
            </text>

            <!-- SOC bar (battery only) -->
            <g v-if="dev.type === 'battery' && dev.switch.closed">
              <rect x="10" y="90" width="80" height="8" rx="2" fill="#e2e8f0"/>
              <rect x="10" y="90" :width="Math.max(0, Math.min(80, 80 * (devSOC(dev) / 100)))" height="8" rx="2"
                :fill="devSOC(dev) > 20 ? '#3b82f6' : '#ef4444'"/>
              <text x="50" y="106" text-anchor="middle" font-size="8" fill="#64748b" font-weight="600">
                SOC {{ devSOC(dev).toFixed(0) }}%
              </text>
            </g>
          </g>
        </g>

        <!-- Empty state -->
        <text v-if="devices.length === 0" :x="cx" y="260" text-anchor="middle" font-size="13" fill="#94a3b8">
          暂无设备，点击左侧「添加设备」开始配置
        </text>
      </svg>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { MicrogridDevice, MicrogridDashboard } from '../../api'

const props = defineProps<{
  devices: MicrogridDevice[]
  dash: MicrogridDashboard
  running: boolean
  busName: string
  busVoltage: number
  gridMeterCapacity?: number
}>()

// ─── Layout constants ───
const CARD_W = 100
const SW_R = 9
const BUS_Y = 160
const swY = BUS_Y + 36
const boxTop = BUS_Y + 70

const COLORS: Record<string, string> = {
  pv: '#16a34a', battery: '#2563eb', load: '#d97706', charger: '#6b7280',
}
const BG_COLORS: Record<string, string> = {
  pv: '#f0fdf4', battery: '#eff6ff', load: '#fffbeb', charger: '#f9fafb',
}
const ICONS: Record<string, string> = {
  pv: '☀️', battery: '🔋', load: '💡', charger: '🔌',
}
const LABELS: Record<string, string> = {
  pv: '光伏', battery: '储能', load: '负荷', charger: '充电桩',
}

// ─── Computed layout ───
const N = computed(() => props.devices.length)
const svgW = computed(() => Math.max(400, N.value * 130 + 80))
const svgH = computed(() => {
  const hasBat = props.devices.some(d => d.type === 'battery')
  return hasBat ? 380 : 360
})
const cx = computed(() => svgW.value / 2)

const spacing = computed(() => {
  if (N.value <= 1) return 0
  const available = svgW.value - 80
  return Math.min(130, available / N.value)
})

const startX = computed(() => {
  if (N.value === 0) return cx.value
  return cx.value - (spacing.value * (N.value - 1)) / 2
})

const busLeft = computed(() => N.value > 0 ? startX.value - 30 : cx.value - 60)
const busRight = computed(() => N.value > 0 ? startX.value + (N.value - 1) * spacing.value + 30 : cx.value + 60)

function devX(idx: number): number {
  return startX.value + idx * spacing.value
}

// ─── Power data ───
const powerMap = computed(() => {
  const map = new Map<string, number>()
  for (const arr of [props.dash.pv, props.dash.battery, props.dash.load, props.dash.charger]) {
    if (!arr) continue
    for (const d of arr) map.set(d.id, d.power_kw ?? 0)
  }
  return map
})

const socMap = computed(() => {
  const map = new Map<string, number>()
  if (props.dash.battery) {
    for (const b of props.dash.battery) map.set(b.id, b.soc ?? 50)
  }
  return map
})

const gridPower = computed(() => props.dash.grid_power_kw ?? 0)
const gridMeterCapacity = computed(() => props.gridMeterCapacity ?? 500)
const gridPowerClass = computed(() => gridPower.value >= 0 ? 'consuming' : 'exporting')
const gridFlowClass = computed(() => gridPower.value > 0.1 ? 'fl-dn' : (gridPower.value < -0.1 ? 'fl-up' : 'fl-idle'))

// ─── Device helpers ───
function devColor(dev: MicrogridDevice): string {
  return dev.switch.closed ? (COLORS[dev.type] || '#6b7280') : '#d1d5db'
}
function devBgColor(dev: MicrogridDevice): string {
  return BG_COLORS[dev.type] || '#f9fafb'
}
function devIcon(type: string): string {
  return ICONS[type] || '⚙️'
}
function devTypeLabel(type: string): string {
  return LABELS[type] || type
}
function devPower(dev: MicrogridDevice): string {
  return (powerMap.value.get(dev.id) ?? 0).toFixed(1)
}
function devSOC(dev: MicrogridDevice): number {
  return socMap.value.get(dev.id) ?? 50
}
function cardHeight(dev: MicrogridDevice): number {
  if (dev.type === 'battery' && dev.switch.closed) return 112
  return 90
}

function devFlowClass(dev: MicrogridDevice): string {
  if (!dev.switch.closed) return 'fl-idle'
  const p = powerMap.value.get(dev.id) ?? 0
  if (dev.type === 'pv') return p > 0.1 ? 'fl-up' : 'fl-idle'
  if (dev.type === 'battery') return p > 0.1 ? 'fl-dn' : (p < -0.1 ? 'fl-up' : 'fl-idle')
  // load/charger: consuming → fl-dn
  return p > 0.1 ? 'fl-dn' : 'fl-idle'
}
</script>

<style scoped>
.section-card { margin-bottom: 0; }

.grid-power-badge {
  font-size: 11px;
  font-weight: 700;
  font-family: monospace;
  padding: 2px 10px;
  border-radius: 12px;
}
.grid-power-badge.consuming {
  background: #fef3c7;
  color: #d97706;
}
.grid-power-badge.exporting {
  background: #dcfce7;
  color: #16a34a;
}

.topology-wrap {
  overflow-x: auto;
  overflow-y: hidden;
  background: #f8fafc;
  border: 1px solid var(--el-border-color-light, #e2e8f0);
  border-radius: 8px;
  padding: 4px;
}

.topology-svg {
  display: block;
  min-height: 360px;
  font-family: system-ui, -apple-system, 'Segoe UI', sans-serif;
}

/* Flow animations */
.topology-svg :deep(.fl-up) {
  stroke-dasharray: 8 4;
  animation: flow-up 0.8s linear infinite;
}
.topology-svg :deep(.fl-dn) {
  stroke-dasharray: 8 4;
  animation: flow-dn 0.8s linear infinite;
}
.topology-svg :deep(.fl-idle) {
  stroke-dasharray: 3 6;
  opacity: 0.5;
}

@keyframes flow-up {
  to { stroke-dashoffset: 24; }
}
@keyframes flow-dn {
  to { stroke-dashoffset: -24; }
}

/* Device hover */
.topology-svg .device-group {
  transition: opacity 0.2s;
}
.topology-svg .device-group:hover {
  opacity: 0.85;
}
</style>
