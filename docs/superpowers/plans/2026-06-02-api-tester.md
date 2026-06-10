# API 测试工具 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 IEC104 模拟器管理系统新增「接口测试」功能，支持 Collection 树形管理、环境变量系统、请求代理。

**Architecture:** 后端新增代理 API (`POST /api/v1/proxy`) + Collection/环境变量 CRUD API，数据存储在 JSON 文件。前端新增独立页面，左侧 Collection 树 + 中间请求编辑器 + 右侧响应区 + 底部环境选择栏。

**Tech Stack:** Go (net/http, encoding/json), Vue 3 + TypeScript + Element Plus, Axios

---

## 文件结构

| 文件 | 职责 |
|------|------|
| `pkg/api/proxy_handler.go` | 代理请求处理器 |
| `pkg/api/proxy_store.go` | Collection + 环境变量 JSON 存储 |
| `cmd/gridsim/main.go` | 注册新路由 |
| `web/src/views/ProxyPage.vue` | 接口测试主页面 |
| `web/src/api/index.ts` | 新增 API 方法 |
| `web/src/router/index.ts` | 新增路由 /proxy |
| `web/src/App.vue` | 侧边栏新增菜单项 |

---

### Task 1: 后端 — 代理请求处理器

**Files:**
- Create: `pkg/api/proxy_handler.go`

- [ ] **Step 1: 创建代理请求结构体和处理器**

```go
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type ProxyRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
	Timeout int               `json:"timeout"`
}

type ProxyResponse struct {
	Status     int               `json:"status"`
	StatusText string            `json:"status_text"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	TimeMs     int64             `json:"time_ms"`
	Size       int               `json:"size"`
	Error      string            `json:"error,omitempty"`
}

type ProxyHandler struct{}

func NewProxyHandler() *ProxyHandler {
	return &ProxyHandler{}
}

func (h *ProxyHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/proxy", h.handleProxy)
}

func (h *ProxyHandler) handleProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req ProxyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if req.Method == "" {
		req.Method = http.MethodGet
	}
	if req.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
		return
	}

	timeout := 30
	if req.Timeout > 0 && req.Timeout <= 120 {
		timeout = req.Timeout
	}

	result := h.executeRequest(req, time.Duration(timeout)*time.Second)
	writeJSON(w, http.StatusOK, result)
}

func (h *ProxyHandler) executeRequest(req ProxyRequest, timeout time.Duration) ProxyResponse {
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	var bodyReader io.Reader
	if req.Body != "" {
		bodyReader = strings.NewReader(req.Body)
	}

	httpReq, err := http.NewRequest(req.Method, req.URL, bodyReader)
	if err != nil {
		return ProxyResponse{Error: "invalid request: " + err.Error()}
	}

	for k, v := range req.Headers {
		if strings.EqualFold(k, "host") {
			continue
		}
		httpReq.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := client.Do(httpReq)
	elapsed := time.Since(start)

	if err != nil {
		return ProxyResponse{
			Error:  err.Error(),
			TimeMs: elapsed.Milliseconds(),
		}
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 10<<20))

	respHeaders := make(map[string]string)
	for k := range resp.Header {
		respHeaders[k] = resp.Header.Get(k)
	}

	statusText := http.StatusText(resp.Status)
	return ProxyResponse{
		Status:     resp.StatusCode,
		StatusText: statusText,
		Headers:    respHeaders,
		Body:       string(bodyBytes),
		TimeMs:     elapsed.Milliseconds(),
		Size:       len(bodyBytes),
	}
}

var varPattern = regexp.MustCompile(`\{\{(\w+)\}\}`)

