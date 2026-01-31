// Copyright 2026 dotandev
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryClear(t *testing.T) {
	r := NewRegistry()

	r.mu.Lock()
	r.loader.plugins["test"] = &mockDecoder{
		name:    "test",
		version: "1.0.0",
	}
	r.mu.Unlock()

	r.Clear()

	if len(r.ListPlugins()) != 0 {
		t.Errorf("registry not cleared properly")
	}
}

func TestConcurrentAccess(t *testing.T) {
	r := NewRegistry()

	r.mu.Lock()
	r.loader.plugins["concurrent"] = &mockDecoder{
		name:    "concurrent",
		version: "1.0.0",
	}
	r.mu.Unlock()

	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			r.ListPlugins()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestErrorHandling(t *testing.T) {
	r := NewRegistry()

	_, _, err := r.FindAndDecode("nonexistent.type", []byte("data"))
	if err == nil {
		t.Errorf("expected error for nonexistent event type")
	}
}

func TestManagerWithInvalidDir(t *testing.T) {
	m, err := NewManager("/nonexistent/path")
	if err != nil {
		t.Errorf("manager creation should not fail immediately: %v", err)
	}

	err = m.Initialize()
	if err == nil {
		t.Errorf("initialize should fail with nonexistent directory")
	}
}

func TestPluginMetadata(t *testing.T) {
	mock := &mockDecoder{
		name:    "meta-test",
		version: "1.5.0",
	}

	meta := mock.Metadata()

	if meta.Name != "meta-test" {
		t.Errorf("expected name meta-test, got %s", meta.Name)
	}
	if meta.Version != "1.5.0" {
		t.Errorf("expected version 1.5.0, got %s", meta.Version)
	}
	if meta.APIVersion != Version {
		t.Errorf("expected API version %s, got %s", Version, meta.APIVersion)
	}
}

func TestLoaderList(t *testing.T) {
	l := NewLoader()

	l.mu.Lock()
	l.plugins["plugin1"] = &mockDecoder{name: "plugin1", version: "1.0.0"}
	l.plugins["plugin2"] = &mockDecoder{name: "plugin2", version: "1.0.0"}
	l.plugins["plugin3"] = &mockDecoder{name: "plugin3", version: "1.0.0"}
	l.mu.Unlock()

	names := l.List()
	if len(names) != 3 {
		t.Errorf("expected 3 plugins, got %d", len(names))
	}

	nameMap := make(map[string]bool)
	for _, n := range names {
		nameMap[n] = true
	}

	expected := []string{"plugin1", "plugin2", "plugin3"}
	for _, e := range expected {
		if !nameMap[e] {
			t.Errorf("expected plugin %s not found", e)
		}
	}
}

func TestDecoderJSON(t *testing.T) {
	mock := &mockDecoder{
		name:    "json-test",
		version: "1.0.0",
	}

	result, err := mock.Decode([]byte(`{"test": "data"}`))
	if err != nil {
		t.Errorf("decode failed: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(result, &decoded); err != nil {
		t.Errorf("failed to unmarshal result: %v", err)
	}

	if decoded["decoded"] != true {
		t.Errorf("expected decoded=true in result")
	}
}

func BenchmarkRegistryFindPlugin(b *testing.B) {
	r := NewRegistry()

	r.mu.Lock()
	for i := 0; i < 100; i++ {
		name := "plugin" + string(rune(i))
		r.loader.plugins[name] = &mockDecoder{
			name:       name,
			version:    "1.0.0",
			canDecodes: map[string]bool{"type.1": true},
		}
	}
	r.mu.Unlock()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.loader.FindForEvent("type.1")
	}
}

func BenchmarkDecodeOperation(b *testing.B) {
	r := NewRegistry()

	r.mu.Lock()
	r.loader.plugins["bench"] = &mockDecoder{
		name:       "bench",
		version:    "1.0.0",
		canDecodes: map[string]bool{"bench.type": true},
	}
	r.mu.Unlock()

	data, _ := json.Marshal(map[string]string{"test": "data"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Decode("bench", "bench.type", data)
	}
}

func setupPluginDir(t *testing.T) string {
	tmpdir, err := os.MkdirTemp("", "plugins-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tmpdir)
	})
	return tmpdir
}

func TestManagerBaseDir(t *testing.T) {
	tmpdir := setupPluginDir(t)

	m, err := NewManager(tmpdir)
	if err != nil {
		t.Errorf("failed to create manager: %v", err)
	}

	if m.baseDir != tmpdir {
		t.Errorf("expected baseDir %s, got %s", tmpdir, m.baseDir)
	}
}

func TestRegistryLoadDirectory(t *testing.T) {
	tmpdir := setupPluginDir(t)
	pluginDir := filepath.Join(tmpdir, "plugins")
	os.Mkdir(pluginDir, 0755)

	r := NewRegistry()
	err := r.LoadFromDirectory(pluginDir)

	if err != nil {
		t.Logf("empty directory returned: %v", err)
	}
}

func TestRaceConditions(t *testing.T) {
	r := NewRegistry()

	r.mu.Lock()
	r.loader.plugins["race-test"] = &mockDecoder{
		name:       "race-test",
		version:    "1.0.0",
		canDecodes: map[string]bool{"race.event": true},
	}
	r.mu.Unlock()

	done := make(chan bool)
	data, _ := json.Marshal(map[string]string{"test": "race"})

	for i := 0; i < 50; i++ {
		go func(idx int) {
			if idx%2 == 0 {
				r.ListPlugins()
			} else {
				r.Decode("race-test", "race.event", data)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 50; i++ {
		<-done
	}
}

func TestPluginGetNonexistent(t *testing.T) {
	r := NewRegistry()

	plugins := r.ListPlugins()
	if len(plugins) != 0 {
		t.Errorf("expected empty plugin list")
	}
}

func BenchmarkConcurrentDecode(b *testing.B) {
	r := NewRegistry()

	r.mu.Lock()
	r.loader.plugins["concurrent-bench"] = &mockDecoder{
		name:       "concurrent-bench",
		version:    "1.0.0",
		canDecodes: map[string]bool{"concurrent.event": true},
	}
	r.mu.Unlock()

	data, _ := json.Marshal(map[string]string{"data": "test"})
	done := make(chan bool, 1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		go func() {
			r.Decode("concurrent-bench", "concurrent.event", data)
			done <- true
		}()
		<-done
	}
}

func TestPluginLifecycle(t *testing.T) {
	r := NewRegistry()

	mock := &mockDecoder{
		name:       "lifecycle",
		version:    "1.0.0",
		canDecodes: map[string]bool{"lifecycle.event": true},
	}

	r.mu.Lock()
	r.loader.plugins["lifecycle"] = mock
	r.mu.Unlock()

	plugins := r.ListPlugins()
	if len(plugins) != 1 {
		t.Errorf("expected 1 plugin after registration")
	}

	data, _ := json.Marshal(map[string]string{"test": "lifecycle"})
	result, err := r.Decode("lifecycle", "lifecycle.event", data)
	if err != nil {
		t.Errorf("decode failed: %v", err)
	}
	if !json.Valid(result) {
		t.Errorf("result is not valid JSON")
	}

	r.Clear()
	if len(r.ListPlugins()) != 0 {
		t.Errorf("plugins not cleared")
	}
}
