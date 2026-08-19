-- +goose Up

-- El a lo macho se quedo con el arroz solo.
--
-- Al separarlo del pescado frito en la 00010 se le escribio la guarnicion desde
-- cero, y ahi se perdio lo que el pescado frito SI tenia: la yuca y la salsa
-- criolla. En una cevicheria van los tres, y en la foto se notaba: plato
-- correcto, pero medio vacio al lado.
--
-- Van "fuera del charco" a proposito: la yuca frita dentro de la salsa se
-- reblandece y la criolla se deshace, asi que si no se dice donde estan, el
-- modelo las ahoga en la salsa y desaparecen.

UPDATE plato_tipico SET
    guarnicion = 'on the same plate beside the fish, all three kept clear of the sauce so they stay dry: a neatly moulded round mound of plain white rice, a heap of thick golden fried cassava sticks, and a nest of salsa criolla on a crisp green lettuce leaf —thin slivers of purple-red onion with strips of red tomato, chopped coriander and a red chilli.',
    actualizado_at = now()
WHERE clave = 'pescado-a-lo-macho';

-- +goose Down
UPDATE plato_tipico SET
    guarnicion = 'a moulded round mound of plain white rice on the same plate at one side, kept clear of the sauce.',
    actualizado_at = now()
WHERE clave = 'pescado-a-lo-macho';
