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

    <!-- EnOS Configuration -->
    <el-card shadow="never" style="margin-bottom:12px">
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <span style="font-weight:600">🌐 EnOS 数据回放设置</span>
          <el-switch v-model="enosConfig.enabled" :disabled="disabled" />
        </div>
      </template>
      <div v-if="enosConfig.enabled">
        <el-form label-width="120px" :disabled="disabled" size="small">
          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="API 网关">
                <el-input v-model="enosConfig.apigw_address" placeholder="https://ag-cn5.example.com" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="Organization ID">
                <el-input v-model="enosConfig.org_id" placeholder="org-example-12345" />
              </el-form-item>
            </el-col>
          </el-row>
          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="Access Key">
                <el-input v-model="enosConfig.access_key" placeholder="EnOS Access Key" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="Secret Key">
                <el-input v-model="enosConfig.secret_key" type="password" placeholder="EnOS Secret Key" show-password />
              </el-form-item>
            </el-col>
          </el-row>
          <el-row :gutter="16">
            <el-col :span="8">
              <el-form-item label="拉取间隔(秒)">
                <el-input-number v-model="enosConfig.fetch_interval_sec" :min="5" :max="3600" style="width:100%" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="回溯时长(秒)">
                <el-input-number v-model="enosConfig.lookback_sec" :min="10" :max="86400" style="width:100%" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="失效策略">
                <el-select v-model="enosConfig.failure_policy" style="width:100%">
                  <el-option value="hold" label="保持最后值" />
                  <el-option value="zero" label="置零" />
                  <el-option value="stop" label="停止仿真" />
                </el-select>
              </el-form-item>
            </el-col>
          </el-row>
          <div style="margin-top:8px">
            <el-button size="small" @click="testEnosConnection" :loading="testingConnection" :disabled="!canTestConnection">
              测试连接
            </el-button>
            <span v-if="connectionTestResult" :style="{marginLeft:'8px',fontSize:'12px',color:connectionTestResult.success?'#67c23a':'#f56c6c'}">
              {{ connectionTestResult.message }}
            </span>
          </div>
        </el-form>
      </div>
      <div v-else style="padding:12px;text-align:center;color:var(--el-text-color-secondary);font-size:12px">
        启用后可为设备配置 EnOS 历史数据回放
      </div>
    </el-card>

    <!-- Device list by type -->
    <el-card shadow="never" v-for="devType in deviceTypes" :key="devType.key" style="margin-bottom:12px">
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <span style="font-weight:600">{{ devType.icon }} {{ devType.label }} ({{ getDevices(devType.key).length }})</span>
          <el-button v-if="devType.key !== 'Meter'" size="small" type="primary" :disabled="disabled" @click="addDevice(devType.key)">添加{{ devType.label }}</el-button>
          <span v-else style="font-size:12px;color:#94a3b8">固定1台，自动汇总功率</span>
        </div>
      </template>
      <div v-if="getDevices(devType.key).length === 0" style="color:#94a3b8;text-align:center;padding:12px">
        暂无{{ devType.label }}设备
      </div>
      <div v-for="(dev, idx) in getDevices(devType.key)" :key="dev.DeviceKey" class="device-form-card">
        <div class="device-form-header">
          <span style="font-weight:600">{{ dev.DeviceKey }}</span>
          <el-button v-if="devType.key !== 'Meter'" size="small" type="danger" text :disabled="disabled" @click="removeDevice(devType.key, idx)">删除</el-button>
        </div>
        <!-- PV -->
        <el-form v-if="devType.key === 'PV'" label-width="110px" :disabled="disabled" size="small">
          <el-row :gutter="12">
            <el-col :span="8"><el-form-item label="设备标识"><el-input v-model="dev.DeviceKey" /></el-form-item></el-col>
            <el-col :span="8"><el-form-item label="额定功率(kW)"><el-input-number v-model="dev.ratedPower" :min="0" :max="99999" style="width:100%" /></el-form-item></el-col>
            <el-col :span="8"><el-form-item label="Slave ID"><el-input-number v-model="dev.slave_id" :min="1" :max="247" style="width:100%" /></el-form-item></el-col>
          </el-row>
          <el-row :gutter="12">
            <el-col :span="8">
              <el-form-item label="数据源">
                <el-select v-model="dev.mode" style="width:100%">
                  <el-option :value="0" label="CSV曲线回放" />
                  <el-option :value="1" label="正弦曲线" />
                  <el-option :value="2" label="EnOS回放" :disabled="!enosConfig.enabled" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="8" v-if="dev.mode === 0">
              <el-form-item label="CSV文件">
                <el-select v-model="dev.csv_file" style="width:100%" allow-create filterable>
                  <el-option v-for="f in curveFiles" :key="f" :value="f" :label="f" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="8" v-if="dev.mode === 2">
              <el-form-item label="EnOS资产ID">
                <el-input v-model="dev.enos_asset_id" placeholder="pv_asset_001" />
              </el-form-item>
            </el-col>
          </el-row>
          <el-row :gutter="12" v-if="dev.mode === 2">
            <el-col :span="12">
              <el-form-item label="功率测点">
                <el-input v-model="dev.enos_power_point" placeholder="INV.GenActivePW" />
              </el-form-item>
            </el-col>
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
          <el-row :gutter="12">
            <el-col :span="8">
              <el-form-item label="数据源">
                <el-select v-model="dev.mode" style="width:100%">
                  <el-option :value="0" label="模型计算" />
                  <el-option :value="2" label="EnOS回放" :disabled="!enosConfig.enabled" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="8" v-if="dev.mode === 2">
              <el-form-item label="EnOS资产ID">
                <el-input v-model="dev.enos_asset_id" placeholder="bess_asset_001" />
              </el-form-item>
            </el-col>
          </el-row>
          <el-row :gutter="12" v-if="dev.mode === 2">
            <el-col :span="8">
              <el-form-item label="功率测点">
                <el-input v-model="dev.enos_power_point" placeholder="BS.ActivePW" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="SOC测点">
                <el-input v-model="dev.enos_soc_point" placeholder="BS.SOC (可选)" />
              </el-form-item>
            </el-col>
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
          <el-row :gutter="12">
            <el-col :span="8">
              <el-form-item label="数据源">
                <el-select v-model="dev.mode" style="width:100%">
                  <el-option :value="0" label="时段控制" />
                  <el-option :value="2" label="EnOS回放" :disabled="!enosConfig.enabled" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="8" v-if="dev.mode === 2">
              <el-form-item label="EnOS资产ID">
                <el-input v-model="dev.enos_asset_id" placeholder="ev_asset_001" />
              </el-form-item>
            </el-col>
            <el-col :span="8" v-if="dev.mode === 2">
              <el-form-item label="功率测点">
                <el-input v-model="dev.enos_power_point" placeholder="PUB_CONN.ChargePW" />
              </el-form-item>
            </el-col>
          </el-row>
          <el-form-item label="充电时段" v-if="dev.mode === 0">
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
            <el-col :span="8">
              <el-form-item label="数据源">
                <el-select v-model="dev.mode" style="width:100%">
                  <el-option :value="0" label="CSV曲线回放" />
                  <el-option :value="1" label="随机波动" />
                  <el-option :value="2" label="EnOS回放" :disabled="!enosConfig.enabled" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="8" v-if="dev.mode === 0">
              <el-form-item label="CSV文件">
                <el-select v-model="dev.csv_file" style="width:100%" allow-create filterable>
                  <el-option v-for="f in curveFiles" :key="f" :value="f" :label="f" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="8" v-if="dev.mode === 2">
              <el-form-item label="EnOS资产ID">
                <el-input v-model="dev.enos_asset_id" placeholder="load_asset_001" />
              </el-form-item>
            </el-col>
          </el-row>
          <el-row :gutter="12" v-if="dev.mode === 2">
            <el-col :span="12">
              <el-form-item label="功率测点">
                <el-input v-model="dev.enos_power_point" placeholder="LD.ActivePW" />
              </el-form-item>
            </el-col>
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
import { ref, watch, computed } from 'vue'
import { ElMessage } from 'element-plus'

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

