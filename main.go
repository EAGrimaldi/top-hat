package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	GuildID        = flag.String("guild", "", "Test guild ID. If not passed - bot registers commands globally")
	SkronkID       = flag.String("skronk", "", "Skronk role ID. If not passed - bot searches for role by name")
	BotToken       = flag.String("token", "", "Bot access token")
	RemoveCommands = flag.Bool("rmcmd", true, "Remove all commands after shutdowning or not")
)

var s *discordgo.Session

func init() { flag.Parse() }

func init() {
	var err error
	s, err = discordgo.New("Bot " + *BotToken)
	if err != nil {
		log.Fatalf("Invalid bot parameters: %v", err)
	}
}

var (
	durationMinValue = 10.0

	commands = []*discordgo.ApplicationCommand{
		{
			Name:        "skronk",
			Description: "skronk someone",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "target",
					Description: "who are you skronk'ing?",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "duration",
					Description: "how long are they skronk'd for? (in seconds)",
					MinValue:    &durationMinValue,
					MaxValue:    60.0 * 60.0 * 24.0 * 7.0,
					Required:    false,
				},
			},
		},
	}

	commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"skronk": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			// TODO:
			// - find "SKRONK'd" role if roleID not provided
			// - blacklist SKRONK'd users from bot commands
			options := i.ApplicationCommandData().Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			margs := make([]interface{}, 0, len(options))
			msgformat := ""
			targetID := ""
			var duration int64 = int64(durationMinValue)

			if opt, ok := optionMap["target"]; ok {
				if opt.UserValue(nil).ID == s.State.User.ID {
					targetID = i.Member.User.ID
					msgformat += "skronk me? skronk ME!? skronk YOURSELF!!!"
				} else {
					targetID = opt.UserValue(nil).ID
				}
			} else {
				targetID = i.Member.User.ID
				msgformat += "how did you fuck that up?"
			}
			margs = append(margs, targetID)
			msgformat += "get skronk'd <@%s>\n"

			if opt, ok := optionMap["duration"]; ok {
				duration = opt.IntValue()
			}
			margs = append(margs, duration)
			msgformat += "see you in %d seconds!\n"

			err := s.GuildMemberRoleAdd(*GuildID, targetID, *SkronkID)
			if err != nil {
				s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
					Content: "Something went wrong while adding a role",
				})
				fmt.Println(err)
				return
			}
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf(
						msgformat,
						margs...,
					),
				},
			})
			time.AfterFunc(time.Second*time.Duration(duration), func() {
				err := s.GuildMemberRoleRemove(*GuildID, targetID, *SkronkID)
				if err != nil {
					s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
						Content: "Something went wrong while removing a role",
					})
					fmt.Println(err)
					return
				}
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: fmt.Sprintf(
							"welcome back <@%s>!",
							targetID,
						),
					},
				})
			})
		},
	}
)

func init() {
	s.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})
}

func main() {
	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
	})
	err := s.Open()
	if err != nil {
		log.Fatalf("Cannot open the session: %v", err)
	}

	log.Println("Adding commands...")
	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands))
	for i, v := range commands {
		cmd, err := s.ApplicationCommandCreate(s.State.User.ID, *GuildID, v)
		if err != nil {
			log.Panicf("Cannot create '%v' command: %v", v.Name, err)
		}
		registeredCommands[i] = cmd
	}

	defer s.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	log.Println("Press Ctrl+C to exit")
	<-stop

	if *RemoveCommands {
		log.Println("Removing commands...")
		for _, v := range registeredCommands {
			err := s.ApplicationCommandDelete(s.State.User.ID, *GuildID, v.ID)
			if err != nil {
				log.Panicf("Cannot delete '%v' command: %v", v.Name, err)
			}
		}
	}

	log.Println("Gracefully shutting down.")
}
