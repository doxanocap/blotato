package config

import "time"

type App struct {
	Mode            string `envconfig:"BLOTATO_MODE" default:"fake"`
	Listen          string `envconfig:"BLOTATO_LISTEN" default:"127.0.0.1:8080"`
	AdminToken      string `envconfig:"BLOTATO_ADMIN_TOKEN"`
	SwaggerBasePath string `envconfig:"SWAGGER_BASE_PATH" default:"/"`
}

type SQLite struct {
	Path string `envconfig:"BLOTATO_DB" default:"blotato.db"`
}

type Instagram struct {
	Host           string        `envconfig:"INSTAGRAM_API_BASE" default:"https://graph.facebook.com/v26.0"`
	AccountID      string        `envconfig:"INSTAGRAM_ACCOUNT_ID"`
	Token          string        `envconfig:"INSTAGRAM_ACCESS_TOKEN"`
	RequestTimeout time.Duration `envconfig:"INSTAGRAM_REQUEST_TIMEOUT" default:"10s"`
}

type Meta struct {
	AppSecret   string `envconfig:"META_APP_SECRET"`
	VerifyToken string `envconfig:"META_VERIFY_TOKEN"`
}

type Clients struct {
	Mode      string
	Instagram Instagram
}

type Jev struct {
	Enabled bool   `envconfig:"BLOTATO_EMAIL_INTENT_ENABLED" default:"false"`
	APIKey  string `envconfig:"OPENROUTER_API_KEY"`
}
