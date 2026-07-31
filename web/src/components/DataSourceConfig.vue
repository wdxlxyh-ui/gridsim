<template>
  <div class="data-source-config">
    <el-form-item label="数据源类型">
      <el-select v-model="modelValue.kind" @change="handleSourceTypeChange">
        <el-option
          v-for="option in sourceOptions"
          :key="option.value"
          :label="option.label"
          :value="option.value"
        />
      </el-select>
    </el-form-item>

    <!-- EnOS 数据源配置 -->
    <template v-if="modelValue.kind === 'enos' || modelValue.kind === 'enos_aggregate'">
      <el-form-item label="EnOS 凭据" v-if="!hideCredential">
        <el-select v-model="selectedCredentialRef" placeholder="选择凭据档案">
          <el-option
            v-for="cred in credentials"
            :key="cred.id"
            :label="cred.name"
            :value="cred.id"
          />
        </el-select>
      </el-form-item>

      <template v-if="modelValue.kind === 'enos'">
        <el-form-item label="资产 ID">
          <el-input
            v-model="enosSource.asset_id"
            placeholder="输入EnOS资产ID"
            @input="updateEnOSSource"
          />
        </el-form-item>

        <el-form-item v-if="needsPowerPoint" label="功率测点">
          <el-input
            v-model="enosSource.power_point"
            :placeholder="getPowerPointPlaceholder()"
            @input="updateEnOSSource"
          />
        </el-form-item>

        <el-form-item v-if="deviceType === 'bess'" label="SOC 测点">
          <el-input
            v-model="enosSource.soc_point"
            placeholder="BS.SOC"
            @input="updateEnOSSource"
          />
          <div class="form-tip">可选，不配置则使用模型计算SOC</div>
        </el-form-item>

        <template v-if="deviceType === 'wind'">
          <el-form-item label="理论功率测点">
            <el-input
              v-model="enosSource.theory_power_point"
              placeholder="WNAC.TheoryActivePW"
              @input="updateEnOSSource"
            />
          </el-form-item>

          <el-form-item label="风速测点">
            <el-input
              v-model="enosSource.wind_speed_point"
              placeholder="WNAC.WindSpeed"
              @input="updateEnOSSource"
            />
            <div class="form-tip">可选，不配置则只使用理论功率</div>
          </el-form-item>
        </template>
      </template>

      <template v-if="modelValue.kind === 'enos_aggregate'">
        <el-form-item label="聚合资产列表">
          <el-select
            v-model="enosSource.aggregate_asset_ids"
            multiple
            filterable
            allow-create
            placeholder="输入或选择资产ID"
            @change="updateEnOSSource"
          >
            <el-option
              v-for="assetId in assetIdOptions"
              :key="assetId"
              :label="assetId"
              :value="assetId"
            />
          </el-select>
          <div class="form-tip">至少需要1个资产ID</div>
        </el-form-item>

        <el-form-item label="理论功率测点">
          <el-input
            v-model="enosSource.theory_power_point"
            placeholder="WNAC.TheoryActivePW"
            @input="updateEnOSSource"
          />
        </el-form-item>

        <el-form-item label="风速测点">
          <el-input
            v-model="enosSource.wind_speed_point"
            placeholder="WNAC.WindSpeed"
            @input="updateEnOSSource"
          />
        </el-form-item>

        <el-form-item label="风速回退资产">
          <el-select
            v-model="enosSource.wind_speed_fallback_asset_ids"
            multiple
            filterable
            allow-create
            placeholder="输入或选择风速回退资产ID"
            @change="updateEnOSSource"
          >
            <el-option
              v-for="assetId in assetIdOptions"
              :key="assetId"
              :label="assetId"
              :value="assetId"
            />
          </el-select>
          <div class="form-tip">可选，用于主要风速失效时的回退</div>
        </el-form-item>

        <el-form-item label="最低覆盖率">
          <el-input-number
            v-model="enosSource.min_coverage_ratio"
            :min="0"
            :max="1"
            :step="0.1"
            placeholder="0.8"
            @change="updateEnOSSource"
          />
          <div class="form-tip">有效资产比例低于此值时执行失效策略</div>
        </el-form-item>
      </template>
    </template>

    <!-- CSV 数据源配置 -->
    <template v-if="modelValue.kind === 'csv'">
      <el-form-item label="CSV 文件">
        <el-select v-model="csvSource.file_id" placeholder="选择CSV文件" @change="updateCSVSource">
          <el-option
            v-for="file in csvFiles"
            :key="file.id"
            :label="file.name"
            :value="file.id"
          />
        </el-select>
      </el-form-item>

      <el-form-item label="插值方式">
        <el-select v-model="csvSource.interpolation" @change="updateCSVSource">
          <el-option label="线性插值" value="linear" />
          <el-option label="保持上值" value="hold" />
        </el-select>
      </el-form-item>

      <el-form-item label="结束行为">
        <el-select v-model="csvSource.end_behavior" @change="updateCSVSource">
          <el-option label="循环播放" value="loop" />
          <el-option label="保持最后值" value="hold" />
          <el-option label="停止仿真" value="stop" />
        </el-select>
      </el-form-item>

      <el-row :gutter="16">
        <el-col :span="12">
          <el-form-item label="倍率">
            <el-input-number
              v-model="csvSource.scale"
              :precision="2"
              :step="0.1"
              @change="updateCSVSource"
            />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="偏移">
            <el-input-number
              v-model="csvSource.offset"
              :precision="2"
              :step="0.1"
              @change="updateCSVSource"
            />
          </el-form-item>
        </el-col>
      </el-row>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import type { DataSource, EnOSSource, CSVSource, EnOSCredential } from '@/api'
