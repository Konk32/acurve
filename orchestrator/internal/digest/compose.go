package digest

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Konk32/acurve/orchestrator/internal/db"
	"github.com/Konk32/acurve/orchestrator/internal/discord"
)

const (
	// maxItemsPerCategory is how many items we pick per category.
	maxItemsPerCategory = 5
	// fallbackMinScore is used when fewer than 3 items qualify at minScore.
	fallbackMinScore = 60
	// minItemsBeforeFallback triggers the score fallback.
	minItemsBeforeFallback = 3
	// discordFieldValueLimit is Discord's per-field character limit.
	discordFieldValueLimit = 1024
)

// categoryColor maps category names to Discord embed sidebar colours.
var categoryColor = map[string]int{
	"kubernetes": 0x326CE5, // Kubernetes blue
	"ai":         0x7B2FBE, // purple
	"security":   0xE53935, // red
	"go":         0x00ADD8, // Go cyan
	"homelab":    0x43A047, // green
	"hardware":   0xFB8C00, // orange
	"tooling":    0x00897B, // teal
	"other":      0x757575, // grey
}

// categoryLabel provides a human-friendly header for each category.
var categoryLabel = map[string]string{
	"kubernetes": "☸️  Kubernetes",
	"ai":         "🤖  AI / ML",
	"security":   "🔐  Security",
	"go":         "🐹  Go",
	"homelab":    "🏠  Homelab",
	"hardware":   "🖥️  Hardware",
	"tooling":    "🛠️  Tooling",
	"other":      "📌  Other",
}

// Result holds the composed Discord embeds and the item IDs they cover.
type Result struct {
	Embeds  []discord.Embed
	ItemIDs []int64
}

// Compose groups items by category, picks the top N per category, and
// returns Discord embeds and the selected item IDs.
func Compose(items []db.DigestItem) Result {
	// Group by category.
	byCategory := make(map[string][]db.DigestItem)
	for _, it := range items {
		byCategory[it.Category] = append(byCategory[it.Category], it)
	}

	// Canonical display order.
	order := []string{"kubernetes", "ai", "security", "go", "homelab", "hardware", "tooling", "other"}

	var embeds []discord.Embed
	var selectedIDs []int64

	for _, cat := range order {
		catItems, ok := byCategory[cat]
		if !ok {
			continue
		}
		// Items already sorted score DESC from the DB query; just cap.
		if len(catItems) > maxItemsPerCategory {
			catItems = catItems[:maxItemsPerCategory]
		}

		label := categoryLabel[cat]
		if label == "" {
			label = strings.ToUpper(cat)
		}
		color := categoryColor[cat]

		var fields []discord.EmbedField
		for _, it := range catItems {
			title := truncate(it.Title, 200)
			summary := truncate(it.Summary, 300)
			value := fmt.Sprintf("**[%s](%s)** `score:%d`\n%s", title, it.URL, it.Score, summary)
			if utf8.RuneCountInString(value) > discordFieldValueLimit {
				value = value[:discordFieldValueLimit-1] + "…"
			}
			fields = append(fields, discord.EmbedField{
				Name:  fmt.Sprintf("# %d", it.Score),
				Value: value,
			})
			selectedIDs = append(selectedIDs, it.ItemID)
		}

		embeds = append(embeds, discord.Embed{
			Title:  label,
			Color:  color,
			Fields: fields,
			Footer: &discord.EmbedFooter{Text: fmt.Sprintf("%d items", len(catItems))},
		})
	}

	return Result{Embeds: embeds, ItemIDs: selectedIDs}
}

// FallbackMinScore returns fallbackMinScore.
func FallbackMinScore() int { return fallbackMinScore }

// MinItemsBeforeFallback returns minItemsBeforeFallback.
func MinItemsBeforeFallback() int { return minItemsBeforeFallback }

func truncate(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes-1]) + "…"
}
