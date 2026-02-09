package meta

import (
	"context"
	"sync"
)

var (
	langMapOnce sync.Once                    //nolint:gochecknoglobals // ensures SetLanguageMap is called once
	langMap     map[string]map[string]string //nolint:gochecknoglobals // for minimizing dependency injection across codebase
	defaultLang string                       //nolint:gochecknoglobals // for minimizing dependency injection across codebase
)

// SetLanguageMap sets the language map and the default language.
func SetLanguageMap(m map[string]map[string]string, defLang string) {
	langMapOnce.Do(func() {
		langMap = m
		defaultLang = defLang
	})
}

// L returns the localized value for the given language, falls back to any available value.
func L(m map[string]string, lang string) string {
	if v, ok := m[lang]; ok {
		return v
	}
	if v, ok := m[defaultLang]; ok {
		return v
	}
	for _, v := range m {
		return v
	}
	return ""
}

// LCtx returns the localized value using the language from the request context,
// falls back to any available value.
func LCtx(ctx context.Context, m map[string]string) string {
	return L(m, Find(ctx, AcceptLanguage))
}

// Tr returns the translated text for the given language.
// Falls back to the default language if the requested language is not found.
func Tr(text, lang string) string {
	if lang == "" {
		lang = defaultLang
	}

	if m, ok := langMap[lang]; ok {
		return getTranslationOrUntranslated(text, m)
	}

	return getTranslationOrUntranslated(text, langMap[defaultLang])
}

// TrCtx returns the translated text using the language from the request context.
func TrCtx(ctx context.Context, text string) string {
	return Tr(text, Find(ctx, AcceptLanguage))
}

func getTranslationOrUntranslated(text string, m map[string]string) string {
	res := m[text]

	if res == "" {
		return "[untranslated]: " + text
	}

	return res
}
