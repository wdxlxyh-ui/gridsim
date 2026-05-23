<template>
  <div>
    <!-- Template + Layout toolbar -->
    <el-card shadow="never" style="margin-bottom: 12px">
      <div style="display: flex; align-items: center; gap: 10px; flex-wrap: wrap">
        <span style="font-size: 13px; font-weight: 500; white-space: nowrap">模板</span>
        <el-select v-model="activeTemplateId" size="small" style="width: 200px" @change="loadTemplate" clearable placeholder="无模板">
          <el-option v-for="tpl in templates" :key="tpl.id" :label="tpl.name" :value="tpl.id" />
        </el-select>
        <el-button size="small" @click="saveTemplate" :disabled="allTraces.length === 0">💾 覆盖保存</el-button>
        <el-button size="small" @click="showSaveAs = true">📋 另存为</el-button>
        <el-button size="small" @click="deleteTemplate" :disabled="!activeTemplateId" type="danger" text>🗑</el-button>
        <el-divider direction="vertical" />
        <el-radio-group v-model="layoutMode" size="small">
          <el-radio-button value="single">📊 单</el-radio-button>
          <el-radio-button value="horizontal">⬅➡ 左右</el-radio-button>
          <el-radio-button value="vertical">⬆⬇ 上下</el-radio-button>
        </el-radio-group>
        <el-button size="small" type="primary" @click="addEmptyPanel">+ 面板</el-button>
        <span style="font-size: 12px; color: var(--el-text-color-secondary); margin-left: auto">
          {{ panels.length }} 个面板 · {{ layoutMode === 'single' ? '单列' : layoutMode === 'horizontal' ? '左右分屏' : '上下分屏' }}
        </span>
      </div>
    </el-card>

    <!-- Panel grid -->
    <div :class="'panel-grid panel-grid--' + layoutMode">
      <TrendPanel
        v-for="(p, i) in panels"
        :key="p.id"
        :panel-id="p.id"
        :traces="p.traceConfigs"
        :time-range="15"
        :poll-interval="1000"
        @remove="removePanel"
        @add-trace="onAddTrace"
      />
    </div>

    <!-- Save-as dialog -->
    <el-dialog v-model="showSaveAs" title="另存为模板" width="360px">
      <el-input v-model="newTemplateName" placeholder="模板名称，如: 变电站A看板" />
      <template #footer>
        <el-button @click="showSaveAs = false">取消</el-button>
        <el-button type="primary" @click="saveAsTemplate" :disabled="!newTemplateName.trim()">保存</el-button>
      </template>
    </el-dialog>

    <!-- Add panel dialog -->
    <el-dialog v-model="showAddPanel" title="添加面板 — 选择模板" width="400px">
      <el-select v-model="selectedTemplateForPanel" style="width: 100%" placeholder="选择模板">
        <el-option v-for="tpl in templates" :key="tpl.id" :label="tpl.name" :value="tpl.id" />
      </el-select>
      <template #footer>
        <el-button @click="showAddPanel = false">取消</el-button>
        <el-button type="primary" @click="addPanel" :disabled="!selectedTemplateForPanel">添加</el-button>
      </template>
    </el-dialog>

    <!-- Add trace dialog -->
    <el-dialog v-model="showAddTrace" title="添加测点" width="480px" @opened="initAddTraceDialog">
      <el-form label-width="60px">
        <el-form-item label="实例">
          <el-select v-model="addTraceInst" filterable style="width: 100%" @change="onAddTraceInstChange">
            <el-option v-for="inst in allInstances" :key="inst.id" :label="inst.name + ' (' + inst.id + ')'" :value="inst.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="测点">
          <el-select v-model="addTraceIoas" filterable multiple style="width: 100%" :disabled="!addTraceInst">
            <el-option v-for="pt in addTracePoints" :key="pt.ioa" :label="pt.name + ' (IOA:' + pt.ioa + ')'" :value="pt.ioa" />
          </el-select>
        </el-form-item>
        <el-form-item label="别名">
          <el-input v-model="addTraceAlias" placeholder="可选" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddTrace = false">取消</el-button>
        <el-button type="primary" @click="confirmAddTrace" :disabled="addTraceIoas.length === 0">确认 ({{ addTraceIoas.length }})</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listInstances, getPoints, getMicrogridPoints, type PointSnapshot } from '../api'
