<template>
  <div>
    <!-- Template + Layout toolbar -->
    <el-card shadow="never" style="margin-bottom: 12px">
      <div style="display: flex; align-items: center; gap: 10px; flex-wrap: wrap">
        <el-radio-group v-model="trendMode" size="small">
          <el-radio-button value="realtime">实时趋势</el-radio-button>
          <el-radio-button value="history">历史查询</el-radio-button>
        </el-radio-group>
        <template v-if="trendMode === 'history'">
          <el-date-picker
            v-model="historyRange"
            type="datetimerange"
            range-separator="至"
            start-placeholder="开始时间"
            end-placeholder="结束时间"
            format="YYYY-MM-DD HH:mm:ss"
            style="width: 370px"
          />
          <el-button size="small" type="primary" @click="queryHistory">查询历史</el-button>
          <el-button size="small" type="danger" plain @click="openHistoryCleanup">清理测点历史</el-button>
          <span style="font-size: 12px; color: var(--el-text-color-secondary)">固定结果，不自动刷新</span>
        </template>
        <template v-else>
          <span style="font-size: 12px; color: var(--el-color-success)">持续刷新最新数据</span>
        </template>
        <el-divider direction="vertical" />
        <span style="font-size: 13px; font-weight: 500; white-space: nowrap">模板</span>
        <el-select v-model="activeTemplateId" size="small" style="width: 200px" @change="loadTemplate" clearable placeholder="无模板">
          <el-option v-for="tpl in templates" :key="tpl.id" :label="tpl.name" :value="tpl.id" />
        </el-select>
        <el-button size="small" @click="saveTemplate" :disabled="!activeTemplateId || allTraces.length === 0">💾 覆盖保存</el-button>
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
        :panel-kind="p.kind"
        :traces="p.traceConfigs"
        :time-range="15"
        :poll-interval="1000"
        :mode="trendMode"
        :history-from="historyFrom"
        :history-to="historyTo"
        :history-query-token="historyQueryToken"
        :history-cleanup="historyCleanup"
        @remove="removePanel"
        @add-trace="onAddTrace"
        @traces-changed="onTracesChanged"
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
            <el-option v-for="pt in addTracePoints" :key="pt.ioa" :label="`${pt.name || '未命名'} · ${pt.point_type} · IOA:${pt.ioa}`" :value="pt.ioa" />
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

    <!-- History cleanup dialog -->
    <el-dialog v-model="showHistoryCleanup" title="清理测点历史数据" width="560px" @opened="loadCleanupInstances">
      <el-alert
        title="该操作会永久删除所选测点在数据库中的全部历史样本，无法恢复。运行中的 AI、DI、PI 会在下一个采样周期重新写入新样本。"
        type="warning"
        :closable="false"
        show-icon
        style="margin-bottom: 16px"
      />
      <el-form label-width="72px">
        <el-form-item label="实例">
          <el-select v-model="cleanupInstanceId" filterable style="width: 100%" @change="loadCleanupPoints">
            <el-option v-for="inst in allInstances" :key="inst.id" :label="inst.name + ' (' + inst.id + ')'" :value="inst.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="测点">
          <el-select
            v-model="cleanupIOAs"
            filterable
            multiple
            collapse-tags
            collapse-tags-tooltip
            style="width: 100%"
            :loading="cleanupPointsLoading"
            :disabled="!cleanupInstanceId"
            placeholder="仅显示存在历史数据的测点"
          >
            <el-option
              v-for="point in cleanupPoints"
              :key="point.ioa"
              :label="`${point.name || '未命名'} · ${point.point_type} · IOA:${point.ioa}`"
              :value="point.ioa"
            />
          </el-select>
        </el-form-item>
        <el-form-item v-if="cleanupIOAs.length > 0" label="删除范围">
          <span style="color: var(--el-color-danger)">将清理 {{ cleanupIOAs.length }} 个测点的全部已持久化历史数据</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showHistoryCleanup = false">取消</el-button>
        <el-button type="danger" :loading="cleanupSubmitting" :disabled="cleanupIOAs.length === 0" @click="confirmHistoryCleanup">
          清理所选历史
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listInstances, getPoints, getMicrogridPoints, getPersistenceStatus, getLatestPersistedSnapshot, deletePointHistory, type PointSnapshot, type PersistedPointSnapshot } from '../api'
import TrendPanel from './TrendPanel.vue'

