# EnFlex/EGC 设备模型信息库

> 来源: `/mnt/d/工具/SIL模拟器/EGC模型信息/` (14个Excel文件)
> 模型版本: v2.4.9 ~ v2.4.11
> 内容: 模型标识符、测点、属性的完整记录，供 EGC 测试和开发参考

---

## 模型总览

| # | 模型标识符 (Model ID) | 中文名 | 英文名 | 资产类型 | 属性数 | 测点数 |
|---|---|---|---|---|---|---|
| 1 | `EnOS_Breaker` |  |  |  | 0 | 1 |
| 2 | `EnOS_EnFlex_Ferroalloy_Furnace` | 矿热炉 | Ferroalloy Furnace |  | 1 | 3 |
| 3 | `EnOS_EnFlex_Gas_Storage` |  |  |  | 21 | 12 |
| 4 | `EnOS_EnFlex_Gas_Turbine` | 尾气发电机柜 |  |  | 9 | 6 |
| 5 | `EnOS_EnFlex_WT_Generic` | 风机通用子模型(EnFlex) | 风机通用子模型(EnFlex) |  | 1 | 4 |
| 6 | `EnOS_Microgrid_Controller` | 微网控制器 | EnOS Microgrid Controller |  | 2 | 9 |
| 7 | `EnOS_RE_CONN` | 充电枪(户用) | 充电枪(户用) |  | 11 | 22 |
| 8 | `EnOS_RE_EV` | 充电桩(户用) | 充电桩(户用) |  | 3 | 0 |
| 9 | `EnOS_RE_WS` | 气象站(户用) | WeatherStation (Residential) |  | 0 | 2 |
| 10 | `EnOS_Solar_BoxSubstation` |  |  |  | 1 | 4 |
| 11 | `EnOS_Solar_METER_Generic` | 光伏电表(Generic) | Solar Meter (Generic) |  | 9 | 17 |
| 12 | `EnOS_Solar_RE_BS` | 储能电池(户用) | Battery Storage (Residential) |  | 14 | 21 |
| 13 | `EnOS_Solar_RE_INV` | 逆变器(户用) | Inverter (Residential) |  | 9 | 28 |
| 14 | `EnOS_Solar_RE_SITE` | 光伏场站(户用) | Solar Site (Residential) |  | 31 | 3 |

---

## EnOS_Breaker

- **文件**: `EnOS_Breaker_v2.4.9.xlsx`
- **中文名**: 
- **英文名**: 

### 测点 (Measurement Points)

| 标识符 | 中文名 | 英文名 | 四遥类型 | EGC采集间隔 | EGC依赖 | EGC含义 |
|---|---|---|---|---|---|---|
| `XCBR.StateClosing` | 合位 | State Closing | DI | 500 毫秒/500ms | 可选/Optional | 双进线控制： 1:合 0:分 |

---

## EnOS_EnFlex_Ferroalloy_Furnace

- **文件**: `EnOS_EnFlex_Ferroalloy_Furnace_v2.4.11.xlsx`
- **中文名**: 矿热炉
- **英文名**: Ferroalloy Furnace

### 属性 (Attributes)

| 标识符 | 中文名 | 英文名 |
|---|---|---|
| `k_value` | 热值常量 |  |

### 测点 (Measurement Points)

| 标识符 | 中文名 | 英文名 | 四遥类型 | EGC采集间隔 | EGC依赖 | EGC含义 |
|---|---|---|---|---|---|---|
| `FF.LoadDistributionPW` | 负荷分配功率 | Load Power Distribution | AI |  |  |  |
| `FF.ActivePW` | 实时功率 | Active Power | AI |  |  |  |
| `FF.ConsumedKWH` | 累计耗电量 | Cumulative Energy Consumption | AI |  |  |  |

---

## EnOS_EnFlex_Gas_Storage

- **文件**: `EnOS_EnFlex_Gas_Storage_v2.4.11.xlsx`
- **中文名**: 
- **英文名**: 

### 属性 (Attributes)

