<template>
  <div v-if="ioaConflicts.length > 0" class="ioa-conflict-banner">
    <div class="ioa-conflict-icon">⚠️</div>
    <div class="ioa-conflict-body">
      <strong>IOA 冲突检测</strong><br>
      <span v-for="(err, i) in ioaConflicts" :key="i" class="ioa-conflict-line">{{ err }}</span>
    </div>
    <el-button size="small" type="danger" plain @click="$emit('auto-resolve')">自动修复</el-button>
  </div>
  <div v-else-if="hasDevices && !running" class="ioa-ok-banner">
    <span>✓ IOA 分配正常，无冲突</span>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  ioaConflicts: string[]
  hasDevices: boolean
  running: boolean
}>()

defineEmits<{
  'auto-resolve': []
}>()
</script>

<style scoped>
.ioa-conflict-banner {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 14px;
  margin-bottom: 12px;
  background: #fef0f0;
  border: 1px solid #f56c6c;
  border-radius: 6px;
  font-size: 12px;
  color: #f56c6c;
}
.ioa-conflict-icon { font-size: 16px; flex-shrink: 0; line-height: 1.4; }
.ioa-conflict-body { flex: 1; line-height: 1.6; }
.ioa-conflict-line { display: block; font-size: 11px; }
.ioa-ok-banner {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 14px;
  margin-bottom: 12px;
  background: #f0f9eb;
  border: 1px solid #67c23a;
  border-radius: 6px;
  font-size: 12px;
  color: #67c23a;
}
</style>
