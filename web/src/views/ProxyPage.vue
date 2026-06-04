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
          <el-dropdown trigger="click">
            <el-button size="small">📤 导出 <el-icon><ArrowDown /></el-icon></el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="exportConfig">导出 GridSim 格式</el-dropdown-item>
                <el-dropdown-item @click="exportPostman">导出 Postman 格式</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <el-dropdown trigger="click">
            <el-button size="small">📥 导入 <el-icon><ArrowDown /></el-icon></el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="triggerImport('gridsim')">导入 GridSim 格式</el-dropdown-item>
                <el-dropdown-item @click="triggerImport('postman')">导入 Postman 格式</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <input ref="importFileInput" type="file" accept=".json" style="display:none" @change="handleImportFile" />
        </div>

        <div class="tabs-area">
          <div v-for="tab in ['headers','body','pre-script','post-script']" :key="tab"
            class="tab" :class="{ active: activeTab === tab }" @click="activeTab = tab">
            {{ tab === 'pre-script' ? 'Pre-Script' : tab === 'post-script' ? 'Post-Script' : tab.charAt(0).toUpperCase() + tab.slice(1) }}
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
                  @input="onPreScriptInput"
                  placeholder="示例：&#10;vars.timestamp = $now()&#10;vars.date = $formatTime('yyyy-MM-dd')&#10;vars.time = $formatTime('HH:mm:ss')&#10;vars.token = 'my-token-123'"
                  style="font-family: 'JetBrains Mono', monospace; font-size: 12px;" />
                <div style="margin-top: 8px; font-size: 11px; color: #64748b; line-height: 1.8;">
                  <div><span style="color: #f59e0b;">内置函数：</span></div>
                  <div><code v-text="'$now()'"></code> — 当前时间戳 (ISO 8601)</div>
                  <div><code v-text="'$formatTime(fmt)'"></code> — 格式化时间，如 <code v-text="'yyyy-MM-dd HH:mm:ss'"></code></div>
                  <div><code v-text="'$timestamp()'"></code> — Unix 秒级时间戳</div>
                  <div><code v-text="'$uuid()'"></code> — 随机 ID</div>
                </div>
                <div class="env-vars-box">
                  <div class="env-vars-header">
                    <span class="env-vars-title">当前环境: <span style="color: #f59e0b;">{{ activeEnvName }}</span></span>
                    <div style="display: flex; align-items: center; gap: 6px;">
                      <span v-if="Object.keys(activeEnvVars).length" class="env-vars-count">{{ Object.keys(activeEnvVars).length }} 个变量</span>
                      <span v-else class="env-vars-count" style="color: #64748b;">无变量</span>
                      <el-button text size="small" type="primary" title="复制当前环境为 dotenv 格式" @click.stop="copyEntireEnv(environments.find(e => e.id === activeEnvId))" style="padding: 0 4px; font-size: 12px; color: #f59e0b;">📋</el-button>
                    </div>
                  </div>
                  <div v-if="Object.keys(activeEnvVars).length" class="env-vars-grid">
                    <div v-for="(v, k) in activeEnvVars" :key="k" class="env-var-item" :class="{ 'var-invalid': !isValidVarName(k) }" :title="varTitle(k)">
                      <span class="env-var-key">{{ k }}</span>
                      <span class="env-var-eq">=</span>
                      <span class="env-var-val">{{ v }}</span>
                    </div>
                  </div>
                  <div v-if="hasInvalidVarName" class="var-name-warn" style="margin-top: 6px;">
                    ⚠ 部分变量名含非法字符（如 <code>-</code>），在 URL/Body 中引用将不会被替换。请在「环境管理」中修改。
                  </div>
                </div>
              </div>
              <div v-if="activeTab === 'post-script'">
                <div style="font-size: 12px; color: #94a3b8; margin-bottom: 8px;">
                  发送后执行脚本，用 <code style="background: #1a1f2e; padding: 1px 6px; border-radius: 3px; color: #f59e0b; font-family: 'JetBrains Mono', monospace; font-size: 11px;" v-text="'pm.response.json(\'data.id\')'"></code> 提取响应内容
                </div>
                <el-input v-model="request.test_script" type="textarea" :rows="10"
                  @input="onPreScriptInput"
                  placeholder="示例：&#10;var data = pm.response.json()&#10;if (data.token) {&#10;  pm.vars.set('token', data.token)&#10;  console.log('token 已保存')&#10;}&#10;pm.expect(pm.response.code).to.equal(200)"
                  style="font-family: 'JetBrains Mono', monospace; font-size: 12px;" />
                <div style="margin-top: 8px; font-size: 11px; color: #64748b; line-height: 1.8;">
                  <div><span style="color: #f59e0b;">pm API：</span></div>
                  <div><code v-text="'pm.response.json(path?)'"></code> — 解析 JSON 响应，可选路径如 <code v-text="'data.user.id'"></code></div>
                  <div><code v-text="'pm.response.text()'"></code> — 原始响应文本</div>
                  <div><code v-text="'pm.response.code'"></code> — HTTP 状态码</div>
                  <div><code v-text="'pm.response.headers.get(k)'"></code> — 读取响应头</div>
                  <div><code v-text="'pm.vars.set(k, v) / get(k) / has(k)'"></code> — 写入/读取会话变量</div>
                  <div><code v-text="'pm.environment.set(k, v) / get(k)'"></code> — 写入/读取环境变量（自动持久化）</div>
                  <div><code v-text="'pm.expect(v).to.equal(x)'"></code> — 断言检查（失败仅提示，不中断）</div>
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
          <el-button text size="small" @click="copyEntireEnv(environments.find(e => e.id === activeEnvId))" title="复制当前环境" style="color: #94a3b8; padding: 0 4px; font-size: 12px;">📋</el-button>
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
          <span>🕐 {{ h.timestamp }}</span>
          <span style="margin-left: auto; color: #64748b;">{{ historyOpen[i] ? '▼ 收起' : '▶ 详情' }}</span>
        </div>
        <div v-if="historyOpen[i]" class="history-detail" @click.stop>
          <!-- 最终 URL（含变量替换） -->
          <div class="detail-section">
            <div class="detail-label">🔗 最终 URL <span style="color: #64748b; font-size: 10px;">(变量已替换)</span></div>
            <pre class="detail-pre">{{ h.resolved_url || h.url }}</pre>
            <div v-if="h.url && h.resolved_url && h.url !== h.resolved_url" class="detail-original">
              <div class="detail-label" style="font-size: 10px;">原始模板</div>
              <pre class="detail-pre" style="color: #64748b;">{{ h.url }}</pre>
            </div>
          </div>

          <!-- 请求 Headers -->
          <div v-if="h.headers && Object.keys(h.headers).length" class="detail-section">
            <div class="detail-label">📨 请求 Headers <span style="color: #64748b; font-size: 10px;">({{ Object.keys(h.headers).length }})</span></div>
            <div v-for="(v, k) in h.headers" :key="k" class="detail-kv">
              <span class="detail-k">{{ k }}:</span>
              <span class="detail-v">{{ v }}</span>
            </div>
          </div>

          <!-- 请求 Body -->
          <div v-if="h.body" class="detail-section">
            <div class="detail-label">📦 请求 Body</div>
            <pre class="detail-pre">{{ h.body }}</pre>
          </div>

          <!-- 响应 Headers -->
          <div v-if="h.response_headers && Object.keys(h.response_headers).length" class="detail-section">
            <div class="detail-label">📥 响应 Headers <span style="color: #64748b; font-size: 10px;">({{ Object.keys(h.response_headers).length }})</span></div>
            <div v-for="(v, k) in h.response_headers" :key="k" class="detail-kv">
              <span class="detail-k">{{ k }}:</span>
              <span class="detail-v">{{ v }}</span>
            </div>
          </div>

          <!-- 响应 Body -->
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
          <div v-for="env in environments" :key="env.id"
            class="env-item" :class="{ active: env.id === activeEnvId }"
            :style="{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '6px 8px' }">
            <span style="flex: 1; cursor: pointer; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;" :title="env.name"
              @click="editEnv = { ...env, variables: { ...(env.variables || {}) } }">
              {{ env.name }}
              <span v-if="env.id === activeEnvId" style="color: #f59e0b; font-size: 10px; margin-left: 4px;">●</span>
            </span>
            <el-button text size="small" type="primary" title="复制此环境 (KEY=VALUE)" style="padding: 0 4px; font-size: 12px;"
              @click.stop="copyEntireEnv(env)">📋</el-button>
            <el-button v-if="environments.length > 1" text size="small" type="danger" style="padding: 0 4px; font-size: 12px;"
              @click.stop="deleteEntireEnv(env.id)" title="删除整个环境">×</el-button>
          </div>
          <el-button size="small" style="margin-top: 8px; width: 100%;" @click="addEnvironment">+ 新建</el-button>
        </div>
        <div style="flex: 1; display: flex; flex-direction: column;" v-if="editEnv">
          <el-form label-position="top" size="small" style="flex: 1;">
                <div style="display: flex; align-items: center; gap: 8px;">
                  <el-form-item label="环境名称">
                    <el-input v-model="editEnv.name" />
                  </el-form-item>
                  <el-button size="small" type="primary" @click="copyEntireEnv(editEnv)">📋 复制此环境</el-button>
                </div>
                <el-form-item label="变量">
                  <div style="width: 100%;">
                    <div v-for="(val, key) in editEnv.variables" :key="key" class="kv-row" style="margin-bottom: 6px;">
                      <el-input :model-value="key" size="small" style="flex: 0.5;" :class="{ 'var-key-invalid': !isValidVarName(key) }" disabled />
                      <el-input :model-value="val" size="small" style="flex: 0.5;" disabled />
                      <el-button text size="small" type="primary" title="复制为 KEY=VALUE" @click="copyVar(key, val)">📋</el-button>
                      <el-button text size="small" type="warning" @click="startEditVar(key, val)">✏</el-button>
                      <el-button text size="small" type="danger" @click="deleteVar(key)">🗑</el-button>
                    </div>
                    <div class="kv-row" style="margin-top: 8px; padding-top: 8px; border-top: 1px solid #1e293b;">
                      <el-input v-model="newVarKey" size="small" placeholder="变量名 (字母/数字/下划线/点)" style="flex: 0.5;" :class="{ 'var-key-invalid': newVarKey && !isValidVarName(newVarKey) }" @input="newVarKeyTouched = true" />
                      <el-input v-model="newVarValue" size="small" placeholder="值" style="flex: 0.5;" />
                      <el-button size="small" type="warning" @click="addVar">+ 添加</el-button>
                    </div>
                    <div v-if="newVarKey && !isValidVarName(newVarKey)" class="var-name-warn">
                      ⚠ 变量名只能包含字母、数字、下划线和点（不支持 <code>-</code>、空格、中文等），否则在 URL/Body 中 <code>{{ buildVarRef(newVarKey) }}</code> 将无法被替换
                    </div>
                    <div class="var-name-hint">
                      💡 引用语法：<code v-text="buildVarRef('变量名')"></code>，如 <code v-text="buildVarRef('base_url')"></code>
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
import { ref, reactive, computed, onMounted, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowDown } from '@element-plus/icons-vue'
import {
  proxyRequest, getCollections, saveCollection, deleteCollection,
  getEnvironments, saveEnvironment, deleteEnvironment, activateEnvironment, exportProxyConfig,
  type CollectionItem, type ProxyEnvironment, type ProxyResponse
} from '../api'

