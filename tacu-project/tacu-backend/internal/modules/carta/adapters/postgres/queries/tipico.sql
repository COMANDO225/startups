-- El banco de platos: que ES cada plato, para poder fotografiarlo. Ver la
-- migracion 00007.

-- Se trae entero y no por nombre: emparejar es cosa del dominio —patron mas
-- largo, y el descrito le gana al que no lo esta— y esa regla no se escribe dos
-- veces, una en Go y otra en SQL. Son 419 filas flacas y se lee una vez por
-- carta, no por plato.
-- name: ListarPlatosTipicos :many
SELECT clave, nombre, cocina, curso, aspecto, recipiente, guarnicion, jamas,
       envasado, patrones, ingredientes
  FROM plato_tipico
 ORDER BY clave;

-- Para la foto ya solo hace falta el que se emparejo al leer.
-- name: PlatoTipicoPorClave :one
SELECT clave, nombre, cocina, curso, aspecto, recipiente, guarnicion, jamas,
       envasado, patrones
  FROM plato_tipico
 WHERE clave = @clave;

-- Lo que el banco aprende de una carta nueva.
--
-- DO NOTHING y no DO UPDATE: si la clave ya existe, gana lo que hay. El canon
-- esta escrito y medido a mano, y una respuesta de la IA no puede pisarlo. Lo
-- que se aprende nace sin revisar, para poder mirarlo despues con fotocheck.
-- name: GuardarPlatoTipico :exec
INSERT INTO plato_tipico
    (clave, nombre, cocina, curso, aspecto, recipiente, guarnicion, jamas, patrones, origen, revisado)
VALUES
    (@clave, @nombre, @cocina, @curso, @aspecto, @recipiente, @guarnicion, @jamas, @patrones, 'ia', false)
ON CONFLICT (clave) DO UPDATE SET
    aspecto    = EXCLUDED.aspecto,
    recipiente = EXCLUDED.recipiente,
    guarnicion = EXCLUDED.guarnicion,
    jamas      = EXCLUDED.jamas,
    patrones   = EXCLUDED.patrones,
    origen     = 'ia',
    actualizado_at = now()
 WHERE plato_tipico.aspecto = '' AND plato_tipico.origen <> 'canon';

-- Reengancha UN plato con el banco sin tocar nada mas.
--
-- No pasa por GuardarCarta a proposito: esa empieza borrando los platos y los
-- reinserta con ids nuevos, y de esos ids cuelgan las fotos ya pagadas.
--
-- El del DUENO no se pisa: si corrigio a mano que su "Combo Tribuna" es un
-- ceviche, ampliar el banco no puede deshacerlo.
-- name: MarcarTipicoDePlato :exec
UPDATE plato
   SET plato_tipico  = @clave,
       tipico_origen = @origen
 WHERE id = @plato_id
   AND tipico_origen <> 'dueno';

-- Lo que trajo la cosecha y nadie ha descrito: nombre e ingredientes, sin una
-- sola linea de como se ve. Es la materia prima de cmd/describir.
--
-- El canon no entra: esta escrito y medido a mano.
-- name: PlatosTipicosSinDescribir :many
SELECT clave, nombre, cocina, curso, patrones, ingredientes
  FROM plato_tipico
 WHERE aspecto = '' AND origen <> 'canon'
 ORDER BY clave
 LIMIT @limite;
