package messagehandler

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

// newSessionWithGuild seeds the session's local state cache with a guild,
// channel and member, so permission checks resolve without any network call.
func newSessionWithGuild(t *testing.T, guildID, channelID, userID string, memberRoles []string, roles []*discordgo.Role) *discordgo.Session {
	t.Helper()
	s, err := discordgo.New("Bot faketoken")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	s.State.User = &discordgo.User{ID: "bot-id"}

	channel := &discordgo.Channel{ID: channelID, GuildID: guildID, Type: discordgo.ChannelTypeGuildText}
	guild := &discordgo.Guild{ID: guildID, OwnerID: "someone-else", Roles: roles, Channels: []*discordgo.Channel{channel}}
	if err := s.State.GuildAdd(guild); err != nil {
		t.Fatalf("failed to seed guild state: %v", err)
	}
	member := &discordgo.Member{GuildID: guildID, User: &discordgo.User{ID: userID}, Roles: memberRoles}
	if err := s.State.MemberAdd(member); err != nil {
		t.Fatalf("failed to seed member state: %v", err)
	}
	return s
}

func TestHasManageGuildPermission_Authorized(t *testing.T) {
	const guildID, channelID, userID, managerRoleID = "guild-1", "chan-1", "user-1", "role-manager"
	roles := []*discordgo.Role{
		{ID: guildID, Permissions: 0},
		{ID: managerRoleID, Permissions: discordgo.PermissionManageGuild},
	}
	s := newSessionWithGuild(t, guildID, channelID, userID, []string{managerRoleID}, roles)

	msg := &discordgo.MessageCreate{Message: &discordgo.Message{ChannelID: channelID, Author: &discordgo.User{ID: userID}}}

	if !hasManageGuildPermission(s, msg) {
		t.Fatal("expected member with the Manage Server permission to be authorized")
	}
}

func TestHasManageGuildPermission_Unauthorized(t *testing.T) {
	const guildID, channelID, userID = "guild-1", "chan-1", "user-1"
	roles := []*discordgo.Role{{ID: guildID, Permissions: 0}}
	s := newSessionWithGuild(t, guildID, channelID, userID, nil, roles)

	msg := &discordgo.MessageCreate{Message: &discordgo.Message{ChannelID: channelID, Author: &discordgo.User{ID: userID}}}

	if hasManageGuildPermission(s, msg) {
		t.Fatal("expected member without the Manage Server permission to be denied")
	}
}