const COLORS = ['#14b8a6', '#f59e0b', '#3b82f6', '#a855f7', '#ec4899', '#22d3ee', '#f97316', '#8b5cf6']

type PointType = 'AI' | 'DI' | 'PI' | 'AO' | 'DO'
type PanelKind = 'collection' | 'ao-control' | 'do-control'

const PANEL_KIND_ORDER: PanelKind[] = ['collection', 'ao-control', 'do-control']
const PANEL_KIND_LABEL: Record<PanelKind, string> = {
  collection: '采集趋势（AI / DI / PI）',
  'ao-control': 'AO 控制',
  'do-control': 'DO 控制',
}
const MAX_PANELS = 12

interface TraceConfig {
  instId: string; inst: string; ioa: number; name: string; unit: string
  alias: string; colorIdx: number
  pointType?: PointType
}

interface TemplatePanel {
  kind: PanelKind
  traceConfigs: TraceConfig[]
}

interface Template {
  id: string
  name: string
  // panels is the v2 format. traces is retained only to migrate existing browser templates.
  panels?: TemplatePanel[]
  traces?: TraceConfig[]
  createdAt: number
}

interface Panel {
  id: string
  templateId: string
  kind?: PanelKind
  traceConfigs: TraceConfig[]
}

function isPointType(value: unknown): value is PointType {
  return value === 'AI' || value === 'DI' || value === 'PI' || value === 'AO' || value === 'DO'
}

function panelKindFor(pointType: PointType): PanelKind {
  if (pointType === 'AO') return 'ao-control'
  if (pointType === 'DO') return 'do-control'
  return 'collection'
}

function isTraceCompatible(trace: TraceConfig, kind: PanelKind): boolean {
  return Boolean(trace.pointType && panelKindFor(trace.pointType) === kind)
}

function recolorTraces(traces: TraceConfig[]): TraceConfig[] {
  return traces.map((trace, index) => ({ ...trace, colorIdx: index % COLORS.length }))
}

