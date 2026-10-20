<template>
  <el-card shadow="never" style="margin-bottom:12px">
    <template #header>
      <div style="display:flex;justify-content:space-between;align-items:center;flex-wrap:wrap;gap:8px">
        <span style="font-weight:600">TSDB 数据回放</span>
        <div style="display:flex;gap:12px;align-items:center">
          <el-switch
            v-model="localConfig.replay.enabled"
            :disabled="disabled || !canEdit"
            active-text="启用回放"
            inactive-text="关闭"
          />
        </div>
      </div>
    </template>

    <el-alert v-if="!loaded" type="info" :closable="false" style="margin-bottom:12px">加载中...</el-alert>
    <template v-else>
      <el-alert v-if="!localConfig.workspace_ready" type="warning" :closable="false" style="margin-bottom:12px">
        该实例的独立工作区尚未创建，请先启动一次实例（可立即停止），之后即可在此配置 TSDB 回放。每个实例的 EnOS 连接与订阅相互独立。
      </el-alert>
      <el-alert v-else-if="localConfig.engine_available === false" type="warning" :closable="false" style="margin-bottom:12px">
        当前实例使用的 Python 模拟器不包含 TSDB 回放引擎（缺少 src/datasource/replay_provider.py），配置可以保存，但不会生效。请使用带回放能力的模拟器作为实例的 script_dir。
      </el-alert>
      <el-alert v-if="disabled" type="warning" :closable="false" style="margin-bottom:12px">
        实例运行中，TSDB 配置为只读，请先停止实例
      </el-alert>

      <el-collapse v-model="activeCollapse">
        <!-- EnOS connection -->
        <el-collapse-item title="EnOS 连接" name="enos">
          <el-form label-width="120px" :disabled="disabled || !canEdit" size="small">
            <el-form-item label="API 网关地址">
              <el-input v-model="localConfig.enos.apigw_address" placeholder="ag-cn5.envisioniot.com" />
            </el-form-item>
            <el-form-item label="Access Key">
              <el-input v-model="localConfig.enos.access_key" :type="showSecret ? 'text' : 'password'" autocomplete="off" />
            </el-form-item>
            <el-form-item label="Secret Key">
              <el-input v-model="localConfig.enos.secret_key" :type="showSecret ? 'text' : 'password'" autocomplete="off" />
            </el-form-item>
            <el-form-item label="组织 ID">
              <el-input v-model="localConfig.enos.org_id" placeholder="o17482544567841378" />
            </el-form-item>
            <el-form-item>
              <el-checkbox v-model="showSecret">显示密钥</el-checkbox>
              <el-button size="small" style="margin-left:16px" :disabled="!localConfig.enos.apigw_address" :loading="testing" @click="handleTest">
                测试网关连通性
              </el-button>
              <span style="font-size:12px;color:#94a3b8;margin-left:8px">仅检测网络连通，AK/SK 在回放启动时校验</span>
            </el-form-item>
          </el-form>
        </el-collapse-item>

        <!-- Replay parameters -->
        <el-collapse-item title="回放参数" name="replay">
          <el-form label-width="120px" :disabled="disabled || !canEdit" size="small">
            <el-form-item label="时间模式">
              <el-radio-group v-model="localConfig.replay.time_mode">
                <el-radio value="relative">相对时间</el-radio>
                <el-radio value="absolute">绝对时间</el-radio>
              </el-radio-group>
              <div class="hint">
                <template v-if="localConfig.replay.time_mode === 'relative'">
                  回放时刻 = 当前时刻 − 目标滞后，跟随实时数据，输出比真实设备晚「目标滞后」秒
                </template>
                <template v-else>
                  从指定的历史时刻开始，之后随实际时间 1:1 向前推进（每次启动实例都会从该时刻重新开始）
                </template>
              </div>
            </el-form-item>
            <el-form-item v-if="localConfig.replay.time_mode === 'absolute'" label="回放起始时间">
              <el-date-picker
                v-model="localConfig.replay.replay_start_time"
                type="datetime"
                value-format="YYYY-MM-DD HH:mm:ss"
                format="YYYY-MM-DD HH:mm:ss"
                placeholder="选择历史时刻（北京时间）"
                style="width:240px"
              />
            </el-form-item>
            <el-row :gutter="12">
              <el-col :span="12">
                <el-form-item label="拉取间隔(秒)">
                  <el-input-number v-model="localConfig.replay.fetch_interval_sec" :min="5" :max="3600" style="width:100%" />
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item label="回溯时长(秒)">
                  <el-input-number v-model="localConfig.replay.lookback_sec" :min="10" :max="86400" style="width:100%" />
                </el-form-item>
              </el-col>
            </el-row>
            <el-row :gutter="12">
              <el-col :span="12">
                <el-form-item label="查询延迟(秒)">
                  <el-input-number v-model="localConfig.replay.delay_sec" :min="0" :max="86400" style="width:100%" />
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item label="目标滞后(秒)">
                  <el-input-number
                    v-model="localConfig.replay.target_lag_sec"
                    :min="0"
                    :max="86400"
                    :disabled="localConfig.replay.time_mode === 'absolute'"
                    style="width:100%"
                  />
                </el-form-item>
              </el-col>
            </el-row>
          </el-form>
        </el-collapse-item>

        <!-- Asset subscriptions -->
        <el-collapse-item name="assets">
          <template #title>资产订阅（{{ localConfig.replay.assets.length }}）</template>
          <div style="margin-bottom:12px">
            <el-button size="small" type="primary" :disabled="disabled || !canEdit" @click="addAsset">+ 添加资产</el-button>
            <span class="hint" style="margin-left:8px">手动填写 EnOS 资产 ID 与测点 ID，保存后在下方「设备映射」中选用</span>
          </div>
          <div v-for="(asset, idx) in localConfig.replay.assets" :key="idx" class="card-row">
            <div class="card-head">
              <span style="font-weight:600">资产 {{ idx + 1 }}</span>
              <el-button size="small" text type="danger" :disabled="disabled || !canEdit" @click="removeAsset(idx)">删除</el-button>
            </div>
            <el-form label-width="80px" :disabled="disabled || !canEdit" size="small">
              <el-form-item label="资产 ID">
                <el-input v-model="asset.asset_id" placeholder="例如 yARGkDuq" style="width:240px" />
              </el-form-item>
              <el-form-item label="测点 ID">
                <div style="display:flex;flex-wrap:wrap;gap:6px;align-items:center">
                  <el-tag
                    v-for="(pid, pidx) in asset.point_ids"
                    :key="pid"
                    :closable="!disabled && canEdit"
                    @close="asset.point_ids.splice(pidx, 1)"
                  >{{ pid }}</el-tag>
                  <el-input
                    v-model="newPointIds[idx]"
                    placeholder="例如 WNAC.TheoryActivePW"
                    style="width:220px"
                    @keyup.enter="addPoint(idx)"
                  />
                  <el-button @click="addPoint(idx)">添加</el-button>
                </div>
              </el-form-item>
            </el-form>
          </div>
          <div v-if="localConfig.replay.assets.length === 0" class="empty">暂无资产订阅</div>
        </el-collapse-item>

        <!-- Device mapping (device.json) -->
        <el-collapse-item name="mapping">
          <template #title>设备映射（{{ mappingRows.length }}）</template>
          <div class="hint" style="margin-bottom:8px">
            把负荷、风机设备的数据源切换为 TSDB，并选择对应的资产与测点（写入设备配置的 mode / replay_* 字段）。其他设备类型暂不支持 TSDB 回放。
          </div>
          <el-table v-if="mappingRows.length" :data="mappingRows" size="small" border>
            <el-table-column label="类型" width="70" prop="type" />
            <el-table-column label="设备" min-width="110">
              <template #default="{ row }">{{ row.dev.DeviceKey || '(未命名)' }} <span class="hint">#{{ row.dev.slave_id }}</span></template>
            </el-table-column>
            <el-table-column label="数据源" width="150">
              <template #default="{ row }">
                <el-tag v-if="row.dev.mode === 3" type="info" size="small">聚合模式</el-tag>
                <el-select v-else v-model="row.dev.mode" size="small" :disabled="disabled || !canEdit" @change="onModeChange(row)">
                  <el-option :value="0" label="本地曲线" />
                  <el-option :value="2" label="TSDB 回放" />
                </el-select>
              </template>
            </el-table-column>
            <el-table-column label="资产" min-width="170">
              <template #default="{ row }">
                <el-select
                  v-if="row.dev.mode === 2"
                  v-model="row.dev.replay_asset_id"
                  size="small"
                  filterable
                  placeholder="选择资产"
                  :disabled="disabled || !canEdit"
                  @change="onAssetChange(row)"
                >
                  <el-option v-for="a in assetOptions" :key="a" :value="a" :label="a" />
                </el-select>
                <span v-else class="hint">—</span>
              </template>
            </el-table-column>
            <el-table-column label="测点" min-width="250">
              <template #default="{ row }">
                <template v-if="row.dev.mode === 2">
                  <el-select
                    v-if="row.type === 'Load'"
                    v-model="row.dev.replay_point_id"
                    size="small"
                    filterable
                    placeholder="有功功率测点"
                    :disabled="disabled || !canEdit"
                  >
                    <el-option v-for="p in pointOptions(row.dev.replay_asset_id)" :key="p" :value="p" :label="p" />
                  </el-select>
                  <div v-else style="display:flex;flex-direction:column;gap:4px">
                    <el-select v-model="row.dev.replay_point_theory" size="small" filterable clearable placeholder="理论功率（默认 WNAC.TheoryActivePW）" :disabled="disabled || !canEdit">
                      <el-option v-for="p in pointOptions(row.dev.replay_asset_id)" :key="p" :value="p" :label="p" />
                    </el-select>
                    <el-select v-model="row.dev.replay_point_wind_speed" size="small" filterable clearable placeholder="风速（默认 WNAC.WindSpeed）" :disabled="disabled || !canEdit">
                      <el-option v-for="p in pointOptions(row.dev.replay_asset_id)" :key="p" :value="p" :label="p" />
                    </el-select>
                  </div>
                </template>
                <span v-else class="hint">—</span>
              </template>
            </el-table-column>
          </el-table>
          <div v-else class="empty">当前配置中没有负荷或风机设备</div>
        </el-collapse-item>
      </el-collapse>

      <div style="display:flex;justify-content:flex-end;margin-top:16px;gap:8px">
        <el-button :disabled="disabled" @click="reload">重置</el-button>
        <el-button type="primary" :disabled="disabled || !canEdit" :loading="saving" @click="handleSave">保存 TSDB 配置</el-button>
      </div>
    </template>
  </el-card>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import {
  getTsdbConfig,
  saveTsdbConfig,
  testTsdbConnection,
  savePyMicrogridConfig,
  type TsdbConfig,
} from '../../api'

const props = defineProps<{
  instanceId: string
  disabled: boolean
  /** device.json content owned by the parent page */
  deviceConfig: any
}>()

const emit = defineEmits<{ (e: 'saved'): void }>()

const REPLAY_TYPES = ['Load', 'Wind']

const activeCollapse = ref(['enos', 'replay', 'assets', 'mapping'])
const showSecret = ref(false)
const loaded = ref(false)
const testing = ref(false)
const saving = ref(false)
const newPointIds = ref<Record<number, string>>({})

function emptyConfig(): TsdbConfig {
  return {
    enos: { apigw_address: '', access_key: '', secret_key: '', org_id: '' },
    replay: {
      enabled: false,
      time_mode: 'relative',
      replay_start_time: '',
      fetch_interval_sec: 30,
      lookback_sec: 300,
      delay_sec: 0,
      target_lag_sec: 0,
      assets: [],
    },
    workspace_ready: false,
    engine_available: true,
  }
}

const localConfig = ref<TsdbConfig>(emptyConfig())
// Working copy of device.json, edited only through the mapping table.
const localDevice = ref<any>(null)

const canEdit = computed(() => !!localConfig.value.workspace_ready)

interface MappingRow { type: string; dev: any }
const mappingRows = computed<MappingRow[]>(() => {
  const rows: MappingRow[] = []
  for (const group of localDevice.value?.Devices ?? []) {
    for (const type of REPLAY_TYPES) {
      for (const dev of group[type] ?? []) rows.push({ type, dev })
    }
  }
  return rows
})

