package detail

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"iec104-sim/internal/model"
)

type AutoChangeStore struct {
	mu      sync.RWMutex
	baseDir string
	configs map[string]map[uint32]*model.AutoChangeConfig
}

func NewAutoChangeStore(configDir string) *AutoChangeStore {
	return &AutoChangeStore{
		baseDir: filepath.Join(configDir, "auto_changes"),
		configs: make(map[string]map[uint32]*model.AutoChangeConfig),
	}
}

func (s *AutoChangeStore) instancePath(instanceID string) string {
	return filepath.Join(s.baseDir, instanceID+".json")
}

func (s *AutoChangeStore) Load(instanceID string) (map[uint32]*model.AutoChangeConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.instancePath(instanceID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			s.configs[instanceID] = make(map[uint32]*model.AutoChangeConfig)
			return s.configs[instanceID], nil
		}
		return nil, err
	}

	var configs map[uint32]*model.AutoChangeConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return nil, err
	}
	if configs == nil {
		configs = make(map[uint32]*model.AutoChangeConfig)
	}
	s.configs[instanceID] = configs
	return configs, nil
}

func (s *AutoChangeStore) Save(instanceID string, configs map[uint32]*model.AutoChangeConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.baseDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(configs, "", "  ")
	if err != nil {
		return err
	}

	s.configs[instanceID] = configs
	return os.WriteFile(s.instancePath(instanceID), data, 0644)
}

func (s *AutoChangeStore) Get(instanceID string, ioa uint32) (*model.AutoChangeConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	instConfigs, ok := s.configs[instanceID]
	if !ok {
		return nil, false
	}
	cfg, ok := instConfigs[ioa]
	return cfg, ok
}

func (s *AutoChangeStore) Set(instanceID string, cfg *model.AutoChangeConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.configs[instanceID]; !ok {
		s.configs[instanceID] = make(map[uint32]*model.AutoChangeConfig)
	}
	s.configs[instanceID][cfg.PointIOA] = cfg
	return s.flush(instanceID)
}

func (s *AutoChangeStore) Delete(instanceID string, ioa uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	instConfigs, ok := s.configs[instanceID]
	if !ok {
		return nil
	}
	delete(instConfigs, ioa)
	return s.flush(instanceID)
}

func (s *AutoChangeStore) DeleteAll(instanceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.configs, instanceID)
	path := s.instancePath(instanceID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *AutoChangeStore) All(instanceID string) map[uint32]*model.AutoChangeConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	instConfigs, ok := s.configs[instanceID]
	if !ok {
		return nil
	}
	result := make(map[uint32]*model.AutoChangeConfig, len(instConfigs))
	for k, v := range instConfigs {
		cp := *v
		result[k] = &cp
	}
	return result
}

func (s *AutoChangeStore) flush(instanceID string) error {
	if err := os.MkdirAll(s.baseDir, 0755); err != nil {
		return err
	}
	instConfigs := s.configs[instanceID]
	data, err := json.MarshalIndent(instConfigs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.instancePath(instanceID), data, 0644)
}
