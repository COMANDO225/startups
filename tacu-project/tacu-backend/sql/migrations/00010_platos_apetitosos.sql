-- +goose Up

-- LAS DESCRIPCIONES OTRA VEZ, MIRANDO LAS FOTOS QUE SALIERON.
--
-- El canon de la 00008 se escribio para que el plato fuera CORRECTO, y lo es:
-- de 23 fotos medidas, 23 salieron con el plato bueno en el recipiente bueno.
-- Pero salieron sosas. Una causa cumplia "cilindro amarillo con relleno" y era
-- un cilindro amarillo pelado, cuando una causa de verdad va cubierta de
-- mayonesa, aceituna, huevo y rocoto picado; el ceviche no llevaba ni un aro de
-- aji rojo; la ensalada de mariscos no daba ganas de comerla.
--
-- Y esa es la unica que importa: la foto es el escaparate del restaurante. Un
-- plato correcto que no apetece no vende.
--
-- Lo que cambia en cada entrada, y por que:
--
--  1. la guarnicion deja de ser "una aceituna al lado" y pasa a ser lo que el
--     plato lleva ENCIMA. Los modelos ponen al borde lo que se les nombra al
--     borde, y lo del borde se lee como adorno olvidado.
--  2. se nombra el COLOR de cada cosa. "Aji" no pinta nada; "aros rojos de
--     rocoto" si.
--  3. se dice el TAMANO del monton. Sin "generoso" y "colmado" sale la racion
--     minima, que es la mediana de las fotos de stock.
--
-- No se toca la 00008: una migracion aplicada no se edita. Esto pasa por encima
-- con el mismo ON CONFLICT.

INSERT INTO plato_tipico
    (clave, nombre, cocina, curso, aspecto, recipiente, guarnicion, jamas, patrones, origen)
VALUES

-- La causa no es el cilindro: es lo que lleva encima y las capas que se ven.
('causa', 'Causa', 'cevicheria', 'entrada',
 'a tall layered block of bright golden-yellow mashed potato, built in thick even layers that are all clearly visible from the side: a solid yellow potato base, then a generous creamy layer of shredded chicken or crab bound in mayonnaise, then a layer of green avocado slices, then another yellow potato layer closing it on top. The potato is dense and smooth and holds a clean edge, deep egg-yellow from the yellow chilli worked into it.',
 'a flat white plate, the block standing upright in the middle with its layered side facing the camera so every layer can be counted.',
 'the top of the block is decorated and that decoration is the dish: white mayonnaise piped over it in a lattice or in swirls, half a black olive, slices of hard-boiled egg with their yellow yolks showing, a scattering of finely diced red pepper and a little chopped green parsley. A crisp green lettuce leaf sits under one edge on the plate.',
 'never a bare plain cylinder of potato with nothing on it, and never a loose mash: the layers and the topping are always visible and always generous. It is cold, it holds its shape, and the yellow is the potato itself, not a sauce poured over it.',
 '["causa","causa-limena","causa-acevichada","causa-rellena"]', 'canon'),

-- Al ceviche le faltaba el aji. Un ceviche sin rocoto rojo no es un ceviche.
('ceviche', 'Ceviche', 'cevicheria', 'fondo',
 'a big generous mound of raw white fish cut into even bite-size cubes, opaque and firm at the edges, heaped high and sitting in a shallow pool of cloudy pale lime marinade. Over the mound: a thick nest of thin slivers of raw purple-red onion, plenty of chopped green coriander, and several bright red rings of fresh rocoto chilli, wet and shining. The whole thing glistens with the marinade.',
 'a wide shallow white ceramic bowl, or a white plate with a deep rim, so the marinade pools at the bottom instead of running off.',
 'on the same plate, each element in its own separate place and never mixed into the fish, all of them in generous quantity: two thick round slices of cooked orange sweet potato, a spoonful of large white boiled corn kernels, a small heap of toasted golden corn, and a crisp green lettuce leaf under one edge.',
 'the fish is never cooked, never breaded, never fried and never warm. This is not a salad, not a stew and not a dry plate: the marinade is always visible, and there is always red chilli on top.',
 '["ceviche","cebiche","ceviche-de-pescado","ceviche-mixto"]', 'canon'),

-- Mixto es variedad de FORMAS. Salia una fuente de trozos todos iguales.
('chicharron-de-pescado', 'Chicharrón de Pescado', 'cevicheria', 'fondo',
 'a big generous heap of freshly fried seafood, piled high and loose so it looks abundant, every piece a deep appetising golden brown with a rough, blistered, craggy crust that is visibly crunchy and dry. The pieces are irregular and clearly different from one another in size and shape.',
 'a flat white plate with the fried pieces heaped high in the centre.',
 'a generous nest of salsa criolla resting right on top of the heap: thin slivers of purple-red onion with strips of red tomato, chopped green coriander and rings of red chilli, wet with lime. Beside the heap, on the same plate, thick golden sticks of fried cassava, a wedge of lime, and a small dish of pale yellow chilli sauce.',
 'never breaded flat like a fillet, never sitting in sauce and never pale: they stay separate crunchy pieces. The salsa criolla goes on top, never forgotten at the edge.',
 '["chicharron-de-pescado","chicharron-mixto","chicharron-de-calamar","chicharron-de-langostino","chicharron-de-camaron","chicharron-de-mariscos"]', 'canon'),

