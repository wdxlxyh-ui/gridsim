<template>
  <div class="microgrid-editor">
    <!-- Header -->
    <el-card shadow="never" class="header-card">
      <div class="header-row">
        <div class="header-left">
          <el-button @click="goBack" text>
            ← 返回
          </el-button>
          <span style="font-size: 16px; font-weight: 600; margin-left: 8px">{{ instanceName || '微电网仿真系统' }}</span>
          <el-tag :type="running ? 'success' : 'info'" style="margin-left: 12px">
            {{ running ? '运行中' : '已停止' }}
          </el-tag>
        </div>
        <div class="header-right">
          <el-button v-if="!running" type="success" :loading="actionLoading" @click="handleStart">启动</el-button>
          <el-button v-else type="warning" :loading="actionLoading" @click="handleStop">停止</el-button>
          <el-button
            :disabled="running || !topologyChanged"
            type="primary"
            @click="handleSaveTopology"
          >保存拓扑</el-button>
          <el-button @click="handleExportTopology">导出拓扑</el-button>
          <el-button @click="handleImportTopology" :disabled="running">导入拓扑</el-button>
          <el-button @click="handleExportXLSX">导出点表</el-button>
          <input ref="topoImportRef" type="file" accept=".json" style="display:none" @change="onTopoFile" />
        </div>
      </div>
    </el-card>

    <!-- Tabs -->
    <el-tabs v-model="activeTab" type="border-card">
      <!-- Tab 1: 拓扑配置 -->
      <el-tab-pane label="拓扑配置" name="topology">
        <div class="topology-grid">
          <!-- Left column: config + devices -->
          <div class="topo-left">
            <!-- Grid Meter Config -->
            <el-card shadow="never" class="section-card">
              <template #header><span style="font-weight: 600">关口表配置</span></template>
              <el-form label-width="100px" size="small">
                <el-form-item label="额定容量">
                  <el-input-number v-model="gridMeter.rated_capacity_kw" :min="1" :max="99999" style="width:100%" />
                  <span style="margin-left:6px;font-size:12px;color:var(--el-text-color-secondary)">kW</span>
                </el-form-item>
                <el-form-item label="母线名称">
                  <el-input v-model="busName" placeholder="如: 10kV 母线" />
                </el-form-item>
                <el-form-item label="母线电压">
                  <el-input-number v-model="busVoltage" :min="0.4" :max="220" :step="0.5" style="width:100%" />
                  <span style="margin-left:6px;font-size:12px;color:var(--el-text-color-secondary)">kV</span>
                </el-form-item>
                <el-form-item label="孤岛模式">
                  <el-switch v-model="gridMeter.island_mode" />
                  <span style="margin-left:8px;font-size:12px;color:var(--el-text-color-secondary)">
                    {{ gridMeter.island_mode ? '离网运行' : '并网运行' }}
                  </span>
                </el-form-item>
              </el-form>
            </el-card>

            <!-- Device List -->
            <el-card shadow="never" class="section-card">
              <template #header>
                <div style="display:flex;justify-content:space-between;align-items:center">
                  <span style="font-weight:600">设备列表 ({{ devices.length }})</span>
                  <el-button type="primary" size="small" :disabled="running" @click="showAddDevice = true">
                    + 添加设备
                  </el-button>
                </div>
              </template>
              <div v-if="devices.length === 0" style="text-align:center;color:var(--el-text-color-secondary);padding:20px">
                暂无设备，点击上方按钮添加
              </div>
              <div v-for="dev in devices" :key="dev.id" class="device-card">
                <div class="device-header">
                  <el-tag :type="devTypeTag(dev.type)" size="small">{{ devTypeLabel(dev.type) }}</el-tag>
                  <span style="font-weight:500;margin-left:8px">{{ dev.name }}</span>
                  <el-tag
                    :type="dev.switch.closed ? 'success' : 'danger'"
                    size="small"
                    style="margin-left:auto"
                  >{{ dev.switch.closed ? '合闸' : '分闸' }}</el-tag>
                </div>
                <div class="device-params">
                  <template v-if="dev.type === 'pv'">{{ dev.params.rated_power_kw || '-' }} kW</template>
                  <template v-else-if="dev.type === 'battery'">
                    {{ dev.params.capacity_kwh || '-' }} kWh / {{ dev.params.rated_power_kw_b || '-' }} kW
                  </template>
                  <template v-else-if="dev.type === 'load'">{{ dev.params.load_rated_kw || '-' }} kW</template>
                  <template v-else-if="dev.type === 'charger'">{{ dev.params.charger_rated_kw || '-' }} kW</template>
                </div>
                <div class="device-actions">
                  <el-switch
                    v-model="dev.switch.closed"
                    :disabled="!running"
                    size="small"
                    active-text="合"
                    inactive-text="分"
                    @change="(val: boolean) => handleSwitchToggle(dev.id, val)"
                  />
                  <el-button text size="small" @click="editDevice(dev)">参数</el-button>
                  <el-button text size="small" type="danger" :disabled="running" @click="handleDeleteDevice(dev.id)">删除</el-button>
                </div>
              </div>
            </el-card>
          </div>

          <!-- Right column: dashboard + topology -->
          <div class="topo-right">
            <!-- Dashboard (when running) -->
            <el-card v-if="running" shadow="never" class="section-card">
              <template #header><span style="font-weight:600">实时运行数据</span></template>
              <div class="dashboard-grid">
                <div class="dash-item">
                  <div class="dash-label">并网点功率</div>
                  <div class="dash-value" :style="{ color: (dash.grid_power_kw ?? 0) >= 0 ? '#e6a23c' : '#67c23a' }">{{ dash.grid_power_kw ?? '-' }} kW</div>
                </div>
                <div class="dash-item">
                  <div class="dash-label">光伏总功率</div>
                  <div class="dash-value" style="color:#67c23a">{{ dash.total_pv_kw ?? '-' }} kW</div>
                </div>
                <div class="dash-item">
                  <div class="dash-label">负荷总功率</div>
                  <div class="dash-value" style="color:#e6a23c">{{ (dash.total_load_kw ?? 0) + (dash.total_charger_kw ?? 0) }} kW</div>
                </div>
              </div>
              <div style="font-size:11px;margin-top:6px;color:#909399;line-height:1.7;display:flex;flex-wrap:wrap;gap:4px 16px">
                <template v-for="p in (dash.pv || [])" :key="p.id">
                  <span>☀️ {{ p.name }}: <strong :style="{color: p.closed ? '#67c23a' : '#c0c4cc'}">{{ p.closed ? p.power_kw+' kW' : '已断开' }}</strong></span>
                </template>
                <template v-for="b in (dash.battery || [])" :key="b.id">
                  <span>🔋 {{ b.name }}: <strong :style="{color:'#409eff'}">{{ b.power_kw }} kW</strong> (SOC {{ b.soc }}%)</span>
                </template>
                <template v-for="l in (dash.load || [])" :key="l.id">
                  <span>💡 {{ l.name }}: <strong :style="{color: l.closed ? '#e6a23c' : '#c0c4cc'}">{{ l.closed ? l.power_kw+' kW' : '已断开' }}</strong></span>
                </template>
                <template v-for="c in (dash.charger || [])" :key="c.id">
                  <span>🔌 {{ c.name }}: <strong :style="{color: c.closed ? '#909399' : '#c0c4cc'}">{{ c.closed ? c.power_kw+' kW' : '已断开' }}</strong></span>
                </template>
              </div>
            </el-card>

            <!-- Vertical Topology SVG -->
            <el-card shadow="never" class="section-card">
              <template #header><span style="font-weight:600">拓扑图</span></template>
              <div class="topology-wrap">
                <div v-html="svgTopology" :key="'svg-' + devices.length + '-' + running" class="topology-html"></div>
              </div>
            </el-card>

            <!-- Auto-generated formula preview -->
            <el-card v-if="devices.length > 0" shadow="never" class="section-card">
              <template #header><span style="font-weight:600">公式预览（自动生成）</span></template>
              <div class="formula-preview">
                <div v-for="f in autoFormulas" :key="f.label" class="formula-row">
                  <span class="formula-label">{{ f.label }}</span>
                  <code class="formula-expr">{{ f.expr }}</code>
                </div>
              </div>
            </el-card>
          </div>
        </div>
      </el-tab-pane>

      <!-- Tab 2: 测点管理 -->
      <el-tab-pane label="测点管理" name="points">
        <el-card shadow="never">
          <template #header>
            <div style="display:flex;justify-content:space-between;align-items:center">
              <span style="font-weight:600">IEC104 测点列表</span>
              <el-button size="small" @click="fetchPoints" :loading="loadingPoints" type="primary" plain>
                刷新
              </el-button>
            </div>
          </template>
          <el-table :data="points" stripe size="small" max-height="520" v-loading="loadingPoints" empty-text="暂无测点数据">
            <el-table-column prop="ioa" label="IOA" width="90" />
            <el-table-column label="类型" width="80">
              <template #default="{ row }">
                <el-tag size="small" :type="row.point_type === 'AI' ? 'primary' : 'warning'" effect="plain">{{ row.point_type }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="名称" min-width="160">
              <template #default="{ row }">{{ row.name }}</template>
            </el-table-column>
            <el-table-column label="当前值" width="100">
              <template #default="{ row }">{{ row.value ?? '-' }}</template>
            </el-table-column>
            <el-table-column label="控制模式" width="120">
              <template #default="{ row }">
                <span v-if="!row.can_toggle" style="font-size:11px;color:#c0c4cc">—</span>
                <el-switch v-else v-model="row.local_mode" size="small"
                  active-text="本地" inactive-text="远方"
                  @change="(val: boolean) => togglePointMode(row, val)" />
              </template>
            </el-table-column>
            <el-table-column label="策略" width="100" fixed="right">
              <template #default="{ row }">
                <el-button v-if="row.can_toggle && row.local_mode" size="small" text type="warning"
                  @click="configPointStrategy(row)">配置策略</el-button>
                <span v-else-if="row.can_toggle" style="font-size:11px;color:#909399">引擎控制</span>
                <span v-else style="font-size:11px;color:#c0c4cc">—</span>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- Add Device Dialog -->
    <el-dialog v-model="showAddDevice" title="添加设备" width="520px" destroy-on-close>
      <el-form label-width="110px" size="small">
        <el-form-item label="设备类型">
          <el-radio-group v-model="newDeviceType">
            <el-radio-button value="pv">光伏</el-radio-button>
            <el-radio-button value="battery">储能</el-radio-button>
            <el-radio-button value="load">负荷</el-radio-button>
            <el-radio-button value="charger">充电桩</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="设备名称">
          <el-input v-model="newDeviceName" placeholder="例如: PV-1" />
        </el-form-item>
        <!-- PV params -->
        <template v-if="newDeviceType === 'pv'">
          <el-form-item label="额定功率">
            <el-input-number v-model="newDeviceParams.rated_power_kw" :min="0" :max="99999" style="width:100%" /> kW
          </el-form-item>
          <el-form-item label="效率">
            <el-input-number v-model="newDeviceParams.efficiency" :min="0" :max="1" :step="0.05" style="width:100%" />
          </el-form-item>
        </template>
        <!-- Battery params -->
        <template v-if="newDeviceType === 'battery'">
          <el-form-item label="额定容量">
            <el-input-number v-model="newDeviceParams.capacity_kwh" :min="0" :max="99999" style="width:100%" /> kWh
          </el-form-item>
          <el-form-item label="额定功率">
            <el-input-number v-model="newDeviceParams.rated_power_kw_b" :min="0" :max="99999" style="width:100%" /> kW
          </el-form-item>
          <el-form-item label="初始 SOC">
            <el-input-number v-model="newDeviceParams.init_soc" :min="0" :max="100" style="width:100%" /> %
          </el-form-item>
          <el-form-item label="SOC 范围">
            <el-input-number v-model="newDeviceParams.soc_min" :min="0" :max="100" style="width:45%" /> %
            ~
            <el-input-number v-model="newDeviceParams.soc_max" :min="0" :max="100" style="width:45%" /> %
          </el-form-item>
        </template>
        <!-- Load params -->
        <template v-if="newDeviceType === 'load'">
          <el-form-item label="额定功率">
            <el-input-number v-model="newDeviceParams.load_rated_kw" :min="0" :max="99999" style="width:100%" /> kW
          </el-form-item>
          <el-form-item label="功率因数">
            <el-input-number v-model="newDeviceParams.power_factor" :min="0" :max="1" :step="0.05" style="width:100%" />
          </el-form-item>
        </template>
        <!-- Charger params -->
        <template v-if="newDeviceType === 'charger'">
          <el-form-item label="额定功率">
            <el-input-number v-model="newDeviceParams.charger_rated_kw" :min="0" :max="99999" style="width:100%" /> kW
          </el-form-item>
          <el-form-item label="效率">
            <el-input-number v-model="newDeviceParams.charger_eff" :min="0" :max="1" :step="0.05" style="width:100%" />
          </el-form-item>
        </template>
      </el-form>
      <template #footer>
        <el-button @click="showAddDevice = false">取消</el-button>
        <el-button type="primary" @click="handleAddDevice" :loading="addingDevice">添加</el-button>
      </template>
    </el-dialog>

    <!-- Edit Device Params Dialog -->
    <el-dialog v-model="showEditDevice" title="编辑设备参数" width="520px" destroy-on-close>
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
          <el-form-item label="额定功率">
            <el-input-number v-model="editingDeviceParams.rated_power_kw" :min="0" :max="99999" style="width:100%" /> kW
          </el-form-item>
          <el-form-item label="效率">
            <el-input-number v-model="editingDeviceParams.efficiency" :min="0" :max="1" :step="0.05" style="width:100%" />
          </el-form-item>
        </template>
        <template v-if="editingDevice?.type === 'battery'">
          <el-form-item label="额定容量">
            <el-input-number v-model="editingDeviceParams.capacity_kwh" :min="0" :max="99999" style="width:100%" /> kWh
          </el-form-item>
          <el-form-item label="额定功率">
            <el-input-number v-model="editingDeviceParams.rated_power_kw_b" :min="0" :max="99999" style="width:100%" /> kW
          </el-form-item>
          <el-form-item label="初始 SOC">
            <el-input-number v-model="editingDeviceParams.init_soc" :min="0" :max="100" style="width:100%" /> %
          </el-form-item>
          <el-form-item label="SOC 范围">
            <el-input-number v-model="editingDeviceParams.soc_min" :min="0" :max="100" style="width:45%" /> %
            ~
            <el-input-number v-model="editingDeviceParams.soc_max" :min="0" :max="100" style="width:45%" /> %
          </el-form-item>
        </template>
        <template v-if="editingDevice?.type === 'load'">
          <el-form-item label="额定功率">
            <el-input-number v-model="editingDeviceParams.load_rated_kw" :min="0" :max="99999" style="width:100%" /> kW
          </el-form-item>
          <el-form-item label="功率因数">
            <el-input-number v-model="editingDeviceParams.power_factor" :min="0" :max="1" :step="0.05" style="width:100%" />
          </el-form-item>
        </template>
        <template v-if="editingDevice?.type === 'charger'">
          <el-form-item label="额定功率">
            <el-input-number v-model="editingDeviceParams.charger_rated_kw" :min="0" :max="99999" style="width:100%" /> kW
          </el-form-item>
          <el-form-item label="效率">
            <el-input-number v-model="editingDeviceParams.charger_eff" :min="0" :max="1" :step="0.05" style="width:100%" />
          </el-form-item>
        </template>
      </el-form>
      <template #footer>
        <el-button @click="showEditDevice = false">取消</el-button>
        <el-button type="primary" @click="handleUpdateDevice" :loading="updatingDevice">保存</el-button>
      </template>
    </el-dialog>

    <!-- Strategy Config Dialog -->
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
        <el-button type="primary" @click="confirmStrategy" :loading="savingStrategy">确认策略</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
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
} from '../api'

