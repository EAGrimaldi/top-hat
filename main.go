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
	RemoveCommands = flag.Bool("rmcmd", true, "Remove all commands after shutting down or not")

	s *discordgo.Session

	durationMinValue = 10.0

	commands = []*discordgo.ApplicationCommand{
		{
			Name:        "skronk",
			Description: "skronk someone",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "target",
					Description: "Who will you skronk?",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "duration",
					Description: "How long will you skronk them for? (in seconds)",
					MinValue:    &durationMinValue,
					MaxValue:    60.0 * 60.0 * 24.0 * 7.0,
					Required:    false,
				},
			},
		},
	}

	commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"skronk": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			/* TODO:
			- track cumulative SKRONK'd duration when multiple /skronk commands on the same use overlap
			*/

			// pass command if skronk role not provided
			if len(*SkronkID) == 0 {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Command unavailable: skronk role not provided",
					},
				})
				return
			}

			// pass command if sender is skronk'd
			for _, role := range i.Member.Roles {
				if role == *SkronkID {
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "The skronk'd cannot skronk others >:(",
						},
					})
					return
				}
			}

			// get command options
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
				targetID = opt.UserValue(nil).ID
				if targetID == s.State.User.ID {
					targetID = i.Member.User.ID
					msgformat += "Skronk me? Skronk ME!? Skronk YOURSELF!!!\n"
				}
			}
			margs = append(margs, targetID)
			msgformat += "Get skronk'd <@%s>\n"

			if opt, ok := optionMap["duration"]; ok {
				duration = opt.IntValue()
			}
			margs = append(margs, duration)
			msgformat += "See you in %d seconds!\n"

			// add skronk role to target - handle possible error (usually role hierarchy issue)
			err := s.GuildMemberRoleAdd(*GuildID, targetID, *SkronkID)
			if err != nil { // probably a permission or hierarchy issue
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Something went wrong while adding a role",
					},
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

			// wait duration and remove skronk role from target
			time.AfterFunc(time.Second*time.Duration(duration), func() {
				err := s.GuildMemberRoleRemove(*GuildID, targetID, *SkronkID)
				if err != nil { // probably a permission or hierarchy issue
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "Something went wrong while removing a role",
						},
					})
					fmt.Println(err)
					return
				}
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: fmt.Sprintf(
							"Welcome back <@%s>!",
							targetID,
						),
					},
				})
			})
		},
	}
)

func init() {
	flag.Parse()

	var err error
	s, err = discordgo.New("Bot " + *BotToken)
	if err != nil {
		log.Fatalf("Invalid bot parameters: %v", err)
	}

	// find skronk role by name if role ID not provided at start up
	if len(*SkronkID) == 0 {
		log.Println("Skronk role ID was not provided, searching for skronk role by name")
		roles, err := s.GuildRoles(*GuildID)
		if err != nil {
			log.Fatalf("Invalid guild parameters: %v", err)
		}
		for _, role := range roles {
			if role.Name == "SKRONK'd" {
				*SkronkID = role.ID
			}
		}
		if len(*SkronkID) == 0 {
			log.Println("Skronk role was not found, skronk command will be unavailable")
		} else {
			log.Printf("Skronk role found. Skronk role ID is %s\nWRITE THAT DOWN!", *SkronkID)
		}
	}

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