const searchText = ref('')
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

const collections = ref<(CollectionItem & { _open?: boolean })[]>([])
const environments = ref<ProxyEnvironment[]>([])
const activeEnvId = ref('')
const editEnv = ref<ProxyEnvironment | null>(null)
const history = ref<any[]>([])
const historyOpen = ref<Record<number, boolean>>({})
const newVarKey = ref('')
const newVarValue = ref('')
const editingVarKey = ref<string | null>(null)
const newVarKeyTouched = ref(false)
let scriptSaveTimer: ReturnType<typeof setTimeout> | null = null

// 变量名合法字符：字母、数字、下划线、点（不支持 -、空格、中文等）
function isValidVarName(name: string): boolean {
  if (!name) return false
  return /^[A-Za-z_][A-Za-z0-9_.]*$/.test(name)
}

// 构造含花括号的提示文本（避免在模板里直接写 {{}} 触发 Vue 解析冲突）
function varTitle(k: string): string {
  const ref = '{' + '{' + k + '}' + '}'
  if (isValidVarName(k)) return ref
  return `变量名 "${k}" 含非法字符（如 - 空格），${ref} 将无法被替换`
}

// 在模板里构造 {{xxx}} 字符串（避免与 Vue 插值语法冲突）
function buildVarRef(name: string): string {
  return '{' + '{' + name + '}' + '}'
}