func ReplaceVars(text string, vars map[string]string) string {
	if len(vars) == 0 {
		return text
	}
	return varPattern.ReplaceAllStringFunc(text, func(match string) string {
		key := match[2 : len(match)-2]
		if v, ok := vars[key]; ok {
			return v
		}
		return match
	})
}
```

- [ ] **Step 2: 编译验证**

Run: `cd /root/IEC-SIM/iec104-sim-master && go build ./pkg/api/`
Expected: 编译通过

- [ ] **Step 3: Commit**

```bash
git add pkg/api/proxy_handler.go
git commit -m "feat(api): add HTTP proxy handler"
```

---

### Task 2: 后端 — Collection + 环境变量存储

**Files:**
- Create: `pkg/api/proxy_store.go`

- [ ] **Step 1: 创建存储结构体**

```go
package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type CollectionItem struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Type     string            `json:"type"`
	Method   string            `json:"method,omitempty"`
	URL      string            `json:"url,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Body     string            `json:"body,omitempty"`
	Children []*CollectionItem `json:"children,omitempty"`
}

type Environment struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Variables map[string]string `json:"variables"`
}

type ProxyStore struct {
	mu           sync.RWMutex
	filePath     string
	Collections  []*CollectionItem `json:"collections"`
	Environments []*Environment    `json:"environments"`
	ActiveEnvID  string            `json:"active_env_id"`
}

func NewProxyStore(configDir string) *ProxyStore {
	return &ProxyStore{
		filePath:     filepath.Join(configDir, "proxy-store.json"),
		Collections:  []*CollectionItem{},
		Environments: []*Environment{},
	}
}

func (s *ProxyStore) Load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return s.Save()
		}
		return err
	}
	return json.Unmarshal(data, s)
}

func (s *ProxyStore) Save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

func (s *ProxyStore) GetCollections() []*CollectionItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Collections
}

func (s *ProxyStore) SaveCollection(item *CollectionItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, c := range s.Collections {
		if c.ID == item.ID {
			s.Collections[i] = item
			return s.Save()
		}
	}
	s.Collections = append(s.Collections, item)
	return s.Save()
}

func (s *ProxyStore) DeleteCollection(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Collections = deleteByID(s.Collections, id)
	return s.Save()
}

func deleteByID(items []*CollectionItem, id string) []*CollectionItem {
	result := make([]*CollectionItem, 0, len(items))
	for _, item := range items {
		if item.ID == id {
			continue
		}
		if item.Children != nil {
			item.Children = deleteByID(item.Children, id)
		}
		result = append(result, item)
	}
	return result
}

func (s *ProxyStore) GetEnvironments() []*Environment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Environments
}

func (s *ProxyStore) GetActiveEnv() *Environment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, env := range s.Environments {
		if env.ID == s.ActiveEnvID {
			return env
		}
	}
	return nil
}

func (s *ProxyStore) SaveEnvironment(env *Environment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.Environments {
		if e.ID == env.ID {
			s.Environments[i] = env
			return s.Save()
		}
	}
	s.Environments = append(s.Environments, env)
	return s.Save()
}

func (s *ProxyStore) DeleteEnvironment(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]*Environment, 0, len(s.Environments))
	for _, env := range s.Environments {
		if env.ID != id {
			result = append(result, env)
		}
	}
	s.Environments = result
	if s.ActiveEnvID == id {
		s.ActiveEnvID = ""
	}
	return s.Save()
}

func (s *ProxyStore) SetActiveEnv(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ActiveEnvID = id
	return s.Save()
}
```

- [ ] **Step 2: 编译验证**

Run: `cd /root/IEC-SIM/iec104-sim-master && go build ./pkg/api/`
Expected: 编译通过

- [ ] **Step 3: Commit**

```bash
git add pkg/api/proxy_store.go
git commit -m "feat(api): add proxy store for collections and environments"
```

---

### Task 3: 后端 — 注册路由

**Files:**
- Modify: `cmd/gridsim/main.go:187-210` (registerRoutes 方法)

- [ ] **Step 1: 在 registerRoutes 中添加代理路由**

在 `mux.HandleFunc("/api/v1/protocols", ws.handleProtocols)` 之后添加：

```go
proxyHandler := api.NewProxyHandler()
proxyHandler.Register(mux)

proxyStore := api.NewProxyStore(configDir)
if err := proxyStore.Load(); err != nil {
    slog.Warn("加载代理配置失败", "error", err)
}
ws.proxyStore = proxyStore
ws.proxyHandler = proxyHandler
```

在 `webServer` 结构体中添加字段：

