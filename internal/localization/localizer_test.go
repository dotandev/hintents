package localization

import (
	"testing"
)

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		name   string
		env    string
		expect Language
	}{
		{"english default", "", English},
		{"english explicit", "en", English},
		{"spanish", "es", Spanish},
		{"chinese", "zh", Chinese},
		{"invalid fallback", "fr", English},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ERST_LANG", tt.env)
			lang := detectLanguage()
			if lang != tt.expect {
				t.Errorf("expected %s, got %s", tt.expect, lang)
			}
		})
	}
}

func TestLocalizerSetLanguage(t *testing.T) {
	loc := New()

	err := loc.SetLanguage(Spanish)
	if err != nil {
		t.Errorf("failed to set valid language: %v", err)
	}

	if loc.GetLanguage() != Spanish {
		t.Error("language not updated")
	}

	err = loc.SetLanguage(Language("fr"))
	if err == nil {
		t.Error("expected error for unsupported language")
	}
}

func TestLocalizerRegisterMessages(t *testing.T) {
	loc := New()

	msgs := map[string]string{
		"greeting": "Hello",
		"farewell": "Goodbye",
	}

	err := loc.RegisterMessages(English, msgs)
	if err != nil {
		t.Errorf("failed to register messages: %v", err)
	}

	if loc.Get("greeting") != "Hello" {
		t.Error("message not retrieved correctly")
	}
}

func TestLocalizerFallback(t *testing.T) {
	loc := New()

	msgs := map[string]string{
		"key1": "English message",
	}

	loc.RegisterMessages(English, msgs)
	loc.RegisterMessages(Spanish, map[string]string{})

	loc.SetLanguage(Spanish)

	result := loc.Get("key1")
	if result != "English message" {
		t.Errorf("expected fallback to English, got: %s", result)
	}
}

func TestTranslateWithArgs(t *testing.T) {
	loc := New()

	msgs := map[string]string{
		"error.network": "invalid network: %s",
	}

	loc.RegisterMessages(English, msgs)

	result := loc.Translate("error.network", "testnet")
	if result != "invalid network: testnet" {
		t.Errorf("expected formatted message, got: %s", result)
	}
}

func TestLoadTranslations(t *testing.T) {
	err := LoadTranslations()
	if err != nil {
		t.Errorf("failed to load translations: %v", err)
	}

	if Get("cli.debug.short") == "" {
		t.Error("translation not loaded")
	}
}

func TestMissingKeyFallback(t *testing.T) {
	loc := New()
	result := loc.Get("nonexistent.key")
	if result != "nonexistent.key" {
		t.Errorf("expected key as fallback, got: %s", result)
	}
}
