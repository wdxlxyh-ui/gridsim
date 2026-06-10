# 微电网仿真模拟器 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Embed microgrid simulation (PV/Battery/Load/Grid/Diesel/Wind) as a new instance type in GridSim, reusing IEC104/Modbus protocol infrastructure.

**Architecture:** New `internal/microgrid/` package defines topology model + physics engine. Microgrid instances in Manager gain `Microgrid *microgrid.Engine` field and `Protocols []protocol.Protocol` (was single protocol). Protocol factory gains `"microgrid"` case — creates IEC104/Modbus protocol sharing the Store. Web UI extends ConfigPage (protocol selector, topology editor), DetailPage (topology viz, control panel, dashboard), and TrendPage.

**Tech Stack:** Go 1.21+ (microgrid engine), Vue 3 + TypeScript + Element Plus + ECharts (Web UI), library.Store (shared point storage)

**Design Doc:** `docs/superpowers/specs/2026-05-23-microgrid-simulation-design.md`

---

### Task 1: Define microgrid data model

**Files:**
- Create: `internal/microgrid/model.go`

- [ ] **Step 1: Write model.go with all data types**

```go
package microgrid

// ComponentType represents a microgrid component type.
type ComponentType string

const (
	CompGrid      ComponentType = "grid"
	CompPV        ComponentType = "pv"
	CompBattery   ComponentType = "battery"
	CompLoad      ComponentType = "load"
	CompDiesel    ComponentType = "diesel"
	CompWind      ComponentType = "wind"
	CompStaticVar ComponentType = "static_var"
)

// ComponentParams holds per-component type parameters.
type ComponentParams struct {
	RatedPowerKW       float64 `json:"rated_power_kw,omitempty"`
	PanelCount         int     `json:"panel_count,omitempty"`
	Efficiency         float64 `json:"efficiency,omitempty"`
	CapacityKWH        float64 `json:"capacity_kwh,omitempty"`
	InitSOC            float64 `json:"init_soc,omitempty"`
	MaxChargeKW        float64 `json:"max_charge_kw,omitempty"`
	MaxDischargeKW     float64 `json:"max_discharge_kw,omitempty"`
	ChargeEfficiency   float64 `json:"charge_eff,omitempty"`
	DischargeEfficiency float64 `json:"discharge_eff,omitempty"`
	SOCMin             float64 `json:"soc_min,omitempty"`
	SOCMax             float64 `json:"soc_max,omitempty"`
	PowerFactor        float64 `json:"power_factor,omitempty"`
	GridCapacityKW     float64 `json:"grid_capacity_kw,omitempty"`
	IslandMode         bool    `json:"island_mode,omitempty"`
	FuelCapacityL      float64 `json:"fuel_capacity_l,omitempty"`
	FuelLevel          float64 `json:"fuel_level,omitempty"`
	FuelConsumption    float64 `json:"fuel_consumption,omitempty"`
	CutInWindSpeed     float64 `json:"cut_in_wind,omitempty"`
	CutOutWindSpeed    float64 `json:"cut_out_wind,omitempty"`
}

// Component is a microgrid component connected to a bus.
type Component struct {
	ID      string          `json:"id"`
	Type    ComponentType   `json:"type"`
	Name    string          `json:"name"`
	BusID   string          `json:"bus_id"`
	Enabled bool            `json:"enabled"`
	Params  ComponentParams `json:"params"`
}

// Bus is an electrical bus.
type Bus struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	VoltageKV float64 `json:"voltage_kv"`
}

// Connection is a bus-to-bus connection with a breaker.
type Connection struct {
	ID     string `json:"id"`
	BusA   string `json:"bus_a"`
	BusB   string `json:"bus_b"`
	Closed bool   `json:"closed"`
}

// Topology is the full microgrid topology definition.
type Topology struct {
	Buses       []Bus         `json:"buses"`
	Components  []Component   `json:"components"`
	Connections []Connection  `json:"connections"`
}

// PowerBalanceResult holds the result of one power balance calculation.
type PowerBalanceResult struct {
	TotalGenerationKW float64
	TotalLoadKW       float64
	BatteryPowerKW    float64
	GridPowerKW       float64
	ImbalanceKW       float64
	Frequency         float64
	VoltagePU         float64
	LoadShed          bool
	Island            bool
}

// MicrogridConfig is the per-instance configuration stored in InstanceConfig.
type MicrogridConfig struct {
	TopologyJSON    string  `json:"topology_json,omitempty"`
	TickMs          int     `json:"tick_ms,omitempty"`
	SpeedMultiplier float64 `json:"speed_multiplier,omitempty"`
	BaseIOA         uint32  `json:"base_ioa,omitempty"`
	EnvDataFile     string  `json:"env_data_file,omitempty"`
}

const (
	DefaultTickMs  = 1000
	DefaultBaseIOA = 100000
	MaxSpeedMul    = 100
)
```

- [ ] **Step 2: Verify build passes**

Run: `go build ./internal/microgrid/`
Expected: exit 0

- [ ] **Step 3: Commit**

```bash
git add internal/microgrid/model.go
git commit -m "feat(microgrid): add data model types (Component, Bus, Topology, Config)"
```

---

### Task 2: Extend InstanceConfig and protocol factory

**Files:**
- Modify: `internal/model/instance.go` (add MicrogridConfig field, ModbusInstanceConfig stays)
- Modify: `pkg/protocol/factory.go` (add "microgrid" case)
- Modify: `pkg/protocol/protocol.go` (no changes needed — interface unchanged)
- Test: Existing tests should pass

- [ ] **Step 1: Add MicrogridConfig to InstanceConfig**

In `internal/model/instance.go`, after the `ModbusConfig` field:

```go
	MicrogridConfig *MicrogridConfig `json:"microgrid_config,omitempty"`
```

Add import for `"gridsim/internal/microgrid"`. Then add the MicrogridConfig type alias (or re-export):

```go
// MicrogridConfig re-exported for model package convenience.
type MicrogridConfig = microgrid.MicrogridConfig
```

Wait — `model` importing `microgrid` would create a circular dependency because `microgrid` might need `model`. Instead, define the config struct directly in `model/instance.go`:

```go
// MicrogridInstanceConfig is the microgrid-specific instance configuration.
type MicrogridInstanceConfig struct {
	TopologyJSON    string  `json:"topology_json,omitempty"`
	TickMs          int     `json:"tick_ms,omitempty"`
	SpeedMultiplier float64 `json:"speed_multiplier,omitempty"`
	BaseIOA         uint32  `json:"base_ioa,omitempty"`
	EnvDataFile     string  `json:"env_data_file,omitempty"`
}
```

Keep `MicrogridConfig` in the `microgrid` package as the engine-level config, and use `MicrogridInstanceConfig` in `model` for persistence.

- [ ] **Step 2: Extend protocol factory**

In `pkg/protocol/factory.go`, add:

```go
	case "microgrid":
		// microgrid uses its own engine; protocol factory only creates
		// the communication protocol layer if ports are configured.
		if cfg.IEC104Port > 0 {
			return NewIEC104Wrapper(cfg.IEC104Port), nil
		}
		if cfg.ModbusConfig != nil && cfg.ModbusConfig.Port > 0 {
			port := cfg.ModbusConfig.Port
			slaveID := uint8(1)
			byteOrder := "ABCD"
			if cfg.ModbusConfig.SlaveID > 0 {
				slaveID = cfg.ModbusConfig.SlaveID
			}
			if cfg.ModbusConfig.ByteOrder != "" {
				byteOrder = cfg.ModbusConfig.ByteOrder
			}
			return modbus.NewTCPServer(port, slaveID, byteOrder), nil
		}
		return nil, fmt.Errorf("microgrid requires at least one protocol port (iec104 or modbus)")
```

Update `SupportedProtocols()` to include `"microgrid"`:

```go
func SupportedProtocols() []string {
	return []string{"iec104", "modbus_tcp", "microgrid"}
}
```

- [ ] **Step 3: Run tests**

Run: `go build ./...`
Expected: exit 0

Run: `go test ./pkg/protocol/... ./internal/model/...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/model/instance.go pkg/protocol/factory.go
git commit -m "feat(microgrid): extend InstanceConfig + protocol factory for microgrid type"
```

---

### Task 3: Implement topology → point expansion

**Files:**
- Create: `internal/microgrid/pointmap.go`

- [ ] **Step 1: Write point expansion logic**

