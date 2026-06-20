<template>
  <div class="proxy-page">
    <div class="proxy-layout">
      <ProxyCollectionPanel
        :collections="collections"
        :active-request-id="activeRequestId"
        :active-folder-id="activeFolderId"
        @add-folder="addFolder"
        @add-request="addRequest"
        @select-folder="selectFolder"
        @select-request="loadRequest"
        @rename-folder="renameFolder"
        @delete-folder="deleteFolder"
        @copy-request="copyRequest"
        @delete-request="deleteReq"
        @move-request="handleMoveRequest"
      />

      <div class="editor-panel">
        <ProxyRequestBar
          :method="request.method"
          :url="request.url"
          :sending="sending"
          @update:method="request.method = $event"
          @update:url="request.url = $event"
          @send="send"
          @save="saveCurrentRequest"
          @export-gridsim="exportConfig"
          @export-postman="exportPostman"
          @import="(fmt: any) => triggerImport(fmt)"
        />

        <div class="tabs-area">
          <div v-for="tab in tabList" :key="tab.key" class="tab" :class="{ active: activeTab === tab.key }"
            @click="activeTab = tab.key">{{ tab.label }}</div>
        </div>

        <div class="content-split">
          <div class="request-panel">
            <div class="panel-header"><span>请求配置</span></div>
            <div class="panel-body">
              <div v-if="activeTab === 'headers'">
                <div v-for="(h, i) in headerList" :key="i" class="kv-row">
                  <el-checkbox v-model="h.enabled" />
                  <el-input v-model="h.key" placeholder="Key" size="small" />
                  <el-input v-model="h.value" placeholder="Value" size="small" />
                  <el-button text size="small" @click="headerList.splice(i, 1)">×</el-button>
                </div>
                <el-button size="small" @click="headerList.push({ key: '', value: '', enabled: true })">+ 添加</el-button>
              </div>
              <div v-if="activeTab === 'body'">
                <el-radio-group v-model="bodyType" size="small" style="margin-bottom: 8px;">
                  <el-radio-button value="json">JSON</el-radio-button>
                  <el-radio-button value="text">Text</el-radio-button>
                  <el-radio-button value="none">None</el-radio-button>
                </el-radio-group>
                <el-input v-if="bodyType !== 'none'" v-model="request.body" type="textarea" :rows="8" placeholder="请求体..." />
              </div>
              <div v-if="activeTab === 'pre-script'">
                <div style="font-size: 12px; color: #94a3b8; margin-bottom: 8px;">
                  发送前执行脚本，用 <code style="background: #1a1f2e; padding: 1px 6px; border-radius: 3px; color: #f59e0b; font-family: 'JetBrains Mono', monospace; font-size: 11px;">vars.变量名 = 值</code> 给变量赋值
                </div>
                <el-input v-model="request.pre_script" type="textarea" :rows="10" @input="scheduleAutoSave"
                  placeholder="示例：&#10;vars.timestamp = $now()&#10;vars.date = $formatTime('yyyy-MM-dd')"
                  style="font-family: 'JetBrains Mono', monospace; font-size: 12px;" />
                <div class="env-vars-box">
                  <div class="env-vars-header">
                    <span class="env-vars-title">当前环境: <span style="color: #f59e0b;">{{ activeEnvName }}</span></span>
                  </div>
                  <div v-if="Object.keys(activeEnvVars).length" class="env-vars-grid">
                    <div v-for="(v, k) in activeEnvVars" :key="k" class="env-var-item">
                      <span class="env-var-key">{{ k }}</span>
                      <span class="env-var-eq">=</span>
                      <span class="env-var-val">{{ v }}</span>
                    </div>
                  </div>
                </div>
              </div>
              <div v-if="activeTab === 'post-script'">
                <div style="font-size: 12px; color: #94a3b8; margin-bottom: 8px;">
                  发送后执行脚本，用 <code style="background: #1a1f2e; padding: 1px 6px; border-radius: 3px; color: #f59e0b; font-family: 'JetBrains Mono', monospace; font-size: 11px;">pm.response.json()</code> 提取响应内容
                </div>
                <el-input v-model="request.test_script" type="textarea" :rows="10" @input="scheduleAutoSave"
                  placeholder="示例：&#10;var data = pm.response.json()"
                  style="font-family: 'JetBrains Mono', monospace; font-size: 12px;" />
              </div>
            </div>
          </div>

          <ProxyResponsePanel :response="response" @toggle-history="showHistory = true" />
        </div>

        <div class="env-bar">
          <span style="font-size: 12px; color: #94a3b8;">环境:</span>
          <el-select v-model="activeEnvId" size="small" @change="switchEnv" style="width: 140px;">
            <el-option v-for="env in environments" :key="env.id" :value="env.id" :label="env.name" />
          </el-select>
          <el-button text size="small" @click="showEnvModal = true">⚙ 管理</el-button>
        </div>
      </div>
    </div>

    <el-drawer v-model="showHistory" title="请求历史" size="640px">
      <div v-for="(h, i) in history" :key="i" class="history-item" @click="toggleHistoryDetail(i)">
        <div style="display: flex; align-items: center; gap: 8px;">
          <span class="history-method" :class="h.method">{{ h.method }}</span>
          <span class="history-url">{{ h.resolved_url || h.url }}</span>
        </div>
        <div class="history-meta">
          <span :style="{ color: h.status < 400 ? '#22c55e' : '#ef4444' }">● {{ h.status }} {{ h.status_text || '' }}</span>
          <span>⏱ {{ h.time_ms }}ms</span>
          <span>📦 {{ h.size }}B</span>
        </div>
        <div v-if="historyOpen[i]" class="history-detail">
          <div class="detail-section">
            <div class="detail-label">🔗 最终 URL</div>
            <pre class="detail-pre">{{ h.resolved_url || h.url }}</pre>
          </div>
          <div v-if="h.response_body" class="detail-section">
            <div class="detail-label">📄 响应 Body</div>
            <pre class="detail-pre">{{ h.response_body }}</pre>
          </div>
        </div>
      </div>
    </el-drawer>

    <el-dialog v-model="showEnvModal" title="环境变量管理" width="620px">
      <div style="display: flex; gap: 16px; min-height: 320px;">
        <div style="width: 140px; border-right: 1px solid #1e293b; padding-right: 12px; flex-shrink: 0;">
          <div v-for="env in environments" :key="env.id" class="env-item" :class="{ active: env.id === activeEnvId }"
            style="display:flex;align-items:center;justify-content:space-between;padding:6px 8px">
            <span style="flex:1;cursor:pointer;overflow:hidden;text-overflow:ellipsis;white-space:nowrap"
              @click="editEnv = { ...env, variables: { ...(env.variables || {}) } }">
              {{ env.name }}
              <span v-if="env.id === activeEnvId" style="color:#f59e0b;font-size:10px;margin-left:4px">●</span>
            </span>
            <el-button v-if="environments.length > 1" text size="small" type="danger" style="padding:0 4px;font-size:12px"
              @click.stop="deleteEntireEnv(env.id)" title="删除">×</el-button>
          </div>
          <el-button size="small" style="margin-top:8px;width:100%" @click="addEnvironment">+ 新建</el-button>
        </div>
        <div style="flex:1" v-if="editEnv">
          <el-form label-position="top" size="small">
            <el-form-item label="环境名称">
              <el-input v-model="editEnv.name" />
            </el-form-item>
            <el-form-item label="变量">
              <div v-for="(val, key) in editEnv.variables" :key="key" class="kv-row" style="margin-bottom:6px">
                <el-input :model-value="key" size="small" disabled />
                <el-input :model-value="val" size="small" disabled />
                <el-button text size="small" type="danger" @click="delete editEnv.variables[key]">🗑</el-button>
              </div>
              <div class="kv-row" style="margin-top:8px;padding-top:8px;border-top:1px solid #1e293b">
                <el-input v-model="newVarKey" size="small" placeholder="变量名" style="flex:0.5" />
                <el-input v-model="newVarValue" size="small" placeholder="值" style="flex:0.5" />
                <el-button size="small" type="warning" @click="addVar">+ 添加</el-button>
              </div>
            </el-form-item>
          </el-form>
        </div>
      </div>
      <template #footer>
        <el-button @click="showEnvModal = false">取消</el-button>
        <el-button type="warning" @click="saveEnvs">保存</el-button>
      </template>
    </el-dialog>

    <input ref="importFileInput" type="file" accept=".json" style="display:none" @change="handleImportFile" />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'

