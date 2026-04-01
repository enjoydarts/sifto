"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { RefreshCw } from "lucide-react";
import {
  api,
  PromptAdminCapabilities,
  PromptExperiment,
  PromptExperimentArm,
  PromptTemplateDefault,
  PromptTemplate,
  PromptTemplateDetailResponse,
} from "@/lib/api";
import { PageTransition } from "@/components/page-transition";
import { PageHeader } from "@/components/ui/page-header";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";

function parsePrettyJSON(raw: string) {
  const trimmed = raw.trim();
  if (!trimmed) return {};
  return JSON.parse(trimmed) as Record<string, unknown>;
}

function normalizePromptTemplateDetail(detail: PromptTemplateDetailResponse): PromptTemplateDetailResponse {
  return {
    ...detail,
    versions: detail.versions ?? [],
    experiments: detail.experiments ?? [],
    arms: detail.arms ?? [],
    default_template: {
      label: detail.default_template?.label ?? "",
      system_instruction: detail.default_template?.system_instruction ?? "",
      prompt_text: detail.default_template?.prompt_text ?? "",
      fallback_prompt_text: detail.default_template?.fallback_prompt_text ?? "",
      variables_schema: detail.default_template?.variables_schema ?? {},
      preview_variables: detail.default_template?.preview_variables ?? {},
      notes: detail.default_template?.notes ?? "",
    },
  };
}

function stringifySchema(value: PromptTemplateDefault["variables_schema"]) {
  if (typeof value === "string") {
    return value.trim() || "{}";
  }
  return JSON.stringify(value ?? {}, null, 2);
}

function formFromDefaultTemplate(defaultTemplate: PromptTemplateDefault) {
  return {
    label: defaultTemplate.label ?? "",
    system_instruction: defaultTemplate.system_instruction ?? "",
    prompt_text: defaultTemplate.prompt_text ?? "",
    fallback_prompt_text: defaultTemplate.fallback_prompt_text ?? "",
    variables_schema: stringifySchema(defaultTemplate.variables_schema),
    notes: defaultTemplate.notes ?? "",
  };
}

function normalizePreviewVariables(value: PromptTemplateDefault["preview_variables"]) {
  if (typeof value === "string") {
    try {
      return JSON.parse(value) as Record<string, unknown>;
    } catch {
      return {};
    }
  }
  return (value as Record<string, unknown> | null | undefined) ?? {};
}

function renderPromptTemplate(text: string, variables: Record<string, unknown>) {
  let rendered = text ?? "";
  for (const [key, value] of Object.entries(variables)) {
    const renderedValue = String(value ?? "");
    rendered = rendered.replaceAll(`{{${key}}}`, renderedValue);
    rendered = rendered.replaceAll(`{${key}}`, renderedValue);
  }
  return rendered;
}

