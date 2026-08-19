import type { Metadata, Viewport } from "next";
import "./globals.css";
import { Proveedores } from "./proveedores";
import { Geist } from "next/font/google";

const geist = Geist({ subsets: ["latin"], variable: "--font-sans" });

export const metadata: Metadata = {
  title: "Tacu",
  description: "Sube la foto de tu carta y publica tu catalogo",
};

// El dueno revisa su carta desde el telefono: sin zoom forzado ni escalado raro.
export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  themeColor: "#ffffff",
};

// HeroUI v3 no necesita provider ni "use client": el unico proveedor es el de
// TanStack Query.
export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="es" className={`font-sans ${geist.variable}`}>
      <body className="min-h-dvh bg-background text-foreground antialiased">
        <Proveedores>{children}</Proveedores>
      </body>
    </html>
  );
}