| 标识符 | 中文名 | 英文名 |
|---|---|---|
| `totalHolderVolume` | 气柜总标方 | Total Gas Holder Volume |
| `holderHeight` | 气柜高度 | Gas Holder Height |
| `socL` | Soc下限 | SOC-L |
| `coEmpHeatVol` | CO经验热值 | CO Empirical Heating Value |
| `coEmpVolFract` | CO经验含量 | CO Empirical Volume Fraction |
| `h2EmpHeatVol` | H2经验热值 | H2 Empirical Heating Value |
| `h2EmpVolFract` | H2经验含量 | H2 Empirical Volume Fraction |
| `ratiokgtokJ` | kJ/kg 转换 | Convertion ratio from KJ to Kg |
| `socEndMin` | 优化期结束SOC最小值 | Min end SOC |
| `socEndMax` | 优化期结束SOC最大值 | Max end SOC |
| `socH` | SOC-H | SOC-H |
| `hoderHeightAscRateLimit` | 气柜高度上升爬坡率限值 | Gas Holder Height Ascend Ramp Rate Limit |
| `hoderHeightDesRateLimit` | 气柜高度下降滑坡率限值 | Gas Holder Height Descend Slope Rate Limit |
| `socLLL` | SOC-LLL | SOC-LLL |
| `socLL` | SOC-LL | SOC-LL |
| `socL` | SOC-L | SOC-L |
| `socH` | SOC-H | SOC-H |
| `socHH` | SOC-HH | SOC-HH |
| `socHHH` | SOC-HHH | SOC-HHH |
| `gasHolderBaseArea` | 底面积 | Gas Holder Base Area |
| `opToStdConvCoeff` | 工况转标况的系数 | Operating-to-Standard Conversion Coefficient |

### 测点 (Measurement Points)

| 标识符 | 中文名 | 英文名 | 四遥类型 | EGC采集间隔 | EGC依赖 | EGC含义 |
|---|---|---|---|---|---|---|
| `GS.CoHeatingVal` | CO实时热值 | CO Heating Value |  |  |  |  |
| `GS.CoVolFract` | CO实时含量 | CO Volume Fraction |  |  |  |  |
| `GS.H2HeatingVal` | H2实时热值 | H2 Heating Value |  |  |  |  |
| `GS.H2VolFract` | H2实时含量 | H2 Volume Fraction |  |  |  |  |
| `GS.IntakeRate` | 进气量 | Gas Inflow Rate |  |  |  |  |
| `GS.O2VolFract` | O2实时含量 | O2 Volume Fraction |  |  |  |  |
| `GS.OutputRate` | 出气量 | Gas Output Rate |  |  |  |  |
| `GS.TotalIntakeVol` | 进气量累计 | Total Gas Intake Volume |  |  |  |  |
| `GS.TotalOutputVol` | 出气量累计 | Gas Output Volume |  |  |  |  |
| `GS.Soc` | SOC | SOC |  |  |  |  |
| `GS.HolderLevel` | 实时柜位高度 | Gas Holder Level |  |  |  |  |
| `GS.GasHolderLiftRate` | 气体升降速度 | Gas Holder Lift Rate |  |  |  |  |

---

## EnOS_EnFlex_Gas_Turbine

- **文件**: `EnOS_EnFlex_Gas_Turbine_v2.4.11.xlsx`
- **中文名**: 尾气发电机柜
- **英文名**: 

### 属性 (Attributes)

| 标识符 | 中文名 | 英文名 |
|---|---|---|
| `ratedPower` | 额定功率 | Rated Power |
| `minPowerLimit` | 最小功率限制 | Minimum Power Limit |
| `maxPowerLimit` | 最大功率限制 | Maximum Power Limit |
| `rampUpLimitWinter` | 上升爬坡率限制-冬季 | Ramp Up Limit - Winter |
| `rampUpLimitSummer` | 上升爬坡率限制-夏季 | Ramp Up Limit - Summer |
| `rampDownLimit` | 下降滑坡率限制 | Ramp Down Limit |
| `minShutdownDuration` | 最短停机时长 | Minimum Shutdown Duration |
| `minStartupDuration` | 最短开机时长 | Minimum Startup Duration |
| `ratedHeatCons` | 额定热耗 | Rated Heat Consumption |

### 测点 (Measurement Points)

| 标识符 | 中文名 | 英文名 | 四遥类型 | EGC采集间隔 | EGC依赖 | EGC含义 |
|---|---|---|---|---|---|---|
| `GT.GenActivePW` | 发电有功功率 | Active Power |  |  |  |  |
| `GT.ProductionTD` | 当日发电量 | Production Today |  |  |  |  |
| `GT.ProductionMTD` | 当月发电量 | Production This Month |  |  |  |  |
| `GT.CtrlState` | 控制状态 | Controllale State |  |  |  |  |
| `GT.OemState` | 采集状态 | Upload State |  |  |  |  |
| `GT.RealTimeDispatchCap` | 实时可调节能力 | Real-Time Dispatchable Capcity |  |  |  |  |