import ProxyCollectionPanel from '../components/proxy/ProxyCollectionPanel.vue'
import ProxyRequestBar from '../components/proxy/ProxyRequestBar.vue'
import ProxyResponsePanel from '../components/proxy/ProxyResponsePanel.vue'

import {
  proxyRequest, getCollections, saveCollection, deleteCollection,
  getEnvironments, saveEnvironment, deleteEnvironment, activateEnvironment, exportProxyConfig,
  type CollectionItem, type ProxyEnvironment, type ProxyResponse
} from '../api'

const tabList = [
  { key: 'headers', label: 'Headers' },
  { key: 'body', label: 'Body' },
  { key: 'pre-script', label: 'Pre-Script' },
  { key: 'post-script', label: 'Post-Script' },
]

const activeTab = ref('headers')
const bodyType = ref('json')
const sending = ref(false)
const showHistory = ref(false)
const showEnvModal = ref(false)
const importFileInput = ref<HTMLInputElement | null>(null)
const importFormat = ref<'gridsim' | 'postman'>('gridsim')

const request = reactive({ method: 'GET', url: '', body: '', pre_script: '', test_script: '' })
const headerList = ref<{ key: string; value: string; enabled: boolean }[]>([])
const response = ref<ProxyResponse | null>(null)
const activeRequestId = ref('')
const activeFolderId = ref('')

