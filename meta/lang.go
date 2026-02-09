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
		return "UNTRANSLATED: " + text
	}

	return res
}
