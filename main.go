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
	after := time.After(interval)
	defer func() {
		log.Printf("waiting %v", interval)
		<-after
		log.Print("exiting")
	}()

	username := os.Getenv("TWITTER_USER")
	if username == "" {
		log.Print("TWITTER_USER not set")
		return
	}
	webhook := os.Getenv("DISCORD_WEBHOOK")
	if webhook == "" {
		log.Print("DISCORD_WEBHOOK not set")
		return
	}

	var cookiesJSON []byte
	if cookiesEnv := os.Getenv("COOKIES"); cookiesEnv != "" {
		cookiesJSON = []byte(cookiesEnv)
	} else {
		var err error
		cookiesJSON, err = os.ReadFile("cookies.json")
		if err != nil {
			log.Printf("failed to load cookies.json: %v", err)
			return
		}
	}
	var cookies []playwright.OptionalCookie
	if err := json.Unmarshal([]byte(cookiesJSON), &cookies); err != nil {
		log.Printf("failed to parse cookie: %v", err)
		return
	}
	for i := range cookies {
		cookies[i].SameSite = playwright.SameSiteAttributeNone
	}

	db, err := newDB(context.TODO())
	if err != nil {
		log.Printf("failed to create DynamoDB client: %v", err)
		return
	}
	lastFetched, since, err := db.GetLastFetched(context.TODO(), username)
	if err != nil {
		log.Printf("failed to get last_fetched: %v", err)
		return
	}

	pw, err := playwright.Run()
	if err != nil {
		log.Printf("failed to start playwright: %v", err)
		return
	}
	browser, err := pw.Chromium.Launch()
	if err != nil {
		log.Printf("failed to launch browser: %v", err)
		return
	}

	browserContext, err := browser.NewContext(playwright.BrowserNewContextOptions{
		Screen: &playwright.Size{Width: 1080, Height: 1920},
	})
	if err != nil {
		log.Printf("failed to create browser context: %v", err)
		return
	}

	if err := browserContext.AddCookies(cookies); err != nil {
		log.Printf("failed to set cookie: %v", err)
		return
	}

	page, err := browserContext.NewPage()
	if err != nil {
		log.Printf("failed to create page: %v", err)
		return
	}

	waitLoad := func() error {
		for {
			entries, err := page.GetByRole(*playwright.AriaRoleProgressbar).All()
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				break
			}
			time.Sleep(time.Second)
		}
		return nil
	}

	if _, err = page.Goto(fmt.Sprintf(
		"https://twitter.com/%s",
		username,
	)); err != nil {
		log.Printf("failed to goto: %v", err)
		return
	}
	if err := waitLoad(); err != nil {
		log.Printf("failed to wait loading: %v", err)
		return
	}

	name, err := page.Locator(`div[data-testid="UserName"] span:not(:has(span))`).First().InnerText()
	if err != nil {
		log.Printf("failed to get user name text: %v", err)
		return
	}
	log.Printf("name: %s", name)

	query := fmt.Sprintf("from:@%s", username)
	if since != "" {
		query = fmt.Sprintf("(%s) since:%s", query, since)
	}
	query = query + " include:nativeretweets"
	log.Printf("query: %s", query)

	if _, err = page.Goto(fmt.Sprintf(
		"https://twitter.com/search?q=%s&src=recent_search_click&f=live",
		query,
	)); err != nil {
		log.Printf("failed to goto: %v", err)
		return
	}
	if err := waitLoad(); err != nil {
		log.Printf("failed to wait loading: %v", err)
		return
	}

	var tweets []string

	for try := 0; try < maxTry; try++ {
		entries, err := page.Locator("article").All()
		if err != nil {
			log.Printf("failed to get articles: %v", err)
			return
		}
		var tl []string
		var caughtUp bool
		for _, entry := range entries {
			href, err := entry.Locator("a:has(time)").GetAttribute("href")
			if err != nil {
				log.Printf("failed to get text content: %v", err)
				return
			}
			if href == lastFetched {
				caughtUp = true
				break
			}
			tl = append(tl, href)
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
			log.Printf("failed to evaluate: %v", err)
			return
		}
		if bottom, _ := ret.(bool); bottom {
			log.Print("hit page bottom")
			break
		}

		if err := page.Mouse().Wheel(0, 1000); err != nil {
			log.Printf("failed to scroll: %v", err)
			return
		}
		time.Sleep(time.Second)
		if err := waitLoad(); err != nil {
			log.Printf("failed to wait loading: %v", err)
			return
		}
	}
	log.Println(len(tweets), "tweets fetched")

	var lastPosted string
	postedAll := true
	for i := len(tweets) - 1; i >= 0; i-- {
		log.Println(tweets[i])
		if err := postDiscord(username, name, tweets[i], webhook); err != nil {
			postedAll = false
			log.Printf("failed to post: %v", err)
			break
		}
		lastPosted = tweets[i]
		time.Sleep(discordPostInterval)
	}

	if lastPosted != "" {
		if postedAll {
			since = time.Now().Add(-24 * time.Hour).Format("2006-01-02")
		}
		if err := db.PutLastFetched(context.TODO(), username, lastPosted, since); err != nil {
			log.Printf("failed to put last_fetched: %v", err)
			return
		}
	}

	if err := page.Close(); err != nil {
		log.Printf("failed to close page: %v", err)
		return
	}

	if err := browser.Close(); err != nil {
		log.Printf("failed to close browser: %v", err)
		return
	}
	if err := pw.Stop(); err != nil {
		log.Printf("failed to stop Playwright: %v", err)
		return
	}
}
