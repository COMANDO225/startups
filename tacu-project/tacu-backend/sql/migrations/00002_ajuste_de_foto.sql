-- +goose Up

-- Lo que el dueno escribe para corregir la foto de UN plato suyo: "el ceviche va
-- con mas cancha", "nuestro chicharron lleva yuca frita, no papa".
--
-- ES UNA COLUMNA Y NO UNA TABLA, y esa decision es el punto entero de esta
-- migracion. Lo que se guarda aqui es SOLO la correccion del dueno; el
-- conocimiento compartido —que una fuente es una bandeja ovalada con tres veces
-- la comida, que un 1/4 es una pierna con su muslo— sigue viviendo en Go
-- (domain/formato.go, domain/porcion.go) y NO se copia por restaurante.
--
-- El comportamiento que se buscaba —default compartido para todos, override
-- dedicado para quien lo edite— lo da el resolver de app.PromptFoto, no el
-- almacenamiento: el default se aplica siempre y esta columna se le suma encima
-- cuando tiene algo. Guardar tambien el default por plato lo duplicaria 60 veces
-- por carta y congelaria cada mejora futura del prompt en las cartas ya
-- importadas, que es exactamente lo contrario de lo que se quiere.
--
-- El dia que un restaurante necesite su PROPIO default —una cadena con su estilo
-- de fotografia— eso es una tabla nueva colgada de restaurante, y el resolver ya
-- tiene el sitio donde consultarla. No obliga a mover nada de lo de hoy.
--
-- Vacio y no NULL, por lo mismo que las otras columnas de foto: 'no tiene
-- ajuste' es la cadena vacia, y cada columna nulable se vuelve un puntero en
-- todo el codigo que la toca.
ALTER TABLE plato ADD COLUMN foto_ajuste text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE plato DROP COLUMN foto_ajuste;
