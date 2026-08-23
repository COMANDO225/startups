-- +goose Up

-- La pizarra de jugos de una jugueria peruana tiene dos bebidas con nombre
-- propio, y el banco no sabia ninguna de las dos.
--
-- Se vieron sobre la carta de una cevicheria real: "Surtido" salio como un vaso
-- lleno de MARISCOS —la variante de "surtido" asumia mar sin mirar que era una
-- bebida— y al arreglar eso salio beige con medio maracuya al lado, que es casi
-- exactamente el OTRO jugo de la misma pizarra.
--
-- Lo que las separa a simple vista, y por eso hay dos filas y no una:
--   surtido  -> ROSA, y el color es de la betarraga rallada que lleva
--   especial -> CREMA, y el color es de la leche, el huevo y la algarrobina
--
-- El maracuya venia de la guarnicion del jugo generico: "one small piece of THE
-- fruit it is made from". En un jugo mezclado no hay "la" fruta, asi que el
-- generador elegia una al azar. Por eso las dos filas traen su propia
-- guarnicion, que es lo unico que puede pisar aquella.
--
-- Los patrones NO llevan "surtido" ni "especial" a secas: emparejar es por
-- subcadena, y "Ceviche Surtido" o "Combo Especial" se los llevarian. Un nombre
-- suelto lo resuelve la tarea de conocer, que si ve la seccion de la carta.

INSERT INTO plato_tipico
    (clave, nombre, cocina, curso, aspecto, recipiente, guarnicion, jamas, patrones, origen)
VALUES

('jugo-surtido', 'Jugo Surtido', '', 'bebida',
 'a tall glass filled almost to the rim with a thick, completely opaque blended fruit drink of a warm pinkish rose-red, a soft magenta-salmon colour: papaya, pineapple, banana and strawberry blended together with a little grated beetroot, and it is the beetroot that gives the whole glass that pink. A fine pale froth sits on the surface. The colour is even all the way down, with no layers and no separation.',
 'a tall plain clear drinking glass standing upright on the surface.',
 'nothing beside the glass: it is a blend of several fruits, so no single fruit, wedge, slice or half fruit accompanies it.',
 'it is never yellow, never orange and never beige — those are other juices. It is not watery or translucent, it is not a layered smoothie, and there is no fruit resting next to the glass. There is no food and no plate anywhere in the picture.',
 '["jugo-surtido","surtido-de-frutas","jugo-de-frutas-surtido"]', 'canon'),

('jugo-especial', 'Jugo Especial', '', 'bebida',
 'a tall glass filled with a very thick, heavy, completely opaque cream-coloured drink: a pale warm beige, milky, so dense it barely moves. Papaya and banana blended with milk, a whole egg and dark algarrobina syrup. A thick foam collar sits on top, and a dark brown thread of algarrobina is drizzled over the foam.',
 'a tall plain clear drinking glass standing upright on the surface.',
 'nothing beside the glass: this one is a milkshake and it stands on its own.',
 'it is never pink, never red and never bright yellow: that is the surtido, which is another drink. It is not thin, not translucent and not watery. There is no food and no plate anywhere in the picture.',
 '["jugo-especial","especial-de-la-casa-jugo"]', 'canon');

-- +goose Down
DELETE FROM plato_tipico WHERE clave IN ('jugo-surtido', 'jugo-especial');
