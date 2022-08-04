package source

import (
	"errors"
	"fmt"
	"github.com/metafates/mangal/config"
	"github.com/metafates/mangal/util"
	"github.com/spf13/viper"
	lua "github.com/yuin/gopher-lua"
	"strings"
	"sync"
)

type Chapter struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Index    uint16 `json:"index"`
	SourceID string `json:"source_id"`
	Manga    *Manga
	Pages    []*Page
}

func chapterFromTable(table *lua.LTable, manga *Manga, index uint16) (*Chapter, error) {
	name := table.RawGetString("name")

	if name.Type() != lua.LTString {
		return nil, errors.New("type of field \"name\" should be string")
	}

	url := table.RawGetString("url")
	if url.Type() != lua.LTString {
		return nil, errors.New("type of field \"url\" should be string")
	}

	chapter := &Chapter{
		Name:  strings.TrimSpace(name.String()),
		URL:   strings.TrimSpace(url.String()),
		Manga: manga,
		Index: index,
		Pages: []*Page{},
	}

	manga.Chapters = append(manga.Chapters, chapter)
	return chapter, nil
}

// DownloadPages downloads the Pages contents of the Chapter.
// Pages needs to be set before calling this function.
func (c *Chapter) DownloadPages() error {
	wg := sync.WaitGroup{}
	wg.Add(len(c.Pages))

	var err error
	for _, page := range c.Pages {
		go func(page *Page) {
			defer wg.Done()

			// if at any point, an error is encountered, stop downloading other pages
			if err != nil {
				return
			}

			err = page.Download()
		}(page)
	}

	wg.Wait()
	return err
}

func (c *Chapter) FormattedName() (name string) {
	template := viper.GetString(config.DownloaderChapterNameTemplate)

	name = strings.ReplaceAll(template, "{manga}", c.Manga.Name)
	name = strings.ReplaceAll(name, "{chapter}", c.Name)
	name = strings.ReplaceAll(name, "{index}", fmt.Sprintf("%d", c.Index))
	name = strings.ReplaceAll(name, "{padded-index}", util.PadZero(fmt.Sprintf("%d", c.Index), 4))

	return
}
