"use client";

import type { FormEvent } from "react";
import type { UserSettings } from "@/lib/api";

export type AccessProviderID =
  | "anthropic"
  | "openai"
  | "google"
  | "groq"
  | "deepseek"
  | "alibaba"
  | "mistral"
  | "minimax"
  | "xiaomi_mimo_token_plan"
  | "moonshot"
  | "xai"
  | "zai"
  | "fireworks"
  | "together"
  | "poe"
  | "siliconflow"
  | "openrouter"
  | "deepinfra"
  | "featherless"
  | "azure_speech"
  | "aivis"
  | "elevenlabs"
  | "fish";

export type AccessCardRuntime = {
  value: string;
  onChange: (value: string) => void;
  secondaryValue?: string;
  onSecondaryChange?: (value: string) => void;
  onSubmit: (event: FormEvent) => Promise<void>;
  onDelete: () => Promise<void>;
  saving: boolean;
  deleting: boolean;
};

export function createAccessCardRuntime(
  value: string,
  onChange: (value: string) => void,
  onSubmit: (event: FormEvent) => Promise<void>,
  onDelete: () => Promise<void>,
  saving: boolean,
  deleting: boolean,
  secondaryValue?: string,
  onSecondaryChange?: (value: string) => void,
): AccessCardRuntime {
  return {
    value,
    onChange,
    secondaryValue,
    onSecondaryChange,
    onSubmit,
    onDelete,
    saving,
    deleting,
  };
}

export type AccessCard = {
  id: AccessProviderID;
  title: string;
  description: string;
  configured: boolean;
  last4: string | null | undefined;
  secondaryValue?: string;
  onSecondaryChange?: (value: string) => void;
  secondaryLabel?: string;
  secondaryPlaceholder?: string;
  secondaryStatusText?: string | null;
  value: string;
  onChange: (value: string) => void;
  onSubmit: (event: FormEvent) => Promise<void>;
  onDelete: () => Promise<void>;
  placeholder: string;
  saving: boolean;
  deleting: boolean;
  notSet: string;
};

type AccessCardMetadata = {
  id: AccessProviderID;
  titleKey: string;
  descriptionKey: string;
  notSetKey: string;
  placeholder: string;
  secondaryLabelKey?: string;
  secondaryPlaceholder?: string;
  selectSecondaryStatus?: (settings: UserSettings) => string | null | undefined;
  selectStatus: (settings: UserSettings) => { configured: boolean; last4: string | null | undefined };
};

