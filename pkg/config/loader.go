package config

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

func LoadFromXLSX(path string, protocol string) ([]*Point, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	defer f.Close()

	sheet := "point"
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("get rows from sheet %q: %w", sheet, err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("sheet %q has no data rows", sheet)
	}

	var points []*Point
	// 不同测点类型（AI/AO/DI/DO）可以复用相同的 IOA 地址空间
	seen := make(map[string]bool)
	occupiedModbus := make(map[uint32]int)

	for i, row := range rows[1:] {
		if len(row) < 6 {
			continue
		}

		name := strings.TrimSpace(row[0])
		if name == "" {
			continue
		}

		ioaStr := strings.TrimSpace(row[1])
		ioa, err := strconv.ParseUint(ioaStr, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid IOA %q: %w", i+2, ioaStr, err)
		}

		ptRaw := strings.TrimSpace(row[3])
		var pt PointType
		switch strings.ToUpper(ptRaw) {
		case "AI":
			pt = TypeAI
		case "DI":
			pt = TypeDI
		case "PI":
			pt = TypePI
		case "DO":
			pt = TypeDO
		case "AO":
			pt = TypeAO
		default:
			return nil, fmt.Errorf("row %d: unknown point-type %q", i+2, ptRaw)
		}

		key := string(pt) + ":" + ioaStr
		if seen[key] {
			return nil, fmt.Errorf("row %d: duplicate %s IOA %d", i+2, pt, ioa)
		}
		seen[key] = true

		vtRaw := strings.TrimSpace(row[2])
		var vt ValueType
		switch strings.ToUpper(vtRaw) {
		case "FLOAT":
			vt = VTFloat
		case "DOUBLE":
			vt = VTDouble
		case "INT":
			vt = VTInt
		case "BIT":
			vt = VTBit
		default:
			vt = VTFloat
		}

		efficient := 1.0
		if len(row) > 4 {
			eStr := strings.TrimSpace(row[4])
			if eStr != "" {
				efficient, err = strconv.ParseFloat(eStr, 64)
				if err != nil {
					return nil, fmt.Errorf("row %d: invalid efficient %q: %w", i+2, eStr, err)
				}
			}
		}

		baseValue := 0.0
		if len(row) > 5 {
			bStr := strings.TrimSpace(row[5])
			if bStr != "" {
				baseValue, err = strconv.ParseFloat(bStr, 64)
				if err != nil {
					return nil, fmt.Errorf("row %d: invalid base-value %q: %w", i+2, bStr, err)
				}
			}
		}

		alias := ""
		if len(row) > 6 {
			alias = strings.TrimSpace(row[6])
		}

		functionCode := uint8(0)
		registerAddr := uint16(0)
		functionCodeSet := false
		registerAddrSet := false
		byteOrder := "ABCD"

		// Modbus TCP 格式列顺序: register-address, function-code, value-type(可忽略), group-number, call-interval, user-defined-rule
		// 列7 (index 7): register-address
		// 列8 (index 8): function-code
		// 列9-12: group-number, call-interval, user-defined-rule (忽略)
		if len(row) > 7 {
			if raStr := strings.TrimSpace(row[7]); raStr != "" {
				ra, err := strconv.ParseUint(raStr, 10, 16)
				if err != nil {
					return nil, fmt.Errorf("row %d: invalid register_address %q: %w", i+2, raStr, err)
				}
				registerAddr = uint16(ra)
				registerAddrSet = true
			}
		}

		if len(row) > 8 {
			if fcStr := strings.TrimSpace(row[8]); fcStr != "" {
				fc, err := strconv.ParseUint(fcStr, 10, 8)
				if err != nil {
					return nil, fmt.Errorf("row %d: invalid function_code %q: %w", i+2, fcStr, err)
				}
				functionCode = uint8(fc)
				functionCodeSet = true
			}
		}

		// 列9-12 (index 9-12): 忽略 group-number, call-interval, user-defined-rule

		isModbus := protocol == "modbus_tcp" || protocol == "modbus_rtu"
		if isModbus && !functionCodeSet {
			return nil, fmt.Errorf("row %d: function_code is required for Modbus protocol", i+2)
		}
		if isModbus && !registerAddrSet {
			return nil, fmt.Errorf("row %d: register_address is required for Modbus protocol", i+2)
		}
		if isModbus {
			switch functionCode {
			case 1, 2, 3, 4, 5, 6, 15, 16:
			default:
				return nil, fmt.Errorf("row %d: unsupported Modbus function_code %d", i+2, functionCode)
			}
			addressSpace := functionCode
			switch functionCode {
			case 5, 15:
				addressSpace = 1
			case 6, 16:
				addressSpace = 3
			}
			width := uint32(1)
			if (addressSpace == 3 || addressSpace == 4) &&
				(pt == TypeAI || pt == TypeAO || pt == TypePI) {
				width = 2
			}
			if uint32(registerAddr)+width > 1<<16 {
				return nil, fmt.Errorf("row %d: Modbus value at register_address %d exceeds address space", i+2, registerAddr)
			}
			for offset := uint32(0); offset < width; offset++ {
				key := uint32(addressSpace)<<16 | uint32(registerAddr) + offset
				if previousRow, exists := occupiedModbus[key]; exists {
					return nil, fmt.Errorf("row %d: Modbus address range overlaps row %d at register_address %d", i+2, previousRow, uint32(registerAddr)+offset)
				}
				occupiedModbus[key] = i + 2
			}
		}

		p := &Point{
			IOA:             uint32(ioa),
			Name:            name,
			ValueType:       vt,
			PointType:       pt,
			Efficient:       efficient,
			BaseValue:       baseValue,
			Alias:           alias,
			FunctionCode:    functionCode,
			RegisterAddress: registerAddr,
			ByteOrder:       byteOrder,
		}

		switch pt {
		case TypeAI, TypeAO:
			p.Value = baseValue * efficient
		case TypeDI, TypeDO:
			p.BoolValue = int64(baseValue) != 0
		case TypePI:
			p.IntValue = int32(baseValue)
		}

		points = append(points, p)
	}

	return points, nil
}

// ValidatePointTable applies protocol constraints after an XLSX file has been parsed.
// LoadFromXLSX already validates the point sheet structure, row data, point types,
// duplicate point-type/IOA pairs, and Modbus register/function-code mappings.
func ValidatePointTable(points []*Point, protocol string) error {
	if len(points) == 0 {
		return fmt.Errorf("point table contains no points")
	}

	isIEC104 := protocol == "iec104" || protocol == "iec104_client"
	for i, point := range points {
		row := i + 2
		if point == nil {
			return fmt.Errorf("row %d: empty point", row)
		}
		if strings.TrimSpace(point.Name) == "" {
			return fmt.Errorf("row %d: point name is required", row)
		}
		if isIEC104 && point.IOA > 0xFFFFFF {
			return fmt.Errorf("row %d: IEC104 IOA %d exceeds the 3-byte limit (16777215)", row, point.IOA)
		}
	}
	return nil
}
