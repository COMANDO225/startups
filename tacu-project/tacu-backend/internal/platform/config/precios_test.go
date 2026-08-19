package config_test

import (
	"testing"
	"time"

	"tacu-backend/internal/platform/config"
)

const rutaConfig = "../../../config/config.yaml"

// Una tarifa promocional que vence en silencio convierte todo el calculo de
// costos en mentira, y nadie se entera hasta que llega la factura al doble.
// Este test es la alarma: revienta el dia que caduca y obliga a revisarla.
func TestNingunPrecioVencido(t *testing.T) {
	cfg, err := config.Cargar(rutaConfig)
	if err != nil {
		t.Fatalf("cargando config: %v", err)
	}

	hoy := time.Now()

	for ref, p := range cfg.IA.Precios {
		if p.VigenteHasta == "" {
			continue
		}

		hasta, err := time.Parse("2006-01-02", p.VigenteHasta)
		if err != nil {
			t.Errorf("%s: vigente_hasta %q no tiene formato AAAA-MM-DD", ref, p.VigenteHasta)
			continue
		}

		if hoy.After(hasta) {
			t.Errorf("la tarifa de %s vencio el %s: verificar el precio real del proveedor "+
				"y actualizar config.yaml, o los costos calculados estan mal",
				ref, p.VigenteHasta)
		}

		if faltan := time.Until(hasta); faltan < 30*24*time.Hour && faltan > 0 {
			t.Logf("aviso: la tarifa de %s vence en %d dias (%s)",
				ref, int(faltan.Hours()/24), p.VigenteHasta)
		}
	}
}

// El config versionado tiene que cargar y validar. Si este test falla, el
// servicio no arranca.
func TestConfigVersionadoEsValido(t *testing.T) {
	if _, err := config.Cargar(rutaConfig); err != nil {
		t.Fatalf("config/config.yaml no es valido: %v", err)
	}
}