---

## EnOS_EnFlex_WT_Generic

- **文件**: `EnOS_EnFlex_WT_Generic_v2.4.11.xlsx`
- **中文名**: 风机通用子模型(EnFlex)
- **英文名**: 风机通用子模型(EnFlex)

### 属性 (Attributes)

| 标识符 | 中文名 | 英文名 |
|---|---|---|
| `ratedPower` | 额定容量 | Rated Power |

### 测点 (Measurement Points)

| 标识符 | 中文名 | 英文名 | 四遥类型 | EGC采集间隔 | EGC依赖 | EGC含义 |
|---|---|---|---|---|---|---|
| `WGEN.GenActivePW` | 发电机有功功率 | Active Power |  |  |  |  |
| `WNAC.TheoryActivePW` | 理论功率 | Theory Active Power |  |  |  |  |
| `WTUR.AllConnectionSts` | 风机通讯状态 | Turbine ALL Conn State |  |  |  |  |
| `PUB_WT.APProductionKWH` | 有功发电量总计 |  |  |  |  |  |

---

## EnOS_Microgrid_Controller

- **文件**: `EnOS_Microgrid_Controller_v2.4.11.xlsx`
- **中文名**: 微网控制器
- **英文名**: EnOS Microgrid Controller

### 属性 (Attributes)

| 标识符 | 中文名 | 英文名 |
|---|---|---|
| `fcr_controllable_point` |  |  |
| `control_conversion_rules` | 控制转换规则 | Control Conversion Rules |

### 测点 (Measurement Points)

| 标识符 | 中文名 | 英文名 | 四遥类型 | EGC采集间隔 | EGC依赖 | EGC含义 |
|---|---|---|---|---|---|---|
| `DR.LoadForecastReport` | 负荷预测数据 |  |  |  |  |  |
| `DR.LoadDataReport` | 负荷实时数据 |  |  |  |  |  |
| `DR.EventRequest` | DR事件请求 |  |  |  |  |  |
| `DR.EventResponse` | DR事件响应 |  |  |  |  |  |
| `DR.Energy` | DR电量 |  |  |  |  |  |
| `DR.State` | DR执行状态 |  |  |  |  |  |
| `DR.EventList` | DR事件列表 |  |  |  |  |  |
| `SITE.State` | 场站状态 |  |  |  |  |  |
| `MGC.state` |  |  |  |  |  |  |

---

## EnOS_RE_CONN

- **文件**: `EnOS_RE_CONN_v2.4.11.xlsx`
- **中文名**: 充电枪(户用)
- **英文名**: 充电枪(户用)

### 属性 (Attributes)

| 标识符 | 中文名 | 英文名 |
|---|---|---|
| `PUB_CONN.displayOrder` | 展示顺序 | Display Order |
| `PUB_CONN.topoParentDeviceID` | 父节点设备ID | Parent Device ID |
| `PUB_CONN.MinChargePW` | 最小充电功率 | Min Charging Power |
| `PUB_CONN.RatedPW` | 额定功率 | Rated Power |
| `PUB_CONN.MinChargeCurrent` | 最小充电电流 |  |
| `PUB_CONN.MaxChargeCurrent` | 最大充电电流 |  |
| `PUB_CONN.ControlGroup` | 控制分组 | Control Group |
| `PUB_CONN.EnablePhaseSwitching` | 启用相位切换 | Enable Phase Switching |
| `PUB_CONN.PhaseMapL1` | L1对应电网相 | L1 to Grid Phase Mapping |
| `PUB_CONN.PhaseMapL2` | L2 对应电网相 | L2 to Grid Phase Mapping |
| `PUB_CONN.PhaseMapL3` | L3 对应电网相 | L3 to Grid Phase Mapping |

### 测点 (Measurement Points)