const filteredCollections = computed(() => {
  if (!searchText.value) return collections.value
  const q = searchText.value.toLowerCase()
  return collections.value.filter(f => f.name.toLowerCase().includes(q) || (f.children || []).some(r => r.name.toLowerCase().includes(q)))
})

const activeEnvName = computed(() => environments.value.find(e => e.id === activeEnvId.value)?.name || '无')
const activeEnvVars = computed(() => environments.value.find(e => e.id === activeEnvId.value)?.variables || {})
const hasInvalidVarName = computed(() => Object.keys(activeEnvVars.value).some(k => !isValidVarName(k)))

function genId() { return Date.now().toString(36) + Math.random().toString(36).slice(2, 8) }

// Auto-save script content with debounce to prevent losing changes on refresh
function onPreScriptInput() {
  if (scriptSaveTimer) clearTimeout(scriptSaveTimer)
  scriptSaveTimer = setTimeout(() => {
    autoSaveRequest()
  }, 800)
}

// 构造沙箱时间辅助函数（pre/post script 复用）
function buildTimeHelpers() {
  const now = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  const fmt = (f: string) => f
    .replace('yyyy', String(now.getFullYear()))
    .replace('MM', pad(now.getMonth() + 1))
    .replace('dd', pad(now.getDate()))
    .replace('HH', pad(now.getHours()))
    .replace('mm', pad(now.getMinutes()))
    .replace('ss', pad(now.getSeconds()))
  return {
    $now: () => now.toISOString(),
    $formatTime: fmt,
    $timestamp: () => String(Math.floor(now.getTime() / 1000)),
    $uuid: () => Date.now().toString(36) + Math.random().toString(36).slice(2, 10),
  }
}

// 解析 JSON 路径（支持 a.b[0].c 语法）
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

