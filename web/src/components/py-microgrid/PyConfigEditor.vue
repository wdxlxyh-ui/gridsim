<template>
  <div class="py-config-editor">
    <!-- Save button (sticky top bar) -->
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:12px;padding:10px 16px;background:var(--el-bg-color);border-radius:8px;border:1px solid var(--el-border-color);position:sticky;top:0;z-index:10">
      <span style="font-size:13px;color:var(--el-text-color-secondary)">{{ disabled ? '⚠️ 实例运行中，配置为只读' : '📝 编辑设备配置后点击右侧保存' }}</span>
      <el-button type="primary" :disabled="disabled" :loading="saving" @click="handleSave">保存配置</el-button>
    </div>

    <!-- Global config -->
    <el-card shadow="never" style="margin-bottom:12px">
      <template #header><span style="font-weight:600">全局设置</span></template>
      <el-form label-width="100px" :disabled="disabled">
        <el-form-item label="仿真起始时间">
          <el-input v-model="localConfig.start_time" placeholder="HH:MM" style="width:150px" />
          <span style="font-size:12px;color:#6b7280;margin-left:8px">PV/Load曲线从此时间开始</span>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- Device list by type -->
    <el-card shadow="never" v-for="devType in deviceTypes" :key="devType.key" style="margin-bottom:12px">
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <span style="font-weight:600">{{ devType.icon }} {{ devType.label }} ({{ getDevices(devType.key).length }})</span>
          <el-button size="small" type="primary" :disabled="disabled" @click="addDevice(devType.key)">添加{{ devType.label }}</el-button>
        </div>
      </template>
      <div v-if="getDevices(devType.key).length === 0" style="color:#94a3b8;text-align:center;padding:12px">
        暂无{{ devType.label }}设备
      </div>
      <div v-for="(dev, idx) in getDevices(devType.key)" :key="dev.DeviceKey" class="device-form-card">
        <div class="device-form-header">
          <span style="font-weight:600">{{ dev.DeviceKey }}</span>
          <el-button size="small" type="danger" text :disabled="disabled" @click="removeDevice(devType.key, idx)">删除</el-button>
        </div>
        <!-- PV -->
        <el-form v-if="devType.key === 'PV'" label-width="110px" :disabled="disabled" size="small">
          <el-row :gutter="12">
            <el-col :span="8"><el-form-item label="设备标识"><el-input v-model="dev.DeviceKey" /></el-form-item></el-col>
            <el-col :span="8"><el-form-item label="额定功率(kW)"><el-input-number v-model="dev.ratedPower" :min="0" :max="99999" style="width:100%" /></el-form-item></el-col>
            <el-col :span="8"><el-form-item label="Slave ID"><el-input-number v-model="dev.slave_id" :min="1" :max="247" style="width:100%" /></el-form-item></el-col>
          </el-row>
          <el-row :gutter="12">
            <el-col :span="8"><el-form-item label="出力模式"><el-select v-model="dev.mode" style="width:100%"><el-option :value="0" label="CSV曲线回放" /><el-option :value="1" label="正弦曲线" /></el-select></el-form-item></el-col>
            <el-col :span="8" v-if="dev.mode === 0"><el-form-item label="CSV文件"><el-select v-model="dev.csv_file" style="width:100%" allow-create filterable><el-option v-for="f in curveFiles" :key="f" :value="f" :label="f" /></el-select></el-form-item></el-col>
          </el-row>
        </el-form>
        <!-- BESS -->
        <el-form v-if="devType.key === 'BESS'" label-width="120px" :disabled="disabled" size="small">
          <el-row :gutter="12">
            <el-col :span="8"><el-form-item label="设备标识"><el-input v-model="dev.DeviceKey" /></el-form-item></el-col>
            <el-col :span="8"><el-form-item label="额定容量(kWh)"><el-input-number v-model="dev.ratedCapacity" :min="1" style="width:100%" /></el-form-item></el-col>
            <el-col :span="8"><el-form-item label="Slave ID"><el-input-number v-model="dev.slave_id" :min="1" :max="247" style="width:100%" /></el-form-item></el-col>
          </el-row>
          <el-row :gutter="12">
            <el-col :span="8"><el-form-item label="最大充电(kW)"><el-input-number v-model="dev.maxChargePower" :min="0" style="width:100%" /></el-form-item></el-col>
            <el-col :span="8"><el-form-item label="最大放电(kW)"><el-input-number v-model="dev.maxDischargePower" :min="0" style="width:100%" /></el-form-item></el-col>
            <el-col :span="8"><el-form-item label="初始SOC"><el-input-number v-model="dev.initial_soc" :min="0" :max="1" :step="0.1" :precision="2" style="width:100%" /></el-form-item></el-col>
          </el-row>
          <el-row :gutter="12">
            <el-col :span="8"><el-form-item label="SOC上限"><el-input-number v-model="dev.socMax" :min="0" :max="100" style="width:100%" /></el-form-item></el-col>
            <el-col :span="8"><el-form-item label="SOC下限"><el-input-number v-model="dev.socMin" :min="0" :max="100" style="width:100%" /></el-form-item></el-col>
            <el-col :span="8"><el-form-item label="标称电压(V)"><el-input-number v-model="dev.voltage_nominal" :min="0" style="width:100%" /></el-form-item></el-col>
          </el-row>
        </el-form>
        <!-- EV -->
        <el-form v-if="devType.key === 'EV'" label-width="120px" :disabled="disabled" size="small">
          <el-row :gutter="12">
            <el-col :span="8"><el-form-item label="设备标识"><el-input v-model="dev.DeviceKey" /></el-form-item></el-col>
            <el-col :span="8"><el-form-item label="额定功率(kW)"><el-input-number v-model="dev['PUB_CONN.RatedPW']" :min="0" style="width:100%" /></el-form-item></el-col>
            <el-col :span="8"><el-form-item label="Slave ID"><el-input-number v-model="dev.slave_id" :min="1" :max="247" style="width:100%" /></el-form-item></el-col>
          </el-row>
          <el-row :gutter="12">
            <el-col :span="8"><el-form-item label="充电器类型"><el-select v-model="dev.charger_type" style="width:100%"><el-option value="AC" /><el-option value="DC" /></el-select></el-form-item></el-col>
            <el-col :span="8"><el-form-item label="相模式"><el-select v-model="dev.phase_mode" style="width:100%"><el-option :value="1" label="单相" /><el-option :value="3" label="三相" /></el-select></el-form-item></el-col>
            <el-col :span="8"><el-form-item label="电压(V)"><el-input-number v-model="dev.voltage" :min="100" :max="600" style="width:100%" /></el-form-item></el-col>
          </el-row>
          <el-row :gutter="12">
            <el-col :span="8"><el-form-item label="功率因数"><el-input-number v-model="dev.power_factor" :min="0" :max="1" :step="0.01" :precision="2" style="width:100%" /></el-form-item></el-col>
            <el-col :span="8"><el-form-item label="最小功率(kW)"><el-input-number v-model="dev['PUB_CONN.MinChargePW']" :min="0" style="width:100%" /></el-form-item></el-col>
            <el-col :span="8"><el-form-item label="充电系数"><el-input-number v-model="dev.charge_factor" :min="0" :max="2" :step="0.1" :precision="2" style="width:100%" /></el-form-item></el-col>
          </el-row>
          <el-form-item label="充电时段">
            <div v-for="(sch, si) in (dev.charge_schedule || [])" :key="si" style="display:flex;gap:8px;margin-bottom:4px;align-items:center">
              <el-input v-model="sch.start" placeholder="开始 HH:MM" style="width:120px" />
              <span>~</span>
              <el-input v-model="sch.stop" placeholder="结束 HH:MM" style="width:120px" />
              <el-button size="small" text type="danger" @click="dev.charge_schedule.splice(si, 1)">删除</el-button>
            </div>
            <el-button size="small" @click="addSchedule(dev)">添加时段</el-button>
          </el-form-item>
        </el-form>
        <!-- Load -->
        <el-form v-if="devType.key === 'Load'" label-width="110px" :disabled="disabled" size="small">
          <el-row :gutter="12">
            <el-col :span="8"><el-form-item label="设备标识"><el-input v-model="dev.DeviceKey" /></el-form-item></el-col>
            <el-col :span="8"><el-form-item label="基础功率(kW)"><el-input-number v-model="dev.base_power" :min="0" style="width:100%" /></el-form-item></el-col>
            <el-col :span="8"><el-form-item label="Slave ID"><el-input-number v-model="dev.slave_id" :min="1" :max="247" style="width:100%" /></el-form-item></el-col>
          </el-row>
          <el-row :gutter="12">
            <el-col :span="8"><el-form-item label="出力模式"><el-select v-model="dev.mode" style="width:100%"><el-option :value="0" label="CSV曲线回放" /><el-option :value="1" label="随机波动" /></el-select></el-form-item></el-col>
            <el-col :span="8" v-if="dev.mode === 0"><el-form-item label="CSV文件"><el-select v-model="dev.csv_file" style="width:100%" allow-create filterable><el-option v-for="f in curveFiles" :key="f" :value="f" :label="f" /></el-select></el-form-item></el-col>
          </el-row>
        </el-form>
        <!-- Meter -->
        <el-form v-if="devType.key === 'Meter'" label-width="110px" :disabled="disabled" size="small">
          <el-row :gutter="12">
            <el-col :span="8"><el-form-item label="设备标识"><el-input v-model="dev.DeviceKey" /></el-form-item></el-col>
            <el-col :span="8"><el-form-item label="Slave ID"><el-input-number v-model="dev.slave_id" :min="1" :max="247" style="width:100%" /></el-form-item></el-col>
          </el-row>
        </el-form>
      </div>
    </el-card>

    <!-- Upload curve -->
    <el-card shadow="never" style="margin-bottom:12px">
      <template #header><span style="font-weight:600">功率曲线管理</span></template>
      <div style="display:flex;align-items:center;gap:12px;flex-wrap:wrap">
        <span style="font-size:13px;color:#6b7280">已有曲线: {{ curveFiles.length > 0 ? curveFiles.join(', ') : '无' }}</span>
        <el-upload :auto-upload="false" :show-file-list="false" accept=".csv" :on-change="onCurveFile" :disabled="disabled">
          <el-button size="small" type="primary" :disabled="disabled">上传CSV曲线</el-button>
        </el-upload>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'

