// Pakiet mdns odpowiada w sieci lokalnej na pytania o adresy
// *.malleus.local — dzięki temu przeglądarka znajdzie serwer
// po nazwie, bez konfigurowania czegokolwiek.
//
// mDNS (multicast DNS) to "DNS bez serwera": urządzenie pyta całą
// sieć lokalną (adres grupowy 224.0.0.251, port 5353) "kto to jest
// filmy.malleus.local?", a my odpowiadamy własnym adresem IP.
// Rozumieją to Windows, macOS, Linux, Android i iOS.
package mdns

import (
	"log"
	"net"
	"strings"

	"golang.org/x/net/dns/dnsmessage"
	"golang.org/x/net/ipv4"
)

// Suffix to domena, na którą odpowiadamy (z kropką na końcu,
// bo tak nazwy zapisuje protokół DNS).
const Suffix = "malleus.local."

// Serve nasłuchuje i odpowiada. Blokuje — uruchamiaj w goroutine.
func Serve() error {
	// Nasłuch na 0.0.0.0:5353 przyjmuje i multicast (po dołączeniu
	// do grupy), i zwykłe pakiety — przydatne do testów.
	conn, err := net.ListenPacket("udp4", "0.0.0.0:5353")
	if err != nil {
		return err // np. port zajęty przez avahi — wołający zaloguje
	}
	pc := ipv4.NewPacketConn(conn)

	group := &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251)}
	joined := 0
	ifaces, _ := net.Interfaces()
	for i := range ifaces {
		ifc := &ifaces[i]
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagMulticast == 0 {
			continue
		}
		if err := pc.JoinGroup(ifc, group); err == nil {
			joined++
		}
	}
	if joined == 0 {
		log.Printf("mdns: żaden interfejs nie dołączył do grupy multicast — nazwy .malleus.local mogą nie działać")
	}

	buf := make([]byte, 1500)
	for {
		n, _, src, err := pc.ReadFrom(buf)
		if err != nil {
			return err
		}
		pkt := make([]byte, n)
		copy(pkt, buf[:n])
		go answer(pc, src, pkt)
	}
}

// answer parsuje zapytanie i — jeśli pyta o nasze nazwy —
// odsyła rekord A z naszym adresem IPv4.
func answer(pc *ipv4.PacketConn, src net.Addr, pkt []byte) {
	var parser dnsmessage.Parser
	hdr, err := parser.Start(pkt)
	if err != nil || hdr.Response {
		return // to nie zapytanie
	}

	ip := localIPv4()
	if ip == nil {
		return
	}

	var questions []dnsmessage.Question
	var answers []dnsmessage.Resource
	for {
		q, err := parser.Question()
		if err != nil {
			break // koniec pytań
		}
		if q.Type != dnsmessage.TypeA {
			continue // odpowiadamy tylko na pytania o IPv4
		}
		name := strings.ToLower(q.Name.String())
		if name != Suffix && !strings.HasSuffix(name, "."+Suffix) {
			continue // nie nasza domena
		}
		questions = append(questions, q)
		answers = append(answers, dnsmessage.Resource{
			Header: dnsmessage.ResourceHeader{
				Name:  q.Name,
				Type:  dnsmessage.TypeA,
				Class: dnsmessage.ClassINET,
				TTL:   120,
			},
			Body: &dnsmessage.AResource{A: [4]byte(ip)},
		})
	}
	if len(answers) == 0 {
		return
	}

	// Klient "klasyczny" (spoza portu 5353, np. dig) dostaje
	// odpowiedź wprost, z ID i pytaniem — jak od zwykłego DNS.
	// Prawdziwe zapytania mDNS dostają odpowiedź na adres grupowy,
	// z ID=0 (tak każe RFC 6762).
	srcUDP, ok := src.(*net.UDPAddr)
	legacy := ok && srcUDP.Port != 5353

	msg := dnsmessage.Message{
		Header:  dnsmessage.Header{Response: true, Authoritative: true},
		Answers: answers,
	}
	if legacy {
		msg.Header.ID = hdr.ID
		msg.Questions = questions
	}
	out, err := msg.Pack()
	if err != nil {
		return
	}
	if legacy {
		pc.WriteTo(out, nil, srcUDP)
	} else {
		pc.WriteTo(out, nil, &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353})
	}
}

// localIPv4 zwraca pierwszy niepętlowy adres IPv4 maszyny.
func localIPv4() net.IP {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			continue
		}
		if v4 := ipnet.IP.To4(); v4 != nil {
			return v4
		}
	}
	return nil
}
