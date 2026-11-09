package rss

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gshlan/gshbot/config"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mmcdole/gofeed"
	"github.com/tidwall/gjson"

	simplejsondb "github.com/pnkj-kmr/simple-json-db"
)

// Declare a global wait group to ensure all goroutines are finished before exiting

// A struct to define what an RSS Feed looks like
type RSSFeed struct {
	Name            string `json:"name"`         // the name of the RSS feed
	Url             string `json:"url"`          // The URL of the RSS feed (i.e. https://example.com/rss)
	ChannelId       string `json:"channelId"`    // The Discord Channel ID to send RSS feed updates to
	Timer           int    `json:"timer"`        // The interval (in seconds) in which the RSS parser will check a feed for updates
	ActiveStatus    bool   `json:"activeStatus"` // A boolean to track if the RSS feed should be actively parsed or not
	MentionEveryOne bool   `json:"mentionEveryone"`
}

// A struct to define a slice of RSS feeds to prevent concurrent access the slice
type RSSFeeds struct {
	mutex    sync.Mutex // A mutex to ensure only one thread can access the slice at a time
	RSSFeeds []RSSFeed  // The slice of RSS feeds
	//config   *config.Discord
}

// A struct to define a slice of Discord messages to prevent concurrent access to the slice
type MessageQueue struct {
	mutex        sync.Mutex          // A mutex to ensure only one thread can access the slice at a time
	MessageQueue map[string][]string // The slice of Discord messages
}

// Initializing variables for RSS feeds, Message Queue, and background workers
var (
	rssFeeds      RSSFeeds
	messageQueue  MessageQueue
	feedWorkersMu sync.Mutex
	feedWorkers   = make(map[string]context.CancelFunc)
	activeSession *discordgo.Session
)