const ACCESS_CARD_METADATA: AccessCardMetadata[] = [
  {
    id: "anthropic",
    titleKey: "settings.anthropicTitle",
    descriptionKey: "settings.anthropicDescription",
    notSetKey: "settings.anthropicNotSet",
    placeholder: "sk-ant-...",
    selectStatus: (settings) => ({ configured: settings.has_anthropic_api_key, last4: settings.anthropic_api_key_last4 }),
  },
  {
    id: "openai",
    titleKey: "settings.openaiTitle",
    descriptionKey: "settings.openaiDescription",
    notSetKey: "settings.openaiNotSet",
    placeholder: "sk-...",
    selectStatus: (settings) => ({ configured: settings.has_openai_api_key, last4: settings.openai_api_key_last4 }),
  },
  {
    id: "google",
    titleKey: "settings.googleTitle",
    descriptionKey: "settings.googleDescription",
    notSetKey: "settings.googleNotSet",
    placeholder: "AIza...",
    selectStatus: (settings) => ({ configured: settings.has_google_api_key, last4: settings.google_api_key_last4 }),
  },
  {
    id: "groq",
    titleKey: "settings.groqTitle",
    descriptionKey: "settings.groqDescription",
    notSetKey: "settings.groqNotSet",
    placeholder: "gsk_...",
    selectStatus: (settings) => ({ configured: settings.has_groq_api_key, last4: settings.groq_api_key_last4 }),
  },
  {
    id: "deepseek",
    titleKey: "settings.deepseekTitle",
    descriptionKey: "settings.deepseekDescription",
    notSetKey: "settings.deepseekNotSet",
    placeholder: "sk-...",
    selectStatus: (settings) => ({ configured: settings.has_deepseek_api_key, last4: settings.deepseek_api_key_last4 }),
  },
  {
    id: "alibaba",
    titleKey: "settings.alibabaTitle",
    descriptionKey: "settings.alibabaDescription",
    notSetKey: "settings.alibabaNotSet",
    placeholder: "sk-...",
    selectStatus: (settings) => ({ configured: settings.has_alibaba_api_key, last4: settings.alibaba_api_key_last4 }),
  },
  {
    id: "minimax",
    titleKey: "settings.minimaxTitle",
    descriptionKey: "settings.minimaxDescription",
    notSetKey: "settings.minimaxNotSet",
    placeholder: "sk-...",
    selectStatus: (settings) => ({ configured: settings.has_minimax_api_key, last4: settings.minimax_api_key_last4 }),
  },
  {
    id: "xiaomi_mimo_token_plan",
    titleKey: "settings.xiaomiMimoTokenPlanTitle",
    descriptionKey: "settings.xiaomiMimoTokenPlanDescription",
    notSetKey: "settings.xiaomiMimoTokenPlanNotSet",
    placeholder: "mimo_...",
    selectStatus: (settings) => ({ configured: settings.has_xiaomi_mimo_token_plan_api_key, last4: settings.xiaomi_mimo_token_plan_api_key_last4 }),
  },
  {
    id: "mistral",
    titleKey: "settings.mistralTitle",
    descriptionKey: "settings.mistralDescription",
    notSetKey: "settings.mistralNotSet",
    placeholder: "sk-...",
    selectStatus: (settings) => ({ configured: settings.has_mistral_api_key, last4: settings.mistral_api_key_last4 }),
  },
  {
    id: "moonshot",
    titleKey: "settings.moonshotTitle",
    descriptionKey: "settings.moonshotDescription",
    notSetKey: "settings.moonshotNotSet",
    placeholder: "sk-...",
    selectStatus: (settings) => ({ configured: settings.has_moonshot_api_key, last4: settings.moonshot_api_key_last4 }),
  },
  {
    id: "xai",
    titleKey: "settings.xaiTitle",
    descriptionKey: "settings.xaiDescription",
    notSetKey: "settings.xaiNotSet",
    placeholder: "xai-...",
    selectStatus: (settings) => ({ configured: settings.has_xai_api_key, last4: settings.xai_api_key_last4 }),
  },
  {
    id: "zai",
    titleKey: "settings.zaiTitle",
    descriptionKey: "settings.zaiDescription",
    notSetKey: "settings.zaiNotSet",
    placeholder: "zai-...",
    selectStatus: (settings) => ({ configured: settings.has_zai_api_key, last4: settings.zai_api_key_last4 }),
  },
  {
    id: "fireworks",
    titleKey: "settings.fireworksTitle",
    descriptionKey: "settings.fireworksDescription",
    notSetKey: "settings.fireworksNotSet",
    placeholder: "fw_...",
    selectStatus: (settings) => ({ configured: settings.has_fireworks_api_key, last4: settings.fireworks_api_key_last4 }),
  },
  {
    id: "together",
    titleKey: "settings.togetherTitle",
    descriptionKey: "settings.togetherDescription",
    notSetKey: "settings.togetherNotSet",
    placeholder: "together-...",
    selectStatus: (settings) => ({ configured: settings.has_together_api_key, last4: settings.together_api_key_last4 }),
  },
  {
    id: "poe",
    titleKey: "settings.poeTitle",
    descriptionKey: "settings.poeDescription",
    notSetKey: "settings.poeNotSet",
    placeholder: "sk-...",
    selectStatus: (settings) => ({ configured: settings.has_poe_api_key, last4: settings.poe_api_key_last4 }),
  },
  {
    id: "siliconflow",
    titleKey: "settings.siliconflowTitle",
    descriptionKey: "settings.siliconflowDescription",
    notSetKey: "settings.siliconflowNotSet",
    placeholder: "sk-...",
    selectStatus: (settings) => ({ configured: settings.has_siliconflow_api_key, last4: settings.siliconflow_api_key_last4 }),
  },
  {
    id: "openrouter",
    titleKey: "settings.openrouterTitle",
    descriptionKey: "settings.openrouterDescription",
    notSetKey: "settings.openrouterNotSet",
    placeholder: "sk-or-v1-...",
    selectStatus: (settings) => ({ configured: settings.has_openrouter_api_key, last4: settings.openrouter_api_key_last4 }),
  },
  {
    id: "deepinfra",
    titleKey: "settings.deepinfraTitle",
    descriptionKey: "settings.deepinfraDescription",
    notSetKey: "settings.deepinfraNotSet",
    placeholder: "deepinfra_...",
    selectStatus: (settings) => ({ configured: Boolean(settings.has_deepinfra_api_key), last4: settings.deepinfra_api_key_last4 ?? null }),
  },
  {
    id: "featherless",
    titleKey: "settings.featherlessTitle",
    descriptionKey: "settings.featherlessDescription",
    notSetKey: "settings.featherlessNotSet",
    placeholder: "sk-...",
    selectStatus: (settings) => ({ configured: Boolean(settings.has_featherless_api_key), last4: settings.featherless_api_key_last4 ?? null }),
  },
  {
    id: "azure_speech",
    titleKey: "settings.azureSpeechTitle",
    descriptionKey: "settings.azureSpeechDescription",
    notSetKey: "settings.azureSpeechNotSet",
    placeholder: "speech-key-...",
    secondaryLabelKey: "settings.azureSpeechRegionLabel",
    secondaryPlaceholder: "japaneast",
    selectStatus: (settings) => ({
      configured: Boolean(settings.has_azure_speech_api_key && settings.azure_speech_region),
      last4: settings.azure_speech_api_key_last4 ?? null,
    }),
    selectSecondaryStatus: (settings) => settings.azure_speech_region ?? null,
  },
  {
    id: "aivis",
    titleKey: "settings.aivisTitle",
    descriptionKey: "settings.aivisDescription",
    notSetKey: "settings.aivisNotSet",
    placeholder: "sk-...",
    selectStatus: (settings) => ({ configured: settings.has_aivis_api_key, last4: settings.aivis_api_key_last4 }),
  },
  {
    id: "elevenlabs",
    titleKey: "settings.elevenlabsTitle",
    descriptionKey: "settings.elevenlabsDescription",
    notSetKey: "settings.elevenlabsNotSet",
    placeholder: "sk_...",
    selectStatus: (settings) => ({ configured: Boolean(settings.has_elevenlabs_api_key), last4: settings.elevenlabs_api_key_last4 ?? null }),
  },
  {
    id: "fish",
    titleKey: "settings.fishTitle",
    descriptionKey: "settings.fishDescription",
    notSetKey: "settings.fishNotSet",
    placeholder: "fish_...",
    selectStatus: (settings) => ({ configured: Boolean(settings.has_fish_api_key), last4: settings.fish_api_key_last4 ?? null }),
  },
];

