# MALLEUS·OS

**Bootujesz z pendrive'a na dowolnym sprzęcie i zarządzasz serwerem
z przeglądarki.**

MalleusOS to open-source'owy system zarządzania homelabem. Wgrywasz go na
starego laptopa, mini-PC albo Raspberry Pi — i zamiast wieczorów w terminalu
dostajesz ciemny panel w przeglądarce: metryki na żywo, kontenery Dockera
jednym kliknięciem, katalog aplikacji (Jellyfin, Nextcloud, Pi-hole…)
i tryb kiosku na wbudowany ekranik LCD.

🌐 **Strona projektu:** https://malleusos.vercel.app

![Zrzut ekranu panelu MalleusOS — wkrótce](docs/screenshot-placeholder.png)
*(screenshot panelu pojawi się wraz z pierwszym wydaniem)*

## Co potrafi

- **Metryki na żywo** — CPU, RAM, dyski, sieć i temperatury prosto
  z `/proc` i `/sys`, odświeżane co sekundę.
- **Kontenery jednym kliknięciem** — start, stop, restart i logi na żywo,
  bez wpisywania komend.
- **Katalog aplikacji** — instalacja popularnych usług homelabowych jednym
  kliknięciem, adresy w rodzaju `nazwa.malleus.local`.
- **Tryb kiosku** — widok metryk pod mały panoramiczny ekran LCD.

## Szybki start

```bash
curl -LO https://github.com/terlikk/MalleusOS/releases/latest/download/malleus-linux-amd64
chmod +x malleus-linux-amd64
sudo ./malleus-linux-amd64
```

Potem otwórz w przeglądarce `http://adres-serwera:8443`.
Na Raspberry Pi zamień `amd64` na `arm64`.

## Architektura (w skrócie)

- Jedna statycznie skompilowana binarka Go (`malleus`) — backend i agent
  metryk razem, bez CGO, kompilacja krzyżowa na amd64 i arm64.
- Dashboard: Svelte 5 + Vite, wkompilowany w binarkę (`embed.FS`) —
  całość działa offline.
- Dane: SQLite w czystym Go (`modernc.org/sqlite`).
- Live: SSE dla metryk i logów, WebSocket tylko dla terminala.

## Struktura repozytorium

| Katalog    | Zawartość                                        |
|------------|--------------------------------------------------|
| `/site`    | ta strona projektu (statyczna, deploy na Vercel) |
| `/agent`   | zbieranie metryk z `/proc` i `/sys`              |
| `/server`  | serwer HTTP, API, SSE                            |
| `/web`     | dashboard Svelte (panel serwera)                 |
| `/catalog` | szablony aplikacji do instalacji                 |
| `/image`   | budowa bootowalnego obrazu                       |
| `/tools`   | narzędzia developerskie                          |

## Licencja

[AGPL-3.0](https://www.gnu.org/licenses/agpl-3.0.html) — używaj, zmieniaj,
udostępniaj dalej na tych samych zasadach.
