import { getTranslations } from "next-intl/server";

/**
 * Placeholder landing page (AC8). The real home page lives in app/(app)/ and is
 * added by M0-T06, which removes this file because both map to "/".
 */
export default async function PlaceholderPage() {
  const t = await getTranslations("common");

  return (
    <main className="flex min-h-dvh flex-col items-center justify-center gap-3 px-4 text-center">
      <h1 className="text-2xl font-semibold tracking-tight">{t("appName")}</h1>
      <p className="text-muted-foreground max-w-md text-sm">{t("tagline")}</p>
    </main>
  );
}