function cloneTraces(traces: TraceConfig[]): TraceConfig[] {
  return JSON.parse(JSON.stringify(traces))
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

type TrendMode = 'realtime' | 'history'
const trendMode = ref<TrendMode>('realtime')
const historyRange = ref<[Date, Date] | null>(null)
const historyQueryToken = ref(0)
const historyFrom = computed(() => historyRange.value?.[0].getTime() ?? null)
const historyTo = computed(() => historyRange.value?.[1].getTime() ?? null)

function queryHistory() {
  if (!historyRange.value || historyFrom.value === null || historyTo.value === null) {
    ElMessage.warning('请选择历史数据的开始和结束时间')
    return
  }
  if (historyFrom.value >= historyTo.value) {
    ElMessage.warning('结束时间必须晚于开始时间')
    return
  }
  historyQueryToken.value += 1
}

// Add trace dialog state
const showAddTrace = ref(false)
const addTracePanelId = ref('')
const allInstances = ref<{ id: string; name: string; protocol?: string }[]>([])
const addTraceInst = ref('')
const addTraceIoas = ref<number[]>([])
const addTraceAlias = ref('')
const addTracePoints = ref<PointSnapshot[]>([])

// History cleanup dialog state. Points come from persisted snapshots so stopped
// instances can still expose the history records that may be deleted.
const showHistoryCleanup = ref(false)
const cleanupInstanceId = ref('')
const cleanupIOAs = ref<number[]>([])
const cleanupPoints = ref<PersistedPointSnapshot[]>([])
const cleanupPointsLoading = ref(false)
const cleanupSubmitting = ref(false)
const historyCleanup = ref<{ token: number; instanceId: string; ioas: number[] } | null>(null)

// ── Computed ──
const allTraces = computed(() => {
  const result: TraceConfig[] = []
  panels.value.forEach(p => result.push(...p.traceConfigs))
  return result
})

// ── Panel classification, template CRUD, and routing ──
async function refreshInstances() {
  const list = await listInstances()
  allInstances.value = list.map(instance => ({ id: instance.id, name: instance.name, protocol: instance.protocol }))
}

async function hydrateTracePointTypes(traces: TraceConfig[]): Promise<boolean> {
  const missingInstanceIds = Array.from(new Set(
    traces.filter(trace => !isPointType(trace.pointType)).map(trace => trace.instId),
  ))
  if (missingInstanceIds.length === 0) return true

  try {
    if (allInstances.value.length === 0) await refreshInstances()
    const pointTypes = new Map<string, PointType>()
    for (const instanceId of missingInstanceIds) {
      const instance = allInstances.value.find(item => item.id === instanceId)
      const result = instance?.protocol === 'microgrid'
        ? await getMicrogridPoints(instanceId)
        : await getPoints(instanceId)
      for (const point of result.points || []) {
        if (isPointType(point.point_type)) pointTypes.set(`${instanceId}:${point.ioa}`, point.point_type)
      }
    }
    traces.forEach(trace => {
      if (!isPointType(trace.pointType)) trace.pointType = pointTypes.get(`${trace.instId}:${trace.ioa}`)
    })
  } catch {
    // The caller reports unresolved types instead of placing a trace in an arbitrary panel.
  }

  return traces.every(trace => isPointType(trace.pointType))
}

function splitTracesByKind(traces: TraceConfig[]): TemplatePanel[] {
  const groups = new Map<PanelKind, TraceConfig[]>()
  for (const kind of PANEL_KIND_ORDER) groups.set(kind, [])
  traces.forEach(trace => {
    if (trace.pointType) groups.get(panelKindFor(trace.pointType))?.push(trace)
  })
  return PANEL_KIND_ORDER
    .map(kind => ({ kind, traceConfigs: recolorTraces(groups.get(kind) || []) }))
    .filter(panel => panel.traceConfigs.length > 0)
}

function cloneTemplatePanels(template: Template): TemplatePanel[] {
  if (template.panels?.length) {
    return template.panels.map(panel => ({
      kind: panel.kind,
      traceConfigs: recolorTraces(cloneTraces(panel.traceConfigs)),
    }))
  }
  return splitTracesByKind(cloneTraces(template.traces || []))
}

function panelSnapshot(): TemplatePanel[] {
  return panels.value
    .filter(panel => panel.kind && panel.traceConfigs.length > 0)
    .map(panel => ({ kind: panel.kind as PanelKind, traceConfigs: recolorTraces(cloneTraces(panel.traceConfigs)) }))
}

function normalizePanels() {
  const normalized: Panel[] = []
  panels.value.forEach(panel => {
    const groups = splitTracesByKind(panel.traceConfigs)
    if (groups.length === 0) {
      normalized.push({ ...panel, kind: undefined, traceConfigs: [] })
      return
    }
    groups.forEach((group, index) => {
      normalized.push({
        id: index === 0 ? panel.id : genId(),
        templateId: panel.templateId,
        kind: group.kind,
        traceConfigs: group.traceConfigs,
      })
    })
  })
  panels.value = normalized.slice(0, MAX_PANELS)
}

function findTargetPanel(kind: PanelKind, requestedPanelId: string): Panel | undefined {
  const requested = panels.value.find(panel => panel.id === requestedPanelId)
  if (requested && (requested.traceConfigs.length === 0 || requested.kind === kind)) {
    requested.kind = kind
    return requested
  }
  return panels.value.find(panel => panel.kind === kind)
}

function routeTraces(requestedPanelId: string, traces: TraceConfig[]): number {
  const groups = splitTracesByKind(traces)
  let added = 0
  const unavailable: PanelKind[] = []

  groups.forEach(group => {
    let target = findTargetPanel(group.kind, requestedPanelId)
    if (!target) {
      if (panels.value.length >= MAX_PANELS) {
        unavailable.push(group.kind)
        return
      }
      target = { id: genId(), templateId: '', kind: group.kind, traceConfigs: [] }
      panels.value.push(target)
    }

    const additions = group.traceConfigs.filter(trace =>
      !target!.traceConfigs.some(existing => existing.instId === trace.instId && existing.ioa === trace.ioa),
    )
    if (additions.length > 0) {
      target.traceConfigs = recolorTraces([...target.traceConfigs, ...additions])
      added += additions.length
    }
  })

  if (unavailable.length > 0) {
    const labels = unavailable.map(kind => PANEL_KIND_LABEL[kind]).join('、')
    ElMessage.warning(`最多 ${MAX_PANELS} 个看板，无法添加：${labels}`)
  }
  return added
}

function loadTemplates() {
  try {
    const raw = localStorage.getItem('trend_templates')
    templates.value = raw ? JSON.parse(raw) : []
  } catch { templates.value = [] }
}

function saveTemplates() {
  try {
    localStorage.setItem('trend_templates', JSON.stringify(templates.value))
  } catch (err) {
    console.error('保存模板失败:', err)
    ElMessage.error('保存模板失败：本地存储已满或处于隐私模式')
  }
}

async function loadTemplate(id: string) {
  const template = templates.value.find(item => item.id === id)
  if (!template) return
  if (panels.value.some(panel => panel.traceConfigs.length > 0)) {
    try {
      await ElMessageBox.confirm('加载模板会覆盖当前所有看板的测点配置，是否继续？', '确认加载', { type: 'warning' })
    } catch {
      activeTemplateId.value = ''
      return
    }
  }

  const legacyTraces = template.panels?.length ? [] : cloneTraces(template.traces || [])
  if (legacyTraces.length > 0 && !await hydrateTracePointTypes(legacyTraces)) {
    ElMessage.error('无法识别旧模板中部分测点的类型，未加载模板')
    return
  }
  const source: Template = legacyTraces.length > 0 ? { ...template, traces: legacyTraces } : template
  const templatePanels = cloneTemplatePanels(source)
  panels.value = templatePanels.map(panel => ({
    id: genId(), templateId: template.id, kind: panel.kind, traceConfigs: panel.traceConfigs,
  }))
  debouncedSave()
  ElMessage.success('模板已加载')
}

function saveTemplate() {
  if (!activeTemplateId.value) {
    ElMessage.warning('请先选择一个模板，或使用「另存为」创建新模板')
    return
  }
  const template = templates.value.find(item => item.id === activeTemplateId.value)
  if (!template) return
  template.panels = panelSnapshot()
  delete template.traces
  saveTemplates()
  ElMessage.success('模板已保存')
}

function saveAsTemplate() {
  const name = newTemplateName.value.trim()
  if (!name) return
  templates.value.push({ id: genId(), name, panels: panelSnapshot(), createdAt: Date.now() })
  saveTemplates()
  activeTemplateId.value = templates.value[templates.value.length - 1].id
  showSaveAs.value = false
  newTemplateName.value = ''
  ElMessage.success('模板已保存')
}

function deleteTemplate() {
  if (!activeTemplateId.value) return
  ElMessageBox.confirm('确定删除该模板？', '确认', { type: 'warning' }).then(() => {
    templates.value = templates.value.filter(template => template.id !== activeTemplateId.value)
    saveTemplates()
    activeTemplateId.value = ''
  }).catch(() => {})
}

// ── Panel management ──
function addEmptyPanel() {
  if (panels.value.length >= MAX_PANELS) { ElMessage.warning(`最多 ${MAX_PANELS} 个面板`); return }
  panels.value.push({ id: genId(), templateId: '', traceConfigs: [] })
  debouncedSave()
}

async function addPanel() {
  const template = templates.value.find(item => item.id === selectedTemplateForPanel.value)
  if (!template) return
  const templatePanels = cloneTemplatePanels(template)
  if (panels.value.length + templatePanels.length > MAX_PANELS) {
    ElMessage.warning(`最多 ${MAX_PANELS} 个面板，无法添加该模板`)
    return
  }
  panels.value.push(...templatePanels.map(panel => ({
    id: genId(), templateId: template.id, kind: panel.kind, traceConfigs: panel.traceConfigs,
  })))
  showAddPanel.value = false
  selectedTemplateForPanel.value = ''
  debouncedSave()
}

function removePanel(panelId: string) {
  const index = panels.value.findIndex(panel => panel.id === panelId)
  if (index !== -1) panels.value.splice(index, 1)
  debouncedSave()
}

function onTracesChanged(panelId: string, traces: TraceConfig[]) {
  const panel = panels.value.find(item => item.id === panelId)
  if (!panel) return
  panel.traceConfigs = recolorTraces(traces.filter(trace => panel.kind ? isTraceCompatible(trace, panel.kind) : true))
  if (panel.traceConfigs.length === 0) panel.kind = undefined
  debouncedSave()
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
    await refreshInstances()
  } catch { ElMessage.error('加载实例列表失败') }
}

