# Microgrid V2 重构 — 功率方向统一 + 动态拓扑图 + SOC 复用

> **For agentic workers:** Each task below is independently verifiable. Use `go test ./internal/microgrid/` and `npm run build` after each task to validate.

**Goal:** 统一功率正负定义、实现动态功率流拓扑图、SOC 策略复用、测点固定排序。

**Architecture:** 
- 引擎层：翻转电池功率符号（charging=+，discharging=-），重写 `calcBatteryPowerLocked`/`updateSOC`/`calcPowerBalanceLocked`/`Dashboard`/`syncStoreLocked`
- 前端层：SVG 拓扑改为全设备滚动视图+CSS zoom 缩放+功率流着色动画，删除公式配置 Tab，测点表按 IOA 排序+1s 轮询

**Tech Stack:** Go 1.21+, Vue 3 + TypeScript, SVG + CSS transforms

**受影响文件:** `engine.go` (核心)、`handler.go` (排序)、`MicrogridEditor.vue` (模板/逻辑/样式)

---

## 需求排查 & 根因分析

### 1. 关口表公式符号

**现状：** `autoFormulas` 当前显示 `GRID_P = (负荷+充电桩) - (光伏+储能)`。

**问题：** 储能符号未区分充/放。实际上储能功率为正代表充电（用电），负代表放电（发电）。因此 GRID_P 在公式层面应为统一形式。

**修正公式：**
```
GRID_P = LOAD + CHARGER + Battery(charge>0) - PV - Battery(discharge<0的绝对值)
简化: GRID_P = Load + Charger + Battery - PV
```
其中 Battery 的正负已按照新规约（+ = charge, - = discharge）。

### 2. 拓扑图需要动态功率流 + 全设备 + 缩放

**现状：** `visibleDevices` 只取前 3 个；SVG 固定 600×480 viewBox；无缩放；无实时功率流动画。

**根因：** `maxVisibleDevices=3` 硬编码；SVG 坐标固定不随设备数变化；缺少 CSS transform 缩放；未用 running 状态驱动着色。

**方案：**
- 移除 `maxVisibleDevices` 限制，渲染所有设备
- SVG viewBox 高度动态计算：`viewBox="0 0 600 ${260 + devices.length * 140 + 40}"`
- 容器使用 CSS `overflow:auto` + 外部 `<div>` 包裹缩放控制（+/- 按钮、reset）
- 缩放通过 CSS `transform: scale(s)` 实现（操作 SVG 外层 div 的 style）
- 功率流向：合闸设备连线根据类型着色（绿/蓝/橙），每 1s 刷新 dash 数据驱动流向箭头方向
- 每个设备竖线旁标注实时功率值（从 store 点表轮询获取）

### 3. 储能 SOC 复用策略

**现状：** `calcBatteryPowerLocked` 内调用 `updateSOC`，但仅在 AO setpoint / 按时段调度分支内调用。公式求值后电池功率可能变化但 SOC 未更新。

**根因：** `updateSOC` 没有在每个 tick 无条件执行。SOC 计算仅耦合在特定代码路径中。

**方案：**
- 在 tick() step 3 之后、step 4 之前，对所有 battery 设备统一调用 `updateSOCByDevice`
- `updateSOCByDevice` 基于 step 3 计算的最终 `batPower[dev.ID]` 更新 SOC
- 公式：`ΔSOC = batPower * dtHours / CapacityKWH * 100`（charging+ → SOC↑, discharge- → SOC↓）
- 受限于 `[SOCMin, SOCMax]`

### 4. 功率正负含义统一

**现状（混乱）：**
| 量 | 当前约定 | 问题 |
|----|----------|------|
| PV | + = 发电 | ✅ 正确 |
| Battery | + = 放电 | ❌ 与常识相反 |
| Load | + = 用电 | ✅ 正确 |
| Grid | + = ? | ❌ 无明确定义 |

**新规约：**
| 量 | + 含义 | - 含义 | 范围 |
|----|--------|--------|------|
| PV | 发电 | 无（始终 ≥0） | [0, ratedPower] |
| Battery | 充电 | 放电 | [-ratedPower, +ratedPower] |
| Load | 用电 | 发电（罕见） | [0, ratedLoad] |
| Grid | 从电网用电 | 送电给电网 | [-cap, +cap] |

**影响范围（全部需修改）：**