// 构造 pm 沙箱：vars (本请求临时) + environment (本环境持久化)
function buildPmSandbox(vars: Record<string, string>, res: ProxyResponse | null) {
  const envGet = (k: string) => activeEnvVars.value[k] ?? ''
  const envSet = (k: string, v: any) => {
    const env = environments.value.find(e => e.id === activeEnvId.value)
    if (!env) return
    env.variables[k] = String(v)
    // 写回后端 + 触发本地响应式更新
    saveEnvironment(env).catch((e: any) => ElMessage.error('保存环境变量失败: ' + e.message))
  }
  const responseObj: any = {
    code: res?.status ?? 0,
    status: res?.status_text ?? '',
    headers: {
      get: (k: string) => {
        if (!res?.headers) return undefined
        const lower = k.toLowerCase()
        for (const [hk, hv] of Object.entries(res.headers)) {
          if (hk.toLowerCase() === lower) return hv as string
        }
        return undefined
      },
    },
    text: () => res?.body ?? '',
    json: (path?: string) => {
      let parsed: any
      try { parsed = JSON.parse(res?.body || '{}') } catch { parsed = {} }
      if (!path) return parsed
      return resolveJsonPath(parsed, path)
    },
  }
  return {
    response: responseObj,
    vars: {
      get: (k: string) => vars[k] ?? '',
      set: (k: string, v: any) => { vars[k] = String(v) },
      has: (k: string) => k in vars,
      unset: (k: string) => { delete vars[k] },
      toObject: () => ({ ...vars }),
    },
    environment: {
      get: envGet,
      set: envSet,
      has: (k: string) => k in activeEnvVars.value,
      unset: (k: string) => {
        const env = environments.value.find(e => e.id === activeEnvId.value)
        if (!env) return
        delete env.variables[k]
        saveEnvironment(env).catch((e: any) => ElMessage.error('删除环境变量失败: ' + e.message))
      },
    },
    expect: (actual: any) => {
      // Postman 风格：pm.expect(actual).to.equal(expected) 失败时自动抛错
      const result = (pass: boolean, expected: any) => {
        const r = { pass, message: pass ? `断言通过` : `断言失败: 期望 ${JSON.stringify(expected)}，实际 ${JSON.stringify(actual)}` }
        if (!pass) {
          const e: any = new Error(r.message)
          e.pass = r.pass
          e.message = r.message
          throw e
        }
        return r
      }
      const chain: any = {
        to: {
          equal: (expected: any) => result(actual === expected, expected),
          eql: (expected: any) => result(JSON.stringify(actual) === JSON.stringify(expected), expected),
          be: (expected: any) => result(actual === expected, expected),
        },
        equal: (expected: any) => result(actual === expected, expected),
        eql: (expected: any) => result(JSON.stringify(actual) === JSON.stringify(expected), expected),
      }
      return chain
    },
  }
}

// 在 post-script 中抛出的断言包装为 ElMessage
function runPostScript(testScript: string, vars: Record<string, string>, res: ProxyResponse | null) {
  if (!testScript || !testScript.trim()) return
  const helpers = buildTimeHelpers()
  const pm = buildPmSandbox(vars, res)
  const scriptFn = new Function(
    'vars', 'pm', '$now', '$formatTime', '$timestamp', '$uuid',
    `'use strict';\n${testScript}`
  )
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
      } catch (e: any) {
        ElMessage.error('Pre-Script 执行出错: ' + e.message)
        sending.value = false
        return
      }
    }

    const resolve = (s: string) => s.replace(/\{\{(\w+)\}\}/g, (_, k) => vars[k] !== undefined ? vars[k] : `{{${k}}}`)
    const resolvedUrl = resolve(request.url)
    const resolvedBody = bodyType.value === 'none' ? '' : resolve(request.body)
    const resolvedHeaders: Record<string, string> = {}
    for (const [k, v] of Object.entries(headers)) {
      resolvedHeaders[k] = resolve(v)
    }

    const res = await proxyRequest({
      method: request.method, url: resolvedUrl, headers: resolvedHeaders,
      body: resolvedBody, timeout: 30,
    })
    response.value = res

    // 历史中保存最终发出的完整参数（变量已替换为实际值）
    const entry = {
      method: request.method,
      url: request.url,
      resolved_url: resolvedUrl,
      headers: resolvedHeaders,
      body: resolvedBody,
      status: res.status,
      status_text: res.status_text,
      time_ms: res.time_ms,
      size: res.size,
      response_body: res.body,
      response_headers: res.headers,
      timestamp: new Date().toLocaleTimeString(),
    }
    history.value.unshift(entry)
    if (history.value.length > 50) history.value.pop()
    localStorage.setItem('proxy_history', JSON.stringify(history.value))

    // 执行 Post-Script（在 response 拿到后），可用 pm.response / pm.vars / pm.environment
    if (request.test_script && request.test_script.trim()) {
      try {
        runPostScript(request.test_script, vars, res)
      } catch (e: any) {
        // 解析断言失败的返回对象 (pm.expect 返回 { pass, message })
        if (e && typeof e === 'object' && 'pass' in e && 'message' in e) {
          if (e.pass) {
            ElMessage.success('Post-Script 断言通过: ' + e.message)
          } else {
            ElMessage.warning('Post-Script 断言失败: ' + e.message)
          }
        } else {
          ElMessage.error('Post-Script 执行出错: ' + (e?.message || e))
        }
      }
    }

    // 自动保存请求（包括脚本）
    await autoSaveRequest()
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
    const req: CollectionItem = { id: genId(), name: value, type: 'request', method: 'GET', url: '', headers: {}, body: '', pre_script: '', test_script: '' }
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
  request.test_script = req.test_script || ''
  headerList.value = Object.entries(req.headers || {}).map(([k, v]) => ({ key: k, value: v, enabled: true }))
  debouncedAutoSave()
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
    if (req) { req.method = request.method; req.url = request.url; req.body = request.body; req.headers = headers; req.pre_script = request.pre_script; req.test_script = request.test_script; await saveCollection(folder); return }
  }
}

