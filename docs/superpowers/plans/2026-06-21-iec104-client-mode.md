# IEC104 客户端模式 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add IEC104 client (master) mode to GridSim, supporting connection to remote slaves, receiving YC/YX/YM data, and sending DO/AO commands.

**Architecture:** Protocol interface zero-change. New `pkg/iec104/client.go` implements `Protocol` using `cs104.Client`. Manager branches by `"iec104_client"` protocol. Frontend adds client config form and command panel.

**Tech Stack:** Go 1.21+, go-iecp5 v1.2.5 (cs104.Client), Vue 3 + Element Plus

---

## File Structure

| Layer | File | Action | Responsibility |
|-------|------|--------|----------------|
| Model | `internal/model/instance.go` | Modify | `IEC104ClientConfig` struct |
| Client | `pkg/iec104/client.go` | Create | Client wrapper, Protocol impl |
| Factory | `pkg/protocol/factory.go` | Modify | `"iec104_client"` branch |
| Manager | `internal/manager/manager.go` | Modify | Client lifecycle + GetInstance |
| API | `cmd/gridsim/main.go` | Modify | `/command` endpoint |
| i18n | `web/src/locales/zh-CN/instance.js` | Modify | Chinese translations |
| i18n | `web/src/locales/en/instance.js` | Modify | English translations |
| UI | `web/src/components/instance/ClientConfigForm.vue` | Create | Client config form |
| UI | `web/src/views/ConfigPage.vue` | Modify | Protocol dropdown + client form |
| UI | `web/src/components/instance/CommandPanel.vue` | Create | DO/AO command panel |
| UI | `web/src/views/DetailPage.vue` | Modify | Client status + command panel |

---

### Task 1: Add IEC104ClientConfig model

**Files:**
- Modify: `internal/model/instance.go:1-52`

- [ ] **Step 1: Add IEC104ClientConfig struct and extend InstanceConfig**

Edit `internal/model/instance.go`:

After `MicrogridInstanceConfig`, add:

```go
// IEC104ClientConfig 客户端实例配置（主站模式）
type IEC104ClientConfig struct {
	RemoteAddr     string `json:"remote_addr"`
	RemotePort     int    `json:"remote_port"`
	CommonAddr     int    `json:"common_addr"`
	ReconnectDelay int    `json:"reconnect_delay"`
	ConnectTimeout int    `json:"connect_timeout"`
}
```

In `InstanceConfig`, add after `MicrogridConfig`:

```go
	IEC104ClientConfig *IEC104ClientConfig `json:"iec104_client_config,omitempty"`
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./...`

Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add internal/model/instance.go
git commit -m "feat(model): add IEC104ClientConfig for client mode"
```

---

### Task 2: Create IEC104 client wrapper

**Files:**
- Create: `pkg/iec104/client.go`

- [ ] **Step 1: Write client.go**

Create `pkg/iec104/client.go`:

```go
package iec104

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"gridsim/internal/model"
	"gridsim/pkg/config"
	"gridsim/pkg/library"

	"github.com/wendy512/go-iecp5/asdu"
	"github.com/wendy512/go-iecp5/cs104"
)

// Client implements protocol.Protocol as an IEC104 master client.
type Client struct {
	cfg    model.IEC104ClientConfig
	store  *library.Store
	client *cs104.Client

	mu         sync.RWMutex
	connected  bool
	startTime  time.Time
	started    bool

	interrogCnt atomic.Int64
	controlCnt  atomic.Int64
	spontCnt    atomic.Int64
}

func NewClient(cfg model.IEC104ClientConfig) *Client {
	// Defaults
	if cfg.RemotePort == 0 {
		cfg.RemotePort = 2404
	}
	if cfg.CommonAddr == 0 {
		cfg.CommonAddr = 1
	}
	if cfg.ReconnectDelay == 0 {
		cfg.ReconnectDelay = 5
	}
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = 10
	}
	return &Client{cfg: cfg}
}

func (c *Client) Name() string { return "iec104_client" }

