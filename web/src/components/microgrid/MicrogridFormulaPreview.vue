<template>
  <el-card v-if="devices.length > 0" shadow="never" class="section-card">
    <template #header><span style="font-weight:600">公式预览（自动生成）</span></template>
    <div class="formula-preview">
      <div v-for="f in autoFormulas" :key="f.label" class="formula-row">
        <span class="formula-label">{{ f.label }}</span>
        <code class="formula-expr">{{ f.expr }}</code>
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { MicrogridDevice } from '../../api'

const props = defineProps<{
  devices: MicrogridDevice[]
}>()

const autoFormulas = computed(() => {
  const result: { label: string; expr: string }[] = []
  const active = (d: any) => d.switch.closed
  const pvs = props.devices.filter(d => d.type === 'pv')
  const bats = props.devices.filter(d => d.type === 'battery')
  const loads = props.devices.filter(d => d.type === 'load')
  const chargers = props.devices.filter(d => d.type === 'charger')
  const mkRef = (dev: any) => `{${dev.id}_Power}`
  const plus = (arr: string[]) => arr.join(' + ') || '0'
  const activeRef = (arr: any[]) => plus(arr.filter(active).map(mkRef))
  const activeName = (dev: any) => `${dev.name} (${dev.switch.closed ? '合' : '断'})`

  if (pvs.length) {
    const a = pvs.filter(active)
    result.push({ label: '光伏总功率', expr: a.length ? activeRef(pvs) : '0 (全部断开)' })
    for (const d of pvs) result.push({ label: `  ${activeName(d)}`, expr: active(d) ? `${mkRef(d)} = SETPOINT ∈ [0, ${d.params.rated_power_kw || '?'}]` : '0 (断路)' })
  }
  if (bats.length) {
    const a = bats.filter(active)
    result.push({ label: '储能总功率', expr: a.length ? activeRef(bats) : '0 (全部断开)' })
    for (const b of bats) {
      if (active(b)) {
        result.push({ label: `  ${activeName(b)}`, expr: `${mkRef(b)} = SETPOINT (±${b.params.rated_power_kw_b || '?'} kW, +充电 −放电)` })
      } else { result.push({ label: `  ${activeName(b)}`, expr: '0 (断路)' }) }
    }
  }
  if (loads.length) {
    const a = loads.filter(active)
    result.push({ label: '负荷总功率', expr: a.length ? activeRef(loads) : '0 (全部断开)' })
    for (const d of loads) { if (!active(d)) result.push({ label: `  ${activeName(d)}`, expr: '0 (断路)' }) }
  }
  if (chargers.length) {
    const a = chargers.filter(active)
    result.push({ label: '充电桩总功率', expr: a.length ? activeRef(chargers) : '0 (全部断开)' })
    for (const d of chargers) { if (!active(d)) result.push({ label: `  ${activeName(d)}`, expr: '0 (断路)' }) }
  }

  const genExpr = [...pvs.filter(active)].map(mkRef).join(' + ') || '0'
  const loadExpr = [...loads.filter(active), ...chargers.filter(active), ...bats.filter(active)].map(mkRef).join(' + ') || '0'
  result.push({ label: '关口表功率 (GRID_P)', expr: `(${loadExpr}) − (${genExpr})` })

  return result
})
</script>

<style scoped>
.section-card { margin-bottom: 0; }
.formula-preview { display: flex; flex-direction: column; gap: 6px; }
.formula-row { display: flex; align-items: center; gap: 10px; font-size: 12px; }
.formula-label { min-width: 110px; color: var(--el-text-color-secondary); }
.formula-expr { background: var(--el-fill-color); padding: 3px 8px; border-radius: 4px; font-size: 12px; font-family: 'SF Mono', 'Menlo', monospace; color: var(--el-color-primary); }
</style>
