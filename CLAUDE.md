# startups

Monorepo de productos SaaS para el mercado peruano. Lee **[`CONTEXT.md`](CONTEXT.md)
primero**: ahí está lo decidido, lo aprendido y lo que falta. La memoria técnica de
cada proyecto está en su propio `CLAUDE.md`:

- [`facturacion-service/CLAUDE.md`](facturacion-service/CLAUDE.md) — glosario SUNAT,
  reglas de correctitud, bugs y por qué.
- [`tacu-project/CLAUDE.md`](tacu-project/CLAUDE.md) — convenciones de Tacu, dinero,
  modelos de IA, base de datos.

## Commits

**Ninguna atribución a Claude, ni como autor ni como coautor.** Nada de
`Co-Authored-By: Claude`, nada de "Generated with Claude Code". El único autor del
repo es Anderson. La historia se reescribió una vez para quitar ese trailer de los
46 commits; no hace falta una segunda.

Los mensajes sí son largos y narrativos a propósito: título en una línea que cuenta
qué cambió en voz de producto, y cuerpo que explica el bug encontrado, la medición
que decidió y la trampa que se evitó. Ese es el registro de por qué, y se mantiene.

## Capturas

`capturas/` guarda el estado visual de las pantallas que corren en local. Es para
saber cómo va la cosa de un vistazo, no es un test de regresión: se sobrescriben.