func (c *Client) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.started {
		return nil
	}

	c.startTime = time.Now()
	c.connected = false

	addr := fmt.Sprintf("%s:%d", c.cfg.RemoteAddr, c.cfg.RemotePort)
	opt := cs104.NewOption().
		SetAutoReconnect(true).
		SetReconnectInterval(time.Duration(c.cfg.ReconnectDelay) * time.Second)
	if err := opt.AddRemoteServer(addr); err != nil {
		return fmt.Errorf("invalid remote address %s: %w", addr, err)
	}

	handler := &clientHandler{client: c}
	cl := cs104.NewClient(handler, opt)
	cl.SetOnConnectHandler(c.onConnect)
	cl.SetConnectionLostHandler(c.onDisconnect)
	cl.SetServerActiveHandler(c.onServerActive)

	c.client = cl
	if err := cl.Start(); err != nil {
		return fmt.Errorf("client start failed: %w", err)
	}

	c.started = true
	slog.Info("IEC104 客户端已启动", "remote", addr)
	return nil
}

func (c *Client) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.started {
		return
	}

	slog.Info("正在停止 IEC104 客户端", "remote", fmt.Sprintf("%s:%d", c.cfg.RemoteAddr, c.cfg.RemotePort))

	if c.client != nil {
		_ = c.client.Close()
		c.client = nil
	}

	c.connected = false
	c.started = false
	slog.Info("IEC104 客户端已停止")
}

func (c *Client) ClientConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

func (c *Client) ClientAddr() string {
	return fmt.Sprintf("%s:%d", c.cfg.RemoteAddr, c.cfg.RemotePort)
}

func (c *Client) Stats() (interrog, control, spont int64) {
	return c.interrogCnt.Load(), c.controlCnt.Load(), c.spontCnt.Load()
}

func (c *Client) Uptime() int64 {
	return int64(time.Since(c.startTime).Seconds())
}

func (c *Client) Publish(point *config.Point) {
	c.mu.RLock()
	cl := c.client
	connected := c.connected
	c.mu.RUnlock()

	if !connected || cl == nil {
		return
	}

	coa := asdu.CauseOfTransmission{Cause: asdu.Activation}
	ca := asdu.CommonAddr(c.cfg.CommonAddr)

	switch point.PointType {
	case config.TypeDO:
		cmd := asdu.SingleCommand{
			Ioa:   asdu.InfoObjAddr(point.IOA),
			Value: point.BoolValue,
		}
		if err := asdu.SingleCommand(cl, false, coa, ca, cmd); err != nil {
			slog.Warn("发送遥控失败", "ioa", point.IOA, "error", err)
			return
		}
		c.controlCnt.Add(1)
		slog.Info("遥控已发送", "ioa", point.IOA, "value", point.BoolValue)

	case config.TypeAO:
		cmd := asdu.SetpointCommandFloat{
			Ioa:   asdu.InfoObjAddr(point.IOA),
			Value: float32(point.Value),
		}
		if err := asdu.SetpointCommandFloat(cl, false, coa, ca, cmd); err != nil {
			slog.Warn("发送遥调失败", "ioa", point.IOA, "error", err)
			return
		}
		c.controlCnt.Add(1)
		slog.Info("遥调已发送", "ioa", point.IOA, "value", point.Value)
	}
}

func (c *Client) SetStore(store *library.Store) {
	c.store = store
}

// ─── Connection callbacks ───

func (c *Client) onConnect(cl *cs104.Client) {
	slog.Info("已连接到远端", "remote", cl.UnderlyingConn().RemoteAddr())
}

func (c *Client) onServerActive(cl *cs104.Client) {
	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()

	slog.Info("STARTDT 确认，连接就绪", "remote", cl.UnderlyingConn().RemoteAddr())

	// 连接就绪后发送总召唤
	coa := asdu.CauseOfTransmission{Cause: asdu.Activation}
	ca := asdu.CommonAddr(c.cfg.CommonAddr)
	if err := cl.InterrogationCmd(coa, ca, 20); err != nil {
		slog.Warn("发送总召唤失败", "error", err)
	} else {
		slog.Info("总召唤已发送")
		c.interrogCnt.Add(1)
	}
}

