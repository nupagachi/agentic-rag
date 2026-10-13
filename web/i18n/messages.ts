import type { AbstractIntlMessages } from "next-intl";

import { messageFeatures, type Locale } from "./config";

type FeatureModule = { default: AbstractIntlMessages };

const loaders: Record<Locale, Record<(typeof messageFeatures)[number], () => Promise<FeatureModule>>> = {
  vi: {
    common: () => import("../messages/vi/common.json"),
  },
  en: {
    common: () => import("../messages/en/common.json"),
  },
};

/** Merge every feature file of a locale into one messages object (top-level key = feature name). */
export async function loadMessages(locale: Locale): Promise<AbstractIntlMessages> {
  const entries = await Promise.all(
    messageFeatures.map(async (feature) => {
      const mod = await loaders[locale][feature]();
      return [feature, mod.default] as const;
    }),
  );
  return Object.fromEntries(entries);
}
