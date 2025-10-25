package notifications

import (
	"math/rand"
	"strings"
)

// TerryTranslations contains multiple Brooklyn 99 Terry Crews-style variations for each notification
// Terry speaks in third person, is enthusiastic about fitness/health, and wholesome but intense
// Placeholders: {{time}}, {{savings}}
// Future: Will be replaced with AI-powered rephrasing, but keeping these as fallbacks
var TerryTranslations = map[string][]string{
	// Start dishwasher NOW (no delay)
	"dishwasher_now": {
		"START THE DISHWASHER NOW! Terry says prices are perfect.",
		"Run it NOW! Terry loves these cheap electricity prices.",
		"GO TIME! Terry detected optimal prices for the dishwasher.",
		"Terry says DO IT! Electricity is cheap right now!",
		"PERFECT TIMING! Terry found the sweet spot! Start dishwasher!",
		"Terry is EXCITED! Now is the time! Prices are great!",
		"LET'S GO! Terry detected rock-bottom prices! Dishwasher time!",
		"Terry loves this! Run the dishwasher NOW! Great rates!",
		"BOOM! Terry says start it! Electricity prices are amazing!",
		"Terry found the perfect moment! Start that dishwasher now!",
	},
	// Start dishwasher LATER (delayed for better prices)
	"dishwasher_later": {
		"Terry saved {{savings}}%! Dishwasher starts at {{time}}.",
		"WAIT FOR IT! {{savings}}% savings at {{time}}!",
		"Terry scheduled {{time}} start! That's {{savings}}% cheaper!",
		"Patience pays off! Terry saved {{savings}}% at {{time}}!",
		"Terry is SMART! Dishwasher at {{time}} saves {{savings}}%!",
		"BOOM! {{savings}}% discount at {{time}}! Terry loves savings!",
		"Terry optimized this! {{time}} start, {{savings}}% cheaper!",
		"Hold on! Terry found {{savings}}% savings at {{time}}!",
		"Terry scheduled {{time}}! That's {{savings}}% off electricity!",
		"Smart move! {{savings}}% savings! Dishwasher runs at {{time}}!",
	},
}

// GetTerryMessage returns a random Terry-style message for a given key
// Returns the original key if no translation exists (for now, until AI integration)
func GetTerryMessage(key string) string {
	if variations, ok := TerryTranslations[key]; ok && len(variations) > 0 {
		// Return random variation for variety
		return variations[rand.Intn(len(variations))]
	}
	return key // Fallback: return original key
}

// GetTerryMessageOrDefault returns a random Terry-style message or a provided default
func GetTerryMessageOrDefault(key string, defaultMsg string) string {
	if variations, ok := TerryTranslations[key]; ok && len(variations) > 0 {
		return variations[rand.Intn(len(variations))]
	}
	return defaultMsg
}

// GetTerryMessageWithData returns a Terry message with placeholder replacements
// Placeholders: {{savings}}, {{time}}, {{price}}, {{duration}}
func GetTerryMessageWithData(key string, data map[string]string) string {
	msg := GetTerryMessage(key)

	// Replace placeholders
	for placeholder, value := range data {
		msg = strings.ReplaceAll(msg, "{{"+placeholder+"}}", value)
	}

	return msg
}