const props = defineProps<{
  config: any
  disabled: boolean
  saving: boolean
  curveFiles: string[]
}>()

const emit = defineEmits<{
  (e: 'save', config: any): void
  (e: 'upload-curve', file: File): void
}>()

const deviceTypes = [
  { key: 'Meter', label: '电表', icon: '⚡' },
  { key: 'PV', label: '光伏', icon: '☀️' },
  { key: 'BESS', label: '储能', icon: '🔋' },
  { key: 'EV', label: '充电桩', icon: '🔌' },
  { key: 'Load', label: '负荷', icon: '💡' },
]

const localConfig = ref<any>({ start_time: '08:00', Devices: [] })

watch(() => props.config, (val) => {
  if (val && val.Devices && val.Devices.length > 0) {
    localConfig.value = JSON.parse(JSON.stringify(val))
  }
}, { immediate: true, deep: true })

function getDevices(type: string): any[] {
  for (const group of localConfig.value.Devices || []) {
    if (group[type]) return group[type]
  }
  return []
}

function addDevice(type: string) {
  let found = false
  for (const group of localConfig.value.Devices || []) {
    if (group[type]) { group[type].push(createDefault(type)); found = true; break }
  }
  if (!found) {
    if (!localConfig.value.Devices) localConfig.value.Devices = []
    localConfig.value.Devices.push({ [type]: [createDefault(type)] })
  }
}