```go
type webServer struct {
	mgr          *manager.Manager
	httpSrv      *http.Server
	cfgDir       string
	userConfig   *model.UserConfig
	proxyStore   *api.ProxyStore
	proxyHandler *api.ProxyHandler
}
```

添加 Collection/Environment CRUD handler：

```go
mux.HandleFunc("/api/v1/proxy/collections", ws.handleCollections)
mux.HandleFunc("/api/v1/proxy/collections/", ws.handleCollectionByID)
mux.HandleFunc("/api/v1/proxy/environments", ws.handleEnvironments)
mux.HandleFunc("/api/v1/proxy/environments/", ws.handleEnvironmentByID)
```

添加对应 handler 方法：

```go
func (ws *webServer) handleCollections(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"collections": ws.proxyStore.GetCollections()})
	case http.MethodPost:
		var item api.CollectionItem
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if err := ws.proxyStore.SaveCollection(&item); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, item)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (ws *webServer) handleCollectionByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/proxy/collections/")
	switch r.Method {
	case http.MethodDelete:
		if err := ws.proxyStore.DeleteCollection(id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (ws *webServer) handleEnvironments(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"environments": ws.proxyStore.GetEnvironments(),
			"active_id":    ws.proxyStore.ActiveEnvID,
		})
	case http.MethodPost:
		var env api.Environment
		if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if err := ws.proxyStore.SaveEnvironment(&env); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, env)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (ws *webServer) handleEnvironmentByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/proxy/environments/")
	parts := strings.SplitN(path, "/", 2)
	id := parts[0]

	if len(parts) == 2 && parts[1] == "activate" {
		if r.Method == http.MethodPost {
			if err := ws.proxyStore.SetActiveEnv(id); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}
	}

	switch r.Method {
	case http.MethodDelete:
		if err := ws.proxyStore.DeleteEnvironment(id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
```

- [ ] **Step 2: 编译验证**

Run: `cd /root/IEC-SIM/iec104-sim-master && go build ./cmd/gridsim/`
Expected: 编译通过

- [ ] **Step 3: Commit**

```bash
git add cmd/gridsim/main.go
git commit -m "feat(api): register proxy routes in server mode"
```

---

### Task 4: 前端 — API 方法

**Files:**
- Modify: `web/src/api/index.ts`

- [ ] **Step 1: 在文件末尾添加代理 API 方法**

```typescript
// ─── Proxy API Tester ──────────────────────────────────────────────────────

export interface ProxyRequest {
  method: string
  url: string
  headers: Record<string, string>
  body: string
  timeout?: number
}

export interface ProxyResponse {
  status: number
  status_text: string
  headers: Record<string, string>
  body: string
  time_ms: number
  size: number
  error?: string
}

export interface CollectionItem {
  id: string
  name: string
  type: 'folder' | 'request'
  method?: string
  url?: string
  headers?: Record<string, string>
  body?: string
  children?: CollectionItem[]
}

export interface ProxyEnvironment {
  id: string
  name: string
  variables: Record<string, string>
}

export async function proxyRequest(req: ProxyRequest): Promise<ProxyResponse> {
  const res = await http.post('/proxy', req, { timeout: 120000 })
  return res.data
}

export async function getCollections(): Promise<CollectionItem[]> {
  const res = await http.get('/proxy/collections')
  return res.data.collections
}

export async function saveCollection(item: CollectionItem): Promise<CollectionItem> {
  const res = await http.post('/proxy/collections', item)
  return res.data
}

export async function deleteCollection(id: string): Promise<void> {
  await http.delete(`/proxy/collections/${id}`)
}

export async function getEnvironments(): Promise<{ environments: ProxyEnvironment[], active_id: string }> {
  const res = await http.get('/proxy/environments')
  return res.data
}

export async function saveEnvironment(env: ProxyEnvironment): Promise<ProxyEnvironment> {
  const res = await http.post('/proxy/environments', env)
  return res.data
}

export async function deleteEnvironment(id: string): Promise<void> {
  await http.delete(`/proxy/environments/${id}`)
}

export async function activateEnvironment(id: string): Promise<void> {
  await http.post(`/proxy/environments/${id}/activate`)
}
```