export function buildAccessCards(
  settings: UserSettings | null | undefined,
  runtimeByID: Partial<Record<AccessProviderID, AccessCardRuntime>>,
  t: (key: string, fallback?: string) => string,
): AccessCard[] {
  if (!settings) return [];
  return ACCESS_CARD_METADATA.map((item) => {
    const runtime = runtimeByID[item.id];
    if (!runtime) {
      throw new Error(`missing access card runtime for ${item.id}`);
    }
    const status = item.selectStatus(settings);
    return {
      id: item.id,
      title: t(item.titleKey),
      description: t(item.descriptionKey),
      configured: status.configured,
      last4: status.last4,
      secondaryValue: runtime.secondaryValue,
      onSecondaryChange: runtime.onSecondaryChange,
      secondaryLabel: item.secondaryLabelKey ? t(item.secondaryLabelKey) : undefined,
      secondaryPlaceholder: item.secondaryPlaceholder,
      secondaryStatusText: item.selectSecondaryStatus?.(settings) ?? null,
      value: runtime.value,
      onChange: runtime.onChange,
      onSubmit: runtime.onSubmit,
      onDelete: runtime.onDelete,
      placeholder: item.placeholder,
      saving: runtime.saving,
      deleting: runtime.deleting,
      notSet: t(item.notSetKey),
    };
  });
}

export function resolveAccessCardSelection<T extends { id: string; configured: boolean }>(
  accessCards: T[],
  activeAccessProvider: string,
) {
  return {
    configuredProviderCount: accessCards.filter((card) => card.configured).length,
    activeAccessCard: accessCards.find((card) => card.id === activeAccessProvider) ?? accessCards[0],
  };
}
