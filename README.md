# 🔍 Under The Hood — Web Tech Detector

A fast, lightweight Go tool that detects the tech stack behind any website — frameworks, CMSs, CDNs, UI libraries, hosting providers, and more. Works as a **CLI tool** and as a **Telegram bot** with optional SOCKS5 proxy support.

---

## ✨ Features

- Detects **30+ technologies** including:
  - **Frontend frameworks**: React, Next.js, Vue, Nuxt, Svelte/SvelteKit, Astro, Angular, Remix, Gatsby, Qwik, SolidJS, Ember, Flutter Web
  - **CSS / UI libraries**: Tailwind CSS, Bootstrap, jQuery, Alpine.js, HTMX, Vite
  - **CMS platforms**: WordPress, Drupal, Joomla, Shopify, WooCommerce, Wix, Squarespace, Webflow, Framer
  - **Backend hints**: Laravel/Inertia, Ruby on Rails, Django, Livewire, Hotwire Turbo
  - **CDN & hosting**: Cloudflare, Vercel, AWS CloudFront, Netlify, GitHub Pages, Render.com
  - **Analytics**: Google Analytics, Google Tag Manager
- **Confidence levels** for each detection: `Definite`, `Likely`, or `Possible`
- Dual mode: **CLI** for quick scans and **Telegram bot** for on-the-go use
- SOCKS5 proxy support (useful when Telegram API is blocked)
- Minimal dependencies, single binary

---

## 🚀 Quick Start

### Prerequisites

- Go 1.22+

### Clone & Build

```bash
git clone https://github.com/aminjoharinia/UnderTheHoodWebTech.git
cd UnderTheHoodWebTech
go build -o underthehood main.go
```


### Cross-compile

```bash
# Linux amd64
GOOS=linux GOARCH=amd64 go build -o Builds/UnderTheHoodWeb_linux_amd64 main.go

# Linux arm64
GOOS=linux GOARCH=arm64 go build -o Builds/UnderTheHoodWeb_linux_arm64 main.go

# Windows
GOOS=windows GOARCH=amd64 go build -o Builds/UnderTheHoodWeb_Windows_amd64.exe main.go

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o Builds/UnderTheHoodWeb_MacOS_arm64 main.go

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o Builds/UnderTheHoodWeb_MacOS_amd64 main.go
```


```bash
chmod +x ./UnderTheHoodWeb_linux_amd64
# or macOS
chmod +x ./UnderTheHoodWeb_MacOS_arm64
```

```bash
# Install
sudo apt install upx        # Linux (Debian/Ubuntu)
brew install upx            # macOS

# Compress
upx --best Builds/UnderTheHoodWeb_linux_amd64
upx --best Builds/UnderTheHoodWeb_Windows_amd64.exe
upx --best Builds/UnderTheHoodWeb_MacOS_arm64

# Best compression (slower but smallest size)
upx --ultra-brute Builds/UnderTheHoodWeb_linux_amd64
```


### Cross-compile (using [garble](https://github.com/burrowers/garble) for obfuscation)

```bash
# Linux amd64
GOOS=linux GOARCH=amd64 garble -literals -tiny build -o Builds/UnderTheHoodWeb_linux_amd64 main.go

# Linux arm64
GOOS=linux GOARCH=arm64 garble -literals -tiny build -o Builds/UnderTheHoodWeb_linux_arm64 main.go

# Windows
GOOS=windows GOARCH=amd64 garble -literals -tiny build -o Builds/UnderTheHoodWeb_Windows_amd64.exe main.go

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 garble -literals -tiny build -o Builds/UnderTheHoodWeb_MacOS_arm64 main.go
```

---

## 🖥️ CLI Usage

Pass any URL as an argument:

```bash
./underthehood https://vercel.com
./underthehood https://wordpress.org
```

**Sample output:**

```
=================================
 Scanning: https://vercel.com
=================================
 Scan time: 312ms | HTTP 200

 ✓ ▲ Next.js  [definite]
   → score 5/7 signals
 ✓ ▲ Vercel Hosting  [definite]
   → X-Vercel-Id header
 ~ 🌬️ Tailwind CSS  [likely]
   → Tailwind utility classes detected

=================================
```

---

## 🤖 Telegram Bot Usage

### Setup

1. Create a `.env` file in the project root:

```env
BOT_TOKEN=your_telegram_bot_token_here
SOCKS5=127.0.0.1:10808   # optional, only needed if Telegram API is blocked
```

2. Run the bot:

```bash
# Without proxy
./underthehood

# With SOCKS5 proxy
./underthehood --proxy
```

## 🤖 Try It Live - Sample Telegram Bot

[@underthehoodwebbot](https://t.me/underthehoodwebbot)

### Bot Commands

| Command | Description |
|---|---|
| `/start` | Introduction and usage instructions |
| `/help` | How to use the bot |
| Any URL | Scan the URL and return a detection report |

**Example bot interaction:**

```
You:  https://shopify.com

Bot:  🔍 Under The Hood Detection Report
      ━━━━━━━━━━━━━━━━━━━━━━━━
      🌐 URL: https://shopify.com
      ⏱ Scan time: 408ms

      ✅ Definite
        🛍️ Shopify
           ↳ cdn.shopify.com asset
        🟠 Cloudflare CDN
           ↳ CF-Ray header present
```

---

## ⚙️ Configuration

| Variable | Default | Description |
|---|---|---|
| `BOT_TOKEN` | *(required for bot mode)* | Telegram bot token from [@BotFather](https://t.me/botfather) |
| `SOCKS5` | `127.0.0.1:10808` | SOCKS5 proxy address (used with `--proxy` flag) |

Can be set via `.env` file or real environment variables.

---

## 📦 Dependencies

| Package | Purpose |
|---|---|
| `gopkg.in/telebot.v4` | Telegram bot framework |
| `golang.org/x/net` | SOCKS5 proxy dialer |
| `github.com/joho/godotenv` | `.env` file loading |

---

## 📁 Project Structure

```
.
├── main.go          # All detection logic, CLI & bot handlers
├── go.mod
├── go.sum
├── .env             # (not committed) BOT_TOKEN, SOCKS5
├── .gitignore
└── Builds/          # Pre-compiled binaries
```

---

## 📄 License

MIT