```go
package microgrid

import (
	"fmt"
	"gridsim/pkg/config"
)

// pointOffsets defines IOA offsets per component type.
// Each component type gets N consecutive IOA slots.
const ioaSlotsPerComponent = 10

// pointTemplate describes one expanded point.
type pointTemplate struct {
	Offset    uint32
	Name      string
	PointType config.PointType
	ValueType string
	Unit      string
}

// componentPointTemplates returns point templates for a given component type.
func componentPointTemplates(ct ComponentType, index int) []pointTemplate {
	switch ct {
	case CompPV:
		return []pointTemplate{
			{1, "Power", config.TypeAI, "FLOAT", "kW"},
			{2, "DailyEnergy", config.TypeAI, "FLOAT", "kWh"},
			{3, "TotalEnergy", config.TypeAI, "FLOAT", "kWh"},
			{4, "Irradiance", config.TypeAI, "FLOAT", "W/m²"},
			{5, "Status", config.TypeDI, "BIT", ""},
		}
	case CompBattery:
		return []pointTemplate{
			{1, "SOC", config.TypeAI, "FLOAT", "%"},
			{2, "Power", config.TypeAI, "FLOAT", "kW"},
			{3, "Voltage", config.TypeAI, "FLOAT", "V"},
			{4, "Current", config.TypeAI, "FLOAT", "A"},
			{5, "Cycles", config.TypeAI, "FLOAT", ""},
			{6, "Temp", config.TypeAI, "FLOAT", "°C"},
			{7, "Status", config.TypeDI, "BIT", ""},
			{8, "ChgState", config.TypeDI, "BIT", ""},
			{9, "PowerSetpoint", config.TypeAO, "FLOAT", "kW"},
		}
	case CompLoad:
		return []pointTemplate{
			{1, "Power", config.TypeAI, "FLOAT", "kW"},
			{2, "Reactive", config.TypeAI, "FLOAT", "kvar"},
			{3, "PF", config.TypeAI, "FLOAT", ""},
			{4, "Switch", config.TypeDO, "BIT", ""},
		}
	case CompGrid:
		return []pointTemplate{
			{1, "GridPower", config.TypeAI, "FLOAT", "kW"},
			{2, "GridReactive", config.TypeAI, "FLOAT", "kvar"},
			{3, "GridVoltage", config.TypeAI, "FLOAT", "kV"},
			{4, "GridFreq", config.TypeAI, "FLOAT", "Hz"},
			{5, "Connected", config.TypeDI, "BIT", ""},
			{6, "Island", config.TypeDI, "BIT", ""},
		}
	case CompDiesel:
		return []pointTemplate{
			{1, "Power", config.TypeAI, "FLOAT", "kW"},
			{2, "FuelLevel", config.TypeAI, "FLOAT", "L"},
			{3, "RPM", config.TypeAI, "FLOAT", "rpm"},
			{4, "RunHours", config.TypeAI, "FLOAT", "h"},
			{5, "StartStop", config.TypeDO, "BIT", ""},
		}
	case CompWind:
		return []pointTemplate{
			{1, "Power", config.TypeAI, "FLOAT", "kW"},
			{2, "WindSpeed", config.TypeAI, "FLOAT", "m/s"},
			{3, "DailyEnergy", config.TypeAI, "FLOAT", "kWh"},
			{4, "TotalEnergy", config.TypeAI, "FLOAT", "kWh"},
			{5, "Status", config.TypeDI, "BIT", ""},
		}
	default:
		return nil
	}
}

// ExpandPoints converts a topology into a flat point list for library.Store.
// baseIOA is the starting IOA (e.g., 100000).
// Each component gets ioaSlotsPerComponent consecutive IOAs.
func ExpandPoints(topology *Topology, baseIOA uint32) []*config.Point {
	var points []*config.Point

	for i, comp := range topology.Components {
		if !comp.Enabled {
			continue
		}
		templates := componentPointTemplates(comp.Type, i)
		compIndex := uint32(i + 1) // 1-based component index

		for _, tmpl := range templates {
			ioa := baseIOA + compIndex*ioaSlotsPerComponent + tmpl.Offset
			pointName := fmt.Sprintf("%s_%s", comp.Name, tmpl.Name)
			pt := &config.Point{
				IOA:       ioa,
				Name:      pointName,
				PointType: tmpl.PointType,
				ValueType: tmpl.ValueType,
				Efficient: 1.0,
				BaseValue: 0,
				Unit:      tmpl.Unit,
			}
			points = append(points, pt)
		}
	}

	return points
}

// FindPointIOA returns the IOA for a specific component+field combination.
// Returns 0 if not found.
func FindPointIOA(topology *Topology, baseIOA uint32, compID string, fieldName string) uint32 {
	for i, comp := range topology.Components {
		if comp.ID != compID {
			continue
		}
		if !comp.Enabled {
			return 0
		}
		templates := componentPointTemplates(comp.Type, i)
		compIndex := uint32(i + 1)
		for _, tmpl := range templates {
			if tmpl.Name == fieldName {
				return baseIOA + compIndex*ioaSlotsPerComponent + tmpl.Offset
			}
		}
	}
	return 0
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./internal/microgrid/`
Expected: exit 0

- [ ] **Step 3: Write a quick unit check**

```go
// internal/microgrid/pointmap_test.go
package microgrid

import (
	"testing"
	"gridsim/pkg/config"
)

func TestExpandPoints_Count(t *testing.T) {
	topo := &Topology{
		Components: []Component{
			{ID: "pv1", Name: "PV1", Type: CompPV, Enabled: true},
			{ID: "bat1", Name: "BAT1", Type: CompBattery, Enabled: true},
		},
	}
	pts := ExpandPoints(topo, 100000)
	// PV → 5 points, Battery → 9 points = 14 total
	if len(pts) != 14 {
		t.Fatalf("expected 14 points, got %d", len(pts))
	}
	// Check IOA pattern
	if pts[0].IOA != 100011 {
		t.Fatalf("expected PV1 Power IOA=100011, got %d", pts[0].IOA)
	}
	if pts[0].PointType != config.TypeAI {
		t.Fatalf("expected PV1 Power type=AI, got %s", pts[0].PointType)
	}
}

func TestFindPointIOA(t *testing.T) {
	topo := &Topology{
		Components: []Component{
			{ID: "bat1", Name: "BAT1", Type: CompBattery, Enabled: true},
		},
	}
	ioa := FindPointIOA(topo, 100000, "bat1", "SOC")
	if ioa != 100011 {
		t.Fatalf("expected BAT1 SOC IOA=100011, got %d", ioa)
	}
}
```

Run: `go test ./internal/microgrid/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/microgrid/pointmap.go internal/microgrid/pointmap_test.go
git commit -m "feat(microgrid): topology-to-point expansion with IOA allocation"
```

---

### Task 4: Implement simulation engine — core tick loop

**Files:**
- Create: `internal/microgrid/engine.go`
- Create: `internal/microgrid/powerflow.go` (power balance calculation)
- Create: `internal/microgrid/env.go` (environment data driver)

- [ ] **Step 1: Write engine.go — main simulation loop**

