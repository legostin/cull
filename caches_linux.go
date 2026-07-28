//go:build linux

package main

// platformCacheDefs returns the curated cache registry for Linux.
// Only cache subdirectories — never profiles, user data or message stores.
// WhatsApp has no official Linux client, so it has no entry here.
func platformCacheDefs() []cacheDef {
	return []cacheDef{
		// Dev tools
		{Name: "npm cache", Paths: []string{"~/.npm/_cacache"}},
		{Name: "yarn cache", Paths: []string{"~/.cache/yarn"}},
		{Name: "pnpm store", Paths: []string{"~/.local/share/pnpm/store", "~/.pnpm-store"}},
		{Name: "pip cache", Paths: []string{"~/.cache/pip"}},
		{Name: "Go build cache", Paths: []string{"~/.cache/go-build"}},
		{Name: "Cargo registry cache", Paths: []string{"~/.cargo/registry/cache"}},
		{Name: "Gradle caches", Paths: []string{"~/.gradle/caches"}},
		{Name: "Homebrew cache", Paths: []string{"~/.cache/Homebrew"}},
		{Name: "Composer cache", Paths: []string{"~/.cache/composer"}},
		{Name: "Hugging Face hub", Paths: []string{"~/.cache/huggingface"}},
		{Name: "JetBrains caches", Paths: []string{"~/.cache/JetBrains"}},
		{Name: "VS Code cache", Paths: []string{"~/.config/Code/Cache", "~/.config/Code/CachedData"}},
		// Browsers (cache dirs only)
		{Name: "Chrome cache", Paths: []string{"~/.cache/google-chrome"}},
		{Name: "Chromium cache", Paths: []string{"~/.cache/chromium"}},
		{Name: "Firefox cache", Paths: []string{"~/.cache/mozilla/firefox/*"}},
		{Name: "Edge cache", Paths: []string{"~/.cache/microsoft-edge"}},
		{Name: "Brave cache", Paths: []string{"~/.cache/BraveSoftware"}},
		{Name: "Opera cache", Paths: []string{"~/.cache/opera"}},
		{Name: "Yandex Browser cache", Paths: []string{"~/.cache/yandex-browser"}},
		// Messengers (media/web caches only)
		{Name: "Telegram cache", Paths: []string{
			"~/.local/share/TelegramDesktop/tdata/user_data*/media_cache",
		}},
		{Name: "Slack cache", Paths: []string{
			"~/.config/Slack/Cache",
			"~/.config/Slack/Service Worker/CacheStorage",
		}},
		{Name: "Discord cache", Paths: []string{
			"~/.config/discord/Cache",
			"~/.config/discord/Code Cache",
		}},
		// Popular apps
		{Name: "Spotify cache", Paths: []string{"~/.cache/spotify"}},
		{Name: "Zoom cache", Paths: []string{"~/.cache/zoom"}},
	}
}
