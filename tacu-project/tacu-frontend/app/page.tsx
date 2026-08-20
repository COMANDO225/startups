"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Input, Label, TextField } from "@heroui/react";
import { Lateral } from "@/components/Lateral";
import { SelectorDeTipos } from "@/components/SelectorDeTipos";
import { crearRestaurante } from "@/lib/api";
import { useMisRestaurantes } from "@/lib/hooks";
import { secciones as armarSecciones } from "@/lib/flujo";
import { Aviso } from "@/components/ui/Aviso";
import { Boton } from "@/components/ui/Boton";

/**
 * Paso 1 · Tus datos, y el principio de todo.
 *
 * Antes esta pantalla pedia el nombre Y las hojas de la carta en el mismo
 * envio, asi que no habia forma de guardar quien eres hasta haber subido la
 * carta, y el paso "Tus datos" solo existia despues, cuando ya no servia.
 *
 * Ahora aqui se CREA el restaurante: el backend devuelve un token que se guarda
 * en este navegador y hace de sesion —sin registro y sin contrasena— para todo
 * lo que venga despues. Las hojas se piden en el sub-paso siguiente.
 */
export default function Inicio() {
  const router = useRouter();
  const [nombre, setNombre] = useState("");
  const [tipos, setTipos] = useState<string[]>([]);
  const [aviso, setAviso] = useState<string | null>(null);
  const [creando, setCreando] = useState(false);

  const mios = useMisRestaurantes();

  // Sin restaurante todavia: secciones() ya lo contempla y devuelve el 2 y el 3
  // bloqueados, que es exactamente lo que hay que enseñar aqui.
  const secs = armarSecciones({ conFoto: 0, platos: 0 });

  async function crear(evento: React.FormEvent) {
    evento.preventDefault();
    if (creando) return;
    setAviso(null);
    setCreando(true);
    try {
      // crearRestaurante guarda el token ANTES de devolver: se navega despues,
      // nunca antes, porque el token viaja una sola vez.
      const creado = await crearRestaurante(nombre.trim(), tipos);
      router.push(`/i/${creado.id}`);
    } catch (error) {
      setAviso(
        error instanceof Error
          ? error.message
          : "No se pudo crear tu restaurante. Intenta otra vez.",
      );
      setCreando(false);
    }
  }

  return (
    <div className="flex min-h-svh flex-col lg:flex-row">
      <Lateral
        estado="nueva"
        nombre={nombre.trim() || "Tu restaurante"}
        seccion="restaurante"
        secciones={secs}
        sub="datos"
        // Nada donde ir: los otros dos pasos estan bloqueados y este es el que
        // se esta haciendo.
        onIr={() => {}}
      />

      <main className="anima-panel min-w-0 flex-1 px-[14px] pt-4 pb-24 lg:max-w-[1080px] lg:px-8 lg:pt-[26px] lg:pb-10">
        <form className="flex max-w-xl flex-col gap-7" onSubmit={crear}>
          <div>
            <h1 className="font-display text-xl font-semibold leading-[1.2] tracking-[-0.02em]">
              Tus datos
            </h1>
            <p className="mt-1.5 max-w-[58ch] text-[13.5px] leading-[1.5] text-[#8A867D]">
              Con el nombre armamos tu dirección web. El tipo de negocio decide
              cómo se emplatan tus fotos.
            </p>
          </div>

          <TextField fullWidth value={nombre} onChange={setNombre}>
            <Label>Nombre del restaurante</Label>
            <Input
              autoComplete="organization"
              placeholder="Pollería El Rincón"
            />
          </TextField>

          {/* Sin idImportacion: el restaurante todavia no existe, asi que la
              eleccion se guarda aqui y viaja con la creacion. */}
          <SelectorDeTipos valor={tipos} onCambio={setTipos} />

          {aviso && <Aviso tono="bloquea">{aviso}</Aviso>}

          {mios.length > 0 && (
            <div className="flex flex-col gap-2">
              <p className="text-[12px] font-medium text-muted">
                O sigue con uno que ya tienes
              </p>
              <ul className="flex flex-col gap-1.5">
                {mios.map((r) => (
                  <li key={r.id}>
                    <Link
                      className="flex items-center justify-between gap-3 rounded-xl border border-borde-campo bg-surface px-3.5 py-3 text-[13.5px] font-medium transition-colors hover:border-tinta"
                      href={`/i/${r.id}`}
                    >
                      <span className="min-w-0 truncate">{r.nombre}</span>
                      <span className="shrink-0 text-tenue">→</span>
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
          )}

          <div>
            <Boton
              disabled={nombre.trim().length === 0 || creando}
              tamano="lg"
              variante="amarillo"
              type="submit"
            >
              {creando ? "Creando..." : "Continuar"}
            </Boton>
            <p className="mt-2 text-xs text-muted">
              Sin registro ni contraseña. Tu carta queda guardada en este
              navegador.
            </p>
          </div>
        </form>
      </main>
    </div>
  );
}
