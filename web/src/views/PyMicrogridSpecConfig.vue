<template>
  <div class="pymicrogrid-spec-config">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>Python 微电网规范配置 (v2.0)</span>
          <div class="header-actions">
            <el-button @click="loadSpec" :loading="loading">刷新</el-button>
            <el-button @click="validateConfig" :loading="validating">校验配置</el-button>
            <el-button type="primary" @click="saveSpec" :loading="saving" :disabled="!hasChanges">保存配置</el-button>
          </div>
        </div>
      </template>

      <el-form :model="spec" :rules="rules" ref="formRef" label-width="120px" v-loading="loading">
        
        <!-- 基础配置 -->
        <el-card class="section-card">
          <template #header>基础配置</template>
          
          <el-row :gutter="24">
            <el-col :span="12">
              <el-form-item label="Modbus 端口" prop="bridge.modbus_port">
                <el-input-number v-model="spec.bridge.modbus_port" :min="1024" :max="65535" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="轮询间隔" prop="bridge.poll_interval_ms">
                <el-input-number v-model="spec.bridge.poll_interval_ms" :min="100" :max="60000" />
                <span class="unit">ms</span>
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="24">
            <el-col :span="8">
              <el-form-item label="起始时间" prop="simulation.start_time">
                <el-time-picker v-model="startTimeValue" format="HH:mm" value-format="HH:mm" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="时区" prop="simulation.timezone">
                <el-select v-model="spec.simulation.timezone">
                  <el-option label="Asia/Shanghai" value="Asia/Shanghai" />
                  <el-option label="UTC" value="UTC" />
                  <el-option label="America/New_York" value="America/New_York" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="步进间隔" prop="simulation.step_sec">
                <el-input-number v-model="spec.simulation.step_sec" :min="1" :max="3600" />
                <span class="unit">秒</span>
              </el-form-item>
            </el-col>
          </el-row>
        </el-card>

        <!-- EnOS 回放配置 -->
        <el-card class="section-card">
          <template #header>
            <div class="section-header">
              <span>EnOS 数据回放</span>
              <el-switch v-model="spec.replay.enabled" />
            </div>
          </template>

          <template v-if="spec.replay.enabled">
            <el-form-item label="凭据档案" prop="replay.credential_ref">
              <el-select v-model="spec.replay.credential_ref" placeholder="选择EnOS凭据">
                <el-option
                  v-for="cred in credentials"
                  :key="cred.id"
                  :label="cred.name"
                  :value="cred.id"
                />
              </el-select>
              <el-button 
                type="text" 
                @click="manageCredentials"
                style="margin-left: 8px"
              >
                管理凭据
              </el-button>
            </el-form-item>

            <el-row :gutter="24">
              <el-col :span="8">
                <el-form-item label="拉取间隔" prop="replay.fetch_interval_sec">
                  <el-input-number v-model="spec.replay.fetch_interval_sec" :min="5" :max="3600" />
                  <span class="unit">秒</span>
                </el-form-item>
              </el-col>
              <el-col :span="8">
                <el-form-item label="回溯时长" prop="replay.lookback_sec">
                  <el-input-number v-model="spec.replay.lookback_sec" :min="10" :max="86400" />
                  <span class="unit">秒</span>
                </el-form-item>
              </el-col>
              <el-col :span="8">
                <el-form-item label="延迟时间" prop="replay.delay_sec">
                  <el-input-number v-model="spec.replay.delay_sec" :min="0" :max="86400" />
                  <span class="unit">秒</span>
                </el-form-item>
              </el-col>
            </el-row>

            <el-row :gutter="24">
              <el-col :span="8">
                <el-form-item label="目标滞后" prop="replay.target_lag_sec">
                  <el-input-number v-model="spec.replay.target_lag_sec" :min="0" :max="86400" />
                  <span class="unit">秒</span>
                </el-form-item>
              </el-col>
              <el-col :span="8">
                <el-form-item label="失效阈值" prop="replay.max_staleness_sec">
                  <el-input-number v-model="spec.replay.max_staleness_sec" :min="1" />
                  <span class="unit">秒</span>
                </el-form-item>
              </el-col>
              <el-col :span="8">
                <el-form-item label="失效策略" prop="replay.failure_policy">
                  <el-select v-model="spec.replay.failure_policy">
                    <el-option label="保持最后值" value="hold" />
                    <el-option label="置零" value="zero" />
                    <el-option label="停止仿真" value="stop" />
                  </el-select>
                </el-form-item>
              </el-col>
            </el-row>
          </template>
        </el-card>

        <!-- 设备配置 -->
        <el-card class="section-card">
          <template #header>
            <div class="section-header">
              <span>设备配置</span>
              <el-button type="primary" size="small" @click="addDevice">
                <el-icon><Plus /></el-icon>
                添加设备
              </el-button>
            </div>
          </template>

          <el-table :data="spec.devices" style="width: 100%">
            <el-table-column prop="device_key" label="设备标识" width="120" />
            <el-table-column prop="device_type" label="设备类型" width="80">
              <template #default="{ row }">
                <el-tag :type="getDeviceTypeTagType(row.device_type)">
                  {{ getDeviceTypeName(row.device_type) }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="slave_id" label="Slave ID" width="80" />
            <el-table-column prop="source.kind" label="数据源" width="100">
              <template #default="{ row }">
                <el-tag size="small">{{ getSourceKindName(row.source.kind) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="EnOS 配置" min-width="200">
              <template #default="{ row }">
                <div v-if="row.source.kind === 'enos' && row.source.enos" class="enos-info">
                  <div>资产: {{ row.source.enos.asset_id }}</div>
                  <div v-if="row.source.enos.power_point">
                    功率: {{ row.source.enos.power_point }}
                  </div>
                  <div v-if="row.source.enos.theory_power_point">
                    理论功率: {{ row.source.enos.theory_power_point }}
                  </div>
                </div>
                <div v-else-if="row.source.kind === 'enos_aggregate' && row.source.enos">
                  聚合资产: {{ row.source.enos.aggregate_asset_ids?.length || 0 }} 个
                </div>
                <span v-else class="text-secondary">-</span>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="120">
              <template #default="{ row, $index }">
                <el-button size="small" @click="editDevice($index)">编辑</el-button>
                <el-button size="small" type="danger" @click="removeDevice($index)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>

        <!-- 日志配置 -->
        <el-card class="section-card">
          <template #header>日志配置</template>
          
          <el-row :gutter="24">
            <el-col :span="8">
              <el-form-item label="系统日志级别">
                <el-select v-model="spec.logging.system_level">
                  <el-option label="DEBUG" value="DEBUG" />
                  <el-option label="INFO" value="INFO" />
                  <el-option label="WARN" value="WARN" />
                  <el-option label="ERROR" value="ERROR" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="文件大小限制">
                <el-input-number v-model="spec.logging.max_size_mb" :min="1" :max="1024" />
                <span class="unit">MB</span>
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="备份文件数">
                <el-input-number v-model="spec.logging.max_backups" :min="0" :max="100" />
              </el-form-item>
            </el-col>
          </el-row>
        </el-card>
      </el-form>
    </el-card>

    <!-- 设备编辑对话框 -->
    <DeviceConfigDialog
      v-model="deviceDialogVisible"
      :device="currentDevice"
      :credentials="credentials"
      :credential-ref="spec.replay.credential_ref"
      @save="saveDevice"
    />

    <!-- 凭据管理对话框 -->
    <el-dialog v-model="credentialDialogVisible" title="EnOS 凭据管理" width="80%">
      <EnOSCredentials @credential-updated="loadCredentials" />
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox, FormInstance } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { useRoute } from 'vue-router'
import {
  getPyMicrogridSpec,
  setPyMicrogridSpec,
  validatePyMicrogridSpec,
  listEnOSCredentials,
  type PyMicrogridSpec,
  type DeviceConfigSpec,
  type EnOSCredential
} from '@/api'
import DeviceConfigDialog from '@/components/DeviceConfigDialog.vue'
import EnOSCredentials from '@/views/EnOSCredentials.vue'

const route = useRoute()
const instanceId = route.params.id as string

const loading = ref(false)
const saving = ref(false)
const validating = ref(false)
const hasChanges = ref(false)
const formRef = ref<FormInstance>()
const deviceDialogVisible = ref(false)
const credentialDialogVisible = ref(false)
const currentDevice = ref<DeviceConfigSpec | null>(null)
const currentDeviceIndex = ref(-1)
const credentials = ref<EnOSCredential[]>([])
const originalSpec = ref<string>('')
const currentETag = ref('')

// 默认规范配置
const spec = ref<PyMicrogridSpec>({
  schema_version: 2,
  bridge: {
    modbus_port: 5021,
    poll_interval_ms: 1000
  },
  simulation: {
    start_time: '14:30',
    timezone: 'Asia/Shanghai',
    step_sec: 1
  },
  logging: {
    system_level: 'INFO',
    message_level: 'INFO',
    traffic_level: 'INFO',
    max_size_mb: 10,
    max_backups: 5
  },
  replay: {
    enabled: false,
    fetch_interval_sec: 30,
    lookback_sec: 300,
    delay_sec: 10,
    target_lag_sec: 60,
    cache_retention_sec: 600,
    request_timeout_sec: 30,
    page_size: 10000,
    max_staleness_sec: 180,
    failure_policy: 'hold',
    additional_subscriptions: []
  },
  allocations: {
    slave_id_start: 1,
    ioa_base: 1000,
    ioa_step: 100
  },
  devices: []
})

const startTimeValue = computed({
  get: () => spec.value.simulation.start_time,
  set: (val) => {
    spec.value.simulation.start_time = val
    hasChanges.value = true
  }
})

const rules = {
  'bridge.modbus_port': [
    { required: true, message: '请输入Modbus端口', trigger: 'blur' },
    { type: 'number', min: 1024, max: 65535, message: '端口范围 1024-65535', trigger: 'blur' }
  ],
  'replay.credential_ref': [
    {
      validator: (rule: any, value: any, callback: any) => {
        if (spec.value.replay.enabled && !value) {
          callback(new Error('启用回放时必须选择凭据'))
        } else {
          callback()
        }
      },
      trigger: 'change'
    }
  ]
}

onMounted(() => {
  loadCredentials()
  loadSpec()
})

async function loadCredentials() {
  try {
    credentials.value = await listEnOSCredentials()
  } catch (error) {
    console.error('加载凭据失败:', error)
  }
}

async function loadSpec() {
  loading.value = true
  try {
    const response = await getPyMicrogridSpec(instanceId)
    spec.value = response.data
    currentETag.value = response.headers.etag?.replace(/"/g, '') || ''
    originalSpec.value = JSON.stringify(spec.value)
    hasChanges.value = false
  } catch (error: any) {
    if (error.response?.status === 404) {
      // 配置不存在，使用默认值
      ElMessage.info('使用默认配置')
    } else {
      console.error('加载配置失败:', error)
      ElMessage.error('加载配置失败')
    }
  } finally {
    loading.value = false
  }
}

async function saveSpec() {
  if (!formRef.value) return
  
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  saving.value = true
  try {
    await setPyMicrogridSpec(instanceId, spec.value, currentETag.value)
    ElMessage.success('配置保存成功')
    originalSpec.value = JSON.stringify(spec.value)
    hasChanges.value = false
    // 重新加载获取新的ETag
    await loadSpec()
  } catch (error: any) {
    console.error('保存配置失败:', error)
    
    if (error.response?.status === 412) {
      ElMessage.error('配置已被其他用户修改，请刷新后重试')
    } else if (error.response?.status === 428) {
      ElMessage.error('缺少版本信息，请刷新后重试')
    } else {
      ElMessage.error(error.response?.data?.error || '保存配置失败')
    }
  } finally {
    saving.value = false
  }
}

async function validateConfig() {
  validating.value = true
  try {
    const result = await validatePyMicrogridSpec(instanceId, spec.value)
    if (result.valid) {
      ElMessage.success('配置校验通过')
    } else {
      ElMessage.error(`配置校验失败: ${result.error}`)
    }
  } catch (error: any) {
    console.error('校验配置失败:', error)
    ElMessage.error(error.response?.data?.error || '校验配置失败')
  } finally {
    validating.value = false
  }
}

function addDevice() {
  currentDevice.value = {
    device_key: `DEVICE_${spec.value.devices.length + 1}`,
    device_type: 'pv',
    slave_id: spec.value.allocations.slave_id_start + spec.value.devices.length,
    ioa_base: spec.value.allocations.ioa_base + spec.value.devices.length * spec.value.allocations.ioa_step,
    include_in_meter: true,
    params: {
      rated_power_kw: 100
    },
    source: {
      kind: 'synthetic'
    }
  }
  currentDeviceIndex.value = -1
  deviceDialogVisible.value = true
}

function editDevice(index: number) {
  currentDevice.value = JSON.parse(JSON.stringify(spec.value.devices[index]))
  currentDeviceIndex.value = index
  deviceDialogVisible.value = true
}

function saveDevice(device: DeviceConfigSpec) {
  if (currentDeviceIndex.value >= 0) {
    spec.value.devices[currentDeviceIndex.value] = device
  } else {
    spec.value.devices.push(device)
  }
  hasChanges.value = true
  deviceDialogVisible.value = false
}

async function removeDevice(index: number) {
  try {
    await ElMessageBox.confirm(
      `确定要删除设备 "${spec.value.devices[index].device_key}" 吗？`,
      '确认删除',
      { type: 'warning' }
    )
    
    spec.value.devices.splice(index, 1)
    hasChanges.value = true
  } catch (error) {
    // 用户取消
  }
}

function manageCredentials() {
  credentialDialogVisible.value = true
}

function getDeviceTypeName(type: string): string {
  const names: Record<string, string> = {
    meter: '关口表',
    pv: '光伏',
    bess: '储能',
    ev: '充电桩',
    load: '负载',
    wind: '风机'
  }
  return names[type] || type
}

function getDeviceTypeTagType(type: string): string {
  const types: Record<string, string> = {
    meter: '',
    pv: 'warning',
    bess: 'success',
    ev: 'info',
    load: 'danger',
    wind: 'primary'
  }
  return types[type] || ''
}

function getSourceKindName(kind: string): string {
  const names: Record<string, string> = {
    calculated: '计算',
    synthetic: '合成',
    csv: 'CSV',
    enos: 'EnOS',
    enos_aggregate: 'EnOS聚合'
  }
  return names[kind] || kind
}

// 监听变化
watch(() => spec.value, () => {
  const currentSpecStr = JSON.stringify(spec.value)
  hasChanges.value = currentSpecStr !== originalSpec.value
}, { deep: true })
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.section-card {
  margin-bottom: 16px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.unit {
  margin-left: 8px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.enos-info {
  font-size: 12px;
  line-height: 1.4;
}

.enos-info > div {
  margin-bottom: 2px;
}

.text-secondary {
  color: var(--el-text-color-secondary);
}
</style>