const assetOptions = computed(() =>
  localConfig.value.replay.assets.map(a => a.asset_id.trim()).filter(Boolean),
)

function pointOptions(assetId?: string): string[] {
  const a = localConfig.value.replay.assets.find(x => x.asset_id.trim() === assetId)
  return a ? a.point_ids : []
}

function cloneDevice() {
  localDevice.value = props.deviceConfig ? JSON.parse(JSON.stringify(props.deviceConfig)) : null
}
watch(() => props.deviceConfig, cloneDevice, { immediate: true, deep: false })

async function reload() {
  loaded.value = false
  try {
    const cfg = await getTsdbConfig(props.instanceId)
    cfg.replay.assets = cfg.replay.assets ?? []
    localConfig.value = cfg
  } catch (e: any) {
    localConfig.value = emptyConfig()
    ElMessage.error('加载 TSDB 配置失败: ' + errMsg(e))
  } finally {
    loaded.value = true
    cloneDevice()
  }
}

function errMsg(e: any): string {
  const d = e?.response?.data?.error
  return (typeof d === 'string' ? d : d?.message) || e?.message || '未知错误'
}

function addAsset() {
  localConfig.value.replay.assets.push({ asset_id: '', point_ids: [] })
}
function removeAsset(idx: number) {
  localConfig.value.replay.assets.splice(idx, 1)
}
function addPoint(idx: number) {
  const pid = (newPointIds.value[idx] ?? '').trim()
  if (!pid) return
  const asset = localConfig.value.replay.assets[idx]
  if (!asset.point_ids.includes(pid)) asset.point_ids.push(pid)
  newPointIds.value[idx] = ''
}

