package generic

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/belphemur/mangal/constant"
	"github.com/belphemur/mangal/provider/generic/headless"
	"github.com/belphemur/mangal/source"
	"github.com/belphemur/mangal/where"
	"github.com/gocolly/colly/v2"
	"github.com/metafates/mangal/constant"
	"github.com/metafates/mangal/provider/generic/headless"
	"github.com/metafates/mangal/source"
	"github.com/metafates/mangal/where"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var chapterNumberRegex = regexp.MustCompile(`(?m)(\d+\.\d+|\d+)`)
var newLineCharacters = regexp.MustCompile(`\r?\n`)

func standardizeSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// New generates a new scraper with given configuration
func New(conf *Configuration) source.Source {
	s := Scraper{
		mangas:   make(map[string][]*source.Manga),
		chapters: make(map[string][]*source.Chapter),
		pages:    make(map[string][]*source.Page),
		config:   conf,
	}
	s.cache.mangas = cacher.NewCacher[[]*source.Manga](fmt.Sprintf("%s_%s", conf.Name, "mangas"), 6*time.Hour)
	s.cache.chapters = cacher.NewCacher[[]*source.Chapter](fmt.Sprintf("%s_%s", conf.Name, "chapters"), 6*time.Hour)

	collectorOptions := []colly.CollectorOption{
		colly.AllowURLRevisit(),
		colly.Async(true),
	}

	baseCollector := colly.NewCollector(collectorOptions...)
	baseCollector.SetRequestTimeout(30 * time.Second)
	if conf.NeedsHeadlessBrowser {
		transport := headless.GetTransportSingleton()
		baseCollector.WithTransport(transport)
	}

	mangasCollector := baseCollector.Clone()
	mangasCollector.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Referer", "https://google.com")
		r.Headers.Set("accept-language", "en-US")
		r.Headers.Set("Accept", "text/html")
		r.Headers.Set("Host", s.config.BaseURL)
		r.Headers.Set("User-Agent", constant.UserAgent)
	})

	// Get mangas
	mangasCollector.OnHTML("html", func(e *colly.HTMLElement) {
		elements := e.DOM.Find(s.config.MangaExtractor.Selector)
		path := e.Request.URL.String()
		s.mangas[path] = make([]*source.Manga, elements.Length())

		elements.Each(func(i int, selection *goquery.Selection) {
			link := s.config.MangaExtractor.URL(selection)
			url := e.Request.AbsoluteURL(link)

			manga := source.Manga{
				Name:     cleanName(s.config.MangaExtractor.Name(selection)),
				URL:      url,
				Index:    uint16(e.Index),
				Chapters: make([]*source.Chapter, 0),
				ID:       filepath.Base(url),
				Source:   &s,
			}
			manga.Metadata.Cover.ExtraLarge = s.config.MangaExtractor.Cover(selection)

			s.mangas[path][i] = &manga
		})
	})

	_ = mangasCollector.Limit(&colly.LimitRule{
		Parallelism: int(s.config.Parallelism),
		RandomDelay: s.config.Delay,
		DomainGlob:  "*",
	})

	chaptersCollector := baseCollector.Clone()
	chaptersCollector.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Referer", r.Ctx.GetAny("manga").(*source.Manga).URL)
		r.Headers.Set("accept-language", "en-US")
		r.Headers.Set("Accept", "text/html")
		r.Headers.Set("Host", s.config.BaseURL)
		r.Headers.Set("User-Agent", constant.UserAgent)
	})

	// Get chapters
	chaptersCollector.OnHTML("html", func(e *colly.HTMLElement) {
		elements := e.DOM.Find(s.config.ChapterExtractor.Selector)
		path := e.Request.AbsoluteURL(e.Request.URL.Path)
		s.chapters[path] = make([]*source.Chapter, elements.Length())
		manga := e.Request.Ctx.GetAny("manga").(*source.Manga)

		elements.Each(func(i int, selection *goquery.Selection) {
			link := s.config.ChapterExtractor.URL(selection)
			url := e.Request.AbsoluteURL(link)
			name := cleanName(s.config.ChapterExtractor.Name(selection))

			match := chapterNumberRegex.FindString(name)
			var chapterNumber = float32(e.Index)
			if match != "" {
				number, err := strconv.ParseFloat(match, 32)
				if err == nil {
					chapterNumber = float32(number)
				}
			}

			var chapterDate *time.Time
			if s.config.ChapterExtractor.Date != nil {
				chapterDate = s.config.ChapterExtractor.Date(selection)
			}

			chapter := source.Chapter{
				Name:        name,
				URL:         url,
				Index:       uint16(e.Index),
				Number:      chapterNumber,
				ChapterDate: chapterDate,
				Pages:       make([]*source.Page, 0),
				ID:          filepath.Base(url),
				Manga:       manga,
				Volume:      s.config.ChapterExtractor.Volume(selection),
			}
			s.chapters[path][i] = &chapter
		})
		manga.Chapters = s.chapters[path]
	})
	_ = chaptersCollector.Limit(&colly.LimitRule{
		Parallelism: int(s.config.Parallelism),
		RandomDelay: s.config.Delay,
		DomainGlob:  "*",
	})

	pagesCollector := baseCollector.Clone()
	pagesCollector.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Referer", r.Ctx.GetAny("chapter").(*source.Chapter).URL)
		r.Headers.Set("accept-language", "en-US")
		r.Headers.Set("Accept", "text/html")
		r.Headers.Set("User-Agent", constant.UserAgent)
	})

	// Get pages
	pagesCollector.OnHTML("html", func(e *colly.HTMLElement) {
		elements := e.DOM.Find(s.config.PageExtractor.Selector)
		path := e.Request.AbsoluteURL(e.Request.URL.Path)
		s.pages[path] = make([]*source.Page, elements.Length())
		chapter := e.Request.Ctx.GetAny("chapter").(*source.Chapter)

		elements.Each(func(i int, selection *goquery.Selection) {
			link := s.config.PageExtractor.URL(selection)
			ext := filepath.Ext(link)
			// remove some query params from the extension
			ext = strings.Split(ext, "?")[0]

			page := source.Page{
				URL:       link,
				Index:     uint16(i),
				Chapter:   chapter,
				Extension: ext,
			}
			s.pages[path][i] = &page
		})
		chapter.Pages = s.pages[path]
	})
	_ = pagesCollector.Limit(&colly.LimitRule{
		Parallelism: int(s.config.Parallelism),
		RandomDelay: s.config.Delay,
		DomainGlob:  "*",
	})

	s.mangasCollector = mangasCollector
	s.chaptersCollector = chaptersCollector
	s.pagesCollector = pagesCollector

	return &s
}

func cleanName(name string) string {
	return standardizeSpaces(newLineCharacters.ReplaceAllString(name, " "))
}
