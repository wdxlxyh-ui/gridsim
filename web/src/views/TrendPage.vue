<template>
  <section class="trend-page">
    <el-card shadow="never" class="trend-command-deck">
      <div class="deck-heading">
        <div class="deck-title-block">
          <span class="deck-kicker">OPERATIONS / TREND</span>
          <div class="deck-title-row">
            <h1>趋势工作台</h1>
            <span class="mode-indicator" :class="`is-${trendMode}`">
              <i></i>{{ trendMode === 'realtime' ? '实时采样' : '历史回放' }}
            </span>
          </div>
          <p>{{ trendMode === 'realtime' ? '在统一时间轴上持续观察现场采样与控制写入。' : '按指定时间范围回放已持久化的测点数据。' }}</p>
        </div>
        <div class="deck-summary" aria-label="趋势工作台概览">
          <div class="summary-item"><strong>{{ panels.length }}</strong><span>面板</span></div>
          <div class="summary-divider"></div>
          <div class="summary-item"><strong>{{ allTraces.length }}</strong><span>测点</span></div>
          <div class="summary-divider"></div>
          <div class="summary-layout">{{ layoutMode === 'single' ? '单列工作区' : layoutMode === 'horizontal' ? '左右分屏' : '纵向堆叠' }}</div>
        </div>
      </div>

      <div class="deck-controls">
        <div class="control-group control-group--mode">
          <span class="control-label">数据视图</span>
          <el-radio-group v-model="trendMode" size="small" aria-label="数据视图">
            <el-radio-button value="realtime">实时趋势</el-radio-button>
            <el-radio-button value="history">历史查询</el-radio-button>
          </el-radio-group>
        </div>

        <div v-if="trendMode === 'history'" class="control-group control-group--history">
          <span class="control-label">查询范围</span>
          <el-date-picker
            v-model="historyRange"
            type="datetimerange"
            range-separator="至"
            start-placeholder="开始时间"
            end-placeholder="结束时间"
            format="YYYY-MM-DD HH:mm:ss"
            class="history-range"
          />
          <el-button size="small" type="primary" @click="queryHistory">加载曲线</el-button>
          <el-button size="small" type="danger" plain @click="openHistoryCleanup">清理历史</el-button>
        </div>
        <div v-else class="realtime-note"><span class="live-dot"></span>自动刷新最新样本</div>

        <span class="control-separator"></span>

        <div class="control-group control-group--template">
          <span class="control-label">看板模板</span>
          <el-select v-model="activeTemplateId" size="small" class="template-select" @change="loadTemplate" clearable placeholder="选择已保存模板">
            <el-option v-for="tpl in templates" :key="tpl.id" :label="tpl.name" :value="tpl.id" />
          </el-select>
          <div class="template-actions">
            <el-button size="small" @click="saveTemplate" :disabled="!activeTemplateId || allTraces.length === 0">覆盖保存</el-button>
            <el-button size="small" @click="showSaveAs = true">另存为</el-button>
            <el-button size="small" @click="deleteTemplate" :disabled="!activeTemplateId" type="danger" text>删除</el-button>
          </div>
        </div>

        <div class="control-group control-group--layout">
          <span class="control-label">工作区布局</span>
          <el-radio-group v-model="layoutMode" size="small" aria-label="工作区布局">
            <el-radio-button value="single">单列</el-radio-button>
            <el-radio-button value="horizontal">左右</el-radio-button>
            <el-radio-button value="vertical">上下</el-radio-button>
          </el-radio-group>
          <el-button size="small" type="primary" plain @click="addEmptyPanel">添加面板</el-button>
        </div>
      </div>
    </el-card>

    <div :class="['panel-grid', `panel-grid--${layoutMode}`]">
      <TrendPanel
        v-for="p in panels"
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

    <el-dialog v-model="showSaveAs" title="另存为模板" width="360px">
      <el-input v-model="newTemplateName" placeholder="模板名称，如：变电站 A 看板" />
      <template #footer>
        <el-button @click="showSaveAs = false">取消</el-button>
        <el-button type="primary" @click="saveAsTemplate" :disabled="!newTemplateName.trim()">保存模板</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showAddPanel" title="添加面板 — 选择模板" width="400px">
      <el-select v-model="selectedTemplateForPanel" style="width: 100%" placeholder="选择模板">
        <el-option v-for="tpl in templates" :key="tpl.id" :label="tpl.name" :value="tpl.id" />
      </el-select>
      <template #footer>
        <el-button @click="showAddPanel = false">取消</el-button>
        <el-button type="primary" @click="addPanel" :disabled="!selectedTemplateForPanel">添加面板</el-button>
      </template>
    </el-dialog>

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
        <el-form-item label="别名"><el-input v-model="addTraceAlias" placeholder="可选" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddTrace = false">取消</el-button>
        <el-button type="primary" @click="confirmAddTrace" :disabled="addTraceIoas.length === 0">确认添加（{{ addTraceIoas.length }}）</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showHistoryCleanup" title="清理测点历史数据" width="560px" @opened="loadCleanupInstances">
      <el-alert title="该操作会永久删除所选测点在数据库中的全部历史样本，无法恢复。运行中的 AI、DI、PI 会在下一个采样周期重新写入新样本。" type="warning" :closable="false" show-icon class="cleanup-alert" />
      <el-form label-width="72px">
        <el-form-item label="实例">
          <el-select v-model="cleanupInstanceId" filterable style="width: 100%" @change="loadCleanupPoints">
            <el-option v-for="inst in allInstances" :key="inst.id" :label="inst.name + ' (' + inst.id + ')'" :value="inst.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="测点">
          <el-select v-model="cleanupIOAs" filterable multiple collapse-tags collapse-tags-tooltip style="width: 100%" :loading="cleanupPointsLoading" :disabled="!cleanupInstanceId" placeholder="仅显示存在历史数据的测点">
            <el-option v-for="point in cleanupPoints" :key="point.ioa" :label="`${point.name || '未命名'} · ${point.point_type} · IOA:${point.ioa}`" :value="point.ioa" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="cleanupIOAs.length > 0" label="删除范围"><span class="cleanup-count">将清理 {{ cleanupIOAs.length }} 个测点的全部已持久化历史数据</span></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showHistoryCleanup = false">取消</el-button>
        <el-button type="danger" :loading="cleanupSubmitting" :disabled="cleanupIOAs.length === 0" @click="confirmHistoryCleanup">清理所选历史</el-button>
      </template>
    </el-dialog>
  </section>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listInstances, getPoints, getMicrogridPoints, getPersistenceStatus, getLatestPersistedSnapshot, deletePointHistory, listTrendTemplates, saveTrendTemplate, deleteTrendTemplate, type PointSnapshot, type PersistedPointSnapshot, type TrendTemplate as ApiTrendTemplate } from '../api'
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
  created_at: number
  updated_at: number
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

