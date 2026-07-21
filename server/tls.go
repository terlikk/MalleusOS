package server

// HTTPS dla panelu: przy fladze -tls binarka sama generuje
// certyfikat (samopodpisany, ważny 10 lat) i trzyma go w katalogu
// danych. Samopodpisany = przeglądarka za pierwszym razem ostrzeże
// ("połączenie nie jest prywatne") — to normalne, akceptujesz raz.
// Ruch i tak jest od tego momentu szyfrowany.

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// EnsureCert zwraca ścieżki cert.pem/key.pem w katalogu danych,
// generując je przy pierwszym użyciu.
func EnsureCert(dataDir string) (certFile, keyFile string, err error) {
	certFile = filepath.Join(dataDir, "cert.pem")
	keyFile = filepath.Join(dataDir, "key.pem")

	if _, errC := os.Stat(certFile); errC == nil {
		if _, errK := os.Stat(keyFile); errK == nil {
			return certFile, keyFile, nil // już wygenerowane
		}
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return "", "", err
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}

	hostname, _ := os.Hostname()
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "malleus.local"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		// Nazwy i adresy, pod którymi panel bywa otwierany
		DNSNames:    []string{"malleus.local", "*.malleus.local", "localhost", hostname},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)},
	}
	// dopisz aktualne adresy IP maszyny
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				tmpl.IPAddresses = append(tmpl.IPAddresses, ipnet.IP)
			}
		}
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return "", "", err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return "", "", err
	}

	if err := os.WriteFile(certFile,
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644); err != nil {
		return "", "", err
	}
	// Klucz prywatny tylko dla właściciela pliku (0600)
	if err := os.WriteFile(keyFile,
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		return "", "", err
	}
	return certFile, keyFile, nil
}

// ListenAndServeTLS — jak ListenAndServe, ale po HTTPS.
func (s *Server) ListenAndServeTLS(addr, certFile, keyFile string) error {
	srv := newHTTPServer(addr, s.mux)
	return srv.ListenAndServeTLS(certFile, keyFile)
}
