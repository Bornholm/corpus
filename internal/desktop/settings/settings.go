package settings

var Defaults = Settings{
	Servers: []Server{},
}

type Settings struct {
	Servers []Server `json:"servers"`
}

type Server struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	Label     string `json:"label"`
	Preferred bool   `json:"preferred"`
}
