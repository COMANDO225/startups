"use client";

import { useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import { CatalogoEsqueleto } from "@/components/CatalogoEsqueleto";
import { alcance } from "@/lib/flujo";
import { useImportacion } from "@/lib/hooks";

/**
 * /i/{id} a secas manda al paso que toque.
 *
 * Y "el que toque" es el mas avanzado que esta carta permite, no el primero:
 * si hay futuro es porque hay un pasado, asi que volver aqui despues de leer la
 * carta no puede devolverte a subir hojas.
 */
export default function ElPasoQueToca() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const { data: importacion, isPending, isError } = useImportacion(id);

  useEffect(() => {
    if (isPending || isError) return;
    router.replace(`/i/${id}/${alcance(importacion)}`);
  }, [id, importacion, isPending, isError, router]);

  return <CatalogoEsqueleto />;
}
