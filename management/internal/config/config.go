package config

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/spf13/viper"
)

// LoadConfig 加载配置入口
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		return doLoadConfig()
	}
	return doLoadConfigFromPath(path)
}

// doLoadConfig 当未指定路径时，默认尝试从程序同级 conf 目录或容器 /app/config 目录加载所有 toml
func doLoadConfig() (*Config, error) {
	// 1. 获取程序执行所在目录
	pwd, err := os.Executable()
	if err == nil {
		localConfDir := filepath.Join(filepath.Dir(pwd), "conf")
		if isDir(localConfDir) {
			if err := doLoadConfigFromDir(localConfDir); err == nil {
				return unmarshalConfig()
			} else {
				warnConfigLoad(localConfDir, err)
			}
		}
	}

	// 2. 容器环境常驻路径 fallback
	containerConfDir := "/app/config"
	if isDir(containerConfDir) {
		if err := doLoadConfigFromDir(containerConfDir); err == nil {
			return unmarshalConfig()
		} else {
			warnConfigLoad(containerConfDir, err)
		}
	}

	// 3. 兜底逻辑：如果连目录都没有，尝试当前目录下单文件加载
	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		if isConfigMissingError(err) {
			warnConfigLoad("config.toml", err)
			return unmarshalConfig()
		}
		return nil, err
	}

	return unmarshalConfig()
}

// doLoadConfigFromPath 支持指定单一文件，或指定特定目录加载多个文件
func doLoadConfigFromPath(filename string) (*Config, error) {
	info, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			warnConfigLoad(filename, err)
			return unmarshalConfig()
		}
		return nil, err
	}

	if info.IsDir() {
		// 如果传入的是目录，加载该目录下所有的 .toml 文件并合并
		if err := doLoadConfigFromDir(filename); err != nil {
			if isConfigMissingError(err) {
				warnConfigLoad(filename, err)
				return unmarshalConfig()
			}
			return nil, err
		}
	} else {
		// 如果传入的是单文件
		data, err := os.ReadFile(filename)
		if err != nil {
			if os.IsNotExist(err) {
				warnConfigLoad(filename, err)
				return unmarshalConfig()
			}
			return nil, err
		}
		viper.SetConfigType("toml")
		if err := viper.ReadConfig(bytes.NewReader(data)); err != nil {
			return nil, err
		}
	}

	return unmarshalConfig()
}

// doLoadConfigFromDir 遍历目录下所有 .toml 文件并通过 MergeInConfig 融合成一个配置视图
func doLoadConfigFromDir(confDir string) error {
	var files []string
	err := filepath.WalkDir(confDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// 只处理以 .toml 结尾的配置文件
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".toml") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(files)
	for i, path := range files {
		viper.SetConfigFile(path)
		viper.SetConfigType("toml")
		if i == 0 {
			err = viper.ReadInConfig()
		} else {
			err = viper.MergeInConfig()
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// unmarshalConfig 统一收拢：环境变量绑定 -> 反序列化结构体
func unmarshalConfig() (*Config, error) {
	bindEnv()

	var conf Config
	if err := viper.Unmarshal(&conf); err != nil {
		return nil, err
	}
	conf.Database.DSN = os.ExpandEnv(conf.Database.DSN)
	conf.Auth.JWTSigningKey = os.ExpandEnv(conf.Auth.JWTSigningKey)
	conf.Auth.ThrottleHMACKey = os.ExpandEnv(conf.Auth.ThrottleHMACKey)
	conf.Auth.TOTPEncryptionKey = os.ExpandEnv(conf.Auth.TOTPEncryptionKey)
	conf.Cache.Redis.Password = os.ExpandEnv(conf.Cache.Redis.Password)
	return &conf, nil
}

// bindEnv 智能化环境变量绑定
func bindEnv() {
	// 1. 设置环境变量前缀（例如：APP_DATABASE_DSN）
	viper.SetEnvPrefix("APP")
	// 2. 将配置项的句点 "." 转换为下划线 "_" 以匹配标准的系统环境变量命名
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	// 3. 开启自动环境读取
	viper.AutomaticEnv()

	for _, key := range configKeys(reflect.TypeOf(Config{}), "") {
		_ = viper.BindEnv(key)
	}
}

func configKeys(t reflect.Type, prefix string) []string {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		if prefix == "" {
			return nil
		}
		return []string{prefix}
	}

	var keys []string
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}

		name := field.Tag.Get("mapstructure")
		if name == "" || name == "," {
			name = strings.ToLower(field.Name)
		} else if idx := strings.Index(name, ","); idx >= 0 {
			name = name[:idx]
		}
		if name == "" {
			name = strings.ToLower(field.Name)
		}

		next := name
		if prefix != "" {
			next = prefix + "." + name
		}

		ft := field.Type
		for ft.Kind() == reflect.Ptr {
			ft = ft.Elem()
		}
		if ft.Kind() == reflect.Struct && ft.Name() != "Time" {
			keys = append(keys, configKeys(ft, next)...)
			continue
		}
		keys = append(keys, next)
	}
	return keys
}

func isConfigMissingError(err error) bool {
	if err == nil {
		return false
	}
	if os.IsNotExist(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") || strings.Contains(msg, "no such file")
}

func warnConfigLoad(target string, err error) {
	fmt.Fprintf(os.Stderr, "warning: config %q not loaded: %v\n", target, err)
}

// 辅助工具函数：判断路径是否为有效目录
func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
