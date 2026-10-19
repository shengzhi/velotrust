package config

type Config struct {
	App      AppConfig      `mapstructure:"app"`
	HTTP     HTTPConfig     `mapstructure:"http"`
	Database DatabaseConfig `mapstructure:"database"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Cache    CacheConfig    `mapstructure:"cache"`
	OSS      OSSConfig      `mapstructure:"oss"`
}

type AuthConfig struct {
	JWTSigningKey     string         `mapstructure:"jwt_signing_key"`
	ThrottleHMACKey   string         `mapstructure:"throttle_hmac_key"`
	TOTPEncryptionKey string         `mapstructure:"totp_encryption_key"`
	TOTPKeyVersion    string         `mapstructure:"totp_key_version"`
	Session           SessionConfig  `mapstructure:"session"`
	WebAuthn          WebAuthnConfig `mapstructure:"webauthn"`
}

type SessionConfig struct {
	IdleTTLMinutes     int `mapstructure:"idle_ttl_minutes"`
	AbsoluteTTLMinutes int `mapstructure:"absolute_ttl_minutes"`
}

type WebAuthnConfig struct {
	RPID        string   `mapstructure:"rp_id"`
	DisplayName string   `mapstructure:"display_name"`
	Origins     []string `mapstructure:"origins"`
}

type CacheConfig struct {
	Redis RedisConfig `mapstructure:"redis"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type DatabaseConfig struct {
	Driver                       string `mapstructure:"driver"`
	DSN                          string `mapstructure:"dsn"`
	ShowSQL                      bool   `mapstructure:"show_sql"`
	MaxOpenConnections           int    `mapstructure:"max_open_connections"`
	MaxIdleConnections           int    `mapstructure:"max_idle_connections"`
	ConnectionMaxLifetimeSeconds int    `mapstructure:"connection_max_lifetime_seconds"`
}

// OSSConfig configures the object storage adapter. The local driver is a safe
// default for development and tests; cloud adapters can consume the same shape.
type OSSConfig struct {
	Driver    string `mapstructure:"driver"`
	LocalRoot string `mapstructure:"local_root"`
	PublicURL string `mapstructure:"public_url"`
}

type HTTPConfig struct {
	Port           int        `mapstructure:"port"`
	ListenAddr     string     `mapstructure:"listen_addr"`
	TrustedProxies []string   `mapstructure:"trusted_proxies"`
	CORS           CORSConfig `mapstructure:"cors"`
}

type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins"`
	AllowedMethods []string `mapstructure:"allowed_methods"`
	AllowedHeaders []string `mapstructure:"allowed_headers"`
	MaxAgeSeconds  int      `mapstructure:"max_age_seconds"`
}

type AppConfig struct {
	Name      string `mapstructure:"name"`
	Version   string `mapstructure:"version"`
	LogLevel  string `mapstructure:"log_level"`
	LogFormat string `mapstructure:"log_format"`
	// 其他应用相关配置项...
}
