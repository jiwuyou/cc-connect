package core

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

var frontendIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,79}$`)

// FrontendAppRegistry stores user-facing application entry points separately
// from config.toml so apps and frontend slots can be changed without restart.
type FrontendAppRegistry struct {
	path string
	mu   sync.RWMutex
}

type frontendRegistryFile struct {
	Version int                    `json:"version"`
	Apps    map[string]FrontendApp `json:"apps"`
}

// FrontendApp groups stable/beta/dev frontend slots for one application.
type FrontendApp struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	Project     string                  `json:"project"`
	Description string                  `json:"description,omitempty"`
	Metadata    map[string]string       `json:"metadata,omitempty"`
	Slots       map[string]FrontendSlot `json:"slots,omitempty"`
	CreatedAt   time.Time               `json:"created_at"`
	UpdatedAt   time.Time               `json:"updated_at"`
}

// FrontendSlot describes one user-facing frontend entry point.
type FrontendSlot struct {
	Slot            string            `json:"slot"`
	Label           string            `json:"label,omitempty"`
	URL             string            `json:"url"`
	APIBase         string            `json:"api_base,omitempty"`
	AdapterPlatform string            `json:"adapter_platform,omitempty"`
	Enabled         bool              `json:"enabled"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

type FrontendAppUpdate struct {
	Name        *string
	Project     *string
	Description *string
	Metadata    map[string]string
}

type FrontendSlotUpdate struct {
	Label           *string
	URL             *string
	APIBase         *string
	AdapterPlatform *string
	Enabled         *bool
	Metadata        map[string]string
}

func NewFrontendAppRegistry(path string) *FrontendAppRegistry {
	return &FrontendAppRegistry{path: path}
}

func (r *FrontendAppRegistry) Path() string {
	if r == nil {
		return ""
	}
	return r.path
}

func (r *FrontendAppRegistry) ListApps() ([]FrontendApp, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := r.loadLocked()
	if err != nil {
		return nil, err
	}
	apps := make([]FrontendApp, 0, len(data.Apps))
	for _, app := range data.Apps {
		apps = append(apps, app)
	}
	sort.Slice(apps, func(i, j int) bool {
		return strings.ToLower(apps[i].ID) < strings.ToLower(apps[j].ID)
	})
	return apps, nil
}

func (r *FrontendAppRegistry) GetApp(id string) (FrontendApp, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id = normalizeFrontendID(id)
	data, err := r.loadLocked()
	if err != nil {
		return FrontendApp{}, false, err
	}
	app, ok := data.Apps[id]
	return app, ok, nil
}

func (r *FrontendAppRegistry) CreateApp(app FrontendApp) (FrontendApp, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := r.loadLocked()
	if err != nil {
		return FrontendApp{}, err
	}
	if app.ID == "" {
		app.ID = slugFrontendID(app.Name)
	}
	app.ID = normalizeFrontendID(app.ID)
	if err := validateFrontendApp(app); err != nil {
		return FrontendApp{}, err
	}
	if _, exists := data.Apps[app.ID]; exists {
		return FrontendApp{}, fmt.Errorf("frontend app %q already exists", app.ID)
	}
	now := time.Now().UTC()
	app.CreatedAt = now
	app.UpdatedAt = now
	if app.Slots == nil {
		app.Slots = map[string]FrontendSlot{}
	}
	data.Apps[app.ID] = app
	if err := r.saveLocked(data); err != nil {
		return FrontendApp{}, err
	}
	return app, nil
}

func (r *FrontendAppRegistry) UpdateApp(id string, update FrontendAppUpdate) (FrontendApp, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id = normalizeFrontendID(id)
	data, err := r.loadLocked()
	if err != nil {
		return FrontendApp{}, err
	}
	app, ok := data.Apps[id]
	if !ok {
		return FrontendApp{}, fmt.Errorf("frontend app %q not found", id)
	}
	if update.Name != nil {
		app.Name = strings.TrimSpace(*update.Name)
	}
	if update.Project != nil {
		app.Project = strings.TrimSpace(*update.Project)
	}
	if update.Description != nil {
		app.Description = strings.TrimSpace(*update.Description)
	}
	if update.Metadata != nil {
		app.Metadata = update.Metadata
	}
	if err := validateFrontendApp(app); err != nil {
		return FrontendApp{}, err
	}
	app.UpdatedAt = time.Now().UTC()
	data.Apps[id] = app
	if err := r.saveLocked(data); err != nil {
		return FrontendApp{}, err
	}
	return app, nil
}

