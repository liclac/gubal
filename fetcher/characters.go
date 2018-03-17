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

	idStr := strconv.FormatInt(j.ID, 10)
	lib.GetLogger(ctx).Info("Fetching Character", zap.Int64("id", j.ID))

	// Check if the character has a tombstone, bail out if so.
	dead, err := ds.CharacterTombstones().Check(j.ID)
	if dead || err != nil {
		return nil, err
	}

	// Read the character's public status page.
	req, err := http.NewRequest("GET", LodestoneBaseURL+"/character/"+idStr+"/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	// resp, err := http.DefaultClient.Do(req)
	resp, err := doRequestWithCache(GetCacheFS(ctx), "char_"+idStr, req)
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
		j.parseWorld(ctx, &char, doc),
		j.parseBlocks(ctx, &char, doc),
		j.parseJobs(ctx, &char, doc),
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

func (j FetchCharacterJob) parseWorld(ctx context.Context, ch *models.Character, doc *goquery.Document) error {
	world := trim(doc.Find(".frame__chara__world").First().Text())
	switch world {
	case "Aegis":
		ch.World = models.Aegis
	case "Atomos":
		ch.World = models.Atomos
	case "Carbuncle":
		ch.World = models.Carbuncle
	case "Garuda":
		ch.World = models.Garuda
	case "Gungnir":
		ch.World = models.Gungnir
	case "Kujata":
		ch.World = models.Kujata
	case "Ramuh":
		ch.World = models.Ramuh
	case "Tonberry":
		ch.World = models.Tonberry
	case "Typhon":
		ch.World = models.Typhon
	case "Unicorn":
		ch.World = models.Unicorn
	case "Alexander":
		ch.World = models.Alexander
	case "Bahamut":
		ch.World = models.Bahamut
	case "Durandal":
		ch.World = models.Durandal
	case "Fenrir":
		ch.World = models.Fenrir
	case "Ifrit":
		ch.World = models.Ifrit
	case "Ridill":
		ch.World = models.Ridill
	case "Tiamat":
		ch.World = models.Tiamat
	case "Ultima":
		ch.World = models.Ultima
	case "Valefor":
		ch.World = models.Valefor
	case "Yojimbo":
		ch.World = models.Yojimbo
	case "Zeromus":
		ch.World = models.Zeromus
	case "Anima":
		ch.World = models.Anima
	case "Asura":
		ch.World = models.Asura
	case "Belias":
		ch.World = models.Belias
	case "Chocobo":
		ch.World = models.Chocobo
	case "Hades":
		ch.World = models.Hades
	case "Ixion":
		ch.World = models.Ixion
	case "Mandragora":
		ch.World = models.Mandragora
	case "Masamune":
		ch.World = models.Masamune
	case "Pandaemonium":
		ch.World = models.Pandaemonium
	case "Shinryu":
		ch.World = models.Shinryu
	case "Titan":
		ch.World = models.Titan
	case "Adamantoise":
		ch.World = models.Adamantoise
	case "Balmung":
		ch.World = models.Balmung
	case "Cactuar":
		ch.World = models.Cactuar
	case "Coeurl":
		ch.World = models.Coeurl
	case "Faerie":
		ch.World = models.Faerie
	case "Gilgamesh":
		ch.World = models.Gilgamesh
	case "Goblin":
		ch.World = models.Goblin
	case "Jenova":
		ch.World = models.Jenova
	case "Mateus":
		ch.World = models.Mateus
	case "Midgardsormr":
		ch.World = models.Midgardsormr
	case "Sargatanas":
		ch.World = models.Sargatanas
	case "Siren":
		ch.World = models.Siren
	case "Zalera":
		ch.World = models.Zalera
	case "Behemoth":
		ch.World = models.Behemoth
	case "Brynhildr":
		ch.World = models.Brynhildr
	case "Diabolos":
		ch.World = models.Diabolos
	case "Excalibur":
		ch.World = models.Excalibur
	case "Exodus":
		ch.World = models.Exodus
	case "Famfrit":
		ch.World = models.Famfrit
	case "Hyperion":
		ch.World = models.Hyperion
	case "Lamia":
		ch.World = models.Lamia
	case "Leviathan":
		ch.World = models.Leviathan
	case "Malboro":
		ch.World = models.Malboro
	case "Ultros":
		ch.World = models.Ultros
	case "Cerberus":
		ch.World = models.Cerberus
	case "Lich":
		ch.World = models.Lich
	case "Louisoix":
		ch.World = models.Louisoix
	case "Moogle":
		ch.World = models.Moogle
	case "Odin":
		ch.World = models.Odin
	case "Omega":
		ch.World = models.Omega
	case "Phoenix":
		ch.World = models.Phoenix
	case "Ragnarok":
		ch.World = models.Ragnarok
	case "Shiva":
		ch.World = models.Shiva
	case "Zodiark":
		ch.World = models.Zodiark
	default:
		return errors.Errorf("unknown world: '%s'", world)
	}
	return nil
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

	// Split it up into parts.
	parts := strings.SplitN(str, "/", 2)
	if l := len(parts); l != 2 {
		return errors.Errorf("couldn't parse race/clan/gender; wrong number of items: %d", l)
	}
	raceClan := trim(parts[0])
	gender := trim(parts[1])

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
	// birth := trim(sel.Find(".character-block__birth").Text())
	guardian := trim(sel.Find(".character-block__profile").Text())

	switch {
	case strings.HasPrefix(guardian, "Halone"):
		ch.Guardian = models.Halone
	case strings.HasPrefix(guardian, "Menphina"):
		ch.Guardian = models.Menphina
	case strings.HasPrefix(guardian, "Thaliak"):
		ch.Guardian = models.Thaliak
	case strings.HasPrefix(guardian, "Nymeia"):
		ch.Guardian = models.Nymeia
	case strings.HasPrefix(guardian, "Llymlaen"):
		ch.Guardian = models.Llymlaen
	case strings.HasPrefix(guardian, "Oschon"):
		ch.Guardian = models.Oschon
	case strings.HasPrefix(guardian, "Byregot"):
		ch.Guardian = models.Byregot
	case strings.HasPrefix(guardian, "Rhalgr"):
		ch.Guardian = models.Rhalgr
	case strings.HasPrefix(guardian, "Azeyma"):
		ch.Guardian = models.Azeyma
	case strings.HasPrefix(guardian, "Nald'thal"):
		ch.Guardian = models.Naldthal
	case strings.HasPrefix(guardian, "Nophica"):
		ch.Guardian = models.Nophica
	case strings.HasPrefix(guardian, "Althyk"):
		ch.Guardian = models.Althyk
	default:
		return errors.Errorf("unknown guardian: '%s'", guardian)
	}
	return nil
}

func (j FetchCharacterJob) parseCityStateBlock(ctx context.Context, ch *models.Character, doc *goquery.Document, sel *goquery.Selection) error {
	cityState := trim(sel.Find(".character-block__profile").Text())
	switch cityState {
	case "Gridania":
		ch.CityState = models.Gridania
	case "Ul'dah":
		ch.CityState = models.Uldah
	case "Limsa Lominsa":
		ch.CityState = models.Limsa
	default:
		return errors.Errorf("unknown city state: '%s'", cityState)
	}
	return nil
}

func (j FetchCharacterJob) parseGrandCompanyBlock(ctx context.Context, ch *models.Character, doc *goquery.Document, sel *goquery.Selection) error {
	str := trim(sel.Find(".character-block__profile").Text())

	parts := strings.SplitN(str, "/", 2)
	if l := len(parts); l != 2 {
		return errors.Errorf("couldn't parse grand company/rank: wrong number of items: %d", l)
	}
	gcName := trim(parts[0])
	rankName := trim(parts[1])

	switch gcName {
	case "Maelstrom":
		gc := models.Maelstrom
		ch.GC = &gc
	case "Order of the Twin Adder":
		gc := models.Adders
		ch.GC = &gc
	case "Immortal Flames":
		gc := models.Flames
		ch.GC = &gc
	}

	switch {
	case rankName == "":
	case strings.HasSuffix(rankName, "Private Third Class"):
		ch.GCRank = 1
	case strings.HasSuffix(rankName, "Private Second Class"):
		ch.GCRank = 2
	case strings.HasSuffix(rankName, "Private First Class"):
		ch.GCRank = 3
	case strings.HasSuffix(rankName, "Corporal"):
		ch.GCRank = 4
	case strings.HasSuffix(rankName, "Sergeant Third Class"):
		ch.GCRank = 5
	case strings.HasSuffix(rankName, "Sergeant Second Class"):
		ch.GCRank = 6
	case strings.HasSuffix(rankName, "Sergeant First Class"):
		ch.GCRank = 7
	case strings.HasPrefix(rankName, "Chief") && strings.HasSuffix(rankName, "Sergeant"):
		ch.GCRank = 8
	case strings.HasPrefix(rankName, "Second") && strings.HasSuffix(rankName, "Lieutenant"):
		ch.GCRank = 9
	case strings.HasPrefix(rankName, "First") && strings.HasSuffix(rankName, "Lieutenant"):
		ch.GCRank = 10
	}

	return nil
}

func (j FetchCharacterJob) parseJobs(ctx context.Context, ch *models.Character, doc *goquery.Document) error {
	var errs []error
	doc.Find("ul.character__job li").Each(func(i int, sel *goquery.Selection) {
		levelObj := models.Level{CharacterID: ch.ID}

		// Parse level, skip over not yet unlocked jobs.
		levelStr := trim(sel.Find(".character__job__level").First().Text())
		if levelStr == "-" || levelStr == "" {
			return
		}
		level, err := strconv.ParseInt(levelStr, 10, 64)
		if err != nil {
			return
		}
		levelObj.Level = int(level)

		// Parse the job name.
		jobName := trim(sel.Find(".character__job__name").First().Text())
		switch jobName {
		case "":
			return
		case "Paladin", "Gladiator":
			levelObj.Job = models.PLD
		case "Warrior", "Marauder":
			levelObj.Job = models.WAR
		case "Dark Knight":
			levelObj.Job = models.DRK
		case "White Mage", "Conjurer":
			levelObj.Job = models.WHM
		case "Scholar":
			levelObj.Job = models.SCH
		case "Astrologian":
			levelObj.Job = models.AST
		case "Monk", "Pugilist":
			levelObj.Job = models.MNK
		case "Dragoon", "Lancer":
			levelObj.Job = models.DRG
		case "Ninja", "Rogue":
			levelObj.Job = models.NIN
		case "Samurai":
			levelObj.Job = models.SAM
		case "Bard", "Archer":
			levelObj.Job = models.BRD
		case "Machinist":
			levelObj.Job = models.MCH
		case "Black Mage", "Thaumaturge":
			levelObj.Job = models.BLM
		case "Summoner", "Arcanist":
			levelObj.Job = models.SMN
		case "Red Mage":
			levelObj.Job = models.RDM
		case "Carpenter":
			levelObj.Job = models.CRP
		case "Blacksmith":
			levelObj.Job = models.BSM
		case "Armorer":
			levelObj.Job = models.ARM
		case "Goldsmith":
			levelObj.Job = models.GSM
		case "Leatherworker":
			levelObj.Job = models.LTW
		case "Weaver":
			levelObj.Job = models.WVR
		case "Alchemist":
			levelObj.Job = models.ALC
		case "Culinarian":
			levelObj.Job = models.CUL
		case "Miner":
			levelObj.Job = models.MIN
		case "Botanist":
			levelObj.Job = models.BOT
		case "Fisher":
			levelObj.Job = models.FSH
		default:
			errs = append(errs, errors.Errorf("unknown job: '%s'", jobName))
			return
		}

		errs = append(errs, models.GetDataStore(ctx).Levels().Set(&levelObj))
	})
	return multierr.Combine(errs...)
}