// Auto-save on any request field change
let autoSaveTimer: ReturnType<typeof setTimeout> | null = null
function debouncedAutoSave() {
  if (autoSaveTimer) clearTimeout(autoSaveTimer)
  autoSaveTimer = setTimeout(() => { autoSaveRequest() }, 500)
}

// ─── Import / Export ───────────────────────────────────────────────────────

function triggerImport(format: 'gridsim' | 'postman') {
  importFormat.value = format
  importFileInput.value?.click()
}

function handleImportFile(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = async (ev) => {
    try {
      const text = ev.target?.result as string
      if (importFormat.value === 'postman') {
        await importPostman(text)
      } else {
        await importGridSim(text)
      }
    } catch (e: any) {
      ElMessage.error('导入失败: ' + e.message)
    }
  }
  reader.readAsText(file)
  // Reset input so same file can be re-imported
  ;(e.target as HTMLInputElement).value = ''
}

async function importGridSim(text: string) {
  const data = JSON.parse(text)
  if (data.collections && Array.isArray(data.collections)) {
    for (const item of data.collections) {
      await saveCollection(item)
      if (!collections.value.find(c => c.id === item.id)) {
        collections.value.push(item)
      }
    }
  }
  if (data.environments && Array.isArray(data.environments)) {
    for (const env of data.environments) {
      await saveEnvironment(env)
      if (!environments.value.find(e => e.id === env.id)) {
        environments.value.push(env)
      }
    }
  }
  if (data.active_env_id) {
    activeEnvId.value = data.active_env_id
    await activateEnvironment(data.active_env_id)
  }
  ElMessage.success('导入成功')
}

function isPostmanCollection(data: any): boolean {
  return data?.info?.schema && data.info.schema.includes('postman') && Array.isArray(data.item)
}

function parsePostmanUrl(rawUrl: string): { host: string; path: string[]; query: { key: string; value: string }[] } {
  // Parse "{{base_url}}/enos-edge/v1/getStrategyTypeList?orgId={{orgId}}"
  const qIdx = rawUrl.indexOf('?')
  let raw = qIdx >= 0 ? rawUrl.slice(0, qIdx) : rawUrl
  const queryStr = qIdx >= 0 ? rawUrl.slice(qIdx + 1) : ''
  
  const query: { key: string; value: string }[] = []
  if (queryStr) {
    for (const part of queryStr.split('&')) {
      const [k, ...vParts] = part.split('=')
      if (k) query.push({ key: k, value: vParts.join('=') })
    }
  }

  const parts = raw.split('/').filter(Boolean)
  return { host: parts[0] || '', path: parts.slice(1), query }
}

function postmanUrlToRaw(p: { host: string; path: string[]; query: { key: string; value: string }[] }): string {
  let raw = (p.host ? p.host + '/' : '') + p.path.join('/')
  if (p.query.length) {
    raw += '?' + p.query.map(q => `${q.key}=${q.value}`).join('&')
  }
  return raw
}

function postmanToGridsimCollection(item: any): CollectionItem | null {
  if (!item || !item.request) return null
  
  const req = item.request
  const method = (req.method || 'GET').toUpperCase()
  let url = ''
  let headers: Record<string, string> = {}
  let body = ''
  let bodyType = 'json' as string

  if (typeof req.url === 'string') {
    url = req.url
  } else if (req.url) {
    url = postmanUrlToRaw(req.url as any)
  }

  if (req.header) {
    headers = Object.fromEntries(
      (req.header as any[]).filter((h: any) => h.key).map((h: any) => [h.key, h.value || ''])
    )
  }

  if (req.body) {
    if (req.body.mode === 'raw') {
      body = req.body.raw || ''
      if (req.body.options?.raw?.language === 'json') bodyType = 'json'
      else bodyType = 'text'
    } else if (req.body.mode === 'urlencoded') {
      body = (req.body.urlencoded as any[] || [])
        .map((p: any) => `${p.key}=${p.value}`).join('&')
      bodyType = 'text'
      if (!headers['Content-Type']) headers['Content-Type'] = 'application/x-www-form-urlencoded'
    } else if (req.body.mode === 'formdata') {
      body = (req.body.formdata as any[] || [])
        .map((p: any) => `${p.key}=${p.value || ''}`).join('\n')
      bodyType = 'text'
    }
  }

  // Collect prerequest/test scripts separately
  let preScript = ''
  let testScript = ''
  if (item.event) {
    for (const evt of item.event) {
      if (evt.listen === 'prerequest' && evt.script?.exec) {
        preScript += evt.script.exec.join('\n') + '\n'
      }
      if (evt.listen === 'test' && evt.script?.exec) {
        testScript += evt.script.exec.join('\n') + '\n'
      }
    }
    preScript = preScript.trim()
    testScript = testScript.trim()
  }

  // Recursively handle nested items (folders)
  const children: CollectionItem[] = []
  if (Array.isArray(item.item)) {
    for (const child of item.item) {
      const col = postmanToGridsimCollection(child)
      if (col) children.push(col)
    }
  }

  if (children.length > 0) {
    return { id: genId(), name: item.name || 'Folder', type: 'folder', children, _open: true } as any
  }

  if (!url && !children.length) return null

  return {
    id: genId(), name: item.name || 'Untitled', type: 'request' as const,
    method, url, headers, body,
    pre_script: preScript || undefined,
    test_script: testScript || undefined,
  }
}