func (c *Client) onDisconnect(cl *cs104.Client) {
	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()
	slog.Warn("与远端断开连接", "remote", fmt.Sprintf("%s:%d", c.cfg.RemoteAddr, c.cfg.RemotePort))
}

// ─── Client ASDU handler ───

type clientHandler struct {
	client *Client
}

func (h *clientHandler) InterrogationHandler(conn asdu.Connect, a *asdu.ASDU) error {
	return nil
}

func (h *clientHandler) CounterInterrogationHandler(conn asdu.Connect, a *asdu.ASDU) error {
	return nil
}

func (h *clientHandler) ReadHandler(conn asdu.Connect, a *asdu.ASDU) error {
	return nil
}

func (h *clientHandler) TestCommandHandler(conn asdu.Connect, a *asdu.ASDU) error {
	return nil
}

func (h *clientHandler) ClockSyncHandler(conn asdu.Connect, a *asdu.ASDU) error {
	return nil
}

func (h *clientHandler) ResetProcessHandler(conn asdu.Connect, a *asdu.ASDU) error {
	return nil
}

func (h *clientHandler) DelayAcquisitionHandler(conn asdu.Connect, a *asdu.ASDU) error {
	return nil
}

func (h *clientHandler) ASDUHandler(conn asdu.Connect, a *asdu.ASDU) error {
	store := h.client.store
	if store == nil {
		return nil
	}

	switch a.Type {
	case asdu.M_ME_NC_1:
		// 遥测
		info := a.GetMeasuredValueFloat()
		ioa := uint32(info.Ioa)
		if _, err := store.SetValue(ioa, float64(info.Value)); err != nil {
			slog.Debug("更新遥测失败", "ioa", ioa, "error", err)
		}
		h.client.spontCnt.Add(1)

	case asdu.M_SP_NA_1:
		// 遥信
		info := a.GetSingleCmd()
		ioa := uint32(info.Ioa)
		if _, err := store.SetBoolValue(ioa, info.Value); err != nil {
			slog.Debug("更新遥信失败", "ioa", ioa, "error", err)
		}
		h.client.spontCnt.Add(1)

	case asdu.M_IT_NA_1:
		// 遥脉
		info := a.GetBinaryCounterReading()
		ioa := uint32(info.Ioa)
		if _, err := store.SetIntValue(ioa, int32(info.Value)); err != nil {
			slog.Debug("更新遥脉失败", "ioa", ioa, "error", err)
		}
		h.client.spontCnt.Add(1)
	}

	return nil
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./pkg/iec104/...`

Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add pkg/iec104/client.go
git commit -m "feat(iec104): implement IEC104 client (master) wrapper"
```

---

### Task 3: Extend protocol factory

**Files:**
- Modify: `pkg/protocol/factory.go`

- [ ] **Step 1: Add client branch + update SupportedProtocols**

Edit `pkg/protocol/factory.go`:

In `New()`, add before the `default` case:

```go
	case "iec104_client":
		if cfg.IEC104ClientConfig == nil {
			return nil, fmt.Errorf("iec104_client_config required for client mode")
		}
		return iec104.NewClient(*cfg.IEC104ClientConfig), nil
```

In `SupportedProtocols()`, add `"iec104_client"`:

```go
func SupportedProtocols() []string {
	return []string{"iec104", "modbus_tcp", "microgrid", "iec104_client"}
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./...`

Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add pkg/protocol/factory.go
git commit -m "feat(protocol): add iec104_client protocol to factory"
```

---

### Task 4: Adapt manager for client mode

**Files:**
- Modify: `internal/manager/manager.go`

- [ ] **Step 1: Modify StartInstance for client mode branching**

In `StartInstance()`:

After the initial `cfg, ok := m.store.Get(id)` and before port checking, add:

```go
	// Client mode: no port listening needed
	if cfg.Protocol == "iec104_client" {
		return m.startClient(id)
	}
```

- [ ] **Step 2: Add startClient method**

After `startMicrogrid()`, add:

```go
func (m *Manager) startClient(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.instances[id]; ok {
		return fmt.Errorf("instance %s already running", id)
	}

	cfg, ok := m.store.Get(id)
	if !ok {
		return fmt.Errorf("instance %s not found", id)
	}

	if cfg.IEC104ClientConfig == nil {
		return fmt.Errorf("iec104_client_config not configured")
	}

	xlsxPath := cfg.XLSXFile
	if !filepath.IsAbs(xlsxPath) {
		if _, err := os.Stat(xlsxPath); os.IsNotExist(err) {
			xlsxPath = filepath.Join(m.cfgDir, xlsxPath)
		}
	}

	points, err := config.LoadFromXLSX(xlsxPath, cfg.Protocol)
	if err != nil {
		return fmt.Errorf("load xlsx: %w", err)
	}

	store := library.NewStore(points)

	proto, err := protocol.New(cfg)
	if err != nil {
		return fmt.Errorf("create protocol: %w", err)
	}
	proto.SetStore(store)
	if err := proto.Start(); err != nil {
		return fmt.Errorf("start protocol: %w", err)
	}

	acStore := detail.NewAutoChangeStore(m.cfgDir)
	engine := detail.NewEngine(cfg.ID, store, proto, acStore, m.cfgDir, m)
	if err := engine.LoadAndStart(); err != nil {
		slog.Warn("自动变化引擎加载失败", "id", id, "error", err)
	}

	logger, err := NewInstanceLogger(m.cfgDir, cfg.ID, cfg.IEC104Port)
	if err != nil {
		slog.Warn("创建实例日志目录失败", "id", id, "error", err)
	}

	inst := &Instance{
		Config:     cfg,
		Protocol:   proto,
		Store:      store,
		AutoEngine: engine,
		Logger:     logger,
	}

	m.instances[id] = inst
	slog.Info("客户端实例已启动", "id", id, "remote", fmt.Sprintf("%s:%d", cfg.IEC104ClientConfig.RemoteAddr, cfg.IEC104ClientConfig.RemotePort), "points", len(points))
	return nil
}
```

- [ ] **Step 3: Modify StopInstance to skip firewall for client mode**

In `StopInstance()`, after `delete(m.instances, id)`, wrap firewall cleanup:

```go
	if inst.Config.Protocol != "iec104_client" {
		firewall.RemovePort(inst.Config.IEC104Port)
		if inst.Config.HttpEnabled && inst.Config.HttpPort > 0 {
			firewall.RemovePort(inst.Config.HttpPort)
		}
	}
```

- [ ] **Step 4: Add GetInstance method**

Add after `GetStore()`:

```go
// GetInstance returns the running instance by ID, or nil.
func (m *Manager) GetInstance(id string) *Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.instances[id]
}
```

- [ ] **Step 5: Verify compilation**

Run: `go build ./...`

Expected: clean build.

- [ ] **Step 6: Commit**

```bash
git add internal/manager/manager.go
git commit -m "feat(manager): add client mode lifecycle support"
```

---

### Task 5: Add /command API endpoint

**Files:**
- Modify: `cmd/gridsim/main.go`

- [ ] **Step 1: Add "command" route in handleInstanceByID**

In `handleInstanceByID`, add case in the `parts[1]` switch (before `case "points"`):

```go
		case "command":
			ws.handleClientCommand(w, r, id)