function readLocalTemplates(): Template[] {
  try {
    const raw = localStorage.getItem('trend_templates')
    if (!raw) return []
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.map((item: any) => ({
      ...item,
      created_at: item.created_at || item.createdAt || Date.now(),
      updated_at: item.updated_at || item.updatedAt || item.createdAt || Date.now(),
    })).filter((item: Template) => item.id && item.name)
  } catch {
    return []
  }
}

function saveLocalTemplates(items = templates.value) {
  try {
    localStorage.setItem('trend_templates', JSON.stringify(items))
  } catch (err) {
    console.error('保存模板本地备份失败:', err)
  }
}

function apiToTemplate(item: ApiTrendTemplate): Template {
  return {
    id: item.id,
    name: item.name,
    panels: (item.panels || []).map(panel => ({
      kind: panel.kind,
      traceConfigs: (panel.trace_configs || []).map(trace => ({
        instId: trace.inst_id,
        inst: trace.inst,
        ioa: trace.ioa,
        name: trace.name,
        unit: trace.unit,
        alias: trace.alias,
        colorIdx: trace.color_idx,
        pointType: isPointType(trace.point_type) ? trace.point_type : undefined,
      })),
    })),
    created_at: item.created_at,
    updated_at: item.updated_at,
  }
}

function templateToApi(item: Template): ApiTrendTemplate {
  return {
    id: item.id,
    name: item.name,
    panels: (item.panels || []).map(panel => ({
      kind: panel.kind,
      trace_configs: panel.traceConfigs.map(trace => ({
        inst_id: trace.instId,
        inst: trace.inst,
        ioa: trace.ioa,
        name: trace.name,
        unit: trace.unit,
        alias: trace.alias,
        color_idx: trace.colorIdx,
        point_type: trace.pointType,
      })),
    })),
    created_at: item.created_at,
    updated_at: item.updated_at,
  }
}

