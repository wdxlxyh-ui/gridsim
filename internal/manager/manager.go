package manager

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gridsim/internal/detail"
	"gridsim/internal/microgrid"
	"gridsim/internal/model"
	"gridsim/internal/storage"
	persist "gridsim/internal/store"
	"gridsim/pkg/api"
	"gridsim/pkg/config"
	"gridsim/pkg/firewall"
	"gridsim/pkg/library"
	"gridsim/pkg/protocol"
	"gridsim/pkg/protocol/modbus_bridge"
)

func generateID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func instanceProtocolPort(cfg model.InstanceConfig) int {
	if cfg.Protocol == "modbus_tcp" && cfg.ModbusConfig != nil && cfg.ModbusConfig.Port > 0 {
		return cfg.ModbusConfig.Port
	}
	return cfg.IEC104Port
}

func usesProtocolServerPort(cfg model.InstanceConfig) bool {
	return cfg.Protocol != "iec104_client" && cfg.Protocol != "modbus_bridge"
}

// Instance wraps a running protocol server instance.
type Instance struct {
	Config                 model.InstanceConfig
	Protocol               protocol.Protocol
	Store                  *library.Store
	HTTPServer             *http.Server
	AutoEngine             *detail.Engine
	Logger                 *InstanceLogger
	Microgrid              *microgrid.Engine // non-nil only for microgrid instances
	persistenceUnsubscribe func()
}

// MaxInstances is the maximum number of concurrent instances allowed.
// Set to 0 for unlimited (disabled check)
const MaxInstances = 1000

// Manager manages multiple IEC104 server instances.
type Manager struct {
	mu        sync.RWMutex
	instances map[string]*Instance
	store     *storage.ConfigStore
	cfgDir    string
	dataStore *persist.Service
}

// New creates a new Manager.
func New(store *storage.ConfigStore, cfgDir string) *Manager {
	return &Manager{
		instances: make(map[string]*Instance),
		store:     store,
		cfgDir:    cfgDir,
	}
}

// Store returns the underlying ConfigStore.
func (m *Manager) Store() *storage.ConfigStore {
	return m.store
}

// SetDataStore injects the optional SQLite persistence service. A nil value
// disables persistence without changing any simulator behaviour.
func (m *Manager) SetDataStore(dataStore *persist.Service) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dataStore = dataStore
}

// DataStore returns the configured persistence service, or nil when disabled.
func (m *Manager) DataStore() *persist.Service {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.dataStore
}

// periodicPersistenceSamples retains the continuous 1-second record for source
// measurements only. AO/DO are control audit records and are event-driven.
func periodicPersistenceSamples(points []*config.Point, timestamp int64) []persist.Sample {
	result := make([]persist.Sample, 0, len(points))
	for _, point := range points {
		switch point.PointType {
		case config.TypeAI, config.TypeDI, config.TypePI:
			result = append(result, persistenceSample(*point, timestamp))
		}
	}
	return result
}

func persistenceSample(point config.Point, timestamp int64) persist.Sample {
	if timestamp <= 0 {
		timestamp = time.Now().UnixMilli()
	}
	return persist.Sample{
		IOA:       point.IOA,
		Timestamp: timestamp,
		Name:      point.Name,
		PointType: string(point.PointType),
		Value:     point.Value,
		BoolValue: point.BoolValue,
		IntValue:  point.IntValue,
		QDS:       encodeQDS(point.QDS),
	}
}

func shouldPersistImmediately(change library.PointChange) bool {
	switch change.Point.PointType {
	case config.TypeAO, config.TypeDO:
		// Every AO/DO setter call represents a control attempt, even when the
		// requested value equals the current one.
		return true
	case config.TypeAI, config.TypeDI, config.TypePI:
		return change.ValueChanged
	default:
		return false
	}
}

// startPersistence samples AI/DI/PI once per interval and subscribes to Store
// writes for immediate source changes and AO/DO control audit records.
func (m *Manager) startPersistence(instanceID string, pointStore *library.Store) {
	dataStore := m.dataStore
	if dataStore == nil || pointStore == nil {
		return
	}
	if err := dataStore.StartInstance(instanceID, func() []persist.Sample {
		return periodicPersistenceSamples(pointStore.SnapshotAll(), time.Now().UnixMilli())
	}); err != nil {
		slog.Error("启动测点数据持久化失败", "instance", instanceID, "error", err)
		return
	}

	unsubscribe := pointStore.SubscribeChanges(func(change library.PointChange) {
		if shouldPersistImmediately(change) {
			dataStore.Enqueue(instanceID, persistenceSample(change.Point, change.Point.Timestamp.UnixMilli()))
		}
	})
	if inst, ok := m.instances[instanceID]; ok {
		if inst.persistenceUnsubscribe != nil {
			inst.persistenceUnsubscribe()
		}
		inst.persistenceUnsubscribe = unsubscribe
	}
}

