package engine

import (
	"context"
	"sync"
)

// SUNAT no publica sus limites de concurrencia, pero rechaza con 401 las rafagas
// del mismo emisor: en una prueba de 30 emisiones simultaneas cayeron 20.
// Serializar por RUC evita estrellar correlativos contra ese muro.
//
// Bloquea en vez de reprogramar el job a proposito: para cuando la peticion
// llega aqui, Tomar ya marco el comprobante como 'procesando', y soltarlo lo
// dejaria atascado hasta que se abra la ventana de rescate.
//
// ponytail: el semaforo es de proceso. Con una sola instancia de la API alcanza;
// si algun dia corren varias, esto tiene que mudarse a un advisory lock de
// Postgres por RUC.
type limitador struct {
	mu     sync.Mutex
	slots  map[string]chan struct{}
	maximo int
}

func nuevoLimitador(maximo int) *limitador {
	if maximo < 1 {
		maximo = 1
	}
	return &limitador{slots: map[string]chan struct{}{}, maximo: maximo}
}

// adquirir espera turno para hablar con SUNAT en nombre de este RUC y devuelve
// la funcion que lo libera. Si el contexto muere antes, nadie envio nada.
func (l *limitador) adquirir(ctx context.Context, ruc string) (func(), error) {
	slot := l.slot(ruc)

	select {
	case slot <- struct{}{}:
		return func() { <-slot }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (l *limitador) slot(ruc string) chan struct{} {
	l.mu.Lock()
	defer l.mu.Unlock()

	s, ok := l.slots[ruc]
	if !ok {
		s = make(chan struct{}, l.maximo)
		l.slots[ruc] = s
	}
	return s
}
