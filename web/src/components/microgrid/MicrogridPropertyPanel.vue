<template>
  <transition name="panel-slide">
    <div v-if="visible" class="property-panel-overlay" @click.self="close">
      <div class="property-panel" @click.stop>
        <div class="panel-header">
          <div class="panel-title">
            <el-tag :type="devTypeTag(device?.type || 'pv')" size="small">{{ devTypeLabel(device?.type || 'pv') }}</el-tag>
            <span>{{ device?.name || '' }}</span>
          </div>
          <el-button text size="small" @click="close" style="color:#94a3b8">
            <el-icon><Close /></el-icon>
          </el-button>
        </div>

        <el-form label-width="110px" size="small" class="panel-form">
          <el-form-item label="设备名称">
            <el-input v-model="editName" />
          </el-form-item>
          <el-form-item label="控制模式">
            <el-radio-group v-model="editControlMode">
              <el-radio value="remote">远方(AO跟随)</el-radio>
              <el-radio value="local">本地(策略)</el-radio>
            </el-radio-group>
          </el-form-item>

          <!-- PV Params -->
          <template v-if="device?.type === 'pv'">
            <el-form-item label="额定功率">
              <el-input-number v-model="editParams.rated_power_kw" :min="0" :max="99999" style="width:100%" /> kW
            </el-form-item>
            <el-form-item label="效率">
              <el-input-number v-model="editParams.efficiency" :min="0" :max="1" :step="0.05" style="width:100%" />
            </el-form-item>
          </template>

          <!-- Battery Params -->
          <template v-if="device?.type === 'battery'">
            <el-form-item label="额定容量">
              <el-input-number v-model="editParams.capacity_kwh" :min="0" :max="99999" style="width:100%" /> kWh
            </el-form-item>
            <el-form-item label="额定功率">
              <el-input-number v-model="editParams.rated_power_kw_b" :min="0" :max="99999" style="width:100%" /> kW
            </el-form-item>
            <el-form-item label="初始 SOC">
              <el-input-number v-model="editParams.init_soc" :min="0" :max="100" style="width:100%" /> %
            </el-form-item>
            <el-form-item label="SOC 范围">
              <el-input-number v-model="editParams.soc_min" :min="0" :max="100" style="width:45%" /> % ~
              <el-input-number v-model="editParams.soc_max" :min="0" :max="100" style="width:45%" /> %
            </el-form-item>
          </template>

          <!-- Load Params -->
          <template v-if="device?.type === 'load'">
            <el-form-item label="额定功率">
              <el-input-number v-model="editParams.load_rated_kw" :min="0" :max="99999" style="width:100%" /> kW
            </el-form-item>
            <el-form-item label="功率因数">
              <el-input-number v-model="editParams.power_factor" :min="0" :max="1" :step="0.05" style="width:100%" />
            </el-form-item>
          </template>

          <!-- Charger Params -->
          <template v-if="device?.type === 'charger'">
            <el-form-item label="额定功率">
              <el-input-number v-model="editParams.charger_rated_kw" :min="0" :max="99999" style="width:100%" /> kW
            </el-form-item>
            <el-form-item label="效率">
              <el-input-number v-model="editParams.charger_eff" :min="0" :max="1" :step="0.05" style="width:100%" />
            </el-form-item>
          </template>

          <!-- IOA Base -->
          <el-divider style="margin:12px 0">IOA 地址</el-divider>
          <el-form-item label="IOA 基址">
            <el-tag :type="device?.ioa_base ? 'primary' : 'danger'" size="small" effect="plain" style="font-family:monospace">
              {{ device?.ioa_base ? device.ioa_base + ' ~ ' + (device.ioa_base + 49) : '未分配' }}
            </el-tag>
          </el-form-item>

          <!-- Custom Points -->
          <el-divider style="margin:12px 0">自定义测点 ({{ editCustomPoints.length }})</el-divider>
          <div class="custom-points-list">
            <div v-for="(cp, idx) in editCustomPoints" :key="idx" class="custom-point-row">
              <el-input v-model="cp.name" size="small" placeholder="名称" style="flex:1" />
              <el-input v-model="cp.alias" size="small" placeholder="标识" style="flex:1" />
              <el-select v-model="cp.type" size="small" style="width:80px">
                <el-option value="AI" label="AI" />
                <el-option value="DI" label="DI" />
                <el-option value="AO" label="AO" />
                <el-option value="DO" label="DO" />
              </el-select>
              <el-button size="small" type="danger" text @click="editCustomPoints.splice(idx, 1)">✕</el-button>
            </div>
          </div>
          <el-button size="small" plain @click="editCustomPoints.push({ name: '', type: 'AI', alias: '' })" style="margin-top:4px">
            + 添加测点
          </el-button>
        </el-form>

        <div class="panel-footer">
          <el-button @click="close">取消</el-button>
          <el-button type="primary" @click="handleSave" :loading="saving">保存</el-button>
        </div>
      </div>
    </div>
  </transition>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { Close } from '@element-plus/icons-vue'
