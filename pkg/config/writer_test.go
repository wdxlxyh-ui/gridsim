package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.xlsx")

	original := []*Point{
		{IOA: 16385, Name: "母线电压", ValueType: VTFloat, PointType: TypeAI, Efficient: 1.0, BaseValue: 220.0, Alias: "Bus-V"},
		{IOA: 5, Name: "开关1", ValueType: VTBit, PointType: TypeDI, Efficient: 1.0, BaseValue: 0, Alias: ""},
		{IOA: 100, Name: "脉冲1", ValueType: VTInt, PointType: TypePI, Efficient: 1.0, BaseValue: 100, Alias: ""},
	}

	if err := SaveToXLSX(original, path); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("file not created")
	}

	loaded, err := LoadFromXLSX(path, "iec104")
	if err != nil {
		t.Fatal(err)
	}

	if len(loaded) != 3 {
		t.Fatalf("expected 3 points, got %d", len(loaded))
	}
	if loaded[0].Name != "母线电压" {
		t.Fatalf("expected 母线电压, got %s", loaded[0].Name)
	}
	if loaded[0].IOA != 16385 {
		t.Fatalf("expected IOA 16385, got %d", loaded[0].IOA)
	}

	modified := []*Point{
		{IOA: 16385, Name: "修改后的电压", ValueType: VTFloat, PointType: TypeAI, Efficient: 1.5, BaseValue: 230.0, Alias: "Modified"},
	}
	if err := SaveToXLSX(modified, path); err != nil {
		t.Fatal(err)
	}

	loaded2, err := LoadFromXLSX(path, "iec104")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded2) != 1 {
		t.Fatalf("expected 1 point after overwrite, got %d", len(loaded2))
	}
	if loaded2[0].Name != "修改后的电压" {
		t.Fatalf("expected 修改后的电压, got %s", loaded2[0].Name)
	}
}

func TestSaveToXLSXModbus(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "modbus_test.xlsx")

	points := []*Point{
		{IOA: 1, Name: "线圈1", ValueType: VTBit, PointType: TypeDI, Efficient: 1.0, BaseValue: 0, Alias: "",
			FunctionCode: 1, RegisterAddress: 100},
		{IOA: 2, Name: "温度", ValueType: VTFloat, PointType: TypeAI, Efficient: 0.1, BaseValue: 250, Alias: "Temp",
			FunctionCode: 3, RegisterAddress: 200},
	}

	if err := SaveToXLSX(points, path); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadFromXLSX(path, "modbus_tcp")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 points, got %d", len(loaded))
	}
	if loaded[1].FunctionCode != 3 {
		t.Fatalf("expected FC 3, got %d", loaded[1].FunctionCode)
	}
	if loaded[1].RegisterAddress != 200 {
		t.Fatalf("expected reg addr 200, got %d", loaded[1].RegisterAddress)
	}
}
