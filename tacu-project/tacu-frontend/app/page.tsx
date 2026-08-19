"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Alert, Button, Input, Label, TextField } from "@heroui/react";
import { Lateral } from "@/components/Lateral";
import { SelectorDeTipos } from "@/components/SelectorDeTipos";
import { crearRestaurante } from "@/lib/api";
import { secciones as armarSecciones } from "@/lib/flujo";

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
    <div className="flex min-h-svh flex-col md:flex-row">
      <Lateral
        nombre={nombre.trim() || "Tu restaurante"}
        seccion="restaurante"
        secciones={secs}
        sub="datos"
        // Nada donde ir: los otros dos pasos estan bloqueados y este es el que
        // se esta haciendo. El estilo se configura cuando ya hay platos.
        onEstilo={() => {}}
        onIr={() => {}}
      />

      <main className="min-w-0 flex-1 px-4 py-6 md:px-10 md:py-10">
        <form className="flex max-w-xl flex-col gap-7" onSubmit={crear}>
          <div>
            <h1 className="text-2xl font-semibold">Tus datos</h1>
            <p className="mt-1 text-muted">
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

          {aviso && (
            <Alert role="alert" status="danger">
              <Alert.Indicator />
              <Alert.Content>
                <Alert.Description>{aviso}</Alert.Description>
              </Alert.Content>
            </Alert>
          )}

          <div>
            <Button
              isDisabled={nombre.trim().length === 0 || creando}
              size="lg"
              type="submit"
            >
              {creando ? "Creando..." : "Continuar"}
            </Button>
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
