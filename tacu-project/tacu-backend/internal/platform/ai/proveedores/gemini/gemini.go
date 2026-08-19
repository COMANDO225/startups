// Package gemini implementa el Proveedor de ai sobre la API de Google Gemini.
package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"tacu-backend/internal/platform/ai"

	"google.golang.org/genai"
)

// El modelo NO se fija aqui: llega en cada llamada, elegido por la cadena que
// el config define para cada tarea.
type Proveedor struct {
	cli *genai.Client
}

func Nuevo(ctx context.Context, apiKey string) (*Proveedor, error) {
	if apiKey == "" {
		return nil, errors.New("gemini: falta la API key")
	}

	// Los reintentos del SDK vienen APAGADOS: con RetryOptions en nil,
	// retryHTTPRequest hace `if opts == nil { return do(req) }`, o sea un solo
	// intento. Un 429 pasajero fallaba al instante y el Cliente caia al
	// siguiente modelo de la cadena — que en leer_carta es 3.5-flash, el doble
	// de caro. Un limite de tasa transitorio nos escalaba el costo en silencio.
	//
	// El struct vacio da los defaults del SDK: 5 intentos, 1 s inicial, tope
	// 60 s, base 2.0, jitter, y reintento en 408/429/5xx.
	cli, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:      apiKey,
		Backend:     genai.BackendGeminiAPI,
		HTTPOptions: genai.HTTPOptions{RetryOptions: &genai.HTTPRetryOptions{}},
	})
	if err != nil {
		return nil, fmt.Errorf("gemini: creando cliente: %w", err)
	}

	return &Proveedor{cli: cli}, nil
}

func (p *Proveedor) Nombre() string { return "gemini" }

func (p *Proveedor) Soporta(c ai.Capacidad) bool {
	return c == ai.Vision || c == ai.Texto || c == ai.GenerarImagen
}

func (p *Proveedor) Ejecutar(ctx context.Context, modelo string, pet ai.Peticion) (ai.Respuesta, error) {
	if pet.Tarea.Capacidad() == ai.GenerarImagen {
		return p.generarImagen(ctx, modelo, pet)
	}
	return p.completar(ctx, modelo, pet)
}

// generarImagen usa el mismo GenerateContent que el resto, pidiendo la modalidad
// IMAGE. No hay endpoint aparte: el modelo devuelve la imagen como una parte mas
// de la respuesta.
//
// Las Referencias van como partes de entrada, igual que en la lectura de cartas:
// es lo que ancla la foto generada al plato real del restaurante en vez de a un
// plato generico de banco de imagenes.
func (p *Proveedor) generarImagen(ctx context.Context, modelo string, pet ai.Peticion) (ai.Respuesta, error) {
	partes := make([]*genai.Part, 0, len(pet.Referencias)+1)
	for _, ref := range pet.Referencias {
		partes = append(partes, &genai.Part{
			InlineData: &genai.Blob{Data: ref.Bytes, MIMEType: ref.MIME},
		})
	}
	partes = append(partes, genai.NewPartFromText(pet.Prompt))

	cfg := &genai.GenerateContentConfig{
		ResponseModalities: []string{"IMAGE"},
	}
	if pet.Tamano != "" || pet.Proporcion != "" {
		cfg.ImageConfig = &genai.ImageConfig{
			ImageSize:   pet.Tamano,     // 1K, 2K, 4K
			AspectRatio: pet.Proporcion, // 1:1, 4:3, 16:9...
		}
	}

	resp, err := p.cli.Models.GenerateContent(ctx, modelo,
		[]*genai.Content{{Parts: partes, Role: genai.RoleUser}}, cfg)
	if err != nil {
		return ai.Respuesta{}, p.clasificar(err)
	}

	u := uso(resp)
	imgs := extraerImagenes(resp)
	if len(imgs) == 0 {
		// Pasa cuando el filtro de seguridad corta la generacion. Es transitorio
		// a proposito: el mismo prompt suele pasar al segundo intento.
		return ai.Respuesta{Uso: u}, ai.Transitorio(p.Nombre(), "sin_imagen",
			"el modelo no devolvio ninguna imagen", nil)
	}
	u.Imagenes = len(imgs)

	return ai.Respuesta{Imagenes: imgs, Uso: u}, nil
}

func extraerImagenes(resp *genai.GenerateContentResponse) []ai.Imagen {
	var out []ai.Imagen
	for _, c := range resp.Candidates {
		if c.Content == nil {
			continue
		}
		for _, parte := range c.Content.Parts {
			if parte.InlineData != nil && len(parte.InlineData.Data) > 0 {
				out = append(out, ai.Imagen{
					Bytes: parte.InlineData.Data,
					MIME:  parte.InlineData.MIMEType,
				})
			}
		}
	}
	return out
}

