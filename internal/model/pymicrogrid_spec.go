package model

import "time"

// PyMicrogridSpec 规范配置结构（设计文档第6节）
type PyMicrogridSpec struct {
	SchemaVersion int                   `json:"schema_version" binding:"required" example:"2"`
	Bridge        BridgeConfig          `json:"bridge" binding:"required"`
	Simulation    SimulationConfig      `json:"simulation" binding:"required"`
	Logging       LoggingConfig         `json:"logging" binding:"required"`
	Replay        ReplayConfig          `json:"replay"`
	Allocations   AllocationConfig      `json:"allocations" binding:"required"`
	Devices       []DeviceConfig        `json:"devices" binding:"required,min=1"`
}

// BridgeConfig 桥接配置
type BridgeConfig struct {
	ModbusPort     int `json:"modbus_port" binding:"min=1024,max=65535" example:"5021"`
	PollIntervalMs int `json:"poll_interval_ms" binding:"min=100,max=60000" example:"1000"`
}

// SimulationConfig 仿真配置
type SimulationConfig struct {
	StartTime string `json:"start_time" binding:"required" example:"14:30"` // HH:MM
	Timezone  string `json:"timezone" binding:"required" example:"Asia/Shanghai"`
	StepSec   int    `json:"step_sec" binding:"min=1,max=3600" example:"1"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	SystemLevel  string `json:"system_level" binding:"required,oneof=DEBUG INFO WARN ERROR" example:"INFO"`
	MessageLevel string `json:"message_level" binding:"required,oneof=DEBUG INFO WARN ERROR" example:"INFO"`
	TrafficLevel string `json:"traffic_level" binding:"required,oneof=DEBUG INFO WARN ERROR" example:"INFO"`
	MaxSizeMb    int    `json:"max_size_mb" binding:"min=1,max=1024" example:"10"`
	MaxBackups   int    `json:"max_backups" binding:"min=0,max=100" example:"5"`
}

// ReplayConfig TSDB 回放配置
type ReplayConfig struct {
	Enabled            bool                        `json:"enabled"`
	CredentialRef      string                      `json:"credential_ref,omitempty"`
	FetchIntervalSec   int                         `json:"fetch_interval_sec" binding:"min=5,max=3600" example:"30"`
	LookbackSec        int                         `json:"lookback_sec" binding:"min=10,max=86400" example:"300"`
	DelaySec           int                         `json:"delay_sec" binding:"max=86400" example:"10"`
	TargetLagSec       int                         `json:"target_lag_sec" binding:"max=86400" example:"60"`
	CacheRetentionSec  int                         `json:"cache_retention_sec" binding:"min=60" example:"600"`
	RequestTimeoutSec  int                         `json:"request_timeout_sec" binding:"min=1,max=120" example:"30"`
	PageSize           int                         `json:"page_size" binding:"min=1,max=10000" example:"10000"`
	MaxStalenessSec    int                         `json:"max_staleness_sec" example:"180"`
	FailurePolicy      string                      `json:"failure_policy" binding:"oneof=hold zero stop" example:"hold"`
	AdditionalSubs     []AdditionalSubscription    `json:"additional_subscriptions,omitempty"`
}

// AdditionalSubscription 附加订阅
type AdditionalSubscription struct {
	AssetID   string   `json:"asset_id" binding:"required"`
	PointIDs  []string `json:"point_ids" binding:"required,min=1"`
	Required  bool     `json:"required"`
}

// AllocationConfig 地址分配配置
type AllocationConfig struct {
	SlaveIDStart int `json:"slave_id_start" binding:"min=1,max=247" example:"1"`
	IOABase      int `json:"ioa_base" binding:"min=1" example:"1000"`
	IOAStep      int `json:"ioa_step" binding:"min=100" example:"100"`
}

// DeviceConfig 设备配置
type DeviceConfig struct {
	DeviceKey      string         `json:"device_key" binding:"required,min=1,max=64"`
	DeviceType     string         `json:"device_type" binding:"required,oneof=meter pv bess ev load wind"`
	SlaveID        int            `json:"slave_id" binding:"min=1,max=247"`
	IOABase        int            `json:"ioa_base" binding:"min=1"`
	IncludeInMeter bool           `json:"include_in_meter"`
	Params         DeviceParams   `json:"params"`
	Source         DataSource     `json:"source"`
}

// DeviceParams 设备参数（根据设备类型有不同字段）
type DeviceParams struct {
	// Meter 参数
	// (无额外参数，使用计算模式或EnOS模式)

	// PV 参数
	RatedPowerKW *float64 `json:"rated_power_kw,omitempty"`

	// BESS 参数
	RatedCapacityKWH        *float64 `json:"rated_capacity_kwh,omitempty"`
	MaxChargePowerKW        *float64 `json:"max_charge_power_kw,omitempty"`
	MaxDischargePowerKW     *float64 `json:"max_discharge_power_kw,omitempty"`
	SocMinPct               *float64 `json:"soc_min_pct,omitempty"`
	InitialSocPct           *float64 `json:"initial_soc_pct,omitempty"`
	SocMaxPct               *float64 `json:"soc_max_pct,omitempty"`
	VoltageNominalV         *float64 `json:"voltage_nominal_v,omitempty"`
	ResistanceOhm           *float64 `json:"resistance_ohm,omitempty"`

	// EV 参数
	MinChargePowerKW *float64           `json:"min_charge_power_kw,omitempty"`
	ChargeFactor     *float64           `json:"charge_factor,omitempty"`
	ChargerType      string             `json:"charger_type,omitempty"` // AC/DC
	PowerFactor      *float64           `json:"power_factor,omitempty"`
	PhaseMode        *int               `json:"phase_mode,omitempty"` // 1 or 3
	VoltageV         *float64           `json:"voltage_v,omitempty"`
	ChargeSchedule   []ChargeSchedule   `json:"charge_schedule,omitempty"`

	// Load 参数
	BasePowerKW *float64 `json:"base_power_kw,omitempty"`

	// Wind 参数
	MinCoverageRatio *float64 `json:"min_coverage_ratio,omitempty"`
}

// ChargeSchedule 充电时段
type ChargeSchedule struct {
	Start string `json:"start" binding:"required"` // HH:MM
	Stop  string `json:"stop" binding:"required"`  // HH:MM
}

// DataSource 数据源配置
type DataSource struct {
	Kind string    `json:"kind" binding:"required,oneof=calculated synthetic csv enos enos_aggregate"`
	CSV  *CSVSrc   `json:"csv,omitempty"`
	EnOS *EnosSrc  `json:"enos,omitempty"`
}

// CSVSrc CSV数据源
type CSVSrc struct {
	FileID        string  `json:"file_id" binding:"required"`
	Interpolation string  `json:"interpolation" binding:"oneof=linear hold" example:"linear"`
	EndBehavior   string  `json:"end_behavior" binding:"oneof=loop hold stop" example:"loop"`
	PeriodSec     *int    `json:"period_sec,omitempty"`
	Scale         float64 `json:"scale" example:"1"`
	Offset        float64 `json:"offset" example:"0"`
}

// EnosSrc EnOS数据源
type EnosSrc struct {
	AssetID                    string   `json:"asset_id,omitempty"`
	PowerPoint                 string   `json:"power_point,omitempty"`
	SocPoint                   string   `json:"soc_point,omitempty"`
	TheoryPowerPoint           string   `json:"theory_power_point,omitempty"`
	WindSpeedPoint             string   `json:"wind_speed_point,omitempty"`
	AggregateAssetIDs          []string `json:"aggregate_asset_ids,omitempty"`
	WindSpeedFallbackAssetIDs  []string `json:"wind_speed_fallback_asset_ids,omitempty"`
	MinCoverageRatio           *float64 `json:"min_coverage_ratio,omitempty"`
}

// EnOSCredential EnOS 凭据档案
type EnOSCredential struct {
	ID               string    `json:"id" binding:"required"`
	Name             string    `json:"name" binding:"required"`
	ApigwAddress     string    `json:"apigw_address" binding:"required,url"`
	OrgID            string    `json:"org_id" binding:"required"`
	AccessKey        string    `json:"access_key,omitempty"`
	SecretKey        string    `json:"secret_key,omitempty"`
	AccessKeyPresent bool      `json:"access_key_present"`
	SecretKeyPresent bool      `json:"secret_key_present"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// RuntimeStatus 运行时状态
type RuntimeStatus struct {
	OverallState string           `json:"overall_state"`
	Process      ProcessStatus    `json:"process"`
	Modbus       ModbusStatus     `json:"modbus"`
	Replay       ReplayStatus     `json:"replay"`
}

type ProcessStatus struct {
	PID             int    `json:"pid"`
	StartedAt       string `json:"started_at"`
	LastTickAt      string `json:"last_tick_at"`
	RuntimeVersion  string `json:"runtime_version"`
	ConfigRevision  string `json:"config_revision"`
	LastError       string `json:"last_error"`
}

type ModbusStatus struct {
	Connected    bool   `json:"connected"`
	LastPollAt   string `json:"last_poll_at"`
	LastError    string `json:"last_error"`
}

type ReplayStatus struct {
	State                  string            `json:"state"`
	LastRequestAt          string            `json:"last_request_at"`
	LastSuccessAt          string            `json:"last_success_at"`
	ConsecutiveFailures    int               `json:"consecutive_failures"`
	TargetLagSec           int               `json:"target_lag_sec"`
	Caches                 []CacheStatus     `json:"caches"`
}

type CacheStatus struct {
	AssetID      string `json:"asset_id"`
	PointID      string `json:"point_id"`
	Count        int    `json:"count"`
	EarliestAt   string `json:"earliest_at"`
	LatestAt     string `json:"latest_at"`
	AgeSec       int    `json:"age_sec"`
	CursorLagSec int    `json:"cursor_lag_sec"`
	Clamped      bool   `json:"clamped"`
}