const route = useRoute()
const router = useRouter()
const instanceId = route.params.id as string

// ── State ──
const activeTab = ref('topology')
const instanceName = ref('')
const running = ref(false)
const actionLoading = ref(false)
const topologyChanged = ref(false)

const busName = ref('10kV 母线')
const busVoltage = ref(10)
const gridMeter = ref({ rated_capacity_kw: 500, island_mode: false })
const devices = ref<MicrogridDevice[]>([])
const dash = ref<MicrogridDashboard>({ grid_power_kw: 0, total_pv_kw: 0, total_bat_kw: 0, total_load_kw: 0, total_charger_kw: 0 })
const points = ref<any[]>([])
const loadingPoints = ref(false)

// Add device
const showAddDevice = ref(false)
const addingDevice = ref(false)
const newDeviceType = ref<'pv' | 'battery' | 'load' | 'charger'>('pv')
const newDeviceName = ref('')
const newDeviceParams = ref<MicrogridDeviceParams>({})

// Edit device
const showEditDevice = ref(false)
const topoImportRef = ref<HTMLInputElement>()
const updatingDevice = ref(false)
const editingDevice = ref<MicrogridDevice | null>(null)
const editingDeviceName = ref('')
const editingDeviceParams = ref<MicrogridDeviceParams>({})
const editingControlMode = ref<'remote' | 'local'>('remote')

