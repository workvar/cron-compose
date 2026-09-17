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

export function languageLabel(lang: string): string {
  const hit = LANGUAGE_SUGGESTIONS.find((s) => s.value === lang.toLowerCase());
  return hit?.label ?? lang;
}