async function importPostman(text: string) {
  let data: any
  try { data = JSON.parse(text) } catch { throw new Error('无效的 JSON 文件') }
  
  if (!isPostmanCollection(data)) throw new Error('不是有效的 Postman Collection 格式')

  const newCollections: CollectionItem[] = []
  if (Array.isArray(data.item)) {
    for (const item of data.item) {
      const col = postmanToGridsimCollection(item)
      if (col) newCollections.push(col)
    }
  }

  if (newCollections.length === 0) throw new Error('未找到可导入的请求')

  // Merge into existing collections
  collections.value.push(...newCollections)
  
  // Save each new collection to backend
  for (const col of newCollections) {
    await saveCollection(col)
  }

  ElMessage.success(`成功导入 ${newCollections.length} 个请求/文件夹`)
}

async function buildPostmanExport(): Promise<any> {
  const envMap: Record<string, string> = {}
  for (const env of environments.value) {
    if (env.variables) {
      Object.assign(envMap, env.variables)
    }
  }

  const vars: Record<string, string> = {}
  for (const [k, v] of Object.entries(envMap)) {
    vars[k] = `{{${k}}}`
  }

  const postmanItems: any[] = []
  let requestCount = 0

  function convertItem(item: CollectionItem): any {
    if (item.type === 'folder') {
      const children: any[] = []
      if (item.children) {
        for (const child of item.children) {
          children.push(convertItem(child))
        }
      }
      return { name: item.name, item: children }
    }

    // It's a request
    requestCount++
    const headers: any[] = []
    if (item.headers) {
      for (const [k, v] of Object.entries(item.headers)) {
        if (k) headers.push({ key: k, value: v, type: 'text' })
      }
    }

    const postmanReq: any = {
      method: item.method || 'GET',
      header: headers,
    }

    // Convert URL to Postman format
    const parsed = parsePostmanUrl(item.url || '')
    postmanReq.url = {
      raw: item.url || '',
      host: (parsed.host ? [parsed.host] : []),
      path: parsed.path,
      query: parsed.query.map(q => ({ key: q.key, value: q.value, enabled: true })),
    }

    // Convert body
    if (item.body) {
      postmanReq.body = {
        mode: 'raw',
        raw: item.body,
        options: { raw: { language: 'json' } },
      }
      // Try to detect if it's not JSON
      try { JSON.parse(item.body) } catch { postmanReq.body.options.raw.language = 'text' }
    }

    // Convert pre_script/test_script to Postman events
    const events: any[] = []
    if (item.pre_script) {
      events.push({
        listen: 'prerequest',
        script: { type: 'text/javascript', exec: item.pre_script.split('\n').filter(l => l.trim()) },
      })
    }
    if (item.test_script) {
      events.push({
        listen: 'test',
        script: { type: 'text/javascript', exec: item.test_script.split('\n').filter(l => l.trim()) },
      })
    }
    if (events.length) postmanReq.event = events

    return { name: item.name || 'Untitled', request: postmanReq, response: [] }
  }

  for (const col of collections.value) {
    postmanItems.push(convertItem(col))
  }

  return {
    info: {
      _postman_id: genId() + Math.random().toString(36).slice(2, 14),
      name: 'GridSim Export',
      schema: 'https://schema.getpostman.com/json/collection/v2.1.0/collection.json',
    },
    item: postmanItems,
  }
}

async function exportConfig() {
  try {
    const blob = await exportProxyConfig()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `gridsim-proxy-config-${new Date().toISOString().slice(0,10)}.json`
    a.click()
    URL.revokeObjectURL(url)
    ElMessage.success('导出成功')
  } catch (e: any) {
    ElMessage.error('导出失败: ' + e.message)
  }
}