- [ ] **Step 2: 编译验证**

Run: `cd /root/IEC-SIM/iec104-sim-master/web && npx tsc --noEmit`
Expected: 无错误

- [ ] **Step 3: Commit**

```bash
git add web/src/api/index.ts
git commit -m "feat(frontend): add proxy API methods"
```

---

### Task 5: 前端 — 路由和菜单

**Files:**
- Modify: `web/src/router/index.ts`
- Modify: `web/src/App.vue`

- [ ] **Step 1: 在 router/index.ts 添加路由**

在 `{ path: '/trend', ... }` 之后添加：

```typescript
{ path: '/proxy', name: 'proxy', component: () => import('@/views/ProxyPage.vue'), meta: { title: '接口测试' } },
```

- [ ] **Step 2: 在 App.vue 侧边栏添加菜单项**

在 `<el-menu-item index="/trend">` 之后添加：

```html
<el-menu-item index="/proxy">
  <el-icon><Connection /></el-icon>
  <span>接口测试</span>
</el-menu-item>
```

在 import 中添加 `Connection`：

```typescript
import { Setting, Monitor, DataLine, Fold, ArrowDown, Connection } from '@element-plus/icons-vue'
```

- [ ] **Step 3: 编译验证**

Run: `cd /root/IEC-SIM/iec104-sim-master/web && npx tsc --noEmit`
Expected: 无错误

- [ ] **Step 4: Commit**

```bash
git add web/src/router/index.ts web/src/App.vue
git commit -m "feat(frontend): add proxy route and sidebar menu"
```

---

### Task 6: 前端 — ProxyPage 主页面

**Files:**
- Create: `web/src/views/ProxyPage.vue`

- [ ] **Step 1: 创建 ProxyPage.vue 完整组件**