function removeDevice(type: string, idx: number) {
  for (const group of localConfig.value.Devices || []) {
    if (group[type]) { group[type].splice(idx, 1); break }
  }
}

function createDefault(type: string): any {
  const nextId = getDevices(type).length + 1
  switch (type) {
    case 'Meter': return { DeviceKey: `Meter_${String(nextId).padStart(2,'0')}`, slave_id: nextSlaveId() }
    case 'PV': return { DeviceKey: `PV_${String(nextId).padStart(2,'0')}`, ratedPower: 100, mode: 1, csv_file: '', slave_id: nextSlaveId() }
    case 'BESS': return { DeviceKey: `BESS_${String(nextId).padStart(2,'0')}`, ratedCapacity: 200, maxChargePower: 100, maxDischargePower: 100, socMax: 100, socMin: 0, initial_soc: 0.5, voltage_nominal: 48, resistance: 0.01, slave_id: nextSlaveId() }
    case 'EV': return { DeviceKey: `EV_${String(nextId).padStart(2,'0')}`, 'PUB_CONN.RatedPW': 22, 'PUB_CONN.MinChargePW': 0, charge_factor: 1.0, charge_schedule: [{ start: '08:00', stop: '10:00' }], charger_type: 'AC', power_factor: 0.95, phase_mode: 3, voltage: 220, slave_id: nextSlaveId() }
    case 'Load': return { DeviceKey: `LOAD_${String(nextId).padStart(3,'0')}`, base_power: 50, mode: 0, csv_file: 'load_curve.csv', slave_id: nextSlaveId() }
    default: return {}
  }
}

function nextSlaveId(): number {
  const used = new Set<number>()
  for (const group of localConfig.value.Devices || []) {
    for (const devList of Object.values(group) as any[]) {
      for (const dev of devList) { if (dev.slave_id) used.add(dev.slave_id) }
    }
  }
  for (let i = 1; i <= 247; i++) { if (!used.has(i)) return i }
  return 1
}

function addSchedule(dev: any) {
  if (!dev.charge_schedule) dev.charge_schedule = []
  dev.charge_schedule.push({ start: '08:00', stop: '10:00' })
}

function onCurveFile(uploadFile: any) {
  if (uploadFile?.raw) emit('upload-curve', uploadFile.raw)
}

function handleSave() { emit('save', localConfig.value) }
</script>

<style scoped>
.device-form-card { border: 1px solid #e5e7eb; border-radius: 8px; padding: 12px; margin-bottom: 10px; }
.device-form-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; padding-bottom: 8px; border-bottom: 1px solid #f3f4f6; }
</style>
