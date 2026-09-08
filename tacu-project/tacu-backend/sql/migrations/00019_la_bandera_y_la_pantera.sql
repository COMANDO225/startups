-- +goose Up

-- Dos platos que el banco daba por conocidos y no lo eran: estaban metidos como
-- SINONIMOS del plato generico en sus patrones.
--
--   tiradito       -> ["tiradito", "tiradito-a-la-bandera", "tiradito-tricolor"]
--   leche-de-tigre -> ["leche-de-tigre", "leche-de-pantera"]
--
-- Al emparejar por regla, ni la IA llegaba a verlos ni la variante del nombre
-- sabia de ellos, asi que heredaban el aspecto del comun. Sobre la carta real:
-- "Tiradito a la Bandera" salio identico a "Tiradito a la Tribuna", y la "Leche
-- de Pantera" salio palida como las otras dos leches de la misma seccion.
--
-- Y los dos se distinguen justo por lo que se perdia:
--   bandera -> la comida DIBUJA la bandera del Peru: tres franjas paralelas,
--              roja, blanca y roja, sobre las laminas de pescado
--   pantera -> es NEGRA, y lo es por las conchas negras que lleva
--
-- Fuentes: buenazo.pe (tiradito tres colores, leche de pantera),
-- elcomercio.pe (tiradito tricolor).

UPDATE plato_tipico
   SET patrones = patrones - 'tiradito-a-la-bandera' - 'tiradito-tricolor',
       actualizado_at = now()
 WHERE clave = 'tiradito';

UPDATE plato_tipico
   SET patrones = patrones - 'leche-de-pantera',
       actualizado_at = now()
 WHERE clave = 'leche-de-tigre';

INSERT INTO plato_tipico
    (clave, nombre, cocina, curso, aspecto, recipiente, guarnicion, jamas, patrones, origen)
VALUES

('tiradito-a-la-bandera', 'Tiradito a la Bandera', 'cevicheria', 'entrada',
 'thin wide slices of raw white fish laid flat side by side in a single layer, covering the whole surface of the dish, and over them THREE PARALLEL BANDS OF SAUCE running across the fish from one side to the other, each band a solid block of colour that does not blend into the next: the first band deep red, the middle band white and creamy, the third band deep red again. The three stripes are even, straight and clearly separated, so the dish reads at a glance as the red-white-red flag of Peru. The red bands are rocoto and aji limo, glossy and vivid; the white band is a thick pale cream. The fish underneath stays visible at the edges of each band.',
 'a long narrow rectangular white platter, noticeably longer than it is wide, with the fish spread flat across it in one layer so the three stripes run the full width.',
 'a small heap of large white boiled corn kernels at one end and a wedge of lime, plus a scatter of finely chopped red rocoto and green coriander along the edges. Nothing is placed over the middle of the stripes: they have to stay readable.',
 'the sauces are NEVER mixed, never swirled together and never poured as one single colour over everything: three separate straight bands or the dish is not what it is called. It is never a mound and never piled up — the fish lies flat in one layer. The fish is raw and never cooked, never breaded and never in a soup.',
 '["tiradito-a-la-bandera","tiradito-bandera","tiradito-tricolor","tiradito-tres-colores","tiradito-a-la-tricolor"]', 'canon'),

('leche-de-pantera', 'Leche de Pantera', 'cevicheria', 'entrada',
 'a thick opaque marinade of a DARK colour — deep grey-purple, almost black, smoky and dense — filling the glass right up. That darkness is the whole point of the dish: it comes from the black clams blended into it. Suspended inside are pieces of dark grey-purple black clam flesh, white fish and prawn, with specks of red onion, red pepper and green coriander showing against the dark liquid.',
 'a wide stemmed glass goblet on a short foot —a footed coupe with a wide mouth, chilled— standing on a small white plate.',
 'the top of the glass is loaded: several crisp golden pieces of fried squid and fish sitting on the rim, one or two curled yellow plantain chips stuck in upright, a little tangle of dark green seaweed, a scatter of toasted golden corn, a strip of red pepper and a slice of lime on the rim.',
 'it is NEVER pale, never ivory, never creamy white and never peach-orange: that is the leche de tigre, which is a different dish. The dark colour is not a shadow and not a trick of the light, it is the drink itself.',
 '["leche-de-pantera","pantera"]', 'canon');

-- +goose Down
DELETE FROM plato_tipico WHERE clave IN ('tiradito-a-la-bandera', 'leche-de-pantera');
UPDATE plato_tipico SET patrones = patrones || '["tiradito-a-la-bandera","tiradito-tricolor"]'::jsonb WHERE clave = 'tiradito';
UPDATE plato_tipico SET patrones = patrones || '["leche-de-pantera"]'::jsonb WHERE clave = 'leche-de-tigre';