func startFeedWorker(s *discordgo.Session, feed *RSSFeed) {
	if s == nil {
		s = activeSession
	}
	if s == nil {
		return
	}

	key := feed.Name + "_" + feed.ChannelId
	feedWorkersMu.Lock()
	if cancel, exists := feedWorkers[key]; exists {
		cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	feedWorkers[key] = cancel
	feedWorkersMu.Unlock()

	localFeed := *feed
	go parseRSSFeed(ctx, s, &localFeed)
}

func stopFeedWorker(name, channelId string) {
	key := name + "_" + channelId
	feedWorkersMu.Lock()
	if cancel, exists := feedWorkers[key]; exists {
		cancel()
		delete(feedWorkers, key)
	}
	feedWorkersMu.Unlock()
}

// IsWorkerRunning reports whether a background polling goroutine is active for the feed.
func IsWorkerRunning(name, channelId string) bool {
	feedWorkersMu.Lock()
	defer feedWorkersMu.Unlock()
	_, exists := feedWorkers[name+"_"+channelId]
	return exists
}

// StopRSSFeeds stops all active feed polling workers.
func StopRSSFeeds() {
	feedWorkersMu.Lock()
	for key, cancel := range feedWorkers {
		cancel()
		delete(feedWorkers, key)
	}
	feedWorkersMu.Unlock()
}

func setFeedActiveStatusInMemory(name, channelId string, active bool) *RSSFeed {
	rssFeeds.mutex.Lock()
	defer rssFeeds.mutex.Unlock()
	for i, f := range rssFeeds.RSSFeeds {
		if f.Name == name && f.ChannelId == channelId {
			rssFeeds.RSSFeeds[i].ActiveStatus = active
			feedCopy := rssFeeds.RSSFeeds[i]
			return &feedCopy
		}
	}
	return nil
}

func removeFeedFromMemory(name, channelId string) {
	rssFeeds.mutex.Lock()
	defer rssFeeds.mutex.Unlock()
	for i, f := range rssFeeds.RSSFeeds {
		if f.Name == name && f.ChannelId == channelId {
			rssFeeds.RSSFeeds = append(rssFeeds.RSSFeeds[:i], rssFeeds.RSSFeeds[i+1:]...)
			return
		}
	}
}

// Only allow characters that are safe to use as part of a filesystem path,
// since Name/ChannelId end up in the on-disk record key (Name + "_" + ChannelId).
var validFeedName = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
var validChannelId = regexp.MustCompile(`^\d{17,20}$`)

func validateFeedIdentifiers(name, channelId string) error {
	if !validFeedName.MatchString(name) {
		return errors.New("RSS40 - invalid feed name: only letters, digits, '_' and '-' are allowed")
	}
	if !validChannelId.MatchString(channelId) {
		return errors.New("RSS41 - invalid channel id")
	}
	return nil
}

func AddUrlToList(Name string, Url string, ChannelId string, cfg *config.Discord) (bool, error) {
	if err := validateFeedIdentifiers(Name, ChannelId); err != nil {
		return false, err
	}

	var dbName = cfg.DBName
	var dbColName = cfg.DBColName

	// db instance
	log.Println("dbName: ", dbName)
	db, err := simplejsondb.New(dbName, nil)
	if err != nil {
		log.Println("db error: ", err)
		return false, err
	}

	log.Println("database " + dbName + " created")

	// collection1 creation
	dbCol, err := db.Collection(dbColName)
	if err != nil {
		log.Println("table err: ", err)
		return false, err
	}
	log.Println("collection " + dbColName + " created")

	records := dbCol.GetAll()
	log.Println("records are: ", records)

	if len(records) == 0 {
		data := RSSFeed{
			Name:            Name,
			Url:             Url,
			ChannelId:       ChannelId,
			Timer:           300,
			ActiveStatus:    true,
			MentionEveryOne: false,
		}

		jsonData, err := json.Marshal(data)

		if err != nil {
			log.Println("error:", err)
		}
		newKey := Name + "_" + ChannelId
		log.Println("newKey: ", newKey)
		err = dbCol.Create(newKey, []byte(jsonData))

		if err != nil {
			log.Println(err)
			return false, err
		}

	} else {
		for _, r := range records {
			log.Println(string(r))
			feedName := gjson.Get(string(r), "name")
			feedChannel := gjson.Get(string(r), "channelId")
			feedUrl := gjson.Get(string(r), "url")
			if feedName.String() == Name && feedChannel.String() == ChannelId && feedUrl.String() == Url {
				return false, errors.New("RSS10 - Entry already exist")
			} else {
				data := RSSFeed{
					Name:            Name,
					Url:             Url,
					ChannelId:       ChannelId,
					Timer:           300,
					ActiveStatus:    true,
					MentionEveryOne: false,
				}

				jsonData, err := json.Marshal(data)

				if err != nil {
					log.Println("error:", err)
				}
				newKey := Name + "_" + ChannelId
				log.Println("newKey: ", newKey)
				err = dbCol.Create(newKey, []byte(jsonData))

				if err != nil {
					log.Println(err)
					return false, err
				}
			}
		}
	}

	return true, nil
}

func LoadFeeds(cfg *config.Discord) {
	var dbName = cfg.DBName
	var dbColName = cfg.DBColName

	// db instance
	log.Println("dbName: ", dbName)
	db, err := simplejsondb.New(dbName, nil)
	if err != nil {
		log.Println("db error: ", err)
		return
	}

	log.Println("connected to database: ", dbName)

	// collection1 creation
	dbCol, err := db.Collection(dbColName)
	if err != nil {
		log.Println("table err: ", err)
		return
	}
	log.Println("connected to collection: ", dbColName)

	records := dbCol.GetAll()

	if len(records) == 0 {
		log.Println("RSS: no feeds configured")
	} else {
		log.Println("RSS: the following feeds are configured:")
		for _, r := range records {
			feedName := gjson.Get(string(r), "name")
			feedChannel := gjson.Get(string(r), "channelId")
			feedUrl := gjson.Get(string(r), "url")
			feedTimer := gjson.Get(string(r), "timer")
			feedMentionEveryone := gjson.Get(string(r), "mentionEveryone")
			feedActiveStatus := gjson.Get(string(r), "activeStatus")
			log.Println("RSS: Name: ", feedName)
			log.Println("RSS: Url: ", feedUrl)
			log.Println("RSS: ChannelId: ", feedChannel)

			feed := RSSFeed{
				Name:            feedName.Str,
				Url:             feedUrl.Str,
				ChannelId:       feedChannel.Str,
				Timer:           int(feedTimer.Int()),
				ActiveStatus:    feedActiveStatus.Bool(),
				MentionEveryOne: feedMentionEveryone.Bool(),
			}

			// Append the RSSFeed struct to the FeedsSlice
			updateRSSFeeds(&feed)
		}
	}

}

func updateRSSFeeds(feed *RSSFeed) {
	rssFeeds.mutex.Lock()
	defer rssFeeds.mutex.Unlock()

	// Search for the feed by URL in the RSSFeeds slice
	for i, f := range rssFeeds.RSSFeeds {
		if f.Url == feed.Url {
			// Update the ActiveStatus of the found feed
			log.Println("RSS: run updateRSSFeeds for feed: ", feed)
			rssFeeds.RSSFeeds[i].ActiveStatus = feed.ActiveStatus
			return
		}
	}

	// Feed not found, append it to the RSSFeeds slice
	rssFeeds.RSSFeeds = append(rssFeeds.RSSFeeds, *feed)
}

// Configures RSS Feed parsers and starts them concurrently
func ConfigureRSSFeeds(s *discordgo.Session) {
	activeSession = s
	// Initialize the MessageQueue map
	messageQueue.MessageQueue = make(map[string][]string)

	rssFeeds.mutex.Lock()
	defer rssFeeds.mutex.Unlock()
	for _, feed := range rssFeeds.RSSFeeds {
		if !feed.ActiveStatus {
			continue
		}

		// Creates a copy of the feed in memory for manipulation
		localFeed := feed
		startFeedWorker(s, &localFeed)
	}
}

// Parses a given RSS feed and sends the latest item for comparison
func parseRSSFeed(ctx context.Context, s *discordgo.Session, feed *RSSFeed) {
	log.Println("RSS: parsing feed: ", feed)
	ticker := time.NewTicker(time.Duration(feed.Timer) * time.Second)
	defer ticker.Stop()
	backoff := 1
	for {
		select {
		case <-ctx.Done():
			log.Printf("RSS: stopped worker for feed %s (%s)", feed.Name, feed.ChannelId)
			return
		case <-ticker.C:
			// Create new feed parser
			feedParser := gofeed.NewParser()

			// Configures a backoff should the RSS feed be unavailable
			log.Println("RSS: parsing feed url: ", feed.Url)
			feedItems, err := feedParser.ParseURLWithContext(feed.Url, ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("Error parsing %s: %s", feed.Url, err)
				log.Printf("Retrying in %d seconds...", backoff+feed.Timer)
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Duration(backoff) * time.Second):
				}
				backoff *= 2
				continue
			}

			// Reset backoff if no error
			backoff = 1
			if feedItems == nil || len(feedItems.Items) == 0 {
				log.Printf("RSS: feed %s has no items", feed.Url)
				continue
			}

			var itemLink string
			// Format most recent item in RSS Feed
			if strings.Contains(feedItems.Title, "News") {
				itemLink = "https://www.gsh-lan.com" + feedItems.Items[0].Link
			} else {
				itemLink = feedItems.Items[0].Link
			}
			var message string
			if feed.MentionEveryOne {
				message = fmt.Sprintf("@everyone **%s**\n\n%s\n\n%s", feedItems.Title, feedItems.Items[0].Title, itemLink)
			} else {
				message = fmt.Sprintf("**%s**\n\n%s\n\n%s", feedItems.Title, feedItems.Items[0].Title, itemLink)
			}
			// Update the MessageQueue slice with the latest item
			updateMessageQueue(feed.Name, message)

			// Compare messages to ensure no duplicates
			compareMessages(s, feed.Name, feed.ChannelId)
		}
	}
}

