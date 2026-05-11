package storage

import (
	"encoding/json"
	"os"
	"sync"

	"iec104-sim/internal/model"
)

// ConfigStore persists instance configurations as a JSON file.
type ConfigStore struct {
	filePath string
	mu       sync.RWMutex
	configs  []model.InstanceConfig
	dirty    bool
}

// NewConfigStore creates a ConfigStore backed by the given file path.
func NewConfigStore(filePath string) *ConfigStore {
	return &ConfigStore{
		filePath: filePath,
		configs:  make([]model.InstanceConfig, 0),
	}
}

// Load reads instance configurations from the JSON file.
func (s *ConfigStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.configs = make([]model.InstanceConfig, 0)
			return nil
		}
		return err
	}

	if err := json.Unmarshal(data, &s.configs); err != nil {
		return err
	}
	s.dirty = false
	return nil
}

// Save persists instance configurations to the JSON file.
func (s *ConfigStore) Save() error {
	s.mu.RLock()
	data, err := json.MarshalIndent(s.configs, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		return err
	}
	s.mu.Lock()
	s.dirty = false
	s.mu.Unlock()
	return nil
}

// List returns all instance configurations.
func (s *ConfigStore) List() []model.InstanceConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]model.InstanceConfig, len(s.configs))
	copy(result, s.configs)
	return result
}

// Get returns an instance configuration by ID.
func (s *ConfigStore) Get(id string) (model.InstanceConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.configs {
		if c.ID == id {
			return c, true
		}
	}
	return model.InstanceConfig{}, false
}

// Add appends a new instance configuration and persists.
func (s *ConfigStore) Add(cfg model.InstanceConfig) error {
	s.mu.Lock()
	s.configs = append(s.configs, cfg)
	s.dirty = true
	s.mu.Unlock()
	return s.Save()
}

// Update replaces an instance configuration by ID and persists.
func (s *ConfigStore) Update(cfg model.InstanceConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, c := range s.configs {
		if c.ID == cfg.ID {
			s.configs[i] = cfg
			s.dirty = true
			return s.Save()
		}
	}
	return nil
}

// Delete removes an instance configuration by ID and persists.
func (s *ConfigStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, c := range s.configs {
		if c.ID == id {
			s.configs = append(s.configs[:i], s.configs[i+1:]...)
			s.dirty = true
			return s.Save()
		}
	}
	return nil
}

// Count returns the number of instance configurations.
func (s *ConfigStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.configs)
}

// IsDirty returns whether there are unsaved changes.
func (s *ConfigStore) IsDirty() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dirty
}
