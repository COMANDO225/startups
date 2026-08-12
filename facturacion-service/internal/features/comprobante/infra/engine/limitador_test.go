package engine

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// El fallo que motiva el limitador: 30 emisiones simultaneas del mismo emisor
// se convirtieron en 20 rechazos por 401.
func TestLimitadorSerializaPorEmisor(t *testing.T) {
	l := nuevoLimitador(1)

	var enVuelo, pico atomic.Int32
	var wg sync.WaitGroup

	for range 20 {
		wg.Go(func() {
			liberar, err := l.adquirir(context.Background(), "20000000001")
			if err != nil {
				t.Error(err)
				return
			}
			defer liberar()

			n := enVuelo.Add(1)
			for {
				max := pico.Load()
				if n <= max || pico.CompareAndSwap(max, n) {
					break
				}
			}
			time.Sleep(time.Millisecond)
			enVuelo.Add(-1)
		})
	}
	wg.Wait()

	if p := pico.Load(); p > 1 {
		t.Fatalf("hubo %d envios simultaneos del mismo RUC: SUNAT los rechazaria", p)
	}
}

// Un emisor lento no puede frenar a los demas: el limite es por RUC.
func TestLimitadorNoBloqueaEntreEmisores(t *testing.T) {
	l := nuevoLimitador(1)

	liberar, err := l.adquirir(context.Background(), "20000000001")
	if err != nil {
		t.Fatal(err)
	}
	defer liberar()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	otro, err := l.adquirir(ctx, "20512345678")
	if err != nil {
		t.Fatalf("un emisor distinto quedo bloqueado: %v", err)
	}
	otro()
}

// Sin esto, un worker esperando turno ignoraria el JobTimeout de River.
func TestLimitadorRespetaCancelacion(t *testing.T) {
	l := nuevoLimitador(1)

	liberar, err := l.adquirir(context.Background(), "20000000001")
	if err != nil {
		t.Fatal(err)
	}
	defer liberar()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	if _, err := l.adquirir(ctx, "20000000001"); err == nil {
		t.Fatal("ignoro la cancelacion: el worker quedaria colgado hasta el timeout del job")
	}
}

// La perilla debe poder subirse cuando se mida el limite real de SUNAT.
func TestLimitadorRespetaElMaximoConfigurado(t *testing.T) {
	l := nuevoLimitador(3)

	var liberadores []func()
	for i := range 3 {
		f, err := l.adquirir(context.Background(), "20000000001")
		if err != nil {
			t.Fatalf("turno %d: %v", i, err)
		}
		liberadores = append(liberadores, f)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	if _, err := l.adquirir(ctx, "20000000001"); err == nil {
		t.Fatal("dejo pasar un cuarto envio con el maximo en 3")
	}

	for _, f := range liberadores {
		f()
	}
}
