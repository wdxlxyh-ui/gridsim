<template>
  <div class="py-microgrid-editor">
    <!-- Header -->
    <el-card shadow="never" style="margin-bottom: 16px">
      <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 8px">
        <div style="display: flex; align-items: center; gap: 12px">
          <el-button @click="router.push('/config')" text size="small">← 返回</el-button>
          <span style="font-size: 16px; font-weight: 600">{{ instanceName || 'Python 微电网' }}</span>
          <el-tag v-if="running" type="success" size="small">运行中</el-tag>
          <el-tag v-else type="info" size="small">已停止</el-tag>
        </div>
        <div style="display: flex; gap: 8px">
          <el-button v-if="!running" type="success" :loading="actionLoading" @click="handleStart">启动</el-button>
          <el-button v-else type="warning" :loading="actionLoading" @click="handleStop">停止</el-button>
          <el-button v-if="running" @click="handleExportPoints" :loading="exporting">导出点表</el-button>
        </div>
      </div>
    </el-card>

    <el-tabs v-model="activeTab" type="border-card">
      <!-- 拓扑图 Tab -->
      <el-tab-pane label="拓扑组态" name="topology">
        <div class="topology-layout">
          <div class="topo-main">
            <PyMicrogridTopologySvg :devices="topoDevices" :dashboard="dashboard" :running="running" />
          </div>
          <div class="topo-sidebar">
            <PyMicrogridDashboardCard :dashboard="dashboard" :running="running" />
            <PyMicrogridDeviceCards :devices="topoDevices" :dashboard="dashboard" :running="running" @control="handleControl" />
          </div>
        </div>
      </el-tab-pane>

      <!-- 测点数据 Tab -->
      <el-tab-pane label="测点数据" name="points">
        <el-card shadow="never">
          <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:12px">
            <span style="font-weight:600">实时测点 ({{ points.length }})</span>
            <el-button size="small" @click="fetchPoints">刷新</el-button>
          </div>
          <el-table :data="points" stripe size="small" max-height="500">
            <el-table-column prop="ioa" label="IOA" width="70" />
            <el-table-column prop="name" label="名称" min-width="200" />
            <el-table-column prop="point_type" label="类型" width="60" />
            <el-table-column label="值" width="120">
              <template #default="{ row }">
                <span style="font-family:monospace;font-weight:600">
                  {{ row.point_type === 'DI' ? (row.bool_value ? 'ON' : 'OFF') : Number(row.value).toFixed(2) }}
                </span>
              </template>
            </el-table-column>
            <el-table-column prop="unit" label="单位" width="60" />
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- 配置管理 Tab -->
      <el-tab-pane label="配置管理" name="config">
        <el-alert v-if="running" type="warning" :closable="false" style="margin-bottom:12px">
          实例运行中，修改配置需先停止实例
        </el-alert>
        <PyConfigEditor
          :config="deviceConfig"
          :disabled="running"
          :saving="savingConfig"
          :curve-files="curveFiles"
          @save="handleSaveConfig"
          @upload-curve="handleUploadCurve"
        />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import PyMicrogridTopologySvg from '../components/py-microgrid/PyMicrogridTopologySvg.vue'
import PyMicrogridDashboardCard from '../components/py-microgrid/PyMicrogridDashboardCard.vue'
import PyMicrogridDeviceCards from '../components/py-microgrid/PyMicrogridDeviceCards.vue'
import PyConfigEditor from '../components/py-microgrid/PyConfigEditor.vue'
import {
  getInstance,
  startInstance,
  stopInstance,
  getPoints,
  getPyMicrogridTopology,
  getPyMicrogridDashboard,
  pyMicrogridControl,
  getPyMicrogridConfig,
  savePyMicrogridConfig,
  getPyMicrogridCurves,
  uploadPyMicrogridCurve,
  exportPyMicrogridPoints,
  type PyMicrogridDashboard,
  type PointSnapshot,
  type PyMicrogridDevice,
} from '../api'

const route = useRoute()
const router = useRouter()
const instanceId = route.params.id as string

