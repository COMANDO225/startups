# Herramientas de desarrollo de Tacu.
#
# "make ayuda" (o "make" a secas) lista todo. Este archivo es la respuesta a
# "que comandos hay que recordar": si un comando hay que teclearlo de memoria,
# es que falta un objetivo aqui.

# El DSN de desarrollo, el mismo que levanta compose.yaml. Se pisa con el
# entorno: TACU_BD_DSN=... make migrar
TACU_BD_DSN ?= postgres://tacu:tacu@localhost:5433/tacu?sslmode=disable

# Las herramientas se invocan con "go run <modulo>@latest": no exigen instalar
# nada y la version queda anclada al go.sum del propio modulo remoto, no al
# nuestro. La primera corrida descarga; despues sale de la cache de modulos.
GOOSE  := go run github.com/pressly/goose/v3/cmd/goose@latest
SQLC   := go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest
LINTER := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
AIR    := go run github.com/air-verse/air@latest

# Nada de aqui se paraleliza. No es capricho: en esta maquina "make" es un alias
# de "make -j32", y con eso "revisar" lanzaba formato, vet, lint y los tests a la
# vez, entrelazando la salida hasta no saber que paso fallo. Los objetivos duran
# segundos y varios se pisarian entre si (migrar contra arriba, lint contra
# prueba compilando el mismo paquete). El orden vale mas que el paralelismo.
.NOTPARALLEL:

.DEFAULT_GOAL := ayuda
.PHONY: ayuda arriba abajo migrar revertir estado generar compilar api dev prueba prueba-e2e revisar formato vet lint limpiar

ayuda: ## Lista los objetivos
	@echo "Tacu - objetivos disponibles:"
	@grep -hE '^[a-z][a-z0-9-]*:.*##' $(MAKEFILE_LIST) | sort | awk -F':.*## ' '{printf "  %-12s %s\n", $$1, $$2}'
	@echo ""
	@echo "  TACU_BD_DSN = $(TACU_BD_DSN)"

# --- entorno ---

# --wait no devuelve el control hasta que el healthcheck de compose.yaml pasa.
# Sin el, "up -d" vuelve con Postgres todavia arrancando y la primera migracion
# falla por carrera contra un puerto que ya escucha pero aun rechaza conexiones.
# Sale distinto de cero si el contenedor nunca llega a sano, asi que encadenarlo
# con "make arriba migrar" es seguro.
arriba: ## Levanta Postgres y ESPERA a que acepte conexiones
	docker compose up -d --wait
	@echo "postgres listo en $(TACU_BD_DSN)"

abajo: ## Apaga Postgres (conserva los datos)
	docker compose down

# --- base de datos ---

migrar: ## Aplica las migraciones pendientes
	$(GOOSE) -dir sql/migrations postgres "$(TACU_BD_DSN)" up

revertir: ## Deshace la ultima migracion
	$(GOOSE) -dir sql/migrations postgres "$(TACU_BD_DSN)" down

estado: ## Muestra que migraciones estan aplicadas
	$(GOOSE) -dir sql/migrations postgres "$(TACU_BD_DSN)" status

generar: ## Regenera el codigo de sqlc (cartadb)
	$(SQLC) generate

# --- servicio ---

compilar: ## Compila el binario en bin/api
	go build -o bin/api ./cmd/api

# Se compila a bin/api y se ejecuta el binario, NUNCA "go run ./cmd/api": go run
# lanza un binario temporal como HIJO, asi que un kill al proceso de arriba mata
# al padre y deja al hijo escuchando el puerto. Se pierden minutos peleando
# contra un servidor viejo que ya no es el codigo que se acaba de escribir.
# Con bin/api el nombre es conocido y "pkill -f bin/api" lo remata.
api: compilar ## Compila y levanta el servicio
	@set -a; [ -f .env ] && . ./.env; set +a; \
	TACU_BD_DSN="$(TACU_BD_DSN)" ./bin/api

# air (github.com/air-verse/air) sigue siendo lo vigente para recarga en caliente
# en Go en 2026. Se configura por banderas y no por .air.toml para no tener dos
# fuentes de verdad; compila al MISMO bin/api que "make api".
dev: ## Igual que api pero recompila al guardar
	@set -a; [ -f .env ] && . ./.env; set +a; \
	TACU_BD_DSN="$(TACU_BD_DSN)" $(AIR) \
		-build.cmd "go build -o bin/api ./cmd/api" \
		-build.entrypoint "./bin/api" \
		-build.include_ext "go,yaml,sql" \
		-build.exclude_dir "bin,tmp,datos,testdata,experimentos,.git" \
		-build.stop_on_error "true"

# --- pruebas ---

# Un solo nombre de variable en todo el proyecto: el servidor y los tests leen
# pasa explicito para que apunten a la misma base que se acaba de migrar aunque
# el DSN se haya pisado desde el entorno. Sin Postgres delante se saltan solos.
prueba: ## Corre los tests con -race
	TACU_BD_DSN="$(TACU_BD_DSN)" go test -race ./...

prueba-e2e: ## Tests contra la IA real. GASTA DINERO (~0.02 USD por corrida)
	@echo "ATENCION: esto llama a Gemini de verdad y cuesta ~\$$0.02 por corrida."
	@echo "Ctrl-C en 5 segundos para arrepentirse."
	@sleep 5
	@set -a; [ -f .env ] && . ./.env; set +a; \
	TACU_BD_DSN="$(TACU_BD_DSN)" TACU_E2E=1 go test -race -run E2E -v ./...

# --- calidad ---

revisar: formato vet lint prueba ## fmt + vet + lint + tests. La puerta de "esta bien"
	@echo "todo en verde"

# gofmt -l en vez de "go fmt ./...": reescribir en silencio deja pasar la puerta
# de calidad (siempre verde) y mete cambios ajenos en el diff de quien la corre.
formato: ## Falla si hay archivos sin gofmt
	@salida="$$(gofmt -l .)"; \
	if [ -n "$$salida" ]; then echo "sin formatear:"; echo "$$salida"; exit 1; fi

vet: ## go vet
	go vet ./...

lint: ## golangci-lint segun .golangci.yml
	$(LINTER) run

# --- limpieza ---

limpiar: ## Borra binarios, temporales, media local y el volumen de la base
	rm -rf bin tmp
	rm -rf datos/media
	docker compose down -v

# Para volver a probar el flujo de fotos desde cero sin perder las cartas ya
# leidas ni el estilo configurado.
#
# Resetea el ESTADO ademas de borrar los archivos, y ese es el punto: dejar
# platos en 'pendiente' o 'generando' con la imagen borrada hace que el worker
# los vuelva a tomar al arrancar y se pague otra vez la carta entera sin
# haberlo pedido. base_foto y las cartas subidas no se tocan.
limpiar-fotos: ## Borra las fotos generadas y deja los platos sin foto. NO toca cartas ni estilo
	rm -rf datos/media/fotos
	@psql "$(TACU_BD_DSN)" -v ON_ERROR_STOP=1 -c "\
		UPDATE plato SET foto_estado = 'vacia', foto_origen = '', foto_clave = '', \
		       foto_intentos = 0, foto_tomada_at = NULL, foto_actualizada_at = NULL \
		 WHERE foto_estado <> 'vacia' OR foto_clave <> '';"