import TrendPanel from './TrendPanel.vue'

const COLORS = ['#14b8a6', '#f59e0b', '#3b82f6', '#a855f7', '#ec4899', '#22d3ee', '#f97316', '#8b5cf6']

interface TraceConfig {
  instId: string; inst: string; ioa: number; name: string; unit: string
  alias: string; colorIdx: number
}

interface Template {
  id: string
  name: string
  traces: TraceConfig[]
  createdAt: number
}

interface Panel {
  id: string
  templateId: string
  traceConfigs: TraceConfig[]
}

// ── State ──
const templates = ref<Template[]>([])
const activeTemplateId = ref('')
const panels = ref<Panel[]>([])
const layoutMode = ref<'single' | 'horizontal' | 'vertical'>('single')
const showSaveAs = ref(false)
const newTemplateName = ref('')
const showAddPanel = ref(false)
const selectedTemplateForPanel = ref('')

// Add trace dialog state
const showAddTrace = ref(false)
const addTracePanelId = ref('')
const allInstances = ref<{ id: string; name: string; protocol?: string }[]>([])
const addTraceInst = ref('')
const addTraceIoas = ref<number[]>([])
const addTraceAlias = ref('')
const addTracePoints = ref<PointSnapshot[]>([])

// ── Computed ──
const allTraces = computed(() => {
  const result: TraceConfig[] = []
  panels.value.forEach(p => result.push(...p.traceConfigs))
  return result
})

// ── Template CRUD ──
function loadTemplates() {
  try {
    const raw = localStorage.getItem('trend_templates')
    templates.value = raw ? JSON.parse(raw) : []
  } catch { templates.value = [] }
}

function saveTemplates() {
  localStorage.setItem('trend_templates', JSON.stringify(templates.value))
}

function loadTemplate(id: string) {
  const tpl = templates.value.find(t => t.id === id)
  if (!tpl) return
  if (panels.value.length === 0) {
    panels.value.push({ id: genId(), templateId: tpl.id, traceConfigs: JSON.parse(JSON.stringify(tpl.traces)) })
  } else {
    // Update first panel's traces
    panels.value[0].traceConfigs = JSON.parse(JSON.stringify(tpl.traces))
    panels.value[0].templateId = tpl.id
  }
}

function saveTemplate() {
  if (!activeTemplateId.value) return
  const tpl = templates.value.find(t => t.id === activeTemplateId.value)
  if (!tpl) return
  tpl.traces = JSON.parse(JSON.stringify(allTraces.value))
  saveTemplates()
  ElMessage.success('模板已保存')
}

function saveAsTemplate() {
  const name = newTemplateName.value.trim()
  if (!name) return
  templates.value.push({
    id: genId(),
    name,
    traces: JSON.parse(JSON.stringify(allTraces.value)),
    createdAt: Date.now(),
  })
  saveTemplates()
  activeTemplateId.value = templates.value[templates.value.length - 1].id
  showSaveAs.value = false
  newTemplateName.value = ''
  ElMessage.success('模板已保存: ' + name)
}

function deleteTemplate() {
  if (!activeTemplateId.value) return
  ElMessageBox.confirm('确定删除该模板？', '确认', { type: 'warning' }).then(() => {
    templates.value = templates.value.filter(t => t.id !== activeTemplateId.value)
    saveTemplates()
    activeTemplateId.value = ''
  }).catch(() => {})
}

// ── Panel management ──
function addEmptyPanel() {
  if (panels.value.length >= 4) { ElMessage.warning('最多 4 个面板'); return }
  panels.value.push({ id: genId(), templateId: '', traceConfigs: [] })
}

