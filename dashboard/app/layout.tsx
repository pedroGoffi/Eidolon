import type { Metadata } from "next";
import { Inter, JetBrains_Mono } from "next/font/google";
import "./globals.css";
import { Nav } from "@/components/Nav";

const inter = Inter({ subsets: ["latin"], variable: "--font-inter" });
const jbmono = JetBrains_Mono({ subsets: ["latin"], variable: "--font-jbmono" });

export const metadata: Metadata = {
  title: "Eidolon — Painel Administrativo",
  description: "Analytics, segurança e roteamento em tempo real.",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="pt-BR" className={`${inter.variable} ${jbmono.variable}`}>
      <body className="flex bg-eidolon-bg font-sans text-eidolon-text antialiased">
        <Nav />
        <main className="min-h-screen flex-1 overflow-y-auto">{children}</main>
      </body>
    </html>
  );
}
