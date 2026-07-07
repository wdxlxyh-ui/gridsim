# GridSim OPC UA 服务端 — 前端变更说明（可直接落地版）

> 配套文档：`DESIGN-OPCUA支持.md`（整体方案）
> 本文只讲前端，基于现有真实代码逐文件给出"改哪一行、改成什么"，照此改完即可完整适配 `opcua_server` 规约。
> 约定：新规约标识符 `opcua_server`，默认端口 `4840`，端口字段复用 `iec104_port` 承载（与 Modbus 同一套做法），专属参数放 `opcua_config`。

---

## 0. 改动文件总览

| 文件 | 改动点 | 必要性 |
|------|--------|--------|
| `web/src/api/index.ts` | 新增 `OPCUAConfig` 类型；`InstanceConfig` / `InstanceState` 增加 `opcua_config` 字段 | 必须 |
| `web/src/views/ConfigPage.vue` | 规约单选项、说明文案、端口标签、OPC UA 专属表单、提交组装、`protoLabel`/`protoTagType`/`displayPort`、编辑回填 | 必须 |
| `web/src/views/MonitorPage.vue` | `protoLabel`/`protoTag`、连接状态文案 | 必须 |
| `web/src/views/DashboardPage.vue` | `protoLabel`/`protoTag`/`protoColor` | 必须 |
| `web/src/views/DetailPage.vue` | `isOPCUA` 计算属性、头部标签、端点信息卡、列展示策略 | 必须 |
| `web/src/components/PointTableEditor.vue` | 无需改（OPC UA 用标准列，`isModbus` 为 false 自动隐藏 Modbus 列） | 可选 |

下面逐文件给出 diff 级说明。

---

## 1. `web/src/api/index.ts`

### 1.1 新增 OPC UA 配置类型

在 `ModbusConfig` 接口（约第 36 行）附近新增：

```ts
export interface OPCUAConfig {
  port?: number              // OPC UA 监听端口，默认 4840
  namespace_uri?: string     // 命名空间标识，默认 "GridSim"
}
```

### 1.2 扩展 `InstanceConfig`

现有：

```ts
export interface InstanceConfig {
  // ...
  protocol?: string
  modbus_config?: ModbusConfig
  iec104_client_config?: IEC104ClientConfig
}
```

改为追加一行：

```ts
  modbus_config?: ModbusConfig
  iec104_client_config?: IEC104ClientConfig
  opcua_config?: OPCUAConfig            // ← 新增
```

### 1.3 扩展 `InstanceState`

同样在 `InstanceState` 接口里，`modbus_config?: ModbusConfig` 下追加：

```ts
  opcua_config?: OPCUAConfig            // ← 新增
```

> `getProtocols()` 无需改动，后端 `SupportedProtocols()` 加入 `opcua_server` 后会自动返回。

---

## 2. `web/src/views/ConfigPage.vue`

这是改动最集中的文件。OPC UA 是"服务端"语义，和 IEC104 服务端最接近：要端口、要点表、可启 HTTP，**不要**客户端那套远端地址。按下面 7 处改。

### 2.1 规约单选按钮（模板，约第 122 行）

现有：

```html
<el-radio-group v-model="form.protocol">
  <el-radio-button value="iec104">IEC104</el-radio-button>
  <el-radio-button value="iec104_client">IEC104 客户端</el-radio-button>
  <el-radio-button value="modbus_tcp">Modbus TCP</el-radio-button>
  <el-radio-button value="microgrid">微电网</el-radio-button>
</el-radio-group>
```

追加一项：

```html
  <el-radio-button value="microgrid">微电网</el-radio-button>
  <el-radio-button value="opcua_server">OPC UA 服务端</el-radio-button>
```

### 2.2 规约说明文案（模板，约第 128 行）

现有 `template` 链路最后是 `<template v-else>微电网...`。把微电网显式化，再加 OPC UA：

