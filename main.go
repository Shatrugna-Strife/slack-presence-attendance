package main

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"

	"github.com/slack-go/slack/socketmode"

	"slack-user-attendence-app/utility"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

func main() {
	// appToken := os.Getenv("SLACK_APP_TOKEN")
	appToken := "xapp-1-A03FTQK751R-3586199122002-4f4223ea6867aa909080444bdcddd2a88698c1267a8e85884e6d1bf8bd430bf0"
	if appToken == "" {
		fmt.Fprintf(os.Stderr, "SLACK_APP_TOKEN must be set.\n")
		os.Exit(1)
	}

	if !strings.HasPrefix(appToken, "xapp-") {
		fmt.Fprintf(os.Stderr, "SLACK_APP_TOKEN must have the prefix \"xapp-\".")
	}

	// botToken := os.Getenv("SLACK_BOT_TOKEN")
	botToken := "xoxb-3537805463267-3523293428999-OpiYeQDdUzz89AUTiqxqNmil"
	if botToken == "" {
		fmt.Fprintf(os.Stderr, "SLACK_BOT_TOKEN must be set.\n")
		os.Exit(1)
	}

	if !strings.HasPrefix(botToken, "xoxb-") {
		fmt.Fprintf(os.Stderr, "SLACK_BOT_TOKEN must have the prefix \"xoxb-\".")
	}

	api := slack.New(
		botToken,
		slack.OptionDebug(true),
		slack.OptionLog(log.New(os.Stdout, "api: ", log.Lshortfile|log.LstdFlags)),
		slack.OptionAppLevelToken(appToken),
	)

	client := socketmode.New(
		api,
		socketmode.OptionDebug(true),
		socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)

	go event(client, api)

	utility.GetUserList(api)

	client.Run()
}

func event(client *socketmode.Client, api *slack.Client) {
	for {
		select {
		case evt := <-client.Events:
			log.Println(evt)
			switch evt.Type {
			case socketmode.EventTypeConnecting:
				fmt.Println("Connecting to Slack with Socket Mode...")
			case socketmode.EventTypeConnectionError:
				fmt.Println("Connection failed. Retrying later...")
			case socketmode.EventTypeConnected:
				fmt.Println("Connected to Slack with Socket Mode.")
			case socketmode.EventTypeEventsAPI:
				eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
				if !ok {
					fmt.Printf("Ignored %+v\n", evt)
				} else {
					fmt.Printf("Event received: %+v\n", eventsAPIEvent)

					client.Ack(*evt.Request)

					switch eventsAPIEvent.Type {
					case slackevents.CallbackEvent:
						innerEvent := eventsAPIEvent.InnerEvent
						// ev:=
						log.Println(reflect.TypeOf(innerEvent.Data))

						switch ev := innerEvent.Data.(type) {

						case *slackevents.AppMentionEvent:
							log.Println(ev.Text)
							_, _, err := api.PostMessage(ev.Channel, slack.MsgOptionText("Yes, hello.", false))
							if err != nil {
								fmt.Printf("failed posting message: %v", err)
							}
						case *slackevents.MemberJoinedChannelEvent:
							fmt.Printf("user %q joined to channel %q", ev.User, ev.Channel)
						}
					default:
						client.Debugf("unsupported Events API event received")
					}
				}
			default:
				fmt.Fprintf(os.Stderr, "Unexpected event type received: %s\n", evt.Type)
			}
		}
	}
	log.Println("Shit 1")
}
