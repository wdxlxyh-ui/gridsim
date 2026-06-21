<template>
  <div class="content-split">
    <div class="request-panel">
      <div class="panel-header"><span>请求配置</span></div>
      <div class="panel-body">
        <div v-if="activeTab === 'headers'">
          <div v-for="(h, i) in headerList" :key="i" class="kv-row">
            <el-checkbox v-model="h.enabled" />
            <el-input v-model="h.key" placeholder="Key" size="small" />
            <el-input v-model="h.value" placeholder="Value" size="small" />
            <el-button text size="small" @click="removeHeader(i)">×</el-button>
          </div>
          <el-button size="small" @click="addHeader">+ 添加</el-button>
        </div>
        <div v-if="activeTab === 'body'">
          <el-radio-group v-model="bodyType" size="small" style="margin-bottom: 8px;">
            <el-radio-button value="json">JSON</el-radio-button>
            <el-radio-button value="text">Text</el-radio-button>
            <el-radio-button value="none">None</el-radio-button>
          </el-radio-group>
          <el-input v-if="bodyType !== 'none'" v-model="body" type="textarea" :rows="8" placeholder="请求体..." />
        </div>
        <div v-if="activeTab === 'pre-script'">
          <div style="font-size: 12px; color: #94a3b8; margin-bottom: 8px;">
            发送前执行脚本，用 <code style="background: #1a1f2e; padding: 1px 6px; border-radius: 3px; color: #f59e0b; font-family: 'JetBrains Mono', monospace; font-size: 11px;">vars.变量名 = 值</code> 给变量赋值
          </div>
          <el-input v-model="preScript" type="textarea" :rows="10"
            placeholder="示例：&#10;vars.timestamp = $now()&#10;vars.date = $formatTime('yyyy-MM-dd')"
            style="font-family: 'JetBrains Mono', monospace; font-size: 12px;" />
          <div style="margin-top: 8px; font-size: 11px; color: #64748b; line-height: 1.8;">
            <div><span style="color: #f59e0b;">内置函数：</span> <code>$now()</code> <code>$formatTime(fmt)</code> <code>$timestamp()</code> <code>$uuid()</code></div>
          </div>
        </div>
        <div v-if="activeTab === 'post-script'">
          <div style="font-size: 12px; color: #94a3b8; margin-bottom: 8px;">
            发送后执行脚本，用 <code style="background: #1a1f2e; padding: 1px 6px; border-radius: 3px; color: #f59e0b; font-family: 'JetBrains Mono', monospace; font-size: 11px;">pm.response.json()</code> 提取响应内容
          </div>
          <el-input v-model="testScript" type="textarea" :rows="10"
            placeholder="示例：&#10;var data = pm.response.json()"
            style="font-family: 'JetBrains Mono', monospace; font-size: 12px;" />
        </div>
      </div>
    </div>
    <slot />
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'

const props = defineProps<{
  activeTab: string
  headerList: { key: string; value: string; enabled: boolean }[]
  bodyType: string
  body: string
  preScript: string
  testScript: string
}>()

const emit = defineEmits<{
  'update:activeTab': [v: string]
  'update:headerList': [v: { key: string; value: string; enabled: boolean }[]]
  'update:bodyType': [v: string]
  'update:body': [v: string]
  'update:preScript': [v: string]
  'update:testScript': [v: string]
}>()

const activeTab = computed({ get: () => props.activeTab, set: (v) => emit('update:activeTab', v) })
const bodyType = computed({ get: () => props.bodyType, set: (v) => emit('update:bodyType', v) })
const body = computed({ get: () => props.body, set: (v) => emit('update:body', v) })
const preScript = computed({ get: () => props.preScript, set: (v) => emit('update:preScript', v) })
const testScript = computed({ get: () => props.testScript, set: (v) => emit('update:testScript', v) })

function addHeader() {
  const newList = [...props.headerList, { key: '', value: '', enabled: true }]
  emit('update:headerList', newList)
}

function removeHeader(index: number) {
  const newList = [...props.headerList]
  newList.splice(index, 1)
  emit('update:headerList', newList)
}
</script>

<style scoped>
.content-split { display: flex; flex: 1; overflow: hidden; }
.request-panel { flex: 1; display: flex; flex-direction: column; border-right: 1px solid #1e293b; overflow: hidden; }
.panel-header { padding: 8px 14px; font-size: 11px; font-weight: 600; color: #64748b; text-transform: uppercase; letter-spacing: 0.5px; background: #111827; border-bottom: 1px solid #1e293b; display: flex; align-items: center; justify-content: space-between; }
.panel-body { flex: 1; overflow-y: auto; padding: 10px 14px; }
.kv-row { display: flex; gap: 6px; margin-bottom: 5px; align-items: center; }
</style>