func (p *Proveedor) completar(ctx context.Context, modelo string, pet ai.Peticion) (ai.Respuesta, error) {
	partes := make([]*genai.Part, 0, len(pet.Imagenes)+1)

	// Las imagenes van PRIMERO y el prompt al final: el modelo rinde mejor con
	// la instruccion despues del material que tiene que mirar.
	for _, img := range pet.Imagenes {
		parte := &genai.Part{
			InlineData: &genai.Blob{Data: img.Bytes, MIMEType: img.MIME},
		}
		// La densidad se fija POR IMAGEN, no en la config global: el nivel
		// maximo (ultra_high) solo existe a nivel de Part.
		if nivel := nivelDensidad(img.Densidad); nivel != "" {
			parte.MediaResolution = &genai.PartMediaResolution{Level: nivel}
		}
		partes = append(partes, parte)
	}
	partes = append(partes, genai.NewPartFromText(pet.Prompt))

	cfg := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
	}

	if len(pet.SchemaJSON) > 0 {
		var esquema any
		if err := json.Unmarshal(pet.SchemaJSON, &esquema); err != nil {
			return ai.Respuesta{}, ai.Terminal(p.Nombre(), "schema_invalido",
				"el JSON Schema no es JSON valido", err)
		}
		cfg.ResponseJsonSchema = esquema
	}

	if pet.Sistema != "" {
		cfg.SystemInstruction = genai.NewContentFromText(pet.Sistema, genai.RoleUser)
	}
	if pet.MaxTokens > 0 {
		cfg.MaxOutputTokens = int32(pet.MaxTokens)
	}

	contenido := []*genai.Content{{Parts: partes, Role: genai.RoleUser}}

	resp, err := p.cli.Models.GenerateContent(ctx, modelo, contenido, cfg)
	if err != nil {
		return ai.Respuesta{}, p.clasificar(err)
	}

	uso := uso(resp)

	texto := extraerTexto(resp)
	if strings.TrimSpace(texto) == "" {
		return ai.Respuesta{Uso: uso}, ai.Transitorio(p.Nombre(), "respuesta_vacia",
			"el modelo no devolvio contenido", nil)
	}

	return ai.Respuesta{JSON: []byte(texto), Uso: uso}, nil
}

func nivelDensidad(d ai.Densidad) genai.PartMediaResolutionLevel {
	switch d {
	case ai.DensidadBaja:
		return genai.PartMediaResolutionLevelMediaResolutionLow
	case ai.DensidadMedia:
		return genai.PartMediaResolutionLevelMediaResolutionMedium
	case ai.DensidadAlta:
		return genai.PartMediaResolutionLevelMediaResolutionHigh
	case ai.DensidadMaxima:
		return genai.PartMediaResolutionLevelMediaResolutionUltraHigh
	}
	return ""
}

// El costo NO se calcula aqui: el cliente lo resuelve con la tabla de precios,
// que es una sola para todos los proveedores.
func uso(resp *genai.GenerateContentResponse) ai.Uso {
	var u ai.Uso
	if m := resp.UsageMetadata; m != nil {
		u.TokensEntrada = int(m.PromptTokenCount)
		u.TokensSalida = int(m.CandidatesTokenCount)
	}
	return u
}

func extraerTexto(resp *genai.GenerateContentResponse) string {
	var b strings.Builder
	for _, c := range resp.Candidates {
		if c.Content == nil {
			continue
		}
		for _, part := range c.Content.Parts {
			b.WriteString(part.Text)
		}
	}
	return b.String()
}

// clasificar decide si vale la pena que otro proveedor lo intente.
//
// Terminal = el problema es la peticion (schema malo, contenido bloqueado,
// credencial invalida): probar en otro solo gasta dinero.
// Transitorio = el problema es este proveedor ahora (cuota, saturacion, corte).
// esCuotaDiaria distingue "se acabo la cuota del dia" de "vas muy rapido".
//
// Ante la duda dice que NO. Equivocarse hacia el reintento cuesta segundos;
// equivocarse hacia terminal cancela una carta que si se podia generar.
func esCuotaDiaria(msg string) bool {
	if !strings.Contains(msg, "429") && !strings.Contains(msg, "resource_exhausted") &&
		!strings.Contains(msg, "quota") {
		return false
	}
	for _, p := range []string{"limit: 0", "perday", "per day", "daily", "per_day"} {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
}

func (p *Proveedor) clasificar(err error) error {
	msg := strings.ToLower(err.Error())

	terminales := []struct{ patron, codigo string }{
		{"api key not valid", "credencial"},
		{"api_key_invalid", "credencial"},
		{"permission denied", "permiso"},
		{"invalid argument", "peticion_invalida"},
		{"safety", "moderacion"},
		{"blocked", "moderacion"},
	}
	for _, t := range terminales {
		if strings.Contains(msg, t.patron) {
			return ai.Terminal(p.Nombre(), t.codigo, err.Error(), err)
		}
	}

	// La cuota DIARIA es terminal: reintentar en segundos no la arregla, hay que
	// esperar al reset de medianoche hora del Pacifico. Distinguirla importa
	// porque son dos acciones opuestas — una se reintenta, la otra se pospone.
	//
	// Los patrones salen de como los reporta la API. NO se parsea `details[]`
	// por indice: quedo refutado que el cuerpo del 429 traiga garantizados
	// QuotaFailure/RetryInfo, y codificar contra una estructura no garantizada
	// rompe en silencio el dia que cambie.
	if esCuotaDiaria(msg) {
		return ai.Terminal(p.Nombre(), "cuota_diaria", err.Error(), err)
	}

	codigo := "error"
	switch {
	case strings.Contains(msg, "429"), strings.Contains(msg, "resource_exhausted"), strings.Contains(msg, "quota"):
		codigo = "cuota"
	case strings.Contains(msg, "503"), strings.Contains(msg, "unavailable"):
		codigo = "no_disponible"
	case strings.Contains(msg, "deadline"), strings.Contains(msg, "timeout"):
		codigo = "timeout"
	}
	return ai.Transitorio(p.Nombre(), codigo, err.Error(), err)
}
