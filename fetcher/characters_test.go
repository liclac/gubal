package fetcher

import (
	"context"
	"database/sql/driver"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"

	"github.com/liclac/gubal/lib"
	"github.com/liclac/gubal/models"
)

func TestFetchCharacterJob(t *testing.T) {
	testdata := map[string]models.Character{
		testHTMLEmiHawke: {
			ID:        7248246,
			FirstName: "Emi",
			LastName:  "Hawke",
		},
	}
	phases := []struct {
		Name     string
		Existing bool
	}{
		{"Insert", false},
		{"Update", true},
	}
	for html, expected := range testdata {
		testName := fmt.Sprintf("%d_%s_%s", expected.ID, expected.FirstName, expected.LastName)
		t.Run(testName, func(t *testing.T) {
			for _, phase := range phases {
				t.Run(phase.Name, func(t *testing.T) {
					testsrv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
						require.Equal(t, fmt.Sprintf("/character/%d/", expected.ID), req.URL.Path)
						require.Equal(t, UserAgent, req.Header.Get("User-Agent"))
						fmt.Fprint(rw, html)
					}))
					realLodestoneBaseURL := LodestoneBaseURL
					LodestoneBaseURL = testsrv.URL
					defer func() {
						testsrv.Close()
						LodestoneBaseURL = realLodestoneBaseURL
					}()

					mockdb, mock, err := sqlmock.New()
					require.NoError(t, err)

					db, err := gorm.Open("postgres", mockdb)
					require.NoError(t, err)

					ctx := context.Background()
					ctx = lib.WithDB(ctx, db)

					allColumns := []string{"id", "created_at", "updated_at", "first_name", "last_name"}
					allValues := []driver.Value{expected.ID, expected.CreatedAt, expected.UpdatedAt, expected.FirstName, expected.LastName}

					// The job starts by checking if there's a tombstone.
					// We're testing tombstones separately, ignore them here.
					mock.ExpectQuery(
						`SELECT count\(\*\) FROM "character_tombstones" WHERE \("character_tombstones"."id" = \$1\)`,
					).WithArgs(expected.ID).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

					// The rest of the job runs in a transaction.
					mock.ExpectBegin()
					{
						// It checks if there's an existing record; depending on the phase, we
						// return either nothing, or the old (= expected) record.
						existingRows := sqlmock.NewRows(allColumns)
						if phase.Existing {
							existingRows.AddRow(allValues...)
						}
						mock.ExpectQuery(
							`SELECT \* FROM "characters" WHERE \("characters"."id" = \$1\) ORDER BY "characters"."id" ASC LIMIT 1`,
						).WithArgs(expected.ID).WillReturnRows(existingRows)

						// If the previous query returns no rows, gorm does an insert.
						if !phase.Existing {
							mock.ExpectQuery(
								`INSERT INTO "characters" \("id","created_at","updated_at","first_name","last_name"\) VALUES \(\$1,\$2,\$3,\$4,\$5\) RETURNING "characters"."id"`,
							).WithArgs(
								expected.ID,
								sqlmock.AnyArg(),
								sqlmock.AnyArg(),
								"",
								"",
							).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(expected.ID))
						}

						// Finally, the record is updated. This could definitely be optimised away.
						mock.ExpectExec(
							`UPDATE "characters" SET "created_at" = \$1, "updated_at" = \$2, "first_name" = \$3, "last_name" = \$4 WHERE "characters"."id" = \$5`,
						).WithArgs(
							sqlmock.AnyArg(),
							sqlmock.AnyArg(),
							expected.FirstName,
							expected.LastName,
							expected.ID,
						).WillReturnResult(sqlmock.NewResult(expected.ID, 1))
					}
					mock.ExpectCommit()

					job := FetchCharacterJob{ID: fmt.Sprint(expected.ID)}
					jobs, err := job.Run(ctx)
					require.NoError(t, err)
					assert.Len(t, jobs, 0)
					assert.NoError(t, mock.ExpectationsWereMet())
				})
			}
		})
	}

	t.Run("NotFound", func(t *testing.T) {
		id := int64(3)

		testsrv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(http.StatusNotFound)
		}))
		realLodestoneBaseURL := LodestoneBaseURL
		LodestoneBaseURL = testsrv.URL
		defer func() {
			testsrv.Close()
			LodestoneBaseURL = realLodestoneBaseURL
		}()

		mockdb, mock, err := sqlmock.New()
		require.NoError(t, err)

		db, err := gorm.Open("postgres", mockdb)
		require.NoError(t, err)

		ctx := context.Background()
		ctx = lib.WithDB(ctx, db)

		// The job starts by checking if there's a tombstone.
		// There's none yet, but we're about to create one.
		mock.ExpectQuery(
			`SELECT count\(\*\) FROM "character_tombstones" WHERE \("character_tombstones"."id" = \$1\)`,
		).WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		// The rest of the job runs in a transaction.
		mock.ExpectBegin()
		{
			// The page will 404, so a tombstone is created.
			mock.ExpectQuery(
				`INSERT INTO "character_tombstones" \("id","created_at","status_code"\) VALUES \(\$1,\$2,\$3\) RETURNING "character_tombstones"."id"`,
			).WithArgs(
				id,
				sqlmock.AnyArg(),
				http.StatusNotFound,
			).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(id))
		}
		mock.ExpectCommit()

		job := FetchCharacterJob{ID: fmt.Sprint(id)}
		jobs, err := job.Run(ctx)
		require.NoError(t, err)
		assert.Len(t, jobs, 0)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Tombstone", func(t *testing.T) {
		id := int64(3)

		// Crash the test if any lodestone requests are actually sent.
		testsrv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			require.FailNow(t, "no http requests should be made here")
		}))
		realLodestoneBaseURL := LodestoneBaseURL
		LodestoneBaseURL = testsrv.URL
		defer func() {
			testsrv.Close()
			LodestoneBaseURL = realLodestoneBaseURL
		}()

		mockdb, mock, err := sqlmock.New()
		require.NoError(t, err)

		db, err := gorm.Open("postgres", mockdb)
		require.NoError(t, err)

		ctx := context.Background()
		ctx = lib.WithDB(ctx, db)

		// The job starts by checking if there's a tombstone.
		// We're signalling that there is one, so the job should bail out.
		mock.ExpectQuery(
			`SELECT count\(\*\) FROM "character_tombstones" WHERE \("character_tombstones"."id" = \$1\)`,
		).WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		job := FetchCharacterJob{ID: fmt.Sprint(id)}
		jobs, err := job.Run(ctx)
		require.NoError(t, err)
		assert.Len(t, jobs, 0)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

