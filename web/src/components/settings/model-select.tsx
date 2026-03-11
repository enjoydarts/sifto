"use client";

type ModelOption = {
  value: string;
  label: string;
  note?: string;
};

export default function ModelSelect({
  label,
  value,
  onChange,
  options,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  options: ModelOption[];
}) {
  return (
    <div>
      <label className="block text-sm font-medium text-zinc-700">{label}</label>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="mt-1 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900"
      >
        <option value="">Default</option>
        {options.map((opt) => (
          <option key={opt.value} value={opt.value}>
            {opt.note ? `${opt.label} (${opt.note})` : opt.label}
          </option>
        ))}
      </select>
    </div>
  );
}

export type { ModelOption };