async function onAddTraceInstChange() {
  addTraceIoas.value = []
  addTracePoints.value = []
  if (!addTraceInst.value) return
  try {
    const instance = allInstances.value.find(item => item.id === addTraceInst.value)
    const result = instance?.protocol === 'microgrid'
      ? await getMicrogridPoints(addTraceInst.value)
      : await getPoints(addTraceInst.value)
    addTracePoints.value = (result.points || []).sort((a: any, b: any) => a.ioa - b.ioa)
  } catch { ElMessage.warning('加载测点失败') }
}

function confirmAddTrace() {
  const instance = allInstances.value.find(item => item.id === addTraceInst.value)
  if (!instance) return
  const traces: TraceConfig[] = []
  for (const ioa of addTraceIoas.value) {
    const point = addTracePoints.value.find(item => item.ioa === ioa)
    if (!point || !isPointType(point.point_type)) continue
    traces.push({
      instId: addTraceInst.value,
      inst: instance.name,
      ioa,
      name: point.name || '',
      unit: point.unit || '',
      alias: addTraceAlias.value,
      colorIdx: 0,
      pointType: point.point_type,
    })
  }
  const added = routeTraces(addTracePanelId.value, traces)
  showAddTrace.value = false
  if (added > 0) ElMessage.success(`已按测点类型添加 ${added} 个趋势`)
  debouncedSave()
}

