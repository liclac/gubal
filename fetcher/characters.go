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
	); err != nil {
		return nil, err
	}

	return nil, ds.Characters().Save(&char)
}

// parseName parses the character's FirstName and LastName from the page.
func (FetchCharacterJob) parseName(ctx context.Context, ch *models.Character, doc *goquery.Document) error {
	fullName := strings.TrimSpace(doc.Find(".frame__chara__name").First().Text())
	firstAndLast := strings.SplitN(fullName, " ", 2)
	if l := len(firstAndLast); l != 2 {
		return errors.Errorf("malformed name: \"%s\" (%d components)", fullName, l)
	}
	ch.FirstName = firstAndLast[0]
	ch.LastName = firstAndLast[1]
	return nil
}

// parseTitle parses the character's title from the page; titles are stored externally.
func (FetchCharacterJob) parseTitle(ctx context.Context, ch *models.Character, doc *goquery.Document) error {
	titleStr := strings.TrimSpace(doc.Find(".frame__chara__title").First().Text())
	if titleStr == "" {
		ch.Title = nil
		ch.TitleID = null.Int{}
		return nil
	}
	title, err := models.GetDataStore(ctx).CharacterTitles().GetOrCreate(titleStr)
	ch.Title = title
	return err
}