// Strategy dialog
const showStrategyDialog = ref(false)
const savingStrategy = ref(false)
const strategyTab = ref('increment')
const strategyPointIOA = ref(0)
const strategyTargetName = ref('')
const csvFileList = ref<any[]>([])
const strategyForm = ref<any>({
  start_value: 0, step: 1, period_ms: 1000, max_value: 100,
  min_value: 0, max_value_r: 100, decimal_places: 0,
  csv_file: '', time_format: 'relative', time_unit: 'ms', csv_loop: true,
  linked_ioas: '', linked_ioa: '', capacity: 0, init_soc: 50,
  soc_min: 10, soc_max: 90, pulse_energy: 1,
})

// Polling
let pollTimer: ReturnType<typeof setInterval> | null = null
let pointsTimer: ReturnType<typeof setInterval> | null = null

// ── Computed ──

function devPower(id: string): number {
  for (const arr of [dash.value.pv, dash.value.battery, dash.value.load, dash.value.charger]) {
    if (!arr) continue
    const found = arr.find((d: any) => d.id === id)
    if (found) return found.power_kw ?? 0
  }
  return 0
}

const svgTopology = computed(() => {
  const N = devices.value.length
  if (N === 0) return ''
  const BUS_Y = 260, MIN_GAP = 120, ROW_H = 126
  const W = Math.max(680, N * MIN_GAP + 80)
  const H = BUS_Y + N * ROW_H + 60
  const cx = W / 2
  const sp = (W - 80) / Math.max(N, 1)
  const sx = cx - (sp * (N - 1)) / 2
  const minX = sx - 20
  const maxX = sx + (N - 1) * sp + 20
  const swY = BUS_Y + 50, swR = 12, boxT = BUS_Y + 90

  // Compute per-device power from dashboard
  const powerMap = new Map<string, number>()
  for (const arr of [dash.value.pv, dash.value.battery, dash.value.load, dash.value.charger]) {
    if (!arr) continue
    for (const d of arr) powerMap.set(d.id, d.power_kw ?? 0)
  }

  function flowClass(dev: any): string {
    if (!dev.switch.closed) return 'fz'
    const p = powerMap.get(dev.id) ?? 0
    if (dev.type === 'pv') return p > 0.1 ? 'fl-up' : 'fz'
    if (dev.type === 'battery') return p > 0.1 ? 'fl-dn' : (p < -0.1 ? 'fl-up' : 'fz')
    return p > 0.1 ? 'fl-dn' : 'fz'
  }

  function makeFlow(d: any) { return flowClass(d) }

  let rows = ''
  devices.value.forEach((dev: any, idx: number) => {
    const dx = sx + idx * sp
    const cl = dev.switch.closed
    const t = dev.type
    const fc = makeFlow(dev)
    const lc = cl ? (FC as any)[t] : '#c0c4cc'
    const pval = (powerMap.get(dev.id) ?? 0).toFixed(1)

    rows += `<line x1="${dx}" y1="${BUS_Y}" x2="${dx}" y2="${swY - swR}" stroke="${cl ? lc : '#c0c4cc'}" stroke-width="3.5" stroke-linecap="round" class="${fc}"/>`
    const sf = cl ? (t === 'pv' ? '#e8f5e9' : t === 'battery' ? '#e3f2fd' : '#fff3e0') : '#fef0f0'
    rows += `<circle cx="${dx}" cy="${swY}" r="${swR}" fill="${sf}" stroke="${cl ? '#67c23a' : '#f56c6c'}" stroke-width="2" style="cursor:pointer"/>`
    rows += cl
      ? `<line x1="${dx - 7}" y1="${swY}" x2="${dx + 7}" y2="${swY}" stroke="#67c23a" stroke-width="2" stroke-linecap="round"/>`
      : `<line x1="${dx - 6}" y1="${swY - 6}" x2="${dx + 6}" y2="${swY + 6}" stroke="#f56c6c" stroke-width="2" stroke-linecap="round"/>`
    rows += `<line x1="${dx}" y1="${swY + swR}" x2="${dx}" y2="${boxT}" stroke="${cl ? lc : '#c0c4cc'}" stroke-width="3.5" stroke-linecap="round" class="${fc}"/>`
    rows += `<text x="${dx}" y="${swY + swR + 13}" text-anchor="middle" font-size="9" fill="#909399">${dev.switch.name || 'QF' + (idx + 1)}</text>`
    rows += `<rect x="${dx - 46}" y="${boxT}" width="92" height="34" rx="6" fill="${cl ? (FC as any)[t] : '#e0e0e0'}" stroke="${lc}" stroke-width="1.5" opacity="${cl ? 1 : 0.5}"/>`
    rows += `<text x="${dx}" y="${boxT + 12}" text-anchor="middle" font-size="12" font-weight="700" fill="${cl ? '#fff' : '#999'}">${(LB as any)[t]}</text>`
    rows += `<text x="${dx}" y="${boxT + 26}" text-anchor="middle" font-size="10" fill="${cl ? 'rgba(255,255,255,0.9)' : '#999'}">${dev.name}</text>`
    if (cl) {
      rows += `<rect x="${dx - 40}" y="${boxT + 38}" width="80" height="18" rx="4" fill="${lc}" opacity="0.1"/>`
      rows += `<text x="${dx}" y="${boxT + 50}" text-anchor="middle" font-size="11" font-weight="700" style="font-family:monospace" fill="${lc}">${pval} kW</text>`
    } else {
      rows += `<text x="${dx}" y="${boxT + 44}" text-anchor="middle" font-size="10" fill="#c0c4cc">已断开</text>`
    }
  })

  // Summary from dash
  const PV = (dash.value.pv || []).filter((d: any) => d.closed).reduce((s: number, d: any) => s + (d.power_kw ?? 0), 0)
  const LD = (dash.value.load || []).filter((d: any) => d.closed).reduce((s: number, d: any) => s + (d.power_kw ?? 0), 0)
  const CH = (dash.value.charger || []).filter((d: any) => d.closed).reduce((s: number, d: any) => s + (d.power_kw ?? 0), 0)
  const BAT = (dash.value.battery || []).filter((d: any) => d.closed).reduce((s: number, d: any) => s + (d.power_kw ?? 0), 0)
  const GRID = LD + CH + BAT - PV
  const tFlow = GRID > 0.1 ? 'fl-dn' : (GRID < -0.1 ? 'fl-up' : 'fz')
  const svgW = W.toString(), svgH = H.toString()
  return `<svg viewBox="0 0 ${svgW} ${svgH}" width="100%" xmlns="http://www.w3.org/2000/svg">
<rect x="0" y="0" width="${svgW}" height="${svgH}" fill="#f5f7fa"/>
<rect x="${cx - 56}" y="12" width="112" height="38" rx="6" fill="#fef0f0" stroke="#f89898" stroke-width="1.5"/>
<text x="${cx}" y="36" text-anchor="middle" font-size="14" font-weight="700" fill="#e63946">⚡ 电网</text>
<line x1="${cx}" y1="50" x2="${cx}" y2="78" stroke="#bbb" stroke-width="3.5" stroke-linecap="round" class="${tFlow}"/>
<rect x="${cx - 56}" y="80" width="112" height="38" rx="6" fill="#fef7e0" stroke="#e8c560" stroke-width="1.5"/>
<text x="${cx}" y="104" text-anchor="middle" font-size="14" font-weight="700" fill="#b8860b">🔌 关口表</text>
<line x1="${cx}" y1="118" x2="${cx}" y2="${BUS_Y}" stroke="#bbb" stroke-width="3.5" stroke-linecap="round" class="${tFlow}"/>
<text x="${cx + 20}" y="${BUS_Y - 45}" font-size="13" font-weight="700" fill="#303133">10kV 母线</text>
<text x="${cx + 20}" y="${BUS_Y - 30}" font-size="11" fill="#909399">0.4 ~ 220 kV</text>
<line x1="${minX}" y1="${BUS_Y}" x2="${maxX}" y2="${BUS_Y}" stroke="#555" stroke-width="3" stroke-linecap="round"/>
${rows}
</svg>`
})

