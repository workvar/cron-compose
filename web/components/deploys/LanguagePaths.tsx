"use client";

import { useMemo, useState } from "react";
import type { DeploySettings } from "@/lib/types";
import { SearchableSelect } from "@/components/SearchableSelect";
import { LANGUAGE_SUGGESTIONS, languageIconUrl, languageLabel } from "@/lib/language-icons";
import { IconPlus } from "@/components/icons";

// Falls back to an initials badge for anything Simple Icons doesn't have a slug
// for (Java, an unlisted language, or whatever custom name Yash types in).
function LanguageIcon({ lang }: { lang: string }) {
  const [broken, setBroken] = useState(false);
  const src = languageIconUrl(lang);
  if (!src || broken) {
    return (
      <span className="lang-icon lang-icon-fallback" aria-hidden>
        {lang.slice(0, 2).toUpperCase()}
      </span>
    );
  }
  return (
    <span className="lang-icon">
      <img src={src} alt="" width={20} height={20} onError={() => setBroken(true)} />
    </span>
  );
}

export function LanguagePaths({ initial }: { initial: DeploySettings }) {
  const [paths, setPaths] = useState(initial.language_paths);
  const [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [newLang, setNewLang] = useState("");

  const languages = useMemo(
    () => Object.keys(paths).sort((a, b) => a.localeCompare(b)),
    [paths],
  );
  const addOptions = useMemo(
    () => LANGUAGE_SUGGESTIONS.filter((s) => !(s.value in paths)),
    [paths],
  );

  function addLanguage() {
    const key = newLang.trim().toLowerCase();
    if (!key || key in paths) return;
    setPaths((p) => ({ ...p, [key]: `/opt/apps/${key}` }));
    setNewLang("");
    setSaved(false);
  }

  function removeLanguage(lang: string) {
    setPaths((p) => {
      const next = { ...p };
      delete next[lang];
      return next;
    });
    setSaved(false);
  }

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    setSaved(false);
    try {
      const res = await fetch("/api/deploy-settings", {
        method: "PUT",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ language_paths: paths }),
      });
      if (!res.ok) {
        const txt = await res.text();
        throw new Error(txt || `HTTP ${res.status}`);
      }
      const next = (await res.json()) as DeploySettings;
      setPaths(next.language_paths);
      setSaved(true);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={save} className="panel">
      <p className="subtle" style={{ marginTop: 0 }}>
        Repos clone into <code>{"{path}/{repo}"}</code> on the selected agent. Override per deploy if needed.
      </p>

      <div className="lang-grid">
        {languages.map((lang) => (
          <div className="lang-card" key={lang}>
            <LanguageIcon lang={lang} />
            <div className="lang-card-body">
              <label htmlFor={`path-${lang}`}>{languageLabel(lang)}</label>
              <input
                id={`path-${lang}`}
                value={paths[lang]}
                onChange={(e) => setPaths((p) => ({ ...p, [lang]: e.target.value }))}
              />
            </div>
            <button
              type="button"
              className="lang-remove"
              onClick={() => removeLanguage(lang)}
              aria-label={`Remove ${languageLabel(lang)}`}
              title={`Remove ${languageLabel(lang)}`}
            >
              ×
            </button>
          </div>
        ))}
      </div>

      <div className="lang-add-row">
        <SearchableSelect
          value={newLang}
          onChange={setNewLang}
          options={addOptions}
          placeholder="Add a language…"
          allowCustom
          className="lang-add-select"
          aria-label="Add a language"
        />
        <button type="button" className="button secondary sm" onClick={addLanguage} disabled={!newLang.trim()}>
          <IconPlus /> Add
        </button>
      </div>

      {error && <p className="form-error">{error}</p>}
      <button type="submit" className="button" disabled={busy} style={{ marginTop: 14 }}>
        {busy ? "Saving…" : saved ? "Saved" : "Save paths"}
      </button>
    </form>
  );
}
