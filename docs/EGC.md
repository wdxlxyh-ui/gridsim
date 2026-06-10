# EGC 算法日志与配置文件说明

> 来源: `/mnt/d/工具/EGC业务分享/EGC算法日志与配置文件说明.md`
> 日志示例: `/mnt/d/工具/EGC业务分享/日志示例.txt` (INFO+DEBUG)、`日志示例 - INFO.txt` (仅INFO)
> 相关文档: [[microgrid-controller-domain]] · [[EnFlex_EGC设备模型信息]]

本文档汇总 EGC（Energy Generation Controller / 微网控制器）的算法日志解读方法和主要配置文件的含义，适用于 microgridcontroller 等算法的现场调试、问题定位与测试用例编写。

---

## 一、算法日志详解

下面以 `microgridcontroller` 算法**一个完整控制周期**的日志为例，挑选关键日志逐条说明。

> 📋 完整日志样例: `/mnt/d/工具/EGC业务分享/日志示例.txt`（INFO+DEBUG，79行）、`日志示例 - INFO.txt`（仅INFO，38行）

### 1. 输入快照（每周期第1行）

每个周期日志的第一行，是算法按 `app.config` 中的设备顺序，把所读取到的值依次打印出来。

- **顺序**：与 `app.config` 中设备配置顺序一致。
- **分隔规则**：
  - 不同设备类型之间用 `#` 分隔；
  - 同一设备类型下的多台设备之间不使用 `#` 分隔，依次拼接。
- **每个值的后缀含义**：

| 形式 | 含义 |
|------|------|
| `-1N` | 算法没有读到值。可能是属性未配置，或对应测点没有采集。 |
| `xxY` | 算法正确读到了值，且值有效（`Y` = yes，未过期）。算法判断的数据过期时间可配置，**默认 15 分钟**。 |
| `xxN` | 算法**曾经**读到过该值，但已超过过期时间未刷新，后缀为 `N`，表示已过期。 |

**日志示例（节选）**：
```text
[2025-09-24 16:05:21.897] [INFO ]:1Y 1600.000000Y -1N -1N -1N 1400.000000Y ...
```

可以看到 `Y`、`N`、`-1N`、`#` 等标记，按照 `app.config` 中 `INPUT` 的顺序对应每一项采集值。

### 2. 设备类型聚合结果

第二行是算法把第一行采集到的实时数据按**设备类型聚合**后的结果：

| 字段 | 含义 |
|------|------|
| `DG` | 柴发总功率 |
| `PV` | 光伏总功率 |
| `EV` | 充电桩总功率 |
| `BESS` | 储能总功率 |
| `LOAD` | 负载（计算值） |
| `other_meter_active_power` | 其他电表有功功率 |

**LOAD 计算公式**：
```
LOAD = Grid PW + INV PW + DG PW − ESS PW − EVC PW
```
含义：负载功率 = 关口表功率 + 光伏功率 + 柴发功率 − 储能功率 − 充电桩功率。

**日志示例**：
```text
[2025-09-24 16:05:21.901] [INFO ]:DG:0.000000 PV:0.000000 EV:0.000000 BESS:0.000000  LOAD:1600.000000 ...
```

### 3. 优化的功率约束：Pmax 与 Pmin

优化器使用的**约束条件**：

| 参数 | 含义 |
|------|------|
| `Pmax` | 最大用电功率（系统允许的最大输入功率） |
| `Pmin` | 最大上网功率（系统允许的最大输出功率，通常为负） |

这两个值是算法做线性优化时使用的功率上下限。

**日志示例**：
```text
[2025-09-24 16:05:21.901] [INFO ]:[loop]Pmax:2000.000000,Pmin:-10000000.000000
```

### 4. `xxx not active` 的含义

日志中经常出现 `xxx not active`，例如：
```text
DG not active
PV not active
EV hub not active
```

判断规则：
1. 如果 `app.config` 中**不存在**对应的设备类型，会直接打印该日志。
2. 如果**存在**该设备类型，算法会进一步判断设备的可控状态；**不可控**时同样会打印该日志。

> 💡 测试时可通过这条日志反向核对：测试用例构造的输入条件是否符合预期，设备状态是否正常。

### 5. 优化结果：各电源/负载的目标功率

经过约束判断后，算法输出**初步优化结果**：

| 字段 | 含义 | 备注 |
|------|------|------|
| `P_bess_dch` | 储能放电目标功率 | 该值 ≤ 0 |
| `P_bess_ch` | 储能充电目标功率 | 0 ≤ 该值 |
| `P_bess` | 储能合并后的目标功率 | 正充负放 |
| `P_pv` | 光伏目标功率 | — |
| `P_dg_all` | 柴发目标功率 | — |
| `P_ev` | 充电枪目标功率 | — |
| `P_im` | 当前微网预计需要输入的功率 | — |
| `P_ex` | 当前微网预计需要输出的功率 | — |
| `P_grid_import` | 按当前优化结果执行后，预计的关口表输入功率 | 一般为正值 |
| `P_grid_export` | 按当前优化结果执行后，预计的关口表输出功率 | 一般为负值 |

