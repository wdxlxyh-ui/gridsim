<template>
  <el-card shadow="never" class="section-card">
    <template #header><span style="font-weight:600">拓扑图</span></template>
    <div class="topology-wrap">
      <div v-html="svgTopology" :key="'svg-' + deviceCount + '-' + running" class="topology-html"></div>
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
}>()

const deviceCount = computed(() => props.devices.length)

const LB: Record<string, string> = { pv: '光伏', battery: '储能', load: '负荷', charger: '充电桩' }
const FC: Record<string, string> = { pv: '#67c23a', battery: '#409eff', load: '#e6a23c', charger: '#909399' }

const svgTopology = computed(() => {
  const N = props.devices.length
  if (N === 0) return ''
  const svgW = 680, svgH = Math.max(450, 220 + N * 95 + 30)
  const BUS_Y = 220, MIN_GAP = 120
  const cx = svgW / 2
  const sp = (svgW - 80) / Math.max(N, 1)
  const sx = cx - (sp * (N - 1)) / 2
  const minX = sx - 20
  const maxX = sx + (N - 1) * sp + 20
  const swY = BUS_Y + 38, swR = 10, boxT = BUS_Y + 65

  const powerMap = new Map<string, number>()
  for (const arr of [props.dash.pv, props.dash.battery, props.dash.load, props.dash.charger]) {
    if (!arr) continue
    for (const d of arr) powerMap.set(d.id, d.power_kw ?? 0)
  }

  function flowClass(dev: any): string {
    if (!dev.switch.closed) return 'fz'
    const p = powerMap.get(dev.id) ?? 0
    if (dev.type === 'pv') return p > 0.1 ? 'fl-up' : 'fz'
    if (dev.type === 'battery') return p > 0.1 ? 'fl-dn' : (p < -0.1 ? 'fl-up' : 'fz')
    return p > 0.1 ? 'fl-dn' : 'fz'
  }

  let rows = ''
  props.devices.forEach((dev: any, idx: number) => {
    const dx = sx + idx * sp
    const cl = dev.switch.closed
    const t = dev.type
    const fc = flowClass(dev)
    const lc = cl ? (FC as any)[t] : '#c0c4cc'
    const pval = (powerMap.get(dev.id) ?? 0).toFixed(1)

    rows += `<line x1="${dx}" y1="${BUS_Y}" x2="${dx}" y2="${swY - swR}" stroke="${cl ? lc : '#c0c4cc'}" stroke-width="3.5" stroke-linecap="round" class="${fc}"/>`
    const sf = cl ? (t === 'pv' ? '#e8f5e9' : t === 'battery' ? '#e3f2fd' : '#fff3e0') : '#fef0f0'
    rows += `<circle cx="${dx}" cy="${swY}" r="${swR}" fill="${sf}" stroke="${cl ? '#67c23a' : '#f56c6c'}" stroke-width="2" style="cursor:pointer" data-dev-id="${dev.id}" data-action="toggle-switch"/>`
    rows += cl
      ? `<line x1="${dx - 7}" y1="${swY}" x2="${dx + 7}" y2="${swY}" stroke="#67c23a" stroke-width="2" stroke-linecap="round"/>`
      : `<line x1="${dx - 6}" y1="${swY - 6}" x2="${dx + 6}" y2="${swY + 6}" stroke="#f56c6c" stroke-width="2" stroke-linecap="round"/>`
    rows += `<line x1="${dx}" y1="${swY + swR}" x2="${dx}" y2="${boxT}" stroke="${cl ? lc : '#c0c4cc'}" stroke-width="3.5" stroke-linecap="round" class="${fc}"/>`
    rows += `<text x="${dx}" y="${swY + swR + 13}" text-anchor="middle" font-size="9" fill="#909399">${dev.switch.name || 'QF' + (idx + 1)}</text>`
    rows += `<rect x="${dx - 46}" y="${boxT}" width="92" height="34" rx="6" fill="${cl ? (FC as any)[t] : '#e0e0e0'}" stroke="${lc}" stroke-width="1.5" opacity="${cl ? 1 : 0.5}"/>`
    rows += `<text x="${dx}" y="${boxT + 12}" text-anchor="middle" font-size="12" font-weight="700" fill="${cl ? '#fff' : '#999'}">${(LB as any)[t]}</text>`
    rows += `<text x="${dx}" y="${boxT + 26}" text-anchor="middle" font-size="10" fill="${cl ? 'rgba(255,255,255,0.9)' : '#999'}">${dev.name}</text>`
    if (cl) {
      rows += `<rect x="${dx - 40}" y="${boxT + 38}" width="80" height="18" rx="4" fill="${lc}" opacity="0.1"/>`
      rows += `<text x="${dx}" y="${boxT + 50}" text-anchor="middle" font-size="11" font-weight="700" style="font-family:monospace" fill="${lc}">${pval} kW</text>`
    } else {
      rows += `<text x="${dx}" y="${boxT + 44}" text-anchor="middle" font-size="10" fill="#c0c4cc">已断开</text>`
    }
  })

  const PV = (props.dash.pv || []).filter((d: any) => d.closed).reduce((s: number, d: any) => s + (d.power_kw ?? 0), 0)
  const LD = (props.dash.load || []).filter((d: any) => d.closed).reduce((s: number, d: any) => s + (d.power_kw ?? 0), 0)
  const CH = (props.dash.charger || []).filter((d: any) => d.closed).reduce((s: number, d: any) => s + (d.power_kw ?? 0), 0)
  const BAT = (props.dash.battery || []).filter((d: any) => d.closed).reduce((s: number, d: any) => s + (d.power_kw ?? 0), 0)
  const GRID = LD + CH + BAT - PV
  const tFlow = GRID > 0.1 ? 'fl-dn' : (GRID < -0.1 ? 'fl-up' : 'fz')

  return `<svg viewBox="0 0 ${svgW} ${svgH}" width="100%" xmlns="http://www.w3.org/2000/svg">
<rect x="0" y="0" width="${svgW}" height="${svgH}" fill="#f5f7fa"/>
<rect x="${cx - 56}" y="12" width="112" height="38" rx="6" fill="#fef0f0" stroke="#f89898" stroke-width="1.5"/>
<text x="${cx}" y="36" text-anchor="middle" font-size="14" font-weight="700" fill="#e63946">⚡ 电网</text>
<line x1="${cx}" y1="50" x2="${cx}" y2="78" stroke="#bbb" stroke-width="3.5" stroke-linecap="round" class="${tFlow}"/>
<rect x="${cx - 56}" y="80" width="112" height="38" rx="6" fill="#fef7e0" stroke="#e8c560" stroke-width="1.5"/>
<text x="${cx}" y="104" text-anchor="middle" font-size="14" font-weight="700" fill="#b8860b">关口表</text>
<line x1="${cx}" y1="118" x2="${cx}" y2="${BUS_Y}" stroke="#bbb" stroke-width="3.5" stroke-linecap="round" class="${tFlow}"/>
<text x="${cx + 20}" y="${BUS_Y - 45}" font-size="13" font-weight="700" fill="#303133">${props.busName}</text>
<text x="${cx + 20}" y="${BUS_Y - 30}" font-size="11" fill="#909399">${props.busVoltage} kV</text>
<line x1="${minX}" y1="${BUS_Y}" x2="${maxX}" y2="${BUS_Y}" stroke="#555" stroke-width="3" stroke-linecap="round"/>
${rows}
</svg>`
})
</script>

<style scoped>
.section-card { margin-bottom: 0; }
.topology-wrap { overflow: auto; background: var(--el-fill-color); border: 1px solid var(--el-border-color-light); border-radius: 6px; }
.topology-html { display: block; font-family: system-ui, -apple-system, sans-serif; background: var(--el-fill-color); }
</style>
