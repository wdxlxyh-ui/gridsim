<template>
  <!-- Add Device Dialog -->
  <el-dialog v-model="showAddDevice" title="添加设备" width="520px" destroy-on-close @open="updateAddPreview">
    <el-form label-width="110px" size="small">
      <el-form-item label="设备类型">
        <el-radio-group v-model="newDeviceType" @change="updateAddPreview">
          <el-radio-button value="pv">光伏</el-radio-button>
          <el-radio-button value="battery">储能</el-radio-button>
          <el-radio-button value="load">负荷</el-radio-button>
          <el-radio-button value="charger">充电桩</el-radio-button>
        </el-radio-group>
      </el-form-item>
      <el-form-item label="设备名称">
        <el-input v-model="newDeviceName" placeholder="例如: PV-1" />
      </el-form-item>
      <el-form-item label="IOA 分配">
        <div class="ioa-preview-box">
          <div class="ioa-preview-header">
            <span class="ioa-preview-badge">{{ addIOAPreview.base }}~{{ addIOAPreview.base + 49 }}</span>
            <span class="ioa-preview-note">（系统自动分配）</span>
          </div>
          <div class="ioa-preview-ranges">
            <span v-for="r in addIOAPreview.ranges" :key="r" class="ioa-preview-range">{{ r }}</span>
          </div>
        </div>
      </el-form-item>
      <template v-if="newDeviceType === 'pv'">
        <el-form-item label="额定功率"><el-input-number v-model="newDeviceParams.rated_power_kw" :min="0" :max="99999" style="width:100%" /> kW</el-form-item>
        <el-form-item label="效率"><el-input-number v-model="newDeviceParams.efficiency" :min="0" :max="1" :step="0.05" style="width:100%" /></el-form-item>
      </template>
      <template v-if="newDeviceType === 'battery'">
        <el-form-item label="额定容量"><el-input-number v-model="newDeviceParams.capacity_kwh" :min="0" :max="99999" style="width:100%" /> kWh</el-form-item>
        <el-form-item label="额定功率"><el-input-number v-model="newDeviceParams.rated_power_kw_b" :min="0" :max="99999" style="width:100%" /> kW</el-form-item>
        <el-form-item label="初始 SOC"><el-input-number v-model="newDeviceParams.init_soc" :min="0" :max="100" style="width:100%" /> %</el-form-item>
        <el-form-item label="SOC 范围">
          <el-input-number v-model="newDeviceParams.soc_min" :min="0" :max="100" style="width:45%" /> % ~
          <el-input-number v-model="newDeviceParams.soc_max" :min="0" :max="100" style="width:45%" /> %
        </el-form-item>
      </template>
      <template v-if="newDeviceType === 'load'">
        <el-form-item label="额定功率"><el-input-number v-model="newDeviceParams.load_rated_kw" :min="0" :max="99999" style="width:100%" /> kW</el-form-item>
        <el-form-item label="功率因数"><el-input-number v-model="newDeviceParams.power_factor" :min="0" :max="1" :step="0.05" style="width:100%" /></el-form-item>
      </template>
      <template v-if="newDeviceType === 'charger'">
        <el-form-item label="额定功率"><el-input-number v-model="newDeviceParams.charger_rated_kw" :min="0" :max="99999" style="width:100%" /> kW</el-form-item>
        <el-form-item label="效率"><el-input-number v-model="newDeviceParams.charger_eff" :min="0" :max="1" :step="0.05" style="width:100%" /></el-form-item>
      </template>
    </el-form>
    <el-divider style="margin:8px 0;font-size:12px;color:#909399">自定义测点</el-divider>
    <el-table :data="newCustomPoints" size="small" max-height="180" empty-text="暂无">
      <el-table-column label="名称" min-width="100">
        <template #default="{ row }"><el-input v-model="row.name" size="small" placeholder="如: 电芯1电压" /></template>
      </el-table-column>
      <el-table-column label="标识" min-width="100">
        <template #default="{ row }"><el-input v-model="row.alias" size="small" placeholder="如: BMS.CellVol" /></template>
      </el-table-column>
      <el-table-column label="类型" width="80">
        <template #default="{ row }">
          <el-select v-model="row.type" size="small">
            <el-option value="AI" label="AI" /><el-option value="DI" label="DI" /><el-option value="AO" label="AO" /><el-option value="DO" label="DO" />
          </el-select>
        </template>
      </el-table-column>
      <el-table-column label="" width="50">
        <template #default="{ $index }"><el-button size="small" type="danger" text @click="newCustomPoints.splice($index,1)">✕</el-button></template>
      </el-table-column>
    </el-table>
    <el-button size="small" style="margin-top:4px" @click="newCustomPoints.push({name:'',type:'AI',alias:''})">+ 添加</el-button>
    <template #footer>
      <el-button @click="showAddDevice = false">取消</el-button>
      <el-button type="primary" @click="$emit('confirm-add', { type: newDeviceType, name: newDeviceName, params: newDeviceParams, customPoints: newCustomPoints })" :loading="addingDevice">添加</el-button>
    </template>
  </el-dialog>

  <!-- Edit Device Dialog -->
  <el-dialog v-model="showEditDevice" :title="'编辑设备参数 — ' + (editingDevice?.name || '')" width="620px" destroy-on-close>
    <el-form label-width="110px" size="small">
      <el-form-item label="设备名称">
        <el-input v-model="editingDeviceName" />
      </el-form-item>
      <el-form-item label="控制模式">
        <el-radio-group v-model="editingControlMode">
          <el-radio value="remote">远方(AO跟随)</el-radio>
          <el-radio value="local">本地(策略)</el-radio>
        </el-radio-group>
      </el-form-item>
      <template v-if="editingDevice?.type === 'pv'">
        <el-form-item label="额定功率"><el-input-number v-model="editingDeviceParams.rated_power_kw" :min="0" :max="99999" style="width:100%" /> kW</el-form-item>
        <el-form-item label="效率"><el-input-number v-model="editingDeviceParams.efficiency" :min="0" :max="1" :step="0.05" style="width:100%" /></el-form-item>
      </template>
      <template v-if="editingDevice?.type === 'battery'">
        <el-form-item label="额定容量"><el-input-number v-model="editingDeviceParams.capacity_kwh" :min="0" :max="99999" style="width:100%" /> kWh</el-form-item>
        <el-form-item label="额定功率"><el-input-number v-model="editingDeviceParams.rated_power_kw_b" :min="0" :max="99999" style="width:100%" /> kW</el-form-item>
        <el-form-item label="初始 SOC"><el-input-number v-model="editingDeviceParams.init_soc" :min="0" :max="100" style="width:100%" /> %</el-form-item>
        <el-form-item label="SOC 范围">
          <el-input-number v-model="editingDeviceParams.soc_min" :min="0" :max="100" style="width:45%" /> % ~
          <el-input-number v-model="editingDeviceParams.soc_max" :min="0" :max="100" style="width:45%" /> %
        </el-form-item>
      </template>
      <template v-if="editingDevice?.type === 'load'">
        <el-form-item label="额定功率"><el-input-number v-model="editingDeviceParams.load_rated_kw" :min="0" :max="99999" style="width:100%" /> kW</el-form-item>
        <el-form-item label="功率因数"><el-input-number v-model="editingDeviceParams.power_factor" :min="0" :max="1" :step="0.05" style="width:100%" /></el-form-item>
      </template>
      <template v-if="editingDevice?.type === 'charger'">
        <el-form-item label="额定功率"><el-input-number v-model="editingDeviceParams.charger_rated_kw" :min="0" :max="99999" style="width:100%" /> kW</el-form-item>
        <el-form-item label="效率"><el-input-number v-model="editingDeviceParams.charger_eff" :min="0" :max="1" :step="0.05" style="width:100%" /></el-form-item>
      </template>
    </el-form>
    <template #footer>
      <el-button @click="showEditDevice = false">取消</el-button>
      <el-button type="primary" @click="$emit('confirm-edit', { id: editingDevice?.id, type: editingDevice?.type, switch: editingDevice?.switch, name: editingDeviceName, params: editingDeviceParams, control_mode: editingControlMode, customPoints: editingCustomPoints })" :loading="updatingDevice">保存</el-button>
    </template>
  </el-dialog>

  <!-- Strategy Dialog -->
  <el-dialog v-model="showStrategyDialog" :title="'策略配置 — ' + strategyTargetName" width="700px" :close-on-click-modal="false">
    <div style="margin-bottom:12px;font-size:12px;color:#909399">
      目标: {{ strategyTargetName }} (IOA: {{ strategyPointIOA }})
    </div>
    <el-tabs v-model="strategyTab" type="card">
      <el-tab-pane label="递增" name="increment">
        <el-form label-width="100px" size="small">
          <el-form-item label="起始值"><el-input-number v-model="strategyForm.start_value" :min="0" :step="1" style="width:200px" /></el-form-item>
          <el-form-item label="步长"><el-input-number v-model="strategyForm.step" :min="0.1" :step="0.1" style="width:200px" /></el-form-item>
          <el-form-item label="周期(ms)"><el-input-number v-model="strategyForm.period_ms" :min="100" :step="100" style="width:200px" /></el-form-item>
          <el-form-item label="最大值"><el-input-number v-model="strategyForm.max_value" :min="0" :step="1" style="width:200px" /></el-form-item>
        </el-form>
      </el-tab-pane>
      <el-tab-pane label="随机" name="random">
        <el-form label-width="100px" size="small">
          <el-form-item label="周期(ms)"><el-input-number v-model="strategyForm.period_ms" :min="100" :step="100" style="width:200px" /></el-form-item>
          <el-form-item label="最小值"><el-input-number v-model="strategyForm.min_value" :min="0" :step="1" style="width:200px" /></el-form-item>
          <el-form-item label="最大值"><el-input-number v-model="strategyForm.max_value_r" :min="0" :step="1" style="width:200px" /></el-form-item>
          <el-form-item label="小数位数">
            <el-radio-group v-model="strategyForm.decimal_places">
              <el-radio :value="0">整数</el-radio>
              <el-radio :value="1">1位小数</el-radio>
            </el-radio-group>
          </el-form-item>
        </el-form>
      </el-tab-pane>
      <el-tab-pane label="CSV" name="csv">
        <el-form label-width="100px" size="small">
          <el-form-item label="CSV文件">
            <el-select v-model="strategyForm.csv_file" placeholder="选择文件" filterable clearable style="width:200px">
              <el-option v-for="f in csvFileList" :key="f.name" :label="f.name+(f.shared?' (共享)':'')" :value="f.name" />
            </el-select>
          </el-form-item>
          <el-form-item label="时间格式">
            <el-radio-group v-model="strategyForm.time_format"><el-radio value="relative">相对</el-radio><el-radio value="absolute">绝对</el-radio></el-radio-group>
          </el-form-item>
          <el-form-item label="循环"><el-switch v-model="strategyForm.csv_loop" /></el-form-item>
        </el-form>
      </el-tab-pane>
      <el-tab-pane label="最大值(MAX)" name="max">
        <el-form label-width="100px" size="small">
          <el-form-item label="关联IOA"><el-input v-model="strategyForm.linked_ioas" placeholder="以逗号分隔" style="width:300px" /></el-form-item>
        </el-form>
      </el-tab-pane>
      <el-tab-pane label="最小值(MIN)" name="min">
        <el-form label-width="100px" size="small">
          <el-form-item label="关联IOA"><el-input v-model="strategyForm.linked_ioas" placeholder="以逗号分隔" style="width:300px" /></el-form-item>
        </el-form>
      </el-tab-pane>
      <el-tab-pane label="SOC" name="soc">
        <el-form label-width="100px" size="small">
          <el-form-item label="容量(kWh)"><el-input-number v-model="strategyForm.capacity" :min="0" style="width:200px" /></el-form-item>
          <el-form-item label="初始SOC%"><el-input-number v-model="strategyForm.init_soc" :min="0" :max="100" style="width:200px" /></el-form-item>
          <el-form-item label="SOC范围"><el-input-number v-model="strategyForm.soc_min" :min="0" :max="100" style="width:80px" />% ~ <el-input-number v-model="strategyForm.soc_max" :min="0" :max="100" style="width:80px" />%</el-form-item>
        </el-form>
      </el-tab-pane>
      <el-tab-pane label="电量" name="energy">
        <el-form label-width="100px" size="small"><el-form-item label="倍率"><el-input-number v-model="strategyForm.pulse_energy" :min="0" style="width:200px" /></el-form-item></el-form>
      </el-tab-pane>
      <el-tab-pane label="AO关联" name="aofollow">
        <el-form label-width="100px" size="small"><el-form-item label="关联AO ID"><el-input v-model="strategyForm.linked_ioa" style="width:200px" /></el-form-item></el-form>
      </el-tab-pane>
      <el-tab-pane label="手动" name="manual"><div style="font-size:12px;color:#909399">手动模式：仅允许 API 写入，引擎不做计算</div></el-tab-pane>
    </el-tabs>
    <template #footer>
      <el-button @click="showStrategyDialog = false">取消</el-button>
      <el-button type="primary" @click="$emit('confirm-strategy', { ioa: strategyPointIOA, tab: strategyTab, form: strategyForm })" :loading="savingStrategy">确认策略</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import type { MicrogridDevice, MicrogridDeviceParams, MicrogridCustomPoint } from '../../api'

