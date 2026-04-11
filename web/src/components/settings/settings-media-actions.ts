"use client";

type ToastTone = "success" | "error";

export async function copyToClipboardAction({
  value,
  writeText,
  showToast,
  successMessage,
}: {
  value: string;
  writeText: (value: string) => Promise<void>;
  showToast: (message: string, tone: ToastTone) => void;
  successMessage: string;
}) {
  if (!value) return;
  try {
    await writeText(value);
    showToast(successMessage, "success");
  } catch (error) {
    showToast(String(error), "error");
  }
}

export async function readFileAsBase64DataURL(file: File): Promise<string> {
  const dataURL = await new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result ?? ""));
    reader.onerror = () => reject(new Error("failed to read artwork file"));
    reader.readAsDataURL(file);
  });
  const marker = "base64,";
  const index = dataURL.indexOf(marker);
  if (index < 0) {
    throw new Error("invalid artwork file");
  }
  return dataURL.slice(index + marker.length);
}