const collections = ref<(CollectionItem & { _open?: boolean })[]>([])
const environments = ref<ProxyEnvironment[]>([])
const activeEnvId = ref('')
const editEnv = ref<ProxyEnvironment | null>(null)
const history = ref<any[]>([])
const historyOpen = ref<Record<number, boolean>>({})
const newVarKey = ref('')
const newVarValue = ref('')
let scriptSaveTimer: ReturnType<typeof setTimeout> | null = null

const activeEnvName = computed(() => environments.value.find(e => e.id === activeEnvId.value)?.name || '无')
const activeEnvVars = computed(() => environments.value.find(e => e.id === activeEnvId.value)?.variables || {})

function genId() { return Date.now().toString(36) + Math.random().toString(36).slice(2, 8) }

function scheduleAutoSave(delay = 500) {
  if (scriptSaveTimer) clearTimeout(scriptSaveTimer)
  scriptSaveTimer = setTimeout(() => autoSaveRequest(), delay)
}

function buildTimeHelpers() {
  const now = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  const fmt = (f: string) => f.replace('yyyy', String(now.getFullYear())).replace('MM', pad(now.getMonth() + 1))
    .replace('dd', pad(now.getDate())).replace('HH', pad(now.getHours())).replace('mm', pad(now.getMinutes())).replace('ss', pad(now.getSeconds()))
  return {
    $now: () => now.toISOString(),
    $formatTime: fmt,
    $timestamp: () => String(Math.floor(now.getTime() / 1000)),
    $uuid: () => Date.now().toString(36) + Math.random().toString(36).slice(2, 10),
  }
}

function resolveJsonPath(root: any, path: string): any {
  if (!path) return root
  const tokens: (string | number)[] = []
  const re = /([^.[\]]+)|\[(\d+)\]/g
  let m: RegExpExecArray | null
  while ((m = re.exec(path)) !== null) {
    if (m[2] !== undefined) tokens.push(Number(m[2]))
    else tokens.push(m[1])
  }
  let cur: any = root
  for (const t of tokens) {
    if (cur == null) return undefined
    cur = cur[t]
  }
  return cur
}

