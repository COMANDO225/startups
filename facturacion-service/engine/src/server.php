<?php

declare(strict_types=1);

require __DIR__ . '/../vendor/autoload.php';

use Engine\Emitter;
use Engine\Resumen;

// El contrato con el servicio Go es "siempre JSON". PHP, por defecto, imprime
// warnings y fatales como HTML dentro del cuerpo de la respuesta y corrompe el
// JSON: el cliente recibe "invalid character '<'" y no sabe que paso.
ini_set('display_errors', '0');
ini_set('log_errors', '1');
ini_set('error_log', 'php://stderr');

// Las deprecaciones se registran, no se lanzan: son ruido de librerias de
// terceros (Twig avisa que "spaceless" quedo obsoleto dentro de las plantillas
// de Greenter) y convertirlas en excepcion rompe una emision perfectamente
// valida. Warnings y notices si se lanzan: ahi el codigo propio esta mal.
set_error_handler(static function (int $severity, string $message, string $file, int $line): bool {
    if ($severity & (E_DEPRECATED | E_USER_DEPRECATED)) {
        error_log("deprecado: $message en $file:$line");
        return true;
    }
    throw new ErrorException($message, 0, $severity, $file, $line);
});

$responder = static function (int $status, array $body): void {
    if (ob_get_length() !== false) {
        ob_clean();
    }
    http_response_code($status);
    header('Content-Type: application/json; charset=utf-8');
    echo json_encode($body, JSON_UNESCAPED_UNICODE);
};

register_shutdown_function(static function () use ($responder): void {
    $fatal = error_get_last();
    if ($fatal !== null && in_array($fatal['type'], [E_ERROR, E_PARSE, E_CORE_ERROR, E_COMPILE_ERROR], true)) {
        $responder(500, ['error' => 'error fatal del motor: ' . $fatal['message']]);
    }
});

ob_start();

$path   = parse_url($_SERVER['REQUEST_URI'] ?? '/', PHP_URL_PATH);
$method = $_SERVER['REQUEST_METHOD'] ?? 'GET';

try {
    if ($path === '/health') {
        $responder(200, ['status' => 'ok']);
        return;
    }

    if ($method !== 'POST') {
        $responder(405, ['error' => 'metodo no permitido']);
        return;
    }

    if (!in_array($path, ['/emitir', '/firmar', '/resumen', '/baja', '/consultar-ticket'], true)) {
        $responder(404, ['error' => 'ruta no encontrada']);
        return;
    }

    $raw  = file_get_contents('php://input') ?: '';
    $body = json_decode($raw, true);

    if (!is_array($body)) {
        $responder(400, ['error' => 'json invalido']);
        return;
    }

    foreach (['cert_pem', 'ruc', 'sol_user', 'sol_pass'] as $campo) {
        if (empty($body[$campo])) {
            $responder(400, ['error' => "falta el campo obligatorio: $campo"]);
            return;
        }
    }

    $emitter = new Emitter(
        $body['cert_pem'],
        $body['ruc'],
        $body['sol_user'],
        $body['sol_pass'],
        (bool) ($body['produccion'] ?? false),
    );

    switch ($path) {
        case '/emitir':
            $responder(200, $emitter->emitir($body['comprobante'] ?? []));
            break;
        case '/firmar':
            $responder(200, $emitter->firmar($body['comprobante'] ?? []));
            break;
        case '/resumen':
            $responder(200, (new Resumen($emitter->see()))->enviarResumenDiario($body['resumen'] ?? []));
            break;
        case '/baja':
            $responder(200, (new Resumen($emitter->see()))->enviarComunicacionBaja($body['resumen'] ?? []));
            break;
        case '/consultar-ticket':
            if (empty($body['ticket'])) {
                $responder(400, ['error' => 'falta el campo obligatorio: ticket']);
                break;
            }
            $responder(200, $emitter->consultarTicket($body['ticket']));
            break;
    }
} catch (InvalidArgumentException $e) {
    $responder(400, ['error' => $e->getMessage()]);
} catch (Throwable $e) {
    error_log(sprintf('%s: %s en %s:%d', get_class($e), $e->getMessage(), $e->getFile(), $e->getLine()));
    $responder(500, ['error' => $e->getMessage()]);
} finally {
    ob_end_flush();
}