const props = defineProps<{
  showAddDevice: boolean
  showEditDevice: boolean
  showStrategyDialog: boolean
  addingDevice: boolean
  updatingDevice: boolean
  savingStrategy: boolean
  editingDevice: MicrogridDevice | null
  strategyPointIOA?: number
  strategyTargetName: string
  csvFileList: any[]
  devices: MicrogridDevice[]
}>()

const emit = defineEmits<{
  'update:showAddDevice': [v: boolean]
  'update:showEditDevice': [v: boolean]
  'update:showStrategyDialog': [v: boolean]
  'confirm-add': [payload: any]
  'confirm-edit': [payload: any]
  'confirm-strategy': [payload: any]
}>()

const showAddDevice = computed({ get: () => props.showAddDevice, set: (v) => emit('update:showAddDevice', v) })
const showEditDevice = computed({ get: () => props.showEditDevice, set: (v) => emit('update:showEditDevice', v) })
const showStrategyDialog = computed({ get: () => props.showStrategyDialog, set: (v) => emit('update:showStrategyDialog', v) })

const newDeviceType = ref<'pv' | 'battery' | 'load' | 'charger'>('pv')
const newDeviceName = ref('')
const newDeviceParams = ref<MicrogridDeviceParams>({})
const newCustomPoints = ref<MicrogridCustomPoint[]>([])
const addIOAPreview = ref<{ base: number; ranges: string[] }>({ base: 101, ranges: [] })

