// Package trend stores shared trend-analysis templates.
package trend

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const fileVersion = 1
const maxTemplates = 500
const maxPanels = 12
const maxTracesPerPanel = 500

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)

// TraceConfig is the serializable portion of a trend curve.
type TraceConfig struct {
	InstID    string `json:"inst_id"`
	Inst      string `json:"inst"`
	IOA       uint32 `json:"ioa"`
	Name      string `json:"name"`
	Unit      string `json:"unit"`
	Alias     string `json:"alias"`
	ColorIdx  int    `json:"color_idx"`
	PointType string `json:"point_type,omitempty"`
}

// Panel is a type-isolated trend panel in a template.
type Panel struct {
	Kind         string        `json:"kind"`
	TraceConfigs []TraceConfig `json:"trace_configs"`
}

// Template is a shared, server-side trend template.
type Template struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Panels    []Panel `json:"panels"`
	CreatedAt int64   `json:"created_at"`
	UpdatedAt int64   `json:"updated_at"`
}

type fileData struct {
	Version   int        `json:"version"`
	Templates []Template `json:"templates"`
}

// Store persists templates under the application's configuration directory.
type Store struct {
	mu        sync.RWMutex
	filePath  string
	templates []Template
}

// NewStore creates a template store at configDir/trends/templates.json.
func NewStore(configDir string) *Store {
	return NewStoreAt(filepath.Join(configDir, "trends", "templates.json"))
}

// NewStoreAt creates a template store at an explicit path. It is useful for tests.
func NewStoreAt(filePath string) *Store {
	return &Store{filePath: filePath, templates: make([]Template, 0)}
}

// Load reads templates. A missing file is treated as an empty store.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.templates = make([]Template, 0)
			return nil
		}
		return fmt.Errorf("read trend templates: %w", err)
	}

	var document fileData
	if err := json.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("parse trend templates: %w", err)
	}
	if document.Version != 0 && document.Version != fileVersion {
		return fmt.Errorf("unsupported trend template version %d", document.Version)
	}
	if err := validateTemplates(document.Templates); err != nil {
		return err
	}
	s.templates = cloneTemplates(document.Templates)
	return nil
}

// List returns a detached, stable copy sorted by name and then ID.
func (s *Store) List() []Template {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := cloneTemplates(s.templates)
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Name == result[j].Name {
			return result[i].ID < result[j].ID
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result
}

// Save creates or replaces a template and persists the complete document.
func (s *Store) Save(template Template) (Template, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	if template.ID == "" {
		template.ID = fmt.Sprintf("trend-%d", now)
	}
	if template.CreatedAt == 0 {
		template.CreatedAt = now
	}
	template.UpdatedAt = now
	if err := validateTemplate(template); err != nil {
		return Template{}, err
	}

	found := false
	for i := range s.templates {
		if s.templates[i].ID == template.ID {
			// Preserve the original creation timestamp when updating an existing item.
			if s.templates[i].CreatedAt != 0 {
				template.CreatedAt = s.templates[i].CreatedAt
			}
			s.templates[i] = cloneTemplate(template)
			found = true
			break
		}
	}
	if !found {
		if len(s.templates) >= maxTemplates {
			return Template{}, fmt.Errorf("too many trend templates (maximum %d)", maxTemplates)
		}
		s.templates = append(s.templates, cloneTemplate(template))
	}
	if err := s.saveLocked(); err != nil {
		return Template{}, err
	}
	return cloneTemplate(template), nil
}

// Delete removes a template by ID and persists the result.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, template := range s.templates {
		if template.ID == id {
			s.templates = append(s.templates[:i], s.templates[i+1:]...)
			return s.saveLocked()
		}
	}
	return os.ErrNotExist
}

func (s *Store) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0o755); err != nil {
		return fmt.Errorf("create trend config directory: %w", err)
	}
	document := fileData{Version: fileVersion, Templates: cloneTemplates(s.templates)}
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode trend templates: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(s.filePath), ".templates-*.tmp")
	if err != nil {
		return fmt.Errorf("create trend config temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write trend config temporary file: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return fmt.Errorf("set trend config permissions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close trend config temporary file: %w", err)
	}
	// On Unix rename replaces the old file atomically. Windows requires the
	// existing destination to be removed first; the temp file still prevents a
	// partially written JSON document from being observed.
	if err := os.Rename(tmpPath, s.filePath); err != nil {
		if err := os.Remove(s.filePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("replace trend config: %w", err)
		}
		if err := os.Rename(tmpPath, s.filePath); err != nil {
			return fmt.Errorf("install trend config: %w", err)
		}
	}
	return nil
}

func validateTemplates(templates []Template) error {
	if len(templates) > maxTemplates {
		return fmt.Errorf("too many trend templates (maximum %d)", maxTemplates)
	}
	seen := make(map[string]struct{}, len(templates))
	for _, template := range templates {
		if _, ok := seen[template.ID]; ok {
			return fmt.Errorf("duplicate trend template ID %q", template.ID)
		}
		seen[template.ID] = struct{}{}
		if err := validateTemplate(template); err != nil {
			return err
		}
	}
	return nil
}

func validateTemplate(template Template) error {
	if !idPattern.MatchString(template.ID) {
		return fmt.Errorf("invalid trend template ID %q", template.ID)
	}
	name := strings.TrimSpace(template.Name)
	if name == "" {
		return fmt.Errorf("trend template name is required")
	}
	if len([]rune(name)) > 200 {
		return fmt.Errorf("trend template name is too long")
	}
	if len(template.Panels) > maxPanels {
		return fmt.Errorf("too many panels in trend template (maximum %d)", maxPanels)
	}
	for _, panel := range template.Panels {
		switch panel.Kind {
		case "collection", "ao-control", "do-control":
		default:
			return fmt.Errorf("invalid trend panel kind %q", panel.Kind)
		}
		if len(panel.TraceConfigs) > maxTracesPerPanel {
			return fmt.Errorf("too many traces in trend panel (maximum %d)", maxTracesPerPanel)
		}
		for _, trace := range panel.TraceConfigs {
			if strings.TrimSpace(trace.InstID) == "" {
				return fmt.Errorf("trend trace instance ID is required")
			}
		}
	}
	return nil
}

func cloneTemplates(input []Template) []Template {
	result := make([]Template, len(input))
	for i := range input {
		result[i] = cloneTemplate(input[i])
	}
	return result
}

func cloneTemplate(input Template) Template {
	result := input
	result.Panels = make([]Panel, len(input.Panels))
	for i, panel := range input.Panels {
		result.Panels[i] = panel
		result.Panels[i].TraceConfigs = append([]TraceConfig(nil), panel.TraceConfigs...)
	}
	return result
}
