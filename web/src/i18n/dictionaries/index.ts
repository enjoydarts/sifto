import type { Dict, Locale } from "../types";
import { enDict } from "./en";
import { jaDict } from "./ja";

export const dict: Record<Locale, Dict> = {
  ja: jaDict,
  en: enDict,
};

