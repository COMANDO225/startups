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
            // La carta no cambia sola salvo por los polls, que ya traen su
            // intervalo: nada de refetch al volver a la pestana.
            refetchOnWindowFocus: false,
            staleTime: 1000,
          },
        },
      }),
  );

  return <QueryClientProvider client={cliente}>{children}</QueryClientProvider>;
}