```go
package microgrid

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"gridsim/pkg/config"
	"gridsim/pkg/library"
)

// Engine runs the microgrid simulation tick loop.
type Engine struct {
	mu          sync.RWMutex
	topology    *Topology
	store       *library.Store
	cfg         MicrogridConfig
	baseIOA     uint32
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	running     bool
	startTime   time.Time
	envProvider EnvProvider

	// Runtime state (per-component mutable values)
	pvPower       map[string]float64
	loadPower     map[string]float64
	windPower     map[string]float64
	dieselRunning map[string]bool
	soc           map[string]float64
	energy        map[string]float64
	gridIsland    bool
}

// EnvProvider supplies environmental data to the engine.
type EnvProvider interface {
	GetIrradiance(componentID string) float64
	GetWindSpeed(componentID string) float64
	GetTemperature(componentID string) float64
}

// NewEngine creates a microgrid simulation engine.
func NewEngine(topology *Topology, store *library.Store, cfg MicrogridConfig) *Engine {
	baseIOA := cfg.BaseIOA
	if baseIOA == 0 {
		baseIOA = DefaultBaseIOA
	}

	e := &Engine{
		topology:      topology,
		store:         store,
		cfg:           cfg,
		baseIOA:       baseIOA,
		pvPower:       make(map[string]float64),
		loadPower:     make(map[string]float64),
		windPower:     make(map[string]float64),
		dieselRunning: make(map[string]bool),
		soc:           make(map[string]float64),
		energy:        make(map[string]float64),
	}

	// Initialize SOC from component params
	for _, comp := range topology.Components {
		if comp.Type == CompBattery {
			e.soc[comp.ID] = comp.Params.InitSOC
		}
	}

	return e
}

// SetEnvProvider sets the environment data provider.
func (e *Engine) SetEnvProvider(p EnvProvider) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.envProvider = p
}

// Start begins the simulation loop.
func (e *Engine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return nil
	}

	tickMs := e.cfg.TickMs
	if tickMs <= 0 {
		tickMs = DefaultTickMs
	}
	// Apply speed multiplier
	period := time.Duration(tickMs) * time.Millisecond
	if e.cfg.SpeedMultiplier > 1 && e.cfg.SpeedMultiplier <= MaxSpeedMul {
		period = time.Duration(float64(tickMs)/e.cfg.SpeedMultiplier) * time.Millisecond
	}
	if period < 50*time.Millisecond {
		period = 50 * time.Millisecond
	}

	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	e.running = true
	e.startTime = time.Now()

	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		ticker := time.NewTicker(period)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				e.tick()
			}
		}
	}()

	slog.Info("微电网仿真引擎已启动", "period_ms", period.Milliseconds())
	return nil
}

// Stop stops the simulation loop.
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return
	}
	e.cancel()
	e.wg.Wait()
	e.running = false
	slog.Info("微电网仿真引擎已停止")
}

// IsRunning returns whether the engine is running.
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// Uptime returns seconds since start.
func (e *Engine) Uptime() int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if !e.running {
		return 0
	}
	return int64(time.Since(e.startTime).Seconds())
}

// Topology returns the current topology (read-only).
func (e *Engine) Topology() *Topology {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.topology
}

// GetSOC returns the current SOC for a battery component.
func (e *Engine) GetSOC(compID string) (float64, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	v, ok := e.soc[compID]
	return v, ok
}

// SetIsland toggles island mode.
func (e *Engine) SetIsland(island bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.gridIsland = island
	// Update the Island DI point
	ioa := FindPointIOA(e.topology, e.baseIOA, e.findGridID(), "Island")
	if ioa > 0 {
		if p, ok := e.store.Get(ioa); ok {
			p.BoolValue = island
			e.store.SetBoolValue(ioa, island)
		}
	}
	// Update Connected DI point
	connIOA := FindPointIOA(e.topology, e.baseIOA, e.findGridID(), "Connected")
	if connIOA > 0 {
		e.store.SetBoolValue(connIOA, !island)
	}
}

func (e *Engine) findGridID() string {
	for _, comp := range e.topology.Components {
		if comp.Type == CompGrid {
			return comp.ID
		}
	}
	return ""
}

// tick runs one simulation iteration.
func (e *Engine) tick() {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 1. Gather environmental data
	irradiance := 1000.0  // default full sun
	windSpeed := 5.0      // default moderate wind
	if e.envProvider != nil {
		irradiance = e.envProvider.GetIrradiance("")
		windSpeed = e.envProvider.GetWindSpeed("")
	}

	// 2. Calculate each component's power
	totalGen := 0.0
	totalLoad := 0.0
	batPower := 0.0

	for _, comp := range e.topology.Components {
		if !comp.Enabled {
			continue
		}
		switch comp.Type {
		case CompPV:
			// P = irradiance * rated_power * efficiency / 1000
			power := (irradiance / 1000.0) * comp.Params.RatedPowerKW * comp.Params.Efficiency
			if power < 0 {
				power = 0
			}
			e.pvPower[comp.ID] = power
			totalGen += power

		case CompWind:
			power := 0.0
			if windSpeed >= comp.Params.CutInWindSpeed && windSpeed < comp.Params.CutOutWindSpeed {
				// Simplified power curve: quadratic between cut-in and rated
				ratio := (windSpeed - comp.Params.CutInWindSpeed) / (12.0 - comp.Params.CutInWindSpeed)
				if ratio > 1.0 {
					ratio = 1.0
				}
				power = ratio * comp.Params.RatedPowerKW
			}
			e.windPower[comp.ID] = power
			totalGen += power

		case CompDiesel:
			if e.dieselRunning[comp.ID] {
				power := comp.Params.RatedPowerKW * 0.8 // assume 80% load
				totalGen += power
			}

		case CompLoad:
			power := comp.Params.RatedPowerKW * comp.Params.PowerFactor
			e.loadPower[comp.ID] = power
			totalLoad += power

		case CompBattery:
			// Read AO setpoint from store
			spIOA := FindPointIOA(e.topology, e.baseIOA, comp.ID, "PowerSetpoint")
			if spIOA > 0 {
				if p, ok := e.store.Get(spIOA); ok {
					batPower = p.Value
				}
			}
			// Clamp by max charge/discharge
			if batPower > comp.Params.MaxDischargeKW {
				batPower = comp.Params.MaxDischargeKW
			}
			if batPower < -comp.Params.MaxChargeKW {
				batPower = -comp.Params.MaxChargeKW
			}
			totalGen += batPower // positive = discharge (generation)
		}
	}

	// 3. Power balance
	island := e.gridIsland
	gridCapacity := 0.0
	for _, comp := range e.topology.Components {
		if comp.Type == CompGrid {
			gridCapacity = comp.Params.GridCapacityKW
		}
	}

	imbalance := totalGen - totalLoad
	gridPower := 0.0
	loadShed := false

	if !island {
		if imbalance > gridCapacity {
			gridPower = gridCapacity
		} else {
			gridPower = imbalance
		}
	} else {
		// Island mode: must self-balance
		if imbalance < -1e-6 {
			// Not enough generation — try battery first
			// If battery can't cover, shed load
			availableBat := 0.0
			for _, comp := range e.topology.Components {
				if comp.Type == CompBattery && comp.Enabled {
					maxDchg := comp.Params.MaxDischargeKW
					if e.soc[comp.ID] > comp.Params.SOCMin {
						availableBat += maxDchg
					}
				}
			}
			if -imbalance > availableBat {
				loadShed = true
			}
		}
		gridPower = 0
	}

	// 4. Update SOC (battery energy integration)
	dt := float64(e.cfg.TickMs) / 1000.0 / 3600.0 // hours per tick
	for _, comp := range e.topology.Components {
		if comp.Type == CompBattery && comp.Enabled {
			soc := e.soc[comp.ID]
			// batPower > 0 = discharge, soc decreases
			// batPower < 0 = charge, soc increases
			energyDelta := batPower * dt // kWh
			if batPower > 0 {
				energyDelta = energyDelta / comp.Params.DischargeEfficiency
			} else {
				energyDelta = energyDelta * comp.Params.ChargeEfficiency
			}
			socDelta := (energyDelta / comp.Params.CapacityKWH) * 100.0
			soc -= socDelta
			if soc < comp.Params.SOCMin {
				soc = comp.Params.SOCMin
			}
			if soc > comp.Params.SOCMax {
				soc = comp.Params.SOCMax
			}
			e.soc[comp.ID] = soc
		}
	}

	// 5. Frequency simulation (simplified droop)
	freq := 50.0
	if totalGen > 0 {
		freq = 50.0 + (imbalance/totalGen)*0.5
	}

	// 6. Write all values to store
	for _, comp := range e.topology.Components {
		if !comp.Enabled {
			continue
		}
		switch comp.Type {
		case CompPV:
			e.writeAI(comp.ID, "Power", e.pvPower[comp.ID])
			e.writeDI(comp.ID, "Status", e.pvPower[comp.ID] > 0.1)
		case CompWind:
			e.writeAI(comp.ID, "Power", e.windPower[comp.ID])
			e.writeAI(comp.ID, "WindSpeed", windSpeed)
			e.writeDI(comp.ID, "Status", e.windPower[comp.ID] > 0.1)
		case CompDiesel:
			power := 0.0
			if e.dieselRunning[comp.ID] {
				power = comp.Params.RatedPowerKW * 0.8
			}
			e.writeAI(comp.ID, "Power", power)
		case CompLoad:
			e.writeAI(comp.ID, "Power", e.loadPower[comp.ID])
			e.writeAI(comp.ID, "Reactive", e.loadPower[comp.ID]*0.3) // simplified
			e.writeAI(comp.ID, "PF", comp.Params.PowerFactor)
		case CompGrid:
			e.writeAI(comp.ID, "GridPower", gridPower)
			e.writeAI(comp.ID, "GridFreq", freq)
			e.writeDI(comp.ID, "Connected", !island)
			e.writeDI(comp.ID, "Island", island)
		case CompBattery:
			e.writeAI(comp.ID, "SOC", e.soc[comp.ID])
			e.writeAI(comp.ID, "Power", batPower)
			chgState := 0.0
			if batPower > 0.1 {
				chgState = 2.0 // discharging
			} else if batPower < -0.1 {
				chgState = 1.0 // charging
			}
			e.writeDI(comp.ID, "ChgState", chgState > 0.5)
		}
	}
}

// writeAI writes an AI point value.
func (e *Engine) writeAI(compID, field string, value float64) {
	ioa := FindPointIOA(e.topology, e.baseIOA, compID, field)
	if ioa > 0 {
		e.store.SetValue(ioa, value)
	}
}

// writeDI writes a DI point value.
func (e *Engine) writeDI(compID, field string, value bool) {
	ioa := FindPointIOA(e.topology, e.baseIOA, compID, field)
	if ioa > 0 {
		e.store.SetBoolValue(ioa, value)
	}
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./internal/microgrid/`
Expected: exit 0

