<template>
  <el-card shadow="never">
    <template #header>
      <div style="display:flex;justify-content:space-between;align-items:center">
        <span style="font-weight:600">IEC104 测点列表</span>
        <el-button size="small" @click="$emit('refresh')" :loading="loadingPoints" type="primary" plain>刷新</el-button>
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
          <el-switch v-else v-model="row.local_mode" size="small" active-text="本地" inactive-text="远方"
            @change="(val: boolean) => $emit('toggle-mode', row, val)" />
        </template>
      </el-table-column>
      <el-table-column label="策略" width="100" fixed="right">
        <template #default="{ row }">
          <el-button v-if="row.can_toggle && row.local_mode" size="small" text type="warning"
            @click="$emit('config-strategy', row)">配置策略</el-button>
          <span v-else-if="row.can_toggle" style="font-size:11px;color:#909399">引擎控制</span>
          <span v-else style="font-size:11px;color:#c0c4cc">—</span>
        </template>
      </el-table-column>
    </el-table>
  </el-card>
</template>

<script setup lang="ts">
defineProps<{
  points: any[]
  loadingPoints: boolean
}>()

defineEmits<{
  refresh: []
  'toggle-mode': [row: any, local: boolean]
  'config-strategy': [row: any]
}>()
</script>
