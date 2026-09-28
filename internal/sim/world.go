package sim

import (
	"context"
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
	"sync"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/odds"
)

const (
	bookDraftKings = "DraftKings"
	bookFanDuel    = "FanDuel"

	bookMargin              = 0.045
	minFairProbability      = 0.2
	maxFairProbability      = 0.8
	maxProbabilityStep      = 0.02
	maxBookProbabilityDrift = 0.015
	maxQuoteHistory         = 200

	suspensionChancePerDrift = 0.08
	oneSidedSuspensionChance = 0.3
	minSuspension            = 10 * time.Second
	maxSuspension            = 20 * time.Second

	firstTipoffHourUTC = 23
	gameSpacing        = 30 * time.Minute
)

var books = []string{bookDraftKings, bookFanDuel}

type team struct {
	fullName  string
	shortName string
	nickname  string
}

var nbaTeams = []team{
	{fullName: "Los Angeles Lakers", shortName: "LA Lakers", nickname: "Lakers"},
	{fullName: "Boston Celtics", shortName: "BOS Celtics", nickname: "Celtics"},
	{fullName: "Golden State Warriors", shortName: "GS Warriors", nickname: "Warriors"},
	{fullName: "Denver Nuggets", shortName: "DEN Nuggets", nickname: "Nuggets"},
	{fullName: "Milwaukee Bucks", shortName: "MIL Bucks", nickname: "Bucks"},
	{fullName: "Philadelphia 76ers", shortName: "PHI 76ers", nickname: "76ers"},
	{fullName: "New York Knicks", shortName: "NY Knicks", nickname: "Knicks"},
	{fullName: "Miami Heat", shortName: "MIA Heat", nickname: "Heat"},
	{fullName: "Dallas Mavericks", shortName: "DAL Mavericks", nickname: "Mavericks"},
	{fullName: "Phoenix Suns", shortName: "PHO Suns", nickname: "Suns"},
	{fullName: "Oklahoma City Thunder", shortName: "OKC Thunder", nickname: "Thunder"},
	{fullName: "Minnesota Timberwolves", shortName: "MIN Timberwolves", nickname: "Timberwolves"},
}

type moneylineQuote struct {
	home      odds.American
	away      odds.American
	updatedAt time.Time
}

type side string

const (
	sideHome side = "home"
	sideAway side = "away"
)

type suspension struct {
	sides map[side]bool
	until time.Time
}

func (s suspension) covers(suspendedSide side, now time.Time) bool {
	return s.sides[suspendedSide] && now.Before(s.until)
}

type game struct {
	number              int
	home                team
	away                team
	startsAt            time.Time
	homeFairProbability map[string]float64
	quoteHistory        map[string][]moneylineQuote
	suspensions         map[string]suspension
}

type world struct {
	mu     sync.Mutex
	random *rand.Rand
	games  []*game
	now    func() time.Time
}

func newWorld(seed uint64, now func() time.Time) *world {
	created := &world{random: rand.New(rand.NewPCG(seed, seed)), now: now}
	created.games = created.scheduleGames()
	return created
}

func (w *world) run(ctx context.Context, driftInterval time.Duration) {
	ticker := time.NewTicker(driftInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.driftOnePrice()
		}
	}
}

func (w *world) scheduleGames() []*game {
	firstTipoff := firstTipoffOn(w.now())
	var games []*game
	for pairStart := 0; pairStart+1 < len(nbaTeams); pairStart += 2 {
		gameIndex := pairStart / 2
		scheduled := &game{
			number:              gameIndex + 1,
			home:                nbaTeams[pairStart],
			away:                nbaTeams[pairStart+1],
			startsAt:            firstTipoff.Add(time.Duration(gameIndex) * gameSpacing),
			homeFairProbability: make(map[string]float64, len(books)),
			quoteHistory:        make(map[string][]moneylineQuote, len(books)),
			suspensions:         make(map[string]suspension, len(books)),
		}
		w.openMarkets(scheduled)
		games = append(games, scheduled)
	}
	return games
}

func (w *world) openMarkets(scheduled *game) {
	consensusProbability := w.randomBetween(minFairProbability, maxFairProbability)
	for _, book := range books {
		bookDrift := w.randomBetween(-maxBookProbabilityDrift, maxBookProbabilityDrift)
		scheduled.homeFairProbability[book] = clampFairProbability(consensusProbability + bookDrift)
		w.requote(scheduled, book)
	}
}