```html
<div style="font-size:12px;color:#64748b;margin-top:6px;line-height:1.4">
  <template v-if="form.protocol === 'iec104'">电力行业标准规约，适用于变电站自动化系统联调测试</template>
  <template v-else-if="form.protocol === 'iec104_client'">主站模式，连接真实 IEC104 从站设备，接收遥测/遥信并发送遥控/遥调</template>
  <template v-else-if="form.protocol === 'modbus_tcp'">工业自动化领域通用规约，适用于Modbus TCP设备仿真</template>
  <template v-else-if="form.protocol === 'microgrid'">微电网仿真场景，包含光伏、储能、负荷等设备的一体化仿真</template>
  <template v-else-if="form.protocol === 'opcua_server'">OPC UA 服务端模拟器，点表测点映射为 OPC UA 节点，支持 Client 读取/写入/订阅。当前仅 None 安全策略 + 匿名鉴权，建议仅在内网测试使用</template>
</div>
```

### 2.3 网络配置步骤（Step 2，模板，约第 159 行）

现有结构是 `iec104_client` 一个 `<template>`，其余走 `<template v-else>`（含端口、Modbus 从站地址、字节序）。OPC UA 走 `v-else` 分支即可（端口标签需区分），并补充命名空间字段。

将端口标签那行（约第 160 行）：

```html
<el-form-item :label="form.protocol === 'modbus_tcp' ? 'Modbus端口' : 'IEC104端口'" prop="iec104_port">
```

改为用统一函数（见 2.8 新增 `portLabel`）：

```html
<el-form-item :label="portLabel(form.protocol)" prop="iec104_port">
```

在 Modbus 的从站/字节序两个 `el-form-item` 之后、`</template>` 之前，新增 OPC UA 专属项：

```html
  <el-form-item v-if="form.protocol === 'opcua_server'" label="命名空间">
    <el-input v-model="opcuaNamespaceUri" placeholder="GridSim" />
    <div style="font-size:12px;color:#64748b;margin-top:4px">OPC UA Namespace URI，默认 GridSim</div>
  </el-form-item>
  <el-form-item v-if="form.protocol === 'opcua_server'" label="端点预览">
    <el-input :model-value="opcuaEndpointPreview" readonly>
      <template #append>
        <el-button @click="copyEndpoint">复制</el-button>
      </template>
    </el-input>
  </el-form-item>
```

### 2.4 确认步骤（Step 4，模板，约第 230 行）

`review-section` 列表中：端口那块已有 `v-if="form.protocol !== 'iec104_client'"`，OPC UA 会自动显示端口（标签需替换为 `portLabel`）。把那行 review-label：

```html
<div class="review-label">{{ form.protocol === 'modbus_tcp' ? 'Modbus端口' : 'IEC104端口' }}</div>
```

改为：

```html
<div class="review-label">{{ portLabel(form.protocol) }}</div>
```

并在 Modbus 字节序 review-section 之后追加 OPC UA 命名空间确认项：

```html
<div class="review-section" v-if="form.protocol === 'opcua_server'">
  <div class="review-label">命名空间</div>
  <div class="review-value">{{ opcuaNamespaceUri || 'GridSim' }}</div>
</div>
```

> "创建后立即启动"提示语（约第 254 行）写死了 "IEC104 服务"，可改为中性文案：`实例创建后将自动启动服务`。

### 2.5 编辑模式表单（模板，约第 290 行）

编辑表单同样有 `iec104_client` / `v-else` 两分支。端口标签同样替换为 `portLabel(form.protocol)`，并在 Modbus 字节序项之后追加 OPC UA 命名空间项（与 2.3 同样的 `el-form-item`，但只保留命名空间输入，端点预览可选）：

```html
  <el-form-item v-if="form.protocol === 'opcua_server'" label="命名空间">
    <el-input v-model="opcuaNamespaceUri" placeholder="GridSim" />
  </el-form-item>
```

### 2.6 脚本：响应式状态（约第 470 行，`modbusSlaveId` 附近）

```ts
const modbusSlaveId = ref(1)
const modbusByteOrder = ref('ABCD')
const opcuaNamespaceUri = ref('GridSim')   // ← 新增
```

新增端点预览计算属性（放在 computed 区，如 `selectedFileInfo` 附近）：

