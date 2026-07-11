import type {Metadata, Viewport} from "next";
import {Geist, Geist_Mono} from "next/font/google";
import {AuthProvider} from "@/lib/context/AuthContext";
import {QueryProvider} from "@/lib/providers/QueryProvider";
import {RealtimeProvider} from "@/lib/providers/RealtimeProvider";
import {Toaster} from "sonner";
import "./globals.css";
import React from "react";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
};

export const metadata: Metadata = {
  metadataBase: new URL('https://dfe.aoctech.app'),
  
  title: {
    default: 'CTech DF-e',
    template: '%s | CTech DF-e',
  },
  
  description:
    'Plataforma SaaS para emissão de NF-e, NFC-e, CT-e e MDF-e integrada à SEFAZ.',
  
  keywords: [
    'NF-e',
    'NFC-e',
    'CT-e',
    'MDF-e',
    'DF-e',
    'SEFAZ',
    'documentos fiscais eletrônicos',
    'emissão fiscal',
    'API fiscal',
  ],
  
  authors: [
    {
      name: 'Artur O Carvalho',
    },
  ],
  
  openGraph: {
    title: 'CTech DF-e',
    description:
      'Plataforma SaaS para emissão de documentos fiscais eletrônicos.',
    url: 'https://dfe.aoctech.app',
    siteName: 'CTech DF-e',
    locale: 'pt_BR',
    type: 'website',
    images: ["/og-image.png"],
  },
  
  twitter: {
    card: 'summary_large_image',
    title: 'CTech DF-e',
    description:
      'Plataforma SaaS para emissão de documentos fiscais eletrônicos.',
    images: ["/og-image.png"],
  },
  
  robots: {
    index: false,
    follow: false,
  },
  
  manifest: '/site.webmanifest',
}

export default function RootLayout({
                                     children,
                                   }: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang="pt-BR"
      suppressHydrationWarning
      className={`${geistSans.variable} ${geistMono.variable} h-full antialiased`}
    >
    <body className="min-h-full bg-gray-50">
    <QueryProvider>
      <AuthProvider>
        <RealtimeProvider>{children}</RealtimeProvider>
      </AuthProvider>
    </QueryProvider>
    <Toaster richColors position="top-right"/>
    </body>
    </html>
  );
}