We need `SetBoolValue` on `library.Store`. Check if it exists:

Run: `grep -n "SetBoolValue\|SetValue" pkg/library/store.go`
Expected: See existing methods — add SetBoolValue if it doesn't exist.

If `SetBoolValue` doesn't exist, in `pkg/library/store.go`:

```go
// SetBoolValue sets a point's bool value.
func (s *Store) SetBoolValue(ioa uint32, val bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.points[ioa]; ok {
		p.BoolValue = val
		p.UpdatedAt = time.Now()
	}
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/microgrid/engine.go pkg/library/store.go
git commit -m "feat(microgrid): simulation engine with PV/Wind/Battery/Load/Grid power balance"
```

---

### Task 5: Extend Manager to support microgrid instances

**Files:**
- Modify: `internal/manager/manager.go`
- Modify: `cmd/gridsim/main.go`

- [ ] **Step 1: Modify Instance struct to support multiple protocols and microgrid engine**

In `internal/manager/manager.go`:

```go
type Instance struct {
	Config     model.InstanceConfig
	Protocols  []protocol.Protocol  // was single Protocol
	Store      *library.Store
	HTTPServer *http.Server
	AutoEngine *detail.Engine
	Microgrid  *microgrid.Engine    // new: microgrid simulation engine
	Logger     *InstanceLogger
}
```

Rename `inst.Protocol` references throughout to use `inst.Protocols[0]` for backward compatibility, or add a helper:

```go
func (inst *Instance) PrimaryProtocol() protocol.Protocol {
	if len(inst.Protocols) == 0 {
		return nil
	}
	return inst.Protocols[0]
}
```

In `StartInstance`, after the existing protocol creation block, add microgrid branching:

```go
var protocols []protocol.Protocol

if cfg.Protocol == "microgrid" {
	// Microgrid instance: create engine first, then optional protocol(s)
	topo, err := microgrid.ParseTopology(cfg.MicrogridConfig.TopologyJSON)
	if err != nil {
		return fmt.Errorf("parse topology: %w", err)
	}

	points := microgrid.ExpandPoints(topo, cfg.MicrogridConfig.BaseIOA)
	store := library.NewStore(points)

	mgCfg := microgrid.MicrogridConfig{
		TickMs:          cfg.MicrogridConfig.TickMs,
		SpeedMultiplier: cfg.MicrogridConfig.SpeedMultiplier,
		BaseIOA:         cfg.MicrogridConfig.BaseIOA,
	}
	mcEngine := microgrid.NewEngine(topo, store, mgCfg)

	// Create protocol(s) for external access
	if cfg.IEC104Port > 0 {
		proto := protocol.NewIEC104Wrapper(cfg.IEC104Port)
		proto.SetStore(store)
		if err := proto.Start(); err != nil {
			return fmt.Errorf("start iec104: %w", err)
		}
		protocols = append(protocols, proto)
	}
	if cfg.ModbusConfig != nil && cfg.ModbusConfig.Port > 0 {
		// create modbus protocol
		modProto, err := protocol.New(cfg)
		if err != nil {
			return fmt.Errorf("create modbus: %w", err)
		}
		modProto.SetStore(store)
		if err := modProto.Start(); err != nil {
			return fmt.Errorf("start modbus: %w", err)
		}
		protocols = append(protocols, modProto)
	}

	// Create auto-change engine sharing the same store
	var pub protocol.Protocol
	if len(protocols) > 0 {
		pub = protocols[0]
	}
	acStore := detail.NewAutoChangeStore(m.cfgDir)
	engine := detail.NewEngine(cfg.ID, store, pub, acStore, m.cfgDir, m)
	if err := engine.LoadAndStart(); err != nil {
		slog.Warn("自动变化引擎加载失败", "id", id, "error", err)
	}

	// Start microgrid engine
	if err := mcEngine.Start(); err != nil {
		return fmt.Errorf("start microgrid: %w", err)
	}

	inst := &Instance{
		Config:     cfg,
		Protocols:  protocols,
		Store:      store,
		AutoEngine: engine,
		Microgrid:  mcEngine,
	}
	m.instances[id] = inst
	slog.Info("微电网实例已启动", "id", id, "protocols", len(protocols), "points", len(points))
	return nil
}

// Original path for non-microgrid instances
proto, err := protocol.New(cfg)
// ... existing logic wraps result in []protocol.Protocol{proto}
```

Update `StopInstance`, `DeleteConfig`, `StopAll` to also stop `inst.Microgrid`:

```go
if inst.Microgrid != nil {
	inst.Microgrid.Stop()
}
```

Update `GetEngine` to also work for microgrid instances (they have AutoEngine too).

Add `GetMicrogrid(id string) *microgrid.Engine`:

```go
func (m *Manager) GetMicrogrid(id string) *microgrid.Engine {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if inst, ok := m.instances[id]; ok {
		return inst.Microgrid
	}
	return nil
}
```

- [ ] **Step 2: Add ParseTopology helper in microgrid package**

```go
// internal/microgrid/model.go — add:
func ParseTopology(jsonStr string) (*Topology, error) {
	if jsonStr == "" {
		return &Topology{}, nil // empty/default topology
	}
	var topo Topology
	if err := json.Unmarshal([]byte(jsonStr), &topo); err != nil {
		return nil, fmt.Errorf("parse topology json: %w", err)
	}
	return &topo, nil
}

func (t *Topology) ToJSON() (string, error) {
	data, err := json.Marshal(t)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
```

- [ ] **Step 3: Add microgrid API routing in cmd/gridsim/main.go**

In `registerRoutes`, add:

```go
mux.HandleFunc("/api/v1/microgrid/", ws.handleMicrogridRoutes)
```

Implement the handler:

```go
func (ws *webServer) handleMicrogridRoutes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/microgrid/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "missing instance ID")
		return
	}
	id := parts[0]
	mg := ws.mgr.GetMicrogrid(id)
	if mg == nil {
		writeError(w, http.StatusNotFound, "microgrid instance not found or not running")
		return
	}

	if len(parts) == 1 {
		// GET /api/v1/microgrid/{id} — dashboard summary
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"running":  mg.IsRunning(),
			"uptime":   mg.Uptime(),
			"topology": mg.Topology(),
		})
		return
	}

	switch parts[1] {
	case "topology":
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, mg.Topology())
		case http.MethodPut:
			var newTopo microgrid.Topology
			if err := json.NewDecoder(r.Body).Decode(&newTopo); err != nil {
				writeError(w, http.StatusBadRequest, "invalid topology JSON")
				return
			}
			// TODO: update topology at runtime (v2 feature)
			writeError(w, http.StatusNotImplemented, "runtime topology update not yet supported")
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}

	case "switch":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var req struct {
			Island bool `json:"island"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		mg.SetIsland(req.Island)
		writeJSON(w, http.StatusOK, map[string]bool{"island": req.Island})

	case "dashboard":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		topo := mg.Topology()
		dash := map[string]interface{}{
			"running": mg.IsRunning(),
			"uptime":  mg.Uptime(),
		}
		// Collect key metrics
		for _, comp := range topo.Components {
			if !comp.Enabled {
				continue
			}
			if comp.Type == microgrid.CompBattery {
				if soc, ok := mg.GetSOC(comp.ID); ok {
					dash["soc"] = soc
				}
			}
		}
		writeJSON(w, http.StatusOK, dash)

	default:
		writeError(w, http.StatusBadRequest, "unknown microgrid action: "+parts[1])
	}
}
```

- [ ] **Step 4: Build and test**

Run: `go build ./...`
Expected: exit 0

Run: `go test ./internal/microgrid/... ./internal/manager/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/microgrid/ internal/manager/manager.go cmd/gridsim/main.go
git commit -m "feat(microgrid): Manager integration + HTTP API routes for microgrid instances"
```

---

### Task 6: CSV environment data provider

**Files:**
- Create: `internal/microgrid/env.go`

- [ ] **Step 1: Implement CSV env provider**

```go
package microgrid

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

