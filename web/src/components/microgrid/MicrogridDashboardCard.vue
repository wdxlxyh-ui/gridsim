<template>
  <el-card v-if="running" shadow="never" class="section-card">
    <template #header><span style="font-weight:600">实时运行数据</span></template>
    <div class="dashboard-grid">
      <div class="dash-item">
        <div class="dash-label">并网点功率</div>
        <div class="dash-value" :style="{ color: (dash.grid_power_kw ?? 0) >= 0 ? '#e6a23c' : '#67c23a' }">{{ dash.grid_power_kw ?? '-' }} kW</div>
      </div>
      <div class="dash-item">
        <div class="dash-label">光伏总功率</div>
        <div class="dash-value" style="color:#67c23a">{{ dash.total_pv_kw ?? '-' }} kW</div>
      </div>
      <div class="dash-item">
        <div class="dash-label">负荷总功率</div>
        <div class="dash-value" style="color:#e6a23c">{{ (dash.total_load_kw ?? 0) + (dash.total_charger_kw ?? 0) }} kW</div>
      </div>
    </div>
    <div style="font-size:11px;margin-top:6px;color:#909399;line-height:1.7;display:flex;flex-wrap:wrap;gap:4px 16px">
      <template v-for="p in (dash.pv || [])" :key="p.id">
        <span>☀️ {{ p.name }}: <strong :style="{color: p.closed ? '#67c23a' : '#c0c4cc'}">{{ p.closed ? p.power_kw+' kW' : '已断开' }}</strong></span>
      </template>
      <template v-for="b in (dash.battery || [])" :key="b.id">
        <span>🔋 {{ b.name }}: <strong :style="{color:'#409eff'}">{{ b.power_kw }} kW</strong> (SOC {{ b.soc }}%)</span>
      </template>
      <template v-for="l in (dash.load || [])" :key="l.id">
        <span>💡 {{ l.name }}: <strong :style="{color: l.closed ? '#e6a23c' : '#c0c4cc'}">{{ l.closed ? l.power_kw+' kW' : '已断开' }}</strong></span>
      </template>
      <template v-for="c in (dash.charger || [])" :key="c.id">
        <span>🔌 {{ c.name }}: <strong :style="{color: c.closed ? '#909399' : '#c0c4cc'}">{{ c.closed ? c.power_kw+' kW' : '已断开' }}</strong></span>
      </template>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import type { MicrogridDashboard } from '../../api'

defineProps<{
  running: boolean
  dash: MicrogridDashboard
}>()
</script>

<style scoped>
.section-card { margin-bottom: 0; }
.dashboard-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 10px; }
.dash-item { text-align: center; padding: 10px; background: var(--el-fill-color); border-radius: 8px; }
.dash-label { font-size: 12px; color: var(--el-text-color-secondary); margin-bottom: 2px; }
.dash-value { font-size: 18px; font-weight: 700; }
</style>
