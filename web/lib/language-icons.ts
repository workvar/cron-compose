import type { SelectOption } from "./ui-helpers";

// Best-effort logo lookup for the Clone paths language list. Slugs match Simple
// Icons (https://simpleicons.org); languages we don't have a good slug for (or a
// custom one Yash types in) just fall back to an initials badge in the UI.
export const LANGUAGE_ICON_SLUGS: Record<string, string> = {
  node: "nodedotjs",
  python: "python",
  go: "go",
  rust: "rust",
  ruby: "ruby",
  php: "php",
  elixir: "elixir",
  java: "openjdk",
  docker: "docker",
  deno: "deno",
  bun: "bun",
  dotnet: "dotnet",
  csharp: "csharp",
  typescript: "typescript",
  javascript: "javascript",
  kotlin: "kotlin",
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

// Suggestions offered when adding a language. Order roughly by how common they
// are for CronCompose's deploy targets; "unknown" is the catch-all bucket the
// backend already falls back to and isn't offered here.
export const LANGUAGE_SUGGESTIONS: Array<{ value: string; label: string }> = [
  { value: "node", label: "Node.js" },
  { value: "python", label: "Python" },
  { value: "go", label: "Go" },
  { value: "rust", label: "Rust" },
  { value: "ruby", label: "Ruby" },
  { value: "php", label: "PHP" },
  { value: "elixir", label: "Elixir" },
  { value: "java", label: "Java" },
  { value: "docker", label: "Docker" },
  { value: "deno", label: "Deno" },
  { value: "bun", label: "Bun" },
  { value: "dotnet", label: ".NET" },
  { value: "typescript", label: "TypeScript" },
  { value: "kotlin", label: "Kotlin" },
  { value: "swift", label: "Swift" },
  { value: "scala", label: "Scala" },
  { value: "haskell", label: "Haskell" },
  { value: "dart", label: "Dart" },
  { value: "erlang", label: "Erlang" },
  { value: "zig", label: "Zig" },
  { value: "crystal", label: "Crystal" },
];

export function languageIconUrl(lang: string): string | null {
  const slug = LANGUAGE_ICON_SLUGS[lang.toLowerCase()];
  return slug ? `https://cdn.simpleicons.org/${slug}` : null;
}

// GitHub's repo-listing `language` field uses its own vocabulary (GitHub Linguist
// names: "Go", "TypeScript", "C++", ...), not CronCompose's internal language ids
// above, so it gets its own slug map rather than overloading LANGUAGE_ICON_SLUGS.
// Used only for the import list's repo icon; deploy config still goes through
// Detect/DetectAt on the actual files, never this best-effort label.
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
  const hit = LANGUAGE_SUGGESTIONS.find((s) => s.value === lang.toLowerCase());
  return hit?.label ?? lang;
}

// The framework/language picker on an app block: every suggestion, each with its
// logo, plus the current value itself when it's something CronCompose detected
// that isn't in the curated list (so "Detected: rust" still shows up instead of
// silently vanishing from the dropdown). allowCustom on the SearchableSelect
// covers anything typed that matches neither.
export function languageSelectOptions(current?: string): SelectOption[] {
  const options: SelectOption[] = LANGUAGE_SUGGESTIONS.map((s) => ({
    value: s.value,
    label: s.label,
    icon: languageIconUrl(s.value),
  }));
  const val = (current || "").trim().toLowerCase();
  if (val && val !== "unknown" && !options.some((o) => o.value === val)) {
    options.unshift({ value: val, label: current!.trim(), icon: languageIconUrl(val) });
  }
  return options;
}
