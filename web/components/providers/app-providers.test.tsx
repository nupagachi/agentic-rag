import { render, screen } from "@testing-library/react";
import { useTranslations } from "next-intl";
import { describe, expect, it } from "vitest";

import en from "@/messages/en/common.json";
import vi from "@/messages/vi/common.json";

import { AppProviders } from "./app-providers";

function Probe() {
  const t = useTranslations("common");
  return (
    <div>
      <h1>{t("appName")}</h1>
      <p>{t("tagline")}</p>
    </div>
  );
}

describe("AppProviders", () => {
  it("renders vi messages for locale vi", () => {
    render(
      <AppProviders locale="vi" messages={{ common: vi }}>
        <Probe />
      </AppProviders>,
    );
    expect(screen.getByRole("heading", { name: vi.appName })).toBeInTheDocument();
    expect(screen.getByText(vi.tagline)).toBeInTheDocument();
  });

  it("renders en messages for locale en", () => {
    render(
      <AppProviders locale="en" messages={{ common: en }}>
        <Probe />
      </AppProviders>,
    );
    expect(screen.getByText(en.tagline)).toBeInTheDocument();
    expect(screen.queryByText(vi.tagline)).not.toBeInTheDocument();
  });
});