func (m *Manager) stopPersistence(instanceID string) {
	if inst, ok := m.instances[instanceID]; ok && inst.persistenceUnsubscribe != nil {
		inst.persistenceUnsubscribe()
		inst.persistenceUnsubscribe = nil
	}
	if m.dataStore != nil {
		m.dataStore.StopInstance(instanceID)
	}
}

func encodeQDS(q config.QualityDescriptor) uint8 {
	var result uint8
	if q.Invalid {
		result |= 1
	}
	if q.NotTopical {
		result |= 2
	}
	if q.Substituted {
		result |= 4
	}
	if q.Overflow {
		result |= 8
	}
	if q.Blocked {
		result |= 16
	}
	return result
}

// ListConfigs returns all instance configurations.
func (m *Manager) ListConfigs() []model.InstanceConfig {
	return m.store.List()
}

// GetConfig returns an instance configuration by ID.
func (m *Manager) GetConfig(id string) (model.InstanceConfig, bool) {
	return m.store.Get(id)
}

// CreateConfig creates a new instance configuration with an auto-generated ID.
// Returns the created config with the assigned ID.
func (m *Manager) CreateConfig(cfg model.InstanceConfig) (model.InstanceConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if MaxInstances > 0 && m.store.Count() >= MaxInstances {
		return model.InstanceConfig{}, fmt.Errorf("maximum %d instances allowed", MaxInstances)
	}

	// Check the actual protocol listener port, which may come from modbus_config.
	cfgPort := instanceProtocolPort(cfg)
	if usesProtocolServerPort(cfg) && cfg.HttpEnabled && cfgPort == cfg.HttpPort {
		return model.InstanceConfig{}, fmt.Errorf("protocol port and http port cannot both use %d", cfgPort)
	}
	for _, existing := range m.store.List() {
		existingPort := instanceProtocolPort(existing)
		if usesProtocolServerPort(cfg) && usesProtocolServerPort(existing) {
			if cfgPort != 0 && existingPort == cfgPort {
				return model.InstanceConfig{}, fmt.Errorf("port %d already configured for instance %s", cfgPort, existing.ID)
			}
		}
		if usesProtocolServerPort(cfg) && existing.HttpEnabled && existing.HttpPort == cfgPort {
			return model.InstanceConfig{}, fmt.Errorf("port %d already configured as http port for instance %s", cfgPort, existing.ID)
		}
		if cfg.HttpEnabled && usesProtocolServerPort(existing) && existingPort == cfg.HttpPort {
			return model.InstanceConfig{}, fmt.Errorf("http port %d already configured as protocol port for instance %s", cfg.HttpPort, existing.ID)
		}
		if cfg.HttpEnabled && existing.HttpEnabled && existing.HttpPort == cfg.HttpPort {
			return model.InstanceConfig{}, fmt.Errorf("http port %d already configured for instance %s", cfg.HttpPort, existing.ID)
		}
		// Check Modbus bridge port collision
		if cfg.Protocol == "modbus_bridge" && existing.Protocol == "modbus_bridge" &&
			cfg.ModbusBridgeConfig != nil && existing.ModbusBridgeConfig != nil &&
			cfg.ModbusBridgeConfig.ModbusPort == existing.ModbusBridgeConfig.ModbusPort {
			return model.InstanceConfig{}, fmt.Errorf("modbus port %d already configured for instance %s", cfg.ModbusBridgeConfig.ModbusPort, existing.ID)
		}
	}

	// Auto-generate ID if not provided by client
	if cfg.ID == "" {
		cfg.ID = generateID()
	}

	if err := m.store.Add(cfg); err != nil {
		return model.InstanceConfig{}, err
	}
	return cfg, nil
}

