package microgrid

import "encoding/json"

// ComponentType 微电网设备类型
type ComponentType string

const (
	CompPV      ComponentType = "pv"
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

// CustomPoint 自定义测点
type CustomPoint struct {
	Name string `json:"name"`
	Type string `json:"type"` // AI/DI/DO/AO
}

// Device 微电网设备
type Device struct {
	ID           string         `json:"id"`
	Type         ComponentType  `json:"type"`
	Name         string         `json:"name"`
	Switch       DeviceSwitch   `json:"switch"`
	Params       DeviceParams   `json:"params"`
	ControlMode  ControlMode    `json:"control_mode,omitempty"`  // remote | local，默认remote
	CustomPoints []CustomPoint  `json:"custom_points,omitempty"`
}

// FormulaRule 自定义公式变化规则
type FormulaRule struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Target     string `json:"target"`     // 目标测点名，如 "GRID_P"
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
	IslandMode      bool    `json:"island_mode"`
}

// InstanceConfig 微电网实例附加配置
type InstanceConfig struct {
	TopologyJSON string  `json:"topology_json,omitempty"`
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