function buildPmSandbox(vars: Record<string, string>, res: ProxyResponse | null) {
  const envGet = (k: string) => activeEnvVars.value[k] ?? ''
  const envSet = (k: string, v: any) => {
    const env = environments.value.find(e => e.id === activeEnvId.value)
    if (!env) return
    env.variables[k] = String(v)
    saveEnvironment(env).catch((e: any) => ElMessage.error('保存环境变量失败: ' + e.message))
  }
  const responseObj: any = {
    code: res?.status ?? 0,
    status: res?.status_text ?? '',
    headers: { get: (k: string) => {
      if (!res?.headers) return undefined
      const lower = k.toLowerCase()
      for (const [hk, hv] of Object.entries(res.headers)) {
        if (hk.toLowerCase() === lower) return hv as string
      }
      return undefined
    }},
    text: () => res?.body ?? '',
    json: (path?: string) => {
      let parsed: any
      try { parsed = JSON.parse(res?.body || '{}') } catch { parsed = {} }
      return path ? resolveJsonPath(parsed, path) : parsed
    },
  }
  return {
    response: responseObj,
    vars: {
      get: (k: string) => vars[k] ?? '',
      set: (k: string, v: any) => { vars[k] = String(v) },
      has: (k: string) => k in vars,
      toObject: () => ({ ...vars }),
    },
    environment: { get: envGet, set: envSet, has: (k: string) => k in activeEnvVars.value },
    expect: (actual: any) => {
      const result = (pass: boolean, expected: any) => {
        const r = { pass, message: pass ? '断言通过' : `断言失败: 期望 ${JSON.stringify(expected)}，实际 ${JSON.stringify(actual)}` }
        if (!pass) { const e: any = new Error(r.message); e.pass = r.pass; throw e }
        return r
      }
      return { to: { equal: (e: any) => result(actual === e, e) }, equal: (e: any) => result(actual === e, e) }
    },
  }
}

function runPostScript(testScript: string, vars: Record<string, string>, res: ProxyResponse | null) {
  if (!testScript || !testScript.trim()) return
  const helpers = buildTimeHelpers()
  const pm = buildPmSandbox(vars, res)
  const scriptFn = new Function('vars', 'pm', '$now', '$formatTime', '$timestamp', '$uuid', `'use strict';\n${testScript}`)
  scriptFn(vars, pm, helpers.$now, helpers.$formatTime, helpers.$timestamp, helpers.$uuid)
}

async function loadData() {
  collections.value = await getCollections()
  const envData = await getEnvironments()
  environments.value = envData.environments || []
  activeEnvId.value = envData.active_id || ''
  history.value = JSON.parse(localStorage.getItem('proxy_history') || '[]')
}

async function send() {
  if (!request.url) { ElMessage.warning('请输入 URL'); return }
  sending.value = true
  response.value = null
  try {
    const headers: Record<string, string> = {}
    headerList.value.filter(h => h.enabled && h.key).forEach(h => { headers[h.key] = h.value })

    const vars: Record<string, string> = { ...activeEnvVars.value }

    if (request.pre_script && request.pre_script.trim()) {
      try {
        const helpers = buildTimeHelpers()
        const scriptFn = new Function('vars', '$now', '$formatTime', '$timestamp', '$uuid', request.pre_script)
        scriptFn(vars, helpers.$now, helpers.$formatTime, helpers.$timestamp, helpers.$uuid)
      } catch (e: any) { ElMessage.error('Pre-Script 执行出错: ' + e.message); sending.value = false; return }
    }

    const resolve = (s: string) => s.replace(/\{\{([A-Za-z_][A-Za-z0-9_.]*)\}\}/g, (_, k) => vars[k] !== undefined ? vars[k] : `{{${k}}}`)
    const res = await proxyRequest({
      method: request.method, url: resolve(request.url),
      headers: Object.fromEntries(Object.entries(headers).map(([k, v]) => [k, resolve(v)])),
      body: bodyType.value === 'none' ? '' : resolve(request.body), timeout: 30,
    })
    response.value = res

    const entry = {
      method: request.method, url: request.url, resolved_url: resolve(request.url),
      headers, status: res.status, status_text: res.status_text,
      time_ms: res.time_ms, size: res.size,
      response_body: res.body?.length > 102400 ? res.body.slice(0, 102400) + '\n...[truncated]' : res.body,
      timestamp: new Date().toLocaleTimeString(),
    }
    history.value.unshift(entry)
    if (history.value.length > 50) history.value.pop()
    localStorage.setItem('proxy_history', JSON.stringify(history.value))

    if (request.test_script && request.test_script.trim()) {
      try { runPostScript(request.test_script, vars, res) }
      catch (e: any) {
        if (e && typeof e === 'object' && 'pass' in e) {
          e.pass ? ElMessage.success(e.message) : ElMessage.warning(e.message)
        } else { ElMessage.error('Post-Script 出错: ' + (e?.message || e)) }
      }
    }
    await autoSaveRequest()
  } catch (e: any) {
    response.value = { status: 0, status_text: 'Error', headers: {}, body: '', time_ms: 0, size: 0, error: e.message }
  } finally { sending.value = false }
}

