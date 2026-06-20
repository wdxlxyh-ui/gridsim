<template>
  <div class="microgrid-editor">
    <MicrogridTopologyHeader
      :instance-name="instanceName"
      :running="running"
      :action-loading="actionLoading"
      :topology-changed="topologyChanged"
      @go-back="goBack"
      @start="handleStart"
      @stop="handleStop"
      @save-topology="handleSaveTopology"
      @export-topology="handleExportTopology"
      @import-topology="handleImportTopology"
      @export-xlsx="handleExportXLSX"
    />

    <MicrogridIOABanner
      :ioa-conflicts="ioaConflicts"
      :has-devices="devices.length > 0"
      :running="running"
      @auto-resolve="autoResolveConflicts"
    />

    <el-tabs v-model="activeTab" type="border-card">
      <el-tab-pane label="拓扑配置" name="topology">
        <div class="topology-grid">
          <div class="topo-left">
            <MicrogridGridMeterConfig
              :grid-meter="gridMeter"
              :bus-name="busName"
              :bus-voltage="busVoltage"
              @update:grid-meter="gridMeter = $event"
              @update:bus-name="busName = $event"
              @update:bus-voltage="busVoltage = $event"
              @change="topologyChanged = true"
            />
            <MicrogridDeviceList
              :devices="devices"
              :running="running"
              @add-device="showAddDevice = true"
              @edit-device="editDevice($event)"
              @delete-device="handleDeleteDevice($event)"
              @toggle-switch="(id: string, val: boolean) => handleSwitchToggle(id, val)"
            />
          </div>
          <div class="topo-right">
            <MicrogridDashboardCard :running="running" :dash="dash" />
            <MicrogridSvgTopology :devices="devices" :dash="dash" :running="running" :bus-name="busName" :bus-voltage="busVoltage" />
            <MicrogridFormulaPreview :devices="devices" />
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="测点管理" name="points">
        <MicrogridPointTable
          :points="points"
          :loading-points="loadingPoints"
          @refresh="fetchPoints(true)"
          @toggle-mode="togglePointMode($event, $event)"
          @config-strategy="configPointStrategy($event)"
        />
      </el-tab-pane>
    </el-tabs>

    <input ref="topoImportRef" type="file" accept=".json" style="display:none" @change="onTopoFile" />

    <MicrogridDeviceDialogs
      :show-add-device="showAddDevice"
      :show-edit-device="showEditDevice"
      :show-strategy-dialog="showStrategyDialog"
      :adding-device="addingDevice"
      :updating-device="updatingDevice"
      :saving-strategy="savingStrategy"
      :editing-device="editingDevice"
      :strategy-point-ioa="strategyPointIOA"
      :strategy-target-name="strategyTargetName"
      :csv-file-list="csvFileList"
      :devices="devices"
      @update:show-add-device="showAddDevice = $event"
      @update:show-edit-device="showEditDevice = $event"
      @update:show-strategy-dialog="showStrategyDialog = $event"
      @confirm-add="handleAddDevice($event)"
      @confirm-edit="handleUpdateDevice($event)"
      @confirm-strategy="confirmStrategy($event)"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'

import MicrogridTopologyHeader from '../components/microgrid/MicrogridTopologyHeader.vue'
import MicrogridIOABanner from '../components/microgrid/MicrogridIOABanner.vue'
import MicrogridGridMeterConfig from '../components/microgrid/MicrogridGridMeterConfig.vue'
import MicrogridDeviceList from '../components/microgrid/MicrogridDeviceList.vue'
import MicrogridDashboardCard from '../components/microgrid/MicrogridDashboardCard.vue'
import MicrogridSvgTopology from '../components/microgrid/MicrogridSvgTopology.vue'
import MicrogridFormulaPreview from '../components/microgrid/MicrogridFormulaPreview.vue'
import MicrogridPointTable from '../components/microgrid/MicrogridPointTable.vue'
import MicrogridDeviceDialogs from '../components/microgrid/MicrogridDeviceDialogs.vue'

