package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/vpsfreecz/vpsf-status/internal/i18n/catalog"
	"golang.org/x/text/language"
)

const (
	langQueryParam = "lang"
	defaultLang    = "en"
)

//go:embed i18n/*.active.toml
var embeddedLocaleFiles embed.FS

type localeCatalog struct {
	bundle    *i18n.Bundle
	matcher   language.Matcher
	languages []languageInfo
	byCode    map[string]languageInfo
}

type languageInfo struct {
	Code string
	Tag  language.Tag
	Name string
}

type pageLocale struct {
	Code        string
	HTML        string
	Name        string
	SwitchLinks []languageSwitchLink

	localizer *i18n.Localizer
}

type languageSwitchLink struct {
	Code   string
	Label  string
	URL    string
	Active bool
	Title  string
}

var (
	defaultLocaleOnce sync.Once
	defaultLocale     *pageLocale
)

func (app *application) ensureLocales() {
	if app.locales == nil {
		app.locales = newLocaleCatalog()
	}
}

func newLocaleCatalog() *localeCatalog {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	mustAddMessages(bundle, language.English, catalog.Messages())

	infos := []languageInfo{{
		Code: defaultLang,
		Tag:  language.English,
		Name: nativeLanguageName(defaultLang),
	}}

	if err := fs.WalkDir(embeddedLocaleFiles, "i18n", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".active.toml") {
			return nil
		}

		code := strings.TrimSuffix(filepath.Base(path), ".active.toml")
		if code == defaultLang {
			return nil
		}

		data, err := embeddedLocaleFiles.ReadFile(path)
		if err != nil {
			return err
		}
		if _, err := bundle.ParseMessageFileBytes(data, code+".toml"); err != nil {
			return err
		}

		tag, err := language.Parse(code)
		if err != nil {
			return err
		}
		infos = append(infos, languageInfo{
			Code: code,
			Tag:  tag,
			Name: nativeLanguageName(code),
		})
		return nil
	}); err != nil {
		log.Fatalf("Unable to load translations: %v", err)
	}

	sort.Slice(infos, func(i, j int) bool {
		if infos[i].Code == defaultLang {
			return true
		}
		if infos[j].Code == defaultLang {
			return false
		}
		return infos[i].Code < infos[j].Code
	})

	tags := make([]language.Tag, len(infos))
	byCode := make(map[string]languageInfo, len(infos))
	for i, info := range infos {
		tags[i] = info.Tag
		byCode[info.Code] = info
	}

	return &localeCatalog{
		bundle:    bundle,
		matcher:   language.NewMatcher(tags),
		languages: infos,
		byCode:    byCode,
	}
}

func mustAddMessages(bundle *i18n.Bundle, tag language.Tag, messages []*i18n.Message) {
	if err := bundle.AddMessages(tag, messages...); err != nil {
		log.Fatalf("Unable to add source translations: %v", err)
	}
}

func nativeLanguageName(code string) string {
	switch code {
	case "en":
		return "English"
	case "cs":
		return "Česky"
	default:
		return code
	}
}

func (c *localeCatalog) localeForCode(code string, r *http.Request) (*pageLocale, bool) {
	info, ok := c.byCode[code]
	if !ok {
		return nil, false
	}
	return c.newPageLocale(info, r), true
}

func (c *localeCatalog) matchAcceptLanguage(header string) languageInfo {
	_, i := language.MatchStrings(c.matcher, header)
	if i >= 0 && i < len(c.languages) {
		return c.languages[i]
	}
	return c.byCode[defaultLang]
}

func (c *localeCatalog) newPageLocale(info languageInfo, r *http.Request) *pageLocale {
	loc := &pageLocale{
		Code:      info.Code,
		HTML:      info.Tag.String(),
		Name:      info.Name,
		localizer: i18n.NewLocalizer(c.bundle, info.Code),
	}

	loc.SwitchLinks = make([]languageSwitchLink, len(c.languages))
	for i, lang := range c.languages {
		loc.SwitchLinks[i] = languageSwitchLink{
			Code:   lang.Code,
			Label:  lang.Name,
			URL:    languageSwitchURL(r, lang.Code),
			Active: lang.Code == info.Code,
			Title:  loc.TD(catalog.MsgLanguageSwitchTo, map[string]any{"Language": lang.Name}),
		}
	}

	return loc
}

func languageSwitchURL(r *http.Request, code string) string {
	if r == nil || r.URL == nil {
		return "/?" + langQueryParam + "=" + url.QueryEscape(code)
	}

	u := *r.URL
	q := u.Query()
	q.Set(langQueryParam, code)
	u.RawQuery = q.Encode()
	return u.String()
}

func (l *pageLocale) T(id string) string {
	return l.TD(id, nil)
}

func (l *pageLocale) TD(id string, data any) string {
	if l == nil || l.localizer == nil {
		return id
	}

	ret, err := l.localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    id,
		TemplateData: data,
	})
	if err != nil {
		log.Printf("Unable to localize %s: %v", id, err)
		return id
	}
	return ret
}

func (l *pageLocale) URL(path string) string {
	q := url.Values{}
	q.Set(langQueryParam, l.codeOrDefault())
	if strings.Contains(path, "?") {
		u, err := url.Parse(path)
		if err == nil {
			q = u.Query()
			q.Set(langQueryParam, l.codeOrDefault())
			u.RawQuery = q.Encode()
			return u.String()
		}
	}
	return path + "?" + q.Encode()
}

func (l *pageLocale) codeOrDefault() string {
	if l == nil || l.Code == "" {
		return defaultLang
	}
	return l.Code
}

func defaultPageLocale() *pageLocale {
	defaultLocaleOnce.Do(func() {
		c := newLocaleCatalog()
		defaultLocale, _ = c.localeForCode(defaultLang, nil)
	})
	return defaultLocale
}

func (app *application) resolveHTMLLocale(w http.ResponseWriter, r *http.Request) (*pageLocale, bool) {
	app.ensureLocales()

	code := ""
	if r != nil && r.URL != nil {
		code = r.URL.Query().Get(langQueryParam)
	}
	if loc, ok := app.locales.localeForCode(code, r); ok {
		return loc, false
	}

	target := app.locales.matchAcceptLanguage("")
	if r != nil {
		target = app.locales.matchAcceptLanguage(r.Header.Get("Accept-Language"))
	}

	redirectToLanguage(w, r, target.Code)
	return nil, true
}

func redirectToLanguage(w http.ResponseWriter, r *http.Request, code string) {
	if w == nil || r == nil || r.URL == nil {
		return
	}

	u := *r.URL
	q := u.Query()
	q.Set(langQueryParam, code)
	u.RawQuery = q.Encode()
	w.Header().Add("Vary", "Accept-Language")
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func templateDict(values ...any) (map[string]any, error) {
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("dict requires an even number of arguments")
	}

	ret := make(map[string]any, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict key must be a string")
		}
		ret[key] = values[i+1]
	}
	return ret, nil
}

func localizedURL(path string, lang string, values url.Values) string {
	if values == nil {
		values = url.Values{}
	}
	values.Set(langQueryParam, lang)
	return path + "?" + values.Encode()
}