```ts
const opcuaEndpointPreview = computed(
  () => `opc.tcp://${window.location.hostname || 'localhost'}:${form.value.iec104_port || 4840}`
)

async function copyEndpoint() {
  try {
    await navigator.clipboard.writeText(opcuaEndpointPreview.value)
    ElMessage.success('端点已复制')
  } catch {
    ElMessage.warning('复制失败，请手动选择')
  }
}
```

### 2.7 脚本：默认端口联动（建议）

切换规约时给个合理默认端口，避免用户用 2404 起 OPC UA。在 `<script setup>` 增加 watch：

```ts
watch(() => form.value.protocol, (p) => {
  if (editing.value) return
  if (p === 'opcua_server' && form.value.iec104_port === 2404) form.value.iec104_port = 4840
  if (p === 'modbus_tcp' && form.value.iec104_port === 2404) form.value.iec104_port = 502
  if ((p === 'iec104' || p === 'microgrid') && (form.value.iec104_port === 4840 || form.value.iec104_port === 502)) {
    form.value.iec104_port = 2404
  }
})
```

### 2.8 脚本：提交组装（`handleSave`，约第 580 行）

现有：

```ts
const data: InstanceConfig = { ...form.value }
if (data.protocol === 'modbus_tcp') {
  data.modbus_config = { port: data.iec104_port, slave_id: modbusSlaveId.value, byte_order: modbusByteOrder.value }
}
if (data.protocol === 'iec104_client') {
  data.iec104_client_config = { ...clientConfig.value }
}
```

追加：

```ts
if (data.protocol === 'opcua_server') {
  data.opcua_config = {
    port: data.iec104_port,
    namespace_uri: opcuaNamespaceUri.value || 'GridSim',
  }
}
```

### 2.9 脚本：编辑回填（`handleEdit`，约第 540 行）

现有 `handleEdit` 末尾设置了 `modbusSlaveId` / `modbusByteOrder` 默认值。追加 OPC UA 回填：

```ts
modbusSlaveId.value = 1
modbusByteOrder.value = 'ABCD'
opcuaNamespaceUri.value = row.opcua_config?.namespace_uri || 'GridSim'   // ← 新增
```

`resetForm`（约第 615 行）也追加：

```ts
opcuaNamespaceUri.value = 'GridSim'
```

### 2.10 脚本：`portLabel` 新增 + 三个 proto 函数扩展（约第 718 行）

新增统一端口标签函数：

```ts
function portLabel(proto?: string): string {
  if (proto === 'modbus_tcp') return 'Modbus端口'
  if (proto === 'opcua_server') return 'OPC UA端口'
  return 'IEC104端口'
}
```

`protoLabel`（约第 718 行）追加分支：

```ts
function protoLabel(proto?: string): string {
  if (proto === 'modbus_tcp') return 'Modbus TCP'
  if (proto === 'microgrid') return '微电网'
  if (proto === 'iec104_client') return 'IEC104 客户端'
  if (proto === 'opcua_server') return 'OPC UA 服务端'   // ← 新增
  return 'IEC104'
}
```

`protoTagType`（约第 725 行）追加分支（OPC UA 用 `warning` 与微电网区分？微电网已占 warning，给 OPC UA 用 `danger` 或保持 `info`，这里建议复用 Element 既有色，用 `'warning'` 会与微电网撞色，选 `'danger'`）：

```ts
function protoTagType(proto?: string): 'success' | 'primary' | 'info' | 'warning' | 'danger' {
  if (proto === 'modbus_tcp') return 'success'
  if (proto === 'microgrid') return 'warning'
  if (proto === 'iec104_client') return 'info'
  if (proto === 'opcua_server') return 'danger'   // ← 新增
  return 'primary'
}
```

> 注意：返回类型联合需加 `'danger'`。

`displayPort`（约第 733 行）：OPC UA 走默认 `return String(row.iec104_port)` 即可正常显示，无需特殊分支。可选显式加：

```ts
if (row.protocol === 'opcua_server') return String(row.iec104_port)
```

---

## 3. `web/src/views/MonitorPage.vue`

### 3.1 `protoLabel`（约第 149 行）

```ts
function protoLabel(proto?: string): string {
  if (proto === 'modbus_tcp') return 'Modbus TCP'
  if (proto === 'microgrid') return '微电网'
  if (proto === 'iec104_client') return 'IEC104 客户端'
  if (proto === 'opcua_server') return 'OPC UA 服务端'   // ← 新增
  return 'IEC104'
}
```

### 3.2 `protoTag`（约第 157 行）

返回类型联合追加 `'danger'`，并加分支：

```ts
function protoTag(proto?: string): 'success' | 'warning' | 'info' | 'primary' | 'danger' {
  if (proto === 'modbus_tcp') return 'success'
  if (proto === 'microgrid') return 'warning'
  if (proto === 'iec104_client') return 'info'
  if (proto === 'opcua_server') return 'danger'   // ← 新增
  return 'primary'
}
```

### 3.3 连接状态文案（模板，约第 35 行）

现有对 `iec104_client` 显示"已连接/未连接"，其余显示"在线/离线"。OPC UA 是多 Client 服务端，"在线/离线"语义其实是"服务是否可用"，沿用 `v-else` 的"在线/离线"即可，无需单独分支。若想更准确，可加：

```html
<template v-else-if="inst.protocol === 'opcua_server'">
  {{ inst.stats.client_connected ? '服务运行' : '服务停止' }} |
