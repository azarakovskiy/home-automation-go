package priceannouncer

import (
	"strings"
	"testing"
	"time"

	"home-go/internal/domain/pricing"
)

func makeSummaryWindow(fromHour, tillHour int, level pricing.PriceLevel) pricing.SummaryWindow {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	return pricing.SummaryWindow{
		Level: level,
		From:  base.Add(time.Duration(fromHour) * time.Hour),
		Till:  base.Add(time.Duration(tillHour) * time.Hour),
	}
}

func TestFormatter_Morning_Flat(t *testing.T) {
	f := naturalLanguageFormatter{}
	msg := f.Format(AnnounceContext{
		Period:       PeriodMorning,
		CurrentLevel: pricing.PriceLevelAverage,
		Summary:      pricing.IndexSummary{},
	})
	if msg == "" {
		t.Fatal("expected non-empty message for flat day")
	}
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "cheap") || strings.Contains(lower, "expensive") || strings.Contains(lower, "spike") {
		t.Errorf("flat day message should not mention notable windows; got: %s", msg)
	}
}

func TestFormatter_Morning_WithCheapWindow(t *testing.T) {
	f := naturalLanguageFormatter{}
	msg := f.Format(AnnounceContext{
		Period:       PeriodMorning,
		CurrentLevel: pricing.PriceLevelAverage,
		Summary: pricing.IndexSummary{
			CheapWindows: []pricing.SummaryWindow{
				makeSummaryWindow(2, 6, pricing.PriceLevelCheap),
			},
		},
	})
	if !strings.Contains(strings.ToLower(msg), "cheap") {
		t.Errorf("morning message with cheap window should mention cheap; got: %s", msg)
	}
}

func TestFormatter_Morning_WithExpensiveWindow(t *testing.T) {
	f := naturalLanguageFormatter{}
	msg := f.Format(AnnounceContext{
		Period:       PeriodMorning,
		CurrentLevel: pricing.PriceLevelAverage,
		Summary: pricing.IndexSummary{
			ExpensiveWindows: []pricing.SummaryWindow{
				makeSummaryWindow(17, 20, pricing.PriceLevelHigh),
			},
		},
	})
	lower := strings.ToLower(msg)
	if !strings.Contains(lower, "spike") && !strings.Contains(lower, "expensive") && !strings.Contains(lower, "high") {
		t.Errorf("morning message with expensive window should mention spike/expensive/high; got: %s", msg)
	}
}

func TestFormatter_Afternoon_CurrentCheap(t *testing.T) {
	f := naturalLanguageFormatter{}
	msg := f.Format(AnnounceContext{
		Period:       PeriodAfternoon,
		CurrentLevel: pricing.PriceLevelCheap,
		Summary: pricing.IndexSummary{
			CheapWindows: []pricing.SummaryWindow{
				makeSummaryWindow(14, 17, pricing.PriceLevelCheap),
			},
		},
	})
	if !strings.Contains(strings.ToLower(msg), "cheap") {
		t.Errorf("afternoon cheap message should mention cheap; got: %s", msg)
	}
}

func TestFormatter_Afternoon_CurrentHigh(t *testing.T) {
	f := naturalLanguageFormatter{}
	msg := f.Format(AnnounceContext{
		Period:       PeriodAfternoon,
		CurrentLevel: pricing.PriceLevelHigh,
		Summary:      pricing.IndexSummary{},
	})
	lower := strings.ToLower(msg)
	if !strings.Contains(lower, "high") && !strings.Contains(lower, "expensive") {
		t.Errorf("afternoon high message should mention high/expensive; got: %s", msg)
	}
}

func TestFormatter_Afternoon_CurrentAverage_NothingAhead(t *testing.T) {
	f := naturalLanguageFormatter{}
	msg := f.Format(AnnounceContext{
		Period:       PeriodAfternoon,
		CurrentLevel: pricing.PriceLevelAverage,
		Summary:      pricing.IndexSummary{},
	})
	if msg == "" {
		t.Fatal("expected non-empty message")
	}
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "cheap") || strings.Contains(lower, "spike") || strings.Contains(lower, "expensive") {
		t.Errorf("average/nothing message should not mention notable windows; got: %s", msg)
	}
}
