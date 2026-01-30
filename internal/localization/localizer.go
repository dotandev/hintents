// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package localization

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

type Language string

const (
	English Language = "en"
	Spanish Language = "es"
	Chinese Language = "zh"
)

var supported = map[Language]bool{
	English: true,
	Spanish: true,
	Chinese: true,
}

type Localizer struct {
	mu          sync.RWMutex
	lang        Language
	messages    map[Language]map[string]string
	defaultLang Language
}

func New() *Localizer {
	lang := detectLanguage()
	return &Localizer{
		lang:        lang,
		messages:    make(map[Language]map[string]string),
		defaultLang: English,
	}
}

func detectLanguage() Language {
	envLang := os.Getenv("ERST_LANG")
	if envLang == "" {
		return English
	}

	lang := Language(strings.ToLower(strings.TrimSpace(envLang)))
	if supported[lang] {
		return lang
	}

	return English
}

func (l *Localizer) SetLanguage(lang Language) error {
	if !supported[lang] {
		return fmt.Errorf("unsupported language: %s", lang)
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.lang = lang
	return nil
}

func (l *Localizer) GetLanguage() Language {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.lang
}

func (l *Localizer) RegisterMessages(lang Language, messages map[string]string) error {
	if !supported[lang] {
		return fmt.Errorf("unsupported language: %s", lang)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.messages[lang] == nil {
		l.messages[lang] = make(map[string]string)
	}

	for key, msg := range messages {
		l.messages[lang][key] = msg
	}

	return nil
}

func (l *Localizer) Get(key string) string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if msg, ok := l.messages[l.lang][key]; ok {
		return msg
	}

	if msg, ok := l.messages[l.defaultLang][key]; ok {
		return msg
	}

	return key
}

func (l *Localizer) GetForLang(lang Language, key string) string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if msg, ok := l.messages[lang][key]; ok {
		return msg
	}

	if msg, ok := l.messages[l.defaultLang][key]; ok {
		return msg
	}

	return key
}

func (l *Localizer) Translate(key string, args ...interface{}) string {
	template := l.Get(key)
	if len(args) > 0 {
		return fmt.Sprintf(template, args...)
	}
	return template
}

func (l *Localizer) TranslateForLang(lang Language, key string, args ...interface{}) string {
	template := l.GetForLang(lang, key)
	if len(args) > 0 {
		return fmt.Sprintf(template, args...)
	}
	return template
}

var globalLocalizer = New()

func Get(key string) string {
	return globalLocalizer.Get(key)
}

func Translate(key string, args ...interface{}) string {
	return globalLocalizer.Translate(key, args...)
}

func SetLanguage(lang Language) error {
	return globalLocalizer.SetLanguage(lang)
}

func RegisterMessages(lang Language, messages map[string]string) error {
	return globalLocalizer.RegisterMessages(lang, messages)
}
