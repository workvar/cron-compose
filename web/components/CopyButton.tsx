"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { IconClipboard, IconCheck } from "@/components/icons";
import { copyText } from "@/lib/clipboard";

type Props = {
  value: string;
  label?: string;
  className?: string;
};

/** Small button that copies `value` and briefly confirms. */
export default function CopyButton({ value, label = "Copy", className }: Props) {
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => () => {
    if (timer.current) clearTimeout(timer.current);
  }, []);

  const onCopy = useCallback(async () => {
    const ok = await copyText(value);
    if (!ok) return;
    setCopied(true);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => setCopied(false), 1600);
  }, [value]);

  return (
    <button
      type="button"
      onClick={onCopy}
      className={`copy-btn${className ? ` ${className}` : ""}`}
      aria-label={copied ? "Copied" : label}
      title={copied ? "Copied" : label}
    >
      {copied ? <IconCheck /> : <IconClipboard />}
      <span>{copied ? "Copied" : label}</span>
    </button>
  );
}
