import type { SelectOption } from "./ui-helpers";
import { FRAMEWORKS, RUNTIME_LANGUAGES, frameworkPreset } from "./frameworks";

// Extra slugs for values that aren't framework presets (custom / Linguist aliases).
const EXTRA_ICON_SLUGS: Record<string, string> = {
  nodejs: "nodedotjs",
  javascript: "javascript",
  golang: "go",
  aspnet: "dotnet",
  openjdk: "openjdk",
  swift: "swift",
  scala: "scala",
  perl: "perl",
  lua: "lua",
  r: "r",
  haskell: "haskell",
  clojure: "clojure",
  dart: "dart",
  erlang: "erlang",
  zig: "zig",
  crystal: "crystal",
};

/** @deprecated Prefer FRAMEWORKS — kept for LanguagePaths / clone-path settings. */
export const LANGUAGE_SUGGESTIONS: Array<{ value: string; label: string }> = RUNTIME_LANGUAGES;

export function languageIconUrl(lang: string): string | null {
  const key = lang.toLowerCase();
  const preset = frameworkPreset(key);
  const slug = preset?.icon || (preset ? key : undefined) || EXTRA_ICON_SLUGS[key];
  return slug ? `https://cdn.simpleicons.org/${slug}` : null;
}

// GitHub's repo-listing `language` field uses Linguist names, not CronCompose ids.
const GITHUB_LANGUAGE_ICON_SLUGS: Record<string, string> = {
  go: "go",
  javascript: "javascript",
  typescript: "typescript",
  python: "python",
  ruby: "ruby",
  php: "php",
  java: "openjdk",
  "c#": "csharp",
  rust: "rust",
  dart: "dart",
  kotlin: "kotlin",
  swift: "swift",
  scala: "scala",
  elixir: "elixir",
  haskell: "haskell",
  "c++": "cplusplus",
  c: "c",
  html: "html5",
  css: "css3",
  vue: "vuedotjs",
  shell: "gnubash",
  dockerfile: "docker",
  lua: "lua",
  perl: "perl",
  r: "r",
  zig: "zig",
  crystal: "crystal",
  clojure: "clojure",
  erlang: "erlang",
};

export function githubLanguageIconUrl(lang: string | null | undefined): string | null {
  if (!lang) return null;
  const slug = GITHUB_LANGUAGE_ICON_SLUGS[lang.toLowerCase()];
  return slug ? `https://cdn.simpleicons.org/${slug}` : null;
}

export function languageLabel(lang: string): string {
  const key = lang.toLowerCase();
  const framework = FRAMEWORKS.find((s) => s.id === key);
  if (framework) return framework.label;
  const runtime = RUNTIME_LANGUAGES.find((s) => s.value === key);
  return runtime?.label ?? lang;
}

// Framework / language picker: curated presets with logos, plus the current
// value when detection returned something outside the list.
export function languageSelectOptions(current?: string): SelectOption[] {
  const options: SelectOption[] = FRAMEWORKS.map((s) => ({
    value: s.id,
    label: s.label,
    icon: languageIconUrl(s.id),
  }));
  const val = (current || "").trim().toLowerCase();
  if (val && val !== "unknown" && !options.some((o) => o.value === val)) {
    options.unshift({ value: val, label: current!.trim(), icon: languageIconUrl(val) });
  }
  return options;
}
