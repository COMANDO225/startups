// Vuelve a tomar las capturas de Tacu en local.
//
//   cd tacu-project/tacu-backend && make arriba && make dev     # api en :8080
//   cd tacu-project/tacu-frontend && npm run dev                # front en :3000
//   chromium --headless=new --remote-debugging-port=9222 \
//     --user-data-dir=/tmp/perfil-capturas --window-size=1440,900 \
//     --hide-scrollbars --no-first-run --disable-gpu about:blank &
//   node capturas/capturar.mjs
//
// Sin playwright ni puppeteer: Node 24 ya trae WebSocket, y CDP es JSON por un
// socket. La dependencia es chromium, que ya esta instalado.
import { writeFileSync } from "node:fs";
import { dirname } from "node:path";
import { fileURLToPath } from "node:url";

const PUERTO_CDP = process.env.PUERTO_CDP ?? "9222";
const BASE = process.env.TACU_FRONT ?? "http://localhost:3000";
const DESTINO = dirname(fileURLToPath(import.meta.url));

// La importacion de desarrollo y su token. El token en claro no se recupera de la
// base —solo vive su SHA-256—, asi que se le pone uno conocido con el UPDATE que
// esta en el README de esta carpeta.
const IMPORTACION = process.env.TACU_IMPORTACION ?? "01a02bb1-41da-794a-a7bc-387acb4acc4e";
const TOKEN = process.env.TACU_TOKEN ?? "capturas-dev-2026";
const NOMBRE = process.env.TACU_NOMBRE ?? "el riconcito cevichero";

// "/" redirige al editor si el navegador recuerda una carta, asi que la portada
// se toma con localStorage vacio y el resto con el token puesto.
const PASOS = ["datos", "carta", "revisar", "fotos", "publicar"];

const lista = await (await fetch(`http://127.0.0.1:${PUERTO_CDP}/json/list`)).json();
const pagina = lista.find((t) => t.type === "page");
if (!pagina) throw new Error("chromium no expuso ninguna pestana: ¿esta corriendo con --remote-debugging-port?");

const ws = new WebSocket(pagina.webSocketDebuggerUrl);
await new Promise((ok, mal) => ((ws.onopen = ok), (ws.onerror = mal)));

let contador = 0;
const pendientes = new Map();
const oyentes = new Map();

ws.onmessage = (ev) => {
  const m = JSON.parse(ev.data);
  if (m.id && pendientes.has(m.id)) {
    const { ok, mal } = pendientes.get(m.id);
    pendientes.delete(m.id);
    m.error ? mal(new Error(m.error.message)) : ok(m.result);
  } else if (m.method && oyentes.has(m.method)) {
    oyentes.get(m.method)();
    oyentes.delete(m.method);
  }
};

const cdp = (method, params = {}) =>
  new Promise((ok, mal) => {
    const id = ++contador;
    pendientes.set(id, { ok, mal });
    ws.send(JSON.stringify({ id, method, params }));
  });

// El timeout no es una red de seguridad: una navegacion del router de Next que no
// recarga el documento no dispara loadEventFired nunca.
const esperarCarga = (ms = 20000) =>
  new Promise((ok) => (oyentes.set("Page.loadEventFired", ok), setTimeout(ok, ms)));

const dormir = (ms) => new Promise((ok) => setTimeout(ok, ms));

async function capturar(nombre) {
  // Las pantallas piden datos al backend despues de montar; sin esta espera se
  // captura el esqueleto en vez del contenido.
  await dormir(4000);
  const { cssContentSize } = await cdp("Page.getLayoutMetrics");
  const { data } = await cdp("Page.captureScreenshot", {
    format: "png",
    captureBeyondViewport: true,
    clip: {
      x: 0,
      y: 0,
      width: Math.ceil(cssContentSize.width),
      // La pantalla de fotos son 74 platos y se va a ~8500 px. El tope evita una
      // captura de decenas de MB si algun dia son 500.
      height: Math.min(Math.ceil(cssContentSize.height), 9000),
      scale: 1,
    },
  });
  writeFileSync(`${DESTINO}/${nombre}.png`, Buffer.from(data, "base64"));
  console.log(`${nombre}.png  ${Math.ceil(cssContentSize.width)}x${Math.ceil(cssContentSize.height)}`);
}

await cdp("Page.enable");
await cdp("Runtime.enable");

await cdp("Page.navigate", { url: BASE });
await esperarCarga();
await cdp("Runtime.evaluate", { expression: "localStorage.clear(); 'ok'" });
await cdp("Page.navigate", { url: BASE });
await esperarCarga();
await capturar("00-inicio");

await cdp("Runtime.evaluate", {
  expression: `
    localStorage.setItem('tacu.token.${IMPORTACION}', ${JSON.stringify(TOKEN)});
    localStorage.setItem('tacu.restaurantes', JSON.stringify([{id: '${IMPORTACION}', nombre: ${JSON.stringify(NOMBRE)}}]));
    'ok'`,
});

for (const [i, paso] of PASOS.entries()) {
  await cdp("Page.navigate", { url: `${BASE}/i/${IMPORTACION}/${paso}` });
  await esperarCarga();
  await capturar(`0${i + 2}-${paso}`);
}

ws.close();
