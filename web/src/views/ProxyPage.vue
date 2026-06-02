<template>
  <div class="proxy-page">
    <div class="proxy-layout">
      <div class="collection-panel">
        <div class="collection-header">
          <el-input v-model="searchText" placeholder="搜索请求..." size="small" clearable />
          <div class="collection-actions">
            <el-button size="small" @click="addFolder">📁 文件夹</el-button>
            <el-button size="small" @click="addRequest">＋ 请求</el-button>
          </div>
        </div>
        <div class="collection-tree">
          <div v-for="folder in filteredCollections" :key="folder.id" class="tree-folder">
            <div class="tree-folder-header" @click="folder._open = !folder._open">
              <span class="arrow" :class="{ open: folder._open }">▶</span>
              <span class="folder-icon">📁</span>
              <span class="folder-name">{{ folder.name }}</span>
              <span class="folder-actions">
                <el-button text size="small" @click.stop="renameFolder(folder)">✏</el-button>
                <el-button text size="small" @click.stop="deleteFolder(folder.id)">🗑</el-button>
              </span>
            </div>
            <div class="tree-children" :class="{ collapsed: !folder._open }">
              <div v-for="req in (folder.children || [])" :key="req.id"
                class="tree-request" :class="{ active: req.id === activeRequestId }"
                @click="loadRequest(req)">
                <span class="req-method" :class="req.method">{{ req.method }}</span>
                <span class="req-name">{{ req.name }}</span>
                <span class="req-actions">
                  <el-button text size="small" @click.stop="copyRequest(req)">📋</el-button>
                  <el-button text size="small" @click.stop="deleteReq(req.id)">🗑</el-button>
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="editor-panel">
        <div class="request-bar">
          <el-select v-model="request.method" class="method-select" :class="request.method.toLowerCase()">
            <el-option v-for="m in ['GET','POST','PUT','DELETE','PATCH']" :key="m" :value="m" :label="m" />
          </el-select>
          <el-input v-model="request.url" placeholder="输入 URL，支持 {{variable}}" class="url-input" />
          <el-button type="warning" @click="send" :loading="sending">▶ 发送</el-button>
          <el-button @click="saveCurrentRequest">💾 保存</el-button>
        </div>

        <div class="tabs-area">
          <div v-for="tab in ['headers','body','pre-script']" :key="tab"
            class="tab" :class="{ active: activeTab === tab }" @click="activeTab = tab">
            {{ tab === 'pre-script' ? 'Pre-Script' : tab.charAt(0).toUpperCase() + tab.slice(1) }}
          </div>
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
                  发送前执行脚本，用 <code style="background: #1a1f2e; padding: 1px 6px; border-radius: 3px; color: #f59e0b; font-family: 'JetBrains Mono', monospace; font-size: 11px;" v-text="'vars.变量名 = 值'"></code> 给变量赋值
                </div>
                <el-input v-model="request.pre_script" type="textarea" :rows="10"
                  placeholder="示例：&#10;vars.timestamp = $now()&#10;vars.date = $formatTime('yyyy-MM-dd')&#10;vars.time = $formatTime('HH:mm:ss')&#10;vars.token = 'my-token-123'"
                  style="font-family: 'JetBrains Mono', monospace; font-size: 12px;" />
                <div style="margin-top: 8px; font-size: 11px; color: #64748b; line-height: 1.8;">
                  <div><span style="color: #f59e0b;">内置函数：</span></div>
                  <div><code v-text="'$now()'"></code> — 当前时间戳 (ISO 8601)</div>
                  <div><code v-text="'$formatTime(fmt)'"></code> — 格式化时间，如 <code v-text="'yyyy-MM-dd HH:mm:ss'"></code></div>
                  <div><code v-text="'$timestamp()'"></code> — Unix 秒级时间戳</div>
                  <div><code v-text="'$uuid()'"></code> — 随机 ID</div>
                </div>
                <div style="margin-top: 6px; font-size: 11px; color: #64748b; border-top: 1px solid #1e293b; padding-top: 6px;">
                  当前环境: <span style="color: #f59e0b;">{{ activeEnvName }}</span>
                  <span v-if="Object.keys(activeEnvVars).length"> | 变量: </span>
                  <code v-for="(v, k) in activeEnvVars" :key="k" style="color: #93c5fd; margin-right: 6px;">{{k}}={{v}}</code>
                </div>
              </div>
            </div>
          </div>

          <div class="response-panel">
            <div class="panel-header">
              <span>响应结果</span>
              <el-button text size="small" @click="showHistory = true">🕐 历史</el-button>
            </div>
            <div v-if="response" class="response-status">
              <el-tag :type="response.status < 400 ? 'success' : 'danger'">
                ● {{ response.status }} {{ response.status_text }}
              </el-tag>
              <el-tag type="info">⏱ {{ response.time_ms }} ms</el-tag>
              <el-tag type="info">📦 {{ response.size }} B</el-tag>
            </div>
            <div class="response-body">
              <div v-if="response && !response.error" class="json-view" v-html="highlightJson(response.body)" />
              <div v-else-if="response?.error" style="color: #ef4444; padding: 16px;">{{ response.error }}</div>
              <div v-else class="empty-state">
                <div style="font-size: 40px; opacity: 0.3;">📡</div>
                <p style="font-size: 12px; color: #64748b;">点击「发送」发起请求</p>
              </div>
            </div>
          </div>
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

    <el-drawer v-model="showHistory" title="请求历史" size="380px">
      <div v-for="(h, i) in history" :key="i" class="history-item" @click="loadHistory(h)">
        <span class="history-method" :class="h.method">{{ h.method }}</span>
        <span class="history-url">{{ h.url }}</span>
        <div class="history-meta">
          <span :style="{ color: h.status < 400 ? '#22c55e' : '#ef4444' }">● {{ h.status }}</span>
          <span>{{ h.time_ms }}ms</span>
        </div>
      </div>
    </el-drawer>

    <el-dialog v-model="showEnvModal" title="环境变量管理" width="620px">
      <div style="display: flex; gap: 16px; min-height: 320px;">
        <div style="width: 140px; border-right: 1px solid #1e293b; padding-right: 12px; flex-shrink: 0;">
          <div v-for="env in environments" :key="env.id"
            class="env-item" :class="{ active: env.id === activeEnvId }" @click="editEnv = { ...env, variables: { ...(env.variables || {}) } }">
            {{ env.name }}
          </div>
          <el-button size="small" style="margin-top: 8px; width: 100%;" @click="addEnvironment">+ 新建</el-button>
        </div>
        <div style="flex: 1; display: flex; flex-direction: column;" v-if="editEnv">
          <el-form label-position="top" size="small" style="flex: 1;">
            <el-form-item label="环境名称">
              <el-input v-model="editEnv.name" />
            </el-form-item>
            <el-form-item label="变量">
              <div style="width: 100%;">
                <div v-for="(val, key) in editEnv.variables" :key="key" class="kv-row" style="margin-bottom: 6px;">
                  <el-input :model-value="key" size="small" style="flex: 0.5;" disabled />
                  <el-input :model-value="val" size="small" style="flex: 0.5;" disabled />
                  <el-button text size="small" type="warning" @click="startEditVar(key, val)">✏</el-button>
                  <el-button text size="small" type="danger" @click="deleteVar(key)">🗑</el-button>
                </div>
                <div class="kv-row" style="margin-top: 8px; padding-top: 8px; border-top: 1px solid #1e293b;">
                  <el-input v-model="newVarKey" size="small" placeholder="变量名" style="flex: 0.5;" />
                  <el-input v-model="newVarValue" size="small" placeholder="值" style="flex: 0.5;" />
                  <el-button size="small" type="warning" @click="addVar">+ 添加</el-button>
                </div>
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
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  proxyRequest, getCollections, saveCollection, deleteCollection,
  getEnvironments, saveEnvironment, activateEnvironment,
  type CollectionItem, type ProxyEnvironment, type ProxyResponse
} from '../api'

