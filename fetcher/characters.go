package fetcher

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/liclac/gubal/lib"
	"github.com/liclac/gubal/models"
)

func init() { registerJob(func() Job { return &FetchCharacterJob{} }) }

// FetchCharacterJob fetches a character.
// A character that isn't found instead creates a CharacterTombstone in the database to signal this.
type FetchCharacterJob struct {
	ID    string `json:"id"`
	Force bool   `json:"force"`
}

// Type returns the type for a job.
func (FetchCharacterJob) Type() string { return "character" }

// Run runs the job.
func (j FetchCharacterJob) Run(ctx context.Context) (rjobs []Job, rerr error) {
	db := lib.GetDB(ctx)

	// Make sure only to request proper numbers as IDs.
	id, err := strconv.ParseInt(j.ID, 10, 64)
	if err != nil {
		return nil, err
	}

	// Check if the character has a tombstone, bail out if so.
	var tombstoneCount int
	if err := db.Model(models.CharacterTombstone{}).Where(models.CharacterTombstone{ID: id}).Count(&tombstoneCount).Error; err != nil {
		return nil, err
	}
	if tombstoneCount > 0 {
		return nil, nil
	}

	// Run in a transaction.
	db = db.Begin()
	ctx = lib.WithDB(ctx, db)
	defer func() {
		if rerr != nil {
			rerr = multierr.Append(rerr, db.Rollback().Error)
		} else {
			rerr = db.Commit().Error
		}
	}()

	// Read the character's public status page.
	req, err := http.NewRequest("GET", LodestoneBaseURL+"/character/"+j.ID+"/", nil)
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
		lib.GetLogger(ctx).Warn("Character does not exist", zap.Int64("id", id))
		return nil, db.Create(&models.CharacterTombstone{ID: id, StatusCode: resp.StatusCode}).Error
	default:
		return nil, errors.Errorf("incorrect HTTP status code when fetching character data: %d", resp.StatusCode)
	}

	// Read our existing data about this character.
	var char models.Character
	if err := db.FirstOrCreate(&char, map[string]interface{}{"id": id}).Error; err != nil {
		return nil, err
	}

	// Actually parse the page! Parsing steps are split into smaller pieces for maintainability,
	// and are combined into one big multierr so we can check them all in one fell swoop.
	doc, err := goquery.NewDocumentFromReader(bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	if err := multierr.Combine(
		j.parseName(&char, doc),
	); err != nil {
		return nil, err
	}

	return nil, db.Save(&char).Error
}

// parseName parses the character's FirstName and LastName from the page.
func (FetchCharacterJob) parseName(ch *models.Character, doc *goquery.Document) error {
	fullName := strings.TrimSpace(doc.Find(".frame__chara__name").First().Text())
	firstAndLast := strings.SplitN(fullName, " ", 2)
	if l := len(firstAndLast); l != 2 {
		return errors.Errorf("malformed name: \"%s\" (%d components)", fullName, l)
	}
	ch.FirstName = firstAndLast[0]
	ch.LastName = firstAndLast[1]
	return nil
}