| 标识符 | 中文名 | 英文名 | 四遥类型 | EGC采集间隔 | EGC依赖 | EGC含义 |
|---|---|---|---|---|---|---|
| `PUB_CONN.ChargePWSet` | 功率设置 | Charging Power Setting | 遥调点/AO |  |  | 交流充电桩控制功能 |
| `PUB_CONN.OemState` | 设备上报状态 | Status | 计算点 EGC 自动计算/Calculation Point |  |  | EGC 本地监视展示 |
| `PUB_CONN.Current` | 充电电流 | Charge Current | 遥测点/AI |  |  |  |
| `PUB_CONN.Voltage` | 充电电压 | Charge Voltage | 遥测点/AI |  |  |  |
| `PUB_CONN.ChargePW` | 充电功率 | Charging Power | 遥测点/AI | 500 毫秒/500ms |  | 充电桩控制功能 |
| `PUB_CONN.CtrlState` | 可控状态 | Control State | 计算点/Calculation Point
(配置Edge 边缘计算) | 500 毫秒/500ms |  | 充电桩控制功能 |
| `PUB_CONN.ChargeCurSetL1` | 充电电流设值L1 | Charging Current Setting L1 | 遥调点/AO |  |  | 直流充电桩控制功能 |
| `PUB_CONN.ChargeEnergyKWH` | 设备上报累计充电表读数 | Total Charged Energy Reading | 电度点/PI | 1 分钟/1min |  | EGC 本地监视展示 |
| `PUB_CONN.OrderNum` | 订单计数 | Order Numbers | 遥调点/DI |  |  |  |
| `PUB_CONN.OrderElec` | 订单充电电量 | Order Charged Energy | 遥测点/AI |  |  |  |
| `PUB_CONN.OrderElecPrice` | 订单电费 | Order Electricity Price | 遥测点/AI |  |  |  |
| `PUB_CONN.OrderServicePrice` | 订单服务费 | Order Service Price | 遥测点/AI |  |  |  |
| `PUB_CONN.CurrentL1` | 电流L1 | Charging Current L1 | 遥测点/AI | 500 毫秒/500ms |  | 充电桩控制功能 |
| `PUB_CONN.CurrentL2` | 电流L2 | Charging Current L2 | 遥测点/AI | 500 毫秒/500ms |  | 充电桩控制功能 |
| `PUB_CONN.CurrentL3` | 电流L3 | Charging Current L3 | 遥测点/AI | 500 毫秒/500ms |  | 充电桩控制功能 |
| `PUB_CONN.ChargeVolL1` | 充电电压L1 | Charging Voltage L1 | 遥测点/AI |  |  |  |
| `PUB_CONN.ChargeVolL2` | 充电电压L2 | Charging Voltage L2 | 遥测点/AI |  |  |  |
| `PUB_CONN.ChargeVolL3` | 充电电压L3 | Charging Voltage L3 | 遥测点/AI |  |  |  |
| `PUB_CONN.Health` | 健康状态 | Health State | 遥信点/DI | 500 毫秒/500ms |  | 充电桩控制功能 |
| `PUB_CONN.State` | 充电枪状态 | Connector State | 计算点/Calculation Point | NA |  | EGC 本地监视展示 |
| `PUB_CONN.ChargingPhase` | 充电相位 | Charging Phase | 遥测点/AI | 500 毫秒/500ms |  | 充电桩控制功能 |
| `PUB_CONN.SwitchChargingPhase` | 切换充电相位 | Switch Charging Phase | 遥调点/DI | NA |  | 充电桩控制功能 |

---

## EnOS_RE_EV

- **文件**: `EnOS_RE_EV_v2.4.11.xlsx`
- **中文名**: 充电桩(户用)
- **英文名**: 充电桩(户用)

### 属性 (Attributes)

| 标识符 | 中文名 | 英文名 |
|---|---|---|
| `modelName` | 型号 | Charger Model Name |
| `name` | 名称 | Name |
| `PUB_EV.manufacturerName` | 生产商 | Manufacturer Name |

---

## EnOS_RE_WS

- **文件**: `EnOS_RE_WS_v2.4.11.xlsx`
- **中文名**: 气象站(户用)
- **英文名**: WeatherStation (Residential)

### 测点 (Measurement Points)

| 标识符 | 中文名 | 英文名 | 四遥类型 | EGC采集间隔 | EGC依赖 | EGC含义 |
|---|---|---|---|---|---|---|
| `WST.Radiation` | 倾角辐照强度 | Plane of Array Irradiance |  |  |  |  |
| `WST.Temperature` | 环境温度 | Ambient temperature |  |  |  |  |

---

## EnOS_Solar_BoxSubstation