**日志示例**：
```text
[2025-09-24 16:05:21.907] [INFO ]:p_bess_dch =0.000000  p_bess_ch = 0.000000 p_bess =0.000000 p_pv = 0.000000 p_dg_all = 0.000000 p_ev =0.000000 p_im = 0.000000 p_ex = 99.908081  p_grid_import = 1500.091919 p_grid_export = 0.000000 ...
```

### 6. 控制下发结果（周期末）

周期末，算法把根据优化结果计算出的**实际控制目标**打印出来——这些就是真正下发给设备的指令。

**格式**：
```text
deviceid:<设备 id>  pointid:<测点标识符>  value:<下发的值>
```

**日志示例**：
```text
deviceid:EDgM2Efp pointid:md_soc                              value:0.980000
deviceid:EDgM2Efp pointid:available_soc                      value:0.950000
deviceid:EDgM2Efp pointid:dp_month_rt_value_calc            value:1551.260498
deviceid:JbWz7ivN pointid:BS.SysAPSetPoint                   value:0.000000
deviceid:EDgM2Efp pointid:ap_imp_uplimit_value               value:2000.000000
deviceid:EDgM2Efp pointid:opt_status                          value:536870912.000000
deviceid:EDgM2Efp pointid:egc_status                          value:0.000000
deviceid:EDgM2Efp pointid:egc_uptime                          value:237.000000
```

### 7. 完整周期日志阅读路线

按上面 6 个要点对照日志，一个周期通常按这样的顺序展开：

| 阶段 | 日志特征 | 说明 |
|------|---------|------|
| ① 输入快照 | 长串 `xxY`/`-1N`值 | 看采集是否正常、是否过期 |
| ② 设备聚合+模式 | `DG/PV/EV/BESS/LOAD`、`current mode is TOU` | 聚合并判断运行模式 |
| ③ 约束条件 | `Pmax`/`Pmin`、`transform import power is larger...` | 约束触发说明 |
| ④ 设备激活 | `DG not active`/`PV not active`/`EV hub not active` | 设备不可控/不存在 |
| ⑤ 优化求解 | `Number of variables=27`、`Optimal objective value=...`、`p_grid`等 | 线性规划求解结果 |
| ⑥ 控制下发 | `deviceid:xxx pointid:yyy value:zzz` | 最终下发给设备的指令 |

> 💡 调试技巧：先看 INFO 级别的日志（流程主线），DEBUG 级别用于深入定位（如 `apc_lower_limit_grid`、`branch grid import/export`、`Optimal objective value` 等）。

---

## 二、EGC 配置文件解析

EGC 的配置文件是算法运行的基础，**决定了运行哪个算法**以及**算法运行时使用哪些参数**。

主要涉及以下文件：

| 文件 | 用途 | 常见程度 |
|------|------|:------:|
| `config.xml` | 决定算法加载/周期/启停 | ⭐⭐⭐ |
| `app.config` | 算法核心运行配置（采集来源/控制目标） | ⭐⭐⭐ |
| `group.config` | 并网表与回路对应关系 | ⭐⭐ |
| `microgridcontroller.config` | 算法参数配置（key=value） | ⭐⭐ |
| `config.config` | 控制下发前置依赖声明 | ⭐ |
| `template.json` | HMI 判断 POINTS 是否需要修改 | ⭐ |

---

### 2.1 `config.xml`

决定有哪些算法被加载、运行周期是多少、是否启用等。

#### controller 与 subcontroller

- 一个 `config.xml` 中可以有**多个 `controller`**，同一个 `controller` 共用一组 `period` 和 `enable`。
- 一个 `controller` 中可以有**多个 `subcontroller`**。
- **执行关系**：
  - **多个 `controller` 之间**：并行执行。
  - **同一个 `controller` 内的 `subcontroller`**：按顺序串行执行。

#### `period`

- 控制周期，单位秒。例如 `period = 5` 表示**每 5 秒执行一次算法**。
- HMI 控制策略页面输入的控制周期会被解析后写入此字段。

#### `enable`

- `true`：算法按 `period` 循环运行。
- `false`：算法不运行。

#### `lib` / `configFilePath` / `pointFilePath`

- 三者均使用相对于 `bin` 目录的路径。
- 表示算法用到的算法库与配置文件位置。

#### 关键参数

| 参数 | 含义 |
|------|------|
| `ThresholdValue` | 控制阈值。算法将本周期计算结果与上周期对比，**控制指令差值在阈值范围内时不会下发**，避免无效抖动。 |
| `ThresholdTime` | 阈值时间，单位**秒**。即使数据未发生明显变化，**超过该时间也会强制触发一次下发**，保证下游设备拿到最新指令。 |

> ⚠️ `config.xml` 相关部署陷阱：见 [[edge-ems-deployment skill]] § config.xml pitfall。

---

### 2.2 `app.config`

算法核心运行配置，用于声明计算方式、采集来源、控制目标等。

#### `Calc` 部分

##### `GridMeter_Calc_Type`

判断并网表的进线方式：