function onModeChange(row: MappingRow) {
  if (row.dev.mode === 2 && !row.dev.replay_asset_id && assetOptions.value.length === 1) {
    row.dev.replay_asset_id = assetOptions.value[0]
    onAssetChange(row)
  }
}
function onAssetChange(row: MappingRow) {
  const pts = pointOptions(row.dev.replay_asset_id)
  if (row.type === 'Load' && !pts.includes(row.dev.replay_point_id)) {
    row.dev.replay_point_id = pts.length === 1 ? pts[0] : undefined
  }
}

async function handleTest() {
  testing.value = true
  try {
    const r = await testTsdbConnection(props.instanceId, localConfig.value.enos)
    r.success ? ElMessage.success(r.message) : ElMessage.error(r.message)
  } catch (e: any) {
    ElMessage.error('连通性检测失败: ' + errMsg(e))
  } finally {
    testing.value = false
  }
}

/** Returns an error string when the device mapping is inconsistent, else ''. */
function validateMapping(): string {
  const subscribed = new Set(assetOptions.value)
  for (const { type, dev } of mappingRows.value) {
    if (dev.mode !== 2) continue
    const name = dev.DeviceKey || `#${dev.slave_id}`
    if (!dev.replay_asset_id) return `${type} 设备 ${name} 选择了 TSDB 回放，但未选择资产`
    if (!subscribed.has(dev.replay_asset_id)) return `${type} 设备 ${name} 使用的资产 ${dev.replay_asset_id} 不在订阅列表中`
    if (type === 'Load' && !dev.replay_point_id) return `负荷设备 ${name} 未选择测点`
  }
  return ''
}

async function handleSave() {
  const bad = validateMapping()
  if (bad) { ElMessage.error(bad); return }
  // Drop empty optional keys so device.json stays clean.
  for (const { dev } of mappingRows.value) {
    for (const k of ['replay_point_theory', 'replay_point_wind_speed', 'replay_point_id']) {
      if (dev[k] === '' || dev[k] === undefined || dev[k] === null) delete dev[k]
    }
  }
  const deviceChanged = JSON.stringify(localDevice.value) !== JSON.stringify(props.deviceConfig)

  saving.value = true
  try {
    await saveTsdbConfig(props.instanceId, localConfig.value)
    if (deviceChanged && localDevice.value) {
      await savePyMicrogridConfig(props.instanceId, localDevice.value)
    }
    ElMessage.success('TSDB 配置已保存')
    emit('saved')
    await reload()
  } catch (e: any) {
    ElMessage.error('保存失败: ' + errMsg(e))
  } finally {
    saving.value = false
  }
}

onMounted(reload)
</script>

<style scoped>
.hint { font-size: 12px; color: #6b7280; }
.empty { color: #94a3b8; text-align: center; padding: 16px; }
.card-row { border: 1px solid #e5e7eb; border-radius: 8px; padding: 12px; margin-bottom: 8px; }
.card-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; padding-bottom: 8px; border-bottom: 1px solid #f3f4f6; }
</style>