func (w *world) driftOnePrice() {
	w.mu.Lock()
	defer w.mu.Unlock()

	drifting := w.games[w.random.IntN(len(w.games))]
	book := books[w.random.IntN(len(books))]
	step := w.randomBetween(-maxProbabilityStep, maxProbabilityStep)
	drifting.homeFairProbability[book] = clampFairProbability(drifting.homeFairProbability[book] + step)
	w.requote(drifting, book)
	w.maybeSuspendMarket()
}

func (w *world) maybeSuspendMarket() {
	if w.random.Float64() >= suspensionChancePerDrift {
		return
	}
	suspended := w.games[w.random.IntN(len(w.games))]
	book := books[w.random.IntN(len(books))]
	duration := time.Duration(w.randomBetween(float64(minSuspension), float64(maxSuspension)))
	suspended.suspensions[book] = suspension{sides: w.randomSuspendedSides(), until: w.now().Add(duration)}
}

func (w *world) randomSuspendedSides() map[side]bool {
	if w.random.Float64() >= oneSidedSuspensionChance {
		return map[side]bool{sideHome: true, sideAway: true}
	}
	if w.random.IntN(2) == 0 {
		return map[side]bool{sideHome: true}
	}
	return map[side]bool{sideAway: true}
}

func (w *world) requote(quoted *game, book string) {
	quote := quoteFromFairProbability(quoted.homeFairProbability[book], w.now())
	history := append(quoted.quoteHistory[book], quote)
	quoted.quoteHistory[book] = history[max(0, len(history)-maxQuoteHistory):]
}

func (w *world) randomBetween(low, high float64) float64 {
	return low + w.random.Float64()*(high-low)
}

type gameView struct {
	number       int
	home         team
	away         team
	startsAt     time.Time
	quoteHistory map[string][]moneylineQuote
	suspensions  map[string]suspension
}

func (w *world) snapshot() []gameView {
	w.mu.Lock()
	defer w.mu.Unlock()

	views := make([]gameView, 0, len(w.games))
	for _, scheduled := range w.games {
		views = append(views, gameView{
			number:       scheduled.number,
			home:         scheduled.home,
			away:         scheduled.away,
			startsAt:     scheduled.startsAt,
			quoteHistory: cloneHistories(scheduled.quoteHistory),
			suspensions:  maps.Clone(scheduled.suspensions),
		})
	}
	return views
}

func (v gameView) isSuspended(book string, suspendedSide side, now time.Time) bool {
	return v.suspensions[book].covers(suspendedSide, now)
}

func (v gameView) isAnySideSuspended(book string, now time.Time) bool {
	return v.isSuspended(book, sideHome, now) || v.isSuspended(book, sideAway, now)
}

func (v gameView) currentQuote(book string) moneylineQuote {
	history := v.quoteHistory[book]
	return history[len(history)-1]
}

func (v gameView) previousQuote(book string) moneylineQuote {
	history := v.quoteHistory[book]
	return history[max(0, len(history)-2)]
}

func (v gameView) quoteAsOf(book string, moment time.Time) moneylineQuote {
	history := v.quoteHistory[book]
	for index := len(history) - 1; index > 0; index-- {
		if !history[index].updatedAt.After(moment) {
			return history[index]
		}
	}
	return history[0]
}

func cloneHistories(histories map[string][]moneylineQuote) map[string][]moneylineQuote {
	cloned := make(map[string][]moneylineQuote, len(histories))
	for book, history := range histories {
		cloned[book] = slices.Clone(history)
	}
	return cloned
}

func quoteFromFairProbability(homeProbability float64, at time.Time) moneylineQuote {
	return moneylineQuote{
		home:      americanWithMargin(homeProbability),
		away:      americanWithMargin(1 - homeProbability),
		updatedAt: at,
	}
}

func americanWithMargin(fairProbability float64) odds.American {
	price, err := odds.AmericanFromProbability(fairProbability * (1 + bookMargin))
	if err != nil {
		panic(fmt.Sprintf("fair probability %v escaped its clamp: %v", fairProbability, err))
	}
	return price
}

func clampFairProbability(probability float64) float64 {
	return min(max(probability, minFairProbability), maxFairProbability)
}

func firstTipoffOn(now time.Time) time.Time {
	year, month, day := now.UTC().Date()
	return time.Date(year, month, day, firstTipoffHourUTC, 0, 0, 0, time.UTC)
}