const searchText = ref('')
const activeTab = ref('headers')
const bodyType = ref('json')
const sending = ref(false)
const showHistory = ref(false)
const showEnvModal = ref(false)

const request = reactive({ method: 'GET', url: '', body: '', pre_script: '' })
const headerList = ref<{ key: string; value: string; enabled: boolean }[]>([])
const response = ref<ProxyResponse | null>(null)
const activeRequestId = ref('')

const collections = ref<(CollectionItem & { _open?: boolean })[]>([])
const environments = ref<ProxyEnvironment[]>([])
const activeEnvId = ref('')
const editEnv = ref<ProxyEnvironment | null>(null)
const history = ref<any[]>([])
const newVarKey = ref('')
const newVarValue = ref('')
const editingVarKey = ref<string | null>(null)

const filteredCollections = computed(() => {
  if (!searchText.value) return collections.value
  const q = searchText.value.toLowerCase()
  return collections.value.filter(f => f.name.toLowerCase().includes(q) || (f.children || []).some(r => r.name.toLowerCase().includes(q)))
})

const activeEnvName = computed(() => environments.value.find(e => e.id === activeEnvId.value)?.name || '无')
const activeEnvVars = computed(() => environments.value.find(e => e.id === activeEnvId.value)?.variables || {})

function genId() { return Date.now().toString(36) + Math.random().toString(36).slice(2, 8) }

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
        const now = new Date()
        const fmt = (f: string) => {
          const pad = (n: number) => String(n).padStart(2, '0')
          return f
            .replace('yyyy', String(now.getFullYear()))
            .replace('MM', pad(now.getMonth() + 1))
            .replace('dd', pad(now.getDate()))
            .replace('HH', pad(now.getHours()))
            .replace('mm', pad(now.getMinutes()))
            .replace('ss', pad(now.getSeconds()))
        }
        const helpers = {
          $now: () => now.toISOString(),
          $formatTime: fmt,
          $timestamp: () => String(Math.floor(now.getTime() / 1000)),
          $uuid: () => Date.now().toString(36) + Math.random().toString(36).slice(2, 10),
        }
        const scriptFn = new Function('vars', '$now', '$formatTime', '$timestamp', '$uuid', request.pre_script)
        scriptFn(vars, helpers.$now, helpers.$formatTime, helpers.$timestamp, helpers.$uuid)
      } catch (e: any) {
        ElMessage.error('Pre-Script 执行出错: ' + e.message)
        sending.value = false
        return
      }
    }

    const resolve = (s: string) => s.replace(/\{\{(\w+)\}\}/g, (_, k) => vars[k] !== undefined ? vars[k] : `{{${k}}}`)
    const res = await proxyRequest({
      method: request.method, url: resolve(request.url), headers: Object.fromEntries(Object.entries(headers).map(([k, v]) => [k, resolve(v)])),
      body: bodyType.value === 'none' ? '' : resolve(request.body), timeout: 30,
    })
    response.value = res
    const entry = { method: request.method, url: request.url, status: res.status, time_ms: res.time_ms, timestamp: new Date().toLocaleTimeString() }
    history.value.unshift(entry)
    if (history.value.length > 50) history.value.pop()
    localStorage.setItem('proxy_history', JSON.stringify(history.value))
  } catch (e: any) {
    response.value = { status: 0, status_text: 'Error', headers: {}, body: '', time_ms: 0, size: 0, error: e.message }
  } finally { sending.value = false }
}

