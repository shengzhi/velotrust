package i18n

import (
	"bytes"
	"embed"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/pelletier/go-toml/v2"
	"golang.org/x/text/language"
)

//go:embed locales
var dir embed.FS

var I18n *i18n.Bundle

func init() {
	var err error
	I18n, err = loadResource()
	if err != nil {
		panic(err)
	}
}

func loadResource() (*i18n.Bundle, error) {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	if err := walkLangDir(bundle, "locales"); err != nil {
		return nil, err
	}
	return bundle, nil
}

// walkLangDir 递归遍历目录
func walkLangDir(bundle *i18n.Bundle, dirPath string) error {
	entries, err := dir.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("read directory %s failed: %v", dirPath, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		fullPath := filepath.Join(dirPath, entry.Name())
		if runtime.GOOS == "windows" {
			fullPath = strings.ReplaceAll(fullPath, "\\", "/")
		}
		data, err := readAllFiles(fullPath)
		if err != nil {
			return fmt.Errorf("read file %s failed: %v", fullPath, err)
		}
		// 如果是文件，加载它
		_, err = bundle.ParseMessageFileBytes(data, fmt.Sprintf("%s.toml", fullPath))
		if err != nil {
			return fmt.Errorf("load file %s failed: %v", fullPath, err)
		}
	}
	return nil
}

func readAllFiles(dirPath string) ([]byte, error) {
	var buf bytes.Buffer
	files, err := dir.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		fullPath := filepath.Join(dirPath, file.Name())
		if runtime.GOOS == "windows" {
			fullPath = strings.ReplaceAll(fullPath, "\\", "/")
		}
		data, err := dir.ReadFile(fullPath)
		if err != nil {
			return nil, err
		}
		buf.Write(data)
		buf.Write([]byte("\n"))
	}
	return buf.Bytes(), nil
}

type Language string

func (lan Language) Localizer() *i18n.Localizer {
	return i18n.NewLocalizer(I18n, string(lan), "en")
}

func (lan Language) SafeText(messageId, defaultValue string) string {
	v, err := lan.Message(messageId, nil, nil)
	if err == nil {
		return v
	}
	return defaultValue
}

func (lan Language) Message(messageId string, data, plural any) (string, error) {
	return lan.Localizer().Localize(&i18n.LocalizeConfig{
		MessageID:    messageId,
		TemplateData: data,
		PluralCount:  plural,
	})
}
func (lan Language) MustMessage(messageId string, data, plural any) string {
	return lan.Localizer().MustLocalize(&i18n.LocalizeConfig{
		MessageID:    messageId,
		TemplateData: data,
		PluralCount:  plural,
	})
}

func (lan Language) MustText(messageId string) string {
	return lan.MustMessage(messageId, nil, nil)
}
