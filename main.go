package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"reflect"
	"syscall"
	"time"

	"test/config"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/json"
	"github.com/disgoorg/snowflake/v2"
)

var commands = []discord.ApplicationCommandCreate{
	discord.SlashCommandCreate{
		Name:        "test",
		Description: "test",
	},
}

func main() {
	h := handler.New()
	h.Command("/test", testCommand)

	client, err := disgo.New(config.Token,
		bot.WithGatewayConfigOpts(
			config.Intents,
		),
		bot.WithCacheConfigOpts(
			cache.WithCaches(cache.FlagGuilds|cache.FlagMembers|cache.FlagMessages|cache.FlagChannels|cache.FlagEmojis),
		),
		bot.WithEventListeners(h),
	)
	if err != nil {
		log.Panic("Error when creating client: ", err)
	}

	if err = handler.SyncCommands(client, commands, []snowflake.ID{snowflake.MustParse("807241041425334323")}); err != nil {
		log.Panic("Error while syncing commands: ", err)
	}

	defer client.Close(context.TODO())

	if err = client.OpenGateway(context.TODO()); err != nil {
		log.Fatal("error while connecting to gateway: ", err)
	}

	log.Println("testbot is now running. Press CTRL-C to exit.")
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-s
}

func getButtonData(
	c *handler.CommandEvent,
	errorCh chan error,
	boolChannel chan bool,
	newCtxChannel chan events.ComponentInteractionCreate,
) {
	defer close(errorCh)
	defer close(boolChannel)
	defer close(newCtxChannel)

	collector, cancelCollector := bot.NewEventCollector(
		c.Client(),
		func (e *events.ComponentInteractionCreate) bool {
			return e.Type() == discord.ComponentTypeButton &&
			e.ButtonInteractionData().CustomID() == "test:button" &&
			e.User().ID == c.User().ID
		},
	)

	timeout, cancelTimeout := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancelTimeout()

	for {
		select {
			case <-timeout.Done(): {
				cancelCollector()

				errorCh <- errors.New("timeout")
				boolChannel <- false
				newCtxChannel <- events.ComponentInteractionCreate{}

				return
			}
			case componentEvent := <-collector: {
				cancelCollector()

				errorCh <- nil
				boolChannel <- true
				newCtxChannel <- *componentEvent
			}
		}
	}
}

func GetButton(
	c *handler.CommandEvent,
	msg *discord.Message,
) (bool, events.ComponentInteractionCreate, error) {
	boolChannel := make(chan bool)
	newCtxChannel := make(chan events.ComponentInteractionCreate)
	errorCh := make(chan error)
	go getButtonData(c, errorCh, boolChannel, newCtxChannel)
	err, value, newCtx := <- errorCh, <- boolChannel, <- newCtxChannel
	switch err {
		case errors.New("timeout"): {
			if !reflect.ValueOf(newCtx).IsZero() {
				newCtx.DeferUpdateMessage()
			}

			c.Client().Rest().UpdateMessage(
				msg.ChannelID,
				msg.ID,
				discord.MessageUpdate{
					Content:    json.Ptr("Too late!"),
					Embeds:     json.Ptr([]discord.Embed{}),
					Components: json.Ptr([]discord.ContainerComponent{}),
				},
			)

			return false, events.ComponentInteractionCreate{}, errors.New("resolved")
		}
		default: {
			if err != nil {
				if !reflect.ValueOf(newCtx).IsZero() {
					newCtx.DeferUpdateMessage()
				}
	
				c.Client().Rest().UpdateMessage(
					msg.ChannelID,
					msg.ID,
					discord.MessageUpdate{
						Content:    json.Ptr("Random error!"),
						Embeds:     json.Ptr([]discord.Embed{}),
						Components: json.Ptr([]discord.ContainerComponent{}),
					},
				)
	
				return false, events.ComponentInteractionCreate{}, errors.New("resolved")
			
			}
		}
	}

	return value, newCtx, nil
}

func testCommand(c *handler.CommandEvent) error {
	go func() {
		c.Respond(
			discord.InteractionResponseTypeDeferredCreateMessage,
			discord.MessageCreate{},
		)

		msg, err := c.UpdateInteractionResponse(discord.MessageUpdate{
			Content: json.Ptr("Hello world!"),
			Components: json.Ptr([]discord.ContainerComponent{
				discord.ActionRowComponent{
					discord.NewPrimaryButton("button", "test:button"),
				},
			}),
		})
		if err != nil {
			log.Panic("An error ocurred when trying to respond initialy: ", err)
			return
		}

		value, newCtx, err := GetButton(c, msg)
		if err == errors.New("resolved") {
			return
		}

		newCtx.DeferUpdateMessage()

		c.Client().Rest().UpdateMessage(
			msg.ChannelID,
			msg.ID,
			discord.MessageUpdate{
				Content:    json.Ptr(fmt.Sprintf("Thanks for clicking the button! (Val: %v)", value)),
				Embeds:     json.Ptr([]discord.Embed{}),
				Components: json.Ptr([]discord.ContainerComponent{}),
			},
		)
	}()

	return nil
}