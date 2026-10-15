import { BarChart3, Database, MessageSquare, Settings } from "lucide-react";
import { getTranslations } from "next-intl/server";

import { env } from "@/config/env";

import { NavLink } from "./nav-link";

export async function AppSidebar() {
  const t = await getTranslations("nav");
  const items = [
    { href: "/chat", label: t("chat"), icon: MessageSquare },
    { href: "/kb", label: t("knowledgeBases"), icon: Database },
    { href: "/admin/usage", label: t("usage"), icon: BarChart3 },
    { href: "/settings", label: t("settings"), icon: Settings },
  ];

  return (
    <aside className="hidden w-60 shrink-0 border-r bg-sidebar text-sidebar-foreground md:flex md:flex-col">
      <div className="px-4 py-4 text-base font-semibold">{env.NEXT_PUBLIC_APP_NAME}</div>
      <nav className="flex flex-col gap-1 px-2">
        {items.map(({ href, label, icon: Icon }) => (
          <NavLink key={href} href={href}>
            <Icon className="size-4" />
            {label}
          </NavLink>
        ))}
      </nav>
    </aside>
  );
}
