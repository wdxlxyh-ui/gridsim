<template>
  <div class="microgrid-editor">
    <!-- Header -->
    <el-card shadow="never" class="header-card">
      <div class="header-row">
        <div class="header-left">
          <el-button @click="goBack" text>
            ← 返回
          </el-button>
          <span style="font-size: 16px; font-weight: 600; margin-left: 8px">{{ instanceName || '微电网' }}</span>
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
                  <div class="dash-label">发电功率</div>
                  <div class="dash-value" style="color:#67c23a">{{ dash.total_generation_kw }} kW</div>
                </div>
                <div class="dash-item">
                  <div class="dash-label">负荷功率</div>
                  <div class="dash-value" style="color:#e6a23c">{{ dash.total_load_kw }} kW</div>
                </div>
                <div class="dash-item">
                  <div class="dash-label">并网功率</div>
                  <div class="dash-value" style="color:#409eff">{{ dash.grid_power_kw }} kW</div>
                </div>
                <div class="dash-item">
                  <div class="dash-label">电池功率</div>
                  <div class="dash-value" style="color:#909399">{{ dash.battery_power_kw ?? '-' }} kW</div>
                </div>
                <div class="dash-item">
                  <div class="dash-label">电池 SOC</div>
                  <div class="dash-value" style="color:#409eff">{{ dash.battery_soc ?? '-' }} %</div>
                </div>
                <div class="dash-item">
                  <div class="dash-label">频率</div>
                  <div class="dash-value">{{ dash.frequency_hz ?? '-' }} Hz</div>
                </div>
              </div>
            </el-card>

            <!-- Vertical Topology SVG -->
            <el-card shadow="never" class="section-card">
              <template #header><span style="font-weight:600">拓扑图</span></template>
              <div class="topology-svg">
                <svg viewBox="0 0 600 480" xmlns="http://www.w3.org/2000/svg">
                  <!-- Grid (top) -->
                  <rect x="230" y="10" width="100" height="44" rx="6"
                    fill="var(--el-color-primary-light-3)" stroke="var(--el-color-primary)" stroke-width="2" />
                  <text x="280" y="37" text-anchor="middle" fill="#fff" font-size="14" font-weight="600">电网</text>

                  <!-- Arrow: Grid → Meter -->
                  <line x1="280" y1="54" x2="280" y2="84" stroke="var(--el-border-color)" stroke-width="2" />
                  <polygon points="274,80 280,90 286,80" fill="var(--el-border-color)" />

                  <!-- Grid Meter -->
                  <rect x="230" y="88" width="100" height="44" rx="6"
                    fill="var(--el-color-warning-light-3)" stroke="var(--el-color-warning)" stroke-width="2" />
                  <text x="280" y="115" text-anchor="middle" fill="#fff" font-size="13" font-weight="600">关口表</text>

                  <!-- Bus: no box, just label to the right of the vertical line -->
                  <text x="295" y="200" fill="var(--el-text-color-primary)" font-size="13" font-weight="600">
                    {{ busName }}
                  </text>
                  <text x="295" y="216" fill="var(--el-text-color-secondary)" font-size="11">
                    {{ busVoltage }} kV
                  </text>

                  <!-- Arrow: Meter → Devices area -->
                  <line x1="280" y1="132" x2="280" y2="240" stroke="var(--el-border-color)" stroke-width="2" stroke-dasharray="6,3" />
                  <polygon points="274,236 280,246 286,236" fill="var(--el-border-color)" />

                  <!-- Horizontal distribution line -->
                  <line x1="100" y1="260" x2="460" y2="260" stroke="var(--el-border-color)" stroke-width="2" />

                  <!-- Vertical drops for each device -->
                  <template v-for="(dev, idx) in visibleDevices" :key="dev.id">
                    <!-- Vertical tap from horizontal line -->
                    <line
                      x1="100 + idx * 120"
                      y1="260"
                      x2="100 + idx * 120"
                      y2="290"
                      stroke="var(--el-border-color)"
                      stroke-width="2"
                    />

                    <!-- Switch circle (clickable when running) -->
                    <g
                      @click="running && handleSwitchToggle(dev.id, !dev.switch.closed)"
                      style="cursor: pointer"
                    >
                      <circle
                        :cx="100 + idx * 120"
                        :cy="306"
                        r="14"
                        :fill="dev.switch.closed ? '#67c23a' : '#f56c6c'"
                        stroke="var(--el-border-color)"
                        stroke-width="1.5"
                      />
                      <line
                        v-if="dev.switch.closed"
                        x1="92" :x2="108" :y1="306" :y2="306"
                        stroke="white" stroke-width="2.5"
                      />
                      <line
                        v-else
                        x1="92" :x2="108" :y1="298" :y2="314"
                        stroke="white" stroke-width="2.5"
                      />
                    </g>

                    <!-- Wire: switch → device box -->
                    <line
                      :x1="100 + idx * 120"
                      y1="320"
                      :x2="100 + idx * 120"
                      y2="342"
                      stroke="var(--el-border-color)"
                      stroke-width="2"
                    />

                    <text
                      :x="100 + idx * 120"
                      y="332"
                      text-anchor="middle"
                      font-size="9"
                      fill="var(--el-text-color-secondary)"
                    >{{ dev.switch.name || 'QF' + (idx + 1) }}</text>

                    <!-- Device box -->
                    <rect
                      :x="60 + idx * 120"
                      y="342"
                      width="80"
                      height="36"
                      rx="6"
                      :fill="devTypeColor(dev.type)"
                      stroke="var(--el-border-color)"
                      stroke-width="1.5"
                    />
                    <text
                      :x="100 + idx * 120"
                      y="358"
                      text-anchor="middle"
                      fill="#fff"
                      font-size="11"
                      font-weight="600"
                    >{{ devTypeLabel(dev.type) }}</text>
                    <text
                      :x="100 + idx * 120"
                      y="372"
                      text-anchor="middle"
                      fill="rgba(255,255,255,0.85)"
                      font-size="9"
                    >{{ dev.name }}</text>
                  </template>

                  <!-- Overflow indicator -->
                  <text
                    v-if="devices.length > maxVisibleDevices"
                    x="280"
                    y="410"
                    text-anchor="middle"
                    font-size="12"
                    fill="var(--el-text-color-secondary)"
                  >
                    ... 还有 {{ devices.length - maxVisibleDevices }} 个设备未显示
                  </text>

                  <!-- Empty state -->
                  <text
                    v-if="devices.length === 0"
                    x="280"
                    y="330"
                    text-anchor="middle"
                    font-size="13"
                    fill="var(--el-text-color-secondary)"
                  >请在左侧添加设备</text>
                </svg>
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
            <el-table-column label="类型" width="90">
              <template #default="{ row }">
                <el-tag size="small" :type="row.point_type === 'AI' ? 'primary' : row.point_type === 'DI' ? 'warning' : 'info'">
                  {{ row.point_type || row.type }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="name" label="名称" min-width="180" />
            <el-table-column label="当前值" width="120">
              <template #default="{ row }">{{ row.value ?? '-' }}</template>
            </el-table-column>
            <el-table-column prop="unit" label="单位" width="80" />
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- Tab 3: 公式配置 -->
      <el-tab-pane label="公式配置" name="formulas">
        <el-card shadow="never">
          <template #header>
            <div style="display:flex;justify-content:space-between;align-items:center">
              <span style="font-weight:600">自定义公式列表</span>
              <div>
                <el-button size="small" @click="fetchFormulas" :loading="loadingFormulas" type="primary" plain>
                  刷新
                </el-button>
                <el-button size="small" type="success" :disabled="running" @click="openAddFormula">
                  + 添加公式
                </el-button>
              </div>
            </div>
          </template>
          <el-table :data="formulas" stripe size="small" max-height="520" empty-text="暂无公式配置" v-loading="loadingFormulas">
            <el-table-column label="启用" width="80">
              <template #default="{ row }">
                <el-switch v-model="row.enabled" size="small" @change="handleFormulaToggle(row)" />
              </template>
            </el-table-column>
            <el-table-column prop="name" label="名称" min-width="150" />
            <el-table-column prop="target" label="目标测点" width="150" />
            <el-table-column prop="expression" label="表达式" min-width="280">
              <template #default="{ row }">
                <code style="font-size:12px;background:var(--el-fill-color);padding:2px 6px;border-radius:4px">{{ row.expression }}</code>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="180" fixed="right">
              <template #default="{ row }">
                <el-button text size="small" :disabled="running" @click="openEditFormula(row)">编辑</el-button>
                <el-button text size="small" type="danger" :disabled="running" @click="handleDeleteFormula(row.id)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- Formula Dialog -->
    <el-dialog v-model="showFormulaDialog" :title="isEditingFormula ? '编辑公式' : '添加公式'" width="560px" destroy-on-close>
      <el-form label-width="90px" size="small">
        <el-form-item label="公式名称">
          <el-input v-model="editingFormula.name" placeholder="例如: 关口表功率" />
        </el-form-item>
        <el-form-item label="目标测点">
          <el-select v-model="editingFormula.target" placeholder="选择或输入目标测点名" allow-create filterable style="width:100%">
            <el-option label="GRID_P（关口有功）" value="GRID_P" />
            <el-option label="GRID_Q（关口无功）" value="GRID_Q" />
            <el-option v-for="p in points" :key="p.name" :label="p.name" :value="p.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="表达式">
          <el-input
            v-model="editingFormula.expression"
            type="textarea"
            :rows="3"
            placeholder="例如: {battery1_Power} + {load1_Power}&#10;支持 {测点名}、+、-、*、/、()"
          />
        </el-form-item>
        <el-form-item>
          <div style="font-size:12px;color:var(--el-text-color-secondary);line-height:1.6">
            <b>语法说明：</b><br>
            • 使用 <code>{测点名}</code> 引用实时值，例如 <code>{battery1_Power}</code>、<code>{GRID_P}</code><br>
            • 支持运算符: <code>+</code> <code>-</code> <code>*</code> <code>/</code> <code>(</code> <code>)</code><br>
            • 示例: <code>{GRID_P} + {battery1_Power}</code> 或 <code>({pv1_Power} + {battery1_Power}) * 0.9</code>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showFormulaDialog = false">取消</el-button>
        <el-button type="primary" @click="handleSaveFormula" :loading="savingFormula">保存</el-button>
      </template>
    </el-dialog>

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
  getMicrogridFormulas,
  addMicrogridFormula,
  updateMicrogridFormula,
  deleteMicrogridFormula,
  type MicrogridTopology,
  type MicrogridDevice,
  type MicrogridDashboard,
  type MicrogridDeviceParams,
  type MicrogridFormula,
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
const dash = ref<MicrogridDashboard>({ total_generation_kw: 0, total_load_kw: 0, grid_power_kw: 0 })
const points = ref<any[]>([])
const loadingPoints = ref(false)

// Formulas
const formulas = ref<MicrogridFormula[]>([])
const showFormulaDialog = ref(false)
const editingFormula = ref<Partial<MicrogridFormula>>({})
const isEditingFormula = ref(false)
const loadingFormulas = ref(false)
const savingFormula = ref(false)

// Add device
const showAddDevice = ref(false)
const addingDevice = ref(false)
const newDeviceType = ref<'pv' | 'battery' | 'load' | 'charger'>('pv')
const newDeviceName = ref('')
const newDeviceParams = ref<MicrogridDeviceParams>({})

// Edit device
const showEditDevice = ref(false)
const updatingDevice = ref(false)
const editingDevice = ref<MicrogridDevice | null>(null)
const editingDeviceName = ref('')
const editingDeviceParams = ref<MicrogridDeviceParams>({})

// Polling
let pollTimer: ReturnType<typeof setInterval> | null = null

// ── Computed ──
const maxVisibleDevices = 3
const visibleDevices = computed(() => devices.value.slice(0, maxVisibleDevices))

const autoFormulas = computed(() => {
  const result: { label: string; expr: string }[] = []
  const pvs = devices.value.filter(d => d.type === 'pv')
  const bats = devices.value.filter(d => d.type === 'battery')
  const loads = devices.value.filter(d => d.type === 'load')
  const chargers = devices.value.filter(d => d.type === 'charger')

  const mkRef = (dev: any) => `{${dev.id}_Power}`
  const plus = (arr: string[]) => arr.join(' + ') || '0'

  if (pvs.length) result.push({ label: '光伏总功率', expr: plus(pvs.map(mkRef)) })
  if (bats.length) {
    result.push({ label: '储能总功率', expr: plus(bats.map(mkRef)) })
    for (const b of bats) {
      result.push({ label: `${b.name} 功率`, expr: `${mkRef(b)} = SETPOINT (±${b.params.rated_power_kw_b || '?'} kW)` })
    }
  }
  if (loads.length) result.push({ label: '负荷总功率', expr: plus(loads.map(mkRef)) })
  if (chargers.length) result.push({ label: '充电桩总功率', expr: plus(chargers.map(mkRef)) })

  const genExpr = [...pvs, ...bats].map(mkRef).join(' + ') || '0'
  const loadExpr = [...loads, ...chargers].map(mkRef).join(' + ') || '0'
  result.push({ label: '关口表功率 (GRID_P)', expr: `(${genExpr}) - (${loadExpr})` })
  result.push({ label: '系统频率', expr: '50.0 + (P_不平衡 / P_发电) × 0.2 Hz' })

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
  await Promise.all([fetchTopology(), fetchInstance(), fetchPoints(), fetchFormulas()])
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
    })
    ElMessage.success('参数已更新')
    showEditDevice.value = false
    topologyChanged.value = true
    await fetchTopology()
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

function resetNewDevice() {
  newDeviceType.value = 'pv'
  newDeviceName.value = ''
  newDeviceParams.value = {}
}

// ── Formula handlers ──

async function fetchFormulas() {
  loadingFormulas.value = true
  try {
    formulas.value = await getMicrogridFormulas(instanceId)
  } catch {
    formulas.value = []
  } finally {
    loadingFormulas.value = false
  }
}

function openAddFormula() {
  editingFormula.value = { name: '', target: '', expression: '', enabled: true }
  isEditingFormula.value = false
  showFormulaDialog.value = true
}

function openEditFormula(f: MicrogridFormula) {
  editingFormula.value = { ...f }
  isEditingFormula.value = true
  showFormulaDialog.value = true
}

async function handleSaveFormula() {
  if (!editingFormula.value.name || !editingFormula.value.expression || !editingFormula.value.target) {
    ElMessage.warning('请填写完整信息')
    return
  }
  savingFormula.value = true
  try {
    if (isEditingFormula.value && editingFormula.value.id) {
      await updateMicrogridFormula(instanceId, editingFormula.value as MicrogridFormula)
    } else {
      await addMicrogridFormula(instanceId, editingFormula.value)
    }
    ElMessage.success('公式已保存')
    showFormulaDialog.value = false
    await fetchFormulas()
  } catch (e: any) {
    ElMessage.error('保存失败: ' + (e?.response?.data?.error || e.message))
  } finally {
    savingFormula.value = false
  }
}

async function handleDeleteFormula(id: string) {
  const f = formulas.value.find(x => x.id === id)
  try {
    await ElMessageBox.confirm(`确定删除公式 "${f?.name || id}"？`, '确认')
    await deleteMicrogridFormula(instanceId, id)
    ElMessage.success('公式已删除')
    await fetchFormulas()
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error('删除失败: ' + (e?.response?.data?.error || e.message))
    }
  }
}

async function handleFormulaToggle(f: MicrogridFormula) {
  try {
    await updateMicrogridFormula(instanceId, f)
  } catch {
    ElMessage.error('更新失败')
  }
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(async () => {
    await fetchDashboard()
  }, 3000)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
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

/* Topology SVG */
.topology-svg {
  width: 100%;
  overflow-x: auto;
}

.topology-svg svg {
  width: 100%;
  min-width: 400px;
  height: 480px;
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
</style>