import { listEnOSCredentials } from '@/api'

interface Props {
  modelValue: DataSource
  deviceType: 'meter' | 'pv' | 'bess' | 'ev' | 'load' | 'wind'
  hideCredential?: boolean
  selectedCredentialRef?: string
}

interface Emits {
  (e: 'update:modelValue', value: DataSource): void
  (e: 'update:selectedCredentialRef', value: string): void
}

const props = withDefaults(defineProps<Props>(), {
  hideCredential: false
})

const emit = defineEmits<Emits>()

const credentials = ref<EnOSCredential[]>([])
const csvFiles = ref<Array<{ id: string; name: string }>>([])
const assetIdOptions = ref<string[]>([])

// 计算可用的数据源选项
const sourceOptions = computed(() => {
  const options = [
    { label: '合成数据', value: 'synthetic' },
    { label: 'CSV 文件', value: 'csv' },
    { label: 'EnOS 回放', value: 'enos' }
  ]

  if (props.deviceType === 'wind') {
    options.push({ label: 'EnOS 聚合', value: 'enos_aggregate' })
  }

  if (props.deviceType === 'meter') {
    options.unshift({ label: '计算模式', value: 'calculated' })
  }

  return options
})

// 是否需要功率测点配置
const needsPowerPoint = computed(() => {
  return ['pv', 'bess', 'ev', 'load', 'meter'].includes(props.deviceType)
})

// EnOS 源配置
const enosSource = ref<EnOSSource>({
  asset_id: '',
  power_point: '',
  soc_point: '',
  theory_power_point: '',
  wind_speed_point: '',
  aggregate_asset_ids: [],
  wind_speed_fallback_asset_ids: [],
  min_coverage_ratio: 1.0
})

// CSV 源配置
const csvSource = ref<CSVSource>({
  file_id: '',
  interpolation: 'linear',
  end_behavior: 'loop',
  period_sec: undefined,
  scale: 1,
  offset: 0
})

onMounted(() => {
  loadCredentials()
  loadCSVFiles()
  initializeFromModelValue()
})

watch(() => props.modelValue, initializeFromModelValue)

async function loadCredentials() {
  try {
    credentials.value = await listEnOSCredentials()
  } catch (error) {
    console.error('加载凭据失败:', error)
  }
}

function loadCSVFiles() {
  // TODO: 从API加载CSV文件列表
  csvFiles.value = [
    { id: 'pv_curve.csv', name: '光伏出力曲线' },
    { id: 'load_curve.csv', name: '负载曲线' },
    { id: 'wind_curve.csv', name: '风机出力曲线' }
  ]
}

function initializeFromModelValue() {
  if (props.modelValue.enos) {
    Object.assign(enosSource.value, props.modelValue.enos)
  }
  if (props.modelValue.csv) {
    Object.assign(csvSource.value, props.modelValue.csv)
  }
}

function handleSourceTypeChange() {
  const newSource: DataSource = {
    kind: props.modelValue.kind
  }

  if (props.modelValue.kind === 'enos' || props.modelValue.kind === 'enos_aggregate') {
    newSource.enos = { ...enosSource.value }
    // 设置默认测点
    if (needsPowerPoint.value && !newSource.enos.power_point) {
      newSource.enos.power_point = getPowerPointPlaceholder()
    }
  } else if (props.modelValue.kind === 'csv') {
    newSource.csv = { ...csvSource.value }
  }

  emit('update:modelValue', newSource)
}

function updateEnOSSource() {
  const newSource: DataSource = {
    kind: props.modelValue.kind,
    enos: { ...enosSource.value }
  }
  emit('update:modelValue', newSource)
}

function updateCSVSource() {
  const newSource: DataSource = {
    kind: props.modelValue.kind,
    csv: { ...csvSource.value }
  }
  emit('update:modelValue', newSource)
}

function getPowerPointPlaceholder(): string {
  const pointMap: Record<string, string> = {
    meter: 'METER.ActivePW',
    pv: 'INV.GenActivePW',
    bess: 'BS.ActivePW',
    ev: 'PUB_CONN.ChargePW',
    load: 'LD.ActivePW'
  }
  return pointMap[props.deviceType] || 'METER.ActivePW'
}
</script>

<style scoped>
.form-tip {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
}
</style>