function addFolder() {
  ElMessageBox.prompt('文件夹名称', '新建文件夹').then(({ value }) => {
    if (!value) return
    const folder: any = { id: genId(), name: value, type: 'folder', children: [], _open: true }
    collections.value.push(folder)
    saveCollection(folder)
  })
}

function addRequest() {
  ElMessageBox.prompt('请求名称', '新建请求').then(({ value }) => {
    if (!value) return
    const req: CollectionItem = { id: genId(), name: value, type: 'request', method: 'GET', url: '', headers: {}, body: '', pre_script: '', test_script: '' }
    if (collections.value.length === 0) {
      const folder: any = { id: genId(), name: '默认文件夹', type: 'folder', children: [req], _open: true }
      collections.value.push(folder); saveCollection(folder); activeFolderId.value = folder.id
    } else {
      let folder = activeFolderId.value ? collections.value.find(f => f.id === activeFolderId.value) : null
      if (!folder) folder = collections.value[0]
      if (!folder.children) folder.children = []
      folder.children.push(req); saveCollection(folder); activeFolderId.value = folder.id
    }
    activeRequestId.value = req.id
    loadRequest(req)
  })
}

function selectFolder(folder: any) {
  activeFolderId.value = folder.id
  folder._open = !folder._open
}

function handleMoveRequest(reqId: string, fromFolder: any, toFolder: any) {
  if (!fromFolder?.children) return
  const idx = fromFolder.children.findIndex((r: any) => r.id === reqId)
  if (idx < 0) return
  const [req] = fromFolder.children.splice(idx, 1)
  saveCollection(fromFolder)
  if (!toFolder.children) toFolder.children = []
  toFolder.children.push(req)
  toFolder._open = true
  saveCollection(toFolder)
}

function loadRequest(req: CollectionItem) {
  activeRequestId.value = req.id
  request.method = req.method || 'GET'
  request.url = req.url || ''
  request.body = req.body || ''
  request.pre_script = req.pre_script || ''
  request.test_script = req.test_script || ''
  headerList.value = Object.entries(req.headers || {}).map(([k, v]) => ({ key: k, value: v, enabled: true }))
}

function copyRequest(req: CollectionItem) {
  const copy: CollectionItem = { ...req, id: genId(), name: req.name + ' (副本)' }
  const folder = collections.value.find(f => f.children?.some(r => r.id === req.id))
  if (folder) { folder.children!.push(copy); saveCollection(folder) }
}

function deleteReq(id: string) {
  ElMessageBox.confirm('确定删除？', '提示', { type: 'warning' }).then(() => {
    deleteCollection(id)
    collections.value = collections.value.filter(f => {
      if (f.children) f.children = f.children.filter(r => r.id !== id)
      return true
    })
  })
}

function renameFolder(folder: CollectionItem) {
  ElMessageBox.prompt('新名称', '重命名', { inputValue: folder.name }).then(({ value }) => {
    if (!value) return; folder.name = value; saveCollection(folder)
  })
}

function deleteFolder(id: string) {
  ElMessageBox.confirm('确定删除文件夹？', '提示', { type: 'warning' }).then(() => {
    deleteCollection(id)
    collections.value = collections.value.filter(f => f.id !== id)
  })
}

async function saveCurrentRequest() {
  if (!activeRequestId.value) { addRequest(); return }
  await doSaveCurrentRequest()
  ElMessage.success('已保存')
}

async function autoSaveRequest() {
  if (!activeRequestId.value) return
  await doSaveCurrentRequest()
}

async function doSaveCurrentRequest() {
  if (!activeRequestId.value) return
  const headers: Record<string, string> = {}
  headerList.value.filter(h => h.enabled && h.key).forEach(h => { headers[h.key] = h.value })
  for (const folder of collections.value) {
    const req = folder.children?.find(r => r.id === activeRequestId.value)
    if (req) {
      req.method = request.method; req.url = request.url; req.body = request.body
      req.headers = headers; req.pre_script = request.pre_script; req.test_script = request.test_script
      await saveCollection(folder); return
    }
  }
}

function triggerImport(format: 'gridsim' | 'postman') {
  importFormat.value = format
  importFileInput.value?.click()
}

