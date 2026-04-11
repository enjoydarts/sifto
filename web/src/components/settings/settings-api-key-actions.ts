"use client";

import { type FormEvent } from "react";
import type { AccessProviderID } from "@/components/settings/system-access-cards";

type ToastTone = "success" | "error";

type SubmitAPIKeyActionArgs = {
  event: FormEvent;
  value: string;
  setSaving: (saving: boolean) => void;
  save: (value: string) => Promise<unknown>;
  clearValue: () => void;
  reload: () => Promise<unknown>;
  showToast: (message: string, tone: ToastTone) => void;
  emptyValueMessage: string;
  successMessage: string;
  afterSave?: () => void;
};

export async function submitAPIKeyAction({
  event,
  value,
  setSaving,
  save,
  clearValue,
  reload,
  showToast,
  emptyValueMessage,
  successMessage,
  afterSave,
}: SubmitAPIKeyActionArgs) {
  event.preventDefault();
  setSaving(true);
  try {
    const trimmed = value.trim();
    if (!trimmed) {
      throw new Error(emptyValueMessage);
    }
    await save(trimmed);
    clearValue();
    afterSave?.();
    await reload();
    showToast(successMessage, "success");
  } catch (error) {
    showToast(String(error), "error");
  } finally {
    setSaving(false);
  }
}

type DeleteAPIKeyActionArgs = {
  confirm: (options: { title: string; message: string; confirmLabel: string; tone: "danger" }) => Promise<boolean>;
  title: string;
  message: string;
  confirmLabel: string;
  setDeleting: (deleting: boolean) => void;
  remove: () => Promise<unknown>;
  reload: () => Promise<unknown>;
  showToast: (message: string, tone: ToastTone) => void;
  successMessage: string;
  afterDelete?: () => void;
};

export async function deleteAPIKeyAction({
  confirm,
  title,
  message,
  confirmLabel,
  setDeleting,
  remove,
  reload,
  showToast,
  successMessage,
  afterDelete,
}: DeleteAPIKeyActionArgs) {
  if (!(await confirm({ title, message, confirmLabel, tone: "danger" }))) {
    return;
  }
  setDeleting(true);
  try {
    await remove();
    afterDelete?.();
    await reload();
    showToast(successMessage, "success");
  } catch (error) {
    showToast(String(error), "error");
  } finally {
    setDeleting(false);
  }
}

export type APIKeyActionHandlerMap = Record<
  AccessProviderID,
  {
    submit: (event: FormEvent) => Promise<void>;
    remove: () => Promise<void>;
  }
>;

type APIKeyActionDefinition = {
  value: string;
  setValue: (value: string) => void;
  setSaving: (saving: boolean) => void;
  setDeleting: (deleting: boolean) => void;
  save: (value: string) => Promise<unknown>;
  remove: () => Promise<unknown>;
  deleteTitle: string;
  deleteMessage: string;
  emptyValueMessage: string;
  saveSuccessMessage: string;
  deleteSuccessMessage: string;
  afterSave?: () => void;
  afterDelete?: () => void;
};

type CreateAPIKeyActionHandlersArgs = {
  confirm: (options: { title: string; message: string; confirmLabel: string; tone: "danger" }) => Promise<boolean>;
  confirmLabel: string;
  reload: () => Promise<unknown>;
  showToast: (message: string, tone: ToastTone) => void;
  definitions: Record<AccessProviderID, APIKeyActionDefinition>;
};

export function createAPIKeyActionHandlers({
  confirm,
  confirmLabel,
  reload,
  showToast,
  definitions,
}: CreateAPIKeyActionHandlersArgs): APIKeyActionHandlerMap {
  return Object.fromEntries(
    Object.entries(definitions).map(([provider, definition]) => [
      provider,
      {
        submit: async (event: FormEvent) => {
          await submitAPIKeyAction({
            event,
            value: definition.value,
            setSaving: definition.setSaving,
            save: definition.save,
            clearValue: () => definition.setValue(""),
            reload,
            showToast,
            emptyValueMessage: definition.emptyValueMessage,
            successMessage: definition.saveSuccessMessage,
            afterSave: definition.afterSave,
          });
        },
        remove: async () => {
          await deleteAPIKeyAction({
            confirm,
            title: definition.deleteTitle,
            message: definition.deleteMessage,
            confirmLabel,
            setDeleting: definition.setDeleting,
            remove: definition.remove,
            reload,
            showToast,
            successMessage: definition.deleteSuccessMessage,
            afterDelete: definition.afterDelete,
          });
        },
      },
    ]),
  ) as APIKeyActionHandlerMap;
}
