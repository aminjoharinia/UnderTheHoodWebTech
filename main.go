package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/net/proxy"
	tele "gopkg.in/telebot.v4"
)

// ─── Config ──────────────────────────────────────────────────────────────────

const (
	defaultSocks5 = "127.0.0.1:10808"
)

// ─── Detection Result ────────────────────────────────────────────────────────

type Detection struct {
	Name       string
	Emoji      string // emoji
	Confidence string // "definite" | "likely" | "possible"
	Evidence   string // short human-readable reason
}

// ─── HTTP Fetch ──────────────────────────────────────────────────────────────

type PageData struct {
	HTML    string
	Headers http.Header
	Status  int
}

func fetchPage(url string) (*PageData, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	buf := make([]byte, 512*1024) // read up to 512 KB
	n, _ := resp.Body.Read(buf)

	return &PageData{
		HTML:    string(buf[:n]),
		Headers: resp.Header,
		Status:  resp.StatusCode,
	}, nil
}

// ─── Detection Rules ─────────────────────────────────────────────────────────

func detect(pd *PageData) []Detection {
	h := strings.ToLower(pd.HTML)
	headers := pd.Headers
	var found []Detection

	add := func(name, logo, confidence, evidence string) {
		found = append(found, Detection{name, logo, confidence, evidence})
	}

	// ── Server / CDN headers ──────────────────────────────────────────────────
	if v := headers.Get("X-Powered-By"); v != "" {
		add("X-Powered-By: "+v, "🖥️", "definite", "response header")
	}
	if v := headers.Get("Server"); v != "" {
		add("Server: "+v, "🌐", "definite", "response header")
	}
	if v := headers.Get("X-Generator"); v != "" {
		add("Generator: "+v, "⚙️", "definite", "X-Generator header")
	}
	if v := headers.Get("X-Drupal-Cache"); v != "" {
		add("Drupal", "💧", "definite", "X-Drupal-Cache header")
	}
	if headers.Get("X-WP-Nonce") != "" || headers.Get("X-WP-Total") != "" {
		add("WordPress (REST API)", "🔵", "definite", "WP REST header")
	}

	// ── CDN fingerprints ──────────────────────────────────────────────────────
	cf := headers.Get("CF-Ray")
	if cf != "" {
		add("Cloudflare CDN", "🟠", "definite", "CF-Ray header present")
	}
	if v := headers.Get("X-Vercel-Id"); v != "" {
		add("Vercel Hosting", "▲", "definite", "X-Vercel-Id header")
	}
	if v := headers.Get("X-Amz-Cf-Id"); v != "" {
		add("AWS CloudFront", "☁️", "definite", "X-Amz-Cf-Id header")
	}
	if strings.Contains(headers.Get("Via"), "netlify") ||
		headers.Get("X-Nf-Request-Id") != "" {
		add("Netlify Hosting", "🟢", "definite", "Netlify header")
	}

	// ── Next.js ───────────────────────────────────────────────────────────────
	nextScore := 0
	if strings.Contains(h, `"__next_data__"`) || strings.Contains(h, `id="__next_data__"`) ||
		strings.Contains(pd.HTML, `__NEXT_DATA__`) {
		nextScore += 3
	}
	if strings.Contains(h, "/_next/static/") {
		nextScore += 2
	}
	if strings.Contains(h, `id="__next"`) {
		nextScore++
	}
	if strings.Contains(h, "next/dist/") || strings.Contains(h, "next-route-announcer") {
		nextScore++
	}
	if nextScore >= 3 {
		add("Next.js", "▲", "definite", fmt.Sprintf("score %d/7 signals", nextScore))
	} else if nextScore > 0 {
		add("Next.js", "▲", "likely", fmt.Sprintf("score %d/7 signals", nextScore))
	}

	// ── React ─────────────────────────────────────────────────────────────────
	reactScore := 0
	if strings.Contains(h, `id="root"`) || strings.Contains(h, `id='root'`) {
		reactScore++
	}
	if strings.Contains(h, "react.production.min") || strings.Contains(h, "react.development") {
		reactScore += 2
	}
	if strings.Contains(h, "__react_devtools_global_hook__") {
		reactScore += 2
	}
	if strings.Contains(h, "data-reactroot") || strings.Contains(h, "data-reactid") {
		reactScore += 2
	}
	if strings.Contains(h, "_reactfiber") || strings.Contains(h, "_reactlistening") {
		reactScore += 2
	}
	if reactScore >= 2 {
		add("React", "⚛️", confidenceLabel(reactScore, 2, 4),
			fmt.Sprintf("score %d signals", reactScore))
	}

	// ── Vue / Nuxt ────────────────────────────────────────────────────────────
	if strings.Contains(h, "__nuxt__") || strings.Contains(h, "/_nuxt/") ||
		strings.Contains(h, `id="__nuxt"`) || strings.Contains(h, "nuxt-link") {
		add("Nuxt.js", "💚", "definite", "__nuxt__ / /_nuxt/ path")
	} else if strings.Contains(h, "__vue__") || strings.Contains(h, "vue.runtime") ||
		strings.Contains(h, "data-v-") {
		add("Vue.js", "💚", "definite", "__VUE__ / data-v- attributes")
	} else if strings.Contains(h, "vue") && strings.Contains(h, "v-app") {
		add("Vue.js", "💚", "likely", "vue + v-app combo")
	}

	// ── Svelte / SvelteKit ────────────────────────────────────────────────────
	if strings.Contains(h, "/_app/immutable/") || strings.Contains(h, "__sveltekit") {
		add("SvelteKit", "🔥", "definite", "/_app/immutable/ path")
	} else if strings.Contains(h, "svelte") {
		add("Svelte", "🔥", "likely", "svelte keyword in HTML")
	}

	// ── Astro ─────────────────────────────────────────────────────────────────
	if strings.Contains(h, "astro-island") || strings.Contains(h, "astro-slot") {
		add("Astro", "🚀", "definite", "astro-island element")
	} else if regexp.MustCompile(`/_astro/[a-z0-9]+\.(js|css)`).MatchString(h) {
		add("Astro", "🚀", "likely", "/_astro/ asset path")
	}

	// ── Angular ───────────────────────────────────────────────────────────────
	if strings.Contains(h, "ng-version=") {
		add("Angular", "🔴", "definite", "ng-version attribute")
	} else if strings.Contains(h, "ng-app") || strings.Contains(h, "ng-controller") {
		add("AngularJS (v1)", "🔴", "definite", "ng-app / ng-controller")
	} else if strings.Contains(h, "angularjs") {
		add("AngularJS", "🔴", "likely", "angularjs keyword")
	}

	// ── Flutter Web ───────────────────────────────────────────────────────────
	if strings.Contains(h, "flutter.js") || strings.Contains(h, "main.dart.js") {
		add("Flutter Web", "💙", "definite", "flutter.js / main.dart.js")
	} else if strings.Contains(h, "flt-glass-pane") || strings.Contains(h, "flt-scene-host") {
		add("Flutter Web", "💙", "definite", "flt-glass-pane element")
	}

	// ── WordPress ─────────────────────────────────────────────────────────────
	wpScore := 0
	if strings.Contains(h, "/wp-content/") {
		wpScore += 3
	}
	if strings.Contains(h, "/wp-includes/") {
		wpScore += 2
	}
	if strings.Contains(h, "wp-json") || strings.Contains(h, "wp_head") {
		wpScore++
	}
	if strings.Contains(h, "wordpress") {
		wpScore++
	}
	if wpScore >= 3 {
		add("WordPress", "🔵", "definite", fmt.Sprintf("wp-content/includes score %d", wpScore))
	} else if wpScore > 0 {
		add("WordPress", "🔵", "likely", fmt.Sprintf("partial WP signals %d", wpScore))
	}

	// ── Shopify ───────────────────────────────────────────────────────────────
	if strings.Contains(h, "cdn.shopify.com") || strings.Contains(h, "shopify.com/s/files") {
		add("Shopify", "🛍️", "definite", "cdn.shopify.com asset")
	} else if strings.Contains(h, "shopify") {
		add("Shopify", "🛍️", "likely", "shopify keyword")
	}

	// ── Woocommerce ───────────────────────────────────────────────────────────
	if strings.Contains(h, "woocommerce") {
		add("WooCommerce", "🛒", "definite", "woocommerce class/script")
	}

	// ── Drupal ────────────────────────────────────────────────────────────────
	if strings.Contains(h, "drupal.js") || strings.Contains(h, "drupal-settings-json") ||
		strings.Contains(h, "/sites/default/files/") {
		add("Drupal", "💧", "definite", "drupal.js / settings JSON")
	}

	// ── Joomla ────────────────────────────────────────────────────────────────
	if strings.Contains(h, "/media/jui/") || strings.Contains(h, "joomla") {
		add("Joomla", "🟡", "definite", "/media/jui/ path")
	}

	// ── Laravel ───────────────────────────────────────────────────────────────
	if strings.Contains(h, "laravel_session") || strings.Contains(h, "__inertia") {
		add("Laravel (Inertia)", "🔴", "definite", "laravel_session / __inertia")
	}

	// ── Ruby on Rails ─────────────────────────────────────────────────────────
	if strings.Contains(h, "rails-ujs") || strings.Contains(h, "data-turbo-") ||
		strings.Contains(h, "turbolinks") {
		add("Ruby on Rails", "♦️", "definite", "rails-ujs / turbolinks")
	}

	// ── Django ────────────────────────────────────────────────────────────────
	if strings.Contains(h, "csrfmiddlewaretoken") || strings.Contains(h, "__django_") {
		add("Django", "🌿", "likely", "csrfmiddlewaretoken / django token")
	}

	// ── HTMX ─────────────────────────────────────────────────────────────────
	if strings.Contains(h, "htmx.org") || strings.Contains(h, "hx-get=") ||
		strings.Contains(h, "hx-post=") {
		add("HTMX", "⚡", "definite", "hx-get / hx-post attributes")
	}

	// ── Alpine.js ────────────────────────────────────────────────────────────
	if strings.Contains(h, "x-data=") || strings.Contains(h, "alpinejs") {
		add("Alpine.js", "🏔️", "definite", "x-data attribute / alpinejs")
	}

	// ── Remix ────────────────────────────────────────────────────────────────
	if strings.Contains(h, "__remixcontext") || strings.Contains(h, "__remixmanifest") {
		add("Remix", "💿", "definite", "__remixContext in HTML")
	}

	// ── Gatsby ───────────────────────────────────────────────────────────────
	if strings.Contains(h, "gatsby-") || strings.Contains(h, `"gatsby"`) ||
		strings.Contains(h, "/page-data/") {
		add("Gatsby", "💜", "definite", "gatsby- prefix / /page-data/ path")
	}

	// ── Vite ─────────────────────────────────────────────────────────────────
	if strings.Contains(h, "/@vite/client") || strings.Contains(h, "vite/modulepreload-polyfill") {
		add("Vite", "⚡", "definite", "@vite/client script")
	}

	// ── Tailwind ─────────────────────────────────────────────────────────────
	if regexp.MustCompile(`class="[^"]*\b(flex|grid|px-|py-|text-|bg-|rounded|shadow|w-|h-)[^"]*"`).
		MatchString(pd.HTML) {
		add("Tailwind CSS", "🌬️", "likely", "Tailwind utility classes detected")
	}

	// ── Bootstrap ────────────────────────────────────────────────────────────
	if strings.Contains(h, "bootstrap.min.css") || strings.Contains(h, "bootstrap.bundle") ||
		strings.Contains(h, "cdn.jsdelivr.net/npm/bootstrap") {
		add("Bootstrap", "🅱️", "definite", "bootstrap.min.css / bundle")
	}

	// ── jQuery ────────────────────────────────────────────────────────────────
	if strings.Contains(h, "jquery.min.js") || strings.Contains(h, "jquery-") ||
		strings.Contains(h, "code.jquery.com") {
		add("jQuery", "🔷", "definite", "jquery.min.js / CDN")
	}

	// ── Google Analytics / Tag Manager ───────────────────────────────────────
	if strings.Contains(h, "gtag('config'") || strings.Contains(h, "google-analytics.com/analytics.js") {
		add("Google Analytics", "📊", "definite", "gtag config call")
	}
	if strings.Contains(h, "googletagmanager.com/gtm.js") {
		add("Google Tag Manager", "📊", "definite", "GTM script")
	}

	// ── Wix ───────────────────────────────────────────────────────────────────
	if strings.Contains(h, "static.wixstatic.com") || strings.Contains(h, "wix.com") {
		add("Wix", "🟦", "definite", "wixstatic.com asset")
	}

	// ── Squarespace ───────────────────────────────────────────────────────────
	if strings.Contains(h, "squarespace.com") || strings.Contains(h, "squarespace-cdn.com") {
		add("Squarespace", "⬛", "definite", "squarespace CDN asset")
	}

	// ── Webflow ───────────────────────────────────────────────────────────────
	if strings.Contains(h, "webflow.com") || strings.Contains(h, "data-wf-page") {
		add("Webflow", "🔷", "definite", "data-wf-page / webflow.com")
	}

	// ── Framer ────────────────────────────────────────────────────────────────
	if strings.Contains(h, "framerusercontent.com") || strings.Contains(h, "framer.com/motion") {
		add("Framer", "🖼️", "definite", "framerusercontent.com asset")
	}

	// ── Polymer / Lit (Google Web Components) ────────────────────────────────
	polymerScore := 0
	if strings.Contains(h, "ytd-app") || strings.Contains(h, "ytd-watch") {
		polymerScore += 3
	}
	if strings.Contains(h, "<ytd-") || strings.Contains(h, "<yt-") {
		polymerScore += 2
	}
	if strings.Contains(h, "polymer") || strings.Contains(h, "polymer-element") {
		polymerScore += 2
	}
	if strings.Contains(h, "lit-element") || strings.Contains(h, "lit.js") {
		polymerScore += 2
	}
	if strings.Contains(h, "customelements.min.js") {
		polymerScore++
	}
	if polymerScore >= 3 {
		add("Polymer / Lit (Web Components)", "🧪", "definite",
			fmt.Sprintf("score %d: ytd-*/polymer/lit signals", polymerScore))
	} else if polymerScore > 0 {
		add("Polymer / Lit (Web Components)", "🧪", "likely",
			fmt.Sprintf("score %d web component signals", polymerScore))
	}

	// ── YouTube ───────────────────────────────────────────────────────────────
	if strings.Contains(h, "ytInitialData") || strings.Contains(h, "ytcfg.set") ||
		strings.Contains(h, "ytInitialPlayerResponse") || strings.Contains(h, "<ytd-app") {
		add("YouTube (Google)", "▶️", "definite", "ytInitialData / ytcfg / ytd-app")
	}

	// ── Google Wiz ────────────────────────────────────────────────────────────
	if strings.Contains(h, "wiz_progress") || strings.Contains(h, "data-ved=") {
		add("Google Wiz (Internal)", "🔍", "likely", "wiz / data-ved signals")
	}

	// ── Inertia.js ────────────────────────────────────────────────────────────
	if strings.Contains(h, "data-page=") && strings.Contains(h, "inertia") {
		add("Inertia.js", "🔗", "definite", "data-page + inertia marker")
	}

	// ── Livewire (Laravel) ────────────────────────────────────────────────────
	if strings.Contains(h, "livewire") || strings.Contains(h, "wire:id=") {
		add("Livewire", "⚡", "definite", "wire:id / livewire script")
	}

	// ── Ember.js ──────────────────────────────────────────────────────────────
	if strings.Contains(h, "ember-application") || strings.Contains(h, "data-ember-action") {
		add("Ember.js", "🐹", "definite", "ember-application / data-ember-action")
	}

	// ── Qwik ──────────────────────────────────────────────────────────────────
	if strings.Contains(h, "q:container") || strings.Contains(h, "q:base=") {
		add("Qwik", "⚡", "definite", "q:container / q:base attribute")
	}

	// ── SolidJS ───────────────────────────────────────────────────────────────
	if strings.Contains(h, "solid-js") || strings.Contains(h, "_$HY") {
		add("SolidJS", "🔷", "definite", "solid-js / _$HY hydration marker")
	}

	// ── Hotwire Turbo ─────────────────────────────────────────────────────────
	if strings.Contains(h, "data-turbo-frame") || strings.Contains(h, "<turbo-frame") {
		add("Hotwire Turbo", "♦️", "definite", "turbo-frame element")
	}

	// ── TanStack Start ────────────────────────────────────────────────────────
	if strings.Contains(h, "__TSR_DEHYDRATED__") {
		add("TanStack Start", "🔶", "definite", "__TSR_DEHYDRATED__ marker")
	}

	// ── GitHub Pages ──────────────────────────────────────────────────────────
	if strings.Contains(headers.Get("Server"), "GitHub.com") ||
		headers.Get("X-GitHub-Request-Id") != "" {
		add("GitHub Pages", "🐙", "definite", "X-GitHub-Request-Id header")
	}

	// ── Render.com ────────────────────────────────────────────────────────────
	if headers.Get("Rndr-Id") != "" {
		add("Render.com", "🟣", "definite", "Rndr-Id header")
	}

	return found
}

