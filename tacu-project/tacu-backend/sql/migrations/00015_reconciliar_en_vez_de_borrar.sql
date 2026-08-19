-- +goose Up

-- Guardar una lectura dejaba de borrar la carta entera.
--
-- Hasta ahora GuardarCarta empezaba con DELETE FROM plato y reinsertaba todo con
-- ids nuevos. La foto cuelga del id del plato, asi que releer una carta de 74
-- platos tiraba $2.50 en fotos ya pagadas, mas cada precio que el dueno hubiera
-- corregido a mano. Por eso releer daba miedo y quitar una hoja no podia borrar
-- sus platos: cualquier cosa que tocara la lectura lo destruia todo.
--
-- Estas dos columnas son lo que hace falta para cruzar en vez de borrar.

-- De que hoja salio el plato. Vacia = no se sabe, que es el caso de las filas
-- anteriores a esto y el de una lectura que no lo dijo. Se usa para acotar que
-- se considera ausente al releer UNA hoja, y para poder ofrecer "quitar la hoja
-- y sus platos".
ALTER TABLE plato ADD COLUMN hoja_clave text NOT NULL DEFAULT '';

-- El plato que estaba y la ultima lectura ya no trajo. NO se borra: puede tener
-- una foto pagada detras y la lectura pudo equivocarse, asi que lo confirma el
-- dueno. Se limpia solo en cuanto una lectura vuelve a traerlo.
ALTER TABLE plato ADD COLUMN ausente boolean NOT NULL DEFAULT false;

-- La pantalla del catalogo separa los ausentes del resto, y son pocos: el indice
-- parcial solo indexa esos.
CREATE INDEX idx_plato_ausentes ON plato (importacion_id) WHERE ausente;

-- +goose Down
DROP INDEX idx_plato_ausentes;
ALTER TABLE plato DROP COLUMN ausente;
ALTER TABLE plato DROP COLUMN hoja_clave;