import {
  getMicrogridTopology,
  saveMicrogridTopology,
  addMicrogridDevice,
  deleteMicrogridDevice,
  controlMicrogridSwitch,
  updateMicrogridDevice,
  getMicrogridDashboard,
  getMicrogridPoints,
  getInstance,
  startInstance,
  stopInstance,
  setAutoChange,
  deleteAutoChange,
  type MicrogridTopology,
  type MicrogridDevice,
  type MicrogridDashboard,
  type MicrogridDeviceParams,
  type MicrogridCustomPoint,
} from '../api'

const route = useRoute()
const router = useRouter()
const instanceId = route.params.id as string

const activeTab = ref('topology')
const instanceName = ref('')
const running = ref(false)
const actionLoading = ref(false)
const topologyChanged = ref(false)

const busName = ref('10kV 母线')
const busVoltage = ref(10)
const gridMeter = ref({ rated_capacity_kw: 500, island_mode: false })
watch([gridMeter, busVoltage, busName], () => { topologyChanged.value = true }, { deep: true, flush: 'sync' })
const devices = ref<MicrogridDevice[]>([])
const dash = ref<MicrogridDashboard>({ grid_power_kw: 0, total_pv_kw: 0, total_bat_kw: 0, total_load_kw: 0, total_charger_kw: 0 })
const points = ref<any[]>([])
const loadingPoints = ref(false)

const showAddDevice = ref(false)
const addingDevice = ref(false)
const showEditDevice = ref(false)
const topoImportRef = ref<HTMLInputElement>()
const updatingDevice = ref(false)
const editingDevice = ref<MicrogridDevice | null>(null)
const ioaConflicts = ref<string[]>([])

const showStrategyDialog = ref(false)
const savingStrategy = ref(false)
const strategyPointIOA = ref(0)
const strategyTargetName = ref('')
const csvFileList = ref<any[]>([])

let pollTimer: ReturnType<typeof setInterval> | null = null
let pointsTimer: ReturnType<typeof setInterval> | null = null

function validateIOA(): string[] {
  const errors: string[] = []
  const seen = new Map<number, string>()
  for (let i = 1; i <= 100; i++) seen.set(i, '(关口表保留区)')
  for (const d of devices.value) {
    const base = d.ioa_base || 0
    if (base === 0) { errors.push(`设备 "${d.name}" 未分配 IOABase`); continue }
    if (base < 101) { errors.push(`设备 "${d.name}" IOA ${base} 与关口表保留区 (1-100) 冲突`) }
    for (let off = 0; off < 50; off++) {
      const ioa = base + off
      if (seen.has(ioa) && seen.get(ioa) !== d.id) {
        errors.push(`IOA ${ioa} 冲突: 设备 "${d.name}" 与 ${seen.get(ioa)}`)
      }
      seen.set(ioa, d.name)
    }
  }
  return errors
}

function runIOAValidation() { ioaConflicts.value = validateIOA() }

function autoResolveConflicts() {
  const sorted = [...devices.value].sort((a, b) => (a.ioa_base || 0) - (b.ioa_base || 0) || a.id.localeCompare(b.id))
  let base = 101
  for (const d of sorted) {
    while (base <= 100) base += 50
    d.ioa_base = base
    base += 50
  }
  runIOAValidation()
  topologyChanged.value = true
}

function goBack() { router.push('/config') }

async function fetchTopology() {
  try {
    const topo = await getMicrogridTopology(instanceId)
    busName.value = topo.bus_name
    busVoltage.value = topo.bus_voltage_kv
    gridMeter.value = { ...topo.grid_meter }
    devices.value = topo.devices || []
    topologyChanged.value = false
    runIOAValidation()
  } catch (e: any) {
    ElMessage.error('获取拓扑失败: ' + (e?.response?.data?.error || e.message))
  }
}

async function fetchInstance() {
  try {
    const inst = await getInstance(instanceId)
    instanceName.value = inst.name
    running.value = inst.status === 'running'
  } catch {}
}

async function fetchDashboard() {
  try { Object.assign(dash.value, await getMicrogridDashboard(instanceId)) } catch {}
}

