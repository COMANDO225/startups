-- +goose Up

-- Segunda ronda de criticas mirando las fotos, y todas son del mismo tipo: el
-- plato estaba bien y le faltaba lo que lo hace RECONOCIBLE.
--
-- Una leche de tigre en un vaso recto es un vaso con algo blanco dentro. La de
-- verdad va en COPA y lleva chicharron encima, chifle asomando, yuyo y canchita:
-- sin eso no es el plato, es un batido. Lo mismo el arroz con mariscos sin su
-- conchita de abanico, o la infusion en vaso de vidrio cuando en un restaurante
-- sale en taza.
--
-- La regla que sale de esta tanda: donde el plato tiene una senal que lo
-- identifica de lejos, esa senal se escribe. Es lo que separa "correcto" de
-- "apetece".

INSERT INTO plato_tipico
    (clave, nombre, cocina, curso, aspecto, recipiente, guarnicion, jamas, patrones, origen)
VALUES

-- Le faltaba color y le faltaba la concha de abanico, que es la firma del plato.
('arroz-con-mariscos', 'Arroz con Mariscos', 'cevicheria', 'fondo',
 'moist rice stained a deep warm orange-red all the way through by chilli and shellfish stock, the colour even and strong right out to the edges, never white or pale anywhere. Mixed through the whole of it and clearly visible: mussels, white squid rings, prawns, pieces of fish, green peas and small squares of red pepper, with chopped green coriander. A generous handful of shredded white cheese is scattered over the top and a little of it is melting into the hot rice.',
 'a flat white plate with the rice mounded high in the middle.',
 'on top of the rice, one or two scallops still on their fan-shaped ridged half shells, the orange coral attached, plus a whole prawn and a mussel in its shell.',
 'the rice is never plain white, never pale at the edges and never dry: it is deep orange and moist throughout, and the seafood is inside the rice and not only sitting on top. It is not a soup and not a paella with a hard crust.',
 '["arroz-con-mariscos","arroz-c-mariscos","arroz-c-marisco","arroz-con-marisco"]', 'canon'),

-- La jalea es un CERRO. Salia un montoncito plano y ordenado.
('jalea', 'Jalea', 'cevicheria', 'fondo',
 'a tall mountain of deep golden brown deep-fried seafood piled high and loose, every piece thickly coated in a rough craggy blistered batter that is visibly crunchy: chunks of white fish, whole rings of squid, tentacle crowns and prawns, all different shapes and sizes heaped one on top of another so the pile looks abundant and stands well above the rim of the plate.',
 'a large flat white plate, the fried pieces heaped into a high mound in the middle.',
 'crowning the mound, a big generous nest of salsa criolla: thin slivers of purple-red onion with strips of red tomato, chopped coriander and rings of red chilli, wet and shining with lime. Around the base of the mound, all on the same plate: thick golden sticks of fried cassava, a scattering of curled fried plantain chips, a spoonful of toasted golden corn, a wedge of lime and two small dishes of sauce, one creamy white and one bright red.',
 'the pieces are never pale, never soggy and never sitting in sauce: the batter stays dry and crisp. The pile is never small, never flat and never neatly arranged in rows.',
 '["jalea","jalea-mixta","jalea-de-pescado"]', 'canon'),

-- La que mas cambia: sin la copa y sin lo que lleva encima no se reconoce.
('leche-de-tigre', 'Leche de Tigre', 'cevicheria', 'entrada',
 'a thick creamy opaque marinade, pale ivory or warm peach-orange, filling the glass right up, with small pieces of white fish, squid and prawn visible suspended inside it and specks of red onion, red pepper and green coriander through it. It is dense and rich, not watery.',
 'a wide stemmed glass goblet on a short foot —a footed coupe, the glass a Peruvian cevicheria always serves this in— standing on a small white plate.',
 'the top of the glass is loaded, and that is what makes the dish: several crisp golden pieces of fried squid and fish sitting on the rim and standing up out of it, one or two curled yellow plantain chips stuck in upright like fins, a little tangle of dark green seaweed, a scatter of toasted golden corn, a strip of red pepper and a slice of lime on the rim.',
 'never a plain straight household tumbler and never a bare glass of liquid with nothing on it: it is always a stemmed goblet and it is always crowned with fried pieces. The liquid is never clear or watery.',
 '["leche-de-tigre","leche-de-pantera"]', 'canon'),

-- La gaseosa sin marca en el nombre: el modelo se inventaba una etiqueta.
-- Aca eso no es adivinar, es lo que vende cualquier restaurante peruano.
('gaseosa', 'Gaseosa', '', 'bebida',
 'chilled bottles of soft drink standing upright, sealed, unopened and beaded with fresh condensation. The bottle is exactly the size named in the dish: a large plastic screw-cap bottle for the litre sizes, a small glass bottle only when the name says personal. When the dish names a brand, there is one single bottle and it is that brand. When the dish names NO brand at all, there are two bottles, the pair every Peruvian restaurant sells: an Inca Kola in front, its drink bright golden yellow, and a Coca-Cola just behind and to one side, its drink dark brown.',
 'the sealed bottles standing on their own on the surface.',
 'nothing else at all beside them.',
 'the label is always a real brand that exists, never an invented or unknown one. There is no food, no plate, no cutlery and no garnish anywhere in the picture.',
 '["gaseosa","inca-kola","coca-cola","pepsi","sprite","fanta","gaseosas"]', 'canon'),

-- En un restaurante la infusion sale en taza, no en vaso de vidrio.
('infusion', 'Infusión', '', 'bebida',
 'a cup of hot herbal infusion, the liquid pale gold or greenish, with the herb sprig or the tea bag inside it and a wisp of steam rising.',
 'a plain white ceramic cup with a handle, standing on its matching white saucer, the same kind of cup coffee is served in.',
 'a teaspoon resting on the saucer beside the cup.',
 'never a clear glass cup and never a glass tumbler: it is opaque white ceramic. This is a drink: no plate of food anywhere in the picture.',
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
-- Reescribe descripciones existentes; volver atras seria copiar aqui el texto
-- de la 00008 y la 00010. Se deshace escribiendo otra migracion encima.
SELECT 1;