export default function PromptAdminPage() {
  const { t } = useI18n();
  const { showToast } = useToast();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [previewOpen, setPreviewOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [capabilities, setCapabilities] = useState<PromptAdminCapabilities | null>(null);
  const [templates, setTemplates] = useState<PromptTemplate[]>([]);
  const [selectedTemplateId, setSelectedTemplateId] = useState<string>("");
  const [detail, setDetail] = useState<PromptTemplateDetailResponse | null>(null);
  const lastInitializedTemplateIdRef = useRef<string>("");
  const [form, setForm] = useState({
    label: "",
    system_instruction: "",
    prompt_text: "",
    fallback_prompt_text: "",
    variables_schema: "{}",
    notes: "",
  });
  const [experimentForm, setExperimentForm] = useState({
    name: "",
    status: "draft",
    assignment_unit: "item_id",
    version_id: "",
    weight: "100",
  });

  const loadTemplates = useCallback(async () => {
    const [capabilitiesRes, templatesRes] = await Promise.all([
      api.getPromptAdminCapabilities(),
      api.getPromptTemplates(),
    ]);
    setCapabilities(capabilitiesRes);
    setTemplates(templatesRes.templates);
    if (!selectedTemplateId && templatesRes.templates[0]?.id) {
      setSelectedTemplateId(templatesRes.templates[0].id);
    }
    return capabilitiesRes;
  }, [selectedTemplateId]);

  const loadDetail = useCallback(async (templateId: string) => {
    if (!templateId) {
      setDetail(null);
      return null;
    }
    const next = await api.getPromptTemplateDetail(templateId);
    const normalized = normalizePromptTemplateDetail(next);
    setDetail(normalized);
    return normalized;
  }, []);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const nextCapabilities = await loadTemplates();
      if (selectedTemplateId) {
        await loadDetail(selectedTemplateId);
      }
      setError(null);
      if (!nextCapabilities.can_manage_prompts) {
        setDetail(null);
      }
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, [loadDetail, loadTemplates, selectedTemplateId]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    if (!selectedTemplateId) return;
    loadDetail(selectedTemplateId).catch((e) => setError(String(e)));
  }, [loadDetail, selectedTemplateId]);

  useEffect(() => {
    const templateId = detail?.template.id ?? "";
    if (!templateId || lastInitializedTemplateIdRef.current === templateId) return;
    lastInitializedTemplateIdRef.current = templateId;
    setForm(formFromDefaultTemplate(detail.default_template));
  }, [detail]);

  const versionOptions = useMemo(() => detail?.versions ?? [], [detail?.versions]);
  const experimentArmsByExperimentId = useMemo(() => {
    const map = new Map<string, PromptExperimentArm[]>();
    for (const arm of detail?.arms ?? []) {
      const current = map.get(arm.experiment_id) ?? [];
      current.push(arm);
      map.set(arm.experiment_id, current);
    }
    return map;
  }, [detail?.arms]);
  const previewVariables = useMemo(() => normalizePreviewVariables(detail?.default_template?.preview_variables), [detail?.default_template?.preview_variables]);
  const renderedSystemInstruction = useMemo(
    () => renderPromptTemplate(form.system_instruction, previewVariables),
    [form.system_instruction, previewVariables]
  );
  const renderedPromptText = useMemo(() => renderPromptTemplate(form.prompt_text, previewVariables), [form.prompt_text, previewVariables]);

  const handleCreateVersion = useCallback(async () => {
    if (!detail?.template.id) return;
    setSaving(true);
    try {
      await api.createPromptTemplateVersion(detail.template.id, {
        ...form,
        variables_schema: parsePrettyJSON(form.variables_schema),
      });
      showToast(t("promptAdmin.versionCreated"), "success");
      const next = await loadDetail(detail.template.id);
      if (next) {
        setForm(formFromDefaultTemplate(next.default_template));
      }
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSaving(false);
    }
  }, [detail?.template.id, form, loadDetail, showToast, t]);

  const handleActivate = useCallback(
    async (versionId: string | null) => {
      if (!detail?.template.id) return;
      setSaving(true);
      try {
        await api.activatePromptTemplateVersion(detail.template.id, versionId);
        showToast(t("promptAdmin.versionActivated"), "success");
        await loadDetail(detail.template.id);
      } catch (e) {
        showToast(String(e), "error");
      } finally {
        setSaving(false);
      }
    },
    [detail?.template.id, loadDetail, showToast, t]
  );

  const handleCreateExperiment = useCallback(async () => {
    if (!detail?.template.id || !experimentForm.version_id) return;
    setSaving(true);
    try {
      await api.createPromptExperiment({
        template_id: detail.template.id,
        name: experimentForm.name,
        status: experimentForm.status,
        assignment_unit: experimentForm.assignment_unit,
        arms: [{ version_id: experimentForm.version_id, weight: Number(experimentForm.weight) || 100 }],
      });
      showToast(t("promptAdmin.experimentCreated"), "success");
      await loadDetail(detail.template.id);
      setExperimentForm({
        name: "",
        status: "draft",
        assignment_unit: "item_id",
        version_id: "",
        weight: "100",
      });
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSaving(false);
    }
  }, [detail?.template.id, experimentForm, loadDetail, showToast, t]);

  const handleExperimentStatus = useCallback(
    async (experiment: PromptExperiment, status: string) => {
      setSaving(true);
      try {
        const arms = (experimentArmsByExperimentId.get(experiment.id) ?? []).map((arm) => ({
          version_id: arm.version_id,
          weight: arm.weight,
        }));
        await api.updatePromptExperiment(experiment.id, { status, arms });
        showToast(t("promptAdmin.experimentUpdated"), "success");
        await loadDetail(experiment.template_id);
      } catch (e) {
        showToast(String(e), "error");
      } finally {
        setSaving(false);
      }
    },
    [experimentArmsByExperimentId, loadDetail, showToast, t]
  );

  return (
    <PageTransition>
      <div className="space-y-6">
        <PageHeader
          title={t("promptAdmin.title")}
          description={t("promptAdmin.description")}
          actions={
            <button
              type="button"
              onClick={load}
              className="inline-flex items-center gap-2 rounded-full border border-[var(--color-editorial-line)] px-4 py-2 text-sm text-[var(--color-editorial-ink)]"
            >
              <RefreshCw className={`size-4 ${loading ? "animate-spin" : ""}`} />
              {t("promptAdmin.refresh")}
            </button>
          }
        />

        {!loading && capabilities && !capabilities.can_manage_prompts ? (
          <div className="rounded-3xl border border-red-200 bg-red-50 p-6 text-sm text-red-700">
            {t("promptAdmin.forbidden")}
          </div>
        ) : null}

        {error ? <div className="rounded-3xl border border-red-200 bg-red-50 p-4 text-sm text-red-700">{error}</div> : null}

        <div className="grid gap-6 lg:grid-cols-[280px_minmax(0,1fr)]">
          <section className="rounded-3xl border border-[var(--color-editorial-line)] bg-white p-4">
            <div className="mb-3 text-sm font-semibold text-[var(--color-editorial-ink)]">{t("promptAdmin.templates")}</div>
            <div className="space-y-2">
              {templates.map((template) => (
                <button
                  key={template.id}
                  type="button"
                  onClick={() => setSelectedTemplateId(template.id)}
                  className={`w-full rounded-2xl border px-3 py-3 text-left ${
                    selectedTemplateId === template.id ? "border-[var(--color-editorial-accent)] bg-[var(--color-editorial-accent-soft)]" : "border-[var(--color-editorial-line)]"
                  }`}
                >
                  <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{template.key}</div>
                  <div className="text-xs text-[var(--color-editorial-ink-soft)]">{template.purpose}</div>
                </button>
              ))}
            </div>
          </section>

          <section className="space-y-6">
            {detail ? (
              <>
                <div className="rounded-3xl border border-[var(--color-editorial-line)] bg-white p-5">
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <div>
                      <div className="text-lg font-semibold text-[var(--color-editorial-ink)]">{detail.template.key}</div>
                      <div className="text-sm text-[var(--color-editorial-ink-soft)]">{detail.template.description || detail.template.purpose}</div>
                    </div>
                    <button
                      type="button"
                      onClick={() => handleActivate(null)}
                      disabled={saving}
                      className="rounded-full border border-[var(--color-editorial-line)] px-4 py-2 text-sm"
                    >
                      {t("promptAdmin.useDefault")}
                    </button>
                  </div>
                </div>

                <div className="rounded-3xl border border-[var(--color-editorial-line)] bg-white p-5">
                  <div className="mb-4 text-sm font-semibold text-[var(--color-editorial-ink)]">{t("promptAdmin.versions")}</div>
                  <div className="space-y-3">
                    {versionOptions.map((version) => {
                      const active = detail.template.active_version_id === version.id;
                      return (
                        <div key={version.id} className="rounded-2xl border border-[var(--color-editorial-line)] p-4">
                          <div className="flex flex-wrap items-center justify-between gap-3">
                            <div>
                              <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">
                                v{version.version} {version.label ? `- ${version.label}` : ""}
                              </div>
                              <div className="text-xs text-[var(--color-editorial-ink-soft)]">{version.created_by_email || "system"}</div>
                            </div>
                            <button
                              type="button"
                              onClick={() => handleActivate(version.id)}
                              disabled={saving}
                              className={`rounded-full px-4 py-2 text-sm ${active ? "bg-[var(--color-editorial-ink)] text-white" : "border border-[var(--color-editorial-line)]"}`}
                            >
                              {active ? t("promptAdmin.active") : t("promptAdmin.activate")}
                            </button>
                          </div>
                          <pre className="mt-3 max-h-56 overflow-auto rounded-2xl bg-[#f7f4ee] p-3 text-xs whitespace-pre-wrap">{version.prompt_text}</pre>
                        </div>
                      );
                    })}
                  </div>
                </div>

                <div className="rounded-3xl border border-[var(--color-editorial-line)] bg-white p-5">
                  <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("promptAdmin.newVersion")}</div>
                      <div className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">{detail.default_template.notes}</div>
                    </div>
                    <button
                      type="button"
                      onClick={() => setPreviewOpen(true)}
                      className="rounded-full border border-[var(--color-editorial-line)] px-4 py-2 text-sm text-[var(--color-editorial-ink)]"
                    >
                      {t("promptAdmin.openRenderedPreview")}
                    </button>
                  </div>
                  <div className="grid gap-3">
                    <label className="space-y-1.5">
                      <span className="block text-sm font-medium text-[var(--color-editorial-ink)]">{t("promptAdmin.label")}</span>
                      <input className="w-full rounded-2xl border border-[var(--color-editorial-line)] px-4 py-3 text-sm" placeholder={t("promptAdmin.label")} value={form.label} onChange={(e) => setForm((cur) => ({ ...cur, label: e.target.value }))} />
                    </label>
                    <label className="space-y-1.5">
                      <span className="block text-sm font-medium text-[var(--color-editorial-ink)]">{t("promptAdmin.systemInstruction")}</span>
                      <textarea className="min-h-48 w-full resize-y rounded-2xl border border-[var(--color-editorial-line)] px-4 py-3 text-sm" placeholder={t("promptAdmin.systemInstruction")} value={form.system_instruction} onChange={(e) => setForm((cur) => ({ ...cur, system_instruction: e.target.value }))} />
                    </label>
                    <label className="space-y-1.5">
                      <span className="block text-sm font-medium text-[var(--color-editorial-ink)]">{t("promptAdmin.promptText")}</span>
                      <textarea className="min-h-[36rem] w-full resize-y rounded-2xl border border-[var(--color-editorial-line)] px-4 py-3 text-sm" placeholder={t("promptAdmin.promptText")} value={form.prompt_text} onChange={(e) => setForm((cur) => ({ ...cur, prompt_text: e.target.value }))} />
                    </label>
                    <label className="space-y-1.5">
                      <span className="block text-sm font-medium text-[var(--color-editorial-ink)]">{t("promptAdmin.fallbackPromptText")}</span>
                      <textarea className="min-h-40 w-full resize-y rounded-2xl border border-[var(--color-editorial-line)] px-4 py-3 text-sm" placeholder={t("promptAdmin.fallbackPromptText")} value={form.fallback_prompt_text} onChange={(e) => setForm((cur) => ({ ...cur, fallback_prompt_text: e.target.value }))} />
                    </label>
                    <label className="space-y-1.5">
                      <span className="block text-sm font-medium text-[var(--color-editorial-ink)]">{t("promptAdmin.variablesSchema")}</span>
                      <textarea className="min-h-48 w-full resize-y rounded-2xl border border-[var(--color-editorial-line)] px-4 py-3 text-sm font-mono" placeholder={t("promptAdmin.variablesSchema")} value={form.variables_schema} onChange={(e) => setForm((cur) => ({ ...cur, variables_schema: e.target.value }))} />
                    </label>
                    <label className="space-y-1.5">
                      <span className="block text-sm font-medium text-[var(--color-editorial-ink)]">{t("promptAdmin.notes")}</span>
                      <textarea className="min-h-32 w-full resize-y rounded-2xl border border-[var(--color-editorial-line)] px-4 py-3 text-sm" placeholder={t("promptAdmin.notes")} value={form.notes} onChange={(e) => setForm((cur) => ({ ...cur, notes: e.target.value }))} />
                    </label>
                    <button type="button" onClick={handleCreateVersion} disabled={saving} className="rounded-full bg-[var(--color-editorial-ink)] px-4 py-3 text-sm text-white">
                      {t("promptAdmin.createVersion")}
                    </button>
                  </div>
                </div>

                <div className="rounded-3xl border border-[var(--color-editorial-line)] bg-white p-5">
                  <div className="mb-4 text-sm font-semibold text-[var(--color-editorial-ink)]">{t("promptAdmin.experiments")}</div>
                  <div className="mb-5 grid gap-3 md:grid-cols-2">
                    <label className="space-y-1.5">
                      <span className="block text-sm font-medium text-[var(--color-editorial-ink)]">{t("promptAdmin.experimentName")}</span>
                      <input className="w-full rounded-2xl border border-[var(--color-editorial-line)] px-4 py-3 text-sm" placeholder={t("promptAdmin.experimentName")} value={experimentForm.name} onChange={(e) => setExperimentForm((cur) => ({ ...cur, name: e.target.value }))} />
                    </label>
                    <label className="space-y-1.5">
                      <span className="block text-sm font-medium text-[var(--color-editorial-ink)]">{t("promptAdmin.assignmentUnit")}</span>
                      <input className="w-full rounded-2xl border border-[var(--color-editorial-line)] px-4 py-3 text-sm" placeholder={t("promptAdmin.assignmentUnit")} value={experimentForm.assignment_unit} onChange={(e) => setExperimentForm((cur) => ({ ...cur, assignment_unit: e.target.value }))} />
                    </label>
                    <label className="space-y-1.5">
                      <span className="block text-sm font-medium text-[var(--color-editorial-ink)]">{t("promptAdmin.status")}</span>
                      <select className="w-full rounded-2xl border border-[var(--color-editorial-line)] px-4 py-3 text-sm" value={experimentForm.status} onChange={(e) => setExperimentForm((cur) => ({ ...cur, status: e.target.value }))}>
                        <option value="draft">draft</option>
                        <option value="active">active</option>
                        <option value="paused">paused</option>
                      </select>
                    </label>
                    <label className="space-y-1.5">
                      <span className="block text-sm font-medium text-[var(--color-editorial-ink)]">{t("promptAdmin.targetVersion")}</span>
                      <select className="w-full rounded-2xl border border-[var(--color-editorial-line)] px-4 py-3 text-sm" value={experimentForm.version_id} onChange={(e) => setExperimentForm((cur) => ({ ...cur, version_id: e.target.value }))}>
                        <option value="">{t("promptAdmin.selectVersion")}</option>
                        {versionOptions.map((version) => (
                          <option key={version.id} value={version.id}>
                            v{version.version} {version.label}
                          </option>
                        ))}
                      </select>
                    </label>
                    <label className="space-y-1.5">
                      <span className="block text-sm font-medium text-[var(--color-editorial-ink)]">{t("promptAdmin.weight")}</span>
                      <input className="w-full rounded-2xl border border-[var(--color-editorial-line)] px-4 py-3 text-sm" placeholder={t("promptAdmin.weight")} value={experimentForm.weight} onChange={(e) => setExperimentForm((cur) => ({ ...cur, weight: e.target.value }))} />
                    </label>
                  </div>
                  <button type="button" onClick={handleCreateExperiment} disabled={saving} className="mb-5 rounded-full bg-[var(--color-editorial-ink)] px-4 py-3 text-sm text-white">
                    {t("promptAdmin.createExperiment")}
                  </button>
                  <div className="space-y-3">
                    {(detail.experiments ?? []).map((experiment) => {
                      const arms = experimentArmsByExperimentId.get(experiment.id) ?? [];
                      return (
                        <div key={experiment.id} className="rounded-2xl border border-[var(--color-editorial-line)] p-4">
                          <div className="flex flex-wrap items-center justify-between gap-3">
                            <div>
                              <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{experiment.name}</div>
                              <div className="text-xs text-[var(--color-editorial-ink-soft)]">
                                {experiment.status} / {experiment.assignment_unit}
                              </div>
                            </div>
                            <div className="flex gap-2">
                              <button type="button" onClick={() => handleExperimentStatus(experiment, "active")} className="rounded-full border border-[var(--color-editorial-line)] px-3 py-2 text-xs">{t("promptAdmin.activate")}</button>
                              <button type="button" onClick={() => handleExperimentStatus(experiment, "paused")} className="rounded-full border border-[var(--color-editorial-line)] px-3 py-2 text-xs">{t("promptAdmin.pause")}</button>
                            </div>
                          </div>
                          <div className="mt-3 text-xs text-[var(--color-editorial-ink-soft)]">
                            {arms.map((arm) => {
                              const version = versionOptions.find((entry) => entry.id === arm.version_id);
                              return `v${version?.version ?? "?"}(${arm.weight})`;
                            }).join(", ")}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>
              </>
            ) : null}
          </section>
        </div>
      </div>
      {previewOpen ? (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6"
          onClick={() => setPreviewOpen(false)}
        >
          <div
            className="flex max-h-[90vh] w-full max-w-6xl flex-col overflow-hidden rounded-3xl border border-zinc-200 bg-white shadow-2xl"
            onClick={(event) => event.stopPropagation()}
          >
            <div className="flex items-start justify-between gap-4 border-b border-zinc-200 px-5 py-4">
              <div>
                <h2 className="text-base font-semibold text-zinc-900">{t("promptAdmin.renderedPreview")}</h2>
                <p className="mt-1 text-sm text-zinc-500">{t("promptAdmin.renderedPreviewDescription")}</p>
              </div>
              <button
                type="button"
                onClick={() => setPreviewOpen(false)}
                className="rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-sm font-medium text-zinc-700 hover:border-zinc-400 hover:text-zinc-900"
              >
                {t("common.close")}
              </button>
            </div>
            <div className="grid gap-4 overflow-auto px-5 py-4 xl:grid-cols-2">
              <div className="space-y-2">
                <div className="text-xs font-semibold uppercase tracking-wide text-[var(--color-editorial-ink-soft)]">{t("promptAdmin.systemInstruction")}</div>
                <pre className="max-h-[70vh] overflow-auto rounded-2xl bg-[#f7f4ee] p-3 text-xs whitespace-pre-wrap">{renderedSystemInstruction}</pre>
              </div>
              <div className="space-y-2">
                <div className="text-xs font-semibold uppercase tracking-wide text-[var(--color-editorial-ink-soft)]">{t("promptAdmin.promptText")}</div>
                <pre className="max-h-[70vh] overflow-auto rounded-2xl bg-[#f7f4ee] p-3 text-xs whitespace-pre-wrap">{renderedPromptText}</pre>
              </div>
            </div>
          </div>
        </div>
      ) : null}
    </PageTransition>
  );
}
