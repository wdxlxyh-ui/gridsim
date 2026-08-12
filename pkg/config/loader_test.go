package config

import (
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestLoadFromXLSX_ValidFile(t *testing.T) {
	// Create a temporary xlsx for testing
	dir := t.TempDir()
	xlsxPath := filepath.Join(dir, "test.xlsx")

	// Use excelize to create test file
	f := excelize.NewFile()
	f.SetSheetName("Sheet1", "point")
	f.SetCellValue("point", "A1", "point-name")
	f.SetCellValue("point", "B1", "point-number")
	f.SetCellValue("point", "C1", "value-type")
	f.SetCellValue("point", "D1", "point-type")
	f.SetCellValue("point", "E1", "efficient")
	f.SetCellValue("point", "F1", "base-value")
	f.SetCellValue("point", "G1", "alias")

	// Row 2: AI point
	f.SetCellValue("point", "A2", "AI_01")
	f.SetCellValue("point", "B2", 1001)
	f.SetCellValue("point", "C2", "FLOAT")
	f.SetCellValue("point", "D2", "AI")
	f.SetCellValue("point", "E2", 1.0)
	f.SetCellValue("point", "F2", 220.0)
	f.SetCellValue("point", "G2", "AI_01")

	// Row 3: DI point
	f.SetCellValue("point", "A3", "DI_01")
	f.SetCellValue("point", "B3", 2001)
	f.SetCellValue("point", "C3", "BIT")
	f.SetCellValue("point", "D3", "DI")
	f.SetCellValue("point", "E3", 1.0)
	f.SetCellValue("point", "F3", 0.0)

	// Row 4: PI point
	f.SetCellValue("point", "A4", "PI_01")
	f.SetCellValue("point", "B4", 3001)
	f.SetCellValue("point", "C4", "INT")
	f.SetCellValue("point", "D4", "PI")
	f.SetCellValue("point", "E4", 1.0)
	f.SetCellValue("point", "F4", 1000.0)

	if err := f.SaveAs(xlsxPath); err != nil {
		t.Fatalf("failed to create test xlsx: %v", err)
	}

	points, err := LoadFromXLSX(xlsxPath, "")
	if err != nil {
		t.Fatalf("LoadFromXLSX failed: %v", err)
	}

	if len(points) != 3 {
		t.Fatalf("expected 3 points, got %d", len(points))
	}

	// Verify AI point
	p1 := points[0]
	if p1.IOA != 1001 || p1.Name != "AI_01" || p1.PointType != TypeAI || p1.ValueType != VTFloat {
		t.Errorf("AI point mismatch: %+v", p1)
	}
	if p1.Value != 220.0 {
		t.Errorf("expected AI value 220.0, got %f", p1.Value)
	}

	// Verify DI point
	p2 := points[1]
	if p2.IOA != 2001 || p2.PointType != TypeDI || p2.BoolValue != false {
		t.Errorf("DI point mismatch: %+v", p2)
	}

	// Verify PI point
	p3 := points[2]
	if p3.IOA != 3001 || p3.PointType != TypePI || p3.IntValue != 1000 {
		t.Errorf("PI point mismatch: %+v", p3)
	}
}

func TestLoadFromXLSX_DuplicateIOA(t *testing.T) {
	dir := t.TempDir()
	xlsxPath := filepath.Join(dir, "dup.xlsx")

	f := excelize.NewFile()
	f.SetSheetName("Sheet1", "point")
	f.SetCellValue("point", "A1", "point-name")
	f.SetCellValue("point", "B1", "point-number")
	f.SetCellValue("point", "C1", "value-type")
	f.SetCellValue("point", "D1", "point-type")
	f.SetCellValue("point", "E1", "efficient")
	f.SetCellValue("point", "F1", "base-value")

	f.SetCellValue("point", "A2", "P1")
	f.SetCellValue("point", "B2", 1001)
	f.SetCellValue("point", "C2", "FLOAT")
	f.SetCellValue("point", "D2", "AI")
	f.SetCellValue("point", "E2", 1.0)
	f.SetCellValue("point", "F2", 0.0)

	f.SetCellValue("point", "A3", "P2")
	f.SetCellValue("point", "B3", 1001) // duplicate
	f.SetCellValue("point", "C3", "FLOAT")
	f.SetCellValue("point", "D3", "AI")
	f.SetCellValue("point", "E3", 1.0)
	f.SetCellValue("point", "F3", 0.0)

	f.SaveAs(xlsxPath)

	_, err := LoadFromXLSX(xlsxPath, "")
	if err == nil {
		t.Error("expected error for duplicate IOA, got nil")
	}
}

func TestLoadFromXLSX_InvalidPointType(t *testing.T) {
	dir := t.TempDir()
	xlsxPath := filepath.Join(dir, "bad.xlsx")

	f := excelize.NewFile()
	f.SetSheetName("Sheet1", "point")
	f.SetCellValue("point", "A1", "point-name")
	f.SetCellValue("point", "B1", "point-number")
	f.SetCellValue("point", "C1", "value-type")
	f.SetCellValue("point", "D1", "point-type")
	f.SetCellValue("point", "E1", "efficient")
	f.SetCellValue("point", "F1", "base-value")

	f.SetCellValue("point", "A2", "P1")
	f.SetCellValue("point", "B2", 1001)
	f.SetCellValue("point", "C2", "FLOAT")
	f.SetCellValue("point", "D2", "UNKNOWN")
	f.SetCellValue("point", "E2", 1.0)
	f.SetCellValue("point", "F2", 0.0)

	f.SaveAs(xlsxPath)

	_, err := LoadFromXLSX(xlsxPath, "")
	if err == nil {
		t.Error("expected error for unknown point-type, got nil")
	}
}

func TestLoadFromXLSX_ModbusAllowsZeroRegisterAddress(t *testing.T) {
	dir := t.TempDir()
	xlsxPath := filepath.Join(dir, "modbus-zero-address.xlsx")

	f := excelize.NewFile()
	f.SetSheetName("Sheet1", "point")
	headers := []string{"point-name", "point-number", "value-type", "point-type", "efficient", "base-value", "alias", "register-address", "function-code"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue("point", cell, header)
	}
	values := []interface{}{"AI_0", 1, "FLOAT", "AI", 1.0, 12.5, "", 0, 3}
	for i, value := range values {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		f.SetCellValue("point", cell, value)
	}
	if err := f.SaveAs(xlsxPath); err != nil {
		t.Fatalf("failed to create test xlsx: %v", err)
	}

	points, err := LoadFromXLSX(xlsxPath, "modbus_tcp")
	if err != nil {
		t.Fatalf("LoadFromXLSX failed: %v", err)
	}
	if len(points) != 1 || points[0].RegisterAddress != 0 || points[0].FunctionCode != 3 {
		t.Fatalf("unexpected points: %+v", points)
	}
}

func TestLoadFromXLSX_ModbusRejectsDuplicateFunctionAndAddress(t *testing.T) {
	dir := t.TempDir()
	xlsxPath := filepath.Join(dir, "modbus-duplicate-address.xlsx")

	f := excelize.NewFile()
	f.SetSheetName("Sheet1", "point")
	headers := []string{"point-name", "point-number", "value-type", "point-type", "efficient", "base-value", "alias", "register-address", "function-code"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue("point", cell, header)
	}
	rows := [][]interface{}{
		{"AI_1", 1, "FLOAT", "AI", 1.0, 1.0, "", 100, 3},
		{"AI_2", 2, "FLOAT", "AI", 1.0, 2.0, "", 100, 3},
	}
	for rowIndex, row := range rows {
		for columnIndex, value := range row {
			cell, _ := excelize.CoordinatesToCellName(columnIndex+1, rowIndex+2)
			f.SetCellValue("point", cell, value)
		}
	}
	if err := f.SaveAs(xlsxPath); err != nil {
		t.Fatalf("failed to create test xlsx: %v", err)
	}

	if _, err := LoadFromXLSX(xlsxPath, "modbus_tcp"); err == nil {
		t.Fatal("expected duplicate Modbus function/address error")
	}
}

func TestLoadFromXLSX_ModbusRejectsOverlapping32BitRegisters(t *testing.T) {
	dir := t.TempDir()
	xlsxPath := filepath.Join(dir, "modbus-overlap.xlsx")

	f := excelize.NewFile()
	f.SetSheetName("Sheet1", "point")
	headers := []string{"point-name", "point-number", "value-type", "point-type", "efficient", "base-value", "alias", "register-address", "function-code"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue("point", cell, header)
	}
	rows := [][]interface{}{
		{"AI_1", 1, "FLOAT", "AI", 1.0, 1.0, "", 100, 3},
		{"AI_2", 2, "FLOAT", "AI", 1.0, 2.0, "", 101, 3},
	}
	for rowIndex, row := range rows {
		for columnIndex, value := range row {
			cell, _ := excelize.CoordinatesToCellName(columnIndex+1, rowIndex+2)
			f.SetCellValue("point", cell, value)
		}
	}
	if err := f.SaveAs(xlsxPath); err != nil {
		t.Fatalf("failed to create test xlsx: %v", err)
	}

	if _, err := LoadFromXLSX(xlsxPath, "modbus_tcp"); err == nil {
		t.Fatal("expected overlapping 32-bit Modbus register error")
	}
}

func TestLoadFromXLSX_BundledModbusSample(t *testing.T) {
	path := filepath.Join("..", "..", "samples", "ModbusTCP-ESS.xlsx")
	points, err := LoadFromXLSX(path, "modbus_tcp")
	if err != nil {
		t.Fatalf("bundled Modbus sample failed to load: %v", err)
	}
	if len(points) != 5 {
		t.Fatalf("bundled Modbus sample points = %d, want 5", len(points))
	}
}

func TestLoadFromXLSX_ModbusAllowsFC6FloatPoint(t *testing.T) {
	dir := t.TempDir()
	xlsxPath := filepath.Join(dir, "modbus-fc6-float.xlsx")

	f := excelize.NewFile()
	f.SetSheetName("Sheet1", "point")
	headers := []string{"point-name", "point-number", "value-type", "point-type", "efficient", "base-value", "alias", "register-address", "function-code"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue("point", cell, header)
	}
	row := []interface{}{"AO_FC6", 1, "FLOAT", "AO", 1.0, 0.0, "", 31, 6}
	for columnIndex, value := range row {
		cell, _ := excelize.CoordinatesToCellName(columnIndex+1, 2)
		f.SetCellValue("point", cell, value)
	}
	if err := f.SaveAs(xlsxPath); err != nil {
		t.Fatalf("failed to create test xlsx: %v", err)
	}

	points, err := LoadFromXLSX(xlsxPath, "modbus_tcp")
	if err != nil {
		t.Fatalf("FC6 FLOAT point failed to load: %v", err)
	}
	if len(points) != 1 || points[0].FunctionCode != 6 || points[0].RegisterAddress != 31 {
		t.Fatalf("unexpected FC6 point: %+v", points)
	}
}

func TestLoadFromXLSX_ModbusRejectsFC3FC6AddressOverlap(t *testing.T) {
	dir := t.TempDir()
	xlsxPath := filepath.Join(dir, "modbus-fc3-fc6-overlap.xlsx")

	f := excelize.NewFile()
	f.SetSheetName("Sheet1", "point")
	headers := []string{"point-name", "point-number", "value-type", "point-type", "efficient", "base-value", "alias", "register-address", "function-code"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue("point", cell, header)
	}
	rows := [][]interface{}{
		{"AI_FC3", 1, "FLOAT", "AI", 1.0, 1.0, "", 30, 3},
		{"AO_FC6", 2, "FLOAT", "AO", 1.0, 2.0, "", 31, 6},
	}
	for rowIndex, row := range rows {
		for columnIndex, value := range row {
			cell, _ := excelize.CoordinatesToCellName(columnIndex+1, rowIndex+2)
			f.SetCellValue("point", cell, value)
		}
	}
	if err := f.SaveAs(xlsxPath); err != nil {
		t.Fatalf("failed to create test xlsx: %v", err)
	}

	if _, err := LoadFromXLSX(xlsxPath, "modbus_tcp"); err == nil {
		t.Fatal("expected FC3/FC6 holding-register address overlap error")
	}
}