const LB: Record<string, string> = { pv: '光伏', battery: '储能', load: '负荷', charger: '充电桩' }
const FC: Record<string, string> = { pv: '#67c23a', battery: '#409eff', load: '#e6a23c', charger: '#909399' }

const autoFormulas = computed(() => {
  const result: { label: string; expr: string }[] = []
  const active = (d: any) => d.switch.closed
  const pvs = devices.value.filter(d => d.type === 'pv')
  const bats = devices.value.filter(d => d.type === 'battery')
  const loads = devices.value.filter(d => d.type === 'load')
  const chargers = devices.value.filter(d => d.type === 'charger')
  const mkRef = (dev: any) => `{${dev.id}_Power}`
  const plus = (arr: string[]) => arr.join(' + ') || '0'
  const activeRef = (arr: any[]) => plus(arr.filter(active).map(mkRef))
  const activeName = (dev: any) => `${dev.name} (${dev.switch.closed ? '合' : '断'})`

  if (pvs.length) {
    const a = pvs.filter(active)
    result.push({ label: '光伏总功率', expr: a.length ? activeRef(pvs) : '0 (全部断开)' })
    for (const d of pvs) result.push({ label: `  ${activeName(d)}`, expr: active(d) ? `${mkRef(d)} = SETPOINT ∈ [0, ${d.params.rated_power_kw || '?'}]` : '0 (断路)' })
  }
  if (bats.length) {
    const a = bats.filter(active)
    result.push({ label: '储能总功率', expr: a.length ? activeRef(bats) : '0 (全部断开)' })
    for (const b of bats) {
      if (active(b)) {
        result.push({ label: `  ${activeName(b)}`, expr: `${mkRef(b)} = SETPOINT (±${b.params.rated_power_kw_b || '?'} kW, +充电 −放电)` })
      } else { result.push({ label: `  ${activeName(b)}`, expr: '0 (断路)' }) }
    }
  }
  if (loads.length) {
    const a = loads.filter(active)
    result.push({ label: '负荷总功率', expr: a.length ? activeRef(loads) : '0 (全部断开)' })
    for (const d of loads) { if (!active(d)) result.push({ label: `  ${activeName(d)}`, expr: '0 (断路)' }) }
  }
  if (chargers.length) {
    const a = chargers.filter(active)
    result.push({ label: '充电桩总功率', expr: a.length ? activeRef(chargers) : '0 (全部断开)' })
    for (const d of chargers) { if (!active(d)) result.push({ label: `  ${activeName(d)}`, expr: '0 (断路)' }) }
  }

  const genExpr = [...pvs.filter(active)].map(mkRef).join(' + ') || '0'
  const loadExpr = [...loads.filter(active), ...chargers.filter(active), ...bats.filter(active)].map(mkRef).join(' + ') || '0'
  result.push({ label: '关口表功率 (GRID_P)', expr: `(${loadExpr}) − (${genExpr})` })

  return result
})