func (m *Manager) StartInstance(id string) error {
	cfg, ok := m.store.Get(id)
	if ok && cfg.Protocol == "microgrid" {
		return m.startMicrogrid(id)
	}
	if ok && cfg.Protocol == "iec104_client" {
		return m.startClient(id)
	}
	if ok && cfg.Protocol == "modbus_bridge" {
		return m.startBridge(id)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.instances[id]; ok {
		return fmt.Errorf("instance %s already running", id)
	}

	cfg, ok = m.store.Get(id)
	if !ok {
		return fmt.Errorf("instance %s not found", id)
	}

	protocolPort := instanceProtocolPort(cfg)
	if cfg.Protocol == "modbus_tcp" {
		// Keep runtime metadata, logs and firewall rules aligned with the port
		// that protocol.New will actually bind.
		cfg.IEC104Port = protocolPort
	}
	if cfg.HttpEnabled && cfg.HttpPort == protocolPort {
		return fmt.Errorf("protocol port and http port cannot both use %d", protocolPort)
	}
	for _, inst := range m.instances {
		existingPort := instanceProtocolPort(inst.Config)
		if usesProtocolServerPort(inst.Config) && existingPort == protocolPort {
			return fmt.Errorf("port %d already in use by instance %s", protocolPort, inst.Config.ID)
		}
		if inst.Config.HttpEnabled && inst.Config.HttpPort == protocolPort {
			return fmt.Errorf("port %d already used as http port by instance %s", protocolPort, inst.Config.ID)
		}
		if cfg.HttpEnabled && usesProtocolServerPort(inst.Config) && existingPort == cfg.HttpPort {
			return fmt.Errorf("http port %d already used as protocol port by instance %s", cfg.HttpPort, inst.Config.ID)
		}
		if cfg.HttpEnabled && inst.Config.HttpEnabled && inst.Config.HttpPort == cfg.HttpPort {
			return fmt.Errorf("http port %d already in use by instance %s", cfg.HttpPort, inst.Config.ID)
		}
	}

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", protocolPort))
	if err != nil {
		return fmt.Errorf("port %d not available: %w", protocolPort, err)
	}
	ln.Close()

	if cfg.HttpEnabled && cfg.HttpPort > 0 {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.HttpPort))
		if err != nil {
			return fmt.Errorf("http port %d not available: %w", cfg.HttpPort, err)
		}
		ln.Close()
	}

	xlsxPath := cfg.XLSXFile
	if !filepath.IsAbs(xlsxPath) {
		if _, err := os.Stat(xlsxPath); os.IsNotExist(err) {
			xlsxPath = filepath.Join(m.cfgDir, xlsxPath)
		}
	}

	points, err := config.LoadFromXLSX(xlsxPath, cfg.Protocol)
	if err != nil {
		return fmt.Errorf("load xlsx: %w", err)
	}

	store := library.NewStore(points)

	proto, err := protocol.New(cfg)
	if err != nil {
		return fmt.Errorf("create protocol: %w", err)
	}
	proto.SetStore(store)
	if err := proto.Start(); err != nil {
		return fmt.Errorf("start protocol: %w", err)
	}

	acStore := detail.NewAutoChangeStore(m.cfgDir)
	engine := detail.NewEngine(cfg.ID, store, proto, acStore, m.cfgDir, m)
	if p, ok := proto.(interface{ SetAOFollowHandler(func(aoIOA uint32)) }); ok {
		p.SetAOFollowHandler(engine.HandleAOFollow)
	}
	if err := engine.LoadAndStart(); err != nil {
		slog.Warn("自动变化引擎加载失败", "id", id, "error", err)
	}

	logger, err := NewInstanceLogger(m.cfgDir, cfg.ID, cfg.IEC104Port)
	if err != nil {
		slog.Warn("创建实例日志目录失败", "id", id, "error", err)
	}

	inst := &Instance{
		Config:     cfg,
		Protocol:   proto,
		Store:      store,
		AutoEngine: engine,
		Logger:     logger,
	}

	if cfg.HttpEnabled && cfg.HttpPort > 0 {
		apiHandler := api.NewHandler(store, proto, proto)
		detailHandler := detail.NewDetailHandler(cfg.ID, store, engine, m.cfgDir)
		httpMux := http.NewServeMux()
		apiHandler.Register(httpMux)
		detailHandler.Register(httpMux)
		httpSrv := &http.Server{Addr: fmt.Sprintf(":%d", cfg.HttpPort), Handler: httpMux}
		go func() {
			slog.Info("实例HTTP API已启动", "id", id, "port", cfg.HttpPort)
			if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("实例HTTP API失败", "id", id, "error", err)
			}
		}()
		inst.HTTPServer = httpSrv
	}

	firewall.EnsurePort(cfg.IEC104Port, "iec104-sim-instance")
	if cfg.HttpEnabled && cfg.HttpPort > 0 {
		firewall.EnsurePort(cfg.HttpPort, "iec104-sim-instance")
	}

	m.instances[id] = inst
	m.startPersistence(id, store)

	slog.Info("实例已启动", "id", id, "port", cfg.IEC104Port, "name", cfg.Name, "points", len(points))
	return nil
}

