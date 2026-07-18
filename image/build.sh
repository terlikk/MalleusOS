#!/bin/sh
# ============================================================
# MalleusOS — budowa bootowalnego obrazu na pendrive'a (amd64)
#
# Pomysł (tryb "diskless" Alpine Linux):
#   * partycja 1 (FAT32, "MALLEUS")     — jądro + initramfs + apki
#     + konfiguracja startowa; TYLKO DO ODCZYTU w czasie pracy
#   * partycja 2 (ext4, "malleus-data") — dane: baza MalleusOS,
#     kontenery Dockera, wszystko co ma przetrwać restart
#
# System w czasie pracy ŻYJE W PAMIĘCI RAM (overlay tmpfs):
# pendrive można w zasadzie wyjąć, a wyłączenie prądu niczego
# nie psuje. To dokładnie architektura z założeń projektu.
#
# Wymagania (Linux, root):  wget sfdisk losetup mkfs.vfat mkfs.ext4
#                           syslinux tar gzip
# Użycie:
#   make build-all                 # najpierw binarki
#   sudo sh image/build.sh         # potem obraz
#   sudo dd if=image/malleusos-amd64.img of=/dev/sdX bs=4M status=progress
#     (sdX = Twój pendrive — sprawdź dwa razy w `lsblk`, dd nie wybacza!)
# Szybki test bez pendrive'a:
#   qemu-system-x86_64 -m 2048 -drive format=raw,file=image/malleusos-amd64.img
# ============================================================
set -eu

# --- Ustawienia ---
ALPINE_VER="3.21.3"
ALPINE_MAJOR="v3.21"
ARCH="x86_64"
SIZE_MB=1536                # rozmiar obrazu; reszta pendrive'a leży odłogiem
                            # (przy pierwszym starcie można powiększyć p2)
HERE="$(cd "$(dirname "$0")" && pwd)"
IMG="${1:-$HERE/malleusos-amd64.img}"
MALLEUS_BIN="${MALLEUS_BIN:-$HERE/../bin/malleus-linux-amd64}"
CACHE="$HERE/cache"
ISO="$CACHE/alpine-standard-$ALPINE_VER-$ARCH.iso"
ISO_URL="https://dl-cdn.alpinelinux.org/alpine/$ALPINE_MAJOR/releases/$ARCH/alpine-standard-$ALPINE_VER-$ARCH.iso"

say() { printf '\033[1;35m==> %s\033[0m\n' "$*"; }
die() { printf 'BŁĄD: %s\n' "$*" >&2; exit 1; }

# --- Kontrole wstępne ---
[ "$(id -u)" = 0 ] || die "uruchom przez sudo (partycjonowanie wymaga roota)"
for tool in wget sfdisk losetup mkfs.vfat mkfs.ext4 tar gzip; do
	command -v "$tool" >/dev/null || die "brak narzędzia: $tool"
done
command -v syslinux >/dev/null || command -v extlinux >/dev/null \
	|| die "brak syslinux/extlinux (pakiet 'syslinux')"
[ -f "$MALLEUS_BIN" ] || die "brak binarki $MALLEUS_BIN — najpierw: make web && make build-all"

MBR_BIN=""
for p in /usr/lib/syslinux/mbr/mbr.bin /usr/share/syslinux/mbr.bin /usr/lib/syslinux/bios/mbr.bin; do
	[ -f "$p" ] && MBR_BIN="$p" && break
done
[ -n "$MBR_BIN" ] || die "nie znalazłem mbr.bin z pakietu syslinux"

# --- 1. Alpine ISO (pobierane raz, potem z cache) ---
mkdir -p "$CACHE"
if [ ! -f "$ISO" ]; then
	say "Pobieram Alpine $ALPINE_VER…"
	wget -O "$ISO.part" "$ISO_URL" && mv "$ISO.part" "$ISO"
fi

# --- 2. Pusty obraz + tablica partycji ---
say "Tworzę obraz $IMG (${SIZE_MB} MB)…"
rm -f "$IMG"
truncate -s "${SIZE_MB}M" "$IMG"
# p1: 900 MB FAT32 (typ 0c) z flagą bootowalną; p2: reszta, ext4
sfdisk --quiet "$IMG" <<EOF
label: dos
,900M,0c,*
,,83,
EOF

LOOP="$(losetup --show -fP "$IMG")"
trap 'cleanup' EXIT INT
cleanup() {
	set +e
	umount -q /mnt/malleus-iso /mnt/malleus-boot /mnt/malleus-data 2>/dev/null
	[ -n "${LOOP:-}" ] && losetup -d "$LOOP" 2>/dev/null
}

say "Formatuję partycje…"
mkfs.vfat -F 32 -n MALLEUS "${LOOP}p1" >/dev/null
mkfs.ext4 -q -L malleus-data "${LOOP}p2"