// Compare messages to be sent to Discord with messages that have already been sent to avoid duplicates
func compareMessages(s *discordgo.Session, name string, channelId string) {
	log.Println("RSS: comparingMessages for name: " + name + " and channelId: " + channelId)
	// Fetch the last 50 messages from the Discord channel
	sentMessages, err := s.ChannelMessages(channelId, 50, "", "", "")
	if err != nil {
		log.Println("Error fetching messages from Discord channel:", err)
		return
	}

	// Create a map of previous messages for faster lookup
	previousMessagesMap := make(map[string]bool)
	for i := 0; i < len(sentMessages); i++ {
		if sentMessages[i].Author.ID == s.State.User.ID {
			message := sentMessages[i].Content
			previousMessagesMap[message] = true
		}
	}

	// Check if the latest message has already been sent to Discord
	if previousMessagesMap[messageQueue.MessageQueue[name][0]] {
		log.Printf("Message already sent to Discord channel, skipping %s", name)
	} else {
		// Send the latest message to Discord
		log.Printf("Sending message for %s to Discord channel %s", name, channelId)
		_, err := s.ChannelMessageSend(channelId, messageQueue.MessageQueue[name][0])
		if err != nil {
			log.Println("Error sending message to Discord channel:", err)
			return
		}
	}
}

// Updates the MessageQueue slice with the latest RSS feed item
func updateMessageQueue(name string, message string) {
	messageQueue.mutex.Lock()
	defer messageQueue.mutex.Unlock()

	if len(messageQueue.MessageQueue[name]) > 0 {
		messageQueue.MessageQueue[name] = nil
	}

	messageQueue.MessageQueue[name] = append(messageQueue.MessageQueue[name], message)
}

