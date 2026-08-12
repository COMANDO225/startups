# FrankenPHP en modo clasico: una peticion por proceso, igual que php-fpm, pero
# en un solo binario y sin nginx delante.
#
# Reemplaza a `php -S`, que la propia documentacion de PHP marca como servidor de
# desarrollo: no supervisa procesos, no reinicia ordenadamente y no limita
# peticiones. Con PHP_CLI_SERVER_WORKERS aguantaba la carga, pero no es a lo que
# se le confia el camino del dinero.
FROM dunglas/frankenphp:1-php8.3-alpine

RUN install-php-extensions soap zip

COPY --from=composer:2 /usr/bin/composer /usr/bin/composer

WORKDIR /app

COPY composer.json ./
RUN composer install --no-dev --no-interaction --no-progress

COPY src/ ./src/
COPY public/ ./public/

# Sin nombre de dominio, ":8000" hace que escuche en HTTP plano: el motor solo se
# alcanza desde la red interna del compose, nunca desde fuera.
ENV SERVER_NAME=:8000
ENV FRANKENPHP_CONFIG=""

EXPOSE 8000

# No se usa el modo worker de FrankenPHP a proposito: server.php da por hecho un
# proceso limpio por peticion (superglobales, ob_start, shutdown handler), y el
# estado compartido entre peticiones filtraria credenciales de un emisor a otro.