async function exportPostman() {
  try {
    const postmanData = await buildPostmanExport()
    const blob = new Blob([JSON.stringify(postmanData, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `gridsim-postman-export-${new Date().toISOString().slice(0,10)}.json`
    a.click()
    URL.revokeObjectURL(url)
    ElMessage.success('Postman 格式导出成功')
  } catch (e: any) {
    ElMessage.error('导出失败: ' + e.message)
  }
}

function loadHistory(h: any) { request.method = h.method; request.url = h.url; showHistory.value = false }
function toggleHistoryDetail(i: number) { historyOpen.value[i] = !historyOpen.value[i] }
function switchEnv(id: string) {
  activeEnvId.value = id
  activateEnvironment(id)
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
  if (!isValidVarName(key)) {
    ElMessage.error(`变量名 "${key}" 包含非法字符，只能使用字母、数字、下划线和点（不能以数字开头，不能含 - 空格 中文等）。` + `在 URL/Body 中 {{${key}}} 将无法被替换。`)
    return
  }
  if (editEnv.value.variables[key] !== undefined) {
    ElMessage.warning('变量名已存在')
    return
  }
  editEnv.value.variables[key] = newVarValue.value
  newVarKey.value = ''
  newVarValue.value = ''
  newVarKeyTouched.value = false
  ElMessage.success('已添加变量')
}

function deleteVar(key: string) {
  if (!editEnv.value) return
  ElMessageBox.confirm(`确定删除变量 "${key}"？`, '提示', { type: 'warning' }).then(() => {
    delete editEnv.value!.variables[key]
    ElMessage.success('已删除变量')
  }).catch(() => {})
}

// 复制单个环境变量为 KEY=VALUE 格式（可粘贴到 .env / shell / 新输入框中编辑）
async function copyVar(key: string, val: string) {
  // 转义值为单引号包裹的 shell 形式（含特殊字符时）
  const needsQuote = /[\s'"\\$`]/.test(val) || val === ''
  const text = needsQuote ? `${key}='${val.replace(/'/g, "'\\''")}'` : `${key}=${val}`
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success(`已复制: ${text.length > 40 ? text.slice(0, 40) + '…' : text}`)
  } catch (e: any) {
    // 后备方案：使用 textarea + execCommand
    const ta = document.createElement('textarea')
    ta.value = text
    ta.style.position = 'fixed'
    ta.style.left = '-9999px'
    document.body.appendChild(ta)
    ta.select()
    try {
      document.execCommand('copy')
      ElMessage.success(`已复制: ${text.length > 40 ? text.slice(0, 40) + '…' : text}`)
    } catch (err: any) {
      ElMessage.error('复制失败: ' + (err?.message || err))
    } finally {
      document.body.removeChild(ta)
    }
  }
}

// 删除整个环境
async function deleteEntireEnv(id: string) {
  const env = environments.value.find(e => e.id === id)
  if (!env) return
  const varCount = Object.keys(env.variables || {}).length
  const wasActive = activeEnvId.value === id
  try {
    await ElMessageBox.confirm(
      `确定删除环境 "${env.name}"？${varCount ? `该环境含 ${varCount} 个变量。` : ''}此操作不可撤销。`,
      '删除环境',
      { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' }
    )
  } catch { return }
  try {
    await deleteEnvironment(id)
    environments.value = environments.value.filter(e => e.id !== id)
    if (editEnv.value?.id === id) editEnv.value = null
    if (wasActive && environments.value.length > 0) {
      activeEnvId.value = environments.value[0].id
      await activateEnvironment(environments.value[0].id).catch(() => {})
    }
    ElMessage.success(`已删除环境 "${env.name}"`)
  } catch (e: any) {
    ElMessage.error('删除环境失败: ' + (e?.message || e))
  }
}

// 复制整个环境为 dotenv 格式 (KEY='value' / KEY=value)
async function copyEntireEnv(env?: ProxyEnvironment | null) {
  if (!env) {
    ElMessage.warning('当前没有可选中的环境')
    return
  }
  const lines = Object.entries(env.variables || {})
    .map(([key, val]) => {
      const needsQuote = /[\s'"\\$`]/.test(val) || val === ''
      const quoted = needsQuote ? `'${val.replace(/'/g, "'\\''")}'` : val
      return `${key}=${quoted}`
    })
    .join('\n')
  if (!lines) {
    ElMessage.warning('该环境没有变量')
    return
  }
  const text = `# 环境: ${env.name}\n${lines}\n# 共 ${Object.keys(env.variables || {}).length} 个变量`
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success(`已复制环境 "${env.name}" (${Object.keys(env.variables || {}).length} 个变量)`)
  } catch (e: any) {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.style.position = 'fixed'
    ta.style.left = '-9999px'
    document.body.appendChild(ta)
    ta.select()
    try {
      document.execCommand('copy')
      ElMessage.success(`已复制环境 "${env.name}" (${Object.keys(env.variables || {}).length} 个变量)`)
    } catch (err: any) {
      ElMessage.error('复制失败: ' + (err?.message || err))
    } finally {
      document.body.removeChild(ta)
    }
  }
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
  if (editEnv.value) {
    const idx = environments.value.findIndex(e => e.id === editEnv.value!.id)
    if (idx !== -1) {
      environments.value[idx] = { ...editEnv.value, variables: { ...editEnv.value.variables } }
    }
  }
  for (const env of environments.value) { await saveEnvironment(env) }
  showEnvModal.value = false
  ElMessage.success('环境变量已保存')
}

onMounted(loadData)

// Auto-save when any request field changes
watch(() => request.url, () => { if (activeRequestId.value) debouncedAutoSave() })
watch(() => request.body, () => { if (activeRequestId.value) debouncedAutoSave() })
watch(() => request.method, () => { if (activeRequestId.value) debouncedAutoSave() })
watch(() => headerList.value, () => { if (activeRequestId.value) debouncedAutoSave() }, { deep: true })
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
.method-select :deep(.el-input__wrapper) { background: #0d1117 !important; width: auto; min-width: 58px; padding: 0 4px; }
.method-select :deep(.el-input__inner) { text-align: center; font-weight: 600; font-size: 12px; width: 100%; }
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
.history-detail { margin-top: 8px; padding-top: 8px; border-top: 1px dashed #1e293b; }
.detail-section { margin-bottom: 10px; }
.detail-label { font-size: 11px; font-weight: 600; color: #94a3b8; margin-bottom: 4px; }
.detail-pre { font-family: 'JetBrains Mono', monospace; font-size: 11px; color: #e2e8f0; background: #0a0e17; border: 1px solid #1e293b; border-radius: 4px; padding: 6px 8px; margin: 0; white-space: pre-wrap; word-break: break-all; max-height: 200px; overflow: auto; }
.detail-original { margin-top: 4px; }
.detail-kv { display: flex; gap: 6px; padding: 3px 0; font-family: 'JetBrains Mono', monospace; font-size: 11px; }
.detail-k { color: #93c5fd; flex-shrink: 0; }
.detail-v { color: #e2e8f0; word-break: break-all; }
.history-method { font-family: 'JetBrains Mono', monospace; font-size: 10px; font-weight: 600; padding: 2px 6px; border-radius: 3px; margin-right: 8px; }
.history-method.GET { background: rgba(34,197,94,0.15); color: #22c55e; }
.history-method.POST { background: rgba(245,158,11,0.15); color: #f59e0b; }
.history-method.PUT { background: rgba(59,130,246,0.15); color: #3b82f6; }
.history-method.DELETE { background: rgba(239,68,68,0.15); color: #ef4444; }
.history-url { font-family: 'JetBrains Mono', monospace; font-size: 11px; color: #94a3b8; word-break: break-all; }
.history-meta { margin-top: 4px; font-size: 10px; color: #64748b; display: flex; gap: 10px; }
.history-body { margin-top: 6px; padding: 8px; background: #0d1117; border: 1px solid #1e293b; border-radius: 6px; max-height: 150px; overflow: auto; }
.history-body pre { font-family: 'JetBrains Mono', monospace; font-size: 11px; color: #94a3b8; white-space: pre-wrap; word-break: break-all; margin: 0; }
.env-vars-box { margin-top: 10px; border-top: 1px solid #1e293b; padding-top: 8px; }
.env-vars-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 6px; }
.env-vars-title { font-size: 11px; color: #64748b; }
.env-vars-count { font-size: 10px; color: #94a3b8; background: #1a1f2e; padding: 1px 6px; border-radius: 3px; }
.env-vars-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 4px 8px; }
.env-var-item { display: flex; align-items: center; gap: 4px; padding: 3px 6px; background: #0d1117; border: 1px solid #1e293b; border-radius: 4px; font-family: 'JetBrains Mono', monospace; font-size: 11px; min-width: 0; }
.env-var-item.var-invalid { border-color: #ef4444; background: rgba(239, 68, 68, 0.08); }
.env-var-item.var-invalid .env-var-key { color: #ef4444; text-decoration: line-through; }
.env-var-key { color: #93c5fd; flex-shrink: 0; max-width: 100px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.env-var-eq { color: #64748b; flex-shrink: 0; }
.env-var-val { color: #e2e8f0; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.var-key-invalid :deep(.el-input__wrapper) { background: rgba(239, 68, 68, 0.1) !important; border-color: #ef4444 !important; box-shadow: 0 0 0 1px #ef4444 inset !important; }
.var-key-invalid :deep(.el-input__inner) { color: #ef4444 !important; }
.var-name-warn { margin-top: 6px; padding: 6px 8px; background: rgba(239, 68, 68, 0.08); border: 1px solid rgba(239, 68, 68, 0.3); border-radius: 4px; color: #fca5a5; font-size: 11px; line-height: 1.5; }
.var-name-warn code { background: rgba(0,0,0,0.3); padding: 0 4px; border-radius: 2px; color: #fda4af; font-family: 'JetBrains Mono', monospace; }
.var-name-hint { margin-top: 6px; font-size: 11px; color: #64748b; }
.var-name-hint code { background: #1a1f2e; padding: 1px 5px; border-radius: 2px; color: #93c5fd; font-family: 'JetBrains Mono', monospace; }
.empty-state { display: flex; flex-direction: column; align-items: center; justify-content: center; height: 100%; color: #64748b; gap: 10px; }
</style>