| 函数 | 当前逻辑 | 需改为 |
|------|---------|--------|
| `calcBatteryPowerLocked` | 充电返回 `-chargeP`，放电返回 `+dischargeP` | **翻转**：充电返回 `+chargeP`，放电返回 `-dischargeP` |
| `updateSOC` | `deltaSOC = -power * dtHours / cap * 100` (power>0=放电→SOC↓) | `deltaSOC = +power * dtHours / cap * 100` (power>0=充电→SOC↑) |
| `calcPowerBalanceLocked` | `if p>0 {totalGen+=p}` (放电=发电) | `if p<0 {totalGen+=-p}` (放电=发电)；`if p>0 {totalLoad+=p}` (充电=用电) |
| `Dashboard` | 同上 | 同上 |
| `syncStoreLocked` `GRID_Q` | `e.gridPower * 0.15` | 符号跟随 GRID_P |
| 快照 `SimSnapshot` | 记录 raw batPower | 保持 raw 值（正负含义已变） |

### 5. 测点固定顺序 + 1s 刷新

**现状：** `store.GetAll()` 返回 map 迭代顺序（随机）；轮询仅刷仪表盘不刷测点。

**方案：**
- 在 `HandleMicrogridPoints` 中，对 store.GetAll() 的结果按 IOA 升序排序
- `startPolling` 将测点轮询加回，间隔 1000ms（独立于仪表盘的 3000ms）
- 前端 `fetchPoints` 也按返回的 IOA 顺序展示（后端排序后前端不动）

### 6. 移除公式配置界面

**现状：** 前端有「公式配置」Tab 和「公式预览」卡片。用户认为自动生成已足够。

**方案：** 删除模板中 Tab 3 (`<el-tab-pane label="公式配置">`) 和 `Formula Dialog`。保留拓扑 Tab 中的「公式预览」卡片。删除公式相关的 script 函数和 state。

---

## 实施计划

### Task 1: 翻转电池功率符号 (engine.go)

**影响函数:** `calcBatteryPowerLocked`, `updateSOC`, `tick`

- [ ] **Step 1: 修改 `calcBatteryPowerLocked` 返回值符号**

  当前（charge = 负）：
  ```go
  // PV peak - charge battery  →  return -chargeP
  // Evening peak - discharge  →  return dischargeP
  // Off-peak: charge low rate →  return -chargeP
  ```

  改为（charge = 正）：
  ```go
  // PV peak - charge battery  →  return +chargeP
  // Evening peak - discharge  →  return -dischargeP  
  // Off-peak: charge low rate →  return +chargeP
  ```

  同时修改注释 `// 计算电池功率 (+充电, -放电)`

  具体改动（三处 return 值）：
  - `return -chargeP` → `return chargeP`
  - `return dischargeP` → `return -dischargeP`
  - `return -chargeP` → `return chargeP`

- [ ] **Step 2: 修改 `updateSOC` deltaSOC 公式**

  当前（power>0=放电→SOC↓）：
  ```go
  // power positive = discharge (decrease SOC), negative = charge (increase SOC)
  deltaSOC := -power * dtHours / dev.Params.CapacityKWH * 100
  ```

  改为（power>0=充电→SOC↑）：
  ```go
  deltaSOC := power * dtHours / dev.Params.CapacityKWH * 100
  ```

- [ ] **Step 3: 在 tick() step 3 后添加 SOC 独立更新**

  在 `// 3. Calculate battery power` 循环后，step 4 之前，添加：
  ```go
  // 3.5 Update SOC based on computed battery power
  for _, dev := range e.topology.Devices {
      if dev.Type == CompBattery && dev.Switch.Closed {
          e.updateSOC(dev, e.batPower[dev.ID])
      }
  }
  ```

- [ ] **Step 4: 构建验证**

  ```bash
  cd /root/IEC-SIM/iec104-sim-master && go build ./internal/microgrid/
  ```
  预期：无错误

### Task 2: 重写功率平衡计算 (engine.go)

**影响函数:** `calcPowerBalanceLocked`, `Dashboard`, `syncStoreLocked`