// ── History cleanup ──
async function openHistoryCleanup() {
  const status = await getPersistenceStatus()
  if (!status.enabled) {
    ElMessage.warning('数据持久化未启用，无法清理历史数据')
    return
  }
  cleanupInstanceId.value = ''
  cleanupIOAs.value = []
  cleanupPoints.value = []
  showHistoryCleanup.value = true
}

async function loadCleanupInstances() {
  try {
    const list = await listInstances()
    allInstances.value = list.map(s => ({ id: s.id, name: s.name, protocol: s.protocol }))
  } catch {
    ElMessage.error('加载实例列表失败')
  }
}

async function loadCleanupPoints() {
  cleanupIOAs.value = []
  cleanupPoints.value = []
  if (!cleanupInstanceId.value) return
  cleanupPointsLoading.value = true
  try {
    const result = await getLatestPersistedSnapshot(cleanupInstanceId.value)
    cleanupPoints.value = result.points.sort((a, b) => a.ioa - b.ioa)
    if (cleanupPoints.value.length === 0) {
      ElMessage.info('该实例没有可清理的历史数据')
    }
  } catch {
    ElMessage.error('加载数据库测点失败')
  } finally {
    cleanupPointsLoading.value = false
  }
}

async function confirmHistoryCleanup() {
  const instance = allInstances.value.find(item => item.id === cleanupInstanceId.value)
  if (!instance || cleanupIOAs.value.length === 0) return
  if (cleanupIOAs.value.length > 500) {
    ElMessage.warning('单次最多清理 500 个测点')
    return
  }

  try {
    await ElMessageBox.confirm(
      `将永久删除实例「${instance.name}」中 ${cleanupIOAs.value.length} 个测点的全部历史数据。该操作无法恢复，是否继续？`,
      '确认清理历史数据',
      {
        type: 'warning',
        confirmButtonText: '确认永久删除',
        cancelButtonText: '取消',
      },
    )
  } catch {
    return
  }

  cleanupSubmitting.value = true
  try {
    const ioas = [...cleanupIOAs.value]
    const result = await deletePointHistory(instance.id, ioas)
    historyCleanup.value = {
      token: (historyCleanup.value?.token || 0) + 1,
      instanceId: instance.id,
      ioas,
    }
    if (trendMode.value === 'history' && historyRange.value) {
      historyQueryToken.value += 1
    }
    showHistoryCleanup.value = false
    ElMessage.success(`已删除 ${result.deleted} 条历史样本`)
  } catch {
    ElMessage.error('清理历史数据失败')
  } finally {
    cleanupSubmitting.value = false
  }
}

