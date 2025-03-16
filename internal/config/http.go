package config

type HTTP struct {
	BaseURL string `env:"BASE_URL" envDefault:"/"`
	Address string `env:"ADDRESS,expand" envDefault:":3002"`
	Auth    Auth   `envPrefix:"AUTH_"`
}
type Auth struct {
	Enabled  bool   `env:"ENABLED,expand" envDefault:"true"`
	Username string `env:"USERNAME,expand" envDefault:"corpus"`
	Password string `env:"PASSWORD,expand" envDefault:"corpus"`
}