- [ ] **Step 1: 重写 `calcPowerBalanceLocked`**

  当前逻辑（batPower>0=发电）：
  ```go
  case CompBattery:
      p := e.batPower[dev.ID]
      if p > 0 { totalGen += p } else { totalLoad += -p }
  ```

  新逻辑（batPower>0=充电=用电，batPower<0=放电=发电）：
  ```go
  case CompBattery:
      p := e.batPower[dev.ID]
      batChargeTotal += p
      if p < 0 {
          totalGen += -p   // discharge = generation
      } else {
          totalLoad += p   // charge = load
      }
  ```

  GRID_P 公式改为（grid>0=从电网用电）：
  ```go
  // GRID_P = totalLoad - totalGen (positive = importing)
  gridPower := totalLoad - totalGen
  if !island {
      // cap check...
      if gridPower > cap { gridPower = cap }
      else if gridPower < -cap { gridPower = -cap }
  }
  ```

  频率公式相应调整：
  ```go
  freq := 50.0
  if totalLoad+totalGen > 1 {
      freq = 50.0 - gridPower / (totalLoad+totalGen) * 0.5
  }
  ```

- [ ] **Step 2: 同步修改 `Dashboard`**

  与 calcPowerBalanceLocked 保持一致的符号逻辑。

- [ ] **Step 3: 同步修改 `syncStoreLocked`**

  `GRID_Q` 符号跟随 `GRID_P`（无需改动，`gridPower*0.15` 已经是正确的跟随后果）

- [ ] **Step 4: 运行测试验证**

  ```bash
  go test ./internal/microgrid/ -count=1 -v
  ```
  预期：所有测试通过（可能需要更新 formula_test.go 中的期望值）

### Task 3: 前端拓扑图重构 (MicrogridEditor.vue)

> **设计规格**: `docs/superpowers/topology-prototype-v2.html` — 最终 Vue 实现必须 1:1 匹配该原型。

**原型 v2 核心设计决策**:

| 特性 | 实现方式 | Vue 映射 |
|------|---------|---------|
| 防重叠 | `SVG_WIDTH = max(680, N×120+40)`, 每设备间距 ≥120px | `:width` + computed |
| 流动效 | `stroke-dasharray:8 8` + `@keyframes stroke-dashoffset:-24` | `<style scoped>` 同 CSS |
| 流向指示 | `.flow-up`(rev动画=发电↑) `.flow-dn`(fwd动画=用电↓) | `:class` 绑定 |
| 断开设备 | `.flow-z`(静态虚线+灰色) | 条件 class |
| 缩放 | `transform:scale()` + 按钮/Ctrl+滚轮 | `el-button-group` + wheel event |
| 功率标注 | 每个设备框下方 `XX kW` 实时值 | `dev.power \| fetchPoints` |
| 底部公式栏 | `GRID = (load+charger+bat) − PV = ±XX kW` | `autoFormulas` 最后元素 |
| 背景网格 | SVG pattern 40×40 | 直接复制 SVG defs |
| 全设备可见 | 无 maxVisibleDevices 限制 | `devices` 直接遍历 |
| 分闸显示 | 红开关 + 灰线 + 设备透明 + "已断开" | 条件渲染 |

- [ ] **Step 1: 动态 SVG viewBox**

  根据设备数量动态计算高度：
  ```html
  <svg :viewBox="`0 0 600 ${260 + devices.length * 140 + 40}`">
  ```
  移除 `maxVisibleDevices` 限制，`visibleDevices` 改为返回所有 `devices`。

- [ ] **Step 2: 缩放控制**

  在 SVG 外层包裹：
  ```html
  <div class="topology-zoom-controls">
    <el-button-group size="small">
      <el-button @click="zoomIn">+</el-button>
      <el-button @click="zoomOut">-</el-button>
      <el-button @click="zoomReset">1:1</el-button>
    </el-button-group>
  </div>
  <div class="topology-svg" :style="{ transform: `scale(${svgScale})`, transformOrigin: 'top left' }">
    <svg ...>
  ```

  添加 scale state 和 handler：
  ```ts
  const svgScale = ref(1)
  function zoomIn()  { svgScale.value = Math.min(3, +(svgScale.value + 0.2).toFixed(1)) }
  function zoomOut() { svgScale.value = Math.max(0.3, +(svgScale.value - 0.2).toFixed(1)) }
  function zoomReset(){ svgScale.value = 1 }
  ```

- [ ] **Step 3: 功率流实时着色 + 数值标注**

  tap 竖线和 wire 竖线的 stroke 绑定：
  - 合闸 + pv/load → 相应颜色
  - 合闸 + battery → 蓝色（充放都用蓝，箭头朝上=放电，朝下=充电）
  - 分闸 → 灰色

  每个设备框下方添加功率值标注（从点表读取）：
  ```html
  <text :x="100+idx*120" y="390" text-anchor="middle" font-size="10"
    :fill="running && dev.switch.closed ? '#303133' : '#c0c4cc'">
    {{ pointPower(dev.id) ?? '--' }} kW
  </text>
  ```

  需要在 script 中引入一个 per-device 功率查询（从 dash 或单独 polling）