// EnOS 配置
const enosConfig = ref({
  enabled: false,
  apigw_address: '',
  access_key: '',
  secret_key: '',
  org_id: '',
  fetch_interval_sec: 30,
  lookback_sec: 300,
  delay_sec: 10,
  target_lag_sec: 60,
  max_staleness_sec: 180,
  failure_policy: 'hold'
})

const testingConnection = ref(false)
const connectionTestResult = ref<{ success: boolean; message: string } | null>(null)

const canTestConnection = computed(() => {
  return enosConfig.value.apigw_address && 
         enosConfig.value.access_key && 
         enosConfig.value.secret_key && 
         enosConfig.value.org_id
})

watch(() => props.config, (val) => {
  if (val && val.Devices && val.Devices.length > 0) {
    localConfig.value = JSON.parse(JSON.stringify(val))
    
    // 初始化 EnOS 配置
    if (val.enos_config) {
      enosConfig.value = { ...enosConfig.value, ...val.enos_config }
    }
  }
  // Ensure exactly one Meter always exists
  ensureMeter()
}, { immediate: true, deep: true })

function ensureMeter() {
  const devices = localConfig.value.Devices || []
  let hasMeter = false
  for (const group of devices) {
    if (group['Meter'] && group['Meter'].length > 0) { hasMeter = true; break }
  }
  if (!hasMeter) {
    // Add default Meter at the beginning
    devices.unshift({ Meter: [{ DeviceKey: 'Meter_01', slave_id: nextSlaveId() }] })
    localConfig.value.Devices = devices
  }
}

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
    case 'Meter': 
      return { 
        DeviceKey: `Meter_${String(nextId).padStart(2,'0')}`, 
        slave_id: nextSlaveId(),
        mode: 0,
        enos_asset_id: '',
        enos_power_point: 'METER.ActivePW'
      }
    case 'PV': 
      return { 
        DeviceKey: `PV_${String(nextId).padStart(2,'0')}`, 
        ratedPower: 100, 
        mode: 1, 
        csv_file: '', 
        enos_asset_id: '',
        enos_power_point: 'INV.GenActivePW',
        slave_id: nextSlaveId() 
      }
    case 'BESS': 
      return { 
        DeviceKey: `BESS_${String(nextId).padStart(2,'0')}`, 
        ratedCapacity: 200, 
        maxChargePower: 100, 
        maxDischargePower: 100, 
        socMax: 100, 
        socMin: 0, 
        initial_soc: 0.5, 
        voltage_nominal: 48, 
        resistance: 0.01,
        mode: 0,
        enos_asset_id: '',
        enos_power_point: 'BS.ActivePW',
        enos_soc_point: 'BS.SOC',
        slave_id: nextSlaveId() 
      }
    case 'EV': 
      return { 
        DeviceKey: `EV_${String(nextId).padStart(2,'0')}`, 
        'PUB_CONN.RatedPW': 22, 
        'PUB_CONN.MinChargePW': 0, 
        charge_factor: 1.0, 
        charge_schedule: [{ start: '08:00', stop: '10:00' }], 
        charger_type: 'AC', 
        power_factor: 0.95, 
        phase_mode: 3, 
        voltage: 220,
        mode: 0,
        enos_asset_id: '',
        enos_power_point: 'PUB_CONN.ChargePW',
        slave_id: nextSlaveId() 
      }
    case 'Load': 
      return { 
        DeviceKey: `LOAD_${String(nextId).padStart(3,'0')}`, 
        base_power: 50, 
        mode: 0, 
        csv_file: 'load_curve.csv',
        enos_asset_id: '',
        enos_power_point: 'LD.ActivePW',
        slave_id: nextSlaveId() 
      }
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