mkdir -p /mnt/malleus-iso /mnt/malleus-boot /mnt/malleus-data
mount -o loop,ro "$ISO" /mnt/malleus-iso
mount "${LOOP}p1" /mnt/malleus-boot
mount "${LOOP}p2" /mnt/malleus-data

# --- 3. Jądro, initramfs, moduły i apki z ISO na partycję boot ---
say "Kopiuję system Alpine…"
cp -r /mnt/malleus-iso/boot /mnt/malleus-boot/boot
cp -r /mnt/malleus-iso/apks /mnt/malleus-boot/apks

# --- 4. Bootloader syslinux ---
say "Instaluję bootloader…"
mkdir -p /mnt/malleus-boot/boot/syslinux
cat > /mnt/malleus-boot/boot/syslinux/syslinux.cfg <<EOF
DEFAULT malleus
PROMPT 0
TIMEOUT 20
LABEL malleus
  KERNEL /boot/vmlinuz-lts
  INITRD /boot/initramfs-lts
  APPEND modules=loop,squashfs,sd-mod,usb-storage,vfat,ext4 modloop=/boot/modloop-lts quiet
EOF
umount /mnt/malleus-boot
syslinux --directory /boot/syslinux --install "${LOOP}p1"
dd if="$MBR_BIN" of="$IMG" bs=440 count=1 conv=notrunc status=none
mount "${LOOP}p1" /mnt/malleus-boot

# --- 5. apkovl: nasza konfiguracja nakładana na system przy starcie ---
# apkovl to spakowany tar z plikami, które Alpine rozpakowuje na /
# zaraz po starcie — tak "wstrzykujemy" MalleusOS w czysty system.
say "Składam konfigurację (apkovl)…"
OVL="$(mktemp -d)"
mkdir -p "$OVL/etc/apk" "$OVL/etc/init.d" "$OVL/etc/runlevels/default" \
	"$OVL/etc/local.d" "$OVL/etc/network" "$OVL/usr/local/bin" "$OVL/etc/docker"

echo malleus > "$OVL/etc/hostname"

# sieć: DHCP na pierwszej karcie — "podłącz kabel i działa"
cat > "$OVL/etc/network/interfaces" <<EOF
auto lo
iface lo inet loopback
auto eth0
iface eth0 inet dhcp
EOF

# pakiety bazowe (z apków na pendrivie — offline)
cat > "$OVL/etc/apk/world" <<EOF
alpine-base
openrc
e2fsprogs
EOF
echo "/media/usb/apks" > "$OVL/etc/apk/repositories"

# dane trwałe: partycja malleus-data pod /var/lib/malleus
cat > "$OVL/etc/fstab" <<EOF
LABEL=malleus-data /var/lib/malleus ext4 rw,noatime 0 2
EOF

# Docker trzyma obrazy na partycji danych (nie w RAM!)
cat > "$OVL/etc/docker/daemon.json" <<EOF
{ "data-root": "/var/lib/malleus/docker" }
EOF

# usługa MalleusOS (OpenRC — system usług Alpine)
cp "$HERE/malleus.initd" "$OVL/etc/init.d/malleus"
chmod +x "$OVL/etc/init.d/malleus"
cp "$MALLEUS_BIN" "$OVL/usr/local/bin/malleus"
chmod +x "$OVL/usr/local/bin/malleus"

# pierwszy start z internetem: doinstaluj Dockera i gita
# (nie ma ich na ISO — leżą w repozytorium "community")
cat > "$OVL/etc/local.d/10-malleus-firstboot.start" <<EOF
#!/bin/sh
if ! command -v docker >/dev/null; then
	echo "https://dl-cdn.alpinelinux.org/alpine/$ALPINE_MAJOR/main" >> /etc/apk/repositories
	echo "https://dl-cdn.alpinelinux.org/alpine/$ALPINE_MAJOR/community" >> /etc/apk/repositories
	apk update && apk add docker git && rc-update add docker default && rc-service docker start
fi
EOF
chmod +x "$OVL/etc/local.d/10-malleus-firstboot.start"

# usługi wstające automatycznie
for svc in networking local malleus; do
	ln -sf "/etc/init.d/$svc" "$OVL/etc/runlevels/default/$svc"
done

tar -czf /mnt/malleus-boot/malleus.apkovl.tar.gz -C "$OVL" .
rm -rf "$OVL"

# katalogi na dane od razu na miejscu
mkdir -p /mnt/malleus-data/data /mnt/malleus-data/docker

umount /mnt/malleus-boot /mnt/malleus-data /mnt/malleus-iso
losetup -d "$LOOP"; LOOP=""

say "Gotowe: $IMG"
say "Wgraj na pendrive:  sudo dd if=$IMG of=/dev/sdX bs=4M status=progress"
say "Test w emulatorze:  qemu-system-x86_64 -m 2048 -drive format=raw,file=$IMG"