```vue
<template>
  <div class="proxy-page">
    <div class="proxy-layout">
      <!-- Collection Panel -->
      <div class="collection-panel">
        <div class="collection-header">
          <el-input v-model="searchText" placeholder="搜索请求..." size="small" clearable />
          <div class="collection-actions">
            <el-button size="small" @click="addFolder">📁 文件夹</el-button>
            <el-button size="small" @click="addRequest">＋ 请求</el-button>
          </div>
        </div>
        <div class="collection-tree">
          <div v-for="(folder, fi) in filteredCollections" :key="folder.id" class="tree-folder">
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
                @click="loadRequest(req)" @contextmenu.prevent="showCtx($event, req)">
                <span class="req-method" :class="req.method">{{ req.method }}</span>
                <span class="req-name">{{ req.name }}</span>
                <span class="req-actions">
                  <el-button text size="small" @click.stop="copyRequest(req)">📋</el-button>
                  <el-button text size="small" @click.stop="deleteRequest(req.id)">🗑</el-button>
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Editor Panel -->
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
            <div class="panel-header">
              <span>请求配置</span>
            </div>
            <div class="panel-body">
              <!-- Headers -->
              <div v-if="activeTab === 'headers'">
                <div v-for="(h, i) in headerList" :key="i" class="kv-row">
                  <el-checkbox v-model="h.enabled" />
                  <el-input v-model="h.key" placeholder="Key" size="small" />
                  <el-input v-model="h.value" placeholder="Value" size="small" />
                  <el-button text size="small" @click="headerList.splice(i, 1)">×</el-button>
                </div>
                <el-button size="small" @click="headerList.push({ key: '', value: '', enabled: true })">+ 添加</el-button>
              </div>

              <!-- Body -->
              <div v-if="activeTab === 'body'">
                <el-radio-group v-model="bodyType" size="small" style="margin-bottom: 8px;">
                  <el-radio-button value="json">JSON</el-radio-button>
                  <el-radio-button value="text">Text</el-radio-button>
                  <el-radio-button value="none">None</el-radio-button>
                </el-radio-group>
                <el-input v-if="bodyType !== 'none'" v-model="request.body" type="textarea"
                  :rows="8" placeholder="请求体..." />
              </div>

              <!-- Pre-Script -->
              <div v-if="activeTab === 'pre-script'">
                <div style="font-size: 12px; color: var(--el-text-color-secondary); margin-bottom: 8px;">
                  当前环境: <span style="color: var(--accent)">{{ activeEnvName }}</span>
                </div>
                <div v-for="(v, k) in activeEnvVars" :key="k" class="kv-row">
                  <el-input :model-value="k" size="small" disabled style="flex: 0.6;" />
                  <el-input :model-value="v" size="small" disabled />
                </div>
                <div style="margin-top: 8px; font-size: 11px; color: var(--el-text-color-placeholder);">
                  在 URL/Headers/Body 中使用 <code style="background: var(--el-fill-color-dark); padding: 1px 6px; border-radius: 3px;">{{`{{变量名}}`}}</code> 引用变量
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
              <div v-else-if="response?.error" style="color: var(--el-color-danger); padding: 16px;">
                {{ response.error }}
              </div>
              <div v-else class="empty-state">
                <div class="icon">📡</div>
                <p>点击「发送」发起请求</p>
              </div>
            </div>
          </div>
        </div>

        <div class="env-bar">
          <span style="font-size: 12px; color: var(--el-text-color-secondary);">环境:</span>
          <el-select v-model="activeEnvId" size="small" @change="switchEnv" style="width: 140px;">
            <el-option v-for="env in environments" :key="env.id" :value="env.id" :label="env.name" />
          </el-select>
          <el-button text size="small" @click="showEnvModal = true">⚙ 管理</el-button>
        </div>
      </div>
    </div>

    <!-- History Panel -->
    <el-drawer v-model="showHistory" title="请求历史" size="380px">
      <div v-for="(h, i) in history" :key="i" class="history-item" @click="loadHistory(h)">
        <span class="history-method" :class="h.method">{{ h.method }}</span>
        <span class="history-url">{{ h.url }}</span>
        <div class="history-meta">
          <span :style="{ color: h.status < 400 ? 'var(--el-color-success)' : 'var(--el-color-danger)' }">
            ● {{ h.status }}
          </span>
          <span>{{ h.time_ms }}ms</span>
        </div>
      </div>
    </el-drawer>

    <!-- Env Modal -->
    <el-dialog v-model="showEnvModal" title="环境变量管理" width="560px">
      <div style="display: flex; gap: 16px;">
        <div style="width: 140px; border-right: 1px solid var(--el-border-color); padding-right: 12px;">
          <div v-for="env in environments" :key="env.id"
            class="env-item" :class="{ active: env.id === activeEnvId }" @click="editEnv = env">
            {{ env.name }}
          </div>
          <el-button size="small" style="margin-top: 8px; width: 100%;" @click="addEnvironment">+ 新建</el-button>
        </div>
        <div style="flex: 1;" v-if="editEnv">
          <el-form label-position="top" size="small">
            <el-form-item label="环境名称">
              <el-input v-model="editEnv.name" />
            </el-form-item>
            <el-form-item label="变量列表">
              <div v-for="(v, k) in editEnv.variables" :key="k" class="kv-row">
                <el-input :model-value="k" size="small" style="flex: 0.6;" disabled />
                <el-input :model-value="v" size="small" disabled />
              </div>
            </el-form-item>
          </el-form>
        </div>
      </div>
      <template #footer>
        <el-button @click="showEnvModal = false">取消</el-button>
        <el-button type="warning" @click="saveEnvironments">保存</el-button>
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

const request = reactive({
  method: 'GET',
  url: '',
  headers: {} as Record<string, string>,
  body: '',
})

const headerList = ref<{ key: string; value: string; enabled: boolean }[]>([])
const response = ref<ProxyResponse | null>(null)
const activeRequestId = ref('')

const collections = ref<(CollectionItem & { _open?: boolean })[]>([])
const environments = ref<ProxyEnvironment[]>([])
const activeEnvId = ref('')
const editEnv = ref<ProxyEnvironment | null>(null)
const history = ref<any[]>([])

const filteredCollections = computed(() => {
  if (!searchText.value) return collections.value
  const q = searchText.value.toLowerCase()
  return collections.value.filter(f =>
    f.name.toLowerCase().includes(q) ||
    (f.children || []).some(r => r.name.toLowerCase().includes(q))
  )
})

const activeEnvName = computed(() => {
  const env = environments.value.find(e => e.id === activeEnvId.value)
  return env?.name || '无'
})

const activeEnvVars = computed(() => {
  const env = environments.value.find(e => e.id === activeEnvId.value)
  return env?.variables || {}
})

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
    const vars = activeEnvVars.value
    const resolvedUrl = request.url.replace(/\{\{(\w+)\}\}/g, (_, k) => vars[k] || `{{${k}}}`)
    const resolvedBody = request.body.replace(/\{\{(\w+)\}\}/g, (_, k) => vars[k] || `{{${k}}}`)
    const resolvedHeaders: Record<string, string> = {}
    for (const [k, v] of Object.entries(headers)) {
      resolvedHeaders[k] = v.replace(/\{\{(\w+)\}\}/g, (_, k2) => vars[k2] || `{{${k2}}}`)
    }

    const res = await proxyRequest({
      method: request.method,
      url: resolvedUrl,
      headers: resolvedHeaders,
      body: bodyType.value === 'none' ? '' : resolvedBody,
      timeout: 30,
    })
    response.value = res

    const entry = { method: request.method, url: request.url, status: res.status, time_ms: res.time_ms, timestamp: new Date().toLocaleTimeString() }
    history.value.unshift(entry)
    if (history.value.length > 50) history.value.pop()
    localStorage.setItem('proxy_history', JSON.stringify(history.value))
  } catch (e: any) {
    response.value = { status: 0, status_text: 'Error', headers: {}, body: '', time_ms: 0, size: 0, error: e.message }
  } finally {
    sending.value = false
  }
}

function highlightJson(body: string) {
  try {
    const obj = JSON.parse(body)
    const pretty = JSON.stringify(obj, null, 2)
    return pretty.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
      .replace(/("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+-]?\d+)?)/g,
        (match: string) => {
          if (/^"/.test(match)) return /:$/.test(match) ? `<span class="json-key">${match}</span>` : `<span class="json-string">${match}</span>`
          if (/true|false/.test(match)) return `<span class="json-bool">${match}</span>`
          if (/null/.test(match)) return `<span class="json-null">${match}</span>`
          return `<span class="json-number">${match}</span>`
        })
  } catch { return body }
}

function addFolder() {
  ElMessageBox.prompt('文件夹名称', '新建文件夹').then(({ value }) => {
    if (!value) return
    const folder: CollectionItem = { id: genId(), name: value, type: 'folder', children: [], _open: true } as any
    collections.value.push(folder)
    saveCollection(folder)
  })
}

function addRequest() {
  ElMessageBox.prompt('请求名称', '新建请求').then(({ value }) => {
    if (!value) return
    const req: CollectionItem = { id: genId(), name: value, type: 'request', method: 'GET', url: '', headers: {}, body: '' }
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
  headerList.value = Object.entries(req.headers || {}).map(([k, v]) => ({ key: k, value: v, enabled: true }))
}

function copyRequest(req: CollectionItem) {
  const copy: CollectionItem = { ...req, id: genId(), name: req.name + ' (副本)' }
  const folder = collections.value.find(f => f.children?.some(r => r.id === req.id))
  if (folder) {
    folder.children!.push(copy)
    saveCollection(folder)
  }
}

function deleteRequest(id: string) {
  ElMessageBox.confirm('确定删除？', '提示', { type: 'warning' }).then(() => {
    deleteCollection(id)
    collections.value = collections.value.filter(f => {
      if (f.children) f.children = f.children.filter(r => r.id !== id)
      return true
    })
  })
}

function renameFolder(folder: CollectionItem) {
  ElMessageBox.prompt('新名称', '重命名文件夹', { inputValue: folder.name }).then(({ value }) => {
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
  if (!activeRequestId.value) {
    addRequest()
    return
  }
  const headers: Record<string, string> = {}
  headerList.value.filter(h => h.enabled && h.key).forEach(h => { headers[h.key] = h.value })
  for (const folder of collections.value) {
    const req = folder.children?.find(r => r.id === activeRequestId.value)
    if (req) {
      req.method = request.method
      req.url = request.url
      req.body = request.body
      req.headers = headers
      await saveCollection(folder)
      ElMessage.success('已保存')
      return
    }
  }
}

function loadHistory(h: any) {
  request.method = h.method
  request.url = h.url
  showHistory.value = false
}

function switchEnv(id: string) { activateEnvironment(id) }

function addEnvironment() {
  ElMessageBox.prompt('环境名称', '新建环境').then(({ value }) => {
    if (!value) return
    const env: ProxyEnvironment = { id: genId(), name: value, variables: {} }
    environments.value.push(env)
    editEnv.value = env
  })
}

async function saveEnvironments() {
  for (const env of environments.value) {
    await saveEnvironment(env)
  }
  showEnvModal.value = false
  ElMessage.success('环境变量已保存')
}

function showCtx(e: MouseEvent, req: CollectionItem) {
  // placeholder for context menu
}

onMounted(loadData)
</script>

<style scoped>
.proxy-page { height: 100%; display: flex; flex-direction: column; }
.proxy-layout { flex: 1; display: flex; overflow: hidden; }
.collection-panel { width: 260px; background: var(--el-bg-color); border-right: 1px solid var(--el-border-color); display: flex; flex-direction: column; }
.collection-header { padding: 12px; border-bottom: 1px solid var(--el-border-color); display: flex; flex-direction: column; gap: 8px; }
.collection-actions { display: flex; gap: 6px; }
.collection-tree { flex: 1; overflow-y: auto; padding: 8px 0; }
.tree-folder-header { display: flex; align-items: center; gap: 6px; padding: 6px 12px; cursor: pointer; font-size: 13px; }
.tree-folder-header:hover { background: var(--el-fill-color-light); }
.tree-children { padding-left: 16px; }
.tree-children.collapsed { display: none; }
.tree-request { display: flex; align-items: center; gap: 6px; padding: 5px 12px 5px 28px; cursor: pointer; font-size: 12px; border-left: 2px solid transparent; }
.tree-request:hover { background: var(--el-fill-color-light); }
.tree-request.active { color: var(--el-color-warning); background: rgba(245,158,11,0.08); border-left-color: var(--el-color-warning); }
.req-method { font-family: 'JetBrains Mono', monospace; font-size: 10px; font-weight: 600; padding: 1px 4px; border-radius: 3px; }
.req-method.GET { background: rgba(34,197,94,0.15); color: #22c55e; }
.req-method.POST { background: rgba(245,158,11,0.15); color: #f59e0b; }
.req-method.PUT { background: rgba(59,130,246,0.15); color: #3b82f6; }
.req-method.DELETE { background: rgba(239,68,68,0.15); color: #ef4444; }
.req-actions { display: none; gap: 2px; }
.tree-request:hover .req-actions { display: flex; }
.editor-panel { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.request-bar { display: flex; gap: 8px; padding: 12px 16px; background: var(--el-bg-color); border-bottom: 1px solid var(--el-border-color); }
.method-select :deep(.el-input__wrapper) { background: var(--el-fill-color-darker); }
.tabs-area { display: flex; border-bottom: 1px solid var(--el-border-color); background: var(--el-bg-color); padding: 0 16px; }
.tab { padding: 9px 16px; font-size: 13px; font-weight: 500; color: var(--el-text-color-secondary); cursor: pointer; border-bottom: 2px solid transparent; }
.tab:hover { color: var(--el-text-color-regular); }
.tab.active { color: var(--el-color-warning); border-bottom-color: var(--el-color-warning); }
.content-split { display: flex; flex: 1; overflow: hidden; }
.request-panel { flex: 1; display: flex; flex-direction: column; border-right: 1px solid var(--el-border-color); overflow: hidden; }
.response-panel { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.panel-header { padding: 8px 14px; font-size: 11px; font-weight: 600; color: var(--el-text-color-secondary); text-transform: uppercase; letter-spacing: 0.5px; background: var(--el-bg-color); border-bottom: 1px solid var(--el-border-color); display: flex; align-items: center; justify-content: space-between; }
.panel-body { flex: 1; overflow-y: auto; padding: 10px 14px; }
.kv-row { display: flex; gap: 6px; margin-bottom: 5px; align-items: center; }
.response-status { display: flex; gap: 12px; padding: 8px 14px; background: var(--el-bg-color); border-bottom: 1px solid var(--el-border-color); }
.response-body { flex: 1; overflow-y: auto; padding: 14px; }
.json-view { background: var(--el-fill-color-darker); border: 1px solid var(--el-border-color); border-radius: 8px; padding: 14px; font-family: 'JetBrains Mono', monospace; font-size: 12px; line-height: 1.7; white-space: pre-wrap; word-break: break-all; }
.env-bar { display: flex; align-items: center; gap: 8px; padding: 6px 16px; background: var(--el-fill-color-light); border-top: 1px solid var(--el-border-color); }
.env-item { padding: 8px 10px; border-radius: 6px; cursor: pointer; font-size: 12px; margin-bottom: 2px; }
.env-item:hover { background: var(--el-fill-color-light); }
.env-item.active { background: rgba(245,158,11,0.1); color: var(--el-color-warning); }
.history-item { padding: 10px 12px; border-radius: 8px; cursor: pointer; margin-bottom: 4px; }
.history-item:hover { background: var(--el-fill-color-light); }
.history-method { font-family: 'JetBrains Mono', monospace; font-size: 10px; font-weight: 600; padding: 2px 6px; border-radius: 3px; margin-right: 8px; }
.history-method.GET { background: rgba(34,197,94,0.15); color: #22c55e; }
.history-method.POST { background: rgba(245,158,11,0.15); color: #f59e0b; }
.history-method.PUT { background: rgba(59,130,246,0.15); color: #3b82f6; }
.history-method.DELETE { background: rgba(239,68,68,0.15); color: #ef4444; }
.history-url { font-family: 'JetBrains Mono', monospace; font-size: 11px; color: var(--el-text-color-secondary); word-break: break-all; }
.history-meta { margin-top: 4px; font-size: 10px; color: var(--el-text-color-placeholder); display: flex; gap: 10px; }
.empty-state { display: flex; flex-direction: column; align-items: center; justify-content: center; height: 100%; color: var(--el-text-color-placeholder); gap: 10px; }
.empty-state .icon { font-size: 40px; opacity: 0.3; }
.empty-state p { font-size: 12px; }
</style>
```