- **文件**: `EnOS_Solar_BoxSubstation_v2.4.9.xlsx`
- **中文名**: 
- **英文名**: 

### 属性 (Attributes)

| 标识符 | 中文名 | 英文名 |
|---|---|---|
| `capacity_rated` | 额定容量（kVA） | Rated Capacity(kVA) |

### 测点 (Measurement Points)

| 标识符 | 中文名 | 英文名 | 四遥类型 | EGC采集间隔 | EGC依赖 | EGC含义 |
|---|---|---|---|---|---|---|
| `BXTF.ActPowPhBr2` | 1号低压侧有功功率 | Low Voltage Side 1 Active Power | 遥测点/AI | 500 毫秒/500ms | 必须/Required | 箱变防超容功能：使用箱变有功功率判断是否超容 |
| `BXTF.IaBr2` | 1号低压侧Ia | Low Voltage Side 1 Ia | 遥信点/DI | 500 毫秒/500ms | 可选/Optional | 箱变防超容自动切换 |
| `BXTF.IbBr2` | 1号低压侧Ib | Low Voltage Side 1 Ib | 遥信点/DI | 500 毫秒/500ms | 可选/Optional |  |
| `BXTF.IbBr2` | 1号低压侧Ic | Low Voltage Side 1 Ic | 遥信点/DI | 500 毫秒/500ms | 可选/Optional |  |

---

## EnOS_Solar_METER_Generic

- **文件**: `EnOS_Solar_METER_Generic_v2.4.11.xlsx`
- **中文名**: 光伏电表(Generic)
- **英文名**: Solar Meter (Generic)

### 属性 (Attributes)

| 标识符 | 中文名 | 英文名 |
|---|---|---|
| `slope` | 正斜率 | Max Ramp Threshold |
| `scale` | 倍率 | Scale |
| `displayOrder` | 展示顺序 | Display Order |
| `version` | 软件版本号 | version |
| `sn` | 序列号 | SN |
| `modelName` | 机型名称 | Model Name |
| `topoParentDeviceID` | 父节点设备ID | Parent Device ID |
| `meterType` | 电表类型 | Meter Type |
| `primaryID` | 主表ID | Primary ID |

### 测点 (Measurement Points)

| 标识符 | 中文名 | 英文名 | 四遥类型 | EGC采集间隔 | EGC依赖 | EGC含义 |
|---|---|---|---|---|---|---|
| `METER.APProductionKWH` | 表读数-总-正向有功 | Meter Read Active Energy Exported-Total | 遥测点/AI |  |  |  |
| `METER.APConsumedKWH` | 表读数-总-反向有功 | Meter Read Active Energy Imported-Total | 遥测点/AI |  |  |  |
| `METER.ActivePW` | 有功功率 | Active Power | 遥测点/AI | 500 毫秒/500ms |  |  |
| `METER.OemState` | 设备上报状态 | Oem State | 遥调点/DI |  |  |  |
| `METER.MaxDemandPW` | 当月最大需量 | Max Demand Power | 遥测点/AI | 500 毫秒/500ms |  |  |
| `METER.APProductionKWH1` | 表读数-尖-正向有功 | Meter Read Active Energy Emported-Sharp | 遥测点/AI |  |  |  |
| `METER.APConsumedKWH1` | 表读数-尖-反向有功 | Meter Read Active Energy Imported-Sharp | 遥测点/AI |  |  |  |
| `METER.APProductionKWH2` | 表读数-峰-正向有功 | Meter Read Active Energy Emported-Peak | 遥测点/AI |  |  |  |
| `METER.APConsumedKWH2` | 表读数-峰-反向有功 | Meter Read Active Energy Imported-Peak | 遥测点/AI |  |  |  |
| `METER.APProductionKWH3` | 表读数-平-正向有功 | Meter Read Active Energy Emported-Flat | 遥测点/AI |  |  |  |
| `METER.APConsumedKWH3` | 表读数-平-反向有功 | Meter Read Active Energy Imported-Flat | 遥测点/AI |  |  |  |
| `METER.APProductionKWH4` | 表读数-谷-正向有功 | Meter Read Active Energy Emported-Valley | 遥测点/AI |  |  |  |
| `METER.APConsumedKWH4` | 表读数-谷-反向有功 | Meter Read Active Energy Imported-Valley | 遥测点/AI |  |  |  |
| `METER.CurPh1` | A相电流 | A Phase Current | 遥测点/AI | 500 毫秒/500ms |  |  |
| `METER.CurPh2` | B相电流 | B Phase Current | 遥测点/AI | 500 毫秒/500ms |  |  |
| `METER.CurPh3` | C相电流 | C Phase Current | 遥测点/AI | 500 毫秒/500ms |  |  |
| `METER.Frequency` | 频率 | Frequency | 遥测点/AI | 500 毫秒/500ms |  |  |