async function loadTemplates() {
  const localTemplates = readLocalTemplates()
  try {
    const remoteTemplates = (await listTrendTemplates()).map(apiToTemplate)
    if (remoteTemplates.length === 0 && localTemplates.length > 0) {
      // One-time migration for users upgrading from the localStorage-only version.
      const migrated: Template[] = []
      for (const local of localTemplates) {
        let candidate = local
        if ((!candidate.panels || candidate.panels.length === 0) && candidate.traces?.length) {
          const legacyTraces = cloneTraces(candidate.traces)
          if (!await hydrateTracePointTypes(legacyTraces)) continue
          candidate = { ...candidate, panels: splitTracesByKind(legacyTraces) }
        }
        try {
          migrated.push(apiToTemplate(await saveTrendTemplate(templateToApi(candidate))))
        } catch {
          // Keep successfully migrated items and fall back to local data below.
        }
      }
      if (migrated.length === localTemplates.length) {
        templates.value = migrated
        localStorage.removeItem('trend_templates')
        ElMessage.success(`已迁移 ${migrated.length} 个趋势模板到服务端配置`)
        return
      }
      templates.value = migrated.length > 0 ? migrated : localTemplates
      saveLocalTemplates(templates.value)
      return
    }
    templates.value = remoteTemplates
    if (remoteTemplates.length > 0) localStorage.removeItem('trend_templates')
  } catch (err) {
    templates.value = localTemplates
    if (localTemplates.length > 0) {
      ElMessage.warning('趋势模板服务暂不可用，当前使用浏览器中的兼容副本')
    } else {
      console.error('加载趋势模板失败:', err)
    }
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

async function saveTemplate() {
  if (!activeTemplateId.value) {
    ElMessage.warning('请先选择一个模板，或使用「另存为」创建新模板')
    return
  }
  const index = templates.value.findIndex(item => item.id === activeTemplateId.value)
  if (index < 0) return
  const next: Template = {
    ...templates.value[index],
    panels: panelSnapshot(),
    traces: undefined,
  }
  try {
    templates.value[index] = apiToTemplate(await saveTrendTemplate(templateToApi(next)))
    ElMessage.success('模板已保存到服务端配置')
  } catch (err) {
    saveLocalTemplates()
    ElMessage.error('模板保存失败，已保留浏览器兼容副本')
    console.error('保存趋势模板失败:', err)
  }
}

async function saveAsTemplate() {
  const name = newTemplateName.value.trim()
  if (!name) return
  const next: Template = {
    id: genId(),
    name,
    panels: panelSnapshot(),
    created_at: Date.now(),
    updated_at: Date.now(),
  }
  try {
    const saved = apiToTemplate(await saveTrendTemplate(templateToApi(next)))
    templates.value.push(saved)
    activeTemplateId.value = saved.id
    showSaveAs.value = false
    newTemplateName.value = ''
    ElMessage.success('模板已保存到服务端配置')
  } catch (err) {
    templates.value.push(next)
    activeTemplateId.value = next.id
    saveLocalTemplates()
    showSaveAs.value = false
    newTemplateName.value = ''
    ElMessage.error('服务端保存失败，已保存浏览器兼容副本')
    console.error('另存为趋势模板失败:', err)
  }
}

function deleteTemplate() {
  if (!activeTemplateId.value) return
  ElMessageBox.confirm('确定删除该模板？', '确认', { type: 'warning' }).then(async () => {
    const id = activeTemplateId.value
    try {
      await deleteTrendTemplate(id)
      templates.value = templates.value.filter(template => template.id !== id)
      activeTemplateId.value = ''
      ElMessage.success('模板已删除')
    } catch (err) {
      saveLocalTemplates()
      ElMessage.error('模板删除失败，服务端配置未改变')
      console.error('删除趋势模板失败:', err)
    }
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
  saveTimer = setTimeout(() => { savePanels() }, 500)
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
  await loadTemplates()
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
})
</script>

<style scoped>
.trend-page {
  --trend-surface: rgba(11, 20, 38, 0.88);
  --trend-surface-strong: #0d1a30;
  --trend-line: rgba(91, 117, 155, 0.28);
  --trend-muted: #8da1bd;
  --trend-bright: #e7effd;
  --trend-blue: #4d94ff;
  --trend-amber: #f0a636;
}
.trend-command-deck {
  position: relative;
  margin-bottom: 14px;
  overflow: hidden;
  background: linear-gradient(118deg, rgba(13, 27, 49, 0.98), rgba(9, 18, 34, 0.98));
  border: 1px solid var(--trend-line);
  box-shadow: 0 16px 38px rgba(2, 8, 23, 0.2), inset 0 1px 0 rgba(148, 190, 255, 0.05);
}
.trend-command-deck::before {
  position: absolute;
  inset: 0;
  pointer-events: none;
  content: '';
  background-image: linear-gradient(rgba(100, 160, 255, 0.025) 1px, transparent 1px), linear-gradient(90deg, rgba(100, 160, 255, 0.025) 1px, transparent 1px);
  background-size: 22px 22px;
  mask-image: linear-gradient(90deg, black, transparent 76%);
}
.trend-command-deck :deep(.el-card__body) { position: relative; padding: 0; }
.deck-heading { display: flex; align-items: center; justify-content: space-between; gap: 24px; padding: 17px 20px 15px; border-bottom: 1px solid var(--trend-line); }
.deck-title-block { min-width: 0; }
.deck-kicker { display: block; margin-bottom: 5px; color: #6b87ad; font-family: Consolas, 'Courier New', monospace; font-size: 10px; font-weight: 700; letter-spacing: 0.14em; }
.deck-title-row { display: flex; align-items: center; gap: 10px; }
.deck-title-row h1 { margin: 0; color: var(--trend-bright); font-size: 19px; font-weight: 650; letter-spacing: 0.02em; }
.deck-title-block p { margin: 5px 0 0; color: var(--trend-muted); font-size: 12px; line-height: 1.5; }
.mode-indicator { display: inline-flex; align-items: center; gap: 6px; padding: 3px 8px; border: 1px solid rgba(78, 150, 255, 0.3); border-radius: 999px; color: #9bc5ff; background: rgba(42, 110, 205, 0.12); font-size: 11px; white-space: nowrap; }
.mode-indicator i, .live-dot { width: 6px; height: 6px; border-radius: 50%; background: #4fcb9c; box-shadow: 0 0 0 3px rgba(79, 203, 156, 0.12), 0 0 10px rgba(79, 203, 156, 0.7); }
.mode-indicator.is-history { color: #f2be68; border-color: rgba(240, 166, 54, 0.28); background: rgba(240, 166, 54, 0.1); }
.mode-indicator.is-history i { background: #f0a636; box-shadow: 0 0 0 3px rgba(240, 166, 54, 0.12); }
.deck-summary { display: flex; align-items: center; flex-shrink: 0; gap: 13px; padding: 8px 12px; border: 1px solid rgba(109, 138, 179, 0.23); border-radius: 8px; background: rgba(3, 11, 24, 0.34); }
.summary-item { display: grid; gap: 1px; min-width: 27px; text-align: center; }
.summary-item strong { color: #e7effd; font-family: Consolas, 'Courier New', monospace; font-size: 15px; font-variant-numeric: tabular-nums; }
.summary-item span, .summary-layout { color: #778eae; font-size: 10px; white-space: nowrap; }
.summary-divider { width: 1px; height: 23px; background: var(--trend-line); }
.summary-layout { color: #adc0d9; }
.deck-controls { display: flex; align-items: center; flex-wrap: wrap; gap: 11px 16px; padding: 12px 20px; }
.control-group { display: flex; align-items: center; gap: 7px; min-height: 30px; }
.control-label { color: #7087a8; font-size: 10px; font-weight: 700; letter-spacing: 0.05em; text-transform: uppercase; white-space: nowrap; }
.control-group--history { flex: 1 1 520px; }
.control-group--template { flex: 1 1 360px; }
.control-group--layout { margin-left: auto; }
.history-range { width: 370px; max-width: 100%; }
.template-select { width: 190px; }
.template-actions { display: flex; align-items: center; gap: 2px; }
.control-separator { width: 1px; align-self: stretch; min-height: 26px; background: var(--trend-line); }
.realtime-note { display: inline-flex; align-items: center; gap: 7px; color: #98b9aa; font-size: 12px; }
.realtime-note .live-dot { display: inline-block; }
.trend-command-deck :deep(.el-radio-button__inner) { min-width: 54px; border-color: rgba(91, 117, 155, 0.38); color: #9bb0ca; background: rgba(10, 22, 41, 0.54); box-shadow: none; }
.trend-command-deck :deep(.el-radio-button__original-radio:checked + .el-radio-button__inner) { color: #e9f3ff; border-color: #397de0; background: linear-gradient(180deg, #2a68bd, #23549a); box-shadow: -1px 0 0 0 #397de0; }
.trend-command-deck :deep(.el-button--default) { border-color: rgba(91, 117, 155, 0.38); color: #afc0d7; background: rgba(15, 31, 54, 0.65); }
.trend-command-deck :deep(.el-button--default:hover) { color: #e7effd; border-color: #4d94ff; background: rgba(47, 107, 191, 0.17); }
.trend-command-deck :deep(.el-button--primary) { box-shadow: 0 5px 14px rgba(30, 91, 180, 0.2); }
.panel-grid { display: grid; gap: 14px; align-items: start; }
.panel-grid--single { grid-template-columns: minmax(0, 1fr); }
.panel-grid--horizontal { grid-template-columns: repeat(2, minmax(0, 1fr)); }
.panel-grid--vertical { grid-template-columns: minmax(0, 1fr); max-height: calc(100vh - 214px); overflow-y: auto; padding-right: 3px; }
.cleanup-alert { margin-bottom: 16px; }
.cleanup-count { color: var(--el-color-danger); }
@media (max-width: 1180px) { .deck-heading { align-items: flex-start; } .control-group--layout { margin-left: 0; } .control-separator { display: none; } }
@media (max-width: 800px) { .deck-heading { flex-direction: column; gap: 12px; padding: 15px; } .deck-summary { width: 100%; box-sizing: border-box; } .deck-controls { padding: 12px 15px; gap: 10px; } .control-group--history, .control-group--template { flex-basis: 100%; flex-wrap: wrap; } .history-range { width: 100%; } .template-select { flex: 1; min-width: 160px; } .panel-grid--horizontal { grid-template-columns: minmax(0, 1fr); } }
@media (max-width: 520px) { .deck-title-row h1 { font-size: 17px; } .deck-title-block p { font-size: 11px; } .summary-layout { display: none; } .control-group--layout { flex-wrap: wrap; } }
@media (prefers-reduced-motion: reduce) { .mode-indicator i, .live-dot { box-shadow: none; } }
</style>