type csvEnvRow struct {
	timestamp    time.Duration // relative to start
	irradiance   float64
	windSpeed    float64
	temperature  float64
}

// CSVEnvProvider reads environmental data from a CSV file.
type CSVEnvProvider struct {
	mu       sync.RWMutex
	rows     []csvEnvRow
	index    int
	startAt  time.Time
	looping  bool
	duration time.Duration
}

// NewCSVEnvProvider creates a provider from a CSV file.
// CSV format: time,irradiance_wm2,wind_speed_ms,temperature_c
// Time can be "hh:mm:ss" (absolute) or "0s/0ms" (relative).
func NewCSVEnvProvider(csvPath string) (*CSVEnvProvider, error) {
	f, err := os.Open(csvPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var rows []csvEnvRow
	for i, record := range records {
		if i == 0 {
			continue // header
		}
		if len(record) < 4 {
			continue
		}
		var row csvEnvRow
		// Parse time
		if len(record[0]) <= 8 {
			// hh:mm:ss format
			t, err := time.Parse("15:04:05", record[0])
			if err != nil {
				continue
			}
			row.timestamp = time.Duration(t.Hour())*time.Hour +
				time.Duration(t.Minute())*time.Minute +
				time.Duration(t.Second())*time.Second
		} else {
			d, err := time.ParseDuration(record[0])
			if err != nil {
				continue
			}
			row.timestamp = d
		}
		row.irradiance, _ = strconv.ParseFloat(record[1], 64)
		row.windSpeed, _ = strconv.ParseFloat(record[2], 64)
		row.temperature, _ = strconv.ParseFloat(record[3], 64)

		rows = append(rows, row)
	}

	if len(rows) == 0 {
		return nil, os.ErrInvalid
	}

	duration := rows[len(rows)-1].timestamp - rows[0].timestamp

	return &CSVEnvProvider{
		rows:     rows,
		looping:  true,
		duration: duration,
	}, nil
}

// Start begins the CSV playback.
func (p *CSVEnvProvider) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.startAt = time.Now()
	p.index = 0
}

// GetIrradiance returns the current irradiance interpolated from CSV data.
func (p *CSVEnvProvider) GetIrradiance(componentID string) float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, irradiance, _, _ := p.current()
	return irradiance
}

// GetWindSpeed returns the current wind speed.
func (p *CSVEnvProvider) GetWindSpeed(componentID string) float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, _, wind, _ := p.current()
	return wind
}

// GetTemperature returns the current temperature.
func (p *CSVEnvProvider) GetTemperature(componentID string) float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, _, _, temp := p.current()
	return temp
}

func (p *CSVEnvProvider) current() (time.Duration, float64, float64, float64) {
	if len(p.rows) == 0 {
		return 0, 1000, 5, 25
	}
	elapsed := time.Since(p.startAt)
	if p.looping && p.duration > 0 {
		elapsed = elapsed % p.duration
	}

	// Find the two nearest rows and interpolate
	for i := len(p.rows) - 1; i >= 0; i-- {
		if elapsed >= p.rows[i].timestamp {
			if i >= len(p.rows)-1 {
				r := p.rows[i]
				return r.timestamp, r.irradiance, r.windSpeed, r.temperature
			}
			// Linear interpolation
			r0 := p.rows[i]
			r1 := p.rows[i+1]
			dt := float64(r1.timestamp - r0.timestamp)
			if dt == 0 {
				return r0.timestamp, r0.irradiance, r0.windSpeed, r0.temperature
			}
			t := float64(elapsed-r0.timestamp) / dt
			irr := r0.irradiance + t*(r1.irradiance-r0.irradiance)
			wind := r0.windSpeed + t*(r1.windSpeed-r0.windSpeed)
			temp := r0.temperature + t*(r1.temperature-r0.temperature)
			return elapsed, irr, wind, temp
		}
	}

	r := p.rows[0]
	return r.timestamp, r.irradiance, r.windSpeed, r.temperature
}
```

- [ ] **Step 2: Wire env provider into engine startup**

In `Manager.StartInstance` microgrid branch, after creating the engine:

```go
// Wire up environment data provider
if cfg.MicrogridConfig.EnvDataFile != "" {
	envPath := filepath.Join(m.cfgDir, cfg.MicrogridConfig.EnvDataFile)
	envProvider, err := microgrid.NewCSVEnvProvider(envPath)
	if err == nil {
		envProvider.Start()
		mcEngine.SetEnvProvider(envProvider)
		slog.Info("微电网环境数据已加载", "file", cfg.MicrogridConfig.EnvDataFile)
	} else {
		slog.Warn("微电网环境数据加载失败", "file", envPath, "error", err)
	}
}
```

- [ ] **Step 3: Build and test**

Run: `go build ./...`
Expected: exit 0

- [ ] **Step 4: Commit**

```bash
git add internal/microgrid/env.go
git commit -m "feat(microgrid): CSV environment data provider for irradiance/wind/temperature"
```

---

### Task 7: History buffer for trend data

**Files:**
- Create: `internal/microgrid/history.go`

- [ ] **Step 1: Implement ring-buffer history**

```go
package microgrid

import (
	"sync"
	"time"
)

const defaultHistoryMaxFrames = 3600

// HistoryFrame is a snapshot of key metrics at one point in time.
type HistoryFrame struct {
	Timestamp time.Time            `json:"timestamp"`
	Values    map[string]float64   `json:"values"`
}

// HistoryBuffer is a thread-safe ring buffer for simulation history.
type HistoryBuffer struct {
	mu      sync.RWMutex
	frames  []HistoryFrame
	maxSize int
	cursor  int
	wrapped bool
}

// NewHistoryBuffer creates a history buffer with the given capacity.
func NewHistoryBuffer(maxSize int) *HistoryBuffer {
	if maxSize <= 0 {
		maxSize = defaultHistoryMaxFrames
	}
	return &HistoryBuffer{
		frames:  make([]HistoryFrame, maxSize),
		maxSize: maxSize,
	}
}

// Push adds a frame to the buffer.
func (hb *HistoryBuffer) Push(frame HistoryFrame) {
	hb.mu.Lock()
	defer hb.mu.Unlock()
	hb.frames[hb.cursor] = frame
	hb.cursor++
	if hb.cursor >= hb.maxSize {
		hb.cursor = 0
		hb.wrapped = true
	}
}

// Recent returns the most recent N frames (up to maxSize).
func (hb *HistoryBuffer) Recent(n int) []HistoryFrame {
	hb.mu.RLock()
	defer hb.mu.RUnlock()

	if n <= 0 || n > hb.maxSize {
		n = hb.maxSize
	}

	count := hb.cursor
	if hb.wrapped {
		count = hb.maxSize
	}
	if n > count {
		n = count
	}

	result := make([]HistoryFrame, n)
	if hb.wrapped {
		start := (hb.cursor - n + hb.maxSize) % hb.maxSize
		for i := 0; i < n; i++ {
			result[i] = hb.frames[(start+i)%hb.maxSize]
		}
	} else {
		copy(result, hb.frames[hb.cursor-n:hb.cursor])
	}
	return result
}

// All returns all stored frames in chronological order.
func (hb *HistoryBuffer) All() []HistoryFrame {
	return hb.Recent(hb.maxSize)
}

// Len returns the number of frames stored.
func (hb *HistoryBuffer) Len() int {
	hb.mu.RLock()
	defer hb.mu.RUnlock()
	if hb.wrapped {
		return hb.maxSize
	}
	return hb.cursor
}
```

- [ ] **Step 2: Integrate history into engine**

In `microgrid.Engine`, add:

```go
type Engine struct {
	// ... existing fields ...
	history *HistoryBuffer
}

// in NewEngine:
e.history = NewHistoryBuffer(0)

