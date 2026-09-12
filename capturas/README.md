# capturas

Estado visual de Tacu corriendo en local. Es un vistazo de cómo va, no un test de
regresión: se sobrescriben cuando cambia la interfaz.

Tomadas el 2026-09-12 contra `localhost:3000` (frontend) y `localhost:8080`
(backend), a 1440 px de ancho y página completa.

| Archivo | Pantalla | Qué muestra |
|---|---|---|
| `00-inicio.png` | `/` sin carta recordada | Lo que ve quien llega nuevo. Catálogo y Publicar con candado |
| `02-datos.png` | `/i/{id}/datos` | Nombre, tipo de negocio (Cevichería: 35 platos + 39 que no se parecen a ninguno) y foto del local |
| `03-carta.png` | `/i/{id}/carta` | Las hojas subidas |
| `04-revisar.png` | `/i/{id}/revisar` | "Todo cuadra": ningún precio marcado por el cruce de testigos |
| `05-fotos.png` | `/i/{id}/fotos` | Los 74 platos con foto generada |
| `06-publicar.png` | `/i/{id}/publicar` | Vista previa en teléfono. 74 platos · 74 con foto, sin publicar todavía |

Falta `/r/{slug}` — el catálogo público. No hay captura porque nada está publicado:
`restaurante.slug` está vacío y `importacion_publicada_id` en NULL.

## Cómo se vuelven a tomar

El editor va detrás del token del borrador, del que la base solo guarda el SHA-256;
el token en claro viaja una sola vez al crear y no se recupera. Para abrir la
importación que ya existe se le pone uno conocido **en la base de desarrollo**:

```sql
UPDATE restaurante SET token_hash = sha256('capturas-dev-2026'::bytea)
 WHERE id = '01a02bb1-41da-7949-9b07-e39632aa2153';
```

Después, con `make arriba`, `make dev` y `npm run dev` levantados, se conduce
chromium por CDP: se pone `tacu.token.{importacion}` en `localStorage` y se navega
paso a paso. No hace falta playwright ni puppeteer — Node 24 ya trae `WebSocket`.

Ninguna de estas capturas llama a la IA, así que volver a tomarlas no cuesta dinero.
