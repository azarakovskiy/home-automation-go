package priceannouncer

import (
	"fmt"
	"strings"
	"time"

	"home-go/internal/domain/pricing"
)

// AnnouncePeriod indicates which time bucket applies to an on-demand announcement.
type AnnouncePeriod int

const (
	PeriodMorning   AnnouncePeriod = iota // hour < 11: full-day summary
	PeriodAfternoon                       // hour >= 11: remaining-window summary
)

// AnnounceContext carries the inputs the formatter needs.
type AnnounceContext struct {
	Period       AnnouncePeriod
	CurrentLevel pricing.PriceLevel
	Summary      pricing.IndexSummary // scoped to remaining window: now → midnight
}

// MessageFormatter converts an AnnounceContext into a human-readable message.
type MessageFormatter interface {
	Format(AnnounceContext) string
}

type naturalLanguageFormatter struct{}

func (naturalLanguageFormatter) Format(ctx AnnounceContext) string {
	if ctx.Period == PeriodMorning {
		return formatMorning(ctx.Summary)
	}
	return formatAfternoon(ctx.CurrentLevel, ctx.Summary)
}

func formatMorning(s pricing.IndexSummary) string {
	hasCheap := len(s.CheapWindows) > 0
	hasExp := len(s.ExpensiveWindows) > 0
	hasNeg := len(s.NegativeWindows) > 0

	if !hasCheap && !hasExp && !hasNeg {
		return "Prices are flat today — nothing dramatic either way."
	}

	var b strings.Builder
	switch {
	case hasCheap && !hasExp:
		b.WriteString("It's a good day price-wise.")
	case hasExp && !hasCheap:
		b.WriteString("Prices are high today.")
	default:
		b.WriteString("Mixed prices today.")
	}

	if hasNeg {
		w := s.NegativeWindows[0]
		fmt.Fprintf(&b, " Prices go negative around %s.", hourRef(w.From))
	}

	if hasCheap {
		w := s.CheapWindows[0]
		fmt.Fprintf(&b, " Cheapest from %s until %s.", hourRef(w.From), hourRef(w.Till))
	}

	if hasExp {
		w := s.ExpensiveWindows[0]
		fmt.Fprintf(&b, " Prices spike after %s — best to get things done before that.", hourRef(w.From))
	}

	return b.String()
}

func formatAfternoon(current pricing.PriceLevel, s pricing.IndexSummary) string {
	hasCheap := len(s.CheapWindows) > 0
	hasExp := len(s.ExpensiveWindows) > 0

	switch current {
	case pricing.PriceLevelCheap:
		var b strings.Builder
		b.WriteString("It's cheap right now")
		if hasCheap {
			w := s.CheapWindows[0]
			fmt.Fprintf(&b, " and stays that way until %s — good time to act", hourRef(w.Till))
		}
		if hasExp {
			b.WriteString(". After that prices spike")
		}
		b.WriteString(".")
		return b.String()

	case pricing.PriceLevelHigh:
		var b strings.Builder
		b.WriteString("Prices are high right now")
		if hasCheap {
			w := s.CheapWindows[0]
			fmt.Fprintf(&b, ". It gets cheaper after %s if you can wait", hourRef(w.From))
		}
		b.WriteString(".")
		return b.String()

	default: // PriceLevelAverage or PriceLevelUnknown
		if !hasCheap && !hasExp {
			return "Nothing noteworthy ahead — prices are moderate for the rest of the day."
		}
		var b strings.Builder
		b.WriteString("Prices are moderate right now.")
		if hasCheap {
			w := s.CheapWindows[0]
			fmt.Fprintf(&b, " It gets cheap from %s.", hourRef(w.From))
		}
		if hasExp {
			w := s.ExpensiveWindows[0]
			fmt.Fprintf(&b, " Prices spike after %s.", hourRef(w.From))
		}
		return b.String()
	}
}

// hourRef converts a time to a short human clock reference: "midnight", "noon", "6", "5".
func hourRef(t time.Time) string {
	h := t.Hour()
	switch h {
	case 0:
		return "midnight"
	case 12:
		return "noon"
	default:
		if h < 12 {
			return fmt.Sprintf("%d", h)
		}
		return fmt.Sprintf("%d", h-12)
	}
}
