package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/playwright-community/playwright-go"
)

const (
	maxFetch            = 1024
	maxTry              = 256
	discordPostInterval = time.Second
	interval            = 30 * time.Minute
)

func main() {
	username := os.Getenv("TWITTER_USER")
	if username == "" {
		log.Fatal("TWITTER_USER not set")
	}
	webhook := os.Getenv("DISCORD_WEBHOOK")
	if webhook == "" {
		log.Fatal("DISCORD_WEBHOOK not set")
	}

	cookiesJSON, err := os.ReadFile("cookies.json")
	if err != nil {
		log.Fatalf("failed to load cookies.json: %v", err)
	}
	var cookies []playwright.OptionalCookie
	if err := json.Unmarshal([]byte(cookiesJSON), &cookies); err != nil {
		log.Fatalf("failed to parse cookie: %v", err)
	}
	for i := range cookies {
		cookies[i].SameSite = playwright.SameSiteAttributeNone
	}

	db, err := newDB(context.TODO())
	if err != nil {
		log.Fatalf("failed to create DynamoDB client: %v", err)
	}
	lastFetched, since, err := db.GetLastFetched(context.TODO(), username)
	if err != nil {
		log.Fatalf("failed to get last_fetched: %v", err)
	}

	pw, err := playwright.Run()
	if err != nil {
		log.Fatalf("failed to start playwright: %v", err)
	}
	browser, err := pw.Chromium.Launch()
	if err != nil {
		log.Fatalf("failed to launch browser: %v", err)
	}

	browserContext, err := browser.NewContext(playwright.BrowserNewContextOptions{
		Screen: &playwright.Size{Width: 1080, Height: 1920},
	})
	if err != nil {
		log.Fatalf("failed to create browser context: %v", err)
	}

	if err := browserContext.AddCookies(cookies); err != nil {
		log.Fatalf("failed to set cookie: %v", err)
	}

	tick := time.NewTicker(interval)
	defer tick.Stop()

	for {
		page, err := browserContext.NewPage()
		if err != nil {
			log.Fatalf("failed to create page: %v", err)
		}

		waitLoad := func() {
			for {
				entries, err := page.GetByRole(*playwright.AriaRoleProgressbar).All()
				if err != nil {
					log.Fatalf("failed to get loading: %v", err)
				}
				if len(entries) == 0 {
					break
				}
				time.Sleep(time.Second)
			}
		}

		if _, err = page.Goto(fmt.Sprintf(
			"https://twitter.com/%s",
			username,
		)); err != nil {
			log.Fatalf("failed to goto: %v", err)
		}
		waitLoad()

		name, err := page.Locator(`div[data-testid="UserName"] span:not(:has(span))`).First().InnerText()
		if err != nil {
			log.Fatalf("failed to get user name text: %v", err)
		}
		log.Printf("name: %s", name)

		query := fmt.Sprintf("from:@%s", username)
		if since != "" {
			query = fmt.Sprintf("(%s) since:%s", query, since)
		}
		log.Printf("query: %s", query)

		if _, err = page.Goto(fmt.Sprintf(
			"https://twitter.com/search?q=%s&src=recent_search_click&f=live",
			query,
		)); err != nil {
			log.Fatalf("failed to goto: %v", err)
		}
		waitLoad()

		var tweets []string

		for try := 0; try < maxTry; try++ {
			entries, err := page.Locator("article").All()
			if err != nil {
				log.Fatalf("failed to get articles: %v", err)
			}
			var tl []string
			var caughtUp bool
			for _, entry := range entries {
				href, err := entry.Locator("a:has(time)").GetAttribute("href")
				if err != nil {
					log.Fatalf("failed to get text content: %v", err)
				}
				tl = append(tl, href)
				if href == lastFetched {
					caughtUp = true
					break
				}
			}

			tweets = merge(tweets, tl)

			if caughtUp {
				log.Print("caught up", lastFetched)
				break
			}
			if len(entries) >= maxFetch || lastFetched == "" {
				log.Print("reached max fetch count")
				break
			}

			ret, err := page.Evaluate("document.documentElement.scrollHeight - document.documentElement.clientHeight - document.documentElement.scrollTop <= 1")
			if err != nil {
				log.Fatalf("failed to evaluate: %v", err)
			}
			if bottom, _ := ret.(bool); bottom {
				log.Print("hit page bottom")
				break
			}

			if err := page.Mouse().Wheel(0, 1000); err != nil {
				log.Fatalf("failed to scroll: %v", err)
			}
			time.Sleep(time.Second)
			waitLoad()
		}

		var lastPosted string
		for i := len(tweets) - 1; i >= 0; i-- {
			log.Println(tweets[i])
			if err := postDiscord(username, name, tweets[i], webhook); err != nil {
				log.Printf("failed to post: %v", err)
				break
			}
			lastPosted = tweets[i]
			time.Sleep(discordPostInterval)
		}
		log.Println(len(tweets), "tweets fetched")

		if lastPosted != "" {
			if err := db.PutLastFetched(context.TODO(), username, lastPosted); err != nil {
				log.Fatalf("failed to put last_fetched: %v", err)
			}
		}

		if err := page.Close(); err != nil {
			log.Fatalf("failed to close page: %v", err)
		}
		log.Printf("waiting %v", interval)
		<-tick.C
	}

	if err := browser.Close(); err != nil {
		log.Fatalf("failed to close browser: %v", err)
	}
	if err := pw.Stop(); err != nil {
		log.Fatalf("failed to stop Playwright: %v", err)
	}
}