</template>
<template v-else>
  {{ inst.stats.client_connected ? '在线' : '离线' }} |
</template>
```

> `openInstance`（约第 164 行）无需改：OPC UA 不是 microgrid，会走 `/detail/:id`，正确。

---

## 4. `web/src/views/DashboardPage.vue`

仪表盘的协议分布与实例列表，三个函数各加一分支。

### 4.1 `protoColor`（约第 278 行）

```ts
function protoColor(proto?: string): string {
  if (proto === 'modbus_tcp') return '#10b981'
  if (proto === 'microgrid') return '#f59e0b'
  if (proto === 'opcua_server') return '#ec4899'   // ← 新增，粉色，与现有蓝/绿/橙区分
  return '#3b82f6'
}
```

### 4.2 `protoLabel`（约第 284 行）

```ts
function protoLabel(proto?: string): string {
  if (proto === 'modbus_tcp') return 'Modbus TCP'
  if (proto === 'microgrid') return '微电网'
  if (proto === 'opcua_server') return 'OPC UA 服务端'   // ← 新增
  return 'IEC104'
}
```

> 注：此文件原 `protoLabel` 未处理 `iec104_client`，会落到 `return 'IEC104'`。本次可顺手补 `if (proto === 'iec104_client') return 'IEC104 客户端'`，非必须。

### 4.3 `protoTag`（约第 291 行）

```ts
function protoTag(proto?: string): 'success' | 'warning' | 'primary' | 'danger' {
  if (proto === 'modbus_tcp') return 'success'
  if (proto === 'microgrid') return 'warning'
  if (proto === 'opcua_server') return 'danger'   // ← 新增
  return 'primary'
}
```

> `by_protocol` 分布数据由后端聚合自动带出 `opcua_server` 键，前端无需额外处理。

---

## 5. `web/src/views/DetailPage.vue`

详情页的点表、实时值、置数、自动变化策略全部基于 Store，OPC UA 实例同构，**主体表格直接复用**。仅需补协议识别与头部信息。

### 5.1 脚本：新增 `isOPCUA` 计算属性（约第 698 行）

现有：

```ts
const instanceProtocol = ref('iec104')
const isModbus = computed(() => instanceProtocol.value === 'modbus_tcp')
const isClientMode = computed(() => instanceProtocol.value === 'iec104_client')
```

追加：

```ts
const isOPCUA = computed(() => instanceProtocol.value === 'opcua_server')
```

### 5.2 模板：头部规约标签（约第 14 行）

现有：

```html
<el-tag v-if="isClientMode" type="info" size="small" effect="plain">IEC104 客户端</el-tag>
<el-tag v-else-if="isModbus" type="success" size="small" effect="plain">Modbus TCP</el-tag>
```

追加：

```html
<el-tag v-else-if="isOPCUA" type="danger" size="small" effect="plain">OPC UA 服务端</el-tag>
```

### 5.3 模板：OPC UA 端点信息卡（建议，放在头部 card 之后、点表 card 之前）

给测试人员展示连接信息：

```html
<el-card shadow="never" v-if="isOPCUA && instanceStatus === 'running'" style="margin-bottom: 16px">
  <el-descriptions :column="2" size="small" border>
    <el-descriptions-item label="OPC UA 端点">
      opc.tcp://{{ host }}:{{ opcuaPort }}
    </el-descriptions-item>
    <el-descriptions-item label="命名空间">{{ opcuaNamespace }}</el-descriptions-item>
    <el-descriptions-item label="安全策略">None</el-descriptions-item>
    <el-descriptions-item label="鉴权">Anonymous</el-descriptions-item>
  </el-descriptions>
