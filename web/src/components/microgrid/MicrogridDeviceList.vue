<template>
  <el-card shadow="never" class="section-card">
    <template #header>
      <div style="display:flex;justify-content:space-between;align-items:center">
        <span style="font-weight:600">设备列表 ({{ devices.length }})</span>
        <el-button type="primary" size="small" :disabled="running" @click="$emit('add-device')">
          + 添加设备
        </el-button>
      </div>
    </template>
    <div v-if="devices.length === 0" style="text-align:center;color:var(--el-text-color-secondary);padding:20px">
      暂无设备，点击上方按钮添加
    </div>
    <div v-for="dev in devices" :key="dev.id" class="device-card" :class="{ 'device-conflict': deviceHasConflict(dev) }">
      <div class="device-header">
        <el-tag :type="devTypeTag(dev.type)" size="small">{{ devTypeLabel(dev.type) }}</el-tag>
        <span style="font-weight:500;margin-left:8px">{{ dev.name }}</span>
        <el-tag :type="dev.switch.closed ? 'success' : 'danger'" size="small" style="margin-left:auto">
          {{ dev.switch.closed ? '合闸' : '分闸' }}
        </el-tag>
      </div>
      <div class="device-params" style="display:flex;align-items:center;gap:8px;flex-wrap:wrap">
        <template v-if="dev.type === 'pv'">{{ dev.params.rated_power_kw || '-' }} kW</template>
        <template v-else-if="dev.type === 'battery'">
          {{ dev.params.capacity_kwh || '-' }} kWh / {{ dev.params.rated_power_kw_b || '-' }} kW
        </template>
        <template v-else-if="dev.type === 'load'">{{ dev.params.load_rated_kw || '-' }} kW</template>
        <template v-else-if="dev.type === 'charger'">{{ dev.params.charger_rated_kw || '-' }} kW</template>
        <el-tag v-if="dev.ioa_base" :type="deviceHasConflict(dev) ? 'danger' : 'primary'" size="small" effect="plain" style="margin-left:auto;font-family:monospace;font-size:11px">
          IOA {{ dev.ioa_base }}~{{ dev.ioa_base + 49 }}
        </el-tag>
      </div>
      <div class="device-actions">
        <el-switch v-model="dev.switch.closed" :disabled="!running" size="small" active-text="合" inactive-text="分"
          @change="(val: boolean) => $emit('toggle-switch', dev.id, val)" />
        <el-button text size="small" @click="$emit('edit-device', dev)">参数</el-button>
        <el-button text size="small" type="danger" :disabled="running" @click="$emit('delete-device', dev.id)">删除</el-button>
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import type { MicrogridDevice } from '../../api'

defineProps<{
  devices: MicrogridDevice[]
  running: boolean
}>()

defineEmits<{
  'add-device': []
  'edit-device': [dev: MicrogridDevice]
  'delete-device': [id: string]
  'toggle-switch': [id: string, closed: boolean]
}>()

function devTypeLabel(type: string): string {
  const map: Record<string, string> = { pv: '光伏', battery: '储能', load: '负荷', charger: '充电桩' }
  return map[type] || type
}

function devTypeTag(type: string): 'success' | 'primary' | 'warning' | 'info' {
  const map: Record<string, any> = { pv: 'success', battery: 'primary', load: 'warning', charger: 'info' }
  return map[type] || 'info'
}

function deviceHasConflict(dev: MicrogridDevice): boolean {
  if (!dev.ioa_base || dev.ioa_base < 101) return true
  return false
}
</script>

<style scoped>
.section-card { margin-bottom: 0; }
.device-card {
  border: 1px solid var(--el-border-color);
  border-radius: 8px;
  padding: 10px 12px;
  margin-bottom: 8px;
  background: var(--el-bg-color);
  transition: box-shadow 0.2s;
}
.device-card:hover { box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08); }
.device-card.device-conflict { border-color: #f56c6c; background: #fef0f0; }
.device-header { display: flex; align-items: center; margin-bottom: 4px; }
.device-params { font-size: 12px; color: var(--el-text-color-secondary); margin-bottom: 6px; }
.device-actions { display: flex; align-items: center; gap: 8px; }
</style>
