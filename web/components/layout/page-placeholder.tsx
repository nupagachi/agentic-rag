import { getTranslations } from "next-intl/server";

export async function PagePlaceholder({ title, task }: { title: string; task?: string }) {
  const t = await getTranslations("common");
  return (
    <div className="space-y-1">
      <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
      <p className="text-sm text-muted-foreground">
        {t("comingSoon")}
        {task && ` · ${task}`}
      </p>
    </div>
  );
}