- [ ] **Step 2: 编译验证**

Run: `cd /root/IEC-SIM/iec104-sim-master/web && npx vue-tsc --noEmit`
Expected: 无严重错误

- [ ] **Step 3: Commit**

```bash
git add web/src/views/ProxyPage.vue
git commit -m "feat(frontend): add ProxyPage with collection tree and env support"
```

---

### Task 7: 端到端验证

- [ ] **Step 1: 编译后端**

Run: `cd /root/IEC-SIM/iec104-sim-master && go build -o bin/gridsim ./cmd/gridsim/`
Expected: 编译成功

- [ ] **Step 2: 编译前端**

Run: `cd /root/IEC-SIM/iec104-sim-master/web && npm run build`
Expected: 构建成功

- [ ] **Step 3: 启动服务验证**

Run: `./bin/gridsim serve --http :8989 --config-dir ./config --log-dir ./logs`

验证：
1. 浏览器访问 http://localhost:8989 → 左侧菜单出现「接口测试」
2. 点击进入 → 页面正常渲染
3. 环境选择下拉框可用
4. Collection 树形列表可操作

- [ ] **Step 4: 测试代理 API**

```bash
curl -X POST http://localhost:8989/api/v1/proxy \
  -H 'Content-Type: application/json' \
  -d '{"method":"GET","url":"http://localhost:8989/api/v1/status"}'
```

Expected: 返回 `{"status":200,"status_text":"OK",...}`

- [ ] **Step 5: 最终 Commit**

```bash
git add -A
git commit -m "feat: complete API tester with collection tree, env vars, and proxy"
```