func RemoveFeedFromList(Name string, ChannelId string, cfg *config.Discord) (bool, error) {
	if err := validateFeedIdentifiers(Name, ChannelId); err != nil {
		return false, err
	}

	var dbName = cfg.DBName
	var dbColName = cfg.DBColName

	// db instance
	log.Println("dbName: ", dbName)
	db, err := simplejsondb.New(dbName, nil)
	if err != nil {
		log.Println("db error: ", err)
		return false, err
	}

	log.Println("database " + dbName + " connected")

	// collection1 creation
	dbCol, err := db.Collection(dbColName)
	if err != nil {
		log.Println("table err: ", err)
		return false, err
	}
	log.Println("collection " + dbColName + " connected")

	records := dbCol.GetAll()

	if len(records) == 0 {
		return false, errors.New("RSS20 - no feeds configured")
	} else {
		for _, r := range records {
			feedName := gjson.Get(string(r), "name")
			feedChannel := gjson.Get(string(r), "channelId")
			log.Println("fN: " + feedName.Str + " fC: " + feedChannel.Str)
			log.Println("N: " + Name + " CI: " + ChannelId)
			if feedName.Str == Name && feedChannel.Str == ChannelId {
				log.Println("del true true")
				removeKey := feedName.Str + "_" + feedChannel.Str
				log.Println("removeKey: ", removeKey)
				err = dbCol.Delete(removeKey)

				if err != nil {
					log.Println(err)
					return false, err
				}
				stopFeedWorker(Name, ChannelId)
				removeFeedFromMemory(Name, ChannelId)
				return true, nil
			}
		}
		log.Println("del false false")
		return false, errors.New("RSS30 - Entry could not be removed")
	}
}

// SetFeedActiveStatus updates the activeStatus of a feed in the database, updates the in-memory
// feed slice, and either starts or stops the background polling worker accordingly.
func SetFeedActiveStatus(s *discordgo.Session, Name string, ChannelId string, active bool, cfg *config.Discord) (bool, error) {
	if err := validateFeedIdentifiers(Name, ChannelId); err != nil {
		return false, err
	}

	var dbName = cfg.DBName
	var dbColName = cfg.DBColName

	db, err := simplejsondb.New(dbName, nil)
	if err != nil {
		log.Println("db error: ", err)
		return false, fmt.Errorf("failed to open database: %w", err)
	}

	dbCol, err := db.Collection(dbColName)
	if err != nil {
		log.Println("collection error: ", err)
		return false, fmt.Errorf("failed to open collection: %w", err)
	}

	records := dbCol.GetAll()
	if len(records) == 0 {
		return false, errors.New("RSS20 - no feeds configured")
	}

	var updatedFeed *RSSFeed
	for _, r := range records {
		feedName := gjson.Get(string(r), "name").Str
		feedChannel := gjson.Get(string(r), "channelId").Str
		if feedName == Name && feedChannel == ChannelId {
			var feed RSSFeed
			if err := json.Unmarshal(r, &feed); err != nil {
				return false, fmt.Errorf("failed to parse feed record: %w", err)
			}
			feed.ActiveStatus = active
			updatedFeed = &feed

			jsonData, err := json.Marshal(feed)
			if err != nil {
				return false, fmt.Errorf("failed to marshal updated feed: %w", err)
			}
			key := Name + "_" + ChannelId
			if err := dbCol.Create(key, jsonData); err != nil {
				return false, fmt.Errorf("failed to persist updated feed: %w", err)
			}
			break
		}
	}

	if updatedFeed == nil {
		return false, errors.New("RSS31 - feed not found")
	}

	memFeed := setFeedActiveStatusInMemory(Name, ChannelId, active)
	if memFeed == nil {
		updateRSSFeeds(updatedFeed)
		memFeed = updatedFeed
	}

	if !active {
		stopFeedWorker(Name, ChannelId)
	} else {
		sessionToUse := s
		if sessionToUse == nil {
			sessionToUse = activeSession
		}
		if sessionToUse != nil {
			startFeedWorker(sessionToUse, memFeed)
		}
	}

	return true, nil
}

// PauseFeed pauses parsing of a feed and stops its background worker.
func PauseFeed(s *discordgo.Session, Name string, ChannelId string, cfg *config.Discord) (bool, error) {
	return SetFeedActiveStatus(s, Name, ChannelId, false, cfg)
}

// ResumeFeed resumes parsing of a feed and starts its background worker.
func ResumeFeed(s *discordgo.Session, Name string, ChannelId string, cfg *config.Discord) (bool, error) {
	return SetFeedActiveStatus(s, Name, ChannelId, true, cfg)
}
