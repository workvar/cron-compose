package deploy

import "strings"

// defaultCleanup returns a post-build cleanup script for a known framework /
// language when the project did not set one. Keeps runtime artifacts; drops
// source trees and caches that are not needed to serve.
func defaultCleanup(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "nextjs":
		return "rm -rf .git .github node_modules/.cache .next/cache .turbo"
	case "nestjs":
		return "rm -rf .git .github src test node_modules/.cache .turbo *.md"
	case "react", "vue", "nuxt", "astro", "sveltekit", "remix":
		return "rm -rf .git .github src node_modules/.cache .turbo *.md"
	case "express", "node", "typescript", "javascript", "bun":
		return "rm -rf .git .github node_modules/.cache *.md"
	case "go", "golang":
		return "rm -rf .git .github *.md"
	case "rust":
		return "rm -rf .git .github src target/debug *.md"
	case "dotnet", "csharp", "aspnet", "aspnetcore":
		return "rm -rf .git .github *.md"
	case "fastapi", "django", "flask", "python":
		return "rm -rf .git .github __pycache__ .pytest_cache *.md"
	case "rails", "ruby":
		return "rm -rf .git .github tmp/cache log/*.log *.md"
	case "laravel", "php":
		return "rm -rf .git .github tests *.md"
	case "spring", "java", "kotlin":
		return "rm -rf .git .github src *.md"
	case "elixir":
		return "rm -rf .git .github *_test.exs *.md"
	case "deno":
		return "rm -rf .git .github *.md"
	case "docker":
		return ""
	default:
		return "rm -rf .git .github"
	}
}

func cleanupScriptFor(appCleanup, language string) string {
	if s := strings.TrimSpace(appCleanup); s != "" {
		return s
	}
	return defaultCleanup(language)
}
