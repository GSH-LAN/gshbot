package rss

import (
	"encoding/json"
	"strings"
	"testing"

	"gshlan/gshbot/config"

	"github.com/bwmarrin/discordgo"
	simplejsondb "github.com/pnkj-kmr/simple-json-db"
)

const (
	testChannelId  = "123456789012345678"
	otherChannelId = "987654321098765432"
)

// newTestConfig switches the working directory to a fresh temp dir so that
// simple-json-db (which resolves relative paths against the cwd) never
// touches the real data/gshbotdb directory.
func newTestConfig(t *testing.T) *config.Discord {
	t.Helper()
	t.Chdir(t.TempDir())
	return &config.Discord{DBName: "testdb", DBColName: "feeds"}
}

func resetGlobalState() {
	rssFeeds = RSSFeeds{}
	// Mirrors the initialization ConfigureRSSFeeds performs in production,
	// since updateMessageQueue panics on a nil map.
	messageQueue = MessageQueue{MessageQueue: make(map[string][]string)}
	StopRSSFeeds()
	activeSession = nil
}

func TestAddUrlToList_NewEntry(t *testing.T) {
	cfg := newTestConfig(t)

	added, err := AddUrlToList("news", "https://example.com/rss", testChannelId, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !added {
		t.Fatal("expected feed to be added")
	}

	db, err := simplejsondb.New(cfg.DBName, nil)
	if err != nil {
		t.Fatalf("failed to reopen db: %v", err)
	}
	col, err := db.Collection(cfg.DBColName)
	if err != nil {
		t.Fatalf("failed to open collection: %v", err)
	}
	records := col.GetAll()
	if len(records) != 1 {
		t.Fatalf("expected 1 persisted record, got %d", len(records))
	}

	var feed RSSFeed
	if err := json.Unmarshal(records[0], &feed); err != nil {
		t.Fatalf("failed to unmarshal record: %v", err)
	}
	if feed.Name != "news" || feed.Url != "https://example.com/rss" || feed.ChannelId != testChannelId {
		t.Fatalf("unexpected feed persisted: %+v", feed)
	}
}

func TestAddUrlToList_DuplicateEntry(t *testing.T) {
	cfg := newTestConfig(t)

	if _, err := AddUrlToList("news", "https://example.com/rss", testChannelId, cfg); err != nil {
		t.Fatalf("unexpected error on first add: %v", err)
	}

	added, err := AddUrlToList("news", "https://example.com/rss", testChannelId, cfg)
	if err == nil {
		t.Fatal("expected error for duplicate entry")
	}
	if added {
		t.Fatal("expected added to be false for duplicate entry")
	}
	if !strings.Contains(err.Error(), "already exist") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestAddUrlToList_RejectsInvalidName(t *testing.T) {
	cfg := newTestConfig(t)

	added, err := AddUrlToList("../../etc/cron.d/evil", "https://example.com/rss", testChannelId, cfg)
	if err == nil {
		t.Fatal("expected error for name containing path traversal characters")
	}
	if added {
		t.Fatal("expected added to be false for invalid name")
	}
	if !strings.Contains(err.Error(), "invalid feed name") {
		t.Fatalf("unexpected error message: %v", err)
	}

	db, _ := simplejsondb.New(cfg.DBName, nil)
	col, _ := db.Collection(cfg.DBColName)
	if len(col.GetAll()) != 0 {
		t.Fatal("expected no record to be persisted for an invalid name")
	}
}

func TestAddUrlToList_RejectsInvalidChannelId(t *testing.T) {
	cfg := newTestConfig(t)

	added, err := AddUrlToList("news", "https://example.com/rss", "not-a-snowflake", cfg)
	if err == nil {
		t.Fatal("expected error for a non-numeric channel id")
	}
	if added {
		t.Fatal("expected added to be false for invalid channel id")
	}
	if !strings.Contains(err.Error(), "invalid channel id") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestRemoveFeedFromList_Success(t *testing.T) {
	cfg := newTestConfig(t)

	if _, err := AddUrlToList("news", "https://example.com/rss", testChannelId, cfg); err != nil {
		t.Fatalf("unexpected error on add: %v", err)
	}

	removed, err := RemoveFeedFromList("news", testChannelId, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !removed {
		t.Fatal("expected feed to be removed")
	}

	db, _ := simplejsondb.New(cfg.DBName, nil)
	col, _ := db.Collection(cfg.DBColName)
	if len(col.GetAll()) != 0 {
		t.Fatal("expected no records left after removal")
	}
}

func TestRemoveFeedFromList_NoFeedsConfigured(t *testing.T) {
	cfg := newTestConfig(t)

	removed, err := RemoveFeedFromList("news", testChannelId, cfg)
	if err == nil {
		t.Fatal("expected error when no feeds are configured")
	}
	if removed {
		t.Fatal("expected removed to be false")
	}
	if !strings.Contains(err.Error(), "no feeds configured") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestRemoveFeedFromList_Mismatch(t *testing.T) {
	cfg := newTestConfig(t)

	if _, err := AddUrlToList("news", "https://example.com/rss", testChannelId, cfg); err != nil {
		t.Fatalf("unexpected error on add: %v", err)
	}

	removed, err := RemoveFeedFromList("other", otherChannelId, cfg)
	if err == nil {
		t.Fatal("expected error when entry does not match")
	}
	if removed {
		t.Fatal("expected removed to be false")
	}
	if !strings.Contains(err.Error(), "could not be removed") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestRemoveFeedFromList_SkipsMismatchesAndRemovesMatchingRecord(t *testing.T) {
	cfg := newTestConfig(t)

	if _, err := AddUrlToList("other", "https://example.com/other", otherChannelId, cfg); err != nil {
		t.Fatalf("unexpected error on add: %v", err)
	}
	if _, err := AddUrlToList("news", "https://example.com/rss", testChannelId, cfg); err != nil {
		t.Fatalf("unexpected error on add: %v", err)
	}

	removed, err := RemoveFeedFromList("news", testChannelId, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !removed {
		t.Fatal("expected the matching feed to be removed despite an earlier mismatching record")
	}

	db, _ := simplejsondb.New(cfg.DBName, nil)
	col, _ := db.Collection(cfg.DBColName)
	if len(col.GetAll()) != 1 {
		t.Fatal("expected only the matching feed to be removed, other records must remain")
	}
}

func TestRemoveFeedFromList_RejectsInvalidIdentifiers(t *testing.T) {
	cfg := newTestConfig(t)

	removed, err := RemoveFeedFromList("../../etc/passwd", testChannelId, cfg)
	if err == nil {
		t.Fatal("expected error for name containing path traversal characters")
	}
	if removed {
		t.Fatal("expected removed to be false for invalid name")
	}
	if !strings.Contains(err.Error(), "invalid feed name") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestUpdateRSSFeeds_AppendsNewFeed(t *testing.T) {
	resetGlobalState()
	t.Cleanup(resetGlobalState)

	updateRSSFeeds(&RSSFeed{Name: "news", Url: "https://example.com/a", ActiveStatus: true})
	if len(rssFeeds.RSSFeeds) != 1 {
		t.Fatalf("expected 1 feed, got %d", len(rssFeeds.RSSFeeds))
	}

	updateRSSFeeds(&RSSFeed{Name: "other", Url: "https://example.com/b", ActiveStatus: true})
	if len(rssFeeds.RSSFeeds) != 2 {
		t.Fatalf("expected 2 feeds after adding a new url, got %d", len(rssFeeds.RSSFeeds))
	}
}

func TestUpdateRSSFeeds_ReprocessingKnownUrlUpdatesInPlace(t *testing.T) {
	resetGlobalState()
	t.Cleanup(resetGlobalState)

	updateRSSFeeds(&RSSFeed{Name: "news", Url: "https://example.com/a", ActiveStatus: true})
	updateRSSFeeds(&RSSFeed{Name: "news", Url: "https://example.com/a", ActiveStatus: false})

	if len(rssFeeds.RSSFeeds) != 1 {
		t.Fatalf("expected the existing entry to be updated in place, got %d feeds", len(rssFeeds.RSSFeeds))
	}
	if rssFeeds.RSSFeeds[0].ActiveStatus {
		t.Fatal("expected the entry to have ActiveStatus=false")
	}
}

func TestUpdateMessageQueue(t *testing.T) {
	resetGlobalState()
	t.Cleanup(resetGlobalState)

	updateMessageQueue("news", "first message")
	updateMessageQueue("news", "second message")

	queue := messageQueue.MessageQueue["news"]
	if len(queue) != 1 {
		t.Fatalf("expected queue to be reset and hold 1 message, got %d", len(queue))
	}
	if queue[0] != "second message" {
		t.Fatalf("expected latest message to be kept, got %q", queue[0])
	}
}

func TestLoadFeeds(t *testing.T) {
	cfg := newTestConfig(t)
	resetGlobalState()
	t.Cleanup(resetGlobalState)

	if _, err := AddUrlToList("news", "https://example.com/rss", testChannelId, cfg); err != nil {
		t.Fatalf("unexpected error on add: %v", err)
	}

	LoadFeeds(cfg)

	if len(rssFeeds.RSSFeeds) != 1 {
		t.Fatalf("expected 1 feed loaded, got %d", len(rssFeeds.RSSFeeds))
	}
	feed := rssFeeds.RSSFeeds[0]
	if feed.Name != "news" || feed.Url != "https://example.com/rss" || feed.ChannelId != testChannelId {
		t.Fatalf("unexpected feed loaded: %+v", feed)
	}
	if feed.Timer != 300 || !feed.ActiveStatus {
		t.Fatalf("unexpected default feed settings: %+v", feed)
	}
}

func TestPauseFeed_Success(t *testing.T) {
	cfg := newTestConfig(t)
	resetGlobalState()
	t.Cleanup(resetGlobalState)

	if _, err := AddUrlToList("news", "https://example.com/rss", testChannelId, cfg); err != nil {
		t.Fatalf("unexpected error on add: %v", err)
	}
	LoadFeeds(cfg)

	ok, err := PauseFeed(nil, "news", testChannelId, cfg)
	if err != nil {
		t.Fatalf("unexpected error on pause: %v", err)
	}
	if !ok {
		t.Fatal("expected pause to return true")
	}

	// Verify database record
	db, _ := simplejsondb.New(cfg.DBName, nil)
	col, _ := db.Collection(cfg.DBColName)
	records := col.GetAll()
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	var persisted RSSFeed
	if err := json.Unmarshal(records[0], &persisted); err != nil {
		t.Fatalf("failed to unmarshal persisted feed: %v", err)
	}
	if persisted.ActiveStatus {
		t.Fatal("expected persisted ActiveStatus to be false")
	}

	// Verify in-memory state
	if len(rssFeeds.RSSFeeds) != 1 || rssFeeds.RSSFeeds[0].ActiveStatus {
		t.Fatalf("expected in-memory feed ActiveStatus to be false, got: %+v", rssFeeds.RSSFeeds)
	}
}

func TestResumeFeed_Success(t *testing.T) {
	cfg := newTestConfig(t)
	resetGlobalState()
	t.Cleanup(resetGlobalState)

	if _, err := AddUrlToList("news", "https://example.com/rss", testChannelId, cfg); err != nil {
		t.Fatalf("unexpected error on add: %v", err)
	}
	LoadFeeds(cfg)

	if _, err := PauseFeed(nil, "news", testChannelId, cfg); err != nil {
		t.Fatalf("unexpected error on pause: %v", err)
	}

	ok, err := ResumeFeed(nil, "news", testChannelId, cfg)
	if err != nil {
		t.Fatalf("unexpected error on resume: %v", err)
	}
	if !ok {
		t.Fatal("expected resume to return true")
	}

	// Verify database record
	db, _ := simplejsondb.New(cfg.DBName, nil)
	col, _ := db.Collection(cfg.DBColName)
	var persisted RSSFeed
	if err := json.Unmarshal(col.GetAll()[0], &persisted); err != nil {
		t.Fatalf("failed to unmarshal persisted feed: %v", err)
	}
	if !persisted.ActiveStatus {
		t.Fatal("expected persisted ActiveStatus to be true")
	}

	// Verify in-memory state
	if len(rssFeeds.RSSFeeds) != 1 || !rssFeeds.RSSFeeds[0].ActiveStatus {
		t.Fatalf("expected in-memory feed ActiveStatus to be true, got: %+v", rssFeeds.RSSFeeds)
	}
}

func TestPauseFeed_NoFeedsConfigured(t *testing.T) {
	cfg := newTestConfig(t)
	resetGlobalState()
	t.Cleanup(resetGlobalState)

	ok, err := PauseFeed(nil, "news", testChannelId, cfg)
	if err == nil {
		t.Fatal("expected error when no feeds configured")
	}
	if ok {
		t.Fatal("expected ok to be false")
	}
	if !strings.Contains(err.Error(), "no feeds configured") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestPauseFeed_FeedNotFound(t *testing.T) {
	cfg := newTestConfig(t)
	resetGlobalState()
	t.Cleanup(resetGlobalState)

	if _, err := AddUrlToList("news", "https://example.com/rss", testChannelId, cfg); err != nil {
		t.Fatalf("unexpected error on add: %v", err)
	}

	ok, err := PauseFeed(nil, "other", otherChannelId, cfg)
	if err == nil {
		t.Fatal("expected error when feed not found")
	}
	if ok {
		t.Fatal("expected ok to be false")
	}
	if !strings.Contains(err.Error(), "feed not found") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestPauseFeed_RejectsInvalidIdentifiers(t *testing.T) {
	cfg := newTestConfig(t)
	resetGlobalState()
	t.Cleanup(resetGlobalState)

	if _, err := PauseFeed(nil, "../../invalid", testChannelId, cfg); err == nil {
		t.Fatal("expected error on invalid feed name")
	}

	if _, err := PauseFeed(nil, "news", "not-a-snowflake", cfg); err == nil {
		t.Fatal("expected error on invalid channel id")
	}
}

func TestWorkerLifecycle_StartStopAndRemove(t *testing.T) {
	cfg := newTestConfig(t)
	resetGlobalState()
	t.Cleanup(resetGlobalState)

	s, err := discordgo.New("Bot faketoken")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	feed := RSSFeed{
		Name:         "news",
		Url:          "https://example.com/rss",
		ChannelId:    testChannelId,
		Timer:        300,
		ActiveStatus: true,
	}

	// 1. Worker start & stop
	startFeedWorker(s, &feed)
	if !IsWorkerRunning(feed.Name, feed.ChannelId) {
		t.Fatal("expected worker to be running after startFeedWorker")
	}

	stopFeedWorker(feed.Name, feed.ChannelId)
	if IsWorkerRunning(feed.Name, feed.ChannelId) {
		t.Fatal("expected worker to be stopped after stopFeedWorker")
	}

	// 2. RemoveFeedFromList stops active worker
	if _, err := AddUrlToList("news", "https://example.com/rss", testChannelId, cfg); err != nil {
		t.Fatalf("unexpected error on add: %v", err)
	}
	startFeedWorker(s, &feed)
	if !IsWorkerRunning(feed.Name, feed.ChannelId) {
		t.Fatal("expected worker to be running before removal")
	}

	if _, err := RemoveFeedFromList("news", testChannelId, cfg); err != nil {
		t.Fatalf("unexpected error on remove: %v", err)
	}
	if IsWorkerRunning(feed.Name, feed.ChannelId) {
		t.Fatal("expected worker to be stopped after RemoveFeedFromList")
	}
}
