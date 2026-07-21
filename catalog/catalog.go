// Pakiet catalog trzyma szablony aplikacji do instalacji jednym
// kliknięciem. Szablony to pliki YAML w templates/ — wkompilowane
// w binarkę przez embed, więc katalog działa offline.
package catalog

import (
	"embed"
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

//go:embed templates/*.yaml
var files embed.FS

// App to jeden szablon z katalogu. Pola opisane w jellyfin.yaml.
type App struct {
	ID        string   `yaml:"id" json:"id"`
	Name      string   `yaml:"name" json:"name"`
	Tagline   string   `yaml:"tagline" json:"tagline"`
	Image     string   `yaml:"image" json:"image"`
	WebPort   int      `yaml:"webPort" json:"webPort"`
	Subdomain string   `yaml:"subdomain" json:"subdomain"`
	Ports     []Port   `yaml:"ports" json:"ports"`
	Volumes   []Volume `yaml:"volumes" json:"volumes"`
	Env       []EnvVar `yaml:"env" json:"env"`
	Cmd       []string `yaml:"cmd" json:"cmd,omitempty"`

	// Kategoria grupuje aplikacje w panelu (puste = zwykła apka,
	// "gry" = sekcja Serwery gier).
	Kategoria string `yaml:"kategoria" json:"kategoria,omitempty"`
	// Connect to podpowiedź połączenia dla aplikacji bez WWW,
	// np. "HOST:25565" — panel podmienia HOST na adres serwera.
	Connect string `yaml:"connect" json:"connect,omitempty"`
	// Network "host" = kontener współdzieli sieć z serwerem
	// (wymagane np. przez agenta tunelu playit.gg).
	Network string `yaml:"network" json:"network,omitempty"`
	// Hint to podpowiedź "co dalej" pokazywana po instalacji
	// (HOST podmieniany na adres serwera) — np. krok z routerem
	// przy Pi-hole, którego nie da się zautomatyzować.
	Hint string `yaml:"hint" json:"hint,omitempty"`
}

type Port struct {
	Host      int    `yaml:"host" json:"host"`
	Container int    `yaml:"container" json:"container"`
	Protocol  string `yaml:"protocol" json:"protocol,omitempty"` // puste = tcp
}

type Volume struct {
	Name string `yaml:"name" json:"name"`
	Path string `yaml:"path" json:"path"`
	Opis string `yaml:"opis" json:"opis"`
}

type EnvVar struct {
	Name  string `yaml:"name" json:"name"`
	Value string `yaml:"value" json:"value"`
	Opis  string `yaml:"opis" json:"opis"`
	// Pytaj = wartość podaje użytkownik przy instalacji
	// (np. klucz z playit.gg albo hasło do udziału Samby).
	Pytaj bool `yaml:"pytaj" json:"pytaj,omitempty"`
}

// Load wczytuje i parsuje wszystkie szablony (posortowane po nazwie).
func Load() ([]App, error) {
	entries, err := files.ReadDir("templates")
	if err != nil {
		return nil, err
	}

	var apps []App
	for _, e := range entries {
		data, err := files.ReadFile("templates/" + e.Name())
		if err != nil {
			return nil, err
		}
		var app App
		if err := yaml.Unmarshal(data, &app); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		if app.ID == "" || app.Image == "" {
			return nil, fmt.Errorf("%s: brak wymaganych pól id/image", e.Name())
		}
		apps = append(apps, app)
	}

	sort.Slice(apps, func(i, j int) bool { return apps[i].Name < apps[j].Name })
	return apps, nil
}

// ByID znajduje szablon po identyfikatorze.
func ByID(id string) (App, error) {
	apps, err := Load()
	if err != nil {
		return App{}, err
	}
	for _, a := range apps {
		if a.ID == id {
			return a, nil
		}
	}
	return App{}, fmt.Errorf("nie ma aplikacji %q w katalogu", id)
}