- [ ] **Step 4: 构建验证**

  ```bash
  cd web && npm run build
  ```

### Task 4: 测点排序 + 1s 轮询

- [ ] **Step 1: handler.go 排序**

  在 `HandleMicrogridPoints` 中，对 store.GetAll() 结果排序：
  ```go
  import "sort"
  // 在获取 pts 后：
  sort.Slice(pts, func(i, j int) bool { return pts[i].IOA < pts[j].IOA })
  ```

- [ ] **Step 2: 前端 1s 轮询**

  分离仪表盘(3s)和测点(1s)轮询：
  ```ts
  let pointsTimer: ReturnType<typeof setInterval> | null = null

  function startPolling() {
    stopPolling()
    pollTimer = setInterval(() => fetchDashboard(), 3000)
    pointsTimer = setInterval(() => fetchPoints(), 1000)
  }

  function stopPolling() {
    if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
    if (pointsTimer) { clearInterval(pointsTimer); pointsTimer = null }
  }
  ```

### Task 5: 删除公式配置界面

- [ ] **Step 1: 删除模板中的公式 Tab 和 Dialog**

  删除整个 `<el-tab-pane label="公式配置">` 及其内容，以及 `<el-dialog v-model="showFormulaDialog">`。

- [ ] **Step 2: 删除 script 中的公式函数和 state**

  删除: `formulas`, `showFormulaDialog`, `editingFormula`, `isEditingFormula`, `loadingFormulas`, `savingFormula` 等 ref。
  删除: `fetchFormulas`, `openAddFormula`, `openEditFormula`, `handleSaveFormula`, `handleDeleteFormula`, `handleFormulaToggle` 等函数。
  删除: `loadAll` 中的 `fetchFormulas()` 调用。
  删除: import 中的 `getMicrogridFormulas`, `addMicrogridFormula`, `updateMicrogridFormula`, `deleteMicrogridFormula`, `MicrogridFormula`。

- [ ] **Step 3: 构建验证**

  ```bash
  cd web && npm run build
  ```

### Task 6: 更新 autoFormulas 公式预览

- [ ] **Step 1: 更新 GRID_P 公式文字**

  当前:
  ```ts
  result.push({ label: '关口表功率 (GRID_P)', expr: `(${loadExpr}) - (${genExpr})` })
  ```

  改为：
  ```ts
  const batExpr = activeArr(bats).map(mkRef).join(' + ') || '0'  // charging=+, discharge=-
  result.push({ label: '关口表功率 (GRID_P)', expr: `(${loadExpr}) + (${batExpr}) - (${pvExpr})` })

  // 下加注释行
  result.push({ label: '  └ 符号说明', expr: '储能>0=充电, <0=放电 | 关口>0=用电, <0=送电' })
  ```

- [ ] **Step 2: 构建验证**

### Task 7: 最终集成测试 + 打包

- [ ] **Step 1: 运行全量测试**

  ```bash
  go test ./... && cd web && npm run build
  ```

- [ ] **Step 2: 启动服务集成测试**
  ```bash
  ./bin/gridsim serve --http :18999 --config-dir /tmp/gridsim_final
  # 创建实例、配置拓扑、启动、观察仪表盘数据
  ```

- [ ] **Step 3: 打包**

  ```bash
  make dist
  ```

- [ ] **Step 4: 提交**

  ```bash
  git add -A && git commit -m "refactor(microgrid): v2 功率方向统一 + 动态拓扑图 + SOC复用"
  git push
  ```

---

## 风险评估

| 风险 | 缓解措施 |
|------|---------|
| 电池功率符号翻转可能破坏现有关口表公式逻辑 | Task 2 中同步重写 calcPowerBalanceLocked 和 Dashboard |
| formula_test.go 中的期望值可能需更新 | Task 2 Step 4 中运行测试并根据新规约修正期望值 |
| SVG 动态高度可能影响布局 | 使用 overflow:auto 容器，最小高度保底 |
| 删除公式配置 Tab 可能丢失已有公式数据 | 后端 CRUD API 和 store 中 `topology.Formulas` 保留，只是前端不展示 |
