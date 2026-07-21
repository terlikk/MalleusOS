// Pakiet catalog trzyma szablony aplikacji do instalacji jednym
// kliknięciem. Szablony to pliki YAML w templates/ — wkompilowane
// w binarkę przez embed, więc katalog działa offline.
// Do tego dochodzą szablony własne użytkownika: pliki YAML
// w katalogu danych (UserDir), dopisywane z panelu.
package catalog

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed templates/*.yaml
var files embed.FS

// UserDir to katalog na szablony użytkownika (np. data/templates).
// Puste = własne szablony wyłączone. Ustawiane raz przy starcie.
var UserDir string

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
	// Custom = szablon dodany przez użytkownika z panelu
	// (nie zapisujemy go w YAML — wynika z miejsca na dysku).
	Custom bool `yaml:"-" json:"custom,omitempty"`
	// Pliki = aplikacja potrzebuje plików z treścią (np. paczki
	// .zim Kiwiksa). Panel pokazuje wtedy "dodaj treść": serwer
	// pobiera plik prosto do wolumenu i restartuje aplikację.
	Pliki *Pliki `yaml:"pliki" json:"pliki,omitempty"`
}

// Pliki opisuje, dokąd i co pobierać przyciskiem "dodaj treść".
type Pliki struct {
	Volume string `yaml:"volume" json:"volume"` // nazwa wolumenu z App.Volumes
	Ext    string `yaml:"ext" json:"ext"`       // wymagane rozszerzenie, np. ".zim"
	Opis   string `yaml:"opis" json:"opis"`
	// Propozycje to gotowe paczki do pobrania jednym kliknięciem.
	Propozycje []Propozycja `yaml:"propozycje" json:"propozycje,omitempty"`
}

type Propozycja struct {
	Nazwa string `yaml:"nazwa" json:"nazwa"`
	URL   string `yaml:"url" json:"url"`
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

// Load wczytuje i parsuje wszystkie szablony (posortowane po nazwie):
// najpierw wbudowane, potem własne użytkownika z UserDir.
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

	// Szablony użytkownika. Zepsuty plik nie kładzie całego
	// katalogu — pomijamy go (użytkownik mógł edytować ręcznie).
	// Wbudowane id mają pierwszeństwo, duplikat pomijamy.
	if UserDir != "" {
		seen := map[string]bool{}
		for _, a := range apps {
			seen[a.ID] = true
		}
		userFiles, _ := filepath.Glob(filepath.Join(UserDir, "*.yaml"))
		for _, f := range userFiles {
			data, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			var app App
			if err := yaml.Unmarshal(data, &app); err != nil {
				continue
			}
			if app.ID == "" || app.Image == "" || seen[app.ID] {
				continue
			}
			seen[app.ID] = true
			app.Custom = true
			apps = append(apps, app)
		}
	}

	sort.Slice(apps, func(i, j int) bool { return apps[i].Name < apps[j].Name })
	return apps, nil
}

// Slug zamienia nazwę na identyfikator techniczny: "Mój Serwer!"
// → "moj-serwer". Z takiego id robi się nazwa kontenera i wolumenów.
func Slug(name string) string {
	// polskie znaki na łacińskie, reszta nie-alfanumerycznych na "-"
	repl := strings.NewReplacer(
		"ą", "a", "ć", "c", "ę", "e", "ł", "l", "ń", "n",
		"ó", "o", "ś", "s", "ż", "z", "ź", "z",
	)
	s := repl.Replace(strings.ToLower(name))
	s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// Save zapisuje szablon użytkownika jako YAML w UserDir.
// Walidację (unikalne id itd.) robi wołający — tu tylko zapis.
func Save(app App) error {
	if UserDir == "" {
		return fmt.Errorf("własne szablony wymagają katalogu danych")
	}
	if err := os.MkdirAll(UserDir, 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(app)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(UserDir, app.ID+".yaml"), data, 0o644)
}

// Delete usuwa szablon użytkownika. Wbudowanych nie da się usunąć —
// ich pliki siedzą w binarce, nie w UserDir.
func Delete(id string) error {
	if UserDir == "" {
		return fmt.Errorf("własne szablony wymagają katalogu danych")
	}
	return os.Remove(filepath.Join(UserDir, id+".yaml"))
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
