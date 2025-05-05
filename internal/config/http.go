package config

type HTTP struct {
	BaseURL string `env:"BASE_URL,expand" envDefault:"/"`
	Address string `env:"ADDRESS,expand" envDefault:":3002"`
	Auth    Auth   `envPrefix:"AUTH_"`
}
type Auth struct {
	AllowAnonymous bool `env:"ALLOW_ANONYMOUS,expand" envDefault:"true"`
	Reader         User `envPrefix:"READER_"`
	Writer         User `envPrefix:"WRITER_"`
}

type User struct {
	Username string `env:"USERNAME,expand"`
	Password string `env:"PASSWORD,expand"`
}
