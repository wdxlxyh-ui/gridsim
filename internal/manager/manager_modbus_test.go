package manager

import (
	"path/filepath"
	"testing"

	"gridsim/internal/model"
	"gridsim/internal/storage"
)

func TestInstanceProtocolPortUsesModbusConfig(t *testing.T) {
	cfg := model.InstanceConfig{
		Protocol:   "modbus_tcp",
		IEC104Port: 1502,
		ModbusConfig: &model.ModbusInstanceConfig{
			Port: 2502,
		},
	}

	if got := instanceProtocolPort(cfg); got != 2502 {
		t.Fatalf("port = %d, want 2502", got)
	}
}

func TestInstanceProtocolPortFallsBackToCommonPort(t *testing.T) {
	cfg := model.InstanceConfig{Protocol: "modbus_tcp", IEC104Port: 1502}
	if got := instanceProtocolPort(cfg); got != 1502 {
		t.Fatalf("port = %d, want 1502", got)
	}
}

func TestCreateConfigDetectsActualModbusPortConflict(t *testing.T) {
	store := storage.NewConfigStore(filepath.Join(t.TempDir(), "instances.json"))
	manager := New(store, t.TempDir())

	if _, err := manager.CreateConfig(model.InstanceConfig{
		ID: "iec", Name: "IEC", Protocol: "iec104", IEC104Port: 2502,
	}); err != nil {
		t.Fatal(err)
	}
	_, err := manager.CreateConfig(model.InstanceConfig{
		ID: "modbus", Name: "Modbus", Protocol: "modbus_tcp", IEC104Port: 1502,
		ModbusConfig: &model.ModbusInstanceConfig{Port: 2502},
	})
	if err == nil {
		t.Fatal("expected conflict on actual Modbus port 2502")
	}
}

func TestUpdateConfigDetectsProtocolAndHTTPPortConflict(t *testing.T) {
	store := storage.NewConfigStore(filepath.Join(t.TempDir(), "instances.json"))
	manager := New(store, t.TempDir())

	if _, err := manager.CreateConfig(model.InstanceConfig{
		ID: "first", Name: "First", Protocol: "iec104", IEC104Port: 2404,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.CreateConfig(model.InstanceConfig{
		ID: "second", Name: "Second", Protocol: "modbus_tcp", IEC104Port: 1502,
		ModbusConfig: &model.ModbusInstanceConfig{Port: 1502},
	}); err != nil {
		t.Fatal(err)
	}

	err := manager.UpdateConfig(model.InstanceConfig{
		ID: "second", Name: "Second", Protocol: "modbus_tcp", IEC104Port: 1502,
		HttpEnabled: true, HttpPort: 2404,
		ModbusConfig: &model.ModbusInstanceConfig{Port: 1502},
	})
	if err == nil {
		t.Fatal("expected HTTP/protocol cross-port conflict")
	}
}
