<template>
  <el-card shadow="never" class="section-card">
    <template #header><span style="font-weight: 600">关口表配置</span></template>
    <el-form label-width="100px" size="small">
      <el-form-item label="额定容量">
        <el-input-number v-model="gridMeter.rated_capacity_kw" :min="1" :max="99999" style="width:100%" />
        <span style="margin-left:6px;font-size:12px;color:var(--el-text-color-secondary)">kW</span>
      </el-form-item>
      <el-form-item label="进口限值">
        <el-input-number v-model="gridMeter.import_limit_kw" :min="0" :max="99999" style="width:100%" />
        <span style="margin-left:6px;font-size:12px;color:var(--el-text-color-secondary)">kW（0=同额定容量）</span>
      </el-form-item>
      <el-form-item label="出口限值">
        <el-input-number v-model="gridMeter.export_limit_kw" :min="0" :max="99999" style="width:100%" />
        <span style="margin-left:6px;font-size:12px;color:var(--el-text-color-secondary)">kW（0=同额定容量）</span>
      </el-form-item>
      <el-form-item label="并网开关点">
        <el-switch v-model="gridMeter.breaker_points" />
        <span style="margin-left:8px;font-size:12px;color:var(--el-text-color-secondary)">
          生成并网开关状态/遥控测点，遥控分闸即孤岛
        </span>
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
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'

const props = defineProps<{
  gridMeter: { rated_capacity_kw: number; island_mode: boolean; import_limit_kw?: number; export_limit_kw?: number; breaker_points?: boolean }
  busName: string
  busVoltage: number
}>()

const emit = defineEmits<{
  'update:gridMeter': [value: typeof props.gridMeter]
  'update:busName': [value: string]
  'update:busVoltage': [value: number]
  change: []
}>()

const gridMeter = ref({ import_limit_kw: 0, export_limit_kw: 0, breaker_points: false, ...props.gridMeter })
const busName = ref(props.busName)
const busVoltage = ref(props.busVoltage)

watch(gridMeter, (v) => { emit('update:gridMeter', { ...v }); emit('change') }, { deep: true })
watch(busName, (v) => { emit('update:busName', v); emit('change') })
watch(busVoltage, (v) => { emit('update:busVoltage', v); emit('change') })
</script>

<style scoped>
.section-card { margin-bottom: 0; }
</style>
