<template>
  <el-card shadow="never">
    <template #header><span style="font-weight:600">功率汇总</span></template>
    <div class="dash-grid" v-if="running">
      <div class="dash-item">
        <span class="dash-label">☀️ 光伏</span>
        <span class="dash-value pv">{{ dashboard.total_pv_kw.toFixed(1) }} kW</span>
      </div>
      <div class="dash-item">
        <span class="dash-label">🔋 储能</span>
        <span class="dash-value bess">{{ dashboard.total_bat_kw.toFixed(1) }} kW</span>
      </div>
      <div class="dash-item">
        <span class="dash-label">🔌 充电桩</span>
        <span class="dash-value ev">{{ dashboard.total_charger_kw.toFixed(1) }} kW</span>
      </div>
      <div class="dash-item">
        <span class="dash-label">💡 负荷</span>
        <span class="dash-value load">{{ dashboard.total_load_kw.toFixed(1) }} kW</span>
      </div>
      <div class="dash-item full-width">
        <span class="dash-label">⚡ 并网</span>
        <span class="dash-value grid" :style="{color: dashboard.grid_power_kw >= 0 ? '#e6a23c' : '#67c23a'}">
          {{ dashboard.grid_power_kw >= 0 ? '买电' : '卖电' }} {{ Math.abs(dashboard.grid_power_kw).toFixed(1) }} kW
        </span>
      </div>
      <div class="dash-item full-width" v-if="dashboard.battery_soc > 0">
        <span class="dash-label">SOC</span>
        <el-progress :percentage="dashboard.battery_soc" :stroke-width="12" :color="socColor" style="flex:1;margin-left:8px"/>
      </div>
    </div>
    <div v-else style="color:#94a3b8;text-align:center;padding:20px;">实例未运行</div>
  </el-card>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { PyMicrogridDashboard } from '../../api'

const props = defineProps<{ dashboard: PyMicrogridDashboard; running: boolean }>()

const socColor = computed(() => {
  if (props.dashboard.battery_soc > 60) return '#67c23a'
  if (props.dashboard.battery_soc > 20) return '#e6a23c'
  return '#f56c6c'
})
</script>

<style scoped>
.dash-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
.dash-item { display: flex; justify-content: space-between; align-items: center; padding: 6px 8px; background: #f9fafb; border-radius: 6px; }
.dash-item.full-width { grid-column: 1 / -1; }
.dash-label { font-size: 12px; color: #6b7280; }
.dash-value { font-size: 13px; font-weight: 700; font-family: monospace; }
.dash-value.pv { color: #eab308; }
.dash-value.bess { color: #10b981; }
.dash-value.ev { color: #3b82f6; }
.dash-value.load { color: #f97316; }
</style>
