-- +goose Up

-- EL CANON: los platos escritos a mano, con como se VEN.
--
-- La cosecha de la 00007 trajo 419 nombres y su curso, pero sus patrones son el
-- nombre entero ("ceviche-clasico"), y las cartas de verdad dicen "Ceviche de
-- Pescado". Medido contra La Tribuna del Sur: la cosecha sola empareja
-- 0 de 74 platos. Lo que empareja son los patrones COMERCIALES de aqui.
--
-- Los cuatro campos van en INGLES, como porcion.go y formato.go: es el idioma
-- del generador. Y el campo `jamas` es el que mas trabaja: describir lo que la
-- cosa NO es corrige al modelo cuando su estadistica tira para otro lado, que
-- es como el pollo a la brasa dejo de salir rostizado de supermercado.
--
-- ON CONFLICT porque varias de estas claves ya existen de la cosecha: esto las
-- asciende de "nombre suelto con su curso" a entrada descrita.

INSERT INTO plato_tipico
    (clave, nombre, cocina, curso, aspecto, recipiente, guarnicion, jamas, patrones, origen)
VALUES

('ceviche', 'Ceviche', 'cevicheria', 'fondo',
 'a generous mound of raw white fish cut into even bite-size cubes, opaque and firm at the edges, sitting in a shallow pool of cloudy pale lime marinade, with thin slivers of raw red onion piled on top and a scattering of chopped green coriander.',
 'a wide shallow white ceramic bowl, or a white plate with a deep rim, so the marinade pools at the bottom instead of running off.',
 'on the same plate, each element in its own separate place and never mixed into the fish: two thick round slices of cooked orange sweet potato, a spoonful of large white boiled corn kernels, a small heap of toasted golden corn, and a crisp green lettuce leaf under one edge.',
 'the fish is never cooked, never breaded, never fried and never warm. This is not a salad, not a stew and not a dry plate: the marinade is always visible.',
 '["ceviche","cebiche","ceviche-de-pescado","ceviche-mixto"]', 'canon'),

('tiradito', 'Tiradito', 'cevicheria', 'fondo',
 'thin wide slices of raw white fish, laid flat and slightly overlapping in a single fan across the plate, each slice coated in a bright coloured chilli sauce that pools lightly around them.',
 'a flat round white plate, wide and shallow, with the slices spread out across it rather than heaped.',
 'a few kernels of boiled white corn and one thin slice of cooked sweet potato set at the edge of the plate, small and discreet.',
 'the fish is never cut into cubes and never piled into a mound: it always lies flat in slices. No raw onion on top.',
 '["tiradito","tiradito-a-la-bandera","tiradito-tricolor"]', 'canon'),

('leche-de-tigre', 'Leche de Tigre', 'cevicheria', 'entrada',
 'a tall glass filled with the cloudy pale citrus marinade of ceviche, thick and opaque, with a few small pieces of white fish and seafood visible inside it and at the bottom.',
 'a straight-sided tall glass, the kind served at the counter, standing on the surface.',
 'a single piece of toasted golden corn or a small strip of fried squid resting on the rim of the glass.',
 'this is a drink served in a glass, never a plate of food. It is not a soup bowl and it is not clear like water.',
 '["leche-de-tigre","leche-de-pantera"]', 'canon'),

('jalea', 'Jalea', 'cevicheria', 'fondo',
 'a tall pile of deep golden battered and fried pieces of fish and seafood, crisp and irregular, each piece visibly coated in a rough crunchy batter.',
 'a flat white plate, the fried pieces heaped in the middle of it.',
 'a generous nest of salsa criolla on top of the fried pieces: thin slivers of raw red onion with chopped coriander and lime. Slices of fried cassava and a wedge of lime rest at the side.',
 'the pieces are never pale, never soggy and never in sauce: the batter stays dry and crisp. Not a soup, not a stew.',
 '["jalea","jalea-mixta","jalea-de-pescado"]', 'canon'),

('chicharron-de-pescado', 'Chicharrón de Pescado', 'cevicheria', 'fondo',
 'bite-size chunks of white fish coated in a light golden crust and deep fried, piled loosely so the individual pieces are visible, crisp and dry on the outside.',
 'a flat white plate with the fried chunks heaped in the centre.',
 'a small nest of salsa criolla —raw red onion slivers with coriander— resting on top, a wedge of lime and two slices of fried cassava at the edge.',
 'never breaded flat like a fillet, never in sauce, never pale: they are separate crunchy chunks.',
 '["chicharron-de-pescado","chicharron-mixto","chicharron-de-calamar","chicharron-de-langostino","chicharron-de-camaron","chicharron-de-mariscos"]', 'canon'),

