package webhook

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

const maxCuerpoRespuesta = 4 << 10

type Sender struct {
	http          *http.Client
	permitirLocal bool
}

// permitirLocal solo debe activarse en desarrollo: en produccion apuntar un
// webhook a una IP interna convierte el servicio en un proxy hacia la red
// privada (SSRF).
func NewSender(timeout time.Duration, permitirLocal bool) *Sender {
	return &Sender{
		http: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		permitirLocal: permitirLocal,
	}
}

func (s *Sender) Enviar(ctx context.Context, destino string, cuerpo []byte, firma string) error {
	if err := s.validarDestino(destino); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, destino, bytes.NewReader(cuerpo))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Facturacion-Signature", firma)
	req.Header.Set("User-Agent", "facturacion-service/1")

	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("entregando webhook: %w", err)
	}
	defer resp.Body.Close()

	detalle, _ := io.ReadAll(io.LimitReader(resp.Body, maxCuerpoRespuesta))

	// Cualquier 2xx cuenta como entregado. El resto se reintenta.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook rechazado con HTTP %d: %s", resp.StatusCode, string(detalle))
	}

	return nil
}

func (s *Sender) validarDestino(destino string) error {
	u, err := url.Parse(destino)
	if err != nil {
		return fmt.Errorf("url de webhook invalida: %w", err)
	}

	if u.Scheme != "https" && !s.permitirLocal {
		return fmt.Errorf("el webhook debe usar https, recibido %q", u.Scheme)
	}

	if s.permitirLocal {
		return nil
	}

	ips, err := net.LookupIP(u.Hostname())
	if err != nil {
		return fmt.Errorf("no se pudo resolver %q: %w", u.Hostname(), err)
	}

	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
			return fmt.Errorf("el webhook apunta a una direccion no publica: %s", ip)
		}
	}

	return nil
}
