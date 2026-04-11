"use client";

import { type FormEvent } from "react";

type ToastTone = "success" | "error";

type ShowToast = (message: string, tone: ToastTone) => void;

type RunSavingActionArgs = {
  setSaving: (saving: boolean) => void;
  run: () => Promise<unknown>;
  showToast: ShowToast;
  successMessage?: string;
  mapError?: (error: unknown) => string;
};

export async function runSavingAction({
  setSaving,
  run,
  showToast,
  successMessage,
  mapError,
}: RunSavingActionArgs) {
  setSaving(true);
  try {
    await run();
    if (successMessage) {
      showToast(successMessage, "success");
    }
  } catch (error) {
    showToast(mapError ? mapError(error) : String(error), "error");
  } finally {
    setSaving(false);
  }
}

type SubmitSavingFormActionArgs = RunSavingActionArgs & {
  event: FormEvent;
};

export async function submitSavingFormAction({
  event,
  ...rest
}: SubmitSavingFormActionArgs) {
  event.preventDefault();
  await runSavingAction(rest);
}

type ConfirmOptions = {
  title: string;
  message: string;
  confirmLabel: string;
  tone: "danger";
};

type RunConfirmedActionArgs = RunSavingActionArgs & {
  confirm: (options: ConfirmOptions) => Promise<boolean>;
  confirmOptions: ConfirmOptions;
};

export async function runConfirmedAction({
  confirm,
  confirmOptions,
  ...rest
}: RunConfirmedActionArgs) {
  if (!(await confirm(confirmOptions))) {
    return;
  }
  await runSavingAction(rest);
}
