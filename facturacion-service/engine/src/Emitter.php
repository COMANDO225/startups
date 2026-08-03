<?php

declare(strict_types=1);

namespace Engine;

use DateTime;
use InvalidArgumentException;
use Greenter\Model\Client\Client;
use Greenter\Model\Company\Address;
use Greenter\Model\Company\Company;
use Greenter\Model\Sale\FormaPagos\FormaPagoContado;
use Greenter\Model\Sale\Invoice;
use Greenter\Model\Sale\Legend;
use Greenter\Model\Sale\Note;
use Greenter\Model\Sale\SaleDetail;
use Greenter\See;
use Greenter\Ws\Services\SunatEndpoints;

final class Emitter
{
    private See $see;

    public function __construct(
        string $certPem,
        string $ruc,
        string $solUser,
        string $solPass,
        bool $produccion,
    ) {
        $this->see = new See();
        $this->see->setCertificate($certPem);
        $this->see->setService($produccion ? SunatEndpoints::FE_PRODUCCION : SunatEndpoints::FE_BETA);
        $this->see->setClaveSOL($ruc, $solUser, $solPass);
    }

    public function emitir(array $data): array
    {
        $invoice = $this->esNota($data) ? $this->buildNote($data) : $this->buildInvoice($data);

        $response = [
            'nombre'  => $invoice->getName(),
            'xml_b64' => base64_encode($this->see->getXmlSigned($invoice)),
        ];

        $result = $this->see->send($invoice);

        if (!$result->isSuccess()) {
            $err = $result->getError();
            return $response + [
                'estado'  => 'rechazado',
                'codigo'  => $err->getCode(),
                'mensaje' => $err->getMessage(),
            ];
        }

        $cdr = $result->getCdrResponse();

        return $response + [
            'estado'       => $cdr->getCode() === '0' ? 'aceptado' : 'observado',
            'codigo'       => $cdr->getCode(),
            'mensaje'      => $cdr->getDescription(),
            'cdr_b64'      => base64_encode($result->getCdrZip()),
            'observaciones' => $cdr->getNotes(),
        ];
    }

    public function consultarTicket(string $ticket): array
    {
        $result = $this->see->getStatus($ticket);

        if (!$result->isSuccess()) {
            $err = $result->getError();
            return [
                'estado'  => 'pendiente',
                'codigo'  => $err->getCode(),
                'mensaje' => $err->getMessage(),
            ];
        }

        $cdr = $result->getCdrResponse();

        return [
            'estado'        => $cdr->getCode() === '0' ? 'aceptado' : 'observado',
            'codigo'        => $cdr->getCode(),
            'mensaje'       => $cdr->getDescription(),
            'cdr_b64'       => base64_encode($result->getCdrZip()),
            'observaciones' => $cdr->getNotes(),
        ];
    }

    /** Las boletas se firman pero no se envian: viajan en el resumen diario. */
    public function firmar(array $data): array
    {
        $invoice = $this->esNota($data) ? $this->buildNote($data) : $this->buildInvoice($data);

        return [
            'nombre'  => $invoice->getName(),
            'xml_b64' => base64_encode($this->see->getXmlSigned($invoice)),
            'estado'  => 'pendiente_resumen',
            'codigo'  => '',
            'mensaje' => 'firmado, pendiente de resumen diario',
        ];
    }

    private function esNota(array $d): bool
    {
        return in_array($d['tipo_doc'] ?? '', ['07', '08'], true);
    }

    public function see(): See
    {
        return $this->see;
    }

    /** Puente para que Resumen.php reutilice la misma validacion. */
    public static function requerirPublico(array $d, string $campo, string $contexto = '')
    {
        return self::requerir($d, $campo, $contexto);
    }

    /**
     * Un campo faltante debe dar un error claro, no un warning de PHP: los
     * warnings terminan en el cuerpo de la respuesta y rompen el JSON.
     */
    private static function requerir(array $d, string $campo, string $contexto = '')
    {
        if (!array_key_exists($campo, $d)) {
            $ruta = $contexto === '' ? $campo : "$contexto.$campo";
            throw new InvalidArgumentException("falta el campo: $ruta");
        }
        return $d[$campo];
    }

