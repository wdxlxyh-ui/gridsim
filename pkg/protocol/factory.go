package protocol

import (
	"fmt"

	"gridsim/internal/model"
	"gridsim/pkg/iec104"
	"gridsim/pkg/protocol/modbus"
	"gridsim/pkg/protocol/modbus_bridge"
)

func New(cfg model.InstanceConfig) (Protocol, error) {
	return NewWithCfgDir(cfg, "")
}

// NewWithCfgDir creates a protocol instance with access to the config directory.
func NewWithCfgDir(cfg model.InstanceConfig, cfgDir string) (Protocol, error) {
	switch cfg.Protocol {
	case "modbus_tcp":
		port := cfg.IEC104Port
		if cfg.ModbusConfig != nil && cfg.ModbusConfig.Port > 0 {
			port = cfg.ModbusConfig.Port
		}
		slaveID := uint8(1)
		byteOrder := "ABCD"
		if cfg.ModbusConfig != nil {
			if cfg.ModbusConfig.SlaveID > 0 {
				slaveID = cfg.ModbusConfig.SlaveID
			}
			if cfg.ModbusConfig.ByteOrder != "" {
				byteOrder = cfg.ModbusConfig.ByteOrder
			}
		}
		return modbus.NewTCPServer(port, slaveID, byteOrder), nil
	case "modbus_bridge":
		if cfg.ModbusBridgeConfig == nil {
			return nil, fmt.Errorf("modbus_bridge_config required for modbus_bridge protocol")
		}
		return modbus_bridge.New(cfg, cfgDir), nil
	case "microgrid":
		return NewIEC104Wrapper(cfg.IEC104Port), nil
	case "iec104_client":
		if cfg.IEC104ClientConfig == nil {
			return nil, fmt.Errorf("iec104_client_config required for client mode")
		}
		return iec104.NewClient(*cfg.IEC104ClientConfig), nil
	case "", "iec104":
		return NewIEC104Wrapper(cfg.IEC104Port), nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", cfg.Protocol)
	}
}

func SupportedProtocols() []string {
	return []string{"iec104", "modbus_tcp", "microgrid", "iec104_client", "modbus_bridge"}
}