const testHTMLEmiHawke = `<!DOCTYPE html>
<html lang="en-us" class="en-us" xmlns:og="http://ogp.me/ns#" xmlns:fb="http://www.facebook.com/2008/fbml">
<head><meta charset="utf-8">
<title>Emi Hawke | FINAL FANTASY XIV, The Lodestone</title>
<meta name="description" content="Character profile for Emi Hawke.">
<meta name="keywords" content="FF14,FFXIV,Final Fantasy XIV,Final Fantasy 14,Lodestone,players' site,community site,A Realm Reborn,Heavensward,Stormblood,MMO">
<meta name="author" content="SQUARE ENIX Inc.">
<!-- ** CSS ** -->


<style>
#header{
	position:fixed;
	top:0;
	left:0;
}
</style>

<link href="https://img.finalfantasyxiv.com/lds/h/S/PPl6ixyoIEkP_hxbFOIMQvhWsg.css" rel="stylesheet">
<link href="https://img.finalfantasyxiv.com/lds/h/0/VDvDQwppZ6JadVwlxJiT2UPf84.css" rel="stylesheet">
<link href="https://img.finalfantasyxiv.com/lds/h/w/aS4M2VNMcQcFpJ3WGQOY_vEuBU.css" rel="stylesheet">
<link href="https://img.finalfantasyxiv.com/lds/h/8/yr2AhcegQ84o8uTpgIojdR0R4M.css" rel="stylesheet">

<link href="https://img.finalfantasyxiv.com/lds/h/A/9PnNifbAX8iJk1CPclwsUrJ62c.css" rel="stylesheet">



<link href="https://img.finalfantasyxiv.com/lds/h/3/K1dI0S8bpVvH9SK9x6wPtTKjPU.css" rel="stylesheet"
	class="sys_theme_css"
	
		data-theme_white="https://img.finalfantasyxiv.com/lds/h/3/K1dI0S8bpVvH9SK9x6wPtTKjPU.css"
	
		data-theme_black="https://img.finalfantasyxiv.com/lds/h/f/J5Dvkb3pXnhavacQCxFZco9AGs.css"
	
>

<link href="https://img.finalfantasyxiv.com/lds/h/p/F746cHQD4GHo1y7dWDz4N7egXI.css" rel="stylesheet">


<link href="https://img.finalfantasyxiv.com/lds/h/H/_SHjrsr83qTHCKhijAGgZUPJLI.css" rel="stylesheet">









<meta name="viewport" content="width=device-width,initial-scale=1,maximum-scale=1,user-scalable=no">
<meta name="format-detection" content="telephone=no">

<link rel="shortcut icon" type="image/vnd.microsoft.icon" href="https://img.finalfantasyxiv.com/lds/sp/global/images/favicon.ico?1490749553">
<link rel="apple-touch-icon-precomposed" href="https://img.finalfantasyxiv.com/lds/h/0/U2uGfVX4GdZgU1jASO0m9h_xLg.png">

<link rel="alternate" hreflang="ja" href="https://jp.finalfantasyxiv.com/lodestone/character/7248246/">
<link rel="alternate" hreflang="en-gb" href="https://eu.finalfantasyxiv.com/lodestone/character/7248246/">
<link rel="alternate" hreflang="fr" href="https://fr.finalfantasyxiv.com/lodestone/character/7248246/">
<link rel="alternate" hreflang="de" href="https://de.finalfantasyxiv.com/lodestone/character/7248246/">






<!-- ogp -->
<meta property="fb:app_id" content="1400886233468625">
<meta property="og:type" content="website">
<meta property="og:description" content="Character profile for Emi Hawke.">
<meta property="og:title" content="Emi Hawke | FINAL FANTASY XIV, The Lodestone">
<meta property="og:url" content="https://na.finalfantasyxiv.com/lodestone/character/7248246/">
<meta property="og:site_name" content="FINAL FANTASY XIV, The Lodestone">
<meta property="og:locale" content="en_US">

	<meta property="og:image" content="https://img.finalfantasyxiv.com/lds/h/Y/Ai82iNN8Ri5I0GVZ6qP0Ep-gHU.png">


<meta name="twitter:card" content="summary_large_image">

	<meta name="twitter:site" content="@ff_xiv_en">





<script>
	var base_domain = 'finalfantasyxiv.com';
	var strftime_fmt = {
		'dateHHMM_now': 'Today %H:%M',
		'dateYMDHMS': '%m/%d/%Y %\u002dI:%M:%S %p',
		'dateYMDHM': '%m/%d/%Y %\u002dI:%M %p',
		'dateHM': '%\u002dI:%M %p',
		'dateYMDH': '%m/%d/%Y %\u002dI',
		'dateYMD': '%m/%d/%Y',
		'dateEternal': '%m.%d.%Y',
		'dateYMDW': '%m/%d/%Y (%a)',
		'dateHM': '%\u002dI:%M %p',
		'week.0': 'Sun.',
		'week.1': 'Mon.',
		'week.2': 'Tue.',
		'week.3': 'Wed.',
		'week.4': 'Thu.',
		'week.5': 'Fri.',
		'week.6': 'Sat.'
	};
	var base_uri   = '/lodestone/';
	var api_uri    = '/lodestone/api/';
	var csrf_token = '145abae05121cb8a5a9fa48e4fc37eddd319c9d9';
	var cookie_suffix = '';
</script>
<script src="https://img.finalfantasyxiv.com/lds/h/A/PknAmzDJUZCNhTGtSGGMIGi5k4.js"></script>
<script src="https://img.finalfantasyxiv.com/lds/h/R/bK43fUDSheQh717GUPX94CW_1Y.js"></script>








</head>
<body>
<div id="fb-root"></div>
<script>(function(d, s, id) {
  var js, fjs = d.getElementsByTagName(s)[0];
  if (d.getElementById(id)) return;
  js = d.createElement(s); js.id = id;
  
    js.src = "//connect.facebook.net/en_US/sdk.js#xfbml=1&version=v2.3&appId=1400886233468625";
  
  fjs.parentNode.insertBefore(js, fjs);
}(document, 'script', 'facebook-jssdk'));</script>
<a name="pageTop" id="pageTop"></a><header id="header" class="l__header js--global_header"><div class="l__header__bt_menu js--global_menu"></div><div class="l__header__logo"><a href="/lodestone/"><img src="https://img.finalfantasyxiv.com/lds/h/Z/-zkJHjXccKSftFl0KmMmrjjf-s.png" width="140" height="45" alt=""></a></div><div class="l__header__bt_login__wrapper"><a href="/lodestone/account/login/" class="l__header__bt_login"></a></div></header><div class="global_menu__overlay"></div><div class="global_menu"><div class="global_menu__scroll_area"><div class="global_menu__inner"><div class="global_menu__header"><a href="/lodestone/" class="global_menu__bt_home" onClick="ldst_ga('send', 'event', 'lodestone_lo', 'sp_menu', 'en-us_top');"></a><form action="/lodestone/community/search/" class="global_menu__search"><input type="text" name="q" placeholder="Search"></form></div><div class="global_menu__body"><div class="global_menu__banner"><a href="/lodestone/ranking/thefeast/?utm_source=lodestone_lo&amp;utm_medium=sp_menu&amp;utm_campaign=na_feast"><img src="https://img.finalfantasyxiv.com/lds/banner/339/SP_ザ_フィーストランキング_na.png" width="280" height="60" alt=""></a></div><dl class="global_menu__list"><dt class="global_menu__list__category"><a href="/lodestone/news/" onClick="ldst_ga('send', 'event', 'lodestone_lo', 'sp_menu', 'en-us_news');"><div>News</div></a></dt><dd class="global_menu__list__item"><ul><li><a href="/lodestone/special/patchnote_log/" onClick="ldst_ga('send', 'event', 'lodestone_lo', 'sp_menu', 'en-us_patchnote');"><div>Patch Notes</div></a></li><li><a href="/lodestone/special/update_log/" onClick="ldst_ga('send', 'event', 'lodestone_lo', 'sp_menu', 'en-us_updatelog');"><div>Site Update Information</div></a></li></ul></dd><dt class="global_menu__list__category"><a href="/" onClick="ldst_ga('send', 'event', 'lodestone_lo', 'sp_menu', 'en-us_pr');"><div>FINAL FANTASY XIV</div></a></dt><dd class="global_menu__list__item"><ul><li><a href="/stormblood/" onClick="ldst_ga('send', 'event', 'lodestone_lo', 'sp_menu', 'en-us_pr_sb');"><div>Stormblood</div></a></li><li><a href="/product/" onClick="ldst_ga('send', 'event', 'lodestone_lo', 'sp_menu', 'en-us_prduct');"><div>Product</div></a></li><li><a href="/pr/sp/start_ffxiv/" onClick="ldst_ga('send', 'event', 'lodestone_lo', 'sp_menu', 'en-us_startffxiv');"><div>FINAL FANTASY XIV: A Beginner's Guide</div></a></li><li><a href="http://freetrial.finalfantasyxiv.com/na/?utm_source=lodestone&utm_campaign=freetrial-banner&utm_medium=organic&utm_content=menu" onClick="ldst_ga('send', 'event', 'lodestone_lo', 'sp_menu', 'en-us_freetrial');"><div>Free Trial</div></a></li></ul></dd><dt class="global_menu__list__category"><a href="/lodestone/playguide/" onClick="ldst_ga('send', 'event', 'lodestone_lo', 'sp_menu', 'en-us_playguide');"><div>Play Guide</div></a></dt><dd class="global_menu__list__item"><ul><li><a href="/jobguide/battle/" onClick="ldst_ga('send', 'event', 'lodestone_lo', 'sp_menu', 'en-us_jobguide');"><div>Job Guide</div></a></li></ul></dd><dt class="global_menu__list__category"><a href="/lodestone/playguide/db/" onClick="ldst_ga('send', 'event', 'lodestone_lo', 'sp_menu', 'en-us_edb');"><div>Eorzea Database</div></a><a href="http://forum.square-enix.com/ffxiv/forum.php" target="_blank" onClick="ldst_ga('send', 'event', 'lodestone_lo', 'sp_menu', 'en-us_forum');"><div>Forums</div></a><a href="/lodestone/community/" onClick="ldst_ga('send', 'event', 'lodestone_lo', 'sp_menu', 'en-us_community');"><div>Community</div></a></dt><dd class="global_menu__list__item"><ul><li><a href="/lodestone/blog/" onClick="ldst_ga('send', 'event', 'lodestone_lo', 'sp_menu', 'en-us_blog');"><div>Blog</div></a></li><li><a href="/pr/blog/" onClick="ldst_ga('send', 'event', 'lodestone_lo', 'sp_menu', 'en-us_official_blog');"><div>Official Blog</div></a></li><li><a href="/lodestone/special/fankit/" onClick="ldst_ga('send', 'event', 'lodestone_lo', 'sp_menu', 'en-us_official_fankit');"><div>Fan Kit</div></a></li></ul></dd><dt class="global_menu__list__category"><a href="/lodestone/ranking" onClick="ldst_ga('send', 'event', 'lodestone_lo', 'sp_menu', 'en-us_ranking');"><div>Standings</div></a><a href="/lodestone/help/" onClick="ldst_ga('send', 'event', 'lodestone_lo', 'sp_menu', 'en-us_help');"><div>Help & Support</div></a></dt><dd class="global_menu__list__item"><ul><li><a href="http://sqex.to/Msp" target="_blank" onClick="ldst_ga('send', 'event', 'lodestone_lo', 'sp_menu', 'en-us_mogstation');"><div>Mog Station</div></a></li><li><a href="http://www.square-enix.com/na/account/" onClick="ldst_ga('send', 'event', 'lodestone_lo', 'sp_menu', 'en-us_sqex_account');"><div>Square Enix Account</div></a></li></ul></dd></dl><h2 class="global_menu__title">Settings</h2><ul class="global_menu__list global_menu__list__other"><li><dl class="global_menu__language"><dt class="global_menu__language__flag_na js--language__trigger">English</dt><dd class="js--language__select"><ul><li><a href="https://jp.finalfantasyxiv.com/lodestone/character/7248246/" class="global_menu__language__flag_jp" onClick="ldst_ga('send', 'event', 'lodestone_lo', 'sp_menu', 'en-us_lang_jp');">日本語</a></li><li class="global_menu__language__checked"><span class="global_menu__language__flag_na">English</span></li><li><a href="https://eu.finalfantasyxiv.com/lodestone/character/7248246/" class="global_menu__language__flag_eu" onClick="ldst_ga('send', 'event', 'lodestone_lo', 'sp_menu', 'en-us_lang_eu');">English</a></li><li><a href="https://fr.finalfantasyxiv.com/lodestone/character/7248246/" class="global_menu__language__flag_fr" onClick="ldst_ga('send', 'event', 'lodestone_lo', 'sp_menu', 'en-us_lang_fr');">Français</a></li><li><a href="https://de.finalfantasyxiv.com/lodestone/character/7248246/" class="global_menu__language__flag_de" onClick="ldst_ga('send', 'event', 'lodestone_lo', 'sp_menu', 'en-us_lang_de');">Deutsch</a></li></ul></dd></dl></li><li><dl class="global_menu__theme global_menu__list__other sys_theme_switcher"><dt>Theme</dt><dd><a href="javascript:void(0);"><i class="global_menu__theme--white sys_theme  active" data-theme="white"></i></a></dd><dd><a href="javascript:void(0);"><i class="global_menu__theme--black sys_theme " data-theme="black"></i></a></dd></dl></li></ul><h2 class="global_menu__title">Related Sites</h2><ul class="global_menu__list global_menu__list__other global_menu__list__other--site_links global_menu__list--last"><li><a href="http://sqex.to/buyffxiv" target="_blank" onClick="ldst_ga('send', 'event', 'lodestone_lo', 'sp_menu', 'en-us_estore');"><div>Square Enix Online Store</div></a></li></ul></div></div></div><div class="global_menu__close js--global_menu__close"></div></div><div class="ldst_main_content">
<!-- contents -->

<div class="ldst__bg">

	<h1 class="heading__title">
		
			Character
		
	</h1>

	
	<div class="ldst-nav__floating__icon"></div>
	<div class="ldst-nav__floating__list__overlay">
		<div class="ldst-nav__floating">
			<div class="ldst-nav__floating__list">
				<div class="ldst-nav__floating__scroll_area">
				<ul class="ldst-nav">
					
						<li><a href="#anchor__character">Character</a></li>
						<li><a href="#anchor__profile">Profile</a></li>
						<li><a href="#anchor__parameter">Attributes</a></li>
						<li><a href="#anchor__class">Class</a></li>
						
							<li><a href="#anchor__mounts">Mounts</a></li>
						
						
							<li><a href="#anchor__minions">Minions</a></li>
						
						
							<li><a href="/lodestone/character/7248246/achievement/" class="page_link">Achievements</a></li>
						
						
						
							<li><a href="/lodestone/character/7248246/following/" class="page_link">Follow</a></li>
						
					
				</ul>
				</div>
			</div>
		</div>
	</div>
	<div class="ldst-nav__floating__list__close"></div>


	
		<ul class="btn__menu-3">
			<li><a href="/lodestone/character/7248246/" class="btn__menu btn__menu--active">Profile</a></li>
			<li><a href="/lodestone/character/7248246/blog/" class="btn__menu">Blog</a></li>
			<li><a href="/lodestone/character/7248246/event/" class="btn__menu">Events</a></li>
		</ul>
		
	<div class="frame__chara" id="anchor__character">
		<a href="/lodestone/character/7248246/" class="frame__chara__link">
			<div class="frame__chara__face">
				<img src="https://img2.finalfantasyxiv.com/f/6aa9fdf84a8a3fbe7567055bcf81e129_c514cdcdb619439df97d906d4434ccc6fc0_96x96.jpg?1520352080" alt="">
			</div>
			<div class="frame__chara__box">
				<p class="frame__chara__title">Khloe&#39;s Friend</p>
				<p class="frame__chara__name">Emi Hawke</p>
				
				<p class="frame__chara__world">Ultros</p>
			</div>
		</a>
		<div class="parts__connect--state js__connect_btn">
			
			
				<i class="parts__connect_off"></i>
			
		</div>
		
			<div class="character__connect__view js__connect_view">
				
					<p class="character__connect__text">You have no connection with this character.</p>
				

				
					
						
							<div class="heading--md-slide">Follower Requests</div>
							<p class="entry__toggle__text">Follow this character?</p>
							<ul class="btn__color__nav--radius character__connect__form">
								<li><a href="/lodestone/account/login/">Yes</a></li>
								<li><a href="javascript:void(0)" class="js__connect_btn">No</a></li>
							</ul>
						
					
				
			</div>
		
	</div>



	

	






<div class="character__view"><div class="character__class"><div class="character__class__arms"><a href="/lodestone/character/7248246/equipment/0/" class="character__item_icon character__item_icon--0"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/f1/f19278b1526b2088fb88378043878bbf2162f4ff.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div><div class="ic_mirage_stain"><div class="mirage-staining-icon mirage-staining-icon--painted" style="background: #781a1a;"></div></div></a></div><div class="character__class__data"><p>LEVEL 70 </p><div class="character__class_icon"><img src="https://img.finalfantasyxiv.com/lds/h/d/8W89w_G-YGAN_ZRrdD5YL3LqWA.png" width="24" height="24" alt=""></div><img src="https://img.finalfantasyxiv.com/lds/h/V/iQGQZkgIcSv9ron84usFHDIi48.png" width="222" height="24" alt=""></div></div><div class="character__detail"><div class="character__detail__icon"><a href="/lodestone/character/7248246/equipment/2/" class="character__item_icon character__item_icon--2"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/50/50a062166a86674d2565621ff8e6fb483e09aa19.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div><div class="ic_mirage_stain"><div class="mirage-staining-icon mirage-staining-icon--painted" style="background: #1e1e1e;"></div></div></a><a href="/lodestone/character/7248246/equipment/3/" class="character__item_icon character__item_icon--3"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/c8/c8321e50abeae07f2e0073e858797809230aa6df.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div><div class="ic_mirage_stain"><div class="mirage-staining-icon mirage-staining-icon--painted" style="background: #781a1a;"></div></div></a><a href="/lodestone/character/7248246/equipment/4/" class="character__item_icon character__item_icon--4"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/ef/efa7989c4b209f9682c11dc2485fd2392c3ab756.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div><div class="ic_mirage_stain"><div class="mirage-staining-icon mirage-staining-icon--nopainted"></div></div></a><a href="/lodestone/character/7248246/equipment/5/" class="character__item_icon character__item_icon--5"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/56/56cb3a19b96f67bbee8e54663b56cc01be49dbbf.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></a><a href="/lodestone/character/7248246/equipment/6/" class="character__item_icon character__item_icon--6"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/67/67929ac37cd8d3b059ccdf2dd7c64a987aa94bb2.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div><div class="ic_mirage_stain"><div class="mirage-staining-icon mirage-staining-icon--nopainted"></div></div></a><a href="/lodestone/character/7248246/equipment/7/" class="character__item_icon character__item_icon--7"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/9f/9f332dd2e72b00b5a0e854c70545ae17017092a9.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div><div class="ic_mirage_stain"><div class="mirage-staining-icon mirage-staining-icon--nopainted"></div></div></a></div><div class="character__detail__image"><a href="https://img2.finalfantasyxiv.com/f/6aa9fdf84a8a3fbe7567055bcf81e129_c514cdcdb619439df97d906d4434ccc6fl0_640x873.jpg?1520352080" target="_blank"><img src="https://img2.finalfantasyxiv.com/f/6aa9fdf84a8a3fbe7567055bcf81e129_c514cdcdb619439df97d906d4434ccc6fl0_640x873.jpg?1520352080" width="220" height="300" alt=""></a></div><div class="character__detail__icon"><span class="character__item_icon character__item_icon--1"></span><a href="/lodestone/character/7248246/equipment/8/" class="character__item_icon character__item_icon--8"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/f0/f0975dddf9d07692aa5c0d786a8e96186bac1112.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></a><a href="/lodestone/character/7248246/equipment/9/" class="character__item_icon character__item_icon--9"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/22/223fa92a0951811d658ba9e7e28d407052d1dca8.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></a><a href="/lodestone/character/7248246/equipment/10/" class="character__item_icon character__item_icon--10"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/71/717ea404be0d2b2fa0a79760c0e86189da4124ff.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></a><a href="/lodestone/character/7248246/equipment/11/" class="character__item_icon character__item_icon--11"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/c9/c95eba6b993b584d33ddac27f0b0c539c6f1ae69.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></a><a href="/lodestone/character/7248246/equipment/12/" class="character__item_icon character__item_icon--12"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/dc/dc5cdbbe0588cac940d9ba68d86d52a1b3c667fd.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></a><a href="/lodestone/character/7248246/equipment/13/" class="character__item_icon character__item_icon--13"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/80/809367dbbe135a93d4ea8f90bca506d830149d19.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></a></div></div><div class="character__level clearfix"><div class="character__level__list"><h3 class="heading__item">Tank</h3><ul><li><span><img src="https://img.finalfantasyxiv.com/lds/h/f/y4GToCrk6Ti9xJeSDHWHjJVmoQ.png" width="24" height="24" alt=""></span>62</li><li><span><img src="https://img.finalfantasyxiv.com/lds/h/k/d6U_2PbjZIIv8JJ8-dweIGATlc.png" width="24" height="24" alt=""></span>60</li><li><span><img src="https://img.finalfantasyxiv.com/lds/h/A/rnuxXb8g_heKzVRK5xxtt4tnhQ.png" width="24" height="24" alt=""></span>33</li></ul></div><div class="character__level__list"><h3 class="heading__item">Healer</h3><ul><li><span><img src="https://img.finalfantasyxiv.com/lds/h/p/i7RE68GXG_dc4P9cA0QQWBpYM4.png" width="24" height="24" alt=""></span>51</li><li><span><img src="https://img.finalfantasyxiv.com/lds/h/s/-UlKB2SsH12g9Ftrp4QCPejw_U.png" width="24" height="24" alt=""></span>70</li><li><span><img src="https://img.finalfantasyxiv.com/lds/h/0/PHXicpkH8WxgMAtq2rL4QTTWWY.png" width="24" height="24" alt=""></span>50</li></ul></div></div><div class="character__level clearfix"><div class="character__level__list"><h3 class="heading__item">Melee DPS</h3><ul><li><span><img src="https://img.finalfantasyxiv.com/lds/h/C/Mgep5vWlYWV-RKurbW4j41_HuE.png" width="24" height="24" alt=""></span>68</li><li><span><img src="https://img.finalfantasyxiv.com/lds/h/f/wcMw-nrJKmVaAXRo5G4xw6-Njo.png" width="24" height="24" alt=""></span>60</li><li><span><img src="https://img.finalfantasyxiv.com/lds/h/M/5edkH3aPx5xzC7AK3tLhxfcIjQ.png" width="24" height="24" alt=""></span>61</li><li><span><img src="https://img.finalfantasyxiv.com/lds/h/A/CI_AlHO4FeG8MOTO5Z6ppgIwB0.png" width="24" height="24" alt=""></span>53</li></ul></div></div><div class="character__level clearfix"><div class="character__level__list"><h3 class="heading__item">Physical Ranged DPS</h3><ul><li><span><img src="https://img.finalfantasyxiv.com/lds/h/E/lXzIPJjpO_WHS_oNnLwjZSBR3Y.png" width="24" height="24" alt=""></span>38</li><li><span><img src="https://img.finalfantasyxiv.com/lds/h/A/Rn4tQPyamPT20uvxiH8gmtWAGw.png" width="24" height="24" alt=""></span>-</li></ul></div></div><div class="character__level clearfix"><div class="character__level__list"><h3 class="heading__item">Magical Ranged DPS</h3><ul><li><span><img src="https://img.finalfantasyxiv.com/lds/h/d/8W89w_G-YGAN_ZRrdD5YL3LqWA.png" width="24" height="24" alt=""></span>70</li><li><span><img src="https://img.finalfantasyxiv.com/lds/h/M/8Q3Bl-7KbRjzOuRVzFr3plem80.png" width="24" height="24" alt=""></span>70</li><li><span><img src="https://img.finalfantasyxiv.com/lds/h/U/cJaVYjtqo9C7GS9mIfXehgUjpc.png" width="24" height="24" alt=""></span>52</li></ul></div></div><div class="character__level clearfix"><div class="character__level__list"><h3 class="heading__item">Disciples of the Hand</h3><ul><li><span><img src="https://img.finalfantasyxiv.com/lds/h/6/WzZgbzGmOSUFWpxECzxjVRdGyE.png" width="24" height="24" alt=""></span>-</li><li><span><img src="https://img.finalfantasyxiv.com/lds/h/h/OserTu_uxGInTSZ2WbNvd5p2A8.png" width="24" height="24" alt=""></span>-</li><li><span><img src="https://img.finalfantasyxiv.com/lds/h/1/sm1X5o_2dZDEqrdjnftOWNMMKU.png" width="24" height="24" alt=""></span>14</li><li><span><img src="https://img.finalfantasyxiv.com/lds/h/o/6uBa8sibvDu_EsRwe0jfzRKuu4.png" width="24" height="24" alt=""></span>-</li><li><span><img src="https://img.finalfantasyxiv.com/lds/h/e/Ww9S9TDqDwrVipK7FUAQS1pZTc.png" width="24" height="24" alt=""></span>50</li><li><span><img src="https://img.finalfantasyxiv.com/lds/h/z/RjLQxp4sHP2WCUOAfVL-D7gQcQ.png" width="24" height="24" alt=""></span>16</li><li><span><img src="https://img.finalfantasyxiv.com/lds/h/3/ZPympy0DnMkZJVf3BGA-ar3PwQ.png" width="24" height="24" alt=""></span>17</li><li><span><img src="https://img.finalfantasyxiv.com/lds/h/A/m2BECCaj0QbKN8wI4s38PrgRxo.png" width="24" height="24" alt=""></span>12</li></ul></div></div><div class="character__level clearfix"><div class="character__level__list"><h3 class="heading__item">Disciples of the Land</h3><ul><li><span><img src="https://img.finalfantasyxiv.com/lds/h/a/GHx1Bej_ySjgCjFWbGAv-PwBaw.png" width="24" height="24" alt=""></span>60</li><li><span><img src="https://img.finalfantasyxiv.com/lds/h/9/QgCWQNT-CaJEbJkg478kiw3Wfs.png" width="24" height="24" alt=""></span>13</li><li><span><img src="https://img.finalfantasyxiv.com/lds/h/E/PaI1B9Yi_PmRFK7Cz1T_u7Kyi0.png" width="24" height="24" alt=""></span>60</li></ul></div></div></div>



	

	
	<div class="character__profile">
		<h2 class="heading--lg" id="anchor__profile">Profile</h2>
		<div class="character-block">
			<img src="https://img2.finalfantasyxiv.com/f/6aa9fdf84a8a3fbe7567055bcf81e129_c514cdcdb619439df97d906d4434ccc6fc0_96x96.jpg?1520352080" width="32" height="32" alt="" class="character-block__face">
			<div class="character-block__box">
				<p class="character-block__name">Race/Clan/Gender</p>
				<p class="character-block__profile">Au Ra<br />Raen / ♀</p>
			</div>
		</div>
		
		<div class="character-block">
			<img src="https://img.finalfantasyxiv.com/lds/h/I/vxNKDO_Sxoz97OA23aGnJWEuzY.png" width="32" height="32" alt="">
			<div class="character-block__box">
				<p class="character-block__name">Nameday</p>
				<p class="character-block__birth">5th Sun of the 4th Astral Moon</p>
				<p class="character-block__name">Guardian</p>
				<p class="character-block__profile">Oschon, the Wanderer</p>
			</div>
		</div>
		
		<div class="character-block">
			<img src="https://img.finalfantasyxiv.com/lds/h/M/hSWzpQdnEMp_K4KLPEqxDSt_dg.png" width="32" height="32" alt="">
			<div class="character-block__box">
				<p class="character-block__name">City-state</p>
				<p class="character-block__profile">Gridania</p>
			</div>
		</div>

		
		
			<div class="character-block">
				<img src="https://img.finalfantasyxiv.com/lds/h/X/J8I2IE2iJOH4u4V_sMSbYmcirA.png" width="32" height="32" alt="">
				<div class="character-block__box">
					<p class="character-block__name">Grand Company</p>
					<p class="character-block__profile">Maelstrom / Second Storm Lieutenant</p>
				</div>
			</div>
		
		
		
			<div class="entry">
				<a href="/lodestone/freecompany/9234208823458189094/" class="entry__freecompany">
					<div class="character__freecompany__crest">
						<img src="https://img.finalfantasyxiv.com/lds/h/V/strFm2JQDmMpQN9NHfCA2X3vGI.png" width="34" height="34" alt="">
						<div class="character__freecompany__crest__image">
							

		
			<img src="https://img2.finalfantasyxiv.com/c/B27_03117b168c3a39e866bc39e537da398c_a0_64x64.png" width="32" height="32">
		
		<img src="https://img2.finalfantasyxiv.com/c/F3f_fdeb76450beedbae580f24b8275fbeb0_00_64x64.png" width="32" height="32">
		<img src="https://img2.finalfantasyxiv.com/c/S35_573d9ef4dc9f1c1ac3793fae11694dd3_03_64x64.png" width="32" height="32">


						</div>
					</div>
					<div class="character__freecompany__box">
						<div class="character__freecompany__name">
							<p>Free Company</p>
							<h4>Daijobu</h4>
						</div>
					</div>
				</a>
			</div>
		
	</div>

	
	<div class="heading__icon parts__space--reset">
		<h3>Character Profile</h3>
		
	</div>
	
		<div class="character__character_profile">
			
				Alt: <a href="http://na.finalfantasyxiv.com/lodestone/character/13170454/">http://na.finalfantasyxiv.com/lodestone/character/13170454/</a><br /><br />5523d8f5be
			
		</div>
	

	<div class="character__parameter">
		<h2 class="heading--lg" id="anchor__parameter">Attributes</h2>
		

<ul class="character__param">
	<li>
		<div>
			<p class="character__param__text character__param__text__hp--en-us">HP</p>
			<span>31321</span>
		</div>
		<i class="character__param--hp"></i>
	</li>
	<li>
		
			<div>
				<p class="character__param__text character__param__text__mp--en-us">MP</p>
				<span>15480</span>
			</div>
			<i class="character__param--mp"></i>
		
		
		
	</li>
	<li>
		<div>
			<p class="character__param__text character__param__text__tp--en-us">TP</p>
			<span>1000</span>
		</div>
		<i class="character__param--tp"></i>
	</li>
</ul>

<h3 class="heading--lead">
	<i class="icon-c__attributes"></i>
	Attributes
</h3>

<table class="character__param__list">
	<tr>
		<th>Strength</th>
		<td>130</td>
	</tr>
	<tr>
		<th>Dexterity</th>
		<td>294</td>
	</tr>
	<tr>
		<th>Vitality</th>
		<td>1573</td>
	</tr>
	<tr>
		<th>Intelligence</th>
		<td>2304</td>
	</tr>
	<tr>
		<th>Mind</th>
		<td>222</td>
	</tr>
</table>


<h3 class="heading--lead">
	<i class="icon-c__offense"></i>
	Offensive Properties
</h3>
<table class="character__param__list">
	<tr>
		<th>Critical Hit Rate</th>
		<td>1145</td>
	</tr>
	<tr>
		<th>Determination</th>
		<td>1063</td>
	</tr>
	<tr>
		<th>Direct Hit Rate</th>
		<td>1240</td>
	</tr>
</table>

<h3 class="heading--lead">
	<i class="icon-c__deffense"></i>
	Defensive Properties
</h3>
<table class="character__param__list">
	<tr>
		<th>Defense</th>
		<td>1939</td>
	</tr>
	<tr>
		<th>Magic Defense</th>
		<td>3388</td>
	</tr>
</table>

<h3 class="heading--lead">
	<i class="icon-c__melle"></i>
	Physical Properties
</h3>
<table class="character__param__list">
	<tr>
		<th>Attack Power</th>
		<td>130</td>
	</tr>
	<tr>
		<th>Skill Speed</th>
		<td>364</td>
	</tr>
</table>


	<h3 class="heading--lead">
		<i class="icon-c__spell"></i>
		Mental Properties
	</h3>
	<table class="character__param__list">
		<tr>
			<th>Attack Magic Potency</th>
			<td>2304</td>
		</tr>
		<tr>
			<th>Healing Magic Potency</th>
			<td>222</td>
		</tr>
		<tr>
			<th>Spell Speed</th>
			<td>1494</td>
		</tr>
	</table>

	<h3 class="heading--lead">
		<i class="icon-c__role"></i>
		Role
	</h3>
	<table class="character__param__list">
		<tr>
			<th>Tenacity</th>
			<td>364</td>
		</tr>
		<tr>
			<th  class="pb-0">Piety</th>
			<td  class="pb-0">292</td>
		</tr>
	</table>








	</div>

	<div class="character__class">
		<h2 class="heading--lg" id="anchor__class">Class</h2>
		
		

<h3 class="heading--md">DoW/DoM</h3>
<div class="character__job__role">
	<h4 class="heading--lead"><img src="https://img.finalfantasyxiv.com/lds/h/Q/e8-CIpHMk6D5Dau_tyTx0Kt9js.png" width="24" height="24" class="character__job__icon__title">Tank</h4>
	<ul class="character__job clearfix">
		
	
		<li>
			<i class="character__job__icon"><img src="https://img.finalfantasyxiv.com/lds/h/f/y4GToCrk6Ti9xJeSDHWHjJVmoQ.png" width="24" height="24" alt=""></i>
			<div class="character__job__level">62</div>
			<div class="character__job__name">Paladin</div>
			<div class="character__job__exp">3483866 / 5316000</div>
		</li>

		
	
		<li>
			<i class="character__job__icon"><img src="https://img.finalfantasyxiv.com/lds/h/k/d6U_2PbjZIIv8JJ8-dweIGATlc.png" width="24" height="24" alt=""></i>
			<div class="character__job__level">60</div>
			<div class="character__job__name">Warrior</div>
			<div class="character__job__exp">0 / 4470000</div>
		</li>

		
	
		<li>
			<i class="character__job__icon"><img src="https://img.finalfantasyxiv.com/lds/h/A/rnuxXb8g_heKzVRK5xxtt4tnhQ.png" width="24" height="24" alt=""></i>
			<div class="character__job__level">33</div>
			<div class="character__job__name">Dark Knight</div>
			<div class="character__job__exp">97171 / 203500</div>
		</li>

	</ul>
</div>
<div class="character__job__role">
	<h4 class="heading--lead"><img src="https://img.finalfantasyxiv.com/lds/h/8/iUDBiCamMS05753pYarJPvRBkg.png" width="24" height="24" class="character__job__icon__title">Healer</h4>
	<ul class="character__job clearfix">
		
	
		<li>
			<i class="character__job__icon"><img src="https://img.finalfantasyxiv.com/lds/h/p/i7RE68GXG_dc4P9cA0QQWBpYM4.png" width="24" height="24" alt=""></i>
			<div class="character__job__level">51</div>
			<div class="character__job__name">White Mage</div>
			<div class="character__job__exp">725827 / 1058400</div>
		</li>

		
	
		<li>
			<i class="character__job__icon"><img src="https://img.finalfantasyxiv.com/lds/h/s/-UlKB2SsH12g9Ftrp4QCPejw_U.png" width="24" height="24" alt=""></i>
			<div class="character__job__level">70</div>
			<div class="character__job__name">Scholar</div>
			<div class="character__job__exp">-- / --</div>
		</li>

		
	
		<li>
			<i class="character__job__icon"><img src="https://img.finalfantasyxiv.com/lds/h/0/PHXicpkH8WxgMAtq2rL4QTTWWY.png" width="24" height="24" alt=""></i>
			<div class="character__job__level">50</div>
			<div class="character__job__name">Astrologian</div>
			<div class="character__job__exp">812496 / 864000</div>
		</li>

	</ul>
</div>
<div class="character__job__role">
	<h4 class="heading--lead"><img src="https://img.finalfantasyxiv.com/lds/h/V/d9PjsFG2F8p1wwBkwcpOzoETRQ.png" width="24" height="24" class="character__job__icon__title">Melee DPS</h4>
	<ul class="character__job clearfix">
		
	
		<li>
			<i class="character__job__icon"><img src="https://img.finalfantasyxiv.com/lds/h/C/Mgep5vWlYWV-RKurbW4j41_HuE.png" width="24" height="24" alt=""></i>
			<div class="character__job__level">68</div>
			<div class="character__job__name">Monk</div>
			<div class="character__job__exp">2791214 / 9593000</div>
		</li>

		
	
		<li>
			<i class="character__job__icon"><img src="https://img.finalfantasyxiv.com/lds/h/f/wcMw-nrJKmVaAXRo5G4xw6-Njo.png" width="24" height="24" alt=""></i>
			<div class="character__job__level">60</div>
			<div class="character__job__name">Dragoon</div>
			<div class="character__job__exp">0 / 4470000</div>
		</li>

		
	
		<li>
			<i class="character__job__icon"><img src="https://img.finalfantasyxiv.com/lds/h/M/5edkH3aPx5xzC7AK3tLhxfcIjQ.png" width="24" height="24" alt=""></i>
			<div class="character__job__level">61</div>
			<div class="character__job__name">Ninja</div>
			<div class="character__job__exp">2432105 / 4873000</div>
		</li>

		
	
		<li>
			<i class="character__job__icon"><img src="https://img.finalfantasyxiv.com/lds/h/A/CI_AlHO4FeG8MOTO5Z6ppgIwB0.png" width="24" height="24" alt=""></i>
			<div class="character__job__level">53</div>
			<div class="character__job__name">Samurai</div>
			<div class="character__job__exp">1168271 / 1555200</div>
		</li>

	</ul>
</div>
<div class="character__job__role">
	<h4 class="heading--lead"><img src="https://img.finalfantasyxiv.com/lds/h/v/jkXOruroSp9ATWIA43ZiFGQB1Y.png" width="24" height="24" class="character__job__icon__title">Physical Ranged DPS</h4>
	<ul class="character__job clearfix">
		
	
		<li>
			<i class="character__job__icon"><img src="https://img.finalfantasyxiv.com/lds/h/E/lXzIPJjpO_WHS_oNnLwjZSBR3Y.png" width="24" height="24" alt=""></i>
			<div class="character__job__level">38</div>
			<div class="character__job__name">Bard</div>
			<div class="character__job__exp">175864 / 286200</div>
		</li>

		
	
		<li>
			<i class="character__job__icon"><img src="https://img.finalfantasyxiv.com/lds/h/A/Rn4tQPyamPT20uvxiH8gmtWAGw.png" width="24" height="24" alt=""></i>
			<div class="character__job__level">-</div>
			<div class="character__job__name">Machinist</div>
			<div class="character__job__exp">- / -</div>
		</li>

	</ul>
	<h4 class="heading--lead"><img src="https://img.finalfantasyxiv.com/lds/h/b/z3xeul_dFgcxt4E1MhigrzMDVE.png" width="24" height="24" class="character__job__icon__title">Magical Ranged DPS</h4>
	<ul class="character__job clearfix">
		
	
		<li>
			<i class="character__job__icon"><img src="https://img.finalfantasyxiv.com/lds/h/d/8W89w_G-YGAN_ZRrdD5YL3LqWA.png" width="24" height="24" alt=""></i>
			<div class="character__job__level">70</div>
			<div class="character__job__name">Black Mage</div>
			<div class="character__job__exp">-- / --</div>
		</li>

		
	
		<li>
			<i class="character__job__icon"><img src="https://img.finalfantasyxiv.com/lds/h/M/8Q3Bl-7KbRjzOuRVzFr3plem80.png" width="24" height="24" alt=""></i>
			<div class="character__job__level">70</div>
			<div class="character__job__name">Summoner</div>
			<div class="character__job__exp">-- / --</div>
		</li>

		
	
		<li>
			<i class="character__job__icon"><img src="https://img.finalfantasyxiv.com/lds/h/U/cJaVYjtqo9C7GS9mIfXehgUjpc.png" width="24" height="24" alt=""></i>
			<div class="character__job__level">52</div>
			<div class="character__job__name">Red Mage</div>
			<div class="character__job__exp">52671 / 1267200</div>
		</li>

	</ul>
</div>

<h3 class="heading--md">DoH/DoL</h3>
<div class="character__job__role">
	<h4 class="heading--lead"><img src="https://img.finalfantasyxiv.com/lds/h/6/SX0lXLnrhyf1oQQ3aCra-PIjD8.png" width="24" height="24" class="character__job__icon__title">Disciples of the Hand</h4>
	<ul class="character__job clearfix">
		
	
		<li>
			<i class="character__job__icon"><img src="https://img.finalfantasyxiv.com/lds/h/6/WzZgbzGmOSUFWpxECzxjVRdGyE.png" width="24" height="24" alt=""></i>
			<div class="character__job__level">-</div>
			<div class="character__job__name">Carpenter</div>
			<div class="character__job__exp">- / -</div>
		</li>

		
	
		<li>
			<i class="character__job__icon"><img src="https://img.finalfantasyxiv.com/lds/h/h/OserTu_uxGInTSZ2WbNvd5p2A8.png" width="24" height="24" alt=""></i>
			<div class="character__job__level">-</div>
			<div class="character__job__name">Blacksmith</div>
			<div class="character__job__exp">- / -</div>
		</li>

		
	
		<li>
			<i class="character__job__icon"><img src="https://img.finalfantasyxiv.com/lds/h/1/sm1X5o_2dZDEqrdjnftOWNMMKU.png" width="24" height="24" alt=""></i>
			<div class="character__job__level">14</div>
			<div class="character__job__name">Armorer</div>
			<div class="character__job__exp">24401 / 26400</div>
		</li>

		
	
		<li>
			<i class="character__job__icon"><img src="https://img.finalfantasyxiv.com/lds/h/o/6uBa8sibvDu_EsRwe0jfzRKuu4.png" width="24" height="24" alt=""></i>
			<div class="character__job__level">-</div>
			<div class="character__job__name">Goldsmith</div>
			<div class="character__job__exp">- / -</div>
		</li>

		
	
		<li>
			<i class="character__job__icon"><img src="https://img.finalfantasyxiv.com/lds/h/e/Ww9S9TDqDwrVipK7FUAQS1pZTc.png" width="24" height="24" alt=""></i>
			<div class="character__job__level">50</div>
			<div class="character__job__name">Leatherworker</div>
			<div class="character__job__exp">103514 / 864000</div>
		</li>

		
	
		<li>
			<i class="character__job__icon"><img src="https://img.finalfantasyxiv.com/lds/h/z/RjLQxp4sHP2WCUOAfVL-D7gQcQ.png" width="24" height="24" alt=""></i>
			<div class="character__job__level">16</div>
			<div class="character__job__name">Weaver</div>
			<div class="character__job__exp">11303 / 35400</div>
		</li>

		
	
		<li>
			<i class="character__job__icon"><img src="https://img.finalfantasyxiv.com/lds/h/3/ZPympy0DnMkZJVf3BGA-ar3PwQ.png" width="24" height="24" alt=""></i>
			<div class="character__job__level">17</div>
			<div class="character__job__name">Alchemist</div>
			<div class="character__job__exp">3957 / 40500</div>
		</li>

		
	
		<li>
			<i class="character__job__icon"><img src="https://img.finalfantasyxiv.com/lds/h/A/m2BECCaj0QbKN8wI4s38PrgRxo.png" width="24" height="24" alt=""></i>
			<div class="character__job__level">12</div>
			<div class="character__job__name">Culinarian</div>
			<div class="character__job__exp">2249 / 19600</div>
		</li>

	</ul>
</div>
<div class="character__job__role">
	<h4 class="heading--lead"><img src="https://img.finalfantasyxiv.com/lds/h/c/_pbf8dASjs-lT233ljqfamS52o.png" width="24" height="24" class="character__job__icon__title">Disciples of the Land</h4>
	<ul class="character__job clearfix">
		
	
		<li>
			<i class="character__job__icon"><img src="https://img.finalfantasyxiv.com/lds/h/a/GHx1Bej_ySjgCjFWbGAv-PwBaw.png" width="24" height="24" alt=""></i>
			<div class="character__job__level">60</div>
			<div class="character__job__name">Miner</div>
			<div class="character__job__exp">0 / 4470000</div>
		</li>

		
	
		<li>
			<i class="character__job__icon"><img src="https://img.finalfantasyxiv.com/lds/h/9/QgCWQNT-CaJEbJkg478kiw3Wfs.png" width="24" height="24" alt=""></i>
			<div class="character__job__level">13</div>
			<div class="character__job__name">Botanist</div>
			<div class="character__job__exp">5378 / 23700</div>
		</li>

		
	
		<li>
			<i class="character__job__icon"><img src="https://img.finalfantasyxiv.com/lds/h/E/PaI1B9Yi_PmRFK7Cz1T_u7Kyi0.png" width="24" height="24" alt=""></i>
			<div class="character__job__level">60</div>
			<div class="character__job__name">Fisher</div>
			<div class="character__job__exp">45594 / 4470000</div>
		</li>

	</ul>
</div>

	</div>


	
		<div class="character__mounts">
			<h2 class="heading--lg" id="anchor__mounts">Mounts</h2>
			<ul class="character__icon__list">
			<li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/0d/0dc34be5525ff5fba7aff7f70838735f35f5db13.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/5c/5c0fa85829e3173259cbede530c91df176b79f74.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/ca/ca8026da532f69542b0c4531e981cf84becfcaf1.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/b6/b66c640b89146cc5d7f8790175d8b48b40c8c9d4.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/a8/a8bc3ad7f04f82c55f5e9168174da5964e88075a.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/6c/6c0932ba8e21c47200b7d65d3c88714fe33ed8d2.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/db/db6eac5e16dfd52699f66c055ce3b1fa6722ff29.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/fa/fa3c801398d28eb07a4866b39e6ce18af8371a94.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/53/5364acce999dbfc9f65c9456d16db091bdc346f3.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/29/29463c8009e4ff5cc3e50afc7b4eb6470ac9e0d5.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/57/57e2c70836c396c6c2c0da99c333ed350403ee0f.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/27/27cb31690187fcb5fec35a795c7d62455ba8f84a.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/27/272f88112e1322e021a6c3ad796ca674fa988998.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/91/91bd2d9aa893fde34e49e890e7d4306069fa2be2.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/7b/7b2e09fde5d1164d602ededa1e875b3c952e5673.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/d8/d8666795d88025ac8283db68e5df2aa337a51749.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/89/89a3079ba0144f23fb1dda9b7580619e41f7d5c7.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/8c/8c8a9cf4167c72dc1bf980f744ccc83f96af70ff.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/03/03a9b9d5084a9353772b50e68af4c05ed3aa1687.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/ff/ffd24aab7d32f782d0708261d17800e6c976acb3.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/37/37a677e48eac36221ac7f8371f191afb1bea35a9.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/b8/b8543d08e74b5675612166f77e8105fcc593f699.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/23/23f4f069e9e78636065b946ab77f6defb5e75600.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/ab/abf2781ef3b771006dc5278b8d79ce4f302184dc.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/6c/6cc1a05792e76a255f1af9ddf2b4d010fd437f01.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/85/85df0639ccd6c2b398a140e365257b8ccb2cdf40.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/09/09021b148c97ea3839b4672f8e86e23f40e74453.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/24/24d7d3979cb86502acea7f6f6fb7e8545b7b4df4.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li>
			</ul>
		</div>
	

	
		<div class="character__minions">
			<h2 class="heading--lg" id="anchor__minions">Minions</h2>
			<ul class="character__icon__list">
			<li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/05/053bcf93c3398740df4ea05d4a23ba87d9be8502.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/69/69d5150b86887ddbb959c88e062ce6d1bdcae234.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/a4/a41ed46987418c2293d6d5aa53896662d48dc1e6.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/26/268dd5cfc2afc09b50c358fe315d923917e3d2b0.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/c1/c1e3a1c0255cdcb0436e70266a727f92c70a7289.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/33/33e2db418ec86b504b6ab662bb6ac009ce8d64b9.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/b8/b8317ac46a4f8445a9d804379d33199609dfb57f.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/c7/c72ee133352a227242644dd44e91f0a71db0b6ba.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/27/27d443a8a3051c735e61e915ad4c930d1bce7adb.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/ad/adb535c345f035be2b246b660b5dc3f72d450c6f.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/f7/f797253f51e6b1f6f9d7d2f6dc01f6841d554188.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/51/51d013686ee90bd13a35d77c4e932f3dd248f8c0.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/62/629bea63547d8986b68954e6c15c9435948f86c4.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/9d/9dd82848636b608f73d5d77e3af6635fd84c3e48.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/66/66b7323f24c5c70bb40d5d31e8645028f871ac8c.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/40/405110265517ac55dc3cc8b48b868b90379fb902.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/1b/1b416d82a1e3bb6c107fa4ed7305d9ccdae8f248.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/e6/e650e6098f9581c00a4114f064bbb94fbbed70e5.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/df/df9bdd98baeeb06cb99cc51a5b2caf7d00f35517.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/c5/c5b18966084edffc9732117494878b75021cc5fb.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/01/01ab10793de96df19ea6c5059bc79960e2f3f21f.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/fe/fea75bb50366751df92350909dc00ee8ff404f54.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/33/3306098cd0e817b9f1a4216601d659cb38f18cf9.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/4e/4e9acef6ca268df3c274071ed4a28ff20099d7e7.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/08/086058c622f176d32ae48a90c5cbe3df765bf06b.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/4a/4aee9e227fd6b03f739e5838d63993cd06715908.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/87/8739e4985dac04107ab6f04c6acd738fc01f390f.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/5a/5a2d7e9bdaf987c92c67088f4b1118322310544d.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/4e/4e488596aa687ad8260a2625f02d754a2a508d91.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/ab/ab3fdf778b656b1ee0b0962971b1ceefc7e66fed.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/d3/d3e19f75480d9db22e4ede1d85250a4060ea8b9d.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/d6/d644058e1b2eb99fd4fa34e46d75d0355f4de0ae.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/62/62c36c1cb49fc4ad712fd5f62a3089252d2f3b2a.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/45/45760714411ff6127f452c0e2a2a2d94a1c5e65b.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/f7/f74bf2276e3cc306e5165bce248f181bde0b9ac1.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/52/52cf5a6f5ad3796bc7e61714b359584a252f3539.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/d4/d4cef47b20ffa8ccafb23604dc8a4d5f842cd563.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/3e/3e41c266f19c7d2936d5b4469ac7970c657c213e.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/70/7045db3c9a93d4d17dc1acdc99e32d9fa38cd296.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/88/88f1c695a77425bde620d9004eb4f9a512698efb.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/dd/dd65f2b927d14a0117b702c4039efda389855eac.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/bd/bd1aff3b49f47f5d66d28a9070d91865bb18dad6.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/db/dbe5228fd76abfba655f57bdc16e4d8211f52520.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/68/68602cd24e65ad33f398f00b817024f4677105a9.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/ef/efef58378f257d6df1e608457c1bbb0601839902.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/f2/f2a6cd8252112f6cd4af3b6f77029cc253cde546.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/89/89ff219282b5a7dc6bd5be8e42b3b59582833349.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/dd/dd8aacbb67c92f175a7b53d5f0806f2a0211c859.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/de/de1b8afae30dabac3c8bcd940cdece5834a63cc6.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/40/407c4dd08ea67bf60b1e1bf88856a45a55dd61cd.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/26/26b68b1e7de5378617b905c0e26c00cd761f25ac.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/17/17daf605f3391814decdd57fd188102881dadd3b.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/b4/b41c58815a995780f6046243a0c3fe2209fb7c9a.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/38/38589ed240ad882dd8973f44f9e00c05b9b08626.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/6b/6bc74f76458d73d5f48d0c5bbaa1338f38adcf9f.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/ad/adabc83b7ecddfda0dc7daecfd18ced6d55810e3.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/37/376c06e8e974607e5ad56c91086d875351790d93.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/f6/f6c742435d44b915ba42e774d946c196ddfb3018.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/25/25e134e85ae9d430efd08ce0b711cdf78884c658.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/bf/bfc76938effb8bcfdf636d51b6b1dc7f9d489593.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/29/29307fed1842afae1d555d06639d0844a748efa6.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/7f/7f5e162c585d5769aa7cb966748b6d2385ab79be.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/58/588d0ee8df0821ee069ad4c20c331adad17a06e0.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/b2/b28e969d97a80f0a0505c081fc652dd5b8320d31.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/04/04376f271cf6c501c22007d774b0a7345e376a25.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/c4/c4f06fe8d31b222c10513babe8b5c2cb58dd0ab4.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/c0/c01e9684c181b96565d28eb49afa22350ec02940.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/7d/7d34af7709bba00a9f8254a13a6e9a2b8653d855.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/83/837268c160b444eff89a767dbd3d4d8943737ac0.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/b5/b572e57fccb44537f34316afd39ff2e221d0935d.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/eb/ebeea3fff741c7f6e83bdaca924680e3766a6fbf.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/cf/cf79da54e582e27b35025520e5624fa09fde55f2.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/31/31460f7555ef6e239aa264f832b63f84bdb70b96.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/d4/d40c4a617e21a20b2c13e499d90c6ffae78975d4.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/7b/7bb051d149e227260949079a40f2714fb2141648.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/73/73482d763d219e22a3f7ae8b0354e8a9949ce849.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/54/543a52e8e593bc41f266b028f43059908fb445ea.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/55/554499f646f6740d72037ad6613ff37f5200a1e9.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/9c/9c0705f2acbfe4426207ceb16c0e5b11213682bf.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/69/6931f6d13dbadd81bf785096b1408fd7f5978547.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/cd/cd632096ae00d48ab2db86b43a6b85f055497f96.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/1d/1d6997dadc811d5fc43c57cf8b95e2c4cf76e1ff.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/ea/ea091f0dd2ce905cfe73324259de2905ce153e3f.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/6d/6de5f4c3282cdfff7d54ab572c4346d8c589b283.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/bd/bde3cd298cc6c320cf167bc5410e4b1da08d2852.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/08/086286f28acb29428cf5ed9a82a7d7f9dd6cc3b3.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/fd/fdca5cc50a3053217a272781e182c267ba0d0564.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/d3/d3c8514f4820ac0c14892c50cf89e7dba926789b.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/7e/7e19c3f56ddd40d975844836130c4839c0169f93.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/a5/a569a3fd340d6ba224f50cdbc6cf9f4e552a5b15.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li><li><div class="character__item_icon"><img src="https://img.finalfantasyxiv.com/lds/pc/global/images/itemicon/f6/f6a046f44a1bfaaa13bbbaa05fd485610b6a8adb.png?4.21" width="32" height="32" alt=""><div class="character__item_icon character__item_icon--frame"></div></div></li>
			</ul>
		</div>
	

	
<ul id="sns_btn">
	<li class="fb"><div class="fb-like" data-href="https://na.finalfantasyxiv.com/lodestone/character/7248246/" data-width="120" data-layout="button" data-action="like" data-show-faces="true" data-share="false"></div></li>
	<li class="tw">
<a href="https://twitter.com/share" class="twitter-share-button" data-lang="en" data-count="none" data-url="https://na.finalfantasyxiv.com/lodestone/character/7248246/">Tweet</a>
<script>!function(d,s,id){var js,fjs=d.getElementsByTagName(s)[0],p=/^http:/.test(d.location)?'http':'https';if(!d.getElementById(id)){js=d.createElement(s);js.id=id;js.src=p+'://platform.twitter.com/widgets.js';fjs.parentNode.insertBefore(js,fjs);}}(document, 'script', 'twitter-wjs');</script>

</li>
	<li class="go">
		<div class="g-plusone" data-size="medium" data-annotation="none" data-href="https://na.finalfantasyxiv.com/lodestone/character/7248246/"></div>
		<script type="text/javascript">

	window.___gcfg = {lang: 'en-US'};

			(function() {
				var po = document.createElement('script'); po.type = 'text/javascript'; po.async = true;
				po.src = 'https://apis.google.com/js/plusone.js';
					var s = document.getElementsByTagName('script')[0]; s.parentNode.insertBefore(po, s);
			})();
		</script>
	</li>
</ul>


	
		
			<div class="btn__link"><a href="/lodestone/character/7248246/achievement/">Achievements</a></div>
		
		
		
			<div class="btn__link"><a href="/lodestone/character/7248246/following/">Following</a></div>
		
	

	<ul class="breadcrumbs">
		
			<li class="btn__prev"><a href="javascript:history.back();">Back</a></li>
		
		<li class="btn__prev"><a href="/lodestone/">Main Site</a></li>
	</ul>
</div>
<!-- //#contetnts-->


<!-- footer -->

	<div class="l__page-top__base-position"><div id="link_pcsite" class="link_pc-site"><a href="javascript:void(0)" class="fs10">View desktop version of the Lodestone</a></div><span class="l__page-top__button"></span></div><footer class="l__footer"><div class="l__footer__inner"><div class="l__footer__logo"><img src="https://img.finalfantasyxiv.com/lds/h/l/xyai_6mIZpuTtVR4OO0xoR_DdI.png" width="280" height="32" alt=""></div><div class="l__footer__information"><p class="l__footer__title">Official Information</p><ul class="l__footer__officielles__link"><li class="l__footer__officielles__link--fb"><a href="https://www.facebook.com/finalfantasyxiv" target="_blank">Facebook</a></li><li class="l__footer__officielles__link--tw"><a href="https://twitter.com/ff_xiv_en" target="_blank">Twitter</a> / <a href="https://twitter.com/FFXIV_NEWS_EN" target="_blank">Twitter News</a></li><li class="l__footer__officielles__link--yt"><a href="http://www.youtube.com/finalfantasyxiv" target="_blank">Official Channel</a></li><li class="l__footer__officielles__link--ig"><a href="https://www.instagram.com/ffxiv/" target="_blank">Instagram</a></li></ul></div><div class="l__footer__information"><p class="l__footer__title">Platforms</p><p class="l__footer__platformes">PlayStation®4, Windows®, Mac</p></div><ul class="l__footer__link-list"><li><a href="/lodestone/help/license/" target="_blank">License</a></li><li><a href="http://support.na.square-enix.com/rule.php?id=5382&la=1" target="_blank">Rules & Policies</a></li></ul><div class="l__footer__copyright l__footer__copyright--en"><div class="l__footer__sqex"><img src="https://img.finalfantasyxiv.com/lds/h/D/YEzUjFNCUd-16fgyLptUDiM1pk.png" width="170" height="18" alt=""></div><p class="l__footer__copyright--text">© 2010 - 2018 <a href="http://na.square-enix.com/" target="_blank">SQUARE ENIX</a> CO., LTD. All Rights Reserved.</p><div class="l__footer__legal--btn__wrapper"><span class="l__footer__legal--btn js--legal_trigger">LEGAL</span></div><div class="l__footer__legal__area"><div class="l__footer__legal__area__inner"><ul class="l__footer__legal__bnr-list"><li><a href="http://www.esrb.org/" target="_blank"><img src="https://img.finalfantasyxiv.com/lds/h/c/7tEAhKgf18eU44X_Cll39pHurQ.png" width="133" height="60" alt="ESRB Ratings"></a></li></ul><ul class="l__footer__legal__bnr-list"><li><a href="https://www.playstation.com/en-us/" target="_blank"><img src="https://img.finalfantasyxiv.com/lds/h/E/nf39alqtMTd4W49mI6bDAWSU4Y.png" width="37" height="39" alt="PlayStation"></a></li><li><a href="https://www.playstation.com/en-us/" target="_blank"><img src="https://img.finalfantasyxiv.com/lds/h/G/JapBMQ_gUObf0u9C3yi6IqbWwg.png" width="89" height="39" alt="PS4"></a></li></ul><ul class="l__footer__legal__bnr-list"><li class="l__footer__legal__bnr-list--pc_dvd"><div class="l__footer__legal__bnr-list--pc_dvd__inner"><img src="https://img.finalfantasyxiv.com/lds/h/v/oxRQz2cT8riqV5VLCu2-nADVkE.png" width="42" height="39" alt="PC"></div></li><li><img src="https://img.finalfantasyxiv.com/lds/h/R/LxJc19nfjVjJ6cqOhJS1knTWmA.png" width="30" height="39" alt="Mac"></li><li><a href="https://na.square-enix.com/us/documents/privacy" target="_blank"><img src="https://img.finalfantasyxiv.com/lds/h/1/wsPzVXiWO4piAMX_tX22gcS81A.png" width="144" height="53" alt="ESRB"></a></li></ul><p class="l__footer__legal__text">"PlayStation", the "PS" family logo, the PlayStation Network logo and "PS4" are registered trademarks or trademarks of Sony Interactive Entertainment Inc.<br />ESRB and the ESRB rating icon are registered trademarks of the Entertainment Software Association. <br />MAC is a trademark of Apple Inc., registered in the U.S. and other countries.<br />Windows is either a registered trademark or trademark of Microsoft Corporation in the United States and/or other countries.<br />All other trademarks are property of their respective owners. </p><p class="l__footer__legal__text">FINAL FANTASY, FINAL FANTASY XIV, FFXIV, SQUARE ENIX, and the SQUARE ENIX logo are registered trademarks or trademarks of Square Enix Holdings Co., Ltd.<br />STORMBLOOD, HEAVENSWARD, and A REALM REBORN are registered trademarks or trademarks of Square Enix Co., Ltd.</p></div></div></div></div></footer>

<!-- //footer -->
</div>

<script>
(function(i,s,o,g,r,a,m){i['GoogleAnalyticsObject']=r;i[r]=i[r]||function(){
 (i[r].q=i[r].q||[]).push(arguments)},i[r].l=1*new Date();a=s.createElement(o),
 m=s.getElementsByTagName(o)[0];a.async=1;a.src=g;m.parentNode.insertBefore(a,m)
 })(window,document,'script','https://www.google-analytics.com/analytics.js','ga');

ga('create', 'UA-43364669-4', 'auto');
ga('create', 'UA-43364669-1', 'auto', 'anotherTracker');

ga('require', 'linkid');
ga('anotherTracker.require', 'linkid');


ga('set', 'dimension1', 'notloginuser');
ga('anotherTracker.set', 'dimension1', 'notloginuser');


ga('set', 'dimension2', 'white');
ga('anotherTracker.set', 'dimension2', 'white');

ga('send', 'pageview');
ga('anotherTracker.send', 'pageview');
</script>







	<script>
	// INSTRUCTIONS
	// The VersaTag code should be placed at the top of the <BODY> section of the HTML page.
	// To ensure that the full page loads as a prerequisite for the VersaTag
	// being activated (and the working mode is set to synchronous mode), place the tag at the bottom of the page. Note, however, that this may
	// skew the data for slow-loading pages, and in general is not recommended.
	// If the VersaTag code is configured to run in async mode, place the tag at the bottom of the page before the end of the <BODY > section.

	//
	// NOTE: You can test if the tags are working correctly before the campaign launches
	// as follows:  Browse to http://bs.serving-sys.com/BurstingPipe/adServer.bs?cn=at, which is 
	// a page that lets you set your local machine to 'testing' mode.  In this mode, when
	// visiting a page that includes a VersaTag, a new window will open, showing you
	// the tags activated by the VersaTag and the data sent by the VersaTag tag to the Sizmek servers.
	//
	// END of instructions (These instruction lines can be deleted from the actual HTML)

	var versaTag = {};
	versaTag.id = "3124";
	versaTag.sync = 0;
	versaTag.dispType = "js";
	versaTag.ptcl = "HTTPS";
	versaTag.bsUrl = "bs.serving-sys.com/BurstingPipe";
	//VersaTag activity parameters include all conversion parameters including custom parameters and Predefined parameters. Syntax: "ParamName1":"ParamValue1", "ParamName2":"ParamValue2". ParamValue can be empty.
	versaTag.activityParams = {
	//Predefined parameters:
	"Session":""
	//Custom parameters:
	};
	//Static retargeting tags parameters. Syntax: "TagID1":"ParamValue1", "TagID2":"ParamValue2". ParamValue can be empty.
	versaTag.retargetParams = {};
	//Dynamic retargeting tags parameters. Syntax: "TagID1":"ParamValue1", "TagID2":"ParamValue2". ParamValue can be empty.
	versaTag.dynamicRetargetParams = {};
	// Third party tags conditional parameters and mapping rule parameters. Syntax: "CondParam1":"ParamValue1", "CondParam2":"ParamValue2". ParamValue can be empty.
	versaTag.conditionalParams = {};
	</script>
	<script id="ebOneTagUrlId" src="https://secure-ds.serving-sys.com/SemiCachedScripts/ebOneTag.js"></script>
	<noscript>
	<iframe src="https://bs.serving-sys.com/BurstingPipe?
	cn=ot&amp;
	onetagid=3124&amp;
	ns=1&amp;
	activityValues=$$Session=[Session]$$&amp;
	retargetingValues=$$$$&amp;
	dynamicRetargetingValues=$$$$&amp;
	acp=$$$$&amp;"
	style="display:none;width:0px;height:0px"></iframe>
	</noscript>


<script src="https://img.finalfantasyxiv.com/lds/sp/en-us/js/i18n.js?1519285807"></script>
<script src="https://img.finalfantasyxiv.com/lds/h/i/2ur_0e4qXk_NwNv0bGjrUHThCM.js"></script>
<script src="https://img.finalfantasyxiv.com/lds/h/g/uQNBgvCaabd4gw5kMpaT-psp1Q.js"></script>
<script src="https://img.finalfantasyxiv.com/lds/h/8/b_tPmOnEw1tyr2dp2InkGxUlMA.js"></script>
<script src="https://img.finalfantasyxiv.com/lds/h/Y/6WaukM6m3WFE2JXrCEb6tsknik.js"></script>
<script src="https://img.finalfantasyxiv.com/lds/h/E/ImNUSWkI-O9dQP5ADa41RiGxGY.js"></script>
<script src="https://img.finalfantasyxiv.com/lds/h/t/kQLYFR8rOESAOiyT9o2PsRnLq0.js"></script>
<script src="https://img.finalfantasyxiv.com/lds/h/7/-iQtZ2suEP8z5zxTh8jl5Y1byE.js"></script>
<script src="https://img.finalfantasyxiv.com/lds/h/m/MegQy11U6VFa8pLYuEty9_tRFc.js"></script>
<script src="https://img.finalfantasyxiv.com/lds/h/W/jYESLoKcRR7LDUxyfasvLh85wg.js"></script>
<script src="https://img.finalfantasyxiv.com/lds/h/t/03sA_feMPBwlAg4QI6kBawCf_E.js"></script>
<script src="https://img.finalfantasyxiv.com/lds/h/m/xSD5wF5g313cFbmEMg6tG_ueEg.js"></script>
<script src="https://img.finalfantasyxiv.com/lds/h/V/0KFjjgZ-p09-NUSKKOMeXrfCFI.js"></script>
<script src="https://img.finalfantasyxiv.com/lds/h/0/f52zA7ACWvjEMB1oWe8kfyGWH0.js"></script>
<script src="https://img.finalfantasyxiv.com/lds/h/l/-_ePiHXNE_1t8LnRcmRAbhRME8.js"></script>
<script src="https://img.finalfantasyxiv.com/lds/h/b/XSzJDg4K2tyx1ge1uKAvreHKcw.js"></script>
<script src="https://img.finalfantasyxiv.com/lds/h/z/mIJQRtMdsZHuPuHvHEe8K1HC7w.js"></script>
<script src="https://img.finalfantasyxiv.com/lds/h/P/MYGDtxveSuUwuhprm8oj1Hhuq8.js"></script>
<script src="https://img.finalfantasyxiv.com/lds/h/M/it7tc9wvjlJJstEQUNjiModhdM.js"></script>
<script src="https://img.finalfantasyxiv.com/lds/h/n/HMQm8_JEoqrSmuv5RmmKgTldGc.js"></script>
<script src="https://img.finalfantasyxiv.com/lds/h/u/AVbQMBsmCh3GU9JNbEtRUHDioc.js"></script>



</body>
</html>`