// ── Helpers ──
function devTypeLabel(type: string): string {
  const map: Record<string, string> = { pv: '光伏', battery: '储能', load: '负荷', charger: '充电桩' }
  return map[type] || type
}

function devTypeColor(type: string): string {
  const map: Record<string, string> = {
    pv: '#67c23a',
    battery: '#409eff',
    load: '#e6a23c',
    charger: '#909399',
  }
  return map[type] || '#909399'
}

function devTypeTag(type: string): 'success' | 'primary' | 'warning' | 'info' {
  const map: Record<string, any> = { pv: 'success', battery: 'primary', load: 'warning', charger: 'info' }
  return map[type] || 'info'
}

function goBack() {
  router.push('/config')
}

// ── Data Loading ──
async function fetchTopology() {
  try {
    const topo = await getMicrogridTopology(instanceId)
    busName.value = topo.bus_name
    busVoltage.value = topo.bus_voltage_kv
    gridMeter.value = { ...topo.grid_meter }
    devices.value = topo.devices || []
    topologyChanged.value = false
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
  try {
    dash.value = await getMicrogridDashboard(instanceId)
  } catch {}
}

async function fetchPoints() {
  loadingPoints.value = true
  try {
    const data = await getMicrogridPoints(instanceId)
    points.value = data.points || []
  } catch {} finally {
    loadingPoints.value = false
  }
}

async function loadAll() {
  await Promise.all([fetchTopology(), fetchInstance(), fetchPoints()])
  if (running.value) {
    await fetchDashboard()
  }
}

// ── Actions ──
async function handleStart() {
  // Save topology first
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
  } finally {
    actionLoading.value = false
  }
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
  } finally {
    actionLoading.value = false
  }
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
  const topo = { bus_name: busName.value, bus_voltage_kv: busVoltage.value, grid_meter: { ...gridMeter.value }, devices: devices.value }
  const blob = new Blob([JSON.stringify(topo, null, 2)], { type: 'application/json' })
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
    ElMessage.success('拓扑已加载，请点击保存拓扑')
  } catch (e: any) { ElMessage.error('导入失败: ' + (e?.message || '格式错误')) }
}