// ── Persistence ──
let saveTimer: ReturnType<typeof setTimeout> | null = null
function debouncedSave() {
  if (saveTimer) clearTimeout(saveTimer)
  saveTimer = setTimeout(() => { savePanels(); saveTemplates() }, 500)
}

function savePanels() {
  try {
    const save = panels.value.map(panel => ({
      id: panel.id,
      templateId: panel.templateId,
      kind: panel.kind,
      traceConfigs: panel.traceConfigs,
    }))
    localStorage.setItem('trend_panels', JSON.stringify(save))
  } catch (err) {
    console.error('保存面板失败:', err)
    ElMessage.error('保存失败：本地存储已满或处于隐私模式，数据可能无法持久化')
  }
}

function loadPanels() {
  try {
    const raw = localStorage.getItem('trend_panels')
    if (!raw) return
    const saved = JSON.parse(raw)
    if (!Array.isArray(saved)) return
    saved.forEach((item: any) => {
      panels.value.push({
        id: item.id || genId(),
        templateId: item.templateId || '',
        kind: item.kind === 'collection' || item.kind === 'ao-control' || item.kind === 'do-control' ? item.kind : undefined,
        traceConfigs: item.traceConfigs || [],
      })
    })
  } catch { /* ignore */ }
}

async function migrateSavedPanels() {
  const traces = panels.value.flatMap(panel => panel.traceConfigs)
  if (traces.length === 0) return
  const resolved = await hydrateTracePointTypes(traces)
  if (!resolved) {
    ElMessage.warning('部分已保存测点的类型无法识别；保留原看板配置，待测点类型可读取后再分流')
    return
  }
  normalizePanels()
}

// ── Lifecycle ──
function genId(): string {
  return Date.now().toString(36) + Math.random().toString(36).slice(2, 8)
}

onMounted(async () => {
  loadTemplates()
  loadPanels()
  await migrateSavedPanels()

  const pendingRaw = localStorage.getItem('trend_pending_traces')
  if (!pendingRaw) return
  try {
    const pendingTraces: TraceConfig[] = JSON.parse(pendingRaw)
    localStorage.removeItem('trend_pending_traces')
    if (pendingTraces.length === 0) return
    if (!await hydrateTracePointTypes(pendingTraces)) {
      ElMessage.error('无法识别待添加测点的类型，未添加到趋势看板')
      return
    }
    if (panels.value.length === 0) panels.value.push({ id: genId(), templateId: '', traceConfigs: [] })
    const added = routeTraces(panels.value[0].id, pendingTraces)
    savePanels()
    if (added > 0) ElMessage.success(`已按测点类型添加 ${added} 个趋势`)
  } catch {
    localStorage.removeItem('trend_pending_traces')
  }
})

onUnmounted(() => {
  if (saveTimer) clearTimeout(saveTimer)
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
