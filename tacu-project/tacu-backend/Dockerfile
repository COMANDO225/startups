# Imagen del servicio de Tacu.
#
#   docker build -t tacu-api .
#   docker run --rm -p 8080:8080 \
#     -e TACU_BD_DSN="postgres://tacu:tacu@host.docker.internal:5433/tacu?sslmode=disable" \
#     -e GEMINI_API_KEY=... tacu-api
#
# Lo que entra en el contexto lo decide .dockerignore, que es una LISTA BLANCA.

FROM golang:1.26-alpine AS compilacion
WORKDIR /src

# Las dependencias en su propia capa: cambiar una linea de Go no vuelve a bajar
# el modulo entero.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 da un binario estatico, que es lo que permite que la imagen final
# sea scratch: sin libc que enlazar no hace falta sistema operativo debajo.
# -trimpath borra las rutas de la maquina que compilo, que si no acaban dentro
# del binario publicado.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /salida/api ./cmd/api

# El almacen de disco escribe aqui. Se crea EN ESTA etapa y no en la final:
# scratch no tiene shell, asi que no hay ningun mkdir posible alli, y un usuario
# no root no puede crear su propio directorio en la raiz al arrancar.
RUN mkdir -p /salida/datos/media && chown -R 65532:65532 /salida/datos

FROM scratch

# Sin esto toda llamada HTTPS a Gemini o a OpenAI falla con "certificate signed
# by unknown authority": scratch no trae ni un certificado raiz.
COPY --from=compilacion /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY --from=compilacion /salida/api /api
COPY --from=compilacion /src/config /config
COPY --from=compilacion /salida/datos /datos

# UID numerico y no un nombre: scratch no tiene /etc/passwd donde resolverlo.
# 65532 es el "nonroot" de distroless, el mismo que esperan las politicas de
# Kubernetes que exigen runAsNonRoot.
USER 65532:65532

# Informativo; el puerto real sale de config/config.yaml (servidor.puerto).
EXPOSE 8080

ENTRYPOINT ["/api"]