---

## EnOS_Solar_RE_BS

- **文件**: `EnOS_Solar_RE_BS_v2.4.11.xlsx`
- **中文名**: 储能电池(户用)
- **英文名**: Battery Storage (Residential)

### 属性 (Attributes)

| 标识符 | 中文名 | 英文名 |
|---|---|---|
| `topoParentDeviceID` | 父节点设备ID | Parent Device ID |
| `version` | 软件版本号 | version |
| `displayOrder` | 展示顺序 | Display Order |
| `sn` | 序列号 | SN |
| `storageModelName` | 机型名称 | Model Name |
| `slope` | 正斜率 | Max Ramp Threshold |
| `ratedCapacity` | 额定容量 | Rated Capacity |
| `ratedPower` | 额定功率 | Rated Power |
| `incRateChargePower` | 充放电功率上升速率 | Increase Rate |
| `decRateChargePower` | 充放电功率下降速率 | Decrease Rate |
| `socMax` | SoC上限 | SoC Upper Limit |
| `socMin` | SoC下限 | SoC Lower Limit |
| `maxChargePower` | 充电功率上限 | Charge Power Upper Limit |
| `maxDischargePower` | 放电功率上限 | Discharge Power Upper Limit |

### 测点 (Measurement Points)

| 标识符 | 中文名 | 英文名 | 四遥类型 | EGC采集间隔 | EGC依赖 | EGC含义 |
|---|---|---|---|---|---|---|
| `BS.Status` | 充放电状态 | Battery Status | 遥信点/DI |  |  |  |
| `BS.TotalChargingEng` | 设备上送累计充电量 | Total Charging Energy | 电度点/PI | 1 分钟/1min | 必须/Required | EGC 本地监视展示 |
| `BS.TotalDischargingEng` | 设备上送累计放电量 | Total Discharging Energy | 电度点/PI | 1 分钟/1min | 必须/Required | EGC 本地监视展示 |
| `BS.ChargingEngDay` | 当日充电量 | Daily Charging Energy | 遥测点/AI |  |  |  |
| `BS.DischargingEngDay` | 当日放电量 | Daily Discharging Energy | 遥测点/AI |  |  |  |
| `BS.Soc` | 电池SOC | SOC | 遥测点/AI | 500 毫秒/500ms | 必须/Required | 储能实时控制 |
| `BS.ActivePW` | 充放电功率 | Active Power | 遥测点/AI | 500 毫秒/500ms | 必须/Required | 储能实时控制 |
| `BS.OemState` | 厂家上报状态 | OemState | 遥信点/DI |  | 可选/Optional | EGC 本地监视展示 |
| `BS.Soh` | 电池SOH | SOH | 遥测点/AI | 500 毫秒/500ms | 可选/Optional | 储能实时控制 |
| `BS.CtrlState` | 可控状态 | Control State | 计算点/Calculation point
(配置Edge 边缘计算) | 500 毫秒/500ms | 必须/Required | 储能实时控制 |
| `BS.MaxChargePower` | 最大充电功率 | Max Charge Power | 遥测点/AI | 500 毫秒/500ms | 可选/Optional | 储能实时控制 |
| `BS.MaxDischargePower` | 最大放电功率 | Max Discharge Power | 遥测点/AI | 500 毫秒/500ms | 可选/Optional | 储能实时控制 |
| `BS.EndChargeSOC` | 充电截止SOC | End-of-charge SOC | 遥测点/AI | 500 毫秒/500ms | 可选/Optional | 储能实时控制 |
| `BS.EndDischargeSOC` | 放电截止SOC | End-of-discharge SOC | 遥测点/AI | 500 毫秒/500ms | 可选/Optional | 储能实时控制 |
| `BS.SysAPSetPoint` | 系统有功功率控制值 | ESS System Active Power Setting | 遥调点/AO |  | 必须/Required | 储能实时控制 |
| `BS.Start` | 远程启机 | Remote Start | 遥控点/DO | NA+L26 | 可选/Optional | EGC 本地监视展示 |
| `BS.Stop` | 远程关机 | Remote Stop | 遥控点/DO | NA | 可选/Optional | EGC 本地监视展示 |
| `BS.State` | 储能电池状态 | Battery Storage State | 计算点/calculation point | NA | 可选/Optional | EGC 本地监视展示 |
| `BS.TotalCharging` | 累计充电量 | Total Charging Production | 计算点/calculation point | NA | 可选/Optional | EGC 本地监视展示 |
| `BS.TotalDischarging` | 累计放电量 | Total Discharging Production | 计算点/calculation point | NA | 可选/Optional | EGC 本地监视展示 |
| `PUB_BS.BMSControl` | BMS控制 | BMS Control |  |  |  |  |

