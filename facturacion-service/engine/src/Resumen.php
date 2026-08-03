<?php

declare(strict_types=1);

namespace Engine;

use DateTime;
use Greenter\Model\Company\Company;
use Greenter\Model\Summary\Summary;
use Greenter\Model\Summary\SummaryDetail;
use Greenter\Model\Voided\Voided;
use Greenter\Model\Voided\VoidedDetail;
use Greenter\See;
use InvalidArgumentException;

/**
 * Las boletas no se envian una por una: se informan en un Resumen Diario (RC).
 * SUNAT responde con un ticket, no con un CDR, y hay que consultarlo despues.
 */
final class Resumen
{
    public function __construct(private See $see)
    {
    }

    public function enviarResumenDiario(array $d): array
    {
        $summary = (new Summary())
            ->setFecGeneracion(new DateTime(Emitter::requerirPublico($d, 'fecha_ref', 'resumen')))
            ->setFecResumen(new DateTime($d['fecha_emision'] ?? 'now'))
            ->setCorrelativo((string) Emitter::requerirPublico($d, 'correlativo', 'resumen'))
            ->setMoneda($d['moneda'] ?? 'PEN')
            ->setCompany($this->company($d))
            ->setDetails($this->detalles($d));

        return $this->enviar($summary);
    }

    public function enviarComunicacionBaja(array $d): array
    {
        $voided = (new Voided())
            ->setCorrelativo((string) Emitter::requerirPublico($d, 'correlativo', 'baja'))
            ->setFecGeneracion(new DateTime(Emitter::requerirPublico($d, 'fecha_ref', 'baja')))
            ->setFecComunicacion(new DateTime($d['fecha_emision'] ?? 'now'))
            ->setCompany($this->company($d))
            ->setDetails($this->detallesBaja($d));

        return $this->enviar($voided);
    }

    private function enviar($documento): array
    {
        $respuesta = [
            'nombre'  => $documento->getName(),
            'xml_b64' => base64_encode($this->see->getXmlSigned($documento)),
        ];

        $result = $this->see->send($documento);

        if (!$result->isSuccess()) {
            $err = $result->getError();
            return $respuesta + [
                'estado'  => 'rechazado',
                'codigo'  => $err->getCode(),
                'mensaje' => $err->getMessage(),
            ];
        }

        return $respuesta + [
            'estado'  => 'ticket_pendiente',
            'ticket'  => $result->getTicket(),
            'codigo'  => '',
            'mensaje' => 'enviado, pendiente de consulta',
        ];
    }

    private function company(array $d): Company
    {
        $e = Emitter::requerirPublico($d, 'emisor', 'resumen');
        return (new Company())
            ->setRuc($e['ruc'])
            ->setRazonSocial($e['razon_social'])
            ->setNombreComercial($e['nombre_comercial'] ?? $e['razon_social']);
    }

    /** @return SummaryDetail[] */
    private function detalles(array $d): array
    {
        $items = Emitter::requerirPublico($d, 'detalles', 'resumen');
        if (!is_array($items) || $items === []) {
            throw new InvalidArgumentException('resumen.detalles no puede estar vacio');
        }

        $out = [];
        foreach ($items as $n => $i) {
            foreach (['tipo_doc', 'serie_numero', 'estado', 'cliente_tipo', 'cliente_numero', 'total'] as $campo) {
                Emitter::requerirPublico($i, $campo, "resumen.detalles[$n]");
            }

            // Los montos viajan como string desde Go para no perder centavos;
            // Greenter los exige como float, asi que se convierten aqui, en el
            // ultimo paso antes de generar el XML.
            $detalle = (new SummaryDetail())
                ->setTipoDoc($i['tipo_doc'])
                ->setSerieNro($i['serie_numero'])
                ->setEstado((string) $i['estado'])
                ->setClienteTipo($i['cliente_tipo'])
                ->setClienteNro($i['cliente_numero'])
                ->setTotal(self::monto($i['total']))
                ->setMtoOperGravadas(self::monto($i['oper_gravadas'] ?? 0))
                ->setMtoOperExoneradas(self::monto($i['oper_exoneradas'] ?? 0))
                ->setMtoOperInafectas(self::monto($i['oper_inafectas'] ?? 0))
                ->setMtoIGV(self::monto($i['igv'] ?? 0));

            if (!empty($i['doc_referencia'])) {
                $detalle->setDocReferencia($i['doc_referencia']);
            }

            $out[] = $detalle;
        }

        return $out;
    }

    private static function monto($v): float
    {
        return $v === null || $v === '' ? 0.0 : (float) $v;
    }

    /** @return VoidedDetail[] */
    private function detallesBaja(array $d): array
    {
        $items = Emitter::requerirPublico($d, 'detalles', 'baja');
        if (!is_array($items) || $items === []) {
            throw new InvalidArgumentException('baja.detalles no puede estar vacio');
        }

        $out = [];
        foreach ($items as $n => $i) {
            foreach (['tipo_doc', 'serie', 'correlativo', 'motivo'] as $campo) {
                Emitter::requerirPublico($i, $campo, "baja.detalles[$n]");
            }

            $out[] = (new VoidedDetail())
                ->setTipoDoc($i['tipo_doc'])
                ->setSerie($i['serie'])
                ->setCorrelativo((string) $i['correlativo'])
                ->setDesMotivoBaja($i['motivo']);
        }

        return $out;
    }
}
