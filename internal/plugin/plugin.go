package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"plugin"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cam/cine-cli/internal/core"
)

type PluginType string

const (
	PluginTypeProvider PluginType = "provider"
	PluginTypeScraper  PluginType = "scraper"
	PluginTypeResolver PluginType = "resolver"
	PluginTypeHook     PluginType = "hook"
)

type PluginManifest struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Type        PluginType        `json:"type"`
	Author      string            `json:"author"`
	Description string            `json:"description"`
	Entrypoint  string            `json:"entrypoint"`
	Config      map[string]string `json:"config,omitempty"`
	Enabled     bool              `json:"enabled"`
}

type PluginInstance struct {
	Manifest PluginManifest
	Impl     interface{}
}

type Registry struct {
	mu      sync.RWMutex
	plugins map[string]*PluginInstance
	dir     string
}

func NewRegistry(pluginDir string) *Registry {
	_ = os.MkdirAll(pluginDir, 0755)
	return &Registry{
		plugins: make(map[string]*PluginInstance),
		dir:     pluginDir,
	}
}

func (r *Registry) Discover(ctx context.Context) error {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pluginDir := filepath.Join(r.dir, entry.Name())
		manifestPath := filepath.Join(pluginDir, "manifest.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}

		var manifest PluginManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			continue
		}
		if manifest.Name == "" {
			manifest.Name = entry.Name()
		}

		var impl interface{}
		if manifest.Enabled {
			impl = r.loadImpl(pluginDir, manifest)
		}
		r.Register(manifest, impl)
	}
	return nil
}

// loadImpl tries to load a Go plugin (.so) or a script-based ExternalProvider.
func (r *Registry) loadImpl(pluginDir string, manifest PluginManifest) interface{} {
	ep := manifest.Entrypoint
	if ep == "" {
		// auto-detect
		if _, err := os.Stat(filepath.Join(pluginDir, "plugin.so")); err == nil {
			ep = "plugin.so"
		} else if _, err := os.Stat(filepath.Join(pluginDir, "script.sh")); err == nil {
			ep = "script.sh"
		}
	}
	if ep == "" {
		return nil
	}

	full := filepath.Join(pluginDir, ep)
	if strings.HasSuffix(ep, ".so") {
		p, err := LoadGoPlugin(full)
		if err != nil {
			return nil
		}
		return p
	}

	// Treat as executable script
	if st, err := os.Stat(full); err == nil && !st.IsDir() {
		_ = os.Chmod(full, 0755)
		return NewExternalProvider(manifest.Name, 50, full)
	}
	return nil
}

func (r *Registry) Register(manifest PluginManifest, impl interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.plugins[manifest.Name] = &PluginInstance{
		Manifest: manifest,
		Impl:     impl,
	}
}

func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.plugins, name)
}

func (r *Registry) Get(name string) (*PluginInstance, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	return p, ok
}

func (r *Registry) List() []PluginInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]PluginInstance, 0, len(r.plugins))
	for _, p := range r.plugins {
		result = append(result, *p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Manifest.Name < result[j].Manifest.Name
	})
	return result
}

func (r *Registry) ListByType(t PluginType) []PluginInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]PluginInstance, 0)
	for _, p := range r.plugins {
		if p.Manifest.Type == t {
			result = append(result, *p)
		}
	}
	return result
}

func (r *Registry) Enable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %s not found", name)
	}
	p.Manifest.Enabled = true
	return r.saveManifest(p.Manifest)
}

func (r *Registry) Disable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %s not found", name)
	}
	p.Manifest.Enabled = false
	return r.saveManifest(p.Manifest)
}

func (r *Registry) saveManifest(m PluginManifest) error {
	dir := filepath.Join(r.dir, m.Name)
	_ = os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "manifest.json")
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func LoadGoPlugin(path string) (interface{}, error) {
	p, err := plugin.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open plugin %s: %w", path, err)
	}

	sym, err := p.Lookup("Provider")
	if err != nil {
		return nil, fmt.Errorf("plugin missing Provider symbol: %w", err)
	}

	provider, ok := sym.(core.Provider)
	if !ok {
		return nil, fmt.Errorf("plugin Provider symbol does not implement core.Provider")
	}

	return provider, nil
}

const PluginDoc = `# Creating a cine-cli Plugin

## Structure

my-plugin/
  manifest.json    # Plugin metadata
  plugin.so        # Go plugin (optional, for Go plugins)
  script.sh        # Shell script (optional, for script-based plugins)

## manifest.json format

{
  "name": "my-provider",
  "version": "1.0.0",
  "type": "provider",
  "author": "Your Name",
  "description": "My custom provider",
  "entrypoint": "plugin.so",
  "config": {
    "api_key": ""
  },
  "enabled": true
}

## Go Plugin (plugin.so)

Must export a symbol named "Provider" that implements core.Provider:

  type Provider interface {
      Name() string
      Priority() int
      Search(ctx context.Context, query string) ([]MediaRef, error)
      GetStream(ctx context.Context, ref MediaRef) (*Stream, error)
  }

Build with: go build -buildmode=plugin -o plugin.so .

## Script Plugin (script.sh)

The script receives commands as first argument:
  - search <query>   → must output JSON array of MediaRef
  - stream <ref-json> → must output JSON of Stream

## Installation

Place the plugin directory in:
  ~/.config/cine-cli/plugins/

Enable/disable with:
  cine plugin enable <name>
  cine plugin disable <name>
`

type ExternalProvider struct {
	name     string
	priority int
	cmd      string
}

func NewExternalProvider(name string, priority int, cmd string) *ExternalProvider {
	return &ExternalProvider{
		name:     name,
		priority: priority,
		cmd:      cmd,
	}
}

func (p *ExternalProvider) Name() string  { return p.name }
func (p *ExternalProvider) Priority() int { return p.priority }

func (p *ExternalProvider) Search(ctx context.Context, query string) ([]core.MediaRef, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("%s search %q", p.cmd, query))
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("external plugin search: %w", err)
	}

	var refs []core.MediaRef
	if err := json.Unmarshal(out, &refs); err != nil {
		return nil, err
	}
	return refs, nil
}

func (p *ExternalProvider) GetStream(ctx context.Context, ref core.MediaRef) (*core.Stream, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	refJSON, _ := json.Marshal(ref)
	cmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("%s stream '%s'", p.cmd, string(refJSON)))
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("external plugin stream: %w", err)
	}

	var stream core.Stream
	if err := json.Unmarshal(out, &stream); err != nil {
		return nil, err
	}
	return &stream, nil
}

func ValidatePluginDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	return nil
}

func LoadPluginManifest(dir string) (*PluginManifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var m PluginManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func HasGoPlugin(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && strings.HasSuffix(path, ".so")
}

func HasScriptPlugin(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "script.sh"))
	return err == nil && !info.IsDir()
}
