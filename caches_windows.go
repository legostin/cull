//go:build windows

package main

import (
	"os"
	"path/filepath"
)

// platformCacheDefs returns the curated cache registry for Windows.
// Only cache subdirectories — never profiles, user data or message stores.
// Entries under an unset env var are dropped.
func platformCacheDefs() []cacheDef {
	local := os.Getenv("LOCALAPPDATA")
	roam := os.Getenv("APPDATA")
	j := filepath.Join

	var defs []cacheDef
	add := func(base, name string, rel ...string) {
		if base == "" {
			return
		}
		paths := make([]string, 0, len(rel))
		for _, r := range rel {
			paths = append(paths, j(base, r))
		}
		defs = append(defs, cacheDef{Name: name, Paths: paths})
	}

	// Dev tools
	add(local, "npm cache", "npm-cache")
	add(local, "pip cache", j("pip", "cache"))
	add(local, "Go build cache", "go-build")
	add(local, "yarn cache", j("Yarn", "Cache"))
	add(local, "Composer cache", "Composer")
	add(local, "JetBrains caches", "JetBrains")
	defs = append(defs,
		cacheDef{Name: "Cargo registry cache", Paths: []string{"~/.cargo/registry/cache"}},
		cacheDef{Name: "Gradle caches", Paths: []string{"~/.gradle/caches"}},
		cacheDef{Name: "pnpm store", Paths: []string{"~/.pnpm-store"}},
		cacheDef{Name: "Hugging Face hub", Paths: []string{"~/.cache/huggingface"}},
	)
	add(roam, "VS Code cache", j("Code", "Cache"), j("Code", "CachedData"))
	// Browsers (cache dirs only; * matches Default and Profile N)
	add(local, "Chrome cache",
		j("Google", "Chrome", "User Data", "*", "Cache"),
		j("Google", "Chrome", "User Data", "*", "Code Cache"))
	add(local, "Edge cache",
		j("Microsoft", "Edge", "User Data", "*", "Cache"),
		j("Microsoft", "Edge", "User Data", "*", "Code Cache"))
	add(local, "Firefox cache", j("Mozilla", "Firefox", "Profiles", "*", "cache2"))
	add(local, "Brave cache",
		j("BraveSoftware", "Brave-Browser", "User Data", "*", "Cache"))
	add(local, "Opera cache", j("Opera Software", "Opera Stable", "Cache"))
	add(local, "Yandex Browser cache",
		j("Yandex", "YandexBrowser", "User Data", "*", "Cache"))
	// Messengers (media/web caches only)
	add(roam, "Telegram cache", j("Telegram Desktop", "tdata", "user_data*", "media_cache"))
	add(local, "WhatsApp cache", j("WhatsApp", "Cache"))
	add(roam, "Slack cache", j("Slack", "Cache"), j("Slack", "Service Worker", "CacheStorage"))
	add(roam, "Discord cache", j("discord", "Cache"), j("discord", "Code Cache"))
	// Popular apps
	add(local, "Spotify cache", j("Spotify", "Storage"), j("Spotify", "Data"))
	add(roam, "Zoom cache", j("Zoom", "data", "cache"))

	return defs
}
