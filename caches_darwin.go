//go:build darwin

package main

// platformCacheDefs returns the curated cache registry for macOS.
// Only cache subdirectories — never profiles, user data or message stores.
func platformCacheDefs() []cacheDef {
	return []cacheDef{
		// Dev tools
		{Name: "npm cache", Paths: []string{"~/.npm/_cacache"}},
		{Name: "yarn cache", Paths: []string{"~/Library/Caches/Yarn"}},
		{Name: "pnpm store", Paths: []string{"~/Library/pnpm/store", "~/.pnpm-store"}},
		{Name: "pip cache", Paths: []string{"~/Library/Caches/pip"}},
		{Name: "Go build cache", Paths: []string{"~/Library/Caches/go-build"}},
		{Name: "Cargo registry cache", Paths: []string{"~/.cargo/registry/cache"}},
		{Name: "Gradle caches", Paths: []string{"~/.gradle/caches"}},
		{Name: "Homebrew cache", Paths: []string{"~/Library/Caches/Homebrew"}},
		{Name: "Composer cache", Paths: []string{"~/Library/Caches/composer"}},
		{Name: "Hugging Face hub", Paths: []string{"~/.cache/huggingface"}},
		{Name: "CocoaPods cache", Paths: []string{"~/Library/Caches/CocoaPods"}},
		{Name: "Xcode DerivedData", Paths: []string{"~/Library/Developer/Xcode/DerivedData"}},
		{Name: "JetBrains caches", Paths: []string{"~/Library/Caches/JetBrains"}},
		{Name: "VS Code cache", Paths: []string{
			"~/Library/Application Support/Code/Cache",
			"~/Library/Application Support/Code/CachedData",
		}},
		// Browsers (cache dirs only)
		{Name: "Chrome cache", Paths: []string{"~/Library/Caches/Google/Chrome"}},
		{Name: "Chromium cache", Paths: []string{"~/Library/Caches/Chromium"}},
		{Name: "Firefox cache", Paths: []string{"~/Library/Caches/Firefox/Profiles/*"}},
		{Name: "Safari cache", Paths: []string{"~/Library/Caches/com.apple.Safari"}},
		{Name: "Edge cache", Paths: []string{"~/Library/Caches/Microsoft Edge"}},
		{Name: "Brave cache", Paths: []string{"~/Library/Caches/BraveSoftware"}},
		{Name: "Opera cache", Paths: []string{"~/Library/Caches/com.operasoftware.Opera"}},
		{Name: "Arc cache", Paths: []string{"~/Library/Caches/Arc"}},
		{Name: "Yandex Browser cache", Paths: []string{"~/Library/Caches/Yandex/YandexBrowser"}},
		// Messengers (media/web caches only)
		{Name: "Telegram cache", Paths: []string{
			"~/Library/Group Containers/*.keepcoder.Telegram/appstore/account-*/postbox/media",
			"~/Library/Caches/ru.keepcoder.Telegram",
			"~/Library/Application Support/Telegram Desktop/tdata/user_data*/media_cache",
		}},
		{Name: "WhatsApp cache", Paths: []string{
			"~/Library/Caches/net.whatsapp.WhatsApp",
			"~/Library/Group Containers/*.net.whatsapp.WhatsApp*/Library/Caches",
		}},
		{Name: "Slack cache", Paths: []string{
			"~/Library/Application Support/Slack/Cache",
			"~/Library/Application Support/Slack/Service Worker/CacheStorage",
			"~/Library/Caches/com.tinyspeck.slackmacgap",
		}},
		{Name: "Discord cache", Paths: []string{
			"~/Library/Application Support/discord/Cache",
			"~/Library/Application Support/discord/Code Cache",
		}},
		// Popular apps
		{Name: "Spotify cache", Paths: []string{"~/Library/Caches/com.spotify.client"}},
		{Name: "Microsoft Teams cache", Paths: []string{
			"~/Library/Caches/com.microsoft.teams2",
			"~/Library/Caches/com.microsoft.teams",
		}},
		{Name: "Zoom cache", Paths: []string{"~/Library/Caches/us.zoom.xos"}},
	}
}
