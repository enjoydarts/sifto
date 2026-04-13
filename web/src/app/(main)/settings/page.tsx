"use client";

import {
  AudioBriefingVoicePickerStack,
  SettingsDialogStack,
  SummaryAudioVoicePickerStack,
} from "@/components/settings/settings-dialog-stack";
import SettingsHeroActions from "@/components/settings/settings-hero-actions";
import SettingsPageShell from "@/components/settings/settings-page-shell";
import AudioBriefingSettingsSection from "@/components/settings/sections/audio-briefing-settings-section";
import AudioBriefingVoiceMatrixSection from "@/components/settings/sections/audio-briefing-voice-matrix-section";
import BudgetSettingsSection from "@/components/settings/sections/budget-settings-section";
import DigestSettingsSection from "@/components/settings/sections/digest-settings-section";
import IntegrationsSettingsSection from "@/components/settings/sections/integrations-settings-section";
import ModelsSettingsSection from "@/components/settings/sections/models-settings-section";
import NavigatorSettingsSection from "@/components/settings/sections/navigator-settings-section";
import NotificationsSettingsSection from "@/components/settings/sections/notifications-settings-section";
import PersonalizationSettingsSection from "@/components/settings/sections/personalization-settings-section";
import PodcastSettingsSection from "@/components/settings/sections/podcast-settings-section";
import ReadingPlanSettingsSection from "@/components/settings/sections/reading-plan-settings-section";
import SummaryAudioSettingsSection from "@/components/settings/sections/summary-audio-settings-section";
import SystemSettingsSection from "@/components/settings/sections/system-settings-section";
import { DEFAULT_UI_FONT_SANS_KEY, DEFAULT_UI_FONT_SERIF_KEY } from "@/lib/ui-fonts";
import { useSettingsPageData } from "./use-settings-page-data";