---

## EnOS_Solar_RE_INV

- **文件**: `EnOS_Solar_RE_INV_v2.4.11.xlsx`
- **中文名**: 逆变器(户用)
- **英文名**: Inverter (Residential)

### 属性 (Attributes)

| 标识符 | 中文名 | 英文名 |
|---|---|---|
| `capacity` | 装机容量（kWp） | Installed Capacity(kWp) |
| `slope` | 正斜率 | Max Ramp Threshold |
| `scale` | 倍率 | Scale |
| `displayOrder` | 展示顺序 | Display Order |
| `topoParentDeviceID` | 父节点设备ID | Parent Device ID |
| `sn` | 序列号 | SN |
| `version` | 软件版本号 | Version |
| `modelName` | 机型名称 | Model Name |
| `control_group` | 控制分组 | Control Group |

### 测点 (Measurement Points)

| 标识符 | 中文名 | 英文名 | 四遥类型 | EGC采集间隔 | EGC依赖 | EGC含义 |
|---|---|---|---|---|---|---|
| `INV.GenActivePW` | 有功功率 | Active Power | 遥测点/AI | 500 毫秒/500ms |  | 光伏逆变器控制 |
| `INV.BranchCurIn` | 组串电流 | String Current |  |  |  |  |
| `INV.PVPowIn` | 输入功率 | DC Input Power | 遥测点/AI |  |  |  |
| `INV.InvtEffi` | 转换效率 | Inverter Efficiency | 遥测点/AI |  |  |  |
| `INV.LimitPower` | 限有功功率实际值 | Curtailment Active Power | 遥调点/DI |  |  | 光伏逆变器控制 |
| `INV.CurPh1` | A相并网电流 | A Phase Current | 遥测点/AI |  |  |  |
| `INV.CurPh2` | B相并网电流 | B Phase Current | 遥测点/AI |  |  |  |
| `INV.CurPh3` | C相并网电流 | C Phase Current | 遥测点/AI |  |  |  |
| `INV.VolPh1` | A相并网电压 | A Phase Voltage | 遥测点/AI |  |  |  |
| `INV.VolPh2` | B相并网电压 | B Phase Voltage | 遥测点/AI |  |  |  |
| `INV.VolPh3` | C相并网电压 | C Phase Voltage | 遥测点/AI |  |  |  |
| `INV.CurL1` | 线电流 L1-L2 | Line Current L1-L2 | 遥测点/AI |  |  |  |
| `INV.CurL2` | 线电流 L2-L3 | Line Current L2-L3 | 遥测点/AI |  |  |  |
| `INV.CurL3` | 线电流 L3-L1 | Line Current L3-L1 | 遥测点/AI |  |  |  |
| `INV.VolL1` | 线电压 L1-L2 | Line Voltage L1-L2 | 遥测点/AI |  |  |  |
| `INV.VolL2` | 线电压 L2-L3 | Line Voltage L2-L3 | 遥测点/AI |  |  |  |
| `INV.VolL3` | 线电压 L3-L1 | Line Voltage L3-L1 | 遥测点/AI |  |  |  |
| `INV.Start` | 远程启机 | Remote Start | 遥控点/DO |  |  | EGC 本地监视展示 |
| `INV.Stop` | 远程关机 | Remote Stop | 遥控点/DO |  |  | EGC 本地监视展示 |
| `INV.APProductionKWH` | 设备上报总累计发电量 | Total Production Reading | 电度点/PI | 1 分钟/1 min |  | EGC 本地监视展示 |
| `INV.OemState` | OEM状态 | OEM State | 遥调点/DI |  |  | EGC 本地监视展示 |
| `INV.BranchVolIn` | 组串电压 | String Voltage | 遥测点/AI |  |  |  |
| `INV.VolGrid` | 电网电压 | Grid Voltage | 遥测点/AI |  |  |  |
| `INV.CtrlState` | 可控状态 | Control State | 计算点
(配置Edge 边缘计算)/Calculation Point | 500 毫秒/500ms |  | 光伏逆变器控制 |
| `PUB_INV.AbsorbProduction` | 吸收电量 | Total Absorb Production | 遥测点/AI |  |  |  |
| `INV.State` | 逆变器状态 | Inverter State | 计算点/Calculation Point |  |  | EGC 本地监视展示 |
| `INV.APProduction` | 累计发电量 | Total Production | 计算点/Calculation Point |  |  | EGC 本地监视展示 |
| `INV.TheoryPW` | 理论功率 | Theory Power | 遥测点/AI | 500 毫秒/500ms |  | EGC Pro 根据EMS转发的光伏理论发电功率修正执行计划 |