const editingDeviceName = ref('')
const editingDeviceParams = ref<MicrogridDeviceParams>({})
const editingControlMode = ref<'remote' | 'local'>('remote')
const editingCustomPoints = ref<MicrogridCustomPoint[]>([])

const strategyTab = ref('increment')
const strategyForm = ref<any>({
  start_value: 0, step: 1, period_ms: 1000, max_value: 100,
  min_value: 0, max_value_r: 100, decimal_places: 0,
  csv_file: '', time_format: 'relative', time_unit: 'ms', csv_loop: true,
  linked_ioas: '', linked_ioa: '', capacity: 0, init_soc: 50,
  soc_min: 10, soc_max: 90, pulse_energy: 1,
})

const STDOFF: Record<string, { off: number; name: string; type: string }[]> = {
  pv: [
    { off: 0, name: '有功功率', type: 'AI' }, { off: 1, name: '日发电量', type: 'AI' },
    { off: 10, name: '运行状态', type: 'DI' }, { off: 11, name: '开关状态', type: 'DI' },
    { off: 20, name: '功率设定', type: 'AO' }, { off: 30, name: '远程启机', type: 'DO' },
  ],
  battery: [
    { off: 0, name: '电池SOC', type: 'AI' }, { off: 1, name: '充放电功率', type: 'AI' },
    { off: 10, name: '运行状态', type: 'DI' }, { off: 11, name: '开关状态', type: 'DI' },
    { off: 20, name: '功率设定', type: 'AO' }, { off: 30, name: '远程启机', type: 'DO' },
  ],
  load: [
    { off: 0, name: '有功功率', type: 'AI' },
    { off: 10, name: '运行状态', type: 'DI' }, { off: 11, name: '开关状态', type: 'DI' },
    { off: 20, name: '功率设定', type: 'AO' }, { off: 30, name: '遥控分合', type: 'DO' },
  ],
  charger: [
    { off: 0, name: '充电功率', type: 'AI' },
    { off: 10, name: '运行状态', type: 'DI' }, { off: 11, name: '开关状态', type: 'DI' },
    { off: 20, name: '功率设定', type: 'AO' }, { off: 30, name: '遥控分合', type: 'DO' },
  ],
}

