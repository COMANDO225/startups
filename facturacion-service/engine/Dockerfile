FROM php:8.3-cli-alpine

RUN apk add --no-cache libzip \
    && apk add --no-cache --virtual .build-deps libxml2-dev libzip-dev \
    && docker-php-ext-install soap zip \
    && apk del .build-deps

COPY --from=composer:2 /usr/bin/composer /usr/bin/composer

WORKDIR /app

COPY composer.json ./
RUN composer install --no-dev --no-interaction --no-progress

COPY src/ ./src/

EXPOSE 8000

# El servidor embebido de PHP atiende una peticion a la vez. Con varios workers
# de River en paralelo, uno bloquea a los demas hasta el timeout. PHP_CLI_SERVER_WORKERS
# lo hace forkear procesos hijos.
# ponytail: alcanza para este volumen; migrar a php-fpm + nginx si sube la carga.
ENV PHP_CLI_SERVER_WORKERS=6

CMD ["php", "-S", "0.0.0.0:8000", "src/server.php"]