</el-card>
```

配套脚本（`loadInstanceState` 已设置 `instanceProtocol`，再补两个 ref + 取值）：

```ts
const host = computed(() => window.location.hostname || 'localhost')
const opcuaPort = ref(4840)
const opcuaNamespace = ref('GridSim')
```

在 `loadInstanceState`（约第 1707 行）的 `instanceProtocol.value = state.protocol || 'iec104'` 之后追加：

```ts
if (state.opcua_config) {
  opcuaPort.value = state.opcua_config.port || state.iec104_port || 4840
  opcuaNamespace.value = state.opcua_config.namespace_uri || 'GridSim'
} else {
  opcuaPort.value = state.iec104_port || 4840
}
```

### 5.4 列展示策略（模板，约第 154–169 行）

Modbus 专属列用 `v-if="isModbus"` 控制，OPC UA 时这些列自动隐藏，无需改。OPC UA 实例直接展示标准列（IOA / 名称 / 类型 / 实时值 / 操作）即可。

> 置数、自动变化、CSV 回放、QDS 等功能对 OPC UA 实例全部可用（写值→Store→`Publish`→订阅推送），无需禁用。`isClientMode` 的禁用分支与 OPC UA 无关，OPC UA 走"服务端"完整能力路径。

---

## 6. 联调验收清单

完成上述改动后，按以下步骤自测：

1. **创建**：配置页选「OPC UA 服务端」→ 端口自动变 4840 → 填名称、选点表、命名空间 GridSim → 端点预览显示 `opc.tcp://<host>:4840` → 确认页信息齐全 → 创建并启动。
2. **列表/仪表盘**：配置页、监控页、仪表盘均显示「OPC UA 服务端」标签与独立颜色；仪表盘协议分布出现 opcua_server 计数。
3. **详情页**：进入详情，头部显示 OPC UA 标签 + 端点信息卡；点表正常加载，无 Modbus 功能码列。
4. **互操作**：用 UAExpert / Prosys OPC UA Client 连 `opc.tcp://<host>:4840` → Browse 出全部测点节点 → 订阅 AI 点 → 在详情页对该点配「递增」自动变化 → Client 实时收到推送。
5. **写回**：Client 写某 AO/DO 节点 → 详情页该点实时值更新（轮询刷新可见）。
6. **回归**：新建一个 IEC104、一个 Modbus 实例，确认原功能不受影响；编辑已有实例端口标签正确。

---

## 7. 与后端的字段契约（务必一致）

前端按以下 JSON 与后端交互，后端 `OPCUAInstanceConfig`（见整体方案 §4.8）字段名需对齐：

```json
{
  "name": "OPCUA测试",
  "protocol": "opcua_server",
  "iec104_port": 4840,
  "xlsx_file": "points.xlsx",
  "http_enabled": true,
  "http_port": 8081,
  "opcua_config": {
    "port": 4840,
    "namespace_uri": "GridSim"
  }
}
```

- `iec104_port` 与 `opcua_config.port` 取相同值（沿用 Modbus 的做法：表单端口写入 `iec104_port`，提交时同时塞进 `opcua_config.port`），后端工厂优先取 `opcua_config.port`。
- `namespace_uri` 为空时后端回退 `GridSim`。
