"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";

export function Proveedores({ children }: { children: React.ReactNode }) {
  // useState y no un modulo: un cliente por pestana, no uno compartido entre
  // peticiones del servidor.
  const [cliente] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            // Volver a la pestana REFRESCA. El comentario de antes decia que la
            // carta no cambia sola y por eso no hacia falta; dejo de ser verdad
            // cuando las URLs de las imagenes privadas pasaron a caducar. Una
            // pestana dejada abierta durante el almuerzo vuelve con enlaces
            // muertos, y lo que se ve son imagenes rotas.
            refetchOnWindowFocus: true,

            // Cinco minutos: holgadamente por debajo de la hora que vive una
            // firma, y suficiente para que alternar de ventana cada rato no
            // dispare una peticion cada vez.
            staleTime: 5 * 60 * 1000,
          },
        },
      }),
  );

  return <QueryClientProvider client={cliente}>{children}</QueryClientProvider>;
}