export default function SettingsPage() {
  const data = useSettingsPageData();

  if (data.loading) return <p className="text-sm text-zinc-500">{data.t("common.loading")}</p>;
  if (data.error) {
    return (
      <div className="surface-editorial rounded-[24px] border border-red-200 bg-red-50 p-5">
        <p className="text-sm text-red-700">{data.error}</p>
        <button
          type="button"
          onClick={() => void data.load()}
          className="mt-3 inline-flex min-h-10 items-center rounded-full border border-red-200 bg-white px-4 text-sm font-medium text-red-700 hover:bg-red-100"
        >
          {data.t("settings.retry")}
        </button>
      </div>
    );
  }
  if (!data.settings) return null;

  const {
    t,
    activeSection,
    setActiveSection,
    sectionNavItems,
    railNotes,
    selectedSectionMeta,
    applyCostPerformancePreset,
    toggleLLMExtras,
    llm,
    modelSelectLabels,
    audioBriefingSettingsForm,
    audioBriefingSettingsState,
    audioBriefingDuoReadiness,
    audioBriefingScriptModels,
    audioBriefingDictionaryState,
    audioBriefingSettingsActions,
    audioBriefingVoiceMatrixForm,
    audioBriefingVoiceMatrixStatus,
    audioBriefingVoiceMatrixAvailability,
    audioBriefingVoiceMatrixCatalogs,
    audioBriefingVoiceMatrixActions,
    podcastForm,
    podcastState,
    podcastActions,
    summaryAudioForm,
    summaryAudioState,
    summaryAudioActions,
    summaryAudioIntegrations,
    readingPlanForm,
    readingPlanState,
    readingPlanActions,
    preferenceProfile,
    preferenceProfileError,
    resettingPreferenceProfile,
    handleResetPreferenceProfile,
    load,
    digestForm,
    digestState,
    digestActions,
    notificationPriority,
    saveNotificationPriority,
    integrationsState,
    integrationsActions,
    llmModelsForm,
    llmModelsState,
    llmModelsActions,
    llmModelsExtras,
    unavailableSelectedModelWarnings,
    navigatorForm,
    navigatorState,
    navigatorActions,
    budgetForm,
    budgetState,
    budgetActions,
    systemUIFontState,
    systemAccessState,
    audioBriefingPickers,
    summaryAudioPickers,
    audioBriefingVoices,
    activeAivisVoice,
    activeXAIVoice,
    activeElevenLabsVoice,
    activeOpenAITTSVoice,
    activeGeminiTTSVoice,
    activeAzureSpeechVoice,
    aivisModelsData,
    aivisModelsLoading,
    aivisModelsSyncing,
    aivisModelsError,
    xaiVoicesLoading,
    xaiVoicesSyncing,
    xaiVoicesError,
    openAITTSVoicesLoading,
    openAITTSVoicesSyncing,
    openAITTSVoicesError,
    elevenLabsVoicesLoading,
    elevenLabsVoicesError,
    geminiTTSVoicesLoading,
    geminiTTSVoicesError,
    azureSpeechVoicesLoading,
    azureSpeechVoicesError,
    audioBriefingPresets,
    audioBriefingPresetsLoading,
    audioBriefingPresetsError,
    audioBriefingDefaultPersonaMode,
    audioBriefingDefaultPersona,
    audioBriefingConversationMode,
    syncAivisModels,
    syncXAIVoices,
    syncOpenAITTSVoices,
    loadElevenLabsVoices,
    loadGeminiTTSVoices,
    loadAzureSpeechVoices,
    audioBriefingPickerSelectActions,
    summaryAudioPickerSelectActions,
    summaryAudioVoiceModel,
    summaryAudioVoiceStyle,
    uiFonts,
    presets,
    uiFontSansOptions,
    uiFontSerifOptions,
    savedUIFontSansKey,
    savedUIFontSerifKey,
    uiFontSansKey,
    uiFontSerifKey,
    setUIFontSansKey,
    setUIFontSerifKey,
    modelComparisonEntries,
    audioBriefingXAIVoices,
    audioBriefingOpenAITTSVoices,
    audioBriefingGeminiTTSVoices,
    audioBriefingAzureSpeechVoices,
    audioBriefingElevenLabsVoices,
    summaryAudioXAIVoices,
    summaryAudioElevenLabsVoices,
    summaryAudioOpenAITTSVoices,
    summaryAudioGeminiTTSVoices,
    summaryAudioAzureSpeechVoices,
    applyAudioBriefingPreset,
    submitAudioBriefingPresetSave,
    loadAudioBriefingPresets,
    tWithVars,
  } = data;

  return (
    <>
      <SettingsPageShell
        t={t}
        activeSection={activeSection}
        sectionNavItems={sectionNavItems}
        railNotes={railNotes}
        selectedSectionMeta={selectedSectionMeta}
        onSelectSection={setActiveSection}
        heroActions={
          activeSection === "models" ? (
            <SettingsHeroActions
              costPerformanceLabel={t("settings.modelPreset.costPerformance")}
              extrasLabel={t("settings.section.llmExtras")}
              extrasOpen={llm.llmExtrasOpen}
              onApplyCostPerformancePreset={applyCostPerformancePreset}
              onToggleExtras={toggleLLMExtras}
            />
          ) : null
        }
      >
        {activeSection === "audio-briefing" ? (
          <>
            <AudioBriefingSettingsSection
              t={t}
              tWithVars={tWithVars}
              modelSelectLabels={modelSelectLabels}
              settingsForm={audioBriefingSettingsForm}
              settingsState={audioBriefingSettingsState}
              duoReadiness={audioBriefingDuoReadiness}
              scriptModels={audioBriefingScriptModels}
              dictionaryState={audioBriefingDictionaryState}
              actions={audioBriefingSettingsActions}
            />

            <AudioBriefingVoiceMatrixSection
              t={t}
              modelSelectLabels={modelSelectLabels}
              form={audioBriefingVoiceMatrixForm}
              status={audioBriefingVoiceMatrixStatus}
              availability={audioBriefingVoiceMatrixAvailability}
              catalogs={audioBriefingVoiceMatrixCatalogs}
              actions={audioBriefingVoiceMatrixActions}
            />

            <PodcastSettingsSection
              t={t}
              form={podcastForm}
              state={podcastState}
              actions={podcastActions}
            />
          </>
        ) : null}

        {activeSection === "summary-audio" ? (
          <SummaryAudioSettingsSection
            t={t}
            modelSelectLabels={modelSelectLabels}
            form={summaryAudioForm}
            state={summaryAudioState}
            actions={summaryAudioActions}
            integrations={summaryAudioIntegrations}
          />
        ) : null}

        {activeSection === "reading-plan" ? (
          <ReadingPlanSettingsSection
            t={t}
            form={readingPlanForm}
            state={readingPlanState}
            actions={readingPlanActions}
          />
        ) : null}

        {activeSection === "personalization" ? (
          <PersonalizationSettingsSection
            profile={preferenceProfile}
            error={preferenceProfileError}
            resetting={resettingPreferenceProfile}
            actions={{
              onReset: () => {
                void handleResetPreferenceProfile();
              },
              onRetry: () => {
                void load();
              },
            }}
          />
        ) : null}

        {activeSection === "digest" ? (
          <DigestSettingsSection
            t={t}
            form={digestForm}
            state={digestState}
            actions={digestActions}
          />
        ) : null}

        {activeSection === "notifications" ? (
          <NotificationsSettingsSection
            rule={notificationPriority}
            onSaveRule={saveNotificationPriority}
          />
        ) : null}

        {activeSection === "integrations" ? (
          <IntegrationsSettingsSection
            t={t}
            state={integrationsState}
            actions={integrationsActions}
          />
        ) : null}

        {activeSection === "models" ? (
          <ModelsSettingsSection
            t={t}
            labels={modelSelectLabels}
            form={llmModelsForm}
            models={llmModelsState}
            actions={llmModelsActions}
            extras={llmModelsExtras}
            unavailableWarnings={unavailableSelectedModelWarnings}
          />
        ) : null}

        {activeSection === "navigator" ? (
          <NavigatorSettingsSection
            t={t}
            modelSelectLabels={modelSelectLabels}
            form={navigatorForm}
            state={navigatorState}
            actions={navigatorActions}
          />
        ) : null}

        {activeSection === "budget" ? (
          <BudgetSettingsSection
            t={t}
            form={budgetForm}
            state={budgetState}
            actions={budgetActions}
          />
        ) : null}

        {activeSection === "system" ? (
          <SystemSettingsSection
            t={t}
            uiFonts={systemUIFontState}
            access={systemAccessState}
          />
        ) : null}
      </SettingsPageShell>

      <AudioBriefingVoicePickerStack
        pickerState={{
          aivisPickerPersona: audioBriefingPickers.aivisPickerPersona,
          fishPickerPersona: audioBriefingPickers.fishPickerPersona,
          xaiPickerPersona: audioBriefingPickers.xaiPickerPersona,
          openAITTPickerPersona: audioBriefingPickers.openAITTPickerPersona,
          elevenLabsPickerPersona: audioBriefingPickers.elevenLabsPickerPersona,
          geminiTTSPickerPersona: audioBriefingPickers.geminiTTSPickerPersona,
          azureSpeechPickerPersona: audioBriefingPickers.azureSpeechPickerPersona,
          activeAivisVoice: activeAivisVoice ?? null,
          activeXAIVoice: activeXAIVoice ?? null,
          activeOpenAITTSVoice: activeOpenAITTSVoice ?? null,
          activeElevenLabsVoice: activeElevenLabsVoice ?? null,
          activeGeminiTTSVoice: activeGeminiTTSVoice ?? null,
          activeAzureSpeechVoice: activeAzureSpeechVoice ?? null,
          audioBriefingVoices,
        }}
        catalogs={{
          aivisModels: aivisModelsData?.models ?? [],
          aivisModelsLoading,
          aivisModelsSyncing,
          aivisModelsError,
          xaiVoices: audioBriefingXAIVoices,
          xaiVoicesLoading,
          xaiVoicesSyncing,
          xaiVoicesError,
          openAITTSVoices: audioBriefingOpenAITTSVoices,
          openAITTSVoicesLoading,
          openAITTSVoicesSyncing,
          openAITTSVoicesError,
          elevenLabsVoices: audioBriefingElevenLabsVoices,
          elevenLabsVoicesLoading,
          elevenLabsVoicesError,
          geminiTTSVoices: audioBriefingGeminiTTSVoices,
          geminiTTSVoicesLoading,
          geminiTTSVoicesError,
          azureSpeechVoices: audioBriefingAzureSpeechVoices,
          azureSpeechVoicesLoading,
          azureSpeechVoicesError,
        }}
        actions={{
          onCloseAivis: audioBriefingPickers.closeAivisPicker,
          onCloseFish: audioBriefingPickers.closeFishPicker,
          onCloseXAI: audioBriefingPickers.closeXAIPicker,
          onCloseOpenAI: audioBriefingPickers.closeOpenAITTPicker,
          onCloseElevenLabs: audioBriefingPickers.closeElevenLabsPicker,
          onCloseGemini: audioBriefingPickers.closeGeminiTTSPicker,
          onCloseAzureSpeech: audioBriefingPickers.closeAzureSpeechPicker,
          onSyncAivis: () => {
            void syncAivisModels();
          },
          onSyncXAI: () => {
            void syncXAIVoices();
          },
          onSyncOpenAI: () => {
            void syncOpenAITTSVoices().catch(() => undefined);
          },
          onRefreshElevenLabs: () => {
            void loadElevenLabsVoices().catch(() => undefined);
          },
          onRefreshGemini: () => {
            void loadGeminiTTSVoices().catch(() => undefined);
          },
          onRefreshAzureSpeech: () => {
            void loadAzureSpeechVoices().catch(() => undefined);
          },
          onSelectAivis: audioBriefingPickerSelectActions.onSelectAivis,
          onSelectFish: audioBriefingPickerSelectActions.onSelectFish,
          onSelectXAI: audioBriefingPickerSelectActions.onSelectXAI,
          onSelectOpenAI: audioBriefingPickerSelectActions.onSelectOpenAI,
          onSelectElevenLabs: audioBriefingPickerSelectActions.onSelectElevenLabs,
          onSelectGemini: audioBriefingPickerSelectActions.onSelectGemini,
          onSelectAzureSpeech: audioBriefingPickerSelectActions.onSelectAzureSpeech,
        }}
      />

      <SummaryAudioVoicePickerStack
        pickerState={{
          summaryAudioAivisPickerOpen: summaryAudioPickers.summaryAudioAivisPickerOpen,
          summaryAudioFishPickerOpen: summaryAudioPickers.summaryAudioFishPickerOpen,
          summaryAudioElevenLabsPickerOpen: summaryAudioPickers.summaryAudioElevenLabsPickerOpen,
          summaryAudioXAIPickerOpen: summaryAudioPickers.summaryAudioXAIPickerOpen,
          summaryAudioOpenAITTPickerOpen: summaryAudioPickers.summaryAudioOpenAITTPickerOpen,
          summaryAudioGeminiTTSPickerOpen: summaryAudioPickers.summaryAudioGeminiTTSPickerOpen,
          summaryAudioAzureSpeechPickerOpen: summaryAudioPickers.summaryAudioAzureSpeechPickerOpen,
          summaryAudioVoiceModel,
          summaryAudioVoiceStyle,
        }}
        catalogs={{
          aivisModels: aivisModelsData?.models ?? [],
          aivisModelsLoading,
          aivisModelsSyncing,
          aivisModelsError,
          xaiVoices: summaryAudioXAIVoices,
          xaiVoicesLoading,
          xaiVoicesSyncing,
          xaiVoicesError,
          openAITTSVoices: summaryAudioOpenAITTSVoices,
          openAITTSVoicesLoading,
          openAITTSVoicesSyncing,
          openAITTSVoicesError,
          elevenLabsVoices: summaryAudioElevenLabsVoices,
          elevenLabsVoicesLoading,
          elevenLabsVoicesError,
          geminiTTSVoices: summaryAudioGeminiTTSVoices,
          geminiTTSVoicesLoading,
          geminiTTSVoicesError,
          azureSpeechVoices: summaryAudioAzureSpeechVoices,
          azureSpeechVoicesLoading,
          azureSpeechVoicesError,
        }}
        actions={{
          onCloseAivis: summaryAudioPickers.closeAivisPicker,
          onCloseFish: summaryAudioPickers.closeFishPicker,
          onCloseElevenLabs: summaryAudioPickers.closeElevenLabsPicker,
          onCloseXAI: summaryAudioPickers.closeXAIPicker,
          onCloseOpenAI: summaryAudioPickers.closeOpenAITTPicker,
          onCloseGemini: summaryAudioPickers.closeGeminiTTSPicker,
          onCloseAzureSpeech: summaryAudioPickers.closeAzureSpeechPicker,
          onSyncAivis: () => {
            void syncAivisModels();
          },
          onSyncXAI: () => {
            void syncXAIVoices();
          },
          onSyncOpenAI: () => {
            void syncOpenAITTSVoices().catch(() => undefined);
          },
          onRefreshElevenLabs: () => {
            void loadElevenLabsVoices().catch(() => undefined);
          },
          onRefreshGemini: () => {
            void loadGeminiTTSVoices().catch(() => undefined);
          },
          onRefreshAzureSpeech: () => {
            void loadAzureSpeechVoices().catch(() => undefined);
          },
          onSelectAivis: summaryAudioPickerSelectActions.onSelectAivis,
          onSelectFish: summaryAudioPickerSelectActions.onSelectFish,
          onSelectElevenLabs: summaryAudioPickerSelectActions.onSelectElevenLabs,
          onSelectXAI: summaryAudioPickerSelectActions.onSelectXAI,
          onSelectOpenAI: summaryAudioPickerSelectActions.onSelectOpenAI,
          onSelectGemini: summaryAudioPickerSelectActions.onSelectGemini,
          onSelectAzureSpeech: summaryAudioPickerSelectActions.onSelectAzureSpeech,
        }}
      />

      <SettingsDialogStack
        uiFonts={{
          uiFontSansPickerOpen: uiFonts.uiFontSansPickerOpen,
          uiFontSerifPickerOpen: uiFonts.uiFontSerifPickerOpen,
          uiFontSansOptions,
          uiFontSerifOptions,
          savedUIFontSansKey,
          savedUIFontSerifKey,
          uiFontSansKey,
          uiFontSerifKey,
          defaultUIFontSansKey: DEFAULT_UI_FONT_SANS_KEY,
          defaultUIFontSerifKey: DEFAULT_UI_FONT_SERIF_KEY,
          onCloseUIFontSans: uiFonts.closeSansPicker,
          onCloseUIFontSerif: uiFonts.closeSerifPicker,
          onSelectUIFontSans: setUIFontSansKey,
          onSelectUIFontSerif: setUIFontSerifKey,
        }}
        presets={{
          audioBriefingPresetSaveOpen: presets.audioBriefingPresetSaveOpen,
          audioBriefingPresetSaving: presets.audioBriefingPresetSaving,
          audioBriefingPresetName: presets.audioBriefingPresetName,
          audioBriefingDefaultPersonaMode,
          audioBriefingDefaultPersona,
          audioBriefingConversationMode,
          audioBriefingVoices,
          onClosePresetSave: () => {
            presets.setAudioBriefingPresetSaveOpen(false);
            presets.setAudioBriefingPresetName("");
          },
          onChangePresetName: presets.setAudioBriefingPresetName,
          onSavePreset: () => {
            void submitAudioBriefingPresetSave();
          },
          audioBriefingPresetApplyOpen: presets.audioBriefingPresetApplyOpen,
          audioBriefingPresetsLoading,
          audioBriefingPresetsError,
          audioBriefingPresets,
          audioBriefingPresetApplySelection: presets.audioBriefingPresetApplySelection,
          onClosePresetApply: () => {
            presets.setAudioBriefingPresetApplyOpen(false);
            presets.setAudioBriefingPresetApplySelection(null);
          },
          onRefreshPresets: () => {
            void loadAudioBriefingPresets();
          },
          onSelectPreset: presets.setAudioBriefingPresetApplySelection,
          onApplyPreset: applyAudioBriefingPreset,
        }}
        modelGuide={{
          modelGuideOpen: llm.modelGuideOpen,
          modelComparisonEntries,
          onCloseModelGuide: llm.closeModelGuide,
        }}
        t={t}
      />
    </>
  );
}