// UpdateConfig updates an instance configuration, stopping it if running.
// GetAutoChangeActiveIOAs returns IOAs with active auto-change strategies
func (m *Manager) GetAutoChangeActiveIOAs(id string) []uint32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if inst, ok := m.instances[id]; ok && inst.AutoEngine != nil {
		return inst.AutoEngine.GetActiveIOAs()
	}
	return nil
}

func (m *Manager) SaveConfigOnly(cfg model.InstanceConfig) error {
	return m.store.Update(cfg)
}

func (m *Manager) UpdateConfig(cfg model.InstanceConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	newPort := instanceProtocolPort(cfg)
	if usesProtocolServerPort(cfg) && cfg.HttpEnabled && newPort == cfg.HttpPort {
		return fmt.Errorf("protocol port and http port cannot both use %d", newPort)
	}
	for _, existing := range m.store.List() {
		if existing.ID == cfg.ID {
			continue
		}
		existingPort := instanceProtocolPort(existing)
		if usesProtocolServerPort(cfg) && usesProtocolServerPort(existing) && newPort != 0 && existingPort == newPort {
			return fmt.Errorf("port %d already configured for instance %s", newPort, existing.ID)
		}
		if usesProtocolServerPort(cfg) && existing.HttpEnabled && existing.HttpPort == newPort {
			return fmt.Errorf("port %d already configured as http port for instance %s", newPort, existing.ID)
		}
		if cfg.HttpEnabled && usesProtocolServerPort(existing) && existingPort == cfg.HttpPort {
			return fmt.Errorf("http port %d already configured as protocol port for instance %s", cfg.HttpPort, existing.ID)
		}
		if cfg.HttpEnabled && existing.HttpEnabled && existing.HttpPort == cfg.HttpPort {
			return fmt.Errorf("http port %d already configured for instance %s", cfg.HttpPort, existing.ID)
		}
	}

	if inst, ok := m.instances[cfg.ID]; ok {
		oldPort := instanceProtocolPort(inst.Config)
		inst.Protocol.Stop()
		if inst.HTTPServer != nil {
			inst.HTTPServer.Close()
			inst.HTTPServer = nil
		}
		if inst.Logger != nil {
			inst.Logger.Close()
			if oldPort != newPort {
				RenameInstanceLogDir(m.cfgDir, cfg.ID, cfg.ID, oldPort, newPort)
			}
		}
		if usesProtocolServerPort(inst.Config) {
			firewall.RemovePort(oldPort)
		}
		if inst.Config.HttpEnabled && inst.Config.HttpPort > 0 {
			firewall.RemovePort(inst.Config.HttpPort)
		}
		m.stopPersistence(cfg.ID)
		delete(m.instances, cfg.ID)
	}

	return m.store.Update(cfg)
}

func (m *Manager) DeleteConfig(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if inst, ok := m.instances[id]; ok {
		inst.Protocol.Stop()
		if inst.AutoEngine != nil {
			inst.AutoEngine.StopAll()
		}
		if inst.HTTPServer != nil {
			inst.HTTPServer.Close()
			inst.HTTPServer = nil
		}
		if inst.Logger != nil {
			inst.Logger.Close()
			RemoveInstanceLogDir(m.cfgDir, inst.Config.ID, instanceProtocolPort(inst.Config))
		}
		firewall.RemovePort(instanceProtocolPort(inst.Config))
		if inst.Config.HttpEnabled && inst.Config.HttpPort > 0 {
			firewall.RemovePort(inst.Config.HttpPort)
		}
		m.stopPersistence(id)
		delete(m.instances, id)
	} else {
		cfg, _ := m.store.Get(id)
		if cfg.ID != "" {
			RemoveInstanceLogDir(m.cfgDir, cfg.ID, instanceProtocolPort(cfg))
		}
	}

	if err := m.store.Delete(id); err != nil {
		return err
	}
	if m.dataStore != nil {
		if err := m.dataStore.DropInstance(id); err != nil {
			slog.Warn("删除实例历史数据失败", "id", id, "error", err)
		}
	}
	return nil
}