function highlightJson(body: string) {
  try {
    const pretty = JSON.stringify(JSON.parse(body), null, 2)
    return pretty.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
      .replace(/("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+-]?\d+)?)/g, (m: string) => {
        if (/^"/.test(m)) return /:$/.test(m) ? `<span class="json-key">${m}</span>` : `<span class="json-string">${m}</span>`
        if (/true|false/.test(m)) return `<span class="json-bool">${m}</span>`
        if (/null/.test(m)) return `<span class="json-null">${m}</span>`
        return `<span class="json-number">${m}</span>`
      })
  } catch { return body }
}

function addFolder() {
  ElMessageBox.prompt('文件夹名称', '新建文件夹').then(({ value }) => {
    if (!value) return
    const folder = { id: genId(), name: value, type: 'folder' as const, children: [], _open: true }
    collections.value.push(folder)
    saveCollection(folder)
  })
}

function addRequest() {
  ElMessageBox.prompt('请求名称', '新建请求').then(({ value }) => {
    if (!value) return
    const req: CollectionItem = { id: genId(), name: value, type: 'request', method: 'GET', url: '', headers: {}, body: '', pre_script: '' }
    if (collections.value.length === 0) {
      const folder: any = { id: genId(), name: '默认文件夹', type: 'folder', children: [req], _open: true }
      collections.value.push(folder)
      saveCollection(folder)
    } else {
      const folder = collections.value[0]
      if (!folder.children) folder.children = []
      folder.children.push(req)
      saveCollection(folder)
    }
  })
}

function loadRequest(req: CollectionItem) {
  activeRequestId.value = req.id
  request.method = req.method || 'GET'
  request.url = req.url || ''
  request.body = req.body || ''
  request.pre_script = req.pre_script || ''
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
    collections.value = collections.value.filter(f => { if (f.children) f.children = f.children.filter(r => r.id !== id); return true })
  })
}

