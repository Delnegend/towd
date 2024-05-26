package utils

import "github.com/bwmarrin/discordgo"

// =========================================================
// Pre-built discordgo interaction responses for convenience
// =========================================================

// Send a hidden reply to the interaction.
// For a hidden non-reply, use `s.ChannelMessageSend(i.ChannelID, "content")`
func InteractRespHiddenReply(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
		},
	})
}