function nextAvailableIOABase(): number {
  const occupied = new Set<number>()
  for (let i = 1; i <= 100; i++) occupied.add(i)
  for (const d of props.devices) {
    const base = d.ioa_base || 0
    if (base === 0) continue
    for (let off = 0; off < 50; off++) occupied.add(base + off)
  }
  for (let base = 101; base <= 65000; base += 50) {
    let free = true
    for (let off = 0; off < 50; off++) {
      if (occupied.has(base + off)) { free = false; break }
    }
    if (free) return base
  }
  return 0
}

function updateAddPreview() {
  const base = nextAvailableIOABase()
  const offsets = STDOFF[newDeviceType.value] || []
  const ai = offsets.filter(o => o.type === 'AI')
  const di = offsets.filter(o => o.type === 'DI')
  const ranges: string[] = []
  if (ai.length) ranges.push(`AI ${base}~${base + ai.length - 1}`)
  if (di.length) ranges.push(`DI ${base + di[0].off}~${base + di[di.length - 1].off}`)
  const ao = offsets.find(o => o.type === 'AO')
  if (ao) ranges.push(`AO ${base + ao.off}`)
  const do_ = offsets.find(o => o.type === 'DO')
  if (do_) ranges.push(`DO ${base + do_.off}`)
  ranges.push(`自定义 ${base + 40}~${base + 49}`)
  addIOAPreview.value = { base, ranges }
}

function resetNewDevice() {
  newDeviceType.value = 'pv'
  newDeviceName.value = ''
  newDeviceParams.value = {}
  newCustomPoints.value = []
}
</script>

<style scoped>
.ioa-preview-box {
  width: 100%;
  background: #ecf5ff;
  border: 1px solid #b3d8ff;
  border-radius: 6px;
  padding: 10px 12px;
}
.ioa-preview-header { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; }
.ioa-preview-badge {
  font-family: 'SF Mono', 'Menlo', monospace;
  font-size: 12px; font-weight: 600; color: #409eff;
  padding: 1px 8px; background: rgba(64, 158, 255, 0.1); border-radius: 3px;
}
.ioa-preview-note { font-size: 10px; color: #909399; }
.ioa-preview-ranges { display: flex; flex-wrap: wrap; gap: 2px 14px; font-size: 10px; color: #909399; }
.ioa-preview-range { white-space: nowrap; }
</style>