func confidenceLabel(score, likelyThresh, definiteThresh int) string {
	if score >= definiteThresh {
		return "definite"
	}
	if score >= likelyThresh {
		return "likely"
	}
	return "possible"
}

// ─── Format Telegram Message ─────────────────────────────────────────────────

// mdEscape escapes all MarkdownV2 reserved characters in plain text segments.
// Do NOT use on text that already contains intentional md syntax (*, _, `, etc.)
func mdEscape(s string) string {
	reserved := `\_*[]()~` + "`" + `>#+-=|{}.!`
	var b strings.Builder
	for _, c := range s {
		if strings.ContainsRune(reserved, c) {
			b.WriteRune('\\')
		}
		b.WriteRune(c)
	}
	return b.String()
}

func formatMessage(url string, detections []Detection, elapsed time.Duration) string {
	var sb strings.Builder

	sb.WriteString("\U0001f50d *Under The Hood Detection Report*\n")
	sb.WriteString("\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\n")
	sb.WriteString(fmt.Sprintf("\U0001f310 *URL:* `%s`\n", mdEscape(url)))
	sb.WriteString(fmt.Sprintf("\u23f1 *Scan time:* %dms\n\n", elapsed.Milliseconds()))

	if len(detections) == 0 {
		sb.WriteString("\u2753 *No frameworks detected*\n")
		sb.WriteString("_Site may be heavily optimized, server\\-rendered, or custom\\-built_\n")
		return sb.String()
	}

	// group by confidence
	groups := map[string][]Detection{
		"definite": {},
		"likely":   {},
		"possible": {},
	}
	for _, d := range detections {
		groups[d.Confidence] = append(groups[d.Confidence], d)
	}

	writeGroup := func(header string, items []Detection) {
		if len(items) == 0 {
			return
		}
		sb.WriteString(header)
		sb.WriteString("\n")
		for _, d := range items {
			sb.WriteString(fmt.Sprintf("  %s *%s*\n    \u21b3 _%s_\n",
				d.Emoji,
				mdEscape(d.Name),
				mdEscape(d.Evidence),
			))
		}
		sb.WriteString("\n")
	}

	writeGroup("\u2705 *Definite*", groups["definite"])
	writeGroup("\U0001f7e1 *Likely*", groups["likely"])
	writeGroup("\u26aa *Possible*", groups["possible"])

	sb.WriteString("\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\n")
	sb.WriteString(fmt.Sprintf("\U0001f4e6 *Total signals:* %d\n", len(detections)))

	return sb.String()
}