func (r *FrontendAppRegistry) DeleteApp(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	id = normalizeFrontendID(id)
	data, err := r.loadLocked()
	if err != nil {
		return err
	}
	if _, ok := data.Apps[id]; !ok {
		return fmt.Errorf("frontend app %q not found", id)
	}
	delete(data.Apps, id)
	return r.saveLocked(data)
}

func (r *FrontendAppRegistry) ListSlots(appID string) ([]FrontendSlot, error) {
	app, ok, err := r.GetApp(appID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("frontend app %q not found", appID)
	}
	slots := make([]FrontendSlot, 0, len(app.Slots))
	for _, slot := range app.Slots {
		slots = append(slots, slot)
	}
	sort.Slice(slots, func(i, j int) bool {
		return strings.ToLower(slots[i].Slot) < strings.ToLower(slots[j].Slot)
	})
	return slots, nil
}

func (r *FrontendAppRegistry) UpsertSlot(appID string, slot FrontendSlot) (FrontendSlot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	appID = normalizeFrontendID(appID)
	data, err := r.loadLocked()
	if err != nil {
		return FrontendSlot{}, err
	}
	app, ok := data.Apps[appID]
	if !ok {
		return FrontendSlot{}, fmt.Errorf("frontend app %q not found", appID)
	}
	slot.Slot = normalizeFrontendID(slot.Slot)
	if err := validateFrontendSlot(slot); err != nil {
		return FrontendSlot{}, err
	}
	now := time.Now().UTC()
	if app.Slots == nil {
		app.Slots = map[string]FrontendSlot{}
	}
	if existing, exists := app.Slots[slot.Slot]; exists {
		slot.CreatedAt = existing.CreatedAt
	} else {
		slot.CreatedAt = now
	}
	slot.UpdatedAt = now
	app.Slots[slot.Slot] = slot
	app.UpdatedAt = now
	data.Apps[appID] = app
	if err := r.saveLocked(data); err != nil {
		return FrontendSlot{}, err
	}
	return slot, nil
}

func (r *FrontendAppRegistry) UpdateSlot(appID, slotName string, update FrontendSlotUpdate) (FrontendSlot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	appID = normalizeFrontendID(appID)
	slotName = normalizeFrontendID(slotName)
	data, err := r.loadLocked()
	if err != nil {
		return FrontendSlot{}, err
	}
	app, ok := data.Apps[appID]
	if !ok {
		return FrontendSlot{}, fmt.Errorf("frontend app %q not found", appID)
	}
	slot, ok := app.Slots[slotName]
	if !ok {
		return FrontendSlot{}, fmt.Errorf("frontend slot %q not found", slotName)
	}
	if update.Label != nil {
		slot.Label = strings.TrimSpace(*update.Label)
	}
	if update.URL != nil {
		slot.URL = strings.TrimSpace(*update.URL)
	}
	if update.APIBase != nil {
		slot.APIBase = strings.TrimSpace(*update.APIBase)
	}
	if update.AdapterPlatform != nil {
		slot.AdapterPlatform = strings.TrimSpace(*update.AdapterPlatform)
	}
	if update.Enabled != nil {
		slot.Enabled = *update.Enabled
	}
	if update.Metadata != nil {
		slot.Metadata = update.Metadata
	}
	if err := validateFrontendSlot(slot); err != nil {
		return FrontendSlot{}, err
	}
	now := time.Now().UTC()
	slot.UpdatedAt = now
	app.Slots[slotName] = slot
	app.UpdatedAt = now
	data.Apps[appID] = app
	if err := r.saveLocked(data); err != nil {
		return FrontendSlot{}, err
	}
	return slot, nil
}

func (r *FrontendAppRegistry) DeleteSlot(appID, slotName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	appID = normalizeFrontendID(appID)
	slotName = normalizeFrontendID(slotName)
	data, err := r.loadLocked()
	if err != nil {
		return err
	}
	app, ok := data.Apps[appID]
	if !ok {
		return fmt.Errorf("frontend app %q not found", appID)
	}
	if _, ok := app.Slots[slotName]; !ok {
		return fmt.Errorf("frontend slot %q not found", slotName)
	}
	delete(app.Slots, slotName)
	app.UpdatedAt = time.Now().UTC()
	data.Apps[appID] = app
	return r.saveLocked(data)
}

