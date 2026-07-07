package model

// InstanceConfig represents a persisted instance configuration.
type InstanceConfig struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	IEC104Port  int    `json:"iec104_port"`
	XLSXFile    string `json:"xlsx_file"`
	Enabled     bool   `json:"enabled"`
	HttpEnabled bool   `json:"http_enabled"`
	HttpPort    int    `json:"http_port"`

	Protocol           string                   `json:"protocol,omitempty"`
	ModbusConfig       *ModbusInstanceConfig    `json:"modbus_config,omitempty"`
	MicrogridConfig    *MicrogridInstanceConfig `json:"microgrid_config,omitempty"`
	IEC104ClientConfig *IEC104ClientConfig      `json:"iec104_client_config,omitempty"`
	ModbusBridgeConfig *ModbusBridgeConfig      `json:"modbus_bridge_config,omitempty"`
}

// IEC104ClientConfig 客户端实例配置（主站模式）
type IEC104ClientConfig struct {
	RemoteAddr     string `json:"remote_addr"`
	RemotePort     int    `json:"remote_port"`
	CommonAddr     int    `json:"common_addr"`
	ReconnectDelay int    `json:"reconnect_delay"`
	ConnectTimeout int    `json:"connect_timeout"`
	InterrogPeriod int    `json:"interrog_period"` // 总召周期（秒），默认 600
	RetryDelay     int    `json:"retry_delay"`     // 总召失败重试间隔（秒），默认 20
	ControlMode    string `json:"control_mode"`    // "select" 或 "direct"，默认 "select"
}

// MicrogridInstanceConfig 微电网实例配置
type MicrogridInstanceConfig struct {
	TopologyJSON string  `json:"topology_json,omitempty"`
	PointsJSON   string  `json:"points_json,omitempty"`
	TickMs       int     `json:"tick_ms,omitempty"`
	SpeedFactor  float64 `json:"speed_factor,omitempty"`
}

type ModbusInstanceConfig struct {
	Port      int    `json:"port,omitempty"`
	ByteOrder string `json:"byte_order,omitempty"`
	SlaveID   uint8  `json:"slave_id,omitempty"`
}

// ModbusBridgeConfig Python 微电网模拟器桥接配置
type ModbusBridgeConfig struct {
	PythonPath     string `json:"python_path,omitempty"`      // Python 解释器路径，默认 "python3"
	ScriptDir      string `json:"script_dir,omitempty"`       // Python 项目目录（相对于 configDir）
	ModbusPort     int    `json:"modbus_port,omitempty"`      // Python Modbus 监听端口，默认 5021
	PollIntervalMs int    `json:"poll_interval_ms,omitempty"` // 轮询间隔（ms），默认 1000
	StartTime      string `json:"start_time,omitempty"`       // 仿真起始时间 HH:MM，空则用当前时间
	IEC104Port     int    `json:"iec104_port,omitempty"`      // 可选：同时对外暴露 IEC104 端口
	DeviceJSON     string `json:"device_json,omitempty"`      // device.json 相对路径
}

// InstanceStatus represents the runtime status of an instance.
type InstanceStatus string

const (
	StatusStopped InstanceStatus = "stopped"
	StatusRunning InstanceStatus = "running"
	StatusError   InstanceStatus = "error"
)

// InstanceState is the runtime state of a managed instance.
type InstanceState struct {
	Config          InstanceConfig `json:"config"`
	Status          InstanceStatus `json:"status"`
	UptimeSeconds   int64          `json:"uptime_seconds,omitempty"`
	TotalPoints     int            `json:"total_points,omitempty"`
	ClientConnected bool           `json:"client_connected"`
	Interrogations  int64          `json:"interrogations,omitempty"`
	Controls        int64          `json:"controls,omitempty"`
	Spontaneous     int64          `json:"spontaneous,omitempty"`
	Error           string         `json:"error,omitempty"`
}