function addPanel() {
  const tpl = templates.value.find(t => t.id === selectedTemplateForPanel.value)
  if (!tpl) return
  if (panels.value.length >= 4) {
    ElMessage.warning('最多 4 个面板')
    return
  }
  panels.value.push({
    id: genId(),
    templateId: tpl.id,
    traceConfigs: JSON.parse(JSON.stringify(tpl.traces)),
  })
  showAddPanel.value = false
  selectedTemplateForPanel.value = ''
}

function removePanel(panelId: string) {
  const idx = panels.value.findIndex(p => p.id === panelId)
  if (idx !== -1) panels.value.splice(idx, 1)
}

// ── Add trace ──
function onAddTrace(panelId: string) {
  addTracePanelId.value = panelId
  addTraceInst.value = ''
  addTraceIoas.value = []
  addTraceAlias.value = ''
  addTracePoints.value = []
  showAddTrace.value = true
}

async function initAddTraceDialog() {
  try {
    const list = await listInstances()
    allInstances.value = list.map(s => ({ id: s.id, name: s.name, protocol: s.protocol }))
  } catch { ElMessage.error('加载实例列表失败') }
}

async function onAddTraceInstChange() {
  addTraceIoas.value = []
  addTracePoints.value = []
  if (!addTraceInst.value) return
  try {
    const inst = allInstances.value.find(i => i.id === addTraceInst.value)
    const res = inst?.protocol === 'microgrid'
      ? await getMicrogridPoints(addTraceInst.value)
      : await getPoints(addTraceInst.value)
    addTracePoints.value = (res.points || []).sort((a: any, b: any) => a.ioa - b.ioa)
  } catch { ElMessage.warning('加载测点失败') }
}

function confirmAddTrace() {
  const panel = panels.value.find(p => p.id === addTracePanelId.value)
  if (!panel) return
  const inst = allInstances.value.find(i => i.id === addTraceInst.value)
  if (!inst) return
  for (const ioa of addTraceIoas.value) {
    if (panel.traceConfigs.some(t => t.instId === addTraceInst.value && t.ioa === ioa)) continue
    const pt = addTracePoints.value.find(p => p.ioa === ioa)
    panel.traceConfigs = [...panel.traceConfigs, {
      instId: addTraceInst.value,
      inst: inst.name,
      ioa,
      name: pt?.name || '',
      unit: pt?.unit || '',
      alias: addTraceAlias.value,
      colorIdx: panel.traceConfigs.length,
    }]
  }
  showAddTrace.value = false
}

// ── Persistence ──
function savePanels() {
  const save = panels.value.map(p => ({
    id: p.id,
    templateId: p.templateId,
    traceConfigs: p.traceConfigs,
  }))
  localStorage.setItem('trend_panels', JSON.stringify(save))
}

function loadPanels() {
  try {
    const raw = localStorage.getItem('trend_panels')
    if (!raw) return
    const saved = JSON.parse(raw)
    if (!Array.isArray(saved)) return
    saved.forEach((s: any) => {
      panels.value.push({
        id: s.id || genId(),
        templateId: s.templateId || '',
        traceConfigs: s.traceConfigs || [],
      })
    })
  } catch { /* ignore */ }
}

// ── Lifecycle ──
function genId(): string {
  return Date.now().toString(36) + Math.random().toString(36).slice(2, 8)
}

onMounted(() => {
  loadTemplates()
  loadPanels()
  // If no panels, show tips
})

onUnmounted(() => {
  savePanels()
  saveTemplates()
})
</script>

<style scoped>
.panel-grid {
  display: grid;
  gap: 12px;
}
.panel-grid--single {
  grid-template-columns: 1fr;
}
.panel-grid--horizontal {
  grid-template-columns: 1fr 1fr;
}
.panel-grid--vertical {
  grid-template-columns: 1fr;
  max-height: calc(100vh - 140px);
  overflow-y: auto;
}
</style>