async function fetchPoints(reloading = false) {
  if (reloading) loadingPoints.value = true
  try {
    const data = await getMicrogridPoints(instanceId)
    const newPts = data.points || []
    if (points.value.length === 0 || reloading) {
      points.value = newPts
    } else {
      const byIOA = new Map(newPts.map((p: any) => [p.ioa, p]))
      for (const p of points.value) {
        const np = byIOA.get(p.ioa)
        if (np) { p.value = np.value; p.local_mode = np.local_mode }
      }
    }
  } catch {} finally {
    if (reloading) loadingPoints.value = false
  }
}

async function loadAll() {
  await Promise.all([fetchTopology(), fetchInstance(), fetchPoints(true)])
  if (running.value) await fetchDashboard()
}

async function handleStart() {
  await handleSaveTopology()
  actionLoading.value = true
  try {
    await startInstance(instanceId)
    ElMessage.success('微电网已启动')
    running.value = true
    await fetchDashboard()
    startPolling()
  } catch (e: any) {
    ElMessage.error('启动失败: ' + (e?.response?.data?.error || e.message))
  } finally { actionLoading.value = false }
}

async function handleStop() {
  actionLoading.value = true
  try {
    await stopInstance(instanceId)
    ElMessage.success('已停止')
    running.value = false
    stopPolling()
  } catch (e: any) {
    ElMessage.error('停止失败: ' + (e?.response?.data?.error || e.message))
  } finally { actionLoading.value = false }
}

async function handleSaveTopology() {
  const topo: MicrogridTopology = {
    bus_name: busName.value,
    bus_voltage_kv: busVoltage.value,
    grid_meter: { ...gridMeter.value },
    devices: devices.value,
  }
  try {
    await saveMicrogridTopology(instanceId, topo)
    ElMessage.success('拓扑已保存')
    topologyChanged.value = false
  } catch (e: any) {
    ElMessage.error('保存失败: ' + (e?.response?.data?.error || e.message))
  }
}

