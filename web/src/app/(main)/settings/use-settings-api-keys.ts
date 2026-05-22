"use client";

import { type FormEvent, useCallback, useMemo, useState } from "react";

import { api, type UserSettings } from "@/lib/api";
import { createAPIKeyActionHandlers } from "@/components/settings/settings-api-key-actions";
import { buildApiKeyCardLabels } from "@/components/settings/settings-system-helpers";
import { buildAccessCards, createAccessCardRuntime } from "@/components/settings/system-access-cards";

type ToastTone = "success" | "error";
type ShowToast = (message: string, tone: ToastTone) => void;
type Translate = (key: string, fallback?: string) => string;
type Confirm = (options: { title: string; message: string; confirmLabel: string; tone: "danger" }) => Promise<boolean>;

type UseSettingsApiKeysArgs = {
  settings: UserSettings | null;
  reload: () => Promise<unknown>;
  confirm: Confirm;
  showToast: ShowToast;
  t: Translate;
  onResetXAIVoices: () => void;
  onMarkAivisUserDictionariesStale: () => void;
  onResetAivisUserDictionaries: () => void;
  onClearAivisUserDictionarySelection: () => void;
  onResetElevenLabsVoices: () => void;
};

export function useSettingsApiKeys({
  settings,
  reload,
  confirm,
  showToast,
  t,
  onResetXAIVoices,
  onMarkAivisUserDictionariesStale,
  onResetAivisUserDictionaries,
  onClearAivisUserDictionarySelection,
  onResetElevenLabsVoices,
}: UseSettingsApiKeysArgs) {
  const [savingAnthropicKey, setSavingAnthropicKey] = useState(false);
  const [deletingAnthropicKey, setDeletingAnthropicKey] = useState(false);
  const [savingOpenAIKey, setSavingOpenAIKey] = useState(false);
  const [deletingOpenAIKey, setDeletingOpenAIKey] = useState(false);
  const [savingGoogleKey, setSavingGoogleKey] = useState(false);
  const [deletingGoogleKey, setDeletingGoogleKey] = useState(false);
  const [savingGroqKey, setSavingGroqKey] = useState(false);
  const [deletingGroqKey, setDeletingGroqKey] = useState(false);
  const [savingDeepSeekKey, setSavingDeepSeekKey] = useState(false);
  const [deletingDeepSeekKey, setDeletingDeepSeekKey] = useState(false);
  const [savingAlibabaKey, setSavingAlibabaKey] = useState(false);
  const [deletingAlibabaKey, setDeletingAlibabaKey] = useState(false);
  const [savingMistralKey, setSavingMistralKey] = useState(false);
  const [deletingMistralKey, setDeletingMistralKey] = useState(false);
  const [savingCerebrasKey, setSavingCerebrasKey] = useState(false);
  const [deletingCerebrasKey, setDeletingCerebrasKey] = useState(false);
  const [savingMiniMaxKey, setSavingMiniMaxKey] = useState(false);
  const [deletingMiniMaxKey, setDeletingMiniMaxKey] = useState(false);
  const [savingXiaomiMiMoTokenPlanKey, setSavingXiaomiMiMoTokenPlanKey] = useState(false);
  const [deletingXiaomiMiMoTokenPlanKey, setDeletingXiaomiMiMoTokenPlanKey] = useState(false);
  const [savingMoonshotKey, setSavingMoonshotKey] = useState(false);
  const [deletingMoonshotKey, setDeletingMoonshotKey] = useState(false);
  const [savingXAIKey, setSavingXAIKey] = useState(false);
  const [deletingXAIKey, setDeletingXAIKey] = useState(false);
  const [savingZAIKey, setSavingZAIKey] = useState(false);
  const [deletingZAIKey, setDeletingZAIKey] = useState(false);
  const [savingFireworksKey, setSavingFireworksKey] = useState(false);
  const [deletingFireworksKey, setDeletingFireworksKey] = useState(false);
  const [savingTogetherKey, setSavingTogetherKey] = useState(false);
  const [deletingTogetherKey, setDeletingTogetherKey] = useState(false);
  const [savingPoeKey, setSavingPoeKey] = useState(false);
  const [deletingPoeKey, setDeletingPoeKey] = useState(false);
  const [savingSiliconFlowKey, setSavingSiliconFlowKey] = useState(false);
  const [deletingSiliconFlowKey, setDeletingSiliconFlowKey] = useState(false);
  const [savingAzureSpeechConfig, setSavingAzureSpeechConfig] = useState(false);
  const [deletingAzureSpeechConfig, setDeletingAzureSpeechConfig] = useState(false);
  const [savingOpenRouterKey, setSavingOpenRouterKey] = useState(false);
  const [deletingOpenRouterKey, setDeletingOpenRouterKey] = useState(false);
  const [savingDeepInfraKey, setSavingDeepInfraKey] = useState(false);
  const [deletingDeepInfraKey, setDeletingDeepInfraKey] = useState(false);
  const [savingFeatherlessKey, setSavingFeatherlessKey] = useState(false);
  const [deletingFeatherlessKey, setDeletingFeatherlessKey] = useState(false);
  const [savingAivisKey, setSavingAivisKey] = useState(false);
  const [deletingAivisKey, setDeletingAivisKey] = useState(false);
  const [savingElevenLabsKey, setSavingElevenLabsKey] = useState(false);
  const [deletingElevenLabsKey, setDeletingElevenLabsKey] = useState(false);
  const [savingCartesiaKey, setSavingCartesiaKey] = useState(false);
  const [deletingCartesiaKey, setDeletingCartesiaKey] = useState(false);
  const [savingFishKey, setSavingFishKey] = useState(false);
  const [deletingFishKey, setDeletingFishKey] = useState(false);

  const [anthropicApiKeyInput, setAnthropicApiKeyInput] = useState("");
  const [openAIApiKeyInput, setOpenAIApiKeyInput] = useState("");
  const [googleApiKeyInput, setGoogleApiKeyInput] = useState("");
  const [groqApiKeyInput, setGroqApiKeyInput] = useState("");
  const [deepseekApiKeyInput, setDeepseekApiKeyInput] = useState("");
  const [alibabaApiKeyInput, setAlibabaApiKeyInput] = useState("");
  const [mistralApiKeyInput, setMistralApiKeyInput] = useState("");
  const [cerebrasApiKeyInput, setCerebrasApiKeyInput] = useState("");
  const [miniMaxApiKeyInput, setMiniMaxApiKeyInput] = useState("");
  const [xiaomiMiMoTokenPlanApiKeyInput, setXiaomiMiMoTokenPlanApiKeyInput] = useState("");
  const [moonshotApiKeyInput, setMoonshotApiKeyInput] = useState("");
  const [xaiApiKeyInput, setXaiApiKeyInput] = useState("");
  const [zaiApiKeyInput, setZaiApiKeyInput] = useState("");
  const [fireworksApiKeyInput, setFireworksApiKeyInput] = useState("");
  const [togetherApiKeyInput, setTogetherApiKeyInput] = useState("");
  const [poeApiKeyInput, setPoeApiKeyInput] = useState("");
  const [siliconFlowApiKeyInput, setSiliconFlowApiKeyInput] = useState("");
  const [azureSpeechApiKeyInput, setAzureSpeechApiKeyInput] = useState("");
  const [azureSpeechRegionInput, setAzureSpeechRegionInput] = useState("");
  const [openRouterApiKeyInput, setOpenRouterApiKeyInput] = useState("");
  const [deepInfraApiKeyInput, setDeepInfraApiKeyInput] = useState("");
  const [featherlessApiKeyInput, setFeatherlessApiKeyInput] = useState("");
  const [aivisApiKeyInput, setAivisApiKeyInput] = useState("");
  const [elevenLabsApiKeyInput, setElevenLabsApiKeyInput] = useState("");
  const [cartesiaApiKeyInput, setCartesiaApiKeyInput] = useState("");
  const [fishApiKeyInput, setFishApiKeyInput] = useState("");

  const apiKeyHandlers = createAPIKeyActionHandlers({
    confirm,
    confirmLabel: t("settings.delete"),
    reload,
    showToast,
    definitions: {
      anthropic: {
        value: anthropicApiKeyInput, setValue: setAnthropicApiKeyInput, setSaving: setSavingAnthropicKey, setDeleting: setDeletingAnthropicKey,
        save: api.setAnthropicApiKey, remove: api.deleteAnthropicApiKey,
        deleteTitle: t("settings.anthropicDeleteTitle"), deleteMessage: t("settings.anthropicDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.anthropicSaved"), deleteSuccessMessage: t("settings.toast.anthropicDeleted"),
      },
      openai: {
        value: openAIApiKeyInput, setValue: setOpenAIApiKeyInput, setSaving: setSavingOpenAIKey, setDeleting: setDeletingOpenAIKey,
        save: api.setOpenAIApiKey, remove: api.deleteOpenAIApiKey,
        deleteTitle: t("settings.openaiDeleteTitle"), deleteMessage: t("settings.openaiDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.openaiSaved"), deleteSuccessMessage: t("settings.toast.openaiDeleted"),
      },
      google: {
        value: googleApiKeyInput, setValue: setGoogleApiKeyInput, setSaving: setSavingGoogleKey, setDeleting: setDeletingGoogleKey,
        save: api.setGoogleApiKey, remove: api.deleteGoogleApiKey,
        deleteTitle: t("settings.googleDeleteTitle"), deleteMessage: t("settings.googleDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.googleSaved"), deleteSuccessMessage: t("settings.toast.googleDeleted"),
      },
      groq: {
        value: groqApiKeyInput, setValue: setGroqApiKeyInput, setSaving: setSavingGroqKey, setDeleting: setDeletingGroqKey,
        save: api.setGroqApiKey, remove: api.deleteGroqApiKey,
        deleteTitle: t("settings.groqDeleteTitle"), deleteMessage: t("settings.groqDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.groqSaved"), deleteSuccessMessage: t("settings.toast.groqDeleted"),
      },
      deepseek: {
        value: deepseekApiKeyInput, setValue: setDeepseekApiKeyInput, setSaving: setSavingDeepSeekKey, setDeleting: setDeletingDeepSeekKey,
        save: api.setDeepSeekApiKey, remove: api.deleteDeepSeekApiKey,
        deleteTitle: t("settings.deepseekDeleteTitle"), deleteMessage: t("settings.deepseekDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.deepseekSaved"), deleteSuccessMessage: t("settings.toast.deepseekDeleted"),
      },
      alibaba: {
        value: alibabaApiKeyInput, setValue: setAlibabaApiKeyInput, setSaving: setSavingAlibabaKey, setDeleting: setDeletingAlibabaKey,
        save: api.setAlibabaApiKey, remove: api.deleteAlibabaApiKey,
        deleteTitle: t("settings.alibabaDeleteTitle"), deleteMessage: t("settings.alibabaDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.alibabaSaved"), deleteSuccessMessage: t("settings.toast.alibabaDeleted"),
      },
      mistral: {
        value: mistralApiKeyInput, setValue: setMistralApiKeyInput, setSaving: setSavingMistralKey, setDeleting: setDeletingMistralKey,
        save: api.setMistralApiKey, remove: api.deleteMistralApiKey,
        deleteTitle: t("settings.mistralDeleteTitle"), deleteMessage: t("settings.mistralDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.mistralSaved"), deleteSuccessMessage: t("settings.toast.mistralDeleted"),
      },
      cerebras: {
        value: cerebrasApiKeyInput, setValue: setCerebrasApiKeyInput, setSaving: setSavingCerebrasKey, setDeleting: setDeletingCerebrasKey,
        save: api.setCerebrasApiKey, remove: api.deleteCerebrasApiKey,
        deleteTitle: t("settings.cerebrasDeleteTitle"), deleteMessage: t("settings.cerebrasDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.cerebrasSaved"), deleteSuccessMessage: t("settings.toast.cerebrasDeleted"),
      },
      minimax: {
        value: miniMaxApiKeyInput, setValue: setMiniMaxApiKeyInput, setSaving: setSavingMiniMaxKey, setDeleting: setDeletingMiniMaxKey,
        save: api.setMiniMaxApiKey, remove: api.deleteMiniMaxApiKey,
        deleteTitle: t("settings.minimaxDeleteTitle"), deleteMessage: t("settings.minimaxDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.minimaxSaved"), deleteSuccessMessage: t("settings.toast.minimaxDeleted"),
      },
      xiaomi_mimo_token_plan: {
        value: xiaomiMiMoTokenPlanApiKeyInput, setValue: setXiaomiMiMoTokenPlanApiKeyInput, setSaving: setSavingXiaomiMiMoTokenPlanKey, setDeleting: setDeletingXiaomiMiMoTokenPlanKey,
        save: api.setXiaomiMiMoTokenPlanApiKey, remove: api.deleteXiaomiMiMoTokenPlanApiKey,
        deleteTitle: t("settings.xiaomiMimoTokenPlanDeleteTitle"), deleteMessage: t("settings.xiaomiMimoTokenPlanDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.xiaomiMimoTokenPlanSaved"), deleteSuccessMessage: t("settings.toast.xiaomiMimoTokenPlanDeleted"),
      },
      moonshot: {
        value: moonshotApiKeyInput, setValue: setMoonshotApiKeyInput, setSaving: setSavingMoonshotKey, setDeleting: setDeletingMoonshotKey,
        save: api.setMoonshotApiKey, remove: api.deleteMoonshotApiKey,
        deleteTitle: t("settings.moonshotDeleteTitle"), deleteMessage: t("settings.moonshotDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.moonshotSaved"), deleteSuccessMessage: t("settings.toast.moonshotDeleted"),
      },
      xai: {
        value: xaiApiKeyInput, setValue: setXaiApiKeyInput, setSaving: setSavingXAIKey, setDeleting: setDeletingXAIKey,
        save: api.setXAIApiKey, remove: api.deleteXAIApiKey,
        deleteTitle: t("settings.xaiDeleteTitle"), deleteMessage: t("settings.xaiDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.xaiSaved"), deleteSuccessMessage: t("settings.toast.xaiDeleted"),
        afterSave: onResetXAIVoices,
        afterDelete: onResetXAIVoices,
      },
      zai: {
        value: zaiApiKeyInput, setValue: setZaiApiKeyInput, setSaving: setSavingZAIKey, setDeleting: setDeletingZAIKey,
        save: api.setZAIApiKey, remove: api.deleteZAIApiKey,
        deleteTitle: t("settings.zaiDeleteTitle"), deleteMessage: t("settings.zaiDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.zaiSaved"), deleteSuccessMessage: t("settings.toast.zaiDeleted"),
      },
      fireworks: {
        value: fireworksApiKeyInput, setValue: setFireworksApiKeyInput, setSaving: setSavingFireworksKey, setDeleting: setDeletingFireworksKey,
        save: api.setFireworksApiKey, remove: api.deleteFireworksApiKey,
        deleteTitle: t("settings.fireworksDeleteTitle"), deleteMessage: t("settings.fireworksDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.fireworksSaved"), deleteSuccessMessage: t("settings.toast.fireworksDeleted"),
      },
      together: {
        value: togetherApiKeyInput, setValue: setTogetherApiKeyInput, setSaving: setSavingTogetherKey, setDeleting: setDeletingTogetherKey,
        save: api.setTogetherApiKey, remove: api.deleteTogetherApiKey,
        deleteTitle: t("settings.togetherDeleteTitle"), deleteMessage: t("settings.togetherDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.togetherSaved"), deleteSuccessMessage: t("settings.toast.togetherDeleted"),
      },
      poe: {
        value: poeApiKeyInput, setValue: setPoeApiKeyInput, setSaving: setSavingPoeKey, setDeleting: setDeletingPoeKey,
        save: api.setPoeApiKey, remove: api.deletePoeApiKey,
        deleteTitle: t("settings.poeDeleteTitle"), deleteMessage: t("settings.poeDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.poeSaved"), deleteSuccessMessage: t("settings.toast.poeDeleted"),
      },
      siliconflow: {
        value: siliconFlowApiKeyInput, setValue: setSiliconFlowApiKeyInput, setSaving: setSavingSiliconFlowKey, setDeleting: setDeletingSiliconFlowKey,
        save: api.setSiliconFlowApiKey, remove: api.deleteSiliconFlowApiKey,
        deleteTitle: t("settings.siliconflowDeleteTitle"), deleteMessage: t("settings.siliconflowDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.siliconflowSaved"), deleteSuccessMessage: t("settings.toast.siliconflowDeleted"),
      },
      openrouter: {
        value: openRouterApiKeyInput, setValue: setOpenRouterApiKeyInput, setSaving: setSavingOpenRouterKey, setDeleting: setDeletingOpenRouterKey,
        save: api.setOpenRouterApiKey, remove: api.deleteOpenRouterApiKey,
        deleteTitle: t("settings.openrouterDeleteTitle"), deleteMessage: t("settings.openrouterDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.openrouterSaved"), deleteSuccessMessage: t("settings.toast.openrouterDeleted"),
      },
      deepinfra: {
        value: deepInfraApiKeyInput, setValue: setDeepInfraApiKeyInput, setSaving: setSavingDeepInfraKey, setDeleting: setDeletingDeepInfraKey,
        save: api.setDeepInfraApiKey, remove: api.deleteDeepInfraApiKey,
        deleteTitle: t("settings.deepinfraDeleteTitle"), deleteMessage: t("settings.deepinfraDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.deepinfraSaved"), deleteSuccessMessage: t("settings.toast.deepinfraDeleted"),
      },
      featherless: {
        value: featherlessApiKeyInput, setValue: setFeatherlessApiKeyInput, setSaving: setSavingFeatherlessKey, setDeleting: setDeletingFeatherlessKey,
        save: api.setFeatherlessApiKey, remove: api.deleteFeatherlessApiKey,
        deleteTitle: t("settings.featherlessDeleteTitle"), deleteMessage: t("settings.featherlessDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.featherlessSaved"), deleteSuccessMessage: t("settings.toast.featherlessDeleted"),
      },
      aivis: {
        value: aivisApiKeyInput, setValue: setAivisApiKeyInput, setSaving: setSavingAivisKey, setDeleting: setDeletingAivisKey,
        save: api.setAivisApiKey, remove: api.deleteAivisApiKey,
        deleteTitle: t("settings.aivisDeleteTitle"), deleteMessage: t("settings.aivisDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.aivisSaved"), deleteSuccessMessage: t("settings.toast.aivisDeleted"),
        afterSave: onMarkAivisUserDictionariesStale,
        afterDelete: () => {
          onClearAivisUserDictionarySelection();
          onResetAivisUserDictionaries();
        },
      },
      elevenlabs: {
        value: elevenLabsApiKeyInput, setValue: setElevenLabsApiKeyInput, setSaving: setSavingElevenLabsKey, setDeleting: setDeletingElevenLabsKey,
        save: api.setElevenLabsApiKey, remove: api.deleteElevenLabsApiKey,
        deleteTitle: t("settings.elevenlabsDeleteTitle"), deleteMessage: t("settings.elevenlabsDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.elevenlabsSaved"), deleteSuccessMessage: t("settings.toast.elevenlabsDeleted"),
        afterSave: onResetElevenLabsVoices,
        afterDelete: onResetElevenLabsVoices,
      },
      cartesia: {
        value: cartesiaApiKeyInput, setValue: setCartesiaApiKeyInput, setSaving: setSavingCartesiaKey, setDeleting: setDeletingCartesiaKey,
        save: api.setCartesiaApiKey, remove: api.deleteCartesiaApiKey,
        deleteTitle: t("settings.cartesiaDeleteTitle"), deleteMessage: t("settings.cartesiaDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.cartesiaSaved"), deleteSuccessMessage: t("settings.toast.cartesiaDeleted"),
      },
      fish: {
        value: fishApiKeyInput, setValue: setFishApiKeyInput, setSaving: setSavingFishKey, setDeleting: setDeletingFishKey,
        save: api.setFishApiKey, remove: api.deleteFishApiKey,
        deleteTitle: t("settings.fishDeleteTitle"), deleteMessage: t("settings.fishDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.fishSaved"), deleteSuccessMessage: t("settings.toast.fishDeleted"),
      },
    },
  });

  const submitAzureSpeechConfig = useCallback(async (event: FormEvent) => {
    event.preventDefault();
    setSavingAzureSpeechConfig(true);
    try {
      const apiKey = azureSpeechApiKeyInput.trim();
      const region = azureSpeechRegionInput.trim();
      if (!apiKey) throw new Error(t("settings.error.enterApiKey"));
      if (!region) throw new Error(t("settings.azureSpeechRegionRequired"));
      await api.setAzureSpeechConfig(apiKey, region);
      setAzureSpeechApiKeyInput("");
      await reload();
      showToast(t("settings.toast.azureSpeechSaved"), "success");
    } catch (error) {
      showToast(String(error), "error");
    } finally {
      setSavingAzureSpeechConfig(false);
    }
  }, [azureSpeechApiKeyInput, azureSpeechRegionInput, reload, showToast, t]);

  const deleteAzureSpeechConfig = useCallback(async () => {
    if (!(await confirm({
      title: t("settings.azureSpeechDeleteTitle"),
      message: t("settings.azureSpeechDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingAzureSpeechConfig(true);
    try {
      await api.deleteAzureSpeechConfig();
      setAzureSpeechRegionInput("");
      await reload();
      showToast(t("settings.toast.azureSpeechDeleted"), "success");
    } catch (error) {
      showToast(String(error), "error");
    } finally {
      setDeletingAzureSpeechConfig(false);
    }
  }, [confirm, reload, showToast, t]);

  const apiKeyCardLabels = useMemo(() => buildApiKeyCardLabels(t), [t]);

  const accessCards = buildAccessCards(
    settings,
    {
      anthropic: createAccessCardRuntime(anthropicApiKeyInput, setAnthropicApiKeyInput, apiKeyHandlers.anthropic!.submit, apiKeyHandlers.anthropic!.remove, savingAnthropicKey, deletingAnthropicKey),
      openai: createAccessCardRuntime(openAIApiKeyInput, setOpenAIApiKeyInput, apiKeyHandlers.openai!.submit, apiKeyHandlers.openai!.remove, savingOpenAIKey, deletingOpenAIKey),
      google: createAccessCardRuntime(googleApiKeyInput, setGoogleApiKeyInput, apiKeyHandlers.google!.submit, apiKeyHandlers.google!.remove, savingGoogleKey, deletingGoogleKey),
      groq: createAccessCardRuntime(groqApiKeyInput, setGroqApiKeyInput, apiKeyHandlers.groq!.submit, apiKeyHandlers.groq!.remove, savingGroqKey, deletingGroqKey),
      deepseek: createAccessCardRuntime(deepseekApiKeyInput, setDeepseekApiKeyInput, apiKeyHandlers.deepseek!.submit, apiKeyHandlers.deepseek!.remove, savingDeepSeekKey, deletingDeepSeekKey),
      alibaba: createAccessCardRuntime(alibabaApiKeyInput, setAlibabaApiKeyInput, apiKeyHandlers.alibaba!.submit, apiKeyHandlers.alibaba!.remove, savingAlibabaKey, deletingAlibabaKey),
      minimax: createAccessCardRuntime(miniMaxApiKeyInput, setMiniMaxApiKeyInput, apiKeyHandlers.minimax!.submit, apiKeyHandlers.minimax!.remove, savingMiniMaxKey, deletingMiniMaxKey),
      xiaomi_mimo_token_plan: createAccessCardRuntime(xiaomiMiMoTokenPlanApiKeyInput, setXiaomiMiMoTokenPlanApiKeyInput, apiKeyHandlers.xiaomi_mimo_token_plan!.submit, apiKeyHandlers.xiaomi_mimo_token_plan!.remove, savingXiaomiMiMoTokenPlanKey, deletingXiaomiMiMoTokenPlanKey),
      mistral: createAccessCardRuntime(mistralApiKeyInput, setMistralApiKeyInput, apiKeyHandlers.mistral!.submit, apiKeyHandlers.mistral!.remove, savingMistralKey, deletingMistralKey),
      cerebras: createAccessCardRuntime(cerebrasApiKeyInput, setCerebrasApiKeyInput, apiKeyHandlers.cerebras!.submit, apiKeyHandlers.cerebras!.remove, savingCerebrasKey, deletingCerebrasKey),
      moonshot: createAccessCardRuntime(moonshotApiKeyInput, setMoonshotApiKeyInput, apiKeyHandlers.moonshot!.submit, apiKeyHandlers.moonshot!.remove, savingMoonshotKey, deletingMoonshotKey),
      xai: createAccessCardRuntime(xaiApiKeyInput, setXaiApiKeyInput, apiKeyHandlers.xai!.submit, apiKeyHandlers.xai!.remove, savingXAIKey, deletingXAIKey),
      zai: createAccessCardRuntime(zaiApiKeyInput, setZaiApiKeyInput, apiKeyHandlers.zai!.submit, apiKeyHandlers.zai!.remove, savingZAIKey, deletingZAIKey),
      fireworks: createAccessCardRuntime(fireworksApiKeyInput, setFireworksApiKeyInput, apiKeyHandlers.fireworks!.submit, apiKeyHandlers.fireworks!.remove, savingFireworksKey, deletingFireworksKey),
      together: createAccessCardRuntime(togetherApiKeyInput, setTogetherApiKeyInput, apiKeyHandlers.together!.submit, apiKeyHandlers.together!.remove, savingTogetherKey, deletingTogetherKey),
      poe: createAccessCardRuntime(poeApiKeyInput, setPoeApiKeyInput, apiKeyHandlers.poe!.submit, apiKeyHandlers.poe!.remove, savingPoeKey, deletingPoeKey),
      siliconflow: createAccessCardRuntime(siliconFlowApiKeyInput, setSiliconFlowApiKeyInput, apiKeyHandlers.siliconflow!.submit, apiKeyHandlers.siliconflow!.remove, savingSiliconFlowKey, deletingSiliconFlowKey),
      azure_speech: createAccessCardRuntime(
        azureSpeechApiKeyInput,
        setAzureSpeechApiKeyInput,
        submitAzureSpeechConfig,
        deleteAzureSpeechConfig,
        savingAzureSpeechConfig,
        deletingAzureSpeechConfig,
        azureSpeechRegionInput,
        setAzureSpeechRegionInput,
      ),
      openrouter: createAccessCardRuntime(openRouterApiKeyInput, setOpenRouterApiKeyInput, apiKeyHandlers.openrouter!.submit, apiKeyHandlers.openrouter!.remove, savingOpenRouterKey, deletingOpenRouterKey),
      deepinfra: createAccessCardRuntime(deepInfraApiKeyInput, setDeepInfraApiKeyInput, apiKeyHandlers.deepinfra!.submit, apiKeyHandlers.deepinfra!.remove, savingDeepInfraKey, deletingDeepInfraKey),
      featherless: createAccessCardRuntime(featherlessApiKeyInput, setFeatherlessApiKeyInput, apiKeyHandlers.featherless!.submit, apiKeyHandlers.featherless!.remove, savingFeatherlessKey, deletingFeatherlessKey),
      aivis: createAccessCardRuntime(aivisApiKeyInput, setAivisApiKeyInput, apiKeyHandlers.aivis!.submit, apiKeyHandlers.aivis!.remove, savingAivisKey, deletingAivisKey),
      elevenlabs: createAccessCardRuntime(elevenLabsApiKeyInput, setElevenLabsApiKeyInput, apiKeyHandlers.elevenlabs!.submit, apiKeyHandlers.elevenlabs!.remove, savingElevenLabsKey, deletingElevenLabsKey),
      cartesia: createAccessCardRuntime(cartesiaApiKeyInput, setCartesiaApiKeyInput, apiKeyHandlers.cartesia!.submit, apiKeyHandlers.cartesia!.remove, savingCartesiaKey, deletingCartesiaKey),
      fish: createAccessCardRuntime(fishApiKeyInput, setFishApiKeyInput, apiKeyHandlers.fish!.submit, apiKeyHandlers.fish!.remove, savingFishKey, deletingFishKey),
    },
    t,
  );

  return {
    accessCards,
    apiKeyCardLabels,
  };
}