async function handleImportFile(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  const text = await file.text()
  try {
    if (importFormat.value === 'postman') await importPostman(text)
    else await importGridSim(text)
  } catch (e: any) { ElMessage.error('导入失败: ' + e.message) }
  ;(e.target as HTMLInputElement).value = ''
}

async function importGridSim(text: string) {
  const data = JSON.parse(text)
  if (data.collections) {
    for (const item of data.collections) {
      await saveCollection(item)
      if (!collections.value.find(c => c.id === item.id)) collections.value.push(item)
    }
  }
  if (data.environments) {
    for (const env of data.environments) {
      await saveEnvironment(env)
      if (!environments.value.find(e => e.id === env.id)) environments.value.push(env)
    }
  }
  if (data.active_env_id) { activeEnvId.value = data.active_env_id; await activateEnvironment(data.active_env_id) }
  ElMessage.success('导入成功')
}

function isPostmanCollection(data: any): boolean {
  return data?.info?.schema && data.info.schema.includes('postman') && Array.isArray(data.item)
}

function postmanUrlToRaw(p: { host: string; path: string[]; query: { key: string; value: string }[] }): string {
  let raw = (p.host ? p.host + '/' : '') + p.path.join('/')
  if (p.query.length) raw += '?' + p.query.map(q => `${q.key}=${q.value}`).join('&')
  return raw
}

function postmanToGridsimCollection(item: any): CollectionItem | null {
  if (!item || !item.request) return null
  const req = item.request
  const method = (req.method || 'GET').toUpperCase()
  let url = ''
  let headers: Record<string, string> = {}
  let body = ''

  if (typeof req.url === 'string') url = req.url
  else if (req.url) url = postmanUrlToRaw(req.url as any)

  if (req.header) headers = Object.fromEntries((req.header as any[]).filter((h: any) => h.key).map((h: any) => [h.key, h.value || '']))

  if (req.body) {
    if (req.body.mode === 'raw') body = req.body.raw || ''
    else if (req.body.mode === 'urlencoded') {
      body = (req.body.urlencoded as any[] || []).map((p: any) => `${p.key}=${p.value}`).join('&')
      if (!headers['Content-Type']) headers['Content-Type'] = 'application/x-www-form-urlencoded'
    }
  }

  let preScript = ''
  let testScript = ''
  if (item.event) {
    for (const evt of item.event) {
      if (evt.listen === 'prerequest' && evt.script?.exec) preScript += evt.script.exec.join('\n') + '\n'
      if (evt.listen === 'test' && evt.script?.exec) testScript += evt.script.exec.join('\n') + '\n'
    }
    preScript = preScript.trim(); testScript = testScript.trim()
  }

  const children: CollectionItem[] = []
  if (Array.isArray(item.item)) {
    for (const child of item.item) {
      const col = postmanToGridsimCollection(child)
      if (col) children.push(col)
    }
  }
  if (children.length > 0) return { id: genId(), name: item.name || 'Folder', type: 'folder', children, _open: true } as any

  if (!url && !children.length) return null

  return { id: genId(), name: item.name || 'Untitled', type: 'request', method, url, headers, body, pre_script: preScript || undefined, test_script: testScript || undefined }
}

async function importPostman(text: string) {
  const data = JSON.parse(text)
  if (!isPostmanCollection(data)) throw new Error('不是有效的 Postman Collection 格式')
  const newCollections: CollectionItem[] = []
  if (Array.isArray(data.item)) {
    for (const item of data.item) {
      const col = postmanToGridsimCollection(item)
      if (col) newCollections.push(col)
    }
  }
  if (newCollections.length === 0) throw new Error('未找到可导入的请求')
  collections.value.push(...newCollections)
  for (const col of newCollections) await saveCollection(col)
  ElMessage.success(`成功导入 ${newCollections.length} 个请求/文件夹`)
}

