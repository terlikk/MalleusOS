package server

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	"malleus/catalog"
)

// ProxyHandler obsługuje port 80: rozdziela ruch po nazwie hosta.
//
//	filmy.malleus.local  → aplikacja z katalogu (port z szablonu)
//	malleus.local (i IP) → panel MalleusOS
//
// To reverse proxy — pośrednik, który przyjmuje żądanie i w tle
// przekazuje je właściwej aplikacji. Czysty stdlib
// (net/http/httputil), bez zewnętrznego Caddy: jedna binarka
// zostaje jedną binarką.
func (s *Server) ProxyHandler() http.Handler {
	var mu sync.Mutex
	proxies := map[int]*httputil.ReverseProxy{}

	// getProxy zwraca (tworząc raz) proxy na dany port lokalny.
	getProxy := func(port int) *httputil.ReverseProxy {
		mu.Lock()
		defer mu.Unlock()
		if p, ok := proxies[port]; ok {
			return p
		}
		target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
		p := httputil.NewSingleHostReverseProxy(target)
		p.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w,
				"Aplikacja nie odpowiada — sprawdź w panelu MalleusOS, czy jest uruchomiona.",
				http.StatusBadGateway)
		}
		proxies[port] = p
		return p
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h // odetnij ":80" itp.
		}
		host = strings.ToLower(host)

		// Subdomena? Szukamy aplikacji o takim polu subdomain.
		if sub, ok := strings.CutSuffix(host, ".malleus.local"); ok && sub != "" {
			if apps, err := catalog.Load(); err == nil {
				for _, a := range apps {
					if a.Subdomain == sub && a.WebPort > 0 {
						getProxy(a.WebPort).ServeHTTP(w, r)
						return
					}
				}
			}
			http.Error(w,
				"Nie ma aplikacji pod adresem "+host+" — zajrzyj do katalogu w panelu MalleusOS.",
				http.StatusNotFound)
			return
		}

		// malleus.local, adres IP, cokolwiek innego → panel.
		s.mux.ServeHTTP(w, r)
	})
}
