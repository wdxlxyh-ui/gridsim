package microgrid

import "encoding/json"

// ComponentType 微电网设备类型
type ComponentType string

const (
	CompPV      ComponentType = "pv"
	CompWind    ComponentType = "wind"
	CompEMS     ComponentType = "ems"
	CompBattery ComponentType = "battery"
	CompLoad    ComponentType = "load"
	CompCharger ComponentType = "charger"
)

// DeviceSwitch 设备开关
type DeviceSwitch struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Closed       bool   `json:"closed"`
	Controllable bool   `json:"controllable"`
}

// DeviceParams 各设备类型专属参数
type DeviceParams struct {
	// PV
	RatedPowerKW float64 `json:"rated_power_kw,omitempty"`
	Efficiency   float64 `json:"efficiency,omitempty"`

	// Wind (额定功率复用 RatedPowerKW; 风速单位 m/s, 零值取 3/12/25)
	CutInMS  float64 `json:"cut_in_ms,omitempty"`
	RatedMS  float64 `json:"rated_ms,omitempty"`
	CutOutMS float64 `json:"cut_out_ms,omitempty"`

	// EMS: Kind = wind | pv | storage, 管理同类设备; Members 为空时管理全部同类设备
	EMSKind    string   `json:"ems_kind,omitempty"`
	EMSMembers []string `json:"ems_members,omitempty"`

	// 动态特性 (全部零值 = 旧行为: 指令瞬时生效)
	// 爬坡率 kW/s: Up 为功率数值上升方向(储能即向充电), Down 为下降方向; 只配一个时两个方向共用
	RampUpKWPerS    float64 `json:"ramp_up_kw_per_s,omitempty"`
	RampDownKWPerS  float64 `json:"ramp_down_kw_per_s,omitempty"`
	ResponseDelayMs int     `json:"response_delay_ms,omitempty"`
	// AvailableMode="limit": 光伏出力 = min(设定值, 理论功率测点); 风电恒为 limit
	AvailableMode string `json:"available_mode,omitempty"`
	// 储能 SOC 保护: 到限值时在功率层禁止继续充/放
	SOCProtection bool `json:"soc_protection,omitempty"`
	// 远方模式下设定值 0 作为真实 0 指令(默认 0 沿用"置数"语义: 读取 _Power)
	ZeroIsCommand bool `json:"zero_is_command,omitempty"`

	// Battery
	CapacityKWH    float64 `json:"capacity_kwh,omitempty"`
	RatedPowerKW_B float64 `json:"rated_power_kw_b,omitempty"`
	InitSOC        float64 `json:"init_soc,omitempty"`
	SOCMin         float64 `json:"soc_min,omitempty"`
	SOCMax         float64 `json:"soc_max,omitempty"`
	Eff            float64 `json:"eff,omitempty"`

	// Load
	LoadRatedKW float64 `json:"load_rated_kw,omitempty"`
	PowerFactor float64 `json:"power_factor,omitempty"`

	// Charger
	ChargerRatedKW float64 `json:"charger_rated_kw,omitempty"`
	ChargerEff     float64 `json:"charger_eff,omitempty"`
}

// ControlMode 控制模式
type ControlMode string

const (
	ModeRemote ControlMode = "remote" // 远方=跟随AO
	ModeLocal  ControlMode = "local"  // 本地=策略驱动
)

// StrategyConfig 自动变化策略
type StrategyConfig struct {
	Type    string         `json:"type"`    // increment/random/csv/max/min/soc/energy/aofollow/manual
	Enabled bool           `json:"enabled"`
	Params  map[string]any `json:"params,omitempty"`
}

// CustomPoint 自定义测点
type CustomPoint struct {
	Name  string `json:"name"`
	Type  string `json:"type"`  // AI/DI/DO/AO
	Alias string `json:"alias,omitempty"`
}

// Device 微电网设备
type Device struct {
	ID           string          `json:"id"`
	Type         ComponentType   `json:"type"`
	Name         string          `json:"name"`
	IOABase      uint32          `json:"ioa_base,omitempty"` // IOA 起始地址，由系统自动分配
	Switch       DeviceSwitch    `json:"switch"`
	Params       DeviceParams    `json:"params"`
	ControlMode  ControlMode     `json:"control_mode,omitempty"`
	Strategy     *StrategyConfig `json:"strategy,omitempty"`
	CustomPoints []CustomPoint   `json:"custom_points,omitempty"`
}

// FormulaRule 自定义公式变化规则
type FormulaRule struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Target     string `json:"target"`     // 目标测点名，如 "关口表_有功功率"
	Expression string `json:"expression"` // 表达式，如 "{Battery_Power} + {Load_Power}"
	Enabled    bool   `json:"enabled"`
}

// Topology 完整微电网拓扑
type Topology struct {
	GridMeter    GridMeterConfig `json:"grid_meter"`
	BusName      string          `json:"bus_name"`
	BusVoltageKV float64         `json:"bus_voltage_kv"`
	Devices      []Device        `json:"devices"`
	Formulas     []FormulaRule   `json:"formulas,omitempty"`
}

// GridMeterConfig 关口表配置
type GridMeterConfig struct {
	RatedCapacityKW float64 `json:"rated_capacity_kw"`
	// 进口/出口功率限值(kW, 正数); 0 表示沿用 RatedCapacityKW
	ImportLimitKW float64 `json:"import_limit_kw,omitempty"`
	ExportLimitKW float64 `json:"export_limit_kw,omitempty"`
	// 启用可遥控的并网开关点(关口表_并网开关状态/遥控); 默认关闭以保持既有点表不变
	BreakerPoints bool `json:"breaker_points,omitempty"`
	IslandMode      bool    `json:"island_mode"`
}

// InstanceConfig 微电网实例附加配置
type InstanceConfig struct {
	TopologyJSON string  `json:"topology_json,omitempty"`
	PointsJSON   string  `json:"points_json,omitempty"`
	TickMs       int     `json:"tick_ms,omitempty"`
	SpeedFactor  float64 `json:"speed_factor,omitempty"`
	ConfigDir    string  `json:"-"`
}

// SimSnapshot 仿真快照 — 用于历史记录
type SimSnapshot struct {
	Timestamp int64              `json:"ts"`
	Values    map[string]float64 `json:"values"` // device_id → power
}

// FromJSON 从 JSON 字符串解析拓扑
func (t *Topology) FromJSON(jsonStr string) error {
	return json.Unmarshal([]byte(jsonStr), t)
}

// isLimitMode 光伏/风电是否按"设定值与理论功率取小"运行
func (d Device) isLimitMode() bool {
	return d.Type == CompWind || (d.Type == CompPV && d.Params.AvailableMode == "limit")
}

// windCurve 返回风速参数(零值取默认 3/12/25 m/s)
func (p DeviceParams) windCurve() (cutIn, rated, cutOut float64) {
	cutIn, rated, cutOut = p.CutInMS, p.RatedMS, p.CutOutMS
	if cutIn <= 0 {
		cutIn = 3
	}
	if rated <= cutIn {
		rated = 12
	}
	if cutOut <= rated {
		cutOut = 25
	}
	return
}