-- Los choritos: colmados y en circulo, no una fila de conchas casi vacias.
('choritos-a-la-chalaca', 'Choritos a la Chalaca', 'cevicheria', 'entrada',
 'open mussel shells, short and rounded, dark blue-black outside and pearly inside, each one holding a cooked orange mussel completely buried under a heaped spoonful of chopped topping: diced red tomato, purple-red onion, big white corn kernels, green coriander and specks of red chilli, wet and shining with lime. The topping is piled well above the rim of every shell, colourful and abundant, so almost none of the mussel shows through.',
 'a round white plate with the filled shells laid out in a circle, arranged like the petals of a flower with all the hinges pointing to the centre.',
 'a wedge of lime and a small green lettuce leaf in the middle of the circle.',
 'never in a bowl of broth, never closed, and never a bare mussel with a thin sprinkle on it: every shell is open, flat and heaped. The shells are short and rounded, never long thin ones, and they are never laid out in a straight line.',
 '["choritos-a-la-chalaca","choros-a-la-chalaca"]', 'canon'),

-- La ensalada salia palida y triste: el marisco tiene que verse entero y grande.
('ensalada-de-mariscos', 'Ensalada de Mariscos', 'cevicheria', 'entrada',
 'a generous glistening heap of cold cooked seafood, every piece big and whole and easy to recognise one by one: plump pink-orange prawns with their tails on, white squid rings, orange mussels out of their shells and thick slices of octopus with a purple-red rim. All of it tossed with thin slivers of purple-red onion, diced red tomato and plenty of chopped green coriander, shining with lime and olive oil.',
 'a wide shallow white bowl, the mixture heaped loosely and high in it.',
 'a crisp green lettuce leaf under one edge and a wedge of lime at the side.',
 'never warm, never breaded and never in a creamy sauce. The seafood is never chopped small: the pieces stay whole and the colours stay bright.',
 '["ensalada-de-mariscos"]', 'canon'),

-- El pescado frito de una cevicheria viene con arroz. Salia el pescado solo.
('pescado-frito', 'Pescado Frito', 'cevicheria', 'fondo',
 'a whole fish fried until the skin is crisp and deep golden brown, lying on its side with the head and the tail intact, scored with two or three deep diagonal cuts along the body so the white flesh shows through the crackled skin. The skin is blistered and glossy, freshly out of the fryer.',
 'a large oval or long white plate, big enough that the whole fish fits with room for what goes beside it.',
 'on the same plate beside the fish, all three and all generous: a heap of thick golden fried cassava sticks, a nest of salsa criolla on a crisp lettuce leaf —thin slivers of purple-red onion with strips of red tomato, chopped coriander and a red chilli— and a neatly moulded round mound of plain white rice.',
 'never filleted, never in sauce and never pale: it is a whole fish, dry and crisp, with its head on.',
 '["pescado-frito","chita-frita","cabrilla-frita","pescado-a-la-plancha"]', 'canon'),

-- A lo macho sale del pescado frito: aquel dice "nunca en salsa" y este es
-- justamente el que va ahogado en salsa de mariscos. Compartiendo entrada, las
-- dos frases se contradicen dentro del mismo prompt.
('pescado-a-lo-macho', 'Pescado a lo Macho', 'cevicheria', 'fondo',
 'a fried fish completely smothered in a thick glossy orange-red seafood sauce poured over it, the sauce clinging to the fish and pooling around it. Sitting in the sauce on top: mussels in their shells, white squid rings, whole pink prawns and pieces of octopus, plenty of them and all clearly visible above the surface.',
 'a large white plate with a rim, deep enough for the sauce to pool without running off.',
 'a moulded round mound of plain white rice on the same plate at one side, kept clear of the sauce.',
 'the fish is never dry and never bare: the sauce covers it and the shellfish sit on top. It is not a soup and the sauce is not a thin broth.',
 '["pescado-a-lo-macho","chita-a-lo-macho","a-lo-macho","pescado-a-la-macho"]', 'canon'),

-- La gaseosa: el envase lo decide el TAMANO del nombre, no la estadistica del
-- modelo, que por su cuenta devuelve siempre la botellita de vidrio.
('gaseosa', 'Gaseosa', '', 'bebida',
 'a chilled bottle of soft drink standing upright, sealed, unopened and beaded with fresh condensation, the drink dark or brightly coloured inside it. The bottle is exactly the size named in the dish: a large plastic screw-cap bottle for the litre sizes, a small glass bottle only when the name says personal.',
 'the sealed bottle standing on its own on the surface.',
 'nothing at all beside it.',
 'there is no food, no plate, no cutlery and no garnish anywhere in the picture.',
 '["gaseosa","inca-kola","coca-cola","pepsi","sprite","fanta","gaseosas"]', 'canon')

ON CONFLICT (clave) DO UPDATE SET
    nombre     = EXCLUDED.nombre,
    cocina     = EXCLUDED.cocina,
    curso      = EXCLUDED.curso,
    aspecto    = EXCLUDED.aspecto,
    recipiente = EXCLUDED.recipiente,
    guarnicion = EXCLUDED.guarnicion,
    jamas      = EXCLUDED.jamas,
    patrones   = EXCLUDED.patrones,
    origen     = EXCLUDED.origen,
    actualizado_at = now();

-- +goose Down

-- No se puede volver atras entrada por entrada sin duplicar aqui el texto de la
-- 00008. Lo que si se revierte es lo que esta migracion CREO.
DELETE FROM plato_tipico WHERE clave = 'pescado-a-lo-macho';