// At the end of tick(), after writing values to store:
frame := HistoryFrame{
	Timestamp: time.Now(),
	Values:    make(map[string]float64),
}
for _, comp := range e.topology.Components {
	if !comp.Enabled {
		continue
	}
	switch comp.Type {
	case CompPV:
		frame.Values[comp.ID+"_power"] = e.pvPower[comp.ID]
	case CompBattery:
		frame.Values[comp.ID+"_soc"] = e.soc[comp.ID]
		frame.Values[comp.ID+"_power"] = batPower
	case CompLoad:
		frame.Values[comp.ID+"_power"] = e.loadPower[comp.ID]
	case CompGrid:
		frame.Values["grid_power"] = gridPower
		frame.Values["grid_freq"] = freq
	}
}
e.history.Push(frame)
```

Add `History()` accessor:

```go
func (e *Engine) History(n int) []HistoryFrame {
	return e.history.Recent(n)
}
```

- [ ] **Step 3: Add history API handler**

In `cmd/gridsim/main.go` `handleMicrogridRoutes`, add case:

```go
	case "history":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		n := 300 // default
		if nStr := r.URL.Query().Get("n"); nStr != "" {
			if parsed, err := strconv.Atoi(nStr); err == nil && parsed > 0 {
				n = parsed
			}
		}
		writeJSON(w, http.StatusOK, mg.History(n))
```

- [ ] **Step 4: Build**

Run: `go build ./...`
Expected: exit 0

- [ ] **Step 5: Commit**

```bash
git add internal/microgrid/history.go
git commit -m "feat(microgrid): ring-buffer history for trend data + API endpoint"
```

---

### Task 8: MCP tool extensions

**Files:**
- Modify: `internal/mcp/server.go`

- [ ] **Step 1: Add microgrid MCP tools**

In the MCP tool list, add:

```go
// create_microgrid
"create_microgrid": mcp.NewTool("create_microgrid",
	mcp.WithDescription("Create a new microgrid simulation instance"),
	mcp.WithString("name", mcp.Required(), mcp.Description("Instance name")),
	mcp.WithString("topology", mcp.Required(), mcp.Description("Microgrid topology JSON")),
	mcp.WithNumber("iec104_port", mcp.Description("IEC104 port (optional)")),
	mcp.WithNumber("tick_ms", mcp.Description("Simulation tick in ms, default 1000")),
),

// config_microgrid_topology
"config_microgrid_topology": mcp.NewTool("config_microgrid_topology",
	mcp.WithDescription("Update microgrid topology configuration"),
	mcp.WithString("instance_id", mcp.Required(), mcp.Description("Instance ID")),
	mcp.WithString("topology", mcp.Required(), mcp.Description("Topology JSON")),
),

// microgrid_switch
"microgrid_switch": mcp.NewTool("microgrid_switch",
	mcp.WithDescription("Toggle microgrid island mode"),
	mcp.WithString("instance_id", mcp.Required(), mcp.Description("Instance ID")),
	mcp.WithBool("island", mcp.Required(), mcp.Description("Island mode")),
),

// get_microgrid_dashboard
"get_microgrid_dashboard": mcp.NewTool("get_microgrid_dashboard",
	mcp.WithDescription("Get microgrid dashboard data"),
	mcp.WithString("instance_id", mcp.Required(), mcp.Description("Instance ID")),
),
```

Implement handlers in the MCP server's tool dispatch function:

```go
func (s *GridSimMCP) handleCreateMicrogrid(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	name, _ := args["name"].(string)
	topologyJSON, _ := args["topology"].(string)
	iec104Port, _ := args["iec104_port"].(float64)
	tickMs, _ := args["tick_ms"].(float64)

	cfg := model.InstanceConfig{
		Name:       name,
		Protocol:   "microgrid",
		IEC104Port: int(iec104Port),
		MicrogridConfig: &model.MicrogridInstanceConfig{
			TopologyJSON: topologyJSON,
			TickMs:       int(tickMs),
		},
	}
	if cfg.MicrogridConfig.TickMs == 0 {
		cfg.MicrogridConfig.TickMs = 1000
	}

	created, err := s.mgr.CreateConfig(cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText("Microgrid created: "+created.ID), nil
}

func (s *GridSimMCP) handleMicrogridSwitch(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	id, _ := args["instance_id"].(string)
	island, _ := args["island"].(bool)
	mg := s.mgr.GetMicrogrid(id)
	if mg == nil {
		return mcp.NewToolResultError("microgrid instance not found"), nil
	}
	mg.SetIsland(island)
	return mcp.NewToolResultText(fmt.Sprintf("Microgrid %s island mode: %v", id, island)), nil
}

func (s *GridSimMCP) handleGetMicrogridDashboard(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	id, _ := args["instance_id"].(string)
	mg := s.mgr.GetMicrogrid(id)
	if mg == nil {
		return mcp.NewToolResultError("microgrid instance not found"), nil
	}
	topo := mg.Topology()
	result := map[string]interface{}{
		"running": mg.IsRunning(),
		"uptime":  mg.Uptime(),
	}
	for _, comp := range topo.Components {
		if comp.Type == microgrid.CompBattery && comp.Enabled {
			if soc, ok := mg.GetSOC(comp.ID); ok {
				result["soc_"+comp.ID] = soc
			}
		}
	}
	data, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(data)), nil
}
```

- [ ] **Step 2: Build**

Run: `go build ./...`
Expected: exit 0

- [ ] **Step 3: Commit**

```bash
git add internal/mcp/server.go
git commit -m "feat(microgrid): MCP tools (create_microgrid, microgrid_switch, dashboard)"
```

---

### Task 9: Frontend — ConfigPage protocol option

**Files:**
- Modify: `web/src/views/ConfigPage.vue`
- Modify: `web/src/api/index.ts`

- [ ] **Step 1: Add "microgrid" protocol option to ConfigPage**

In the protocol select options:

```html
<el-option label="IEC104" value="iec104" />
<el-option label="Modbus TCP" value="modbus_tcp" />
<el-option label="微电网仿真" value="microgrid" />  <!-- add this -->
```

When `protocol === 'microgrid'`, show additional fields:

```html
<el-form-item v-if="form.protocol === 'microgrid'" label="仿真步长(ms)">
  <el-input-number v-model="form.microgrid_tick_ms" :min="100" :step="100" :max="5000" />
</el-form-item>
<el-form-item v-if="form.protocol === 'microgrid'" label="加速比">
  <el-input-number v-model="form.microgrid_speed" :min="1" :max="100" :step="1" />
</el-form-item>
<el-form-item v-if="form.protocol === 'microgrid'" label="测点基地址">
  <el-input-number v-model="form.microgrid_base_ioa" :min="1" :step="1" />
</el-form-item>
<el-form-item v-if="form.protocol === 'microgrid'" label="拓扑配置">
  <el-button @click="openTopologyEditor">编辑拓扑</el-button>
</el-form-item>
```

Add `form.microgrid_tick_ms`, `form.microgrid_speed`, `form.microgrid_base_ioa` to the form data model.

Before submit, serialize topology JSON and set `microgrid_config.topology_json`.

- [ ] **Step 2: Extend frontend API client**

In `web/src/api/index.ts`:

```typescript
// Microgrid API
export function getMicrogridDashboard(id: string): Promise<AxiosResponse> {
  return api.get(`/api/v1/microgrid/${id}/dashboard`)
}

export function getMicrogridTopology(id: string): Promise<AxiosResponse> {
  return api.get(`/api/v1/microgrid/${id}/topology`)
}

export function getMicrogridHistory(id: string, n = 300): Promise<AxiosResponse> {
  return api.get(`/api/v1/microgrid/${id}/history?n=${n}`)
}

