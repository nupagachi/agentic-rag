"use client";

import { NextIntlClientProvider, type AbstractIntlMessages } from "next-intl";
import { ThemeProvider } from "next-themes";
import type { ReactNode } from "react";

import { TooltipProvider } from "@/components/ui/tooltip";
import type { Locale } from "@/i18n/config";

type AppProvidersProps = {
  locale: Locale;
  messages: AbstractIntlMessages;
  children: ReactNode;
};

/** Client-side providers shared by the whole app: i18n, theme (class based) and tooltips. */
export function AppProviders({ locale, messages, children }: AppProvidersProps) {
  return (
    <NextIntlClientProvider locale={locale} messages={messages}>
      <ThemeProvider attribute="class" defaultTheme="system" enableSystem disableTransitionOnChange>
        <TooltipProvider delayDuration={200}>{children}</TooltipProvider>
      </ThemeProvider>
    </NextIntlClientProvider>
  );
}
