package modbus_bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gridsim/pkg/config"
)

// PointMapping maps a Python Modbus register to a GridSim IOA point.
type PointMapping struct {
	IOA            uint32
	Name           string
	RegisterOffset uint16
	PointType      config.PointType
	Writable       bool
	Description    string
	HighFreq       bool // high-frequency collection group
}

// DeviceMapping represents one Python device with its slave ID and points.
type DeviceMapping struct {
	DeviceKey  string
	DeviceType string
	SlaveID    uint8
	Points     []PointMapping
}

// IOA base allocation per device type
var ioaBaseByType = map[string]uint32{
	"Meter": 1000,
	"PV":    2000,
	"BESS":  3000,
	"EV":    4000,
	"Load":  5000,
}

// Register definitions per device type (matching Python modbus_registers.py)
type regDef struct {
	Name     string
	Offset   uint16
	Type     config.PointType
	Writable bool
	Desc     string
	HighFreq bool // true = 高频采集组 (group 1, 100ms)
}

var registerDefs = map[string][]regDef{
	"Meter": {
		{"METER.ActivePW", 0, config.TypeAI, false, "并网点有功功率 kW", true},
	},
	"PV": {
		{"INV.GenActivePW", 0, config.TypeAI, false, "发电功率 kW", true},
		{"INV.APProductionKWH", 2, config.TypeAI, false, "累计发电量 kWh", false},
		{"INV.LimitPower", 4, config.TypeAO, true, "限功率 kW", false},
		{"INV.OemState", 6, config.TypeDI, false, "OEM状态", false},
		{"INV.CtrlState", 8, config.TypeDI, false, "控制状态", true},
		{"INV.Start", 10, config.TypeDI, false, "启动标志", false},
		{"INV.Stop", 12, config.TypeDI, false, "停止标志", false},
		{"INV.State", 14, config.TypeDI, false, "运行状态", false},
		{"INV.APProduction", 16, config.TypeAI, false, "瞬时发电 kW", false},
	},
	"BESS": {
		{"BS.ActivePW", 0, config.TypeAI, false, "有功功率 kW", true},
		{"BS.Soc", 2, config.TypeAI, false, "SOC %", false},
		{"BS.MaxChargePower", 4, config.TypeAI, false, "最大充电功率 kW", false},
		{"BS.MaxDischargePower", 6, config.TypeAI, false, "最大放电功率 kW", false},
		{"BS.EndChargeSOC", 8, config.TypeAI, false, "充电截止 SOC %", false},
		{"BS.EndDischargeSOC", 10, config.TypeAI, false, "放电截止 SOC %", false},
		{"BS.Soh", 12, config.TypeAI, false, "SOH %", false},
		{"BS.SysAPSetPoint", 14, config.TypeAO, true, "功率设定 kW", false},
		{"BS.OemState", 16, config.TypeDI, false, "OEM状态", false},
		{"BS.CtrlState", 18, config.TypeDI, false, "控制状态", true},
		{"BS.TotalChargingEng", 20, config.TypeAI, false, "累计充电 kWh", false},
		{"BS.TotalDischargingEng", 22, config.TypeAI, false, "累计放电 kWh", false},
	},
	"EV": {
		{"PUB_CONN.ChargePW", 0, config.TypeAI, false, "充电功率 kW", true},
		{"PUB_CONN.CurrentL1", 2, config.TypeAI, false, "L1电流 A", false},
		{"PUB_CONN.CurrentL2", 4, config.TypeAI, false, "L2电流 A", false},
		{"PUB_CONN.CurrentL3", 6, config.TypeAI, false, "L3电流 A", false},
		{"PUB_CONN.ChargePWSet", 8, config.TypeAO, true, "功率设定 kW", false},
		{"PUB_CONN.ChargeCurSetL1", 10, config.TypeAO, true, "电流设定 A", false},
		{"PUB_CONN.OemState", 12, config.TypeDI, false, "OEM状态", false},
		{"PUB_CONN.ChargeEnergyKWH", 14, config.TypeAI, false, "累计电量 kWh", false},
		{"PUB_CONN.CtrlState", 16, config.TypeDI, false, "控制状态", true},
		{"PUB_CONN.State", 18, config.TypeDI, false, "运行状态", false},
		{"PUB_CONN.PhaseMode", 20, config.TypeDI, false, "相模式", false},
	},
	"Load": {
		{"Load.Power", 0, config.TypeAI, false, "负荷功率 kW", true},
	},
}

// ParseDeviceJSON reads device.json and generates IOA-mapped device/point structures.
func ParseDeviceJSON(scriptDir, deviceJSONPath string) ([]DeviceMapping, error) {
	fullPath := filepath.Join(scriptDir, deviceJSONPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", fullPath, err)
	}

	var raw struct {
		StartTime string                     `json:"start_time"`
		Devices   []map[string][]interface{} `json:"Devices"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal device.json: %w", err)
	}

	// Count devices per type for IOA base offset
	typeCount := map[string]int{}
	var devices []DeviceMapping

	for _, devGroup := range raw.Devices {
		for devType, devList := range devGroup {
			for _, devRaw := range devList {
				devBytes, _ := json.Marshal(devRaw)
				var devMap map[string]interface{}
				json.Unmarshal(devBytes, &devMap)

				deviceKey, _ := devMap["DeviceKey"].(string)
				slaveID := uint8(1)
				if sid, ok := devMap["slave_id"].(float64); ok {
					slaveID = uint8(sid)
				}

				idx := typeCount[devType]
				typeCount[devType]++

				ioaBase := ioaBaseByType[devType] + uint32(idx)*100
				regs := registerDefs[devType]

				points := make([]PointMapping, 0, len(regs))
				for i, reg := range regs {
					points = append(points, PointMapping{
						IOA:            ioaBase + uint32(i),
						Name:           deviceKey + "." + reg.Name,
						RegisterOffset: reg.Offset,
						PointType:      reg.Type,
						Writable:       reg.Writable,
						Description:    reg.Desc,
						HighFreq:       reg.HighFreq,
					})
				}

				devices = append(devices, DeviceMapping{
					DeviceKey:  deviceKey,
					DeviceType: devType,
					SlaveID:    slaveID,
					Points:     points,
				})
			}
		}
	}

	return devices, nil
}

// BuildStorePoints creates config.Point slice from device mappings for library.Store.
func BuildStorePoints(devices []DeviceMapping) []*config.Point {
	var points []*config.Point
	for _, dev := range devices {
		for _, pt := range dev.Points {
			p := &config.Point{
				IOA:       pt.IOA,
				Name:      pt.Name,
				PointType: pt.PointType,
				ValueType: config.VTFloat,
				Alias:     pt.Description,
				Timestamp: time.Now(),
			}
			if pt.PointType == config.TypeDI || pt.PointType == config.TypeDO {
				p.ValueType = config.VTBit
			}
			points = append(points, p)
		}
	}
	return points
}

// UpdateStartTime modifies the start_time field in device.json.
func UpdateStartTime(scriptDir, deviceJSONPath, startTime string) error {
	fullPath := filepath.Join(scriptDir, deviceJSONPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return err
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	raw["start_time"] = startTime

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(fullPath, out, 0644)
}