export function switchMicrogridIsland(id: string, island: boolean): Promise<AxiosResponse> {
  return api.post(`/api/v1/microgrid/${id}/switch`, { island })
}
```

- [ ] **Step 3: Build frontend**

Run: `cd web && npm run build`
Expected: exit 0

- [ ] **Step 4: Commit**

```bash
git add web/src/views/ConfigPage.vue web/src/api/index.ts
git commit -m "feat(microgrid): ConfigPage protocol option + frontend API client"
```

---

### Task 10: Frontend — Microgrid topology editor

**Files:**
- Create: `web/src/views/MicrogridEditor.vue`

- [ ] **Step 1: Create topology editor component**

A dialog-based editor that allows adding/removing components and buses.

```vue
<template>
  <el-dialog v-model="visible" title="微电网拓扑编辑" width="700px">
    <el-tabs>
      <el-tab-pane label="母线">
        <el-button @click="addBus" size="small">+ 添加母线</el-button>
        <el-table :data="topology.buses" size="small">
          <el-table-column label="名称">
            <template #default="{ row, $index }">
              <el-input v-model="row.name" size="small" />
            </template>
          </el-table-column>
          <el-table-column label="电压(kV)" width="120">
            <template #default="{ row }">
              <el-input-number v-model="row.voltage_kv" :min="0.1" :step="0.1" size="small" />
            </template>
          </el-table-column>
          <el-table-column label="操作" width="80">
            <template #default="{ $index }">
              <el-button @click="removeBus($index)" type="danger" size="small">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
      <el-tab-pane label="元件">
        <el-button @click="addComponent" size="small">+ 添加元件</el-button>
        <el-table :data="topology.components" size="small">
          <el-table-column label="名称" width="120">
            <template #default="{ row }">
              <el-input v-model="row.name" size="small" />
            </template>
          </el-table-column>
          <el-table-column label="类型" width="120">
            <template #default="{ row }">
              <el-select v-model="row.type" size="small">
                <el-option label="光伏" value="pv" />
                <el-option label="储能" value="battery" />
                <el-option label="负荷" value="load" />
                <el-option label="电网" value="grid" />
                <el-option label="柴发" value="diesel" />
                <el-option label="风电" value="wind" />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="挂接母线" width="120">
            <template #default="{ row }">
              <el-select v-model="row.bus_id" size="small">
                <el-option v-for="bus in topology.buses" :key="bus.id" :label="bus.name" :value="bus.id" />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="额定功率(kW)" width="130">
            <template #default="{ row }">
              <el-input-number v-model="row.params.rated_power_kw" :min="0" :step="10" size="small" />
            </template>
          </el-table-column>
          <el-table-column label="操作" width="60">
            <template #default="{ $index }">
              <el-button @click="removeComponent($index)" type="danger" size="small">×</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
    </el-tabs>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button @click="save" type="primary">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'

interface Bus { id: string; name: string; voltage_kv: number }
interface ComponentParams { rated_power_kw?: number; capacity_kwh?: number; init_soc?: number; [key: string]: any }
interface Component { id: string; name: string; type: string; bus_id: string; enabled: boolean; params: ComponentParams }
interface Topology { buses: Bus[]; components: Component[]; connections: any[] }

const visible = ref(false)
const topology = reactive<Topology>({ buses: [], components: [], connections: [] })

function addBus() {
  topology.buses.push({ id: `bus_${Date.now()}`, name: `母线${topology.buses.length + 1}`, voltage_kv: 10 })
}
function removeBus(index: number) { topology.buses.splice(index, 1) }
function addComponent() {
  topology.components.push({
    id: `comp_${Date.now()}`, name: '', type: 'load', bus_id: '', enabled: true, params: { rated_power_kw: 0 }
  })
}
function removeComponent(index: number) { topology.components.splice(index, 1) }

function open() { visible.value = true }
function close() { visible.value = false }
function setTopology(t: Topology) {
  topology.buses = t.buses || []
  topology.components = t.components || []
  topology.connections = t.connections || []
}
function getTopology(): Topology { return JSON.parse(JSON.stringify(topology)) }

function save() { visible.value = false }

defineExpose({ open, close, setTopology, getTopology })
</script>
```

- [ ] **Step 2: Build frontend**

Run: `cd web && npm run build`
Expected: exit 0

- [ ] **Step 3: Commit**

```bash
git add web/src/views/MicrogridEditor.vue
git commit -m "feat(microgrid): topology editor dialog component"
```

---

### Task 11: Frontend — Microgrid detail page

**Files:**
- Create: `web/src/views/MicrogridDetail.vue`
- Modify: `web/src/router/index.ts` (add route)
- Modify: `web/src/views/DetailPage.vue` (add microgrid tab)

- [ ] **Step 1: Create MicrogridDetail.vue**

```vue
<template>
  <div>
    <el-tabs v-model="activeTab">
      <el-tab-pane label="实时数据" name="points">
        <!-- Reuse point list from DetailPage -->
      </el-tab-pane>
      <el-tab-pane label="微电网拓扑" name="topology">
        <svg-topology :topology="topologyData" />
      </el-tab-pane>
      <el-tab-pane label="控制面板" name="control">
        <el-card>
          <h3>并/离网控制</h3>
          <el-switch v-model="islandMode" active-text="孤岛模式" inactive-text="并网模式"
            @change="onIslandChange" />
        </el-card>
        <el-card>
          <h3>关键指标</h3>
          <el-descriptions :column="2" border>
            <el-descriptions-item label="SOC">{{ dashboard.soc ?? '-' }} %</el-descriptions-item>
            <el-descriptions-item label="运行状态">{{ dashboard.running ? '运行中' : '已停止' }}</el-descriptions-item>
            <el-descriptions-item label="已运行">{{ dashboard.uptime ?? 0 }}s</el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-tab-pane>
      <el-tab-pane label="环境数据" name="env">
        <el-upload :action="`/api/v1/microgrid/${instanceId}/env-data`">
          <el-button>上传环境CSV</el-button>
        </el-upload>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getMicrogridDashboard, switchMicrogridIsland, getMicrogridTopology } from '../api'

const props = defineProps<{ instanceId: string }>()
const activeTab = ref('points')
const islandMode = ref(false)
const dashboard = ref<any>({})
const topologyData = ref<any>({})

async function loadDashboard() {
  const res = await getMicrogridDashboard(props.instanceId)
  dashboard.value = res.data
}
async function loadTopology() {
  const res = await getMicrogridTopology(props.instanceId)
  topologyData.value = res.data
}
async function onIslandChange(val: boolean) {
  await switchMicrogridIsland(props.instanceId, val)
}

onMounted(() => {
  loadDashboard()
  loadTopology()
})
</script>
```

- [ ] **Step 2: Add route for microgrid detail**

In `web/src/router/index.ts`:

```typescript
{
  path: '/microgrid/:id',
  name: 'MicrogridDetail',
  component: () => import('../views/MicrogridDetail.vue'),
}
```

- [ ] **Step 3: Wire microgrid detail tab in DetailPage.vue**

In `DetailPage.vue`, detect if instance protocol is `"microgrid"`:

```html
<el-tabs v-model="activeTab">
  <!-- existing tabs -->
  <el-tab-pane v-if="isMicrogrid" label="微电网" name="microgrid">
    <MicrogridDetail :instance-id="instanceId" />
  </el-tab-pane>
</el-tabs>

<script setup>
import { computed } from 'vue'
import MicrogridDetail from './MicrogridDetail.vue'

const isMicrogrid = computed(() => instance.value?.protocol === 'microgrid')
</script>
```

- [ ] **Step 4: Build frontend**

Run: `cd web && npm run build`
Expected: exit 0

- [ ] **Step 5: Commit**

```bash
git add web/src/views/MicrogridDetail.vue web/src/router/index.ts web/src/views/DetailPage.vue
git commit -m "feat(microgrid): microgrid detail page with control panel + topology tab"
```

---

### Task 12: Frontend — Microgrid topology SVG visualization

**Files:**
- Create: `web/src/views/MicrogridTopologyViz.vue`

- [ ] **Step 1: Create SVG topology visualization**

```vue
<template>
  <div class="microgrid-topology-viz" ref="container">
    <svg :width="width" :height="height" v-if="topology.buses">
      <!-- Buses as horizontal lines -->
      <line v-for="bus in topology.buses" :key="bus.id"
        :x1="margin" :y1="busY(bus.id)" :x2="width - margin" :y2="busY(bus.id)"
        stroke="#409EFF" stroke-width="3" />

      <!-- Components as boxes -->
      <g v-for="comp in topology.components" :key="comp.id"
        :transform="`translate(${compX(comp)}, ${busY(comp.bus_id) - 25})`">
        <rect width="80" height="50" rx="5" :fill="compColor(comp.type)" stroke="#333" stroke-width="1" />
        <text x="40" y="20" text-anchor="middle" fill="white" font-size="12">{{ compLabel(comp.type) }}</text>
        <text x="40" y="38" text-anchor="middle" fill="white" font-size="10">{{ comp.name }}</text>
      </g>

      <!-- Power flow arrows -->
      <!-- (simplified — real implementation uses dynamic arrow animation) -->
    </svg>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'

const props = defineProps<{ topology: any }>()
const width = ref(600)
const height = ref(400)
const margin = 60

function busY(busId: string): number {
  const buses = props.topology?.buses || []
  const idx = buses.findIndex((b: any) => b.id === busId)
  return 60 + idx * 120
}

function compX(comp: any): number {
  const types = ['pv', 'battery', 'load', 'grid', 'diesel', 'wind']
  const idx = types.indexOf(comp.type)
  return margin + 20 + (idx % 3) * 180
}