async function buildPostmanExport(): Promise<any> {
  const envMap: Record<string, string> = {}
  for (const env of environments.value) {
    if (env.variables) Object.assign(envMap, env.variables)
  }
  const vars: Record<string, string> = {}
  for (const [k, v] of Object.entries(envMap)) vars[k] = `{{${k}}}`

  const postmanItems: any[] = []
  let requestCount = 0

  function convertItem(item: CollectionItem): any {
    if (item.type === 'folder') {
      const children: any[] = []
      if (item.children) for (const child of item.children) children.push(convertItem(child))
      return { name: item.name, item: children }
    }
    requestCount++
    const headers: any[] = []
    if (item.headers) for (const [k, v] of Object.entries(item.headers)) if (k) headers.push({ key: k, value: v, type: 'text' })
    const postmanReq: any = { method: item.method || 'GET', header: headers }
    postmanReq.url = { raw: item.url || '', host: [], path: [] }
    const events: any[] = []
    if (item.pre_script) events.push({ listen: 'prerequest', script: { type: 'text/javascript', exec: item.pre_script.split('\n') } })
    if (item.test_script) events.push({ listen: 'test', script: { type: 'text/javascript', exec: item.test_script.split('\n') } })
    if (events.length) postmanReq.event = events
    if (item.body) { postmanReq.body = { mode: 'raw', raw: item.body, options: { raw: { language: 'json' } } } }
    return { name: item.name || 'Untitled', request: postmanReq, response: [] }
  }

  for (const col of collections.value) postmanItems.push(convertItem(col))

  return {
    info: { _postman_id: genId(), name: 'GridSim Export', schema: 'https://schema.getpostman.com/json/collection/v2.1.0/collection.json' },
    item: postmanItems,
  }
}

async function exportConfig() {
  try {
    const blob = await exportProxyConfig()
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob); a.download = `gridsim-proxy-config-${new Date().toISOString().slice(0, 10)}.json`
    a.click(); URL.revokeObjectURL(a.href)
    ElMessage.success('导出成功')
  } catch (e: any) { ElMessage.error('导出失败: ' + e.message) }
}

async function exportPostman() {
  try {
    const postmanData = await buildPostmanExport()
    const blob = new Blob([JSON.stringify(postmanData, null, 2)], { type: 'application/json' })
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob); a.download = `gridsim-postman-export-${new Date().toISOString().slice(0, 10)}.json`
    a.click(); URL.revokeObjectURL(a.href)
    ElMessage.success('Postman 格式导出成功')
  } catch (e: any) { ElMessage.error('导出失败: ' + e.message) }
}

function toggleHistoryDetail(i: number) { historyOpen.value[i] = !historyOpen.value[i] }

async function switchEnv(id: string) {
  const prevId = activeEnvId.value
  activeEnvId.value = id
  try { await activateEnvironment(id) }
  catch { activeEnvId.value = prevId; ElMessage.error('切换环境失败') }
}

function addEnvironment() {
  ElMessageBox.prompt('环境名称', '新建环境').then(({ value }) => {
    if (!value) return
    const env: ProxyEnvironment = { id: genId(), name: value, variables: {} }
    environments.value.push(env)
    editEnv.value = { ...env, variables: {} }
  })
}

function addVar() {
  if (!editEnv.value || !newVarKey.value.trim()) return
  const key = newVarKey.value.trim()
  if (editEnv.value.variables[key] !== undefined) { ElMessage.warning('变量名已存在'); return }
  editEnv.value.variables[key] = newVarValue.value
  newVarKey.value = ''; newVarValue.value = ''
  ElMessage.success('已添加变量')
}

async function deleteEntireEnv(id: string) {
  const env = environments.value.find(e => e.id === id)
  if (!env) return
  try { await ElMessageBox.confirm(`确定删除环境 "${env.name}"？`, '删除环境', { type: 'warning' }) } catch { return }
  try {
    await deleteEnvironment(id)
    environments.value = environments.value.filter(e => e.id !== id)
    if (editEnv.value?.id === id) editEnv.value = null
    if (activeEnvId.value === id && environments.value.length > 0) {
      activeEnvId.value = environments.value[0].id
      await activateEnvironment(environments.value[0].id)
    }
    ElMessage.success(`已删除环境 "${env.name}"`)
  } catch (e: any) { ElMessage.error('删除环境失败: ' + (e?.message || e)) }
}

async function saveEnvs() {
  if (editEnv.value) {
    const idx = environments.value.findIndex(e => e.id === editEnv.value!.id)
    if (idx !== -1) environments.value[idx] = { ...editEnv.value, variables: { ...editEnv.value.variables } }
  }
  for (const env of environments.value) await saveEnvironment(env)
  showEnvModal.value = false
  ElMessage.success('环境变量已保存')
}