| 值 | 含义 | 子配置 |
|:--:|------|------|
| `1` | 单进线多电表 | 解析 `MultiMeters`：`VirtualMeter` + `CalcMeters` 数组 |
| `2` | 双进线 | 解析 `MultiInputs`：`VirtualMeter` + `Devices` 数组（含 Meter/Switcher/Collect） |
| 空 | 未配置 | — |

##### `FrequencyMeter`

频率表的设备信息。**启用 FCR（一次调频）频率控制**时需要选择对应的电表。

##### `TriggerPoints`

`point` 中写入触发点的测点标识符。**当测点值发生变化或更新时，算法会立即触发一次执行，而不再等待 `period` 配置的周期**。

#### `INPUT` / `OUTPUT`

- **`INPUT`**：数组，每个元素包含 `DEVICE` 和 `POINTS`。算法按这些信息读取实际值，作为优化的输入。
- **`OUTPUT`**：数组，结构与 `INPUT` 一致。算法优化的结果按此配置控制到对应设备的指定测点。

#### `POINTS` 中标识符后缀含义

| 形式 | 含义 | 示例 |
|------|------|------|
| 无后缀 | 普通**测点标识符** | 有功功率、可控状态 |
| `##2` | **属性标识符** | 额定容量、额定功率 |
| `##6` | 特殊值（当前仅 `timezone`） | 时区 |
| `##7` | 前置算法→后置算法的**变量传递** | `powerCurtailmenAlarm` |
| `##8` | 对应设备的 **redis key** | `hgetall ems:hmi2ems.configurations`，用于充电桩优先级/储能自定义计划 |

---

### 2.3 `group.config`

用于描述并网表与各回路的对应关系。

| 参数 | 含义 |
|------|------|
| `gridmeter` | 并网表的信息 |
| `group` | 回路数组 |
| `group[].calcType` | 箱变有功功率获取方式：`0`=电表上报，`1`=箱变上报，`2`=累计计算 |
| `group[].calcDevices` | **仅 `calcType=2` 时使用**，参与累计计算的设备信息 |
| `group[].Input` | 当前回路内的设备信息 |

---

### 2.4 `microgridcontroller.config`

微网控制器算法使用的参数配置。

- 格式为 `key=value`。
- 由于参数数量较多，配置文件中**基本都有英文注释**，可直接对照查看。

---

### 2.5 `config.config`

每个算法都会有，但**不常用、不常修改**。主要用于声明部分控制下发的前置依赖。

**示例**：
```
PV#INV.LimitPower@INV.OemState,1:0
```

含义：**算法在对光伏设备的 `INV.LimitPower` 测点下发控制之前，需要先对 `INV.OemState` 测点下发控制指令，值为 `1`**。

格式：`设备类型#目标测点@前置测点,前置值:默认值`

---

### 2.6 `template.json`

包含 `input` 和 `output` 两部分，**供 HMI（edge-ems-hmi）根据对应设备类型的模型，判断是否需要修改 `app.config` 中的 `points`**。

---

## 三、调试速查表

| 场景 | 应该看哪条日志 / 配置 |
|------|----------------------|
| 想确认算法是否在跑 | `config.xml` 的 `enable`、`period` |
| 测点没数据 | 第 1 行输入快照中是否为 `-1N`；`app.config` 的 `INPUT` 是否漏配 |
| 数据不刷新 | 第 1 行输入快照中是否为 `xxN`，确认采集链路 |
| 设备一直 `not active` | `app.config` 中是否配置了该设备；设备的可控状态测点 |
| 算法没下发控制 | 控制结果段是否打印；`config.xml` 的 `ThresholdValue`/`ThresholdTime` 是否过滤 |
| 优化结果异常 | 检查 `Pmax`/`Pmin`、约束日志（`branch grid import/export`、`apc_*`）及 `Optimal objective value` |
| 关口表功率方式错 | `app.config` 的 `GridMeter_Calc_Type` |
| FCR 频率控制不生效 | `app.config` 的 `FrequencyMeter` 是否配置 |
| 充电桩优先级 / 储能自定义计划没生效 | `POINTS` 中 `##8` 后缀；redis: `hgetall ems:hmi2ems.configurations` |
| 时区问题 | `POINTS` 中 `##6` 后缀的 `timezone` |
| 控制前置依赖（如光伏限功率） | `config.config` 中 `@` 前置写测点的配置 |

---

## 四、相关引用

| 文档 | 位置 |
|------|------|
| 领域知识（系统架构/算法策略） | [[microgrid-controller-domain]] (EGC产品/microgrid-tester/knowledge-base/) |
| 设备模型信息（测点/属性清单） | [[EnFlex_EGC设备模型信息]] |
| 日志示例（INFO+DEBUG 完整版） | `/mnt/d/工具/EGC业务分享/日志示例.txt` |
| 日志示例（仅INFO） | `/mnt/d/工具/EGC业务分享/日志示例 - INFO.txt` |
| 部署与 config.xml 陷阱 | [[edge-ems-deployment skill]] |
| EGC 测试团队 | [[egc-testing-team skill]] |