function compColor(type: string): string {
  const colors: Record<string, string> = {
    pv: '#67C23A', battery: '#409EFF', load: '#E6A23C',
    grid: '#909399', diesel: '#F56C6C', wind: '#00B4D8',
  }
  return colors[type] || '#999'
}

function compLabel(type: string): string {
  const labels: Record<string, string> = {
    pv: '光伏', battery: '储能', load: '负荷',
    grid: '电网', diesel: '柴发', wind: '风电',
  }
  return labels[type] || type
}
</script>

<style scoped>
.microgrid-topology-viz {
  border: 1px solid #eee;
  border-radius: 8px;
  overflow: hidden;
  background: #fafafa;
}
</style>
```

- [ ] **Step 2: Build frontend**

Run: `cd web && npm run build`
Expected: exit 0

- [ ] **Step 3: Commit**

```bash
git add web/src/views/MicrogridTopologyViz.vue
git commit -m "feat(microgrid): SVG topology visualization component"
```

---

### Task 13: Simulation engine unit tests

**Files:**
- Create: `internal/microgrid/engine_test.go`

- [ ] **Step 1: Write engine tests**

```go
package microgrid

import (
	"testing"
	"gridsim/pkg/config"
	"gridsim/pkg/library"
)

func TestEngine_StartStop(t *testing.T) {
	topo := &Topology{
		Buses: []Bus{{ID: "bus1", Name: "MainBus", VoltageKV: 10}},
		Components: []Component{
			{ID: "pv1", Name: "PV1", Type: CompPV, BusID: "bus1", Enabled: true,
				Params: ComponentParams{RatedPowerKW: 100, Efficiency: 0.95}},
			{ID: "load1", Name: "Load1", Type: CompLoad, BusID: "bus1", Enabled: true,
				Params: ComponentParams{RatedPowerKW: 50, PowerFactor: 1}},
		},
	}

	points := ExpandPoints(topo, 100000)
	store := library.NewStore(points)

	eng := NewEngine(topo, store, MicrogridConfig{TickMs: 100, BaseIOA: 100000})

	if err := eng.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	if !eng.IsRunning() {
		t.Fatal("engine should be running")
	}

	eng.Stop()
	if eng.IsRunning() {
		t.Fatal("engine should be stopped")
	}
}

func TestEngine_SOC(t *testing.T) {
	topo := &Topology{
		Buses: []Bus{{ID: "bus1", Name: "MainBus", VoltageKV: 10}},
		Components: []Component{
			{ID: "bat1", Name: "BAT1", Type: CompBattery, BusID: "bus1", Enabled: true,
				Params: ComponentParams{
					CapacityKWH: 100, InitSOC: 50, SOCMin: 10, SOCMax: 90,
					MaxChargeKW: 50, MaxDischargeKW: 50,
					ChargeEfficiency: 0.95, DischargeEfficiency: 0.95,
				}},
		},
	}

	points := ExpandPoints(topo, 100000)
	store := library.NewStore(points)

	// Set AO setpoint to discharge (positive = discharge)
	spIOA := FindPointIOA(topo, 100000, "bat1", "PowerSetpoint")
	if spIOA > 0 {
		store.SetValue(spIOA, 30) // 30kW discharge
	}

	eng := NewEngine(topo, store, MicrogridConfig{TickMs: 100, BaseIOA: 100000})
	_ = eng.Start()
	defer eng.Stop()

	// SOC should have decreased from 50 after running
	soc, ok := eng.GetSOC("bat1")
	if !ok {
		t.Fatal("SOC not available")
	}
	if soc >= 50 {
		t.Fatalf("SOC should have decreased from 50, got %.2f", soc)
	}
}

func TestExpandPoints_AllTypes(t *testing.T) {
	topo := &Topology{
		Components: []Component{
			{ID: "pv1", Name: "PV1", Type: CompPV, Enabled: true},
			{ID: "bat1", Name: "BAT1", Type: CompBattery, Enabled: true},
			{ID: "load1", Name: "Load1", Type: CompLoad, Enabled: true},
			{ID: "grid1", Name: "Grid1", Type: CompGrid, Enabled: true},
			{ID: "diesel1", Name: "Diesel1", Type: CompDiesel, Enabled: true},
			{ID: "wind1", Name: "Wind1", Type: CompWind, Enabled: true},
		},
	}
	pts := ExpandPoints(topo, 100000)
	// Check no duplicate IOAs
	ioaSet := make(map[uint32]bool)
	for _, p := range pts {
		if ioaSet[p.IOA] {
			t.Fatalf("duplicate IOA: %d (%s)", p.IOA, p.Name)
		}
		ioaSet[p.IOA] = true
	}
	// PV=5, Battery=9, Load=4, Grid=6, Diesel=5, Wind=5 = 34 total
	if len(pts) != 34 {
		t.Fatalf("expected 34 points, got %d", len(pts))
	}
}

func TestIslandMode(t *testing.T) {
	topo := &Topology{
		Buses: []Bus{{ID: "bus1", Name: "MainBus", VoltageKV: 10}},
		Components: []Component{
			{ID: "grid1", Name: "Grid1", Type: CompGrid, BusID: "bus1", Enabled: true,
				Params: ComponentParams{GridCapacityKW: 500}},
			{ID: "pv1", Name: "PV1", Type: CompPV, BusID: "bus1", Enabled: true,
				Params: ComponentParams{RatedPowerKW: 100, Efficiency: 1}},
			{ID: "load1", Name: "Load1", Type: CompLoad, BusID: "bus1", Enabled: true,
				Params: ComponentParams{RatedPowerKW: 200, PowerFactor: 1}},
		},
	}
	points := ExpandPoints(topo, 100000)
	store := library.NewStore(points)
	eng := NewEngine(topo, store, MicrogridConfig{TickMs: 100, BaseIOA: 100000})

	// initial: connected (not island)
	eng.SetIsland(false)

	// Check Connected DI point
	connIOA := FindPointIOA(topo, 100000, "grid1", "Connected")
	if connIOA > 0 {
		if p, ok := store.Get(connIOA); ok {
			if p.BoolValue != true {
				t.Fatal("expected Connected=true before island mode")
			}
		}
	}

	// Switch to island
	eng.SetIsland(true)
	if p, ok := store.Get(connIOA); ok {
		if p.BoolValue != false {
			t.Fatal("expected Connected=false after island mode")
		}
	}

	// Check Island DI
	islandIOA := FindPointIOA(topo, 100000, "grid1", "Island")
	if islandIOA > 0 {
		if p, ok := store.Get(islandIOA); ok {
			if p.BoolValue != true {
				t.Fatal("expected Island=true after island mode")
			}
		}
	}
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./internal/microgrid/ -v -count=1`
Expected: all PASS

- [ ] **Step 3: Commit**

```bash
git add internal/microgrid/engine_test.go internal/microgrid/pointmap_test.go
git commit -m "test(microgrid): engine lifecycle, SOC, expand-points, island mode tests"
```

---

### Task 14: Full build verification

- [ ] **Step 1: Build all Go code**

Run: `go build ./...`
Expected: exit 0

- [ ] **Step 2: Run all Go tests**

Run: `go test ./...`
Expected: all PASS (or only pre-existing failures)

- [ ] **Step 3: Build frontend**

Run: `cd web && npm run build`
Expected: exit 0

- [ ] **Step 4: Build distribution**

Run: `make dist` (or manual `go build -o bin/gridsim ./cmd/gridsim/`)
Expected: exit 0, binary at `bin/gridsim`

- [ ] **Step 5: Commit final**

```bash
git add -A
git commit -m "chore: build verification passes for microgrid simulation feature"
```

---

### Task 15: Update MCP documentation

**Files:**
- Modify: `MCP.md`
- Modify: `manuals/mcp-manual.md`

- [ ] **Step 1: Document microgrid MCP tools**

In `MCP.md`, add to the tool list:

```markdown
### `create_microgrid`
创建微电网仿真实例。必需参数：`name`（实例名）、`topology`（拓扑JSON）。可选：`iec104_port`、`tick_ms`。

### `config_microgrid_topology`
更新微电网拓扑配置。参数：`instance_id`、`topology`。

### `microgrid_switch`
切换并网/孤岛模式。参数：`instance_id`、`island`（boolean）。

### `get_microgrid_dashboard`
获取微电网仪表盘数据。参数：`instance_id`。
```

- [ ] **Step 2: Commit**

```bash
git add MCP.md manuals/mcp-manual.md
git commit -m "docs(microgrid): add MCP tool documentation for microgrid features"
```
