package config

import "github.com/joeshaw/envdecode"

type Config struct {
	AppName                  string   `env:"APP_NAME"`
	AppVersion               string   `env:"APP_VERSION"`
	AppEnv                   string   `env:"APP_ENV,default=development"`
	ApiHost                  string   `env:"API_HOST"`
	ApiRpcPort               string   `env:"API_RPC_PORT"`
	ApiPort                  string   `env:"API_PORT,default=8760"`
	ApiDocPort               uint16   `env:"API_DOC_PORT,default=8761"`
	ShutdownTimeout          uint     `env:"API_SHUTDOWN_TIMEOUT_SECONDS,default=30"`
	AllowedCredentialOrigins []string `env:"ALLOWED_CREDENTIAL_ORIGINS"`
	MiddlewareAddress        string   `env:"MIDDLEWARE_ADDR"`
	JwtExpireDaysCount       int      `env:"JWT_EXPIRE_DAYS_COUNT"`
}

func NewConfig() *Config {
	var cfg Config
	if err := envdecode.Decode(&cfg); err != nil {
		panic(err)
	}

	return &cfg
}