func (m *Manager) StopInstance(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.instances[id]
	if !ok {
		return fmt.Errorf("instance %s not running", id)
	}

	if inst.Microgrid != nil {
		inst.Microgrid.Stop()
	}
	inst.Protocol.Stop()
	if inst.AutoEngine != nil {
		inst.AutoEngine.StopAll()
	}
	if inst.HTTPServer != nil {
		inst.HTTPServer.Close()
		inst.HTTPServer = nil
	}

	if inst.Logger != nil {
		inst.Logger.Close()
		RemoveInstanceLogDir(m.cfgDir, inst.Config.ID, instanceProtocolPort(inst.Config))
	}

	if usesProtocolServerPort(inst.Config) {
		firewall.RemovePort(instanceProtocolPort(inst.Config))
	}
	if inst.Config.HttpEnabled && inst.Config.HttpPort > 0 {
		firewall.RemovePort(inst.Config.HttpPort)
	}

	m.stopPersistence(id)
	delete(m.instances, id)

	slog.Info("实例已停止", "id", id, "name", inst.Config.Name)
	return nil
}

func (m *Manager) CfgDir() string {
	return m.cfgDir
}

func (m *Manager) GetStore(id string) *library.Store {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if inst, ok := m.instances[id]; ok {
		return inst.Store
	}
	return nil
}

// GetInstance returns the running instance by ID, or nil.
func (m *Manager) GetInstance(id string) *Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.instances[id]
}

func (m *Manager) GetEngine(id string) *detail.Engine {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if inst, ok := m.instances[id]; ok {
		return inst.AutoEngine
	}
	return nil
}

// RestartInstance stops and starts an instance.
// If the instance is not running, it is started directly.
func (m *Manager) RestartInstance(id string) error {
	if err := m.StopInstance(id); err != nil {
		// If the instance is not running, just start it
		if strings.Contains(err.Error(), "not running") {
			return m.StartInstance(id)
		}
		return err
	}
	return m.StartInstance(id)
}

// GetState returns the runtime state of an instance.
func (m *Manager) GetState(id string) (*model.InstanceState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cfg, ok := m.store.Get(id)
	if !ok {
		return nil, fmt.Errorf("instance %s not found", id)
	}

	state := &model.InstanceState{
		Config: cfg,
		Status: model.StatusStopped,
	}

	if inst, running := m.instances[id]; running && inst.Protocol != nil {
		state.Status = model.StatusRunning
		state.UptimeSeconds = inst.Protocol.Uptime()
		state.TotalPoints = inst.Store.TotalCount()
		state.ClientConnected = inst.Protocol.ClientConnected()
		interrog, control, spont := inst.Protocol.Stats()
		state.Interrogations = interrog
		state.Controls = control
		state.Spontaneous = spont
	}

	return state, nil
}

// ListStates returns runtime states for all configured instances.
func (m *Manager) ListStates() []*model.InstanceState {
	cfgs := m.store.List()
	states := make([]*model.InstanceState, 0, len(cfgs))

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, cfg := range cfgs {
		state := &model.InstanceState{
			Config: cfg,
			Status: model.StatusStopped,
		}
		if inst, ok := m.instances[cfg.ID]; ok && inst.Protocol != nil {
			state.Status = model.StatusRunning
			state.UptimeSeconds = inst.Protocol.Uptime()
			state.TotalPoints = inst.Store.TotalCount()
			state.ClientConnected = inst.Protocol.ClientConnected()
			interrog, control, spont := inst.Protocol.Stats()
			state.Interrogations = interrog
			state.Controls = control
			state.Spontaneous = spont
		}
		states = append(states, state)
	}

	return states
}

// RunningCount returns the number of running instances.
func (m *Manager) RunningCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.instances)
}

// DashboardData holds aggregated stats for the dashboard API.
type DashboardData struct {
	TotalInstances   int                    `json:"total_instances"`
	RunningInstances int                    `json:"running_instances"`
	StoppedInstances int                    `json:"stopped_instances"`
	ErrorInstances   int                    `json:"error_instances"`
	TotalPoints      int                    `json:"total_points"`
	ClientsConnected int                    `json:"clients_connected"`
	ByProtocol       map[string]int         `json:"by_protocol"`
	Instances        []*model.InstanceState `json:"instances"`
}

// GetDashboardData returns aggregated dashboard data.
func (m *Manager) GetDashboardData() *DashboardData {
	data := &DashboardData{
		ByProtocol: make(map[string]int),
	}
	states := m.ListStates()
	data.Instances = states
	data.TotalInstances = len(states)

	for _, s := range states {
		proto := s.Config.Protocol
		if proto == "" {
			proto = "iec104"
		}
		data.ByProtocol[proto]++

		switch s.Status {
		case model.StatusRunning:
			data.RunningInstances++
			data.TotalPoints += s.TotalPoints
			if s.ClientConnected {
				data.ClientsConnected++
			}
		case model.StatusStopped:
			data.StoppedInstances++
		case model.StatusError:
			data.ErrorInstances++
		}
	}
	return data
}

