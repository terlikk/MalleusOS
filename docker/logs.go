package docker

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"net/http"
)

// LogLine to jedna linia logu z informacją, skąd pochodzi.
type LogLine struct {
	Stream string `json:"stream"` // "stdout" albo "stderr"
	Line   string `json:"line"`
}

// StreamLogs otwiera strumień logów kontenera (ostatnie `tail`
// linii + wszystko nowe) i wysyła linie w kanał `out`.
// Blokuje aż do zerwania połączenia albo anulowania kontekstu.
//
// Haczyk formatu: kontener BEZ terminala (typowy przypadek)
// dostaje strumień MULTIPLEKSOWANY — stdout i stderr posiekane
// w ramki z 8-bajtowym nagłówkiem: [typ, 0, 0, 0, długość(4B)].
// Kontener Z terminalem (tty) wysyła zwykły tekst. Dlatego
// najpierw pytamy o tty, a potem wybieramy sposób czytania.
func (c *Client) StreamLogs(ctx context.Context, id string, tail string, out chan<- LogLine) error {
	tty, err := c.isTTY(ctx, id)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "GET",
		"http://docker/containers/"+id+"/logs?stdout=1&stderr=1&follow=1&tail="+tail, nil)
	if err != nil {
		return err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if tty {
		return readRaw(res.Body, out)
	}
	return readMultiplexed(res.Body, out)
}

// readRaw: zwykły tekst — linia po linii.
func readRaw(r io.Reader, out chan<- LogLine) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 256*1024) // długie linie też przejdą
	for sc.Scan() {
		out <- LogLine{Stream: "stdout", Line: sc.Text()}
	}
	return sc.Err()
}

// readMultiplexed rozplata ramki Dockera.
// Jedna ramka może zawierać pół linii, a może pięć linii —
// dlatego zbieramy bajty w bufory (osobno stdout i stderr)
// i wycinamy z nich pełne linie zakończone \n.
func readMultiplexed(r io.Reader, out chan<- LogLine) error {
	streams := map[byte]string{1: "stdout", 2: "stderr"}
	buffers := map[byte][]byte{1: nil, 2: nil}
	header := make([]byte, 8)

	for {
		// Nagłówek ramki: bajt 0 = numer strumienia,
		// bajty 4–7 = długość ładunku (big-endian).
		if _, err := io.ReadFull(r, header); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil // koniec strumienia — to nie błąd
			}
			return err
		}
		size := binary.BigEndian.Uint32(header[4:8])
		payload := make([]byte, size)
		if _, err := io.ReadFull(r, payload); err != nil {
			return err
		}

		st := header[0]
		if _, known := streams[st]; !known {
			continue // np. stdin (0) — pomijamy
		}
		buffers[st] = append(buffers[st], payload...)

		// Wycinaj pełne linie, resztę zostaw w buforze na później.
		for {
			i := indexByte(buffers[st], '\n')
			if i < 0 {
				break
			}
			out <- LogLine{Stream: streams[st], Line: string(buffers[st][:i])}
			buffers[st] = buffers[st][i+1:]
		}
	}
}

func indexByte(b []byte, c byte) int {
	for i, x := range b {
		if x == c {
			return i
		}
	}
	return -1
}
