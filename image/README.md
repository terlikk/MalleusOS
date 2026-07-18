# Bootowalny obraz MalleusOS

Skrypt `build.sh` składa obraz pendrive'a, z którego MalleusOS
startuje na dowolnym komputerze x86-64 — bez instalowania
czegokolwiek na dysku.

## Jak to działa

- **Alpine Linux w trybie "diskless"**: system ładuje się z pendrive'a
  do pamięci RAM i tam żyje (overlay tmpfs). Wyłączenie prądu niczego
  nie psuje, bo w czasie pracy nic nie pisze po nośniku systemowym.
- **Partycja 1** (FAT32 `MALLEUS`): jądro, initramfs, pakiety Alpine
  i nasza konfiguracja (`malleus.apkovl.tar.gz`) — w niej binarka
  `malleus`, usługa OpenRC i sieć na DHCP.
- **Partycja 2** (ext4 `malleus-data`): jedyne miejsce zapisu — baza
  MalleusOS i obrazy/wolumeny Dockera. To tu mieszkają Twoje dane.

Przy pierwszym uruchomieniu z dostępem do internetu system sam
doinstalowuje Dockera i gita (nie ma ich na ISO Alpine).

## Budowa

Na Linuksie (może być WSL2), z zainstalowanym `syslinux`:

```bash
make web && make build-all      # binarki MalleusOS
sudo sh image/build.sh          # → image/malleusos-amd64.img
```

## Wgranie i start

```bash
lsblk                                          # znajdź pendrive (np. sdb)
sudo dd if=image/malleusos-amd64.img of=/dev/sdX bs=4M status=progress
```

**Uwaga:** `dd` nadpisuje urządzenie bez pytania — sprawdź literę
dwa razy.

Włóż pendrive w komputer, w BIOS/UEFI wybierz start z USB (tryb
Legacy/CSM), poczekaj ~30 s i wejdź przeglądarką na
`http://malleus.local` (albo adres IP z routera).

## Test bez sprzętu

```bash
qemu-system-x86_64 -m 2048 -drive format=raw,file=image/malleusos-amd64.img
```

## Ograniczenia (na dziś)

- Tylko x86-64 i tylko start Legacy/CSM (UEFI — w planach).
- Docker wymaga internetu przy pierwszym starcie.
- Obraz ma stały rozmiar 1,5 GB — resztę pendrive'a można oddać
  partycji danych (np. `growpart` + `resize2fs`).
