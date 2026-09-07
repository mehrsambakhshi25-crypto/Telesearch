package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ============================================================
// موتور جستجوی DuckDuckGo (رایگان، بدون نیاز به API Key)
// ============================================================
type DuckDuckGoEngine struct{}

func (d DuckDuckGoEngine) Search(query string) (string, error) {
	apiURL := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1&skip_disambig=1", url.QueryEscape(query))
	resp, err := http.Get(apiURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("🔍 *نتایج جستجو برای:* `%s`\n\n", query))

	if abstract, ok := result["AbstractText"].(string); ok && abstract != "" {
		output.WriteString("📌 *خلاصه:*\n")
		output.WriteString(abstract)
		output.WriteString("\n\n")
	}

	if heading, ok := result["Heading"].(string); ok && heading != "" {
		output.WriteString(fmt.Sprintf("📝 *%s*\n", heading))
	}

	if link, ok := result["AbstractURL"].(string); ok && link != "" {
		output.WriteString(fmt.Sprintf("🔗 [لینک بیشتر](%s)\n\n", link))
	}

	if related, ok := result["RelatedTopics"].([]interface{}); ok && len(related) > 0 {
		output.WriteString("📋 *نتایج دیگر:*\n")
		count := 0
		for _, item := range related {
			if count >= 3 {
				break
			}
			topic, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if text, ok := topic["Text"].(string); ok && text != "" {
				output.WriteString(fmt.Sprintf("- %s\n", text))
				count++
			}
		}
	}

	if output.Len() == 0 {
		return "❌ نتیجه‌ای پیدا نشد. لطفاً عبارت دقیق‌تری وارد کنید.", nil
	}

	return output.String(), nil
}

// ============================================================
// موتور جستجوی Google (نیاز به API Key و CX از Google Custom Search)
// ============================================================
type GoogleEngine struct {
	APIKey string
	CX     string
}