    private function buildNote(array $d): Note
    {
        $ref = self::requerir($d, 'referencia', 'comprobante');
        foreach (['tipo_doc', 'serie_numero'] as $campo) {
            self::requerir($ref, $campo, 'comprobante.referencia');
        }

        $base = $this->buildInvoice($d);

        return (new Note())
            ->setUblVersion('2.1')
            ->setTipoDoc($d['tipo_doc'])
            ->setSerie($d['serie'])
            ->setCorrelativo((string) $d['correlativo'])
            ->setFechaEmision($base->getFechaEmision())
            ->setTipDocAfectado($ref['tipo_doc'])
            ->setNumDocfectado($ref['serie_numero'])
            ->setCodMotivo(self::requerir($d, 'codigo_motivo', 'comprobante'))
            ->setDesMotivo(self::requerir($d, 'descripcion_motivo', 'comprobante'))
            ->setTipoMoneda($d['moneda'] ?? 'PEN')
            ->setCompany($base->getCompany())
            ->setClient($base->getClient())
            ->setMtoOperGravadas($base->getMtoOperGravadas())
            ->setMtoIGV($base->getMtoIGV())
            ->setTotalImpuestos($base->getTotalImpuestos())
            ->setMtoImpVenta($base->getMtoImpVenta())
            ->setDetails($base->getDetails())
            ->setLegends($base->getLegends());
    }

    private function buildInvoice(array $d): Invoice
    {
        foreach (['emisor', 'receptor', 'items', 'totales', 'serie', 'correlativo', 'tipo_doc'] as $campo) {
            self::requerir($d, $campo, 'comprobante');
        }

        if (!is_array($d['items']) || $d['items'] === []) {
            throw new InvalidArgumentException('comprobante.items no puede estar vacio');
        }

        $emisor = $d['emisor'];
        $company = (new Company())
            ->setRuc(self::requerir($emisor, 'ruc', 'comprobante.emisor'))
            ->setRazonSocial(self::requerir($emisor, 'razon_social', 'comprobante.emisor'))
            ->setNombreComercial($emisor['nombre_comercial'] ?? $emisor['razon_social'])
            ->setAddress((new Address())
                ->setUbigueo($emisor['ubigeo'] ?? '150101')
                ->setDepartamento($emisor['departamento'] ?? 'LIMA')
                ->setProvincia($emisor['provincia'] ?? 'LIMA')
                ->setDistrito($emisor['distrito'] ?? 'LIMA')
                ->setUrbanizacion('-')
                ->setDireccion(self::requerir($emisor, 'direccion', 'comprobante.emisor')));

        $rec = $d['receptor'];
        $client = (new Client())
            ->setTipoDoc(self::requerir($rec, 'tipo_doc', 'comprobante.receptor'))
            ->setNumDoc(self::requerir($rec, 'num_doc', 'comprobante.receptor'))
            ->setRznSocial(self::requerir($rec, 'razon_social', 'comprobante.receptor'));

        $details = [];
        foreach ($d['items'] as $n => $i) {
            foreach (['codigo', 'descripcion', 'cantidad', 'valor_unitario', 'valor_venta',
                      'base_igv', 'igv', 'total_impuestos', 'precio_unitario'] as $campo) {
                self::requerir($i, $campo, "comprobante.items[$n]");
            }

            $details[] = (new SaleDetail())
                ->setCodProducto($i['codigo'])
                ->setUnidad($i['unidad'] ?? 'NIU')
                ->setDescripcion($i['descripcion'])
                ->setCantidad($i['cantidad'])
                ->setMtoValorUnitario($i['valor_unitario'])
                ->setMtoValorVenta($i['valor_venta'])
                ->setMtoBaseIgv($i['base_igv'])
                ->setPorcentajeIgv($i['porcentaje_igv'] ?? 18.00)
                ->setIgv($i['igv'])
                ->setTipAfeIgv($i['tipo_afectacion'] ?? '10')
                ->setTotalImpuestos($i['total_impuestos'])
                ->setMtoPrecioUnitario($i['precio_unitario']);
        }

        $t = $d['totales'];
        foreach (['oper_gravadas', 'igv', 'total_impuestos', 'valor_venta', 'subtotal', 'importe_total'] as $campo) {
            self::requerir($t, $campo, 'comprobante.totales');
        }

        // setFormaPago es obligatorio en UBL 2.1: sin el, SUNAT responde 3244
        // "debe consignar el tipo de transaccion", mensaje que no sugiere la causa.
        return (new Invoice())
            ->setUblVersion('2.1')
            ->setTipoOperacion($d['tipo_operacion'] ?? '0101')
            ->setTipoDoc($d['tipo_doc'])
            ->setSerie($d['serie'])
            ->setCorrelativo((string) $d['correlativo'])
            ->setFechaEmision(new DateTime($d['fecha_emision']))
            ->setTipoMoneda($d['moneda'] ?? 'PEN')
            ->setFormaPago(new FormaPagoContado())
            ->setCompany($company)
            ->setClient($client)
            ->setMtoOperGravadas($t['oper_gravadas'])
            ->setMtoIGV($t['igv'])
            ->setTotalImpuestos($t['total_impuestos'])
            ->setValorVenta($t['valor_venta'])
            ->setSubTotal($t['subtotal'])
            ->setMtoImpVenta($t['importe_total'])
            ->setDetails($details)
            ->setLegends([
                (new Legend())->setCode('1000')->setValue(self::requerir($d, 'monto_letras', 'comprobante')),
            ]);
    }
}