import type { MicrogridDevice, MicrogridCustomPoint, MicrogridDeviceParams } from '../../api'

const props = defineProps<{
  visible: boolean
  device: MicrogridDevice | null
  saving: boolean
}>()

const emit = defineEmits<{
  'update:visible': [v: boolean]
  'save': [payload: {
    id: string
    name: string
    type: string
    params: MicrogridDeviceParams
    control_mode: 'remote' | 'local'
    customPoints: MicrogridCustomPoint[]
  }]
}>()

const editName = ref('')
const editControlMode = ref<'remote' | 'local'>('remote')
const editParams = ref<MicrogridDeviceParams>({})
const editCustomPoints = ref<MicrogridCustomPoint[]>([])

watch(() => props.visible, (v) => {
  if (v && props.device) {
    editName.value = props.device.name
    editControlMode.value = props.device.control_mode || 'remote'
    editParams.value = { ...props.device.params }
    editCustomPoints.value = (props.device.custom_points || []).map(cp => ({ ...cp }))
  }
})

function close() {
  emit('update:visible', false)
}

function handleSave() {
  if (!props.device) return
  emit('save', {
    id: props.device.id,
    name: editName.value,
    type: props.device.type,
    params: { ...editParams.value },
    control_mode: editControlMode.value,
    customPoints: editCustomPoints.value,
  })
  emit('update:visible', false)
}

function devTypeLabel(type: string): string {
  const map: Record<string, string> = { pv: '光伏', battery: '储能', load: '负荷', charger: '充电桩' }
  return map[type] || type
}

function devTypeTag(type: string): 'success' | 'primary' | 'warning' | 'info' {
  const map: Record<string, any> = { pv: 'success', battery: 'primary', load: 'warning', charger: 'info' }
  return map[type] || 'info'
}
</script>

<style scoped>
.property-panel-overlay {
  position: fixed;
  inset: 0;
  z-index: 8000;
  background: rgba(0, 0, 0, 0.3);
}

.property-panel {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  width: 560px;
  max-width: 90vw;
  background: var(--bg-card, #141b2d);
  border-left: 1px solid var(--border-color, #2d3a4e);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: -4px 0 24px rgba(0, 0, 0, 0.3);
}

.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-color, #2d3a4e);
  flex-shrink: 0;
}

.panel-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  font-size: 14px;
}

.panel-form {
  flex: 1;
  overflow-y: auto;
  padding: 12px 16px;
}

.panel-form :deep(.el-form-item) {
  margin-bottom: 14px;
}

.custom-points-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 8px;
}

.custom-point-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.panel-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 12px 16px;
  border-top: 1px solid var(--border-color, #2d3a4e);
  flex-shrink: 0;
}

.panel-slide-enter-active,
.panel-slide-leave-active {
  transition: transform 0.28s cubic-bezier(0.22, 1, 0.36, 1);
}
.panel-slide-enter-from,
.panel-slide-leave-to {
  transform: translateX(100%);
}
</style>