// StopAll stops all running instances.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, inst := range m.instances {
		if inst.Microgrid != nil {
			inst.Microgrid.Stop()
		}
		inst.Protocol.Stop()
		if inst.AutoEngine != nil {
			inst.AutoEngine.StopAll()
		}
		m.stopPersistence(id)
		delete(m.instances, id)
		slog.Info("实例已停止", "id", id, "name", inst.Config.Name)
	}
}

// ─── Microgrid Support ───

func (m *Manager) startMicrogrid(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.instances[id]; ok {
		return fmt.Errorf("instance %s already running", id)
	}

	cfg, ok := m.store.Get(id)
	if !ok {
		return fmt.Errorf("instance %s not found", id)
	}

	if cfg.MicrogridConfig == nil || cfg.MicrogridConfig.TopologyJSON == "" {
		return fmt.Errorf("microgrid topology not configured")
	}

	var topo microgrid.Topology
	if err := topo.FromJSON(cfg.MicrogridConfig.TopologyJSON); err != nil {
		return fmt.Errorf("invalid topology: %w", err)
	}
	if err := topo.Validate(); err != nil {
		return fmt.Errorf("topology validation failed: %w", err)
	}

	// Port availability check
	for _, inst := range m.instances {
		if inst.Config.IEC104Port == cfg.IEC104Port {
			return fmt.Errorf("port %d already in use", cfg.IEC104Port)
		}
	}
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.IEC104Port))
	if err != nil {
		return fmt.Errorf("port %d not available: %w", cfg.IEC104Port, err)
	}
	ln.Close()

	store := topo.StoreFromTopology()
	proto, err := protocol.New(cfg)
	if err != nil {
		return fmt.Errorf("create protocol: %w", err)
	}
	proto.SetStore(store)
	if err := proto.Start(); err != nil {
		return fmt.Errorf("start protocol: %w", err)
	}

	tickMs := 1000
	speed := 1.0
	if cfg.MicrogridConfig.TickMs > 0 {
		tickMs = cfg.MicrogridConfig.TickMs
	}
	if cfg.MicrogridConfig.SpeedFactor > 0 {
		speed = cfg.MicrogridConfig.SpeedFactor
	}

	eng := microgrid.NewEngine(&topo, store, microgrid.InstanceConfig{
		TickMs:      tickMs,
		SpeedFactor: speed,
		ConfigDir:   m.cfgDir,
		PointsJSON:  cfg.MicrogridConfig.PointsJSON,
	})
	eng.SetPublisher(proto)
	if err := eng.Start(); err != nil {
		proto.Stop()
		return fmt.Errorf("start microgrid engine: %w", err)
	}

	// Start auto-change detail engine for strategy evaluation
	acStore := detail.NewAutoChangeStore(m.cfgDir)
	autoEng := detail.NewEngine(id, store, proto, acStore, m.cfgDir, m)
	if err := autoEng.LoadAndStart(); err != nil {
		slog.Warn("auto-change engine start failed", "id", id, "error", err)
	}

	inst := &Instance{
		Config:     cfg,
		Protocol:   proto,
		Store:      store,
		Microgrid:  eng,
		AutoEngine: autoEng,
	}

	if cfg.HttpEnabled && cfg.HttpPort > 0 {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.HttpPort))
		if err != nil {
			proto.Stop()
			eng.Stop()
			return fmt.Errorf("http port %d not available: %w", cfg.HttpPort, err)
		}
		ln.Close()
		apiHandler := api.NewHandler(store, proto, proto)
		detailHandler := detail.NewDetailHandler(cfg.ID, store, autoEng, m.cfgDir)
		httpMux := http.NewServeMux()
		apiHandler.Register(httpMux)
		detailHandler.Register(httpMux)
		httpSrv := &http.Server{Addr: fmt.Sprintf(":%d", cfg.HttpPort), Handler: httpMux}
		go func() {
			slog.Info("微电网实例HTTP API已启动", "id", id, "port", cfg.HttpPort)
			if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("微电网实例HTTP API失败", "id", id, "error", err)
			}
		}()
		inst.HTTPServer = httpSrv
		firewall.EnsurePort(cfg.HttpPort, "gridsim-microgrid")
	}

	m.instances[id] = inst
	m.startPersistence(id, store)
	firewall.EnsurePort(cfg.IEC104Port, "gridsim-microgrid")
	slog.Info("微电网实例已启动", "id", id, "devices", len(topo.Devices))
	return nil
}