function renameFolder(folder: CollectionItem) {
  ElMessageBox.prompt('新名称', '重命名', { inputValue: folder.name }).then(({ value }) => {
    if (!value) return
    folder.name = value
    saveCollection(folder)
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
  const headers: Record<string, string> = {}
  headerList.value.filter(h => h.enabled && h.key).forEach(h => { headers[h.key] = h.value })
  for (const folder of collections.value) {
    const req = folder.children?.find(r => r.id === activeRequestId.value)
    if (req) { req.method = request.method; req.url = request.url; req.body = request.body; req.headers = headers; req.pre_script = request.pre_script; await saveCollection(folder); ElMessage.success('已保存'); return }
  }
}

function loadHistory(h: any) { request.method = h.method; request.url = h.url; showHistory.value = false }
function switchEnv(id: string) { activateEnvironment(id) }

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
  if (editEnv.value.variables[key] !== undefined) {
    ElMessage.warning('变量名已存在')
    return
  }
  editEnv.value.variables[key] = newVarValue.value
  newVarKey.value = ''
  newVarValue.value = ''
  ElMessage.success('已添加变量')
}

function deleteVar(key: string) {
  if (!editEnv.value) return
  ElMessageBox.confirm(`确定删除变量 "${key}"？`, '提示', { type: 'warning' }).then(() => {
    delete editEnv.value!.variables[key]
    ElMessage.success('已删除变量')
  }).catch(() => {})
}

function startEditVar(key: string, val: string) {
  editingVarKey.value = key
  newVarKey.value = key
  newVarValue.value = val
  if (editEnv.value) {
    delete editEnv.value.variables[key]
  }
}

async function saveEnvs() {
  for (const env of environments.value) { await saveEnvironment(env) }
  showEnvModal.value = false
  ElMessage.success('环境变量已保存')
}

onMounted(loadData)
</script>

<style scoped>
.proxy-page { height: 100%; display: flex; flex-direction: column; }
.proxy-layout { flex: 1; display: flex; overflow: hidden; }
.collection-panel { width: 260px; background: #111827; border-right: 1px solid #1e293b; display: flex; flex-direction: column; }
.collection-header { padding: 12px; border-bottom: 1px solid #1e293b; display: flex; flex-direction: column; gap: 8px; }
.collection-actions { display: flex; gap: 6px; }
.collection-tree { flex: 1; overflow-y: auto; padding: 8px 0; }
.tree-folder-header { display: flex; align-items: center; gap: 6px; padding: 6px 12px; cursor: pointer; font-size: 13px; color: #94a3b8; }
.tree-folder-header:hover { background: #1a1f2e; color: #e2e8f0; }
.arrow { font-size: 10px; transition: transform 0.2s; width: 12px; text-align: center; }
.arrow.open { transform: rotate(90deg); }
.tree-children { padding-left: 16px; }
.tree-children.collapsed { display: none; }
.tree-request { display: flex; align-items: center; gap: 6px; padding: 5px 12px 5px 28px; cursor: pointer; font-size: 12px; color: #64748b; border-left: 2px solid transparent; }
.tree-request:hover { background: #1a1f2e; color: #e2e8f0; }
.tree-request.active { color: #f59e0b; background: rgba(245,158,11,0.08); border-left-color: #f59e0b; }
.req-method { font-family: 'JetBrains Mono', monospace; font-size: 10px; font-weight: 600; padding: 1px 4px; border-radius: 3px; }
.req-method.GET { background: rgba(34,197,94,0.15); color: #22c55e; }
.req-method.POST { background: rgba(245,158,11,0.15); color: #f59e0b; }
.req-method.PUT { background: rgba(59,130,246,0.15); color: #3b82f6; }
.req-method.DELETE { background: rgba(239,68,68,0.15); color: #ef4444; }
.req-actions { display: none; gap: 2px; }
.tree-request:hover .req-actions { display: flex; }
.editor-panel { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.request-bar { display: flex; gap: 8px; padding: 12px 16px; background: #111827; border-bottom: 1px solid #1e293b; }
.method-select :deep(.el-input__wrapper) { background: #0d1117 !important; }
.tabs-area { display: flex; border-bottom: 1px solid #1e293b; background: #111827; padding: 0 16px; }
.tab { padding: 9px 16px; font-size: 13px; font-weight: 500; color: #64748b; cursor: pointer; border-bottom: 2px solid transparent; }
.tab:hover { color: #94a3b8; }
.tab.active { color: #f59e0b; border-bottom-color: #f59e0b; }
.content-split { display: flex; flex: 1; overflow: hidden; }
.request-panel { flex: 1; display: flex; flex-direction: column; border-right: 1px solid #1e293b; overflow: hidden; }
.response-panel { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.panel-header { padding: 8px 14px; font-size: 11px; font-weight: 600; color: #64748b; text-transform: uppercase; letter-spacing: 0.5px; background: #111827; border-bottom: 1px solid #1e293b; display: flex; align-items: center; justify-content: space-between; }
.panel-body { flex: 1; overflow-y: auto; padding: 10px 14px; }
.kv-row { display: flex; gap: 6px; margin-bottom: 5px; align-items: center; }
.response-status { display: flex; gap: 12px; padding: 8px 14px; background: #111827; border-bottom: 1px solid #1e293b; }
.response-body { flex: 1; overflow-y: auto; padding: 14px; }
.json-view { background: #0d1117; border: 1px solid #1e293b; border-radius: 8px; padding: 14px; font-family: 'JetBrains Mono', monospace; font-size: 12px; line-height: 1.7; white-space: pre-wrap; word-break: break-all; color: #e2e8f0; }
.json-key { color: #93c5fd; }
.json-string { color: #86efac; }
.json-number { color: #fbbf24; }
.json-bool { color: #c084fc; }
.json-null { color: #64748b; }
.env-bar { display: flex; align-items: center; gap: 8px; padding: 6px 16px; background: #1a1f2e; border-top: 1px solid #1e293b; }
.env-item { padding: 8px 10px; border-radius: 6px; cursor: pointer; font-size: 12px; color: #94a3b8; margin-bottom: 2px; }
.env-item:hover { background: #1a1f2e; color: #e2e8f0; }
.env-item.active { background: rgba(245,158,11,0.1); color: #f59e0b; }
.history-item { padding: 10px 12px; border-radius: 8px; cursor: pointer; margin-bottom: 4px; }
.history-item:hover { background: #1a1f2e; }
.history-method { font-family: 'JetBrains Mono', monospace; font-size: 10px; font-weight: 600; padding: 2px 6px; border-radius: 3px; margin-right: 8px; }
.history-method.GET { background: rgba(34,197,94,0.15); color: #22c55e; }
.history-method.POST { background: rgba(245,158,11,0.15); color: #f59e0b; }
.history-method.PUT { background: rgba(59,130,246,0.15); color: #3b82f6; }
.history-method.DELETE { background: rgba(239,68,68,0.15); color: #ef4444; }
.history-url { font-family: 'JetBrains Mono', monospace; font-size: 11px; color: #94a3b8; word-break: break-all; }
.history-meta { margin-top: 4px; font-size: 10px; color: #64748b; display: flex; gap: 10px; }
.empty-state { display: flex; flex-direction: column; align-items: center; justify-content: center; height: 100%; color: #64748b; gap: 10px; }
</style>