func (r *FrontendAppRegistry) PromoteSlot(appID, sourceSlot, targetSlot string) (FrontendSlot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	appID = normalizeFrontendID(appID)
	sourceSlot = normalizeFrontendID(sourceSlot)
	targetSlot = normalizeFrontendID(targetSlot)
	if targetSlot == "" {
		targetSlot = "stable"
	}
	data, err := r.loadLocked()
	if err != nil {
		return FrontendSlot{}, err
	}
	app, ok := data.Apps[appID]
	if !ok {
		return FrontendSlot{}, fmt.Errorf("frontend app %q not found", appID)
	}
	source, ok := app.Slots[sourceSlot]
	if !ok {
		return FrontendSlot{}, fmt.Errorf("frontend slot %q not found", sourceSlot)
	}
	if app.Slots == nil {
		app.Slots = map[string]FrontendSlot{}
	}
	promoted := source
	promoted.Slot = targetSlot
	if promoted.Label == "" || promoted.Label == source.Label {
		promoted.Label = targetSlot
	}
	now := time.Now().UTC()
	if existing, exists := app.Slots[targetSlot]; exists {
		promoted.CreatedAt = existing.CreatedAt
	} else {
		promoted.CreatedAt = now
	}
	promoted.UpdatedAt = now
	promoted.Metadata = cloneStringMap(source.Metadata)
	if promoted.Metadata == nil {
		promoted.Metadata = map[string]string{}
	}
	promoted.Metadata["promoted_from"] = sourceSlot
	promoted.Metadata["promoted_at"] = now.Format(time.RFC3339)
	if err := validateFrontendSlot(promoted); err != nil {
		return FrontendSlot{}, err
	}
	app.Slots[targetSlot] = promoted
	app.UpdatedAt = now
	data.Apps[appID] = app
	if err := r.saveLocked(data); err != nil {
		return FrontendSlot{}, err
	}
	return promoted, nil
}

func (r *FrontendAppRegistry) loadLocked() (frontendRegistryFile, error) {
	data := frontendRegistryFile{Version: 1, Apps: map[string]FrontendApp{}}
	if r == nil || strings.TrimSpace(r.path) == "" {
		return data, fmt.Errorf("frontend app registry path is not configured")
	}
	raw, err := os.ReadFile(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return data, nil
		}
		return data, fmt.Errorf("read frontend app registry: %w", err)
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return data, nil
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return data, fmt.Errorf("parse frontend app registry: %w", err)
	}
	if data.Version == 0 {
		data.Version = 1
	}
	if data.Apps == nil {
		data.Apps = map[string]FrontendApp{}
	}
	return data, nil
}

func (r *FrontendAppRegistry) saveLocked(data frontendRegistryFile) error {
	if r == nil || strings.TrimSpace(r.path) == "" {
		return fmt.Errorf("frontend app registry path is not configured")
	}
	if data.Version == 0 {
		data.Version = 1
	}
	if data.Apps == nil {
		data.Apps = map[string]FrontendApp{}
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return fmt.Errorf("create frontend app registry dir: %w", err)
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode frontend app registry: %w", err)
	}
	raw = append(raw, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(r.path), ".frontend-apps-*.tmp")
	if err != nil {
		return fmt.Errorf("create frontend app registry temp file: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write frontend app registry: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("sync frontend app registry: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close frontend app registry: %w", err)
	}
	if err := os.Rename(tmpPath, r.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replace frontend app registry: %w", err)
	}
	return nil
}

func validateFrontendApp(app FrontendApp) error {
	if !frontendIDPattern.MatchString(app.ID) {
		return fmt.Errorf("invalid frontend app id %q", app.ID)
	}
	if strings.TrimSpace(app.Name) == "" {
		return fmt.Errorf("frontend app name is required")
	}
	if strings.TrimSpace(app.Project) == "" {
		return fmt.Errorf("frontend app project is required")
	}
	return nil
}

func validateFrontendSlot(slot FrontendSlot) error {
	if !frontendIDPattern.MatchString(slot.Slot) {
		return fmt.Errorf("invalid frontend slot %q", slot.Slot)
	}
	if strings.TrimSpace(slot.URL) == "" {
		return fmt.Errorf("frontend slot url is required")
	}
	return nil
}

func normalizeFrontendID(value string) string {
	return strings.TrimSpace(value)
}

func slugFrontendID(value string) string {
	raw := strings.ToLower(strings.TrimSpace(value))
	if raw == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '_' || r == '-':
			if !lastDash && b.Len() > 0 {
				b.WriteRune(r)
				lastDash = r == '-'
			}
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	result := strings.Trim(b.String(), "-_")
	if len(result) > 80 {
		result = strings.Trim(result[:80], "-_")
	}
	return result
}

func cloneStringMap(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}
	output := make(map[string]string, len(input))
	for k, v := range input {
		output[k] = v
	}
	return output
}
