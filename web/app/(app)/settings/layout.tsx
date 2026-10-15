import { getTranslations } from "next-intl/server";

import { NavLink } from "@/components/layout/nav-link";

// TODO(M10-01): hide items the current user lacks permission for.
export default async function SettingsLayout({ children }: { children: React.ReactNode }) {
  const t = await getTranslations("settings");
  const items = [
    { href: "/settings/profile", label: t("profile") },
    { href: "/settings/members", label: t("members") },
    { href: "/settings/models", label: t("models") },
    { href: "/settings/quota", label: t("quota") },
    { href: "/settings/audit", label: t("audit") },
  ];

  return (
    <div className="flex flex-col gap-6 md:flex-row">
      <nav className="flex shrink-0 gap-1 overflow-x-auto md:w-52 md:flex-col">
        {items.map((item) => (
          <NavLink key={item.href} href={item.href}>
            {item.label}
          </NavLink>
        ))}
      </nav>
      <section className="min-w-0 flex-1">{children}</section>
    </div>
  );
}
