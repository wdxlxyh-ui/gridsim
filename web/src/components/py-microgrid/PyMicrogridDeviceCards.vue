<template>
  <el-card shadow="never">
    <template #header><span style="font-weight:600">设备控制</span></template>
    <div class="device-list">
      <div v-for="dev in devices" :key="dev.id" class="dev-card" :style="{ borderLeftColor: devColor(dev.type) }">
        <div class="dev-header">
          <span>{{ devIcon(dev.type) }} {{ dev.name }}</span>
          <el-tag size="small" :type="devTagType(dev.type)">{{ devLabel(dev.type) }}</el-tag>
        </div>
        <div class="dev-body" v-if="running">
          <div class="dev-power">{{ fmtDevPower(dev) }}</div>
          <!-- Control: AO points for PV LimitPower and BESS SetPoint -->
          <div v-if="dev.type === 'PV'" class="dev-control">
            <span style="font-size:11px;color:#6b7280">限功率:</span>
            <el-input-number v-model="controlValues[dev.id + '_limit']" size="small" :min="0" :max="9999" :step="10" style="width:100px" />
            <el-button size="small" type="primary" @click="sendControl(dev, 'limit')">设定</el-button>
          </div>
          <div v-if="dev.type === 'BESS'" class="dev-control">
            <span style="font-size:11px;color:#6b7280">功率指令:</span>
            <el-input-number v-model="controlValues[dev.id + '_power']" size="small" :min="-9999" :max="9999" :step="100" style="width:110px" />
            <el-button size="small" type="primary" @click="sendControl(dev, 'power')">设定</el-button>
          </div>
          <div v-if="dev.type === 'EV'" class="dev-control">
            <span style="font-size:11px;color:#6b7280">充电功率:</span>
            <el-input-number v-model="controlValues[dev.id + '_charge']" size="small" :min="0" :max="999" :step="1" style="width:100px" />
            <el-button size="small" type="primary" @click="sendControl(dev, 'charge')">设定</el-button>
          </div>
        </div>
        <div v-else class="dev-body" style="color:#94a3b8;font-size:12px;">未运行</div>
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { reactive } from 'vue'
import type { PyMicrogridDevice, PyMicrogridDashboard } from '../../api'

const props = defineProps<{
  devices: PyMicrogridDevice[]
  dashboard: PyMicrogridDashboard
  running: boolean
}>()

const emit = defineEmits<{ (e: 'control', ioa: number, value: number): void }>()

const controlValues = reactive<Record<string, number>>({})

function devColor(type: string) {
  const map: Record<string, string> = { PV: '#eab308', BESS: '#10b981', EV: '#3b82f6', Load: '#f97316', Meter: '#6366f1' }
  return map[type] || '#64748b'
}
function devIcon(type: string) {
  const map: Record<string, string> = { PV: '☀️', BESS: '🔋', EV: '🔌', Load: '💡', Meter: '⚡' }
  return map[type] || '❓'
}
function devLabel(type: string) {
  const map: Record<string, string> = { PV: '光伏', BESS: '储能', EV: '充电桩', Load: '负荷', Meter: '电表' }
  return map[type] || type
}
function devTagType(type: string): '' | 'success' | 'warning' | 'info' | 'danger' {
  const map: Record<string, '' | 'success' | 'warning' | 'info' | 'danger'> = { PV: 'warning', BESS: 'success', EV: '', Load: 'danger', Meter: 'info' }
  return map[type] || 'info'
}

function fmtDevPower(dev: PyMicrogridDevice) {
  const lists: Record<string, any[]> = { PV: props.dashboard.pv || [], BESS: props.dashboard.battery || [], EV: props.dashboard.charger || [], Load: props.dashboard.load || [] }
  const list = lists[dev.type]
  if (!list) {
    if (dev.type === 'Meter') return props.dashboard.grid_power_kw.toFixed(1) + ' kW'
    return '—'
  }
  const found = list.find((d: any) => d.id === dev.id)
  return found ? found.power_kw.toFixed(1) + ' kW' : '0.0 kW'
}

function sendControl(dev: PyMicrogridDevice, ctrlType: string) {
  // Calculate IOA based on device type and control type
  // PV LimitPower = ioa_base + 2 (INV.LimitPower is 3rd point, index 2)
  // BESS SysAPSetPoint = ioa_base + 7 (BS.SysAPSetPoint is 8th point, index 7)
  // EV ChargePWSet = ioa_base + 4 (PUB_CONN.ChargePWSet is 5th point, index 4)
  let ioaOffset = 0
  let valueKey = ''
  if (ctrlType === 'limit') { ioaOffset = 2; valueKey = dev.id + '_limit' }
  else if (ctrlType === 'power') { ioaOffset = 7; valueKey = dev.id + '_power' }
  else if (ctrlType === 'charge') { ioaOffset = 4; valueKey = dev.id + '_charge' }

  const ioa = dev.ioa_base + ioaOffset
  const value = controlValues[valueKey] || 0
  emit('control', ioa, value)
}
</script>

<style scoped>
.device-list { display: flex; flex-direction: column; gap: 8px; max-height: 400px; overflow-y: auto; }
.dev-card { padding: 10px 12px; border-radius: 8px; border: 1px solid #e5e7eb; border-left: 3px solid; }
.dev-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px; font-size: 13px; font-weight: 600; }
.dev-body { font-size: 12px; }
.dev-power { font-size: 14px; font-weight: 700; font-family: monospace; margin-bottom: 6px; }
.dev-control { display: flex; align-items: center; gap: 6px; margin-top: 6px; }
</style>
