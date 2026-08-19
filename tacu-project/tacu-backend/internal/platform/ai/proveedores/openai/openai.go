// Package openai implementa el Proveedor de ai sobre la API de OpenAI.
//
// Se habla HTTP directo en vez de usar el SDK: son dos endpoints JSON, y asi la
// clasificacion de errores (terminal vs transitorio) queda en nuestras manos,
// que es de lo que depende que la cadena de modelos decida bien.
package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"tacu-backend/internal/platform/ai"
)

const (
	urlChat    = "https://api.openai.com/v1/chat/completions"
	urlImagen  = "https://api.openai.com/v1/images/generations"
	maxCuerpo  = 64 << 20 // las imagenes vuelven en base64 y pesan
	timeoutDef = 3 * time.Minute
)

type Proveedor struct {
	clave string
	http  *http.Client
}

func Nuevo(clave string) (*Proveedor, error) {
	if clave == "" {
		return nil, errors.New("openai: falta la API key")
	}
	return &Proveedor{clave: clave, http: &http.Client{Timeout: timeoutDef}}, nil
}

func (p *Proveedor) Nombre() string { return "openai" }

func (p *Proveedor) Soporta(c ai.Capacidad) bool {
	return c == ai.Vision || c == ai.Texto || c == ai.GenerarImagen
}

func (p *Proveedor) Ejecutar(ctx context.Context, modelo string, pet ai.Peticion) (ai.Respuesta, error) {
	if pet.Tarea.Capacidad() == ai.GenerarImagen {
		return p.generarImagen(ctx, modelo, pet)
	}
	return p.completar(ctx, modelo, pet)
}

// --- texto y vision ---

func (p *Proveedor) completar(ctx context.Context, modelo string, pet ai.Peticion) (ai.Respuesta, error) {
	contenido := make([]any, 0, len(pet.Imagenes)+1)

	// Las imagenes van primero y la instruccion al final: el modelo rinde mejor
	// leyendo la orden despues del material.
	for _, img := range pet.Imagenes {
		contenido = append(contenido, map[string]any{
			"type": "image_url",
			"image_url": map[string]string{
				"url": "data:" + img.MIME + ";base64," + base64.StdEncoding.EncodeToString(img.Bytes),
			},
		})
	}
	contenido = append(contenido, map[string]any{"type": "text", "text": pet.Prompt})

	mensajes := []any{}
	if pet.Sistema != "" {
		mensajes = append(mensajes, map[string]any{"role": "system", "content": pet.Sistema})
	}
	mensajes = append(mensajes, map[string]any{"role": "user", "content": contenido})

	cuerpo := map[string]any{"model": modelo, "messages": mensajes}
	if pet.MaxTokens > 0 {
		cuerpo["max_completion_tokens"] = pet.MaxTokens
	}

	if len(pet.SchemaJSON) > 0 {
		var esquema any
		if err := json.Unmarshal(pet.SchemaJSON, &esquema); err != nil {
			return ai.Respuesta{}, ai.Terminal(p.Nombre(), "schema_invalido",
				"el JSON Schema no es JSON valido", err)
		}
		cuerpo["response_format"] = map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "respuesta",
				"schema": esquema,
				"strict": true,
			},
		}
	}

	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}

	if err := p.pedir(ctx, urlChat, cuerpo, &resp); err != nil {
		return ai.Respuesta{}, err
	}

	uso := ai.Uso{
		TokensEntrada: resp.Usage.PromptTokens,
		TokensSalida:  resp.Usage.CompletionTokens,
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return ai.Respuesta{Uso: uso}, ai.Transitorio(p.Nombre(), "respuesta_vacia",
			"el modelo no devolvio contenido", nil)
	}

	// Un corte por longitud devuelve JSON truncado: parece exito y no lo es.
	if r := resp.Choices[0].FinishReason; r == "length" {
		return ai.Respuesta{Uso: uso}, ai.Transitorio(p.Nombre(), "truncado",
			"la respuesta se corto por limite de tokens", nil)
	}

	return ai.Respuesta{JSON: []byte(resp.Choices[0].Message.Content), Uso: uso}, nil
}

// --- generacion de imagenes ---

