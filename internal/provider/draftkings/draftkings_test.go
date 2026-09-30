package draftkings

import (
	"testing"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
	"github.com/BrendanMoorehead/data-flow/internal/odds"
)

var offerUpdatedAt = time.Date(2026, 9, 28, 18, 0, 0, 0, time.UTC)

func lakersCelticsOffers(status string, outcomes ...outcomeJSON) eventOffersJSON {
	return eventOffersJSON{
		Event: eventJSON{EventID: "dk-5501", League: "NBA", HomeTeam: "LA Lakers", AwayTeam: "BOS Celtics"},
		Offers: []offerJSON{{
			Type:      offerTypeMoneyline,
			Status:    status,
			UpdatedAt: offerUpdatedAt,
			Outcomes:  outcomes,
		}},
	}
}

func TestSnapshotConvertsAmericanStringsToDecimalWithOfferTimestamp(t *testing.T) {
	snapshot := snapshotFromEventOffers(lakersCelticsOffers("OPEN",
		outcomeJSON{Role: "home", OddsAmerican: "+295"},
		outcomeJSON{Role: "away", OddsAmerican: "-380"},
	))

	if len(snapshot.Observations) != 2 || len(snapshot.Rejections) != 0 {
		t.Fatalf("got %d observations and %d rejections, want 2 and 0", len(snapshot.Observations), len(snapshot.Rejections))
	}
	home := snapshot.Observations[0]
	if home.Side != canonical.SideHome || home.Price != odds.American(295).Decimal() || home.RawPrice != "+295" {
		t.Errorf("home observation = %+v, want home at 3.95 with raw +295", home)
	}
	if !home.HasSourceTimestamp || !home.ObservedAt.Equal(offerUpdatedAt) {
		t.Errorf("observed at = %v (has timestamp %v), want the offer's updatedAt", home.ObservedAt, home.HasSourceTimestamp)
	}
}

func TestSuspendedOfferTakesBothSidesOffTheBoardWithLastPrice(t *testing.T) {
	snapshot := snapshotFromEventOffers(lakersCelticsOffers(offerStatusSuspended,
		outcomeJSON{Role: "home", OddsAmerican: "+295"},
		outcomeJSON{Role: "away", OddsAmerican: "-380"},
	))

	for _, observation := range snapshot.Observations {
		if observation.Status != canonical.StatusOffBoard || observation.Price == 0 {
			t.Errorf("%s = %s at %v, want off the board keeping its last price", observation.Side, observation.Status, observation.Price)
		}
	}
}

func TestUnknownRoleAndBadOddsAreRejected(t *testing.T) {
	snapshot := snapshotFromEventOffers(lakersCelticsOffers("OPEN",
		outcomeJSON{Role: "draw", OddsAmerican: "+250"},
		outcomeJSON{Role: "away", OddsAmerican: "-50"},
	))

	if len(snapshot.Rejections) != 2 || len(snapshot.Observations) != 0 {
		t.Errorf("got %d rejections and %d observations, want both outcomes rejected", len(snapshot.Rejections), len(snapshot.Observations))
	}
}