func (g GoogleEngine) Search(query string) (string, error) {
	apiURL := fmt.Sprintf("https://www.googleapis.com/customsearch/v1?key=%s&cx=%s&q=%s&num=5",
		g.APIKey, g.CX, url.QueryEscape(query))

	resp, err := http.Get(apiURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	items, ok := result["items"].([]interface{})
	if !ok || len(items) == 0 {
		return "❌ نتیجه‌ای پیدا نشد.", nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("🔍 *نتایج Google برای:* `%s`\n\n", query))

	for i, item := range items {
		if i >= 5 {
			break
		}
		itemMap := item.(map[string]interface{})
		title := itemMap["title"].(string)
		link := itemMap["link"].(string)
		snippet := itemMap["snippet"].(string)

		output.WriteString(fmt.Sprintf("%d. *%s*\n", i+1, title))
		output.WriteString(fmt.Sprintf("   %s\n", snippet))
		output.WriteString(fmt.Sprintf("   🔗 %s\n\n", link))
	}

	return output.String(), nil
}

// ============================================================
// تابع اصلی
// ============================================================
func main() {
	// دریافت توکن از متغیر محیطی (امن‌تر)
	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatal("❌ BOT_TOKEN environment variable not set")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = false
	log.Printf("✅ ربات %s راه‌اندازی شد!", bot.Self.UserName)

	// مقداردهی موتورهای جستجو
	duckDuckGo := DuckDuckGoEngine{}
	google := GoogleEngine{
		APIKey: os.Getenv("GOOGLE_API_KEY"),
		CX:     os.Getenv("GOOGLE_CX"),
	}

	// موتور پیش‌فرض: DuckDuckGo
	defaultEngine := duckDuckGo

	// تنظیم دریافت آپدیت‌ها
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		chatID := update.Message.Chat.ID
		text := update.Message.Text

		// ========== مدیریت دستورات ==========
		switch {
		case strings.HasPrefix(text, "/start"):
			msg := `🤖 *به TeleSearch خوش آمدید!*

یه موتور جستجوی کامل تو دل تلگرام.

🔎 *چطور استفاده کنم؟*
فقط عبارت مورد نظرت رو بفرست تا برات جستجو کنم.

🔧 *دستورات:*
/start - نمایش این پیام
/help - راهنما
/google <عبارت> - جستجو با گوگل
/ddg <عبارت> - جستجو با DuckDuckGo
/about - درباره ربات

🔒 *حریم خصوصی:* جستجوهای شما ذخیره نمی‌شود.`
			bot.Send(tgbotapi.NewMessage(chatID, msg))

		case strings.HasPrefix(text, "/help"):
			bot.Send(tgbotapi.NewMessage(chatID, "📖 *راهنما:*\nفقط عبارت مورد نظرت رو بفرست. من برات جستجو می‌کنم.\n\nبرای جستجو با گوگل از `/google عبارت` استفاده کن."))

		case strings.HasPrefix(text, "/about"):
			bot.Send(tgbotapi.NewMessage(chatID, "🤖 *TeleSearch v1.0*\nساخته شده با Go + Telegram API\nموتور جستجو: DuckDuckGo + Google"))

		case strings.HasPrefix(text, "/google"):
			query := strings.TrimPrefix(text, "/google ")
			if query == "" {
				bot.Send(tgbotapi.NewMessage(chatID, "❌ لطفاً عبارت مورد نظر رو بعد از /google وارد کن."))
				continue
			}
			// جستجو با گوگل
			waitMsg := tgbotapi.NewMessage(chatID, "⏳ در حال جستجو در Google...")
			waitMsg.ParseMode = "Markdown"
			waitSent, _ := bot.Send(waitMsg)

			result, err := google.Search(query)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ خطا: %v", err)))
				continue
			}

			deleteMsg := tgbotapi.NewDeleteMessage(chatID, waitSent.MessageID)
			bot.Send(deleteMsg)

			respMsg := tgbotapi.NewMessage(chatID, result)
			respMsg.ParseMode = "Markdown"
			respMsg.DisableWebPagePreview = true
			bot.Send(respMsg)

		case strings.HasPrefix(text, "/ddg"):
			query := strings.TrimPrefix(text, "/ddg ")
			if query == "" {
				bot.Send(tgbotapi.NewMessage(chatID, "❌ لطفاً عبارت مورد نظر رو بعد از /ddg وارد کن."))
				continue
			}
			// جستجو با DuckDuckGo
			waitMsg := tgbotapi.NewMessage(chatID, "⏳ در حال جستجو در DuckDuckGo...")
			waitMsg.ParseMode = "Markdown"
			waitSent, _ := bot.Send(waitMsg)

			result, err := duckDuckGo.Search(query)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ خطا: %v", err)))
				continue
			}

			deleteMsg := tgbotapi.NewDeleteMessage(chatID, waitSent.MessageID)
			bot.Send(deleteMsg)

			respMsg := tgbotapi.NewMessage(chatID, result)
			respMsg.ParseMode = "Markdown"
			respMsg.DisableWebPagePreview = true
			bot.Send(respMsg)

		default:
			// جستجوی معمولی با موتور پیش‌فرض (DuckDuckGo)
			waitMsg := tgbotapi.NewMessage(chatID, "⏳ در حال جستجو...")
			waitMsg.ParseMode = "Markdown"
			waitSent, _ := bot.Send(waitMsg)

			result, err := defaultEngine.Search(text)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ خطا: %v", err)))
				continue
			}

			deleteMsg := tgbotapi.NewDeleteMessage(chatID, waitSent.MessageID)
			bot.Send(deleteMsg)

			respMsg := tgbotapi.NewMessage(chatID, result)
			respMsg.ParseMode = "Markdown"
			respMsg.DisableWebPagePreview = true
			bot.Send(respMsg)
		}

		// جلوگیری از اسپم
		time.Sleep(500 * time.Millisecond)
	}
}