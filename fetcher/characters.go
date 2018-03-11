package fetcher

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"gopkg.in/guregu/null.v3"

	"github.com/liclac/gubal/lib"
	"github.com/liclac/gubal/models"
)

func init() { registerJob(func() Job { return &FetchCharacterJob{} }) }

// FetchCharacterJob fetches a character.
// A character that isn't found instead creates a CharacterTombstone in the database to signal this.
type FetchCharacterJob struct {
	ID    int64 `json:"id"`
	Force bool  `json:"force"`
}

// Type returns the type for a job.
func (FetchCharacterJob) Type() string { return "character" }

// Run runs the job.
func (j FetchCharacterJob) Run(ctx context.Context) (rjobs []Job, rerr error) {
	ds := models.GetDataStore(ctx)

	// Check if the character has a tombstone, bail out if so.
	dead, err := ds.CharacterTombstones().Check(j.ID)
	if dead || err != nil {
		return nil, err
	}

	// Read the character's public status page.
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/character/%d/", LodestoneBaseURL, j.ID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := resp.Body.Close(); err != nil {
		return nil, err
	}

	// Bail out if the response code isn't 200 OK.
	switch resp.StatusCode {
	case http.StatusOK:
		// All quiet on the response front.
	case http.StatusNotFound:
		// The character doesn't exist, create a tombstone in the database to mark this and abort.
		lib.GetLogger(ctx).Info("Character does not exist; creating tombstone", zap.Int64("id", j.ID))
		return nil, ds.CharacterTombstones().Create(j.ID)
	default:
		return nil, errors.Errorf("incorrect HTTP status code when fetching character data: %d", resp.StatusCode)
	}

	// Actually parse the page! Parsing steps are split into smaller pieces for maintainability,
	// and are combined into one big multierr so we can check them all in one fell swoop.
	doc, err := goquery.NewDocumentFromReader(bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	char := models.Character{ID: j.ID}
	if err := multierr.Combine(
		j.parseName(ctx, &char, doc),
		j.parseTitle(ctx, &char, doc),
		j.parseBlocks(ctx, &char, doc),
	); err != nil {
		return nil, err
	}

	return nil, ds.Characters().Save(&char)
}

// parseName parses the character's FirstName and LastName from the page.
func (j FetchCharacterJob) parseName(ctx context.Context, ch *models.Character, doc *goquery.Document) error {
	fullName := trim(doc.Find(".frame__chara__name").First().Text())
	firstAndLast := strings.SplitN(fullName, " ", 2)
	if l := len(firstAndLast); l != 2 {
		return errors.Errorf("malformed name: \"%s\" (%d components)", fullName, l)
	}
	ch.FirstName = firstAndLast[0]
	ch.LastName = firstAndLast[1]
	return nil
}

// parseTitle parses the character's title from the page; titles are stored externally.
func (j FetchCharacterJob) parseTitle(ctx context.Context, ch *models.Character, doc *goquery.Document) error {
	titleStr := trim(doc.Find(".frame__chara__title").First().Text())
	if titleStr == "" {
		ch.Title = nil
		ch.TitleID = null.Int{}
		return nil
	}
	title, err := models.GetDataStore(ctx).CharacterTitles().GetOrCreate(titleStr)
	ch.Title = title
	return err
}

// parseBlocks parses the blocks containing Race/Clan/Gender, Nameday/Guardian, City State and GC.
func (j FetchCharacterJob) parseBlocks(ctx context.Context, ch *models.Character, doc *goquery.Document) error {
	var errs []error
	doc.Find(".character-block").Each(func(i int, sel *goquery.Selection) {
		title := trim(sel.Find(".character-block__name").First().Text())
		switch title {
		case "Race/Clan/Gender":
			errs = append(errs, j.parseRaceClanGenderBlock(ctx, ch, doc, sel))
		case "Nameday":
			errs = append(errs, j.parseNamedayGuardianBlock(ctx, ch, doc, sel))
		case "City-state":
			errs = append(errs, j.parseCityStateBlock(ctx, ch, doc, sel))
		case "Grand Company":
			errs = append(errs, j.parseGrandCompanyBlock(ctx, ch, doc, sel))
		default:
			lib.GetLogger(ctx).Warn("unknown box on profile", zap.String("title", title))
		}
	})
	return multierr.Combine(errs...)
}

