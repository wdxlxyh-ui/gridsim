package model

import "time"

type StrategyType string

const (
	StrategyIncrement     StrategyType = "increment"
	StrategyRandom        StrategyType = "random"
	StrategyCSV           StrategyType = "csv"
	StrategyMax           StrategyType = "max"
	StrategyMin           StrategyType = "min"
	StrategySOC           StrategyType = "soc"
	StrategyEnergy        StrategyType = "energy"
	StrategyAOFollow      StrategyType = "aofollow"
	StrategyAPIUpdate     StrategyType = "apiupdate"
	StrategyManual        StrategyType = "manual"
	StrategyCustomFormula StrategyType = "custom"
	StrategyTimestamp     StrategyType = "timestamp"
)

type StrategyParams struct {
	StartValue      float64 `json:"start_value,omitempty"`
	Step            float64 `json:"step,omitempty"`
	PeriodMs        int     `json:"period_ms,omitempty"`
	MaxValue        float64 `json:"max_value,omitempty"`
	MinValue        float64 `json:"min_value,omitempty"`
	MaxValueR       float64 `json:"max_value_r,omitempty"`
	DecimalPlaces   int     `json:"decimal_places,omitempty"`
	CSVFileName     string  `json:"csv_file,omitempty"`
	TimeFormat      string  `json:"time_format,omitempty"`
	TimeUnit        string  `json:"time_unit,omitempty"`
	CSVColumnMap    string  `json:"csv_column_map,omitempty"`
	ParaA           string  `json:"para_a,omitempty"`
	ParaB           string  `json:"para_b,omitempty"`
	InitSOC         float64 `json:"init_soc,omitempty"`
	RatedCap        float64 `json:"rated_cap,omitempty"`
	PowerIOA        uint32  `json:"power_ioa,omitempty"`
	IntegralMs      int     `json:"integral_ms,omitempty"`
	InitEnergy      float64 `json:"init_energy,omitempty"`
	StatType        int     `json:"stat_type,omitempty"`
	EnergyPowerIOA  uint32  `json:"energy_power_ioa,omitempty"`
	EnergyPeriodMs  int     `json:"energy_period_ms,omitempty"`
	FollowAOIOA      uint32  `json:"follow_ao_ioa,omitempty"`
	APIInitValue     float64 `json:"api_init_value,omitempty"`
	CustomIOAs       string  `json:"custom_ioas,omitempty"`
	CustomFormula    string  `json:"custom_formula,omitempty"`
	CSVLoop          *bool   `json:"csv_loop,omitempty"`
	TimestampMode    string  `json:"timestamp_mode,omitempty"`    // 时间戳模式：current, next_quarter, next_hour
	TimestampFormat  string  `json:"timestamp_format,omitempty"`  // 时间戳格式：seconds, milliseconds
	TimestampHighIOA uint32  `json:"timestamp_high_ioa,omitempty"` // 时间戳高位测点IOA
	TimestampLowIOA  uint32  `json:"timestamp_low_ioa,omitempty"`  // 时间戳低位测点IOA
}

type AutoChangeConfig struct {
	PointIOA  uint32         `json:"ioa"`
	Strategy  StrategyType   `json:"strategy"`
	Enabled   bool           `json:"enabled"`
	Params    StrategyParams `json:"params"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type QualityDescriptor struct {
	Invalid     bool `json:"invalid"`
	NotTopical  bool `json:"not_topical"`
	Substituted bool `json:"substituted"`
	Overflow    bool `json:"overflow"`
	Blocked     bool `json:"blocked"`
}

type PointSnapshot struct {
	IOA       uint32            `json:"ioa"`
	Name      string            `json:"name"`
	PointType string            `json:"point_type"`
	Value     float64           `json:"value"`
	BoolValue bool              `json:"bool_value"`
	IntValue  int32             `json:"int_value"`
	UpdatedAt time.Time         `json:"updated_at"`
	QDS       QualityDescriptor `json:"qds"`
	Unit      string            `json:"unit"`

	FunctionCode    uint8  `json:"function_code,omitempty"`
	RegisterAddress uint16 `json:"register_address,omitempty"`
	ByteOrder       string `json:"byte_order,omitempty"`
}
