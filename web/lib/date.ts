import moment from "moment";
import "moment/locale/vi";

import type { Locale } from "@/i18n/config";

/** Moment instances are mutable; always derive via these helpers. */
export function formatDateTime(value: moment.MomentInput, locale: Locale, format = "L LT") {
  return moment(value).locale(locale).format(format);
}

export function fromNow(value: moment.MomentInput, locale: Locale) {
  return moment(value).locale(locale).fromNow();
}