const activeTab = ref('topology')
const instanceName = ref('')
const running = ref(false)
const actionLoading = ref(false)
const exporting = ref(false)
const savingConfig = ref(false)
const topoDevices = ref<PyMicrogridDevice[]>([])
const dashboard = ref<PyMicrogridDashboard>({ status: 'stopped', grid_power_kw: 0, total_pv_kw: 0, total_bat_kw: 0, total_load_kw: 0, total_charger_kw: 0, battery_soc: 0, device_count: 0 })
const points = ref<PointSnapshot[]>([])
const deviceConfig = ref<any>({ start_time: '08:00', Devices: [] })
const curveFiles = ref<string[]>([])

let pollTimer: number | null = null

async function fetchStatus() {
  try {
    const inst = await getInstance(instanceId)
    instanceName.value = inst.name
    running.value = inst.status === 'running'
  } catch {}
}

async function fetchTopology() {
  try {
    const topo = await getPyMicrogridTopology(instanceId)
    topoDevices.value = topo.devices
  } catch {}
}

async function fetchDashboard() {
  if (!running.value) return
  try { dashboard.value = await getPyMicrogridDashboard(instanceId) } catch {}
}

async function fetchPoints() {
  if (!running.value) return
  try { const resp = await getPoints(instanceId); points.value = resp.points } catch {}
}

async function fetchConfig() {
  try { deviceConfig.value = await getPyMicrogridConfig(instanceId) } catch {}
}

async function fetchCurves() {
  try { curveFiles.value = await getPyMicrogridCurves(instanceId) } catch {}
}

async function handleStart() {
  actionLoading.value = true
  try {
    await startInstance(instanceId)
    ElMessage.success('已启动')
    running.value = true
    await fetchTopology()
    startPolling()
  } catch (e: any) {
    ElMessage.error('启动失败: ' + (e?.response?.data?.error?.message || e?.response?.data?.error || e.message))
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

async function handleControl(ioa: number, value: number) {
  try {
    await pyMicrogridControl(instanceId, ioa, value)
    ElMessage.success(`已下发: IOA=${ioa}, 值=${value}`)
  } catch (e: any) { ElMessage.error('控制失败: ' + (e?.response?.data?.error || e.message)) }
}

async function handleSaveConfig(config: any) {
  savingConfig.value = true
  try {
    await savePyMicrogridConfig(instanceId, config)
    deviceConfig.value = config
    ElMessage.success('配置已保存')
    await fetchTopology()
  } catch (e: any) { ElMessage.error('保存失败: ' + (e?.response?.data?.error || e.message)) }
  finally { savingConfig.value = false }
}

async function handleUploadCurve(file: File) {
  try {
    await uploadPyMicrogridCurve(instanceId, file)
    ElMessage.success('曲线文件已上传: ' + file.name)
    await fetchCurves()
  } catch (e: any) { ElMessage.error('上传失败: ' + (e?.response?.data?.error || e.message)) }
}

async function handleExportPoints() {
  exporting.value = true
  try {
    const blob = await exportPyMicrogridPoints(instanceId)
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${instanceName.value || 'py-microgrid'}-点表.xlsx`
    a.click()
    URL.revokeObjectURL(url)
  } catch (e: any) { ElMessage.error('导出失败: ' + (e?.response?.data?.error || e.message)) }
  finally { exporting.value = false }
}

function startPolling() {
  stopPolling()
  fetchDashboard(); fetchPoints()
  pollTimer = window.setInterval(() => {
    fetchDashboard()
    if (activeTab.value === 'points') fetchPoints()
  }, 2000)
}
function stopPolling() { if (pollTimer) { clearInterval(pollTimer); pollTimer = null } }

onMounted(async () => {
  await fetchStatus()
  await fetchTopology()
  await fetchConfig()
  await fetchCurves()
  if (running.value) startPolling()
})
onUnmounted(() => stopPolling())
</script>

<style scoped>
.py-microgrid-editor { padding: 0; }
.topology-layout { display: grid; grid-template-columns: 1fr 360px; gap: 16px; }
.topo-main { min-height: 400px; }
.topo-sidebar { display: flex; flex-direction: column; gap: 12px; }
@media (max-width: 1024px) { .topology-layout { grid-template-columns: 1fr; } }
</style>