// ─── CLI Mode ────────────────────────────────────────────────────────────────

func runCLI(url string) {
	fmt.Println("\n=================================")
	fmt.Printf(" Scanning: %s\n", url)
	fmt.Println("=================================")

	start := time.Now()
	pd, err := fetchPage(url)
	if err != nil {
		fmt.Println("Error fetching page:", err)
		return
	}

	detections := detect(pd)
	elapsed := time.Since(start)

	fmt.Printf(" Scan time: %dms | HTTP %d\n\n", elapsed.Milliseconds(), pd.Status)

	if len(detections) == 0 {
		fmt.Println("No obvious framework detected.")
		return
	}

	for _, d := range detections {
		badge := map[string]string{"definite": "✓", "likely": "~", "possible": "?"}[d.Confidence]
		fmt.Printf(" %s %s %s  [%s]\n    → %s\n", badge, d.Emoji, d.Name, d.Confidence, d.Evidence)
	}
	fmt.Println("\n=================================")
}

// ─── Telegram Bot ────────────────────────────────────────────────────────────

func startBot(token, socks5Addr string) {
	pref := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	// Wire SOCKS5 proxy into the bot's HTTP client if an address is given
	if socks5Addr != "" {
		dialer, err := proxy.SOCKS5("tcp", socks5Addr, nil, proxy.Direct)
		if err != nil {
			log.Fatalf("SOCKS5 proxy error (%s): %v", socks5Addr, err)
		}
		transport := &http.Transport{}
		if dc, ok := dialer.(proxy.ContextDialer); ok {
			transport.DialContext = dc.DialContext
		} else {
			transport.Dial = dialer.Dial //nolint:staticcheck
		}
		pref.Client = &http.Client{Transport: transport}
		fmt.Printf("\U0001f512 Bot using SOCKS5 proxy: %s\n", socks5Addr)
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal("Bot init failed:", err)
	}

	b.Handle("/start", func(c tele.Context) error {
		return c.Send(
			"\U0001f44b *Under The Hood Bot*\n\n"+
				"Send me any URL and I'll detect what tech stack it uses\\.\n\n"+
				"Example:\n`https://vercel\\.com`\n`https://wordpress\\.org`",
			&tele.SendOptions{ParseMode: tele.ModeMarkdownV2},
		)
	})

	b.Handle("/help", func(c tele.Context) error {
		return c.Send(
			"*How to use:*\n\nJust send a URL starting with `http://` or `https://`\\.\n\n"+
				"I detect 30\\+ frameworks, CMSs, CDNs, and libraries\\.",
			&tele.SendOptions{ParseMode: tele.ModeMarkdownV2},
		)
	})

	b.Handle(tele.OnText, func(c tele.Context) error {
		text := strings.TrimSpace(c.Text())

		if !strings.HasPrefix(text, "http://") && !strings.HasPrefix(text, "https://") {
			return c.Send("\u26a0\ufe0f Please send a valid URL starting with `http://` or `https://`",
				&tele.SendOptions{ParseMode: tele.ModeMarkdownV2})
		}

		_ = c.Send("\u23f3 Scanning...")

		start := time.Now()
		pd, err := fetchPage(text)
		if err != nil {
			return c.Send(fmt.Sprintf("\u274c Failed to fetch URL:\n`%s`", err.Error()),
				&tele.SendOptions{ParseMode: tele.ModeMarkdownV2})
		}

		detections := detect(pd)
		elapsed := time.Since(start)

		msg := formatMessage(text, detections, elapsed)
		return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeMarkdownV2})
	})

	fmt.Println("\U0001f916 Bot is running...")

	// ─── Set bot info on startup ─────────────────────────────────────────────────

	// Set bot commands (shows in the / menu)
	commands := []tele.Command{
		{Text: "start", Description: "Start the bot and see usage info"},
		{Text: "help", Description: "How to use the bot"},
	}
	if err := b.SetCommands(commands); err != nil {
		log.Println("Warning: could not set commands:", err)
	}

	// Set bot description (shown before user hits Start for the first time)
	params := map[string]string{
		"description":       "🔍 Detect the tech stack behind any website instantly.\n\nSend any URL and I'll identify frameworks, CMSs, CDNs, libraries and more — with confidence levels.",
		"short_description": "Web tech stack detector — 30+ frameworks, CMSs & CDNs",
	}
	_, err = b.Raw("setMyDescription", params)
	if err != nil {
		log.Println("Warning: could not set description:", err)
	}
	_, err = b.Raw("setMyShortDescription", params)
	if err != nil {
		log.Println("Warning: could not set short description:", err)
	}

	b.Start()
}

func resolveSocks5(useProxy bool) string {
	if !useProxy {
		return ""
	}
	if v := os.Getenv("SOCKS5"); v != "" {
		return v
	}
	return defaultSocks5
}

func main() {
	// Load .env file (ignore error if not present, e.g. in production with real env vars)
	_ = godotenv.Load()

	args := os.Args[1:]

	resolveToken := func() string {
		if v := os.Getenv("BOT_TOKEN"); v != "" {
			return v
		}
		log.Fatal("BOT_TOKEN is not set. Add it to .env or set it as an environment variable.")
		return ""
	}

	// ── CLI mode ──────────────────────────────────────────────────────────────
	if len(args) == 1 && (strings.HasPrefix(args[0], "http://") || strings.HasPrefix(args[0], "https://")) {
		runCLI(args[0])
		return
	}

	// ── Bot mode with proxy ───────────────────────────────────────────────────
	if len(args) > 0 && args[0] == "--proxy" {
		startBot(resolveToken(), resolveSocks5(true))
		return
	}

	// ── Bot mode (no args) ────────────────────────────────────────────────────
	startBot(resolveToken(), resolveSocks5(false))
}