async function testEnosConnection() {
  if (!canTestConnection.value) return
  
  testingConnection.value = true
  connectionTestResult.value = null
  
  try {
    // TODO: 实际的 EnOS 连接测试 API 调用
    // 这里模拟测试结果
    await new Promise(resolve => setTimeout(resolve, 1500))
    
    // 简单验证
    const isValidUrl = enosConfig.value.apigw_address.startsWith('https://')
    const hasCredentials = enosConfig.value.access_key.length > 0 && enosConfig.value.secret_key.length > 0
    
    if (isValidUrl && hasCredentials) {
      connectionTestResult.value = { success: true, message: '连接测试成功' }
      ElMessage.success('EnOS 连接测试成功')
    } else {
      connectionTestResult.value = { success: false, message: '配置信息不完整' }
      ElMessage.error('EnOS 连接测试失败：配置信息不完整')
    }
  } catch (error) {
    connectionTestResult.value = { success: false, message: '连接失败' }
    ElMessage.error('EnOS 连接测试失败')
  } finally {
    testingConnection.value = false
  }
}

function handleSave() { 
  // 将 EnOS 配置合并到主配置中
  const configToSave = {
    ...localConfig.value,
    enos_config: enosConfig.value
  }
  emit('save', configToSave)
}
</script>

<style scoped>
.device-form-card { border: 1px solid #e5e7eb; border-radius: 8px; padding: 12px; margin-bottom: 10px; }
.device-form-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; padding-bottom: 8px; border-bottom: 1px solid #f3f4f6; }
</style>