---

## EnOS_Solar_RE_SITE

- **文件**: `EnOS_Solar_RE_SITE_v2.4.11.xlsx`
- **中文名**: 光伏场站(户用)
- **英文名**: Solar Site (Residential)

### 属性 (Attributes)

| 标识符 | 中文名 | 英文名 |
|---|---|---|
| `capacity` | 装机容量（MW） | Capacity(MW) |
| `operativeDate` | 并网日期 | Commissioning Date |
| `etlDate` | 接入日期 | Onboarding Date |
| `systemType` | 场站类型 | Site Type |
| `cityID` | 城市 | City |
| `latitude` | 纬度 | Latitude |
| `longitude` | 经度 | Longitude |
| `currency` | 货币单位 | Currency |
| `batteryCapacity` | 储能装机容量 | BESS Installed Capacity |
| `batteryCapacityRatedPW` | 储能额定功率 | BESS Rated Power |
| `topologyType` | 光储拓扑类型 | Topology Type |
| `powerDirection` | 电表功率方向 | Meter Active Power Direction |
| `storageCustomer` | 业主 | Customer |
| `storagelnvestor` | 投资方 | Investor |
| `bsPowerDirection` | 电池功率方向 | BESS Power Direction |
| `demandOptAlgorithm` | 需量优化算法 | Demand optimization algorithm |
| `area_number` | 地区编号 | area number |
| `electricityType` | 用电类型 | Electricity type |
| `billMethod` | 电费计费方式 | Billing Method |
| `voltageClass2` | 用电电压等级 | Voltage Class |
| `aimPvProdSource` | 微网PV发电量计算源 | PV Production KPI Source |
| `aimPvAcPowerSource` | 微网总PV有功功率计算源 | PV AC Power KPI Source |
| `aimBessEnergySource` | 微网储能充放电量计算源 | BESS Production KPI Source |
| `aimBessAcPowerSource` | 微网储能有功功率计算源 | BESS AC Power KPI Source |
| `aimLoadConsSource` | 微网负载用电量计算源 | Load Cons KPI Source |
| `aimLoadPowerSource` | 微网负载功率计算源 | Load Power KPI Source |
| `pvPowerDirection` | 光伏功率方向 | PV Active Power Direction |
| `windTurbineCapacity` | 风机装机容量 | Wind Turbine Installed Capacity |
| `gasTurbineRatedPW` | 尾气机组额定功率 | Gas Turbine Rated Power |
| `gasTurbineRatedHeatCons` | 尾气机组额定热耗 | Gas Turbine Rated Heat Cons |
| `ratioHeatValue` | 热值转换系数 | Convertion ratio for Heat Value |

### 测点 (Measurement Points)

| 标识符 | 中文名 | 英文名 | 四遥类型 | EGC采集间隔 | EGC依赖 | EGC含义 |
|---|---|---|---|---|---|---|
| `PUB_SITE.MaxToGridPW` | 最大上网功率限值 | Max To Grid Limit Power |  |  |  |  |
| `PUB_SITE.MaxFromGridPW` | 最大下网功率限值 | Max From Grid Limit Power |  |  |  |  |
| `PUB_SITE.DynamicDemandPwLimit` | 动态需量限值 | Dynamic Demand Power Limit |  |  |  |  |

---