func (p *Proveedor) generarImagen(ctx context.Context, modelo string, pet ai.Peticion) (ai.Respuesta, error) {
	cuerpo := map[string]any{
		"model":  modelo,
		"prompt": pet.Prompt,
		"n":      1,
	}
	if pet.Tamano != "" {
		cuerpo["size"] = pet.Tamano
	}
	if pet.Calidad != "" {
		cuerpo["quality"] = pet.Calidad
	}

	var resp struct {
		Data []struct {
			B64 string `json:"b64_json"`
			// gpt-image-2 REESCRIBE el prompt antes de generar. Cuando una foto
			// sale mal hay que poder ver si el modelo recibio lo que se escribio
			// o un resumen que se comio el parrafo de la escala.
			RevisedPrompt string `json:"revised_prompt"`
		} `json:"data"`
	}

	if err := p.pedir(ctx, urlImagen, cuerpo, &resp); err != nil {
		return ai.Respuesta{}, err
	}
	if len(resp.Data) == 0 {
		return ai.Respuesta{}, ai.Transitorio(p.Nombre(), "sin_imagen",
			"la respuesta no trajo ninguna imagen", nil)
	}

	imagenes := make([]ai.Imagen, 0, len(resp.Data))
	for _, d := range resp.Data {
		datos, err := base64.StdEncoding.DecodeString(d.B64)
		if err != nil {
			return ai.Respuesta{}, ai.Transitorio(p.Nombre(), "base64_invalido",
				"la imagen devuelta no es base64 valido", err)
		}
		imagenes = append(imagenes, ai.Imagen{Bytes: datos, MIME: "image/png"})
	}

	return ai.Respuesta{
		Imagenes:        imagenes,
		PromptReescrito: resp.Data[0].RevisedPrompt,
		Uso:             ai.Uso{Imagenes: len(imagenes)},
	}, nil
}

// --- transporte ---

func (p *Proveedor) pedir(ctx context.Context, url string, cuerpo any, destino any) error {
	datos, err := json.Marshal(cuerpo)
	if err != nil {
		return ai.Terminal(p.Nombre(), "peticion_invalida", "no se pudo serializar la peticion", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(datos))
	if err != nil {
		return ai.Terminal(p.Nombre(), "peticion_invalida", err.Error(), err)
	}
	req.Header.Set("Authorization", "Bearer "+p.clave)
	req.Header.Set("Content-Type", "application/json")

	res, err := p.http.Do(req)
	if err != nil {
		return ai.Transitorio(p.Nombre(), "red", err.Error(), err)
	}
	defer func() { _ = res.Body.Close() }()

	respuesta, err := io.ReadAll(io.LimitReader(res.Body, maxCuerpo))
	if err != nil {
		return ai.Transitorio(p.Nombre(), "lectura", err.Error(), err)
	}

	if res.StatusCode != http.StatusOK {
		return p.clasificar(res.StatusCode, respuesta)
	}

	if err := json.Unmarshal(respuesta, destino); err != nil {
		return ai.Transitorio(p.Nombre(), "respuesta_ilegible",
			fmt.Sprintf("HTTP 200 pero el cuerpo no es el JSON esperado: %s", recortar(respuesta)), err)
	}
	return nil
}

// clasificar decide si vale la pena que el siguiente modelo de la cadena lo
// intente. Un 401 o un 400 fallan igual en todos lados; un 429 o un 503 no.
func (p *Proveedor) clasificar(estado int, cuerpo []byte) error {
	var e struct {
		Error struct {
			Message string `json:"message"`
			Code    string `json:"code"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	_ = json.Unmarshal(cuerpo, &e)

	mensaje := e.Error.Message
	if mensaje == "" {
		mensaje = recortar(cuerpo)
	}
	codigo := e.Error.Code
	if codigo == "" {
		codigo = fmt.Sprintf("http_%d", estado)
	}

	switch {
	case estado == http.StatusTooManyRequests,
		estado >= 500:
		return ai.Transitorio(p.Nombre(), codigo, mensaje, nil)

	case estado == http.StatusUnauthorized,
		estado == http.StatusForbidden,
		estado == http.StatusBadRequest,
		estado == http.StatusNotFound:
		// 404 incluye "modelo no existe": reintentar no lo hace aparecer.
		return ai.Terminal(p.Nombre(), codigo, mensaje, nil)
	}

	return ai.Transitorio(p.Nombre(), codigo, mensaje, nil)
}

func recortar(b []byte) string {
	const max = 300
	if len(b) > max {
		return string(b[:max]) + "..."
	}
	return string(b)
}