func (j FetchCharacterJob) parseRaceClanGenderBlock(ctx context.Context, ch *models.Character, doc *goquery.Document, sel *goquery.Selection) error {
	// Extract the blurb.
	str := trim(sel.Find(".character-block__profile").Text())
	lib.GetLogger(ctx).Info("race/clan/gender", zap.String("str", str))

	// Split it up into parts.
	parts := strings.SplitN(str, "/", 2)
	if l := len(parts); l != 2 {
		return errors.Errorf("couldn't parse race/clan/gender; wrong number of items: %d", l)
	}
	raceClan := trim(parts[0])
	gender := trim(parts[1])
	lib.GetLogger(ctx).Info("race/clan/gender", zap.String("race_clan", raceClan), zap.String("gender", gender))

	// Assign gender.
	ch.Gender = gender

	// Assign race/clan.
	switch raceClan {
	case "HyurMidlander":
		ch.Race = models.Hyur
		ch.Clan = models.HyurMidlander
	case "HyurHighlander":
		ch.Race = models.Hyur
		ch.Clan = models.HyurHighlander
	case "ElezenWildwood":
		ch.Race = models.Elezen
		ch.Clan = models.ElezenWildwood
	case "ElezenDuskwight":
		ch.Race = models.Elezen
		ch.Clan = models.ElezenDuskwight
	case "LalafellPlainsfolk":
		ch.Race = models.Lalafell
		ch.Clan = models.LalafellPlainsfolk
	case "LalafellDunesfolk":
		ch.Race = models.Lalafell
		ch.Clan = models.LalafellDunesfolk
	case "Miqo'teSeeker of the Sun":
		ch.Race = models.Miqote
		ch.Clan = models.MiqoteSunSeeker
	case "Miqo'teKeeper of the Moon":
		ch.Race = models.Miqote
		ch.Clan = models.MiqoteMoonKeeper
	case "RoegadynSea Wolf":
		ch.Race = models.Roegadyn
		ch.Clan = models.RoegadynSeaWolf
	case "RoegadynHellsguard":
		ch.Race = models.Roegadyn
		ch.Clan = models.RoegadynHellsguard
	case "Au RaRaen":
		ch.Race = models.AuRa
		ch.Clan = models.AuRaRaen
	case "Au RaXaela":
		ch.Race = models.AuRa
		ch.Clan = models.AuRaXaela
	default:
		return errors.Errorf("unknown race/clan string: \"%s\"", raceClan)
	}
	return nil
}

func (j FetchCharacterJob) parseNamedayGuardianBlock(ctx context.Context, ch *models.Character, doc *goquery.Document, sel *goquery.Selection) error {
	birth := trim(sel.Find(".character-block__birth").Text())
	guardian := trim(sel.Find(".character-block__profile").Text())
	lib.GetLogger(ctx).Info("nameday/guardian", zap.String("birth", birth), zap.String("guardian", guardian))
	return nil
}

func (j FetchCharacterJob) parseCityStateBlock(ctx context.Context, ch *models.Character, doc *goquery.Document, sel *goquery.Selection) error {
	str := trim(sel.Find(".character-block__profile").Text())
	lib.GetLogger(ctx).Info("city-state", zap.String("city_state", str))
	return nil
}

func (j FetchCharacterJob) parseGrandCompanyBlock(ctx context.Context, ch *models.Character, doc *goquery.Document, sel *goquery.Selection) error {
	str := trim(sel.Find(".character-block__profile").Text())
	lib.GetLogger(ctx).Info("grand company", zap.String("str", str))

	parts := strings.SplitN(str, "/", 2)
	if l := len(parts); l != 2 {
		return errors.Errorf("couldn't parse grand company/rank: wrong number of items: %d", l)
	}
	gc := trim(parts[0])
	rank := trim(parts[1])
	lib.GetLogger(ctx).Info("grand company", zap.String("gc", gc), zap.String("rank", rank))
	return nil
}