function handleExportXLSX() {
  window.open(`/api/v1/microgrid/${instanceId}/export-xlsx`, '_blank')
}

async function handleAddDevice() {
  if (!newDeviceName.value) {
    ElMessage.warning('请输入设备名称')
    return
  }
  addingDevice.value = true
  try {
    await addMicrogridDevice(instanceId, {
      type: newDeviceType.value,
      name: newDeviceName.value,
      params: { ...newDeviceParams.value },
    })
    ElMessage.success('设备已添加')
    showAddDevice.value = false
    topologyChanged.value = true
    await fetchTopology()
    resetNewDevice()
  } catch (e: any) {
    ElMessage.error('添加失败: ' + (e?.response?.data?.error || e.message))
  } finally {
    addingDevice.value = false
  }
}

function editDevice(dev: MicrogridDevice) {
  editingDevice.value = dev
  editingDeviceName.value = dev.name
  editingDeviceParams.value = { ...dev.params }
  editingControlMode.value = dev.control_mode || 'remote'
  showEditDevice.value = true
}

async function handleUpdateDevice() {
  if (!editingDevice.value) return
  updatingDevice.value = true
  try {
    await updateMicrogridDevice(instanceId, {
      ...editingDevice.value,
      name: editingDeviceName.value,
      params: { ...editingDeviceParams.value },
      control_mode: editingControlMode.value,
    })
    ElMessage.success('参数已更新')
    showEditDevice.value = false
    topologyChanged.value = true
    await fetchTopology()
    await fetchInstance()
  } catch (e: any) {
    ElMessage.error('更新失败: ' + (e?.response?.data?.error || e.message))
  } finally {
    updatingDevice.value = false
  }
}