func (m *Manager) startClient(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.instances[id]; ok {
		return fmt.Errorf("instance %s already running", id)
	}

	cfg, ok := m.store.Get(id)
	if !ok {
		return fmt.Errorf("instance %s not found", id)
	}

	if cfg.IEC104ClientConfig == nil {
		return fmt.Errorf("iec104_client_config not configured")
	}

	xlsxPath := cfg.XLSXFile
	if !filepath.IsAbs(xlsxPath) {
		if _, err := os.Stat(xlsxPath); os.IsNotExist(err) {
			xlsxPath = filepath.Join(m.cfgDir, xlsxPath)
		}
	}

	points, err := config.LoadFromXLSX(xlsxPath, cfg.Protocol)
	if err != nil {
		return fmt.Errorf("load xlsx: %w", err)
	}

	store := library.NewStore(points)

	proto, err := protocol.New(cfg)
	if err != nil {
		return fmt.Errorf("create protocol: %w", err)
	}
	proto.SetStore(store)
	if err := proto.Start(); err != nil {
		return fmt.Errorf("start protocol: %w", err)
	}

	acStore := detail.NewAutoChangeStore(m.cfgDir)
	engine := detail.NewEngine(cfg.ID, store, proto, acStore, m.cfgDir, m)
	if err := engine.LoadAndStart(); err != nil {
		slog.Warn("自动变化引擎加载失败", "id", id, "error", err)
	}

	logger, err := NewInstanceLogger(m.cfgDir, cfg.ID, cfg.IEC104Port)
	if err != nil {
		slog.Warn("创建实例日志目录失败", "id", id, "error", err)
	}

	inst := &Instance{
		Config:     cfg,
		Protocol:   proto,
		Store:      store,
		AutoEngine: engine,
		Logger:     logger,
	}

	m.instances[id] = inst
	m.startPersistence(id, store)
	slog.Info("客户端实例已启动", "id", id, "remote",
		fmt.Sprintf("%s:%d", cfg.IEC104ClientConfig.RemoteAddr, cfg.IEC104ClientConfig.RemotePort),
		"points", len(points))
	return nil
}

// GetMicrogridEngine 获取微电网引擎
func (m *Manager) GetMicrogridEngine(id string) *microgrid.Engine {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if inst, ok := m.instances[id]; ok {
		return inst.Microgrid
	}
	return nil
}

// RegisterMicrogridInstance 注册微电网实例 (用于外部创建)
func (m *Manager) RegisterMicrogridInstance(id string, inst *Instance, eng *microgrid.Engine) {
	m.mu.Lock()
	defer m.mu.Unlock()
	inst.Microgrid = eng
	m.instances[id] = inst
	m.startPersistence(id, inst.Store)
}

// ─── Modbus Bridge (Python) Support ───

