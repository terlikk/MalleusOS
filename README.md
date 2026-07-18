# MALLEUS·OS

**Bootujesz z pendrive'a na dowolnym sprzęcie i zarządzasz serwerem
z przeglądarki.**

MalleusOS to open-source'owy system zarządzania homelabem. Wgrywasz go na
starego laptopa, mini-PC albo Raspberry Pi — i zamiast wieczorów w terminalu
dostajesz ciemny panel w przeglądarce: metryki na żywo, kontenery Dockera
jednym kliknięciem, katalog aplikacji (Jellyfin, Nextcloud, Pi-hole…)
i tryb kiosku na wbudowany ekranik LCD.

🌐 **Strona projektu:** https://malleus-os.vercel.app

![Zrzut ekranu panelu MalleusOS — wkrótce](docs/screenshot-placeholder.png)
*(screenshot panelu pojawi się wraz z pierwszym wydaniem)*

## Co potrafi

- **Metryki na żywo** — wykresy CPU, RAM, dysków, sieci i temperatur
  odświeżane co sekundę, historia 15 min – 2 h, progi ostrzegawcze 70/90%.
- **Kontenery jednym kliknięciem** — start, stop, restart i logi na żywo,
  bez wpisywania komend.
- **Katalog aplikacji** — Jellyfin (własny Netflix), Nextcloud (własna
  chmura), Pi-hole (blokowanie reklam), Samba (dysk sieciowy), Uptime Kuma
  (monitoring usług), Kiwix (Wikipedia offline) — instalacja jednym
  kliknięciem, adresy w rodzaju `nazwa.malleus.local`.
- **Deploy własnych aplikacji** — wskazujesz repozytorium git albo ZIP,
  serwer buduje i uruchamia projekt, logi budowania na żywo.
- **Panel zamknięty na hasło** — logowanie od pierwszego uruchomienia.
- **Tryb kiosku** — widok metryk pod mały panoramiczny ekran LCD.

## Jak to działa

1. **Wgrywasz i włączasz** — MalleusOS startuje z pendrive'a na dowolnym
   sprzęcie; niczego nie instalujesz na dysku.
2. **Serwer ogarnia się sam** — jeden mały program czyta metryki prosto
   z systemu i rozmawia z Dockerem, bez dodatkowych agentów i konfiguracji.
3. **Ty otwierasz przeglądarkę** — panel pokazuje wszystko na żywo,
   a aplikacje instalujesz kliknięciem.

## Czym się wyróżnia

- **Jeden plik, zero instalacji** — cały system to jedna mała binarka
  (Intel/AMD i ARM).
- **Działa bez internetu** — panel, wykresy i czcionki są wbudowane w środek.
- **Lekki** — uciągnie go dziesięcioletni laptop i Raspberry Pi.
- **Twoje dane zostają u Ciebie** — bez chmury, kont i telemetrii.

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

## Dla programistów

Wymagany Go ≥ 1.24. Najważniejsze polecenia:

```bash
make vet        # statyczna analiza kodu
make web        # zbuduj panel WWW (wymaga Node ≥ 20)
make build      # binarka na tę maszynę → bin/malleus
make build-all  # kompilacja krzyżowa: amd64 + arm64
make run        # zbuduj i uruchom (panel na :8443)
```

Pełna binarka z panelem: `make web && make build`, potem
`./bin/malleus` i otwórz `http://localhost:8443` w przeglądarce.
Przy pierwszym uruchomieniu panel poprosi o ustawienie hasła.
Do pracy nad samym panelem: `cd web && npm run dev` (Vite serwuje
panel z podmianą na żywo, a zapytania `/api` przekazuje do
działającej binarki).

Tryb kiosku (wielkie metryki pod ekranik LCD): otwórz
`http://adres-serwera:8443/kiosk`.

Ładne adresy (`filmy.malleus.local` itd.): binarka ma w środku
reverse proxy na porcie 80 i responder mDNS — na prawdziwym
serwerze uruchom przez `sudo` (port 80), a nazwy w sieci lokalnej
rozgłoszą się same. Sterowanie flagami: `-proxy ""` wyłącza proxy,
`-mdns=false` wyłącza rozgłaszanie.

Development bez Dockera — atrapa udająca jego API:

```bash
go run ./tools/fakedocker &                     # socket /tmp/fakedocker.sock
./bin/malleus -docker-sock /tmp/fakedocker.sock
```

Wydanie nowej wersji robi się tagiem — CI zbuduje binarki
i opublikuje Release samo: `git tag v0.1.0 && git push origin v0.1.0`.

Szybki test API:

```bash
curl localhost:8443/api/v1/health
curl localhost:8443/api/v1/metrics
curl "localhost:8443/api/v1/metrics/history?range=15m"
curl -N localhost:8443/api/v1/stream   # strumień SSE, Ctrl+C przerywa
```

## Licencja

[AGPL-3.0](https://www.gnu.org/licenses/agpl-3.0.html) — używaj, zmieniaj,
udostępniaj dalej na tych samych zasadach.