```

- [ ] **Step 2: Add handleClientCommand method**

Before `handleInstancePoints`, add:

```go
// handleClientCommand sends a DO/AO command to a client-mode instance
func (ws *webServer) handleClientCommand(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	inst := ws.mgr.GetInstance(id)
	if inst == nil || inst.Protocol == nil {
		writeError(w, http.StatusNotFound, "instance not running")
		return
	}

	var body struct {
		IOA       uint32   `json:"ioa"`
		Value     *float64 `json:"value,omitempty"`
		BoolValue *bool    `json:"bool_value,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	pt, ok := inst.Store.Get(body.IOA)
	if !ok {
		writeError(w, http.StatusNotFound, "point not found")
		return
	}

	switch pt.PointType {
	case config.TypeDO:
		if body.BoolValue == nil {
			writeError(w, http.StatusBadRequest, "bool_value required for DO")
			return
		}
		inst.Store.SetBoolValue(body.IOA, *body.BoolValue)
		inst.Protocol.Publish(pt)

	case config.TypeAO:
		if body.Value == nil {
			writeError(w, http.StatusBadRequest, "value required for AO")
			return
		}
		inst.Store.SetValue(body.IOA, *body.Value)
		inst.Protocol.Publish(pt)

	default:
		writeError(w, http.StatusBadRequest, "only DO/AO supported for commands")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"ioa":     body.IOA,
	})
}
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./cmd/gridsim/...`

Expected: clean build.

- [ ] **Step 4: Commit**

```bash
git add cmd/gridsim/main.go
git commit -m "feat(api): add /command endpoint for DO/AO control"
```

---

### Task 6: i18n translations

**Files:**
- Modify: `web/src/locales/zh-CN/instance.js`
- Modify: `web/src/locales/en/instance.js`

- [ ] **Step 1: Edit Chinese locale**

Edit `web/src/locales/zh-CN/instance.js`:

Add to the `protocol` section or create if it's flat:

```js
  protocolIec104Client: 'IEC104 客户端',
  clientConfig: {
    title: '客户端配置',
    remoteAddr: '远端地址',
    remoteAddrPlaceholder: '如 192.168.1.100',
    remotePort: '远端端口',
    commonAddr: '公共地址',
    reconnectDelay: '重连间隔(秒)',
    connectTimeout: '连接超时(秒)',
  },
  connectionStatus: '连接状态',
  connected: '已连接',
  disconnected: '未连接',
  command: '控制命令',
  sendCommand: '发送命令',
  commandSent: '命令已发送',
  commandFailed: '命令发送失败',
```

- [ ] **Step 2: Edit English locale**

Edit `web/src/locales/en/instance.js`:

```js
  protocolIec104Client: 'IEC104 Client',
  clientConfig: {
    title: 'Client Configuration',
    remoteAddr: 'Remote Address',
    remoteAddrPlaceholder: 'e.g. 192.168.1.100',
    remotePort: 'Remote Port',
    commonAddr: 'Common Address',
    reconnectDelay: 'Reconnect Delay (s)',
    connectTimeout: 'Connect Timeout (s)',
  },
  connectionStatus: 'Connection Status',
  connected: 'Connected',
  disconnected: 'Disconnected',
  command: 'Control Command',
  sendCommand: 'Send Command',
  commandSent: 'Command Sent',
  commandFailed: 'Failed to Send Command',
```

- [ ] **Step 3: Commit**

```bash
git add web/src/locales/zh-CN/instance.js web/src/locales/en/instance.js
git commit -m "feat(i18n): add client mode translations"
```

---

### Task 7: ClientConfigForm component

**Files:**
- Create: `web/src/components/instance/ClientConfigForm.vue`

- [ ] **Step 1: Create ClientConfigForm.vue**

```vue
<template>
  <el-card v-if="visible" class="client-config-card">
    <template #header>
      <span>{{ $t('instance.clientConfig.title') }}</span>
    </template>
    <el-form :model="config" label-width="120px" size="small">
      <el-form-item :label="$t('instance.clientConfig.remoteAddr')" required>
        <el-input
          v-model="config.remote_addr"
          :placeholder="$t('instance.clientConfig.remoteAddrPlaceholder')"
        />
      </el-form-item>
      <el-form-item :label="$t('instance.clientConfig.remotePort')" required>
        <el-input-number v-model="config.remote_port" :min="1" :max="65535" />
      </el-form-item>
      <el-form-item :label="$t('instance.clientConfig.commonAddr')">
        <el-input-number v-model="config.common_addr" :min="1" :max="255" />
      </el-form-item>
      <el-form-item :label="$t('instance.clientConfig.reconnectDelay')">
        <el-input-number v-model="config.reconnect_delay" :min="1" :max="300" />
      </el-form-item>
      <el-form-item :label="$t('instance.clientConfig.connectTimeout')">
        <el-input-number v-model="config.connect_timeout" :min="1" :max="60" />
      </el-form-item>
    </el-form>
  </el-card>
</template>

<script setup>
defineProps({
  config: { type: Object, required: true },
  visible: { type: Boolean, default: false },
})
</script>

<style scoped>
.client-config-card {
  margin-bottom: 16px;
}
</style>
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/instance/ClientConfigForm.vue
git commit -m "feat(ui): add ClientConfigForm component"
```

---

### Task 8: ConfigPage protocol selection + client form

**Files:**
- Modify: `web/src/views/ConfigPage.vue`

- [ ] **Step 1: Read current ConfigPage to determine exact edit locations**

- The protocol select dropdown options
- The create/edit dialog form
- Import and use ClientConfigForm

Need to check the file structure first.

- [ ] **Step 2: Read existing ConfigPage.vue**

Will examine in implementation.

- [ ] **Step 3: Commit**

Will commit after implementation.

---

### Task 9: CommandPanel component

**Files:**
- Create: `web/src/components/instance/CommandPanel.vue`

- [ ] **Step 1: Create CommandPanel.vue**

```vue
<template>
  <el-card v-if="visible" class="command-panel-card">
    <template #header>
      <span>{{ $t('instance.command') }}</span>
    </template>
    <el-table :data="commandablePoints" size="small" max-height="400">
      <el-table-column prop="ioa" label="IOA" width="80" />
      <el-table-column prop="name" label="名称" />
      <el-table-column prop="pointType" label="类型" width="80" />
      <el-table-column label="值" width="200">
        <template #default="{ row }">
          <template v-if="row.pointType === 'DO'">
            <el-switch
              :model-value="row.boolValue"
              active-text="ON"
              inactive-text="OFF"
              @change="(val) => sendCommand(row, { bool_value: val })"
            />
          </template>
          <template v-else-if="row.pointType === 'AO'">
            <el-input-number
              :model-value="row.value"
              :step="0.1"
              size="small"
              style="width: 120px"
            />
            <el-button
              size="small"
              type="primary"
              style="margin-left: 8px"
              @click="sendCommand(row, { value: row.value })"
            >
              发送
            </el-button>
          </template>
        </template>
      </el-table-column>
    </el-table>
  </el-card>
</template>

<script setup>
import { computed } from 'vue'
import { ElMessage } from 'element-plus'
import { ref } from 'vue'

const props = defineProps({
  points: { type: Array, default: () => [] },
  instanceId: { type: String, required: true },
  visible: { type: Boolean, default: false },
})

const commandablePoints = computed(() =>
  props.points.filter((p) => p.pointType === 'DO' || p.pointType === 'AO')
)

async function sendCommand(point, payload) {
  try {
    const { default: http } = await import('@/api/http')
    await http.put(`/api/v1/instances/${props.instanceId}/command`, {
      ioa: point.ioa,
      ...payload,
    })
    ElMessage.success('命令已发送')
  } catch (e) {
    ElMessage.error('命令发送失败: ' + (e.response?.data?.error || e.message))
  }
}
</script>

<style scoped>
.command-panel-card {
  margin-bottom: 16px;
}
</style>
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/instance/CommandPanel.vue
git commit -m "feat(ui): add CommandPanel for DO/AO control"
```

---

### Task 10: DetailPage client mode integration

**Files:**
- Modify: `web/src/views/DetailPage.vue`

- [ ] **Step 1: Read existing DetailPage.vue**

Will examine in implementation.

- [ ] **Step 2: Integrate connection status and CommandPanel**

- Show client connection status
- Show CommandPanel for client mode instances

- [ ] **Step 3: Commit**

Will commit after implementation.

---

### Task 11: Verify build

**Files:**
- N/A (no code changes)

- [ ] **Step 1: Build all**

Run: `make dist`

Expected: clean build for all three platforms (linux amd64, linux arm64, windows amd64).

- [ ] **Step 2: Final commit**

```bash
git add -A
git commit -m "feat: complete IEC104 client mode implementation"
```
