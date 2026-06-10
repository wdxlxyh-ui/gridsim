# 微电网自定义测点 — 基础模板+自定义 设计方案

> 日期: 2026-05-24 | 状态: 方案评审

---

## 1. 目标

让用户在设备上自由添加自定义测点（如电芯电压、温度等）。基础测点（引擎管理）只读展示，自定义测点可增删。

## 2. 数据模型

### 2.1 无需修改（已有）

```go
// model.go — 已存在
type CustomPoint struct {
    Name string `json:"name"`   // 测点名称（如 "电芯1电压"）
    Type string `json:"type"`   // AI/DI/DO/AO
}

type Device struct {
    // ... 现有字段
    CustomPoints []CustomPoint `json:"custom_points,omitempty"` // ← 已有
}
```

### 2.2 pointmap.go — 需修改

自定义测点 IOA 接在基础测点之后：

```go
// 基础点 (固定):
//   AI: baseAI+0~4
//   DI: baseDI+0~1
//   AO: baseAO
//   DO: baseDO

// 自定义测点 (追加):
//   AI: baseAI+5, +7, +9... (间隔2,便于扩展)
//   DI: baseDI+5, +7...
//   AO: baseAO+5, +7...
//   DO: baseDO+5, +7...

// 在 per-device switch 之后追加:
for ci, cp := range dev.CustomPoints {
    offset := 5 + ci*2
    // ... 已有逻辑 (line 97-136)
}
```

### 2.3 handler.go — 无需修改

`saveTopology` 序列化整个 `Topology` JSON，`Device.CustomPoints` 已包含在内。

---

## 3. 前端交互设计

### 3.1 设备编辑弹窗增加 "自定义测点" 区域

```
┌─ 编辑设备参数 ────────────────────────────────┐
│                                                │
│ 设备名称: [ESS-1           ]                    │
│ 控制模式: ○ 远方  ● 本地                       │
│                                                │
│ ══ 类型参数 ══                                 │
│ 额定容量: [100] kWh   额定功率: [200] kW        │
│ SOC范围: [10]% ~ [90]%                          │
│                                                │
│ ══ 测点配置 ══                                 │
│ ┌─ 基础测点 (引擎管理，只读) ─────────────────┐ │
│ │ SOC         AI  │ BS.Soc             │      │ │
│ │ 充放电功率  AI  │ BS.ActivePW        │      │ │
│ │ 运行状态    DI  │ BS.Status          │      │ │
│ │ 开关状态    DI  │ BS.CtrlState       │      │ │
│ │ 功率设定    AO  │ BS.SysAPSetPoint   │      │ │
│ │ 远程启机    DO  │ BS.Start           │      │ │
│ └─────────────────────────────────────────────┘ │
│                                                │
│ ┌─ 自定义测点 ────────────────────────────────┐ │
│ │ [+ 添加测点]                                 │ │
│ │                                              │ │
│ │ 名称             类型        操作             │ │
│ │ 电芯1电压        AI ▼       [删除]           │ │
│ │ 电芯1温度        AI ▼       [删除]           │ │
│ │                                              │ │
│ │ 💡 完整点名自动生成: "储能1_电芯1电压"        │ │
│ │ 💡 IOA由系统分配，alias=测点名称               │ │
│ └──────────────────────────────────────────────┘ │
│                                                │
│                  [取消]    [保存]                │
└────────────────────────────────────────────────┘
```

### 3.2 添加自定义测点弹出层

```
┌─ 添加自定义测点 ──────────────────────┐
│                                        │
│ 测点名称: [电芯1最高温度          ]     │
│ 测点类型: [AI  ▼]  (AI/DI/DO/AO)      │
│                                        │
│ 完整点名: 储能1_电芯1最高温度           │
│ IOA由系统自动分配，无需输入             │
│                                        │
│            [取消]  [确认添加]           │
└────────────────────────────────────────┘
```

> **IOA 无需用户输入**：系统根据设备索引 + 测点类型自动分配，避免冲突。
> **EGC 标识符无需输入**：自动使用测点名称作为 alias。

### 3.3 前端状态

```typescript
// CustomPoint 只需要 Name 和 Type
interface CustomPoint {
    name: string   // 用户输入 "电芯1电压"
    type: string   // AI/DI/DO/AO
}

// 在 editDevice 时初始化
function editDevice(dev: MicrogridDevice) {
    editingDevice.value = { ...dev }
    editingCustomPoints.value = [...(dev.custom_points || [])]
}

// 添加时不涉及IOA
function addCustomPoint() {
    editingCustomPoints.value.push({
        name: newPointName.value,
        type: newPointType.value,
    })
}
```

---

## 4. 实施任务

### Task 1: pointmap.go — 无改动

自定义测点的 IOA 分配和展开逻辑已实现（line 97-136），无需修改。

### Task 2: 前端 — 编辑弹窗加测点区域

**文件**: `MicrogridEditor.vue`

**改动**:
1. 在编辑弹窗参数区域之后，添加「测点配置」卡片
2. 基础测点列表（只读，灰色底）
3. 自定义测点列表（可增删）
4. 添加自定义测点子弹窗
5. `editingCustomPoints` 状态
6. `handleUpdateDevice` 包含 `custom_points`
7. `editDevice` 初始化自定义测点

**工作量**: ~120 行 Vue 模板 + ~30 行 TypeScript

### Task 3: 构建验证

```bash
go build ./... && cd web && npm run build
```

---

## 5. 自定义测点命名规则

| 字段 | 规则 | 示例 |
|------|------|------|
| store Name | `{设备前缀}_{用户输入名}` | `储能1_电芯1电压` |
| EGC Alias | 用户输入 | `BMS.Cell1Vol` |
| IOA | 基础点之后间隔2 | `155` (AI), `1155` (DI) |

---

## 6. 兼容性

- 现有设备 `CustomPoints` 为空 → 只显示基础测点
- 保存拓扑时 `CustomPoints` 随 JSON 持久化
- 导出 xlsx 时自定义测点自动出现在对应设备文件中

## 5. 自定义测点命名规则（修正版）

| 字段 | 来源 | 示例 |
|------|------|------|
| store Name | 系统: `{设备前缀}_{用户输入}` | `储能1_电芯1电压` |
| IOA | 系统: 基础点后间隔2 | `155` (AI) |
| Alias | 系统: 等同用户输入名 | `电芯1电压` |
| **用户只需输入** | **名称 + 类型** | `"电芯1电压", "AI"` |
