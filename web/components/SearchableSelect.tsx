"use client";

import { useEffect, useId, useMemo, useRef, useState } from "react";
import { filterSelectOptions, type SelectOption } from "@/lib/ui-helpers";
import { IconChevronDown } from "./icons";

export type { SelectOption };

type Props = {
  value: string;
  onChange: (value: string) => void;
  options: SelectOption[];
  disabled?: boolean;
  placeholder?: string;
  id?: string;
  allowCustom?: boolean;
  className?: string;
  "aria-label"?: string;
};

export function SearchableSelect({
  value,
  onChange,
  options,
  disabled,
  placeholder = "Select…",
  id,
  allowCustom,
  className,
  "aria-label": ariaLabel,
}: Props) {
  const autoId = useId();
  const inputId = id ?? autoId;
  const wrapRef = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [typing, setTyping] = useState(false);
  const [active, setActive] = useState(0);

  const selected = options.find((o) => o.value === value);
  const display = open && typing ? query : (selected?.label ?? (allowCustom ? value : ""));
  const filtered = useMemo(
    () => filterSelectOptions(options, open && typing ? query : ""),
    [options, open, typing, query],
  );

  useEffect(() => {
    if (!open) return;
    const onDoc = (e: MouseEvent) => {
      if (!wrapRef.current?.contains(e.target as Node)) close();
    };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [open]);

  useEffect(() => {
    setActive(0);
  }, [query, open]);

  function close() {
    setOpen(false);
    setTyping(false);
    setQuery("");
  }

  function commit(v: string) {
    onChange(v);
    close();
  }

  function onKeyDown(e: React.KeyboardEvent) {
    if (!open && e.key === "ArrowDown") {
      e.preventDefault();
      setOpen(true);
      return;
    }
    if (!open) return;
    if (e.key === "Escape") {
      e.preventDefault();
      close();
      return;
    }
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActive((i) => Math.min(i + 1, Math.max(filtered.length - 1, 0)));
      return;
    }
    if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive((i) => Math.max(i - 1, 0));
      return;
    }
    if (e.key === "Enter") {
      e.preventDefault();
      const hit = filtered[active];
      if (hit) commit(hit.value);
      else if (allowCustom && query.trim()) commit(query.trim());
    }
  }

  return (
    <div className={`ss${className ? ` ${className}` : ""}`} ref={wrapRef}>
      <div className="ss-field">
        <input
          id={inputId}
          className="ss-input"
          value={display}
          disabled={disabled}
          placeholder={placeholder}
          autoComplete="off"
          role="combobox"
          aria-expanded={open}
          aria-controls={`${inputId}-list`}
          aria-autocomplete="list"
          aria-label={ariaLabel}
          onChange={(e) => {
            setTyping(true);
            setQuery(e.target.value);
            setOpen(true);
          }}
          onFocus={() => setOpen(true)}
          onKeyDown={onKeyDown}
        />
        <button
          type="button"
          className="ss-chevron"
          tabIndex={-1}
          disabled={disabled}
          aria-label="Toggle options"
          onClick={() => (open ? close() : setOpen(true))}
        >
          <IconChevronDown />
        </button>
      </div>
      {open && !disabled && (
        <ul id={`${inputId}-list`} role="listbox" className="ss-list">
          {filtered.length === 0 && (
            <li className="ss-empty">
              {allowCustom && query.trim() ? (
                <button type="button" className="ss-opt" onClick={() => commit(query.trim())}>
                  Use “{query.trim()}”
                </button>
              ) : (
                "No matches"
              )}
            </li>
          )}
          {filtered.map((o, i) => (
            <li key={o.value}>
              <button
                type="button"
                role="option"
                aria-selected={o.value === value}
                className={`ss-opt${i === active ? " on" : ""}${o.value === value ? " selected" : ""}`}
                onMouseEnter={() => setActive(i)}
                onClick={() => commit(o.value)}
              >
                {o.label}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