onMounted(loadData)
watch(() => request.url, () => { if (activeRequestId.value) scheduleAutoSave() })
watch(() => request.body, () => { if (activeRequestId.value) scheduleAutoSave() })
watch(() => request.method, () => { if (activeRequestId.value) scheduleAutoSave() })
watch(() => headerList.value, () => { if (activeRequestId.value) scheduleAutoSave() }, { deep: true })
onUnmounted(() => { if (scriptSaveTimer) clearTimeout(scriptSaveTimer) })
</script>

<style scoped>
.proxy-page { height: 100%; display: flex; flex-direction: column; }
.proxy-layout { flex: 1; display: flex; overflow: hidden; }
.editor-panel { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.tabs-area { display: flex; border-bottom: 1px solid #1e293b; background: #111827; padding: 0 16px; }
.tab { padding: 9px 16px; font-size: 13px; font-weight: 500; color: #64748b; cursor: pointer; border-bottom: 2px solid transparent; }
.tab:hover { color: #94a3b8; }
.tab.active { color: #f59e0b; border-bottom-color: #f59e0b; }
.content-split { display: flex; flex: 1; overflow: hidden; }
.request-panel { flex: 1; display: flex; flex-direction: column; border-right: 1px solid #1e293b; overflow: hidden; }
.panel-header { padding: 8px 14px; font-size: 11px; font-weight: 600; color: #64748b; text-transform: uppercase; letter-spacing: 0.5px; background: #111827; border-bottom: 1px solid #1e293b; display: flex; align-items: center; justify-content: space-between; }
.panel-body { flex: 1; overflow-y: auto; padding: 10px 14px; }
.kv-row { display: flex; gap: 6px; margin-bottom: 5px; align-items: center; }
.env-bar { display: flex; align-items: center; gap: 8px; padding: 6px 16px; background: #1a1f2e; border-top: 1px solid #1e293b; }
.env-vars-box { margin-top: 10px; border-top: 1px solid #1e293b; padding-top: 8px; }
.env-vars-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 6px; }
.env-vars-title { font-size: 11px; color: #64748b; }
.env-vars-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 4px 8px; }
.env-var-item { display: flex; align-items: center; gap: 4px; padding: 3px 6px; background: #0d1117; border: 1px solid #1e293b; border-radius: 4px; font-family: 'JetBrains Mono', monospace; font-size: 11px; }
.env-var-key { color: #93c5fd; flex-shrink: 0; max-width: 100px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.env-var-eq { color: #64748b; flex-shrink: 0; }
.env-var-val { color: #e2e8f0; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.history-item { padding: 10px 12px; border-radius: 8px; cursor: pointer; margin-bottom: 4px; }
.history-item:hover { background: #1a1f2e; }
.history-detail { margin-top: 8px; padding-top: 8px; border-top: 1px dashed #1e293b; }
.detail-section { margin-bottom: 10px; }
.detail-label { font-size: 11px; font-weight: 600; color: #94a3b8; margin-bottom: 4px; }
.detail-pre { font-family: 'JetBrains Mono', monospace; font-size: 11px; color: #e2e8f0; background: #0a0e17; border: 1px solid #1e293b; border-radius: 4px; padding: 6px 8px; margin: 0; white-space: pre-wrap; word-break: break-all; max-height: 200px; overflow: auto; }
.history-method { font-family: 'JetBrains Mono', monospace; font-size: 10px; font-weight: 600; padding: 2px 6px; border-radius: 3px; margin-right: 8px; }
.history-method.GET { background: rgba(34,197,94,0.15); color: #22c55e; }
.history-method.POST { background: rgba(245,158,11,0.15); color: #f59e0b; }
.history-method.PUT { background: rgba(59,130,246,0.15); color: #3b82f6; }
.history-method.DELETE { background: rgba(239,68,68,0.15); color: #ef4444; }
.history-url { font-family: 'JetBrains Mono', monospace; font-size: 11px; color: #94a3b8; word-break: break-all; }
.history-meta { margin-top: 4px; font-size: 10px; color: #64748b; display: flex; gap: 10px; }
.env-item { padding: 8px 10px; border-radius: 6px; cursor: pointer; font-size: 12px; color: #94a3b8; margin-bottom: 2px; }
.env-item:hover { background: #1a1f2e; color: #e2e8f0; }
.env-item.active { background: rgba(245,158,11,0.1); color: #f59e0b; }
</style>
