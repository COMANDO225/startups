import type { Metadata, Viewport } from "next";
import "./globals.css";
import { Proveedores } from "./proveedores";
import { IBM_Plex_Sans, Space_Grotesk } from "next/font/google";

// Dos familias con un reparto estricto: IBM Plex Sans para todo lo textual y
// Space Grotesk SOLO para cifras, titulos y etiquetas tecnicas. Mezclarlas al
// azar deshace lo unico que hace que un precio se lea como un precio.
const cuerpo = IBM_Plex_Sans({
  subsets: ["latin"],
  weight: ["400", "500", "600"],
  variable: "--fuente-cuerpo",
});

const titulos = Space_Grotesk({
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
  variable: "--fuente-titulos",
});

export const metadata: Metadata = {
  title: "Tacu",
  description: "Sube la foto de tu carta y publica tu catalogo",
};

// El dueno revisa su carta desde el telefono: sin zoom forzado ni escalado raro.
export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  // El rail es oscuro y es lo que pinta la barra del navegador en el movil.
  themeColor: "#14120F",
  // Para que el contenido llegue hasta el borde y las franjas del sistema se
  // puedan leer con env(safe-area-inset-*). Sin esto, la barra de accion se
  // queda debajo del indicador de inicio del iPhone.
  viewportFit: "cover",
};

// HeroUI v3 no necesita provider ni "use client": el unico proveedor es el de
// TanStack Query.
export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html
      lang="es"
      className={`font-sans ${cuerpo.variable} ${titulos.variable}`}
    >
      <body className="min-h-dvh bg-background text-foreground antialiased">
        <Proveedores>{children}</Proveedores>
      </body>
    </html>
  );
}