('arroz-con-mariscos', 'Arroz con Mariscos', 'cevicheria', 'fondo',
 'moist rice stained a warm orange-red by chilli and shellfish stock, mixed all the way through with mussels, squid rings, prawns and small pieces of fish, so the seafood is visible inside the rice and not only on top.',
 'a flat white plate with the rice mounded in the middle.',
 'one whole prawn and a mussel in its shell set on top of the rice, with a small strip of sweet red pepper.',
 'the rice is never plain white and never dry: it is coloured and moist throughout. It is not a soup and not a paella with a hard crust.',
 '["arroz-con-mariscos","arroz-c-mariscos","arroz-c-marisco","arroz-con-marisco"]', 'canon'),

('chaufa-de-mariscos', 'Chaufa de Mariscos', 'cevicheria', 'fondo',
 'chinese-style fried rice, the grains loose and separate and stained brown by soy sauce, tossed through with squid rings, prawns, pieces of fish, sliced spring onion and strips of omelette.',
 'a flat white plate with the rice piled in the middle.',
 'nothing else on the plate: the rice stands on its own.',
 'the rice is never wet, never orange and never in a pool of sauce: the grains stay loose and dry.',
 '["chaufa-de-mariscos","chaufa-de-pescado","arroz-chaufa-de-mariscos"]', 'canon'),

('pulpo-al-olivo', 'Pulpo al Olivo', 'cevicheria', 'entrada',
 'thin round slices of cooked purple-rimmed octopus laid flat and overlapping across the plate, entirely covered by a smooth pale purple-grey sauce of black olives.',
 'a flat round white plate, the slices spread wide and the sauce spooned over them.',
 'a few whole black olives and a thin slice of boiled potato at the edge of the plate.',
 'the octopus is never grilled, never charred and never served whole: it is sliced, cold and coated in the pale olive sauce.',
 '["pulpo-al-olivo"]', 'canon'),

('causa', 'Causa', 'cevicheria', 'entrada',
 'a firm layered cylinder of bright yellow mashed potato, smooth and dense, cut so the filling of shredded chicken or seafood in mayonnaise is visible as a pale layer through the middle.',
 'a flat white plate, the cylinder standing upright in the centre.',
 'a slice of hard-boiled egg, a black olive and a small line of mayonnaise beside the cylinder.',
 'never a loose mash and never a hot dish: it holds its shape, it is cold, and the yellow is the potato itself, not sauce.',
 '["causa","causa-limena","causa-acevichada","causa-rellena"]', 'canon'),

('choritos-a-la-chalaca', 'Choritos a la Chalaca', 'cevicheria', 'entrada',
 'mussels served on the half shell in a row, each shell holding one cooked mussel completely covered by a chopped topping of red onion, tomato, corn kernels and coriander.',
 'a flat white plate with the open shells arranged side by side on it.',
 'a wedge of lime at the edge of the plate.',
 'never in a bowl of broth and never closed: every shell is open, flat and topped.',
 '["choritos-a-la-chalaca","choros-a-la-chalaca"]', 'canon'),

('conchas-a-la-parmesana', 'Conchas a la Parmesana', 'cevicheria', 'entrada',
 'scallops served on the half shell in a row, each one covered with melted cheese browned and bubbling at the edges under the grill.',
 'a flat white plate with the shells arranged side by side, still hot.',
 'a wedge of lime at the edge of the plate.',
 'the cheese is melted and lightly browned, never raw and never a thick crust that hides the shell.',
 '["conchas-a-la-parmesana"]', 'canon'),

('tequenos', 'Tequeños', 'cevicheria', 'entrada',
 'crisp golden fried pastry fingers, long and thin and slightly blistered, stacked in a small pile, with the filling only visible at the open ends.',
 'a flat white plate with the fingers stacked criss-cross in the middle.',
 'a small bowl of pale green creamy avocado sauce beside the stack.',
 'never soft, never pale and never coated in sauce: the pastry stays dry and crunchy.',
 '["tequenos","tequenos-de-queso"]', 'canon'),

('pescado-frito', 'Pescado Frito', 'cevicheria', 'fondo',
 'a whole flat fish fried until the skin is crisp and deep golden, lying on its side with the head and tail intact and the skin visibly crackled.',
 'a long white plate, large enough that the whole fish fits on it.',
 'slices of fried cassava and a small heap of salsa criolla beside the fish.',
 'never filleted, never in sauce and never pale: it is a whole fish, dry and crisp.',
 '["pescado-frito","chita-frita","cabrilla-frita","pescado-a-lo-macho","chita-a-lo-macho"]', 'canon'),

('sudado', 'Sudado', 'cevicheria', 'fondo',
 'a thick piece of white fish sitting in a shallow bath of orange-red broth thickened with tomato and onion, the broth barely covering the fish and steaming.',
 'a wide deep bowl or a shallow clay pot, with the broth pooled around the fish.',
 'a small separate mound of plain white rice beside the bowl, on its own small plate.',
 'never dry, never fried and never breaded: the fish is stewed and always sits in its broth.',
 '["sudado","sudado-de-pescado","sudado-de-tramboyo"]', 'canon'),