async function handleDeleteDevice(devId: string) {
  try {
    await ElMessageBox.confirm('确定删除此设备？', '确认')
    await deleteMicrogridDevice(instanceId, devId)
    ElMessage.success('设备已删除')
    topologyChanged.value = true
    await fetchTopology()
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error('删除失败: ' + (e?.response?.data?.error || e.message))
    }
  }
}

async function handleSwitchToggle(devId: string, closed: boolean) {
  try {
    await controlMicrogridSwitch(instanceId, devId, closed)
  } catch (e: any) {
    ElMessage.error('开关操作失败: ' + (e?.response?.data?.error || e.message))
  }
}

// ── Strategy handlers ──
async function togglePointMode(row: any, local: boolean) {
  if (local) {
    // Switched to local: open strategy dialog
    configPointStrategy(row)
  } else {
    // Switched to remote: delete auto-change strategy
    try {
      await deleteAutoChange(instanceId, row.ioa)
      ElMessage.success('已切换为远方控制')
      await fetchPoints()
    } catch (e: any) {
      ElMessage.error('切换失败: ' + (e?.response?.data?.error || e.message))
    }
  }
}

function configPointStrategy(row: any) {
  strategyPointIOA.value = row.ioa
  strategyTargetName.value = row.name
  strategyTab.value = 'increment'
  showStrategyDialog.value = true
}

