import type { Metadata } from "next";
import { Inter, JetBrains_Mono } from "next/font/google";
import { NextIntlClientProvider } from "next-intl";
import { getLocale } from "next-intl/server";

import { AppProviders } from "@/components/providers/app-providers";
import { env } from "@/config/env";

import "./globals.css";

// Both fonts ship a Vietnamese subset.
const sans = Inter({ variable: "--font-sans", subsets: ["latin", "vietnamese"] });
const mono = JetBrains_Mono({ variable: "--font-geist-mono", subsets: ["latin", "vietnamese"] });

export const metadata: Metadata = {
  title: { default: env.NEXT_PUBLIC_APP_NAME, template: `%s · ${env.NEXT_PUBLIC_APP_NAME}` },
};

export default async function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  const locale = await getLocale();

  return (
    <html lang={locale} suppressHydrationWarning>
      <body className={`${sans.variable} ${mono.variable} antialiased`}>
        <NextIntlClientProvider>
          <AppProviders>{children}</AppProviders>
        </NextIntlClientProvider>
      </body>
    </html>
  );
}