('parihuela', 'Parihuela', 'cevicheria', 'sopa',
 'a deep red-orange seafood broth filling the bowl, with a whole prawn, mussels in their shells, squid and pieces of fish standing up out of the liquid.',
 'a wide deep white bowl, filled close to the rim.',
 'a wedge of lime resting on the rim of the bowl.',
 'never a clear or pale broth and never a dry plate: it is a thick coloured soup with the shellfish visible.',
 '["parihuela","chupe-de-camarones"]', 'canon'),

('ensalada-de-mariscos', 'Ensalada de Mariscos', 'cevicheria', 'entrada',
 'cold cooked seafood —squid rings, mussels, small prawns— tossed with thin slivers of red onion and chopped coriander, glistening with lime and oil.',
 'a wide shallow white bowl, the mixture heaped loosely in it.',
 'a lettuce leaf under one edge and a wedge of lime at the side.',
 'never warm, never breaded and never in a creamy sauce.',
 '["ensalada-de-mariscos"]', 'canon'),

-- --- transversal: lo que no es un plato de fondo ---
--
-- Estas son las que arreglan lo mas visible sin discusion: una gaseosa saliendo
-- con camote y choclo al lado es lo que motivo todo este trabajo.

('gaseosa', 'Gaseosa', '', 'bebida',
 'a chilled bottle of soft drink standing upright, sealed and beaded with condensation, its liquid dark or brightly coloured through the glass.',
 'the sealed bottle standing on the surface, or a tall clear glass filled with the soft drink and ice.',
 'nothing at all beside it.',
 'there is no food, no plate, no cutlery and no garnish anywhere in the picture.',
 '["gaseosa","inca-kola","coca-cola","pepsi","sprite","fanta"]', 'canon'),

('cerveza', 'Cerveza', '', 'bebida',
 'a cold beer: an amber glass bottle or a tall glass of golden beer with a thin white head of foam on top, the glass beaded with condensation.',
 'the bottle and glass standing upright on the surface.',
 'nothing at all beside it.',
 'there is no food, no plate and no garnish anywhere in the picture.',
 '["cerveza","pilsen","cusquena","cuzquena","cristal","pilsen-cristal"]', 'canon'),

('limonada', 'Limonada', '', 'bebida',
 'a tall glass of cloudy pale green limeade, cold and opaque, with ice and a faint froth at the top.',
 'a tall clear glass, or a large clear glass jug when it is served by the jug.',
 'a thin slice of lime on the rim of the glass.',
 'this is a drink: no plate and no food anywhere in the picture.',
 '["limonada","limonada-frozen"]', 'canon'),

('chicha-morada', 'Chicha Morada', '', 'bebida',
 'a tall glass of deep purple drink, dark and clear, cold, with small cubes of diced pineapple and apple settled at the bottom.',
 'a tall clear glass, or a large clear glass jug when it is served by the jug.',
 'a thin slice of lime on the rim of the glass.',
 'this is a drink: no plate and no food anywhere in the picture. The colour is deep purple, never red and never brown.',
 '["chicha-morada"]', 'canon'),

('chicha-de-jora', 'Chicha de Jora', '', 'bebida',
 'a tall glass of cloudy golden-ochre fermented corn drink, opaque and slightly foamy at the top.',
 'a tall glass, or a wide clay tumbler.',
 'nothing beside it.',
 'this is a drink: no plate and no food anywhere in the picture.',
 '["chicha-de-jora"]', 'canon'),

('jugo', 'Jugo', '', 'bebida',
 'a tall glass of freshly made fruit juice, thick and opaque, its colour coming from the fruit itself, with a light froth on the surface.',
 'a tall clear glass standing on the surface.',
 'one small piece of the fruit it is made from resting beside the glass.',
 'this is a drink: no plate and no cooked food anywhere in the picture.',
 '["jugo","jugos","jugo-surtido","papaya","pina","fresa","fresa-c-leche","fresa-con-leche","platanos-c-leche","platano-c-leche","platanos-con-leche","platano-con-leche"]', 'canon'),

('cafe', 'Café', '', 'bebida',
 'a small cup of black coffee, hot, with a thin pale crema on the surface.',
 'a small white ceramic cup standing on its matching saucer.',
 'a teaspoon resting on the saucer beside the cup.',
 'this is a drink: no plate of food anywhere in the picture.',
 '["cafe"]', 'canon'),

('infusion', 'Infusión', '', 'bebida',
 'a clear glass cup of hot herbal infusion, the liquid pale gold or green and translucent, with the herb or tea bag still inside it.',
 'a clear glass cup, or a small white cup, standing on a saucer.',
 'nothing beside it.',
 'this is a drink: no plate of food anywhere in the picture.',
 '["infusion","infusiones","anis","manzanilla","te"]', 'canon')

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
DELETE FROM plato_tipico WHERE origen = 'canon';
