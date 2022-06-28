package main

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/slack-go/slack/socketmode"

	"slack-user-attendence-app/data"
	"slack-user-attendence-app/utility"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

func main() {
	// appToken := os.Getenv("SLACK_APP_TOKEN")
	appToken := "xapp-1-A03HM4PJEQK-3611823453777-8d82cbd55cd6343b8125d000868638c799d55bee02af38f08cf216a3e0c797b1"
	if appToken == "" {
		fmt.Fprintf(os.Stderr, "SLACK_APP_TOKEN must be set.\n")
		os.Exit(1)
	}

	if !strings.HasPrefix(appToken, "xapp-") {
		fmt.Fprintf(os.Stderr, "SLACK_APP_TOKEN must have the prefix \"xapp-\".")
	}

	// botToken := os.Getenv("SLACK_BOT_TOKEN")
	botToken := "xoxb-3584532805223-3592570415062-IzvofH7PCKvE4OFSwBGGz8y6"
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

	userList, err := utility.GetUserList(api)
	if err != nil {
		log.Fatalln(err)
	}

	schedulerChannel := make(chan string)

	go scheduler(userList, api, schedulerChannel)

	client.Run()
}

func scheduler(userList *[]data.UserTimeData, api *slack.Client, mainChannel chan string) {
	var apiLimitPerMinute int = 50

	currentCount := 0

	start := time.Now().Unix()

	apiChannel := make(chan uint64)

	for idx, _ := range *userList {
		(*userList)[idx].LastChecked = time.Now().Unix()
	}

	// for idx, _ := range *userList {
	// 	go getUserPresenceGoroutine(&(*userList)[idx], api, apiChannel)
	// }
	go func() {
		apiChannel <- 0
	}()

	for {
		select {
		case mainEvent := <-mainChannel:
			if mainEvent == "exit" {
				return
			}
			// log.Println(mainEvent)
		case dude := <-apiChannel:
			log.Println(dude)
			if currentCount == apiLimitPerMinute {
				duration := time.Now().Unix() - start
				if duration < 60 {
					currentCount = 0
					//start = time.Now().Unix()
					go func() {
						time.Sleep(time.Duration(60-duration) * time.Second)
						start = time.Now().Unix()
						apiChannel <- dude
					}()

				} else {
					currentCount = 0
					start = time.Now().Unix()
				}
			} else {
				if dude > uint64(len(*userList))-1 {
					go getUserPresenceGoroutine(&(*userList)[0], api, apiChannel, 0)
					currentCount += 1
				} else {
					go getUserPresenceGoroutine(&(*userList)[dude], api, apiChannel, dude)
					currentCount += 1
				}
			}
		}
	}

	fmt.Println(apiLimitPerMinute)
}

func getUserPresenceGoroutine(user *data.UserTimeData, api *slack.Client, apiChannel chan uint64, index uint64) {
	presenceData, err := api.GetUserPresence((*user).UserId)
	log.Println(presenceData, err, user.Name, user.TotalDuration)
	if err != nil {
		log.Println(err)
		apiChannel <- index + 1
	}
	if presenceData.Presence == string(data.Active) {
		if user.PresenceState == data.Active {
			user.TotalDuration += time.Now().Unix() - user.LastChecked
		}
		user.PresenceState = data.Active
		user.ActiveEpoch = time.Now().Unix()
		user.LastChecked = time.Now().Unix()
		apiChannel <- index + 1
	} else {
		if user.PresenceState == data.Active {
			user.TotalDuration += time.Now().Unix() - user.LastChecked
		}
		user.PresenceState = data.Away
		user.AwayEpoch = time.Now().Unix()
		user.LastChecked = time.Now().Unix()
		apiChannel <- index + 1
	}
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