async function confirmStrategy() {
  savingStrategy.value = true
  try {
    if (strategyPointIOA.value > 0) {
      await setAutoChange(instanceId, strategyPointIOA.value, {
        strategy: strategyTab.value, enabled: true, params: { ...strategyForm.value },
      })
      ElMessage.success('策略已保存')
      showStrategyDialog.value = false
      await fetchPoints()
    }
  } catch (e: any) {
    ElMessage.error('策略保存失败: ' + (e?.response?.data?.error || e.message))
  } finally { savingStrategy.value = false }
}

function resetNewDevice() {
  newDeviceType.value = 'pv'
  newDeviceName.value = ''
  newDeviceParams.value = {}
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(async () => { await fetchDashboard() }, 3000)
  pointsTimer = setInterval(async () => { await fetchPoints() }, 1000)
}

function stopPolling() {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
  if (pointsTimer) { clearInterval(pointsTimer); pointsTimer = null }
}

onMounted(async () => {
  await loadAll()
  if (running.value) startPolling()
})

onUnmounted(() => {
  stopPolling()
})
</script>

<style scoped>
.microgrid-editor {
  padding: 16px;
  max-width: 1200px;
  margin: 0 auto;
}

.header-card {
  margin-bottom: 12px;
}

.header-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.header-left {
  display: flex;
  align-items: center;
}

/* Topology tab 2-column layout */
.topology-grid {
  display: grid;
  grid-template-columns: 380px 1fr;
  gap: 16px;
}

@media (max-width: 900px) {
  .topology-grid {
    grid-template-columns: 1fr;
  }
}

.topo-left {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.topo-right {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.section-card {
  margin-bottom: 0;
}

/* Device card */
.device-card {
  border: 1px solid var(--el-border-color);
  border-radius: 8px;
  padding: 10px 12px;
  margin-bottom: 8px;
  background: var(--el-bg-color);
  transition: box-shadow 0.2s;
}

.device-card:hover {
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
}

.device-header {
  display: flex;
  align-items: center;
  margin-bottom: 4px;
}

.device-params {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-bottom: 6px;
}

.device-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

/* Dashboard */
.dashboard-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
}

.dash-item {
  text-align: center;
  padding: 10px;
  background: var(--el-fill-color);
  border-radius: 8px;
}

.dash-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-bottom: 2px;
}

.dash-value {
  font-size: 18px;
  font-weight: 700;
}

/* Formula preview */
.formula-preview {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.formula-row {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 12px;
}
.formula-label {
  min-width: 110px;
  color: var(--el-text-color-secondary);
}
.formula-expr {
  background: var(--el-fill-color);
  padding: 3px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-family: 'SF Mono', 'Menlo', monospace;
  color: var(--el-color-primary);
}

/* Tab pane spacing */
.el-tabs :deep(.el-tabs__content) {
  padding: 16px;
}

/* ═══ SVG Topology ═══ */
.topology-wrap {
  overflow: auto;
  max-height: 560px;
  background: var(--el-bg-color-page);
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
}
.topology-html {
  display: block;
  font-family: system-ui, -apple-system, sans-serif;
}
.topology-html :deep(svg) {
  display: block;
  width: 100%;
  height: auto;
}
.topology-html :deep(text) {
  font-family: system-ui, -apple-system, sans-serif;
}

/* Flow animation: energy beam (style #3) */
@keyframes flow-up { to { stroke-dashoffset: 32; } }
@keyframes flow-dn { to { stroke-dashoffset: -32; } }
.topology-html :deep(.fl-up) {
  stroke-dasharray: 12 4;
  animation: flow-up .6s linear infinite;
  stroke-width: 3.5;
}
.topology-html :deep(.fl-dn) {
  stroke-dasharray: 12 4;
  animation: flow-dn .6s linear infinite;
  stroke-width: 3.5;
}
.topology-html :deep(.fz) {
  stroke-dasharray: 4 8;
  stroke: #c0c4cc !important;
  stroke-width: 2;
}

/* Managed row in points table */
</style>
