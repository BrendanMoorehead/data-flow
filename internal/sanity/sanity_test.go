package sanity

import (
	"testing"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
	"github.com/BrendanMoorehead/data-flow/internal/odds"
)

func side(side canonical.Side, price odds.American, status canonical.MarketStatus) canonical.Observation {
	return canonical.Observation{
		Source: canonical.SourceDraftKingsDirect,
		Book:   canonical.BookDraftKings,
		Event:  canonical.ProviderEvent{ProviderEventID: "dk-5501"},
		Market: canonical.MarketMoneyline,
		Side:   side,
		Status: status,
		Price:  price.Decimal(),
	}
}

func TestCheckMarginsAcceptsATypicalMarket(t *testing.T) {
	accepted, rejected := CheckMargins([]canonical.Observation{
		side(canonical.SideHome, -110, canonical.StatusOpen),
		side(canonical.SideAway, -110, canonical.StatusOpen),
	})

	if len(accepted) != 2 || len(rejected) != 0 {
		t.Errorf("accepted %d, rejected %d; want 2 and 0", len(accepted), len(rejected))
	}
}

func TestCheckMarginsRejectsBothSidesOfAnImpossibleMarket(t *testing.T) {
	accepted, rejected := CheckMargins([]canonical.Observation{
		side(canonical.SideHome, 200, canonical.StatusOpen),
		side(canonical.SideAway, 200, canonical.StatusOpen),
	})

	if len(accepted) != 0 || len(rejected) != 2 {
		t.Errorf("accepted %d, rejected %d; want 0 and 2 for a market paying +200 on both sides", len(accepted), len(rejected))
	}
}

func TestCheckMarginsAcceptsASingleOpenSide(t *testing.T) {
	accepted, rejected := CheckMargins([]canonical.Observation{
		side(canonical.SideHome, 200, canonical.StatusOpen),
		side(canonical.SideAway, 200, canonical.StatusOffBoard),
	})

	if len(accepted) != 2 || len(rejected) != 0 {
		t.Errorf("accepted %d, rejected %d; want both kept when one side is off the board", len(accepted), len(rejected))
	}
}
