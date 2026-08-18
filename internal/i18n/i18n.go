package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"sync"
)

//go:embed locales/*.json
var localeFS embed.FS

type Locale string

const (
	LocaleES Locale = "es"
	LocaleEN Locale = "en"
	LocalePT Locale = "pt"
	LocaleFR Locale = "fr"
	LocaleDE Locale = "de"
	LocaleIT Locale = "it"
)

type I18n struct {
	mu      sync.RWMutex
	current Locale
	strings map[string]string
}

var global *I18n

func init() {
	global = &I18n{
		current: LocaleEN,
		strings: make(map[string]string),
	}
	if err := global.Load(LocaleEN); err != nil {
		_ = err
	}
}

func Get(key string, args ...interface{}) string {
	return global.T(key, args...)
}

func T(key string, args ...interface{}) string {
	return global.T(key, args...)
}

func SetLocale(locale Locale) error {
	return global.SetLocale(locale)
}

func CurrentLocale() Locale {
	return global.Current()
}

func AvailableLocales() []Locale {
	return []Locale{LocaleES, LocaleEN, LocalePT, LocaleFR, LocaleDE, LocaleIT}
}

func (i *I18n) SetLocale(locale Locale) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if err := i.load(locale); err != nil {
		return err
	}
	i.current = locale
	return nil
}

func (i *I18n) Current() Locale {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.current
}

func (i *I18n) Load(locale Locale) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.load(locale)
}

func (i *I18n) load(locale Locale) error {
	data, err := localeFS.ReadFile(fmt.Sprintf("locales/%s.json", locale))
	if err != nil {
		return fmt.Errorf("i18n: locale %s not found: %w", locale, err)
	}

	var translations map[string]string
	if err := json.Unmarshal(data, &translations); err != nil {
		return fmt.Errorf("i18n: parse %s: %w", locale, err)
	}

	for k, v := range translations {
		i.strings[k] = v
	}
	return nil
}

func (i *I18n) T(key string, args ...interface{}) string {
	i.mu.RLock()
	val, ok := i.strings[key]
	i.mu.RUnlock()

	if !ok {
		return key
	}

	if len(args) > 0 {
		return fmt.Sprintf(val, args...)
	}
	return val
}