function handleExportTopology() {
  const blob = new Blob([JSON.stringify({ bus_name: busName.value, bus_voltage_kv: busVoltage.value, grid_meter: gridMeter.value, devices: devices.value }, null, 2)], { type: 'application/json' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob); a.download = `${instanceId}_topology.json`
  a.click(); URL.revokeObjectURL(a.href)
}

function handleImportTopology() { topoImportRef.value?.click() }

async function onTopoFile(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  try {
    const topo = JSON.parse(await file.text())
    busName.value = topo.bus_name || '10kV 母线'
    busVoltage.value = topo.bus_voltage_kv || 10
    gridMeter.value = topo.grid_meter || { rated_capacity_kw: 500, island_mode: false }
    devices.value = topo.devices || []
    topologyChanged.value = true
    runIOAValidation()
    ElMessage.success('拓扑已加载，请点击保存拓扑')
  } catch (e: any) { ElMessage.error('导入失败: ' + (e?.message || '格式错误')) }
}

function handleExportXLSX() { window.open(`/api/v1/microgrid/${instanceId}/export-xlsx`, '_blank') }

async function handleAddDevice(payload: any) {
  if (!payload.name) { ElMessage.warning('请输入设备名称'); return }
  addingDevice.value = true
  try {
    await addMicrogridDevice(instanceId, {
      type: payload.type,
      name: payload.name,
      params: { ...payload.params },
      custom_points: payload.customPoints.filter((p: any) => p.name.trim()),
    })
    ElMessage.success('设备已添加')
    showAddDevice.value = false
    topologyChanged.value = true
    await fetchTopology()
    runIOAValidation()
  } catch (e: any) {
    ElMessage.error('添加失败: ' + (e?.response?.data?.error || e.message))
  } finally { addingDevice.value = false }
}

function editDevice(dev: MicrogridDevice) {
  editingDevice.value = { ...dev, ioa_base: dev.ioa_base }
  showEditDevice.value = true
}

async function handleUpdateDevice(payload: any) {
  if (!editingDevice.value) return
  updatingDevice.value = true
  try {
    await updateMicrogridDevice(instanceId, {
      id: payload.id,
      type: payload.type,
      switch: payload.switch,
      name: payload.name,
      params: { ...payload.params },
      control_mode: payload.control_mode,
      custom_points: payload.customPoints.filter((p: any) => p.name.trim()),
    })
    ElMessage.success('参数已更新')
    showEditDevice.value = false
    topologyChanged.value = true
    await fetchTopology()
    await fetchInstance()
    runIOAValidation()
  } catch (e: any) {
    ElMessage.error('更新失败: ' + (e?.response?.data?.error || e.message))
  } finally { updatingDevice.value = false }
}

async function handleDeleteDevice(devId: string) {
  try {
    await ElMessageBox.confirm('确定删除此设备？', '确认')
    await deleteMicrogridDevice(instanceId, devId)
    ElMessage.success('设备已删除')
    topologyChanged.value = true
    await fetchTopology()
    runIOAValidation()
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error('删除失败: ' + (e?.response?.data?.error || e.message))
  }
}

async function handleSwitchToggle(devId: string, closed: boolean) {
  try { await controlMicrogridSwitch(instanceId, devId, closed) }
  catch (e: any) { ElMessage.error('开关操作失败: ' + (e?.response?.data?.error || e.message)) }
}

async function togglePointMode(row: any, local: boolean) {
  if (local) { configPointStrategy(row) }
  else {
    try {
      await deleteAutoChange(instanceId, row.ioa)
      ElMessage.success('已切换为远方控制')
      await fetchPoints()
    } catch (e: any) { ElMessage.error('切换失败: ' + (e?.response?.data?.error || e.message)) }
  }
}

function configPointStrategy(row: any) {
  strategyPointIOA.value = row.ioa
  strategyTargetName.value = row.name
  showStrategyDialog.value = true
}

async function confirmStrategy(payload: any) {
  savingStrategy.value = true
  try {
    if (payload.ioa > 0) {
      await setAutoChange(instanceId, payload.ioa, {
        strategy: payload.tab, enabled: true, params: { ...payload.form },
      })
      ElMessage.success('策略已保存')
      showStrategyDialog.value = false
      await fetchPoints()
    }
  } catch (e: any) {
    ElMessage.error('策略保存失败: ' + (e?.response?.data?.error || e.message))
  } finally { savingStrategy.value = false }
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(() => fetchDashboard(), 3000)
  pointsTimer = setInterval(() => fetchPoints(), 2000)
}

function stopPolling() {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
  if (pointsTimer) { clearInterval(pointsTimer); pointsTimer = null }
}

function onSvgClick(e: MouseEvent) {
  const target = e.target as HTMLElement
  const circle = target.closest('[data-action="toggle-switch"]') as HTMLElement
  if (!circle) return
  const devId = circle.getAttribute('data-dev-id')
  if (!devId) return
  const dev = devices.value.find(d => d.id === devId)
  if (!dev || !running.value) return
  handleSwitchToggle(devId, !dev.switch.closed)
}

onMounted(async () => {
  await loadAll()
  if (running.value) startPolling()
  document.addEventListener('click', onSvgClick)
})

onUnmounted(() => {
  stopPolling()
  document.removeEventListener('click', onSvgClick)
})
</script>

<style scoped>
.microgrid-editor { padding: 16px; max-width: 100%; margin: 0 auto; }
.topology-grid { display: grid; grid-template-columns: 380px 1fr; gap: 16px; }
@media (max-width: 900px) { .topology-grid { grid-template-columns: 1fr; } }
.topo-left { display: flex; flex-direction: column; gap: 12px; }
.topo-right { display: flex; flex-direction: column; gap: 12px; }
.el-tabs :deep(.el-tabs__content) { padding: 16px; }
.topology-html :deep(svg) { display: block; width: 100%; height: auto; }
.topology-html :deep(text) { font-family: system-ui, -apple-system, sans-serif; }
@keyframes flow-up { to { stroke-dashoffset: 32; } }
@keyframes flow-dn { to { stroke-dashoffset: -32; } }
.topology-html :deep(.fl-up) { stroke-dasharray: 12 4; animation: flow-up .6s linear infinite; stroke-width: 3.5; }
.topology-html :deep(.fl-dn) { stroke-dasharray: 12 4; animation: flow-dn .6s linear infinite; stroke-width: 3.5; }
.topology-html :deep(.fz) { stroke-dasharray: 4 8; stroke: #c0c4cc !important; stroke-width: 2; }
</style>