func (m *Manager) startBridge(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.instances[id]; ok {
		return fmt.Errorf("instance %s already running", id)
	}

	cfg, ok := m.store.Get(id)
	if !ok {
		return fmt.Errorf("instance %s not found", id)
	}

	if cfg.ModbusBridgeConfig == nil {
		return fmt.Errorf("modbus_bridge_config not configured")
	}

	// Parse device.json to generate points
	bc := cfg.ModbusBridgeConfig
	scriptDir := bc.ScriptDir
	if scriptDir == "" {
		scriptDir = "py_simulator"
	}
	if !filepath.IsAbs(scriptDir) {
		scriptDir = filepath.Join(m.cfgDir, scriptDir)
	}

	// Multi-instance isolation: create per-instance workspace
	// Each instance gets its own directory under py_instances/<id>/
	// with independent config/ and log/ but shared code (main.py, src/, utils/)
	instanceWorkDir, err := m.ensureBridgeWorkspace(id, scriptDir)
	if err != nil {
		slog.Warn("failed to create isolated workspace, using shared dir", "id", id, "error", err)
		instanceWorkDir = scriptDir
	}

	deviceJSON := bc.DeviceJSON
	if deviceJSON == "" {
		deviceJSON = "config/device.json"
	}

	devices, err := modbus_bridge.ParseDeviceJSON(instanceWorkDir, deviceJSON)
	if err != nil {
		return fmt.Errorf("parse device.json: %w", err)
	}

	// Build store from device mappings
	points := modbus_bridge.BuildStorePoints(devices)
	if len(points) == 0 {
		return fmt.Errorf("no points generated from device.json")
	}
	store := library.NewStore(points)

	// Override script_dir to point to the isolated instance workspace
	bridgeCfg := *cfg.ModbusBridgeConfig
	bridgeCfg.ScriptDir = instanceWorkDir
	cfgForProto := cfg
	cfgForProto.ModbusBridgeConfig = &bridgeCfg

	// Create and start bridge protocol
	proto, err := protocol.NewWithCfgDir(cfgForProto, m.cfgDir)
	if err != nil {
		return fmt.Errorf("create protocol: %w", err)
	}
	proto.SetStore(store)
	if err := proto.Start(); err != nil {
		return fmt.Errorf("start bridge: %w", err)
	}

	// Start auto-change engine
	acStore := detail.NewAutoChangeStore(m.cfgDir)
	engine := detail.NewEngine(cfg.ID, store, proto, acStore, m.cfgDir, m)
	if err := engine.LoadAndStart(); err != nil {
		slog.Warn("auto-change engine start failed for bridge", "id", id, "error", err)
	}

	inst := &Instance{
		Config:     cfg,
		Protocol:   proto,
		Store:      store,
		AutoEngine: engine,
	}

	// Optional: start per-instance HTTP API
	if cfg.HttpEnabled && cfg.HttpPort > 0 {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.HttpPort))
		if err != nil {
			proto.Stop()
			return fmt.Errorf("http port %d not available: %w", cfg.HttpPort, err)
		}
		ln.Close()
		apiHandler := api.NewHandler(store, proto, proto)
		detailHandler := detail.NewDetailHandler(cfg.ID, store, engine, m.cfgDir)
		httpMux := http.NewServeMux()
		apiHandler.Register(httpMux)
		detailHandler.Register(httpMux)
		httpSrv := &http.Server{Addr: fmt.Sprintf(":%d", cfg.HttpPort), Handler: httpMux}
		go func() {
			slog.Info("Bridge实例HTTP API已启动", "id", id, "port", cfg.HttpPort)
			if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("Bridge实例HTTP API失败", "id", id, "error", err)
			}
		}()
		inst.HTTPServer = httpSrv
		firewall.EnsurePort(cfg.HttpPort, "gridsim-bridge")
	}

	m.instances[id] = inst
	m.startPersistence(id, store)
	firewall.EnsurePort(bridgeCfg.ModbusPort, "gridsim-bridge-modbus")
	slog.Info("Bridge实例已启动", "id", id, "devices", len(devices), "points", len(points))
	return nil
}

// ensureBridgeWorkspace creates an isolated per-instance workspace directory for Python
// microgrid simulator. It copies config/ independently while sharing code files via
// file copy from the template directory. This enables multiple instances to run
// simultaneously with independent device configurations and log directories.
func (m *Manager) ensureBridgeWorkspace(instanceID, templateDir string) (string, error) {
	instancesDir := filepath.Join(m.cfgDir, "py_instances")
	workDir := filepath.Join(instancesDir, instanceID)

	// If workspace already exists, just return it
	if _, err := os.Stat(workDir); err == nil {
		return workDir, nil
	}

	// Create workspace structure
	if err := os.MkdirAll(filepath.Join(workDir, "config"), 0755); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(workDir, "log"), 0755); err != nil {
		return "", fmt.Errorf("create log dir: %w", err)
	}

	// Copy main.py from template
	if err := copyFile(filepath.Join(templateDir, "main.py"), filepath.Join(workDir, "main.py")); err != nil {
		return "", fmt.Errorf("copy main.py: %w", err)
	}

	// Copy src/ directory (code shared across instances)
	if err := copyDir(filepath.Join(templateDir, "src"), filepath.Join(workDir, "src")); err != nil {
		return "", fmt.Errorf("copy src: %w", err)
	}

	// Copy utils/ directory
	if err := copyDir(filepath.Join(templateDir, "utils"), filepath.Join(workDir, "utils")); err != nil {
		return "", fmt.Errorf("copy utils: %w", err)
	}

	// Copy config/ directory (each instance gets its own config copy)
	if err := copyDir(filepath.Join(templateDir, "config"), filepath.Join(workDir, "config")); err != nil {
		return "", fmt.Errorf("copy config: %w", err)
	}

	slog.Info("Bridge工作目录已创建", "id", instanceID, "path", workDir)
	return workDir, nil
}

// copyFile copies a single file from src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = out.ReadFrom(in)
	return err
}

// copyDir recursively copies a directory from src to dst.
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}
