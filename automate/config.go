package automate

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jakopako/goskyr/scraper"
	"github.com/jakopako/goskyr/utils"
	"golang.org/x/net/html"
)

func pathToSelector(pathSlice []string) string {
	return strings.Join(pathSlice, " > ")
}

func selectorToPath(s string) []string {
	return strings.Split(s, " > ")
}

func elementsToConfig(s *scraper.Scraper, l ...scraper.ElementLocation) {
	var itemSelector string
outer:
	for i := 0; ; i++ {
		var c string
		for j, e := range l {
			if i >= len(selectorToPath(e.Selector)) {
				itemSelector = pathToSelector(selectorToPath(e.Selector)[:i-1])
				break outer
			}
			if j == 0 {
				c = selectorToPath(e.Selector)[i]
			} else {
				if selectorToPath(e.Selector)[i] != c {
					itemSelector = pathToSelector(selectorToPath(e.Selector)[:i-1])
					break outer
				}
			}
		}
	}
	fmt.Println(itemSelector)
}

func GetDynamicFieldsConfig(s *scraper.Scraper, g *scraper.GlobalConfig) error {
	if s.URL == "" {
		return errors.New("URL field cannot be empty")
	}
	res, err := utils.FetchUrl(s.URL, g.UserAgent)
	if err != nil {
		return err
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("status code error: %d %s", res.StatusCode, res.Status)
	}
	// body > div.content > div.mainContentContainer > div.mainContent > div.mainContentFloat > h1
	// body > div.content > div.mainContentContainer > div.mainContent > div.mainContentFloat > div.leftContainer > div:nth-child(2) > div.quoteDetails > div.quoteText
	// body > div.content > div.mainContentContainer > div.mainContent > div.mainContentFloat > div.leftContainer > div:nth-child(2) > div.quoteDetails > div.quoteText > span
	z := html.NewTokenizer(res.Body)
	locOcc := map[scraper.ElementLocation]int{}
	locExamples := map[scraper.ElementLocation][]string{}
	nrChildren := map[string]int{}
	nodePath := []string{}
	depth := 0
	inBody := false
parse:
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			break parse
		case html.TextToken:
			if inBody {
				text := string(z.Text())
				p := pathToSelector(nodePath)
				if len(strings.TrimSpace(text)) > 1 {
					l := scraper.ElementLocation{
						Selector:   p,
						ChildIndex: nrChildren[p],
					}
					if nr, found := locOcc[l]; found {
						locOcc[l] = nr + 1
					} else {
						locOcc[l] = 1
					}
					if len(locExamples[l]) < 4 {
						locExamples[l] = append(locExamples[l], strings.TrimSpace(text))
					}
				}
				nrChildren[p] += 1
			}
		case html.StartTagToken, html.EndTagToken:
			tn, _ := z.TagName()
			tnString := string(tn)
			if tnString == "body" {
				inBody = !inBody
			}
			if inBody {
				// what type of token is <br /> ? Same as <br> ?
				if tnString == "br" {
					nrChildren[pathToSelector(nodePath)] += 1
					continue
				}
				if tt == html.StartTagToken {
					nrChildren[pathToSelector(nodePath)] += 1
					moreAttr := true
					for moreAttr {
						k, v, m := z.TagAttr()
						if string(k) == "class" && string(v) != "" {
							tnString += fmt.Sprintf(".%s", strings.Replace(strings.TrimSpace(string(v)), " ", ".", -1))
						}
						moreAttr = m
					}
					if tnString != "br" {
						nodePath = append(nodePath, tnString)
						nrChildren[pathToSelector(nodePath)] = 0
						depth++
					}
				} else {
					n := true
					for n && depth > 0 {
						if strings.Split(nodePath[len(nodePath)-1], ".")[0] == tnString {
							if tnString == "body" {
								break parse
							}
							n = false
						}
						delete(nrChildren, pathToSelector(nodePath))
						nodePath = nodePath[:len(nodePath)-1]
						depth--
					}
				}
			}
		}
	}

	frequencyBuckets := map[int][]scraper.ElementLocation{}
	for k, v := range locOcc {
		frequencyBuckets[v] = append(frequencyBuckets[v], k)
	}
	highestOcc := 0
	highestOccFr := 0
	minFr := 5
	for k, v := range frequencyBuckets {
		n := len(v)
		if n > highestOcc && k >= minFr {
			highestOcc = n
			highestOccFr = k
		}
	}

	f := frequencyBuckets[highestOccFr]
	sort.Slice(f, func(p, q int) bool {
		return f[p].Selector > f[q].Selector
	})
	for i, e := range f {
		fmt.Printf("field [%d] examples:\n\t%s\n\n", i, strings.Join(locExamples[e], "\n\t"))
	}

	fmt.Println("please select one or more of the suggested fields by typing the according numbers separated by spaces:")
	fmt.Println("thanks you selected 4 & 5")

	elementsToConfig(s, frequencyBuckets[highestOccFr][4], frequencyBuckets[highestOccFr][5])

	return nil
}
