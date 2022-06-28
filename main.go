package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/slack-go/slack/socketmode"

	"slack-user-attendence-app/config"
	"slack-user-attendence-app/data"
	"slack-user-attendence-app/utility"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

func main() {

	if config.AppToken == "" {
		fmt.Fprintf(os.Stderr, "SLACK_APP_TOKEN must be set.\n")
		os.Exit(1)
	}

	if !strings.HasPrefix(config.AppToken, "xapp-") {
		fmt.Fprintf(os.Stderr, "SLACK_APP_TOKEN must have the prefix \"xapp-\".")
		os.Exit(1)
	}

	if config.BotToken == "" {
		fmt.Fprintf(os.Stderr, "SLACK_BOT_TOKEN must be set.\n")
		os.Exit(1)
	}

	if !strings.HasPrefix(config.BotToken, "xoxb-") {
		fmt.Fprintf(os.Stderr, "SLACK_BOT_TOKEN must have the prefix \"xoxb-\".")
		os.Exit(1)
	}

	api := slack.New(
		config.BotToken,
		slack.OptionDebug(true),
		slack.OptionLog(log.New(os.Stdout, "api: ", log.Lshortfile|log.LstdFlags)),
		slack.OptionAppLevelToken(config.AppToken),
	)

	client := socketmode.New(
		api,
		socketmode.OptionDebug(true),
		socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)

	ctx := context.Background()
	conf, err := google.JWTConfigFromJSON([]byte(config.GoogleServiceJsonKey), sheets.SpreadsheetsScope)
	checkError(err)

	clientSheet := conf.Client(context.TODO())
	srv, err := sheets.NewService(ctx, option.WithHTTPClient(clientSheet))
	checkError(err)

	go dayScheduler(api, srv)

	go event(client, api)
	client.Run()
}

func checkError(err error) {
	if err != nil {
		panic(err.Error())
	}
}

func dayScheduler(api *slack.Client, srv *sheets.Service) {
	//declaration

	schedulerChannel := make(chan string)

	sheetschedulerChannel := make(chan string)

	daytempchannel := make(chan bool)

	// false schedulers not running
	go func() {
		daytempchannel <- false
	}()

	// go
	for dude := range daytempchannel {
		log.Println(dude)
		currentTime := time.Now()
		startTime := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), config.StartTimeStamp.Hour, config.StartTimeStamp.Minute, 0, 0, currentTime.Location())
		endTime := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), config.EndTimeStamp.Hour, config.EndTimeStamp.Minute, 0, 0, currentTime.Location())
		if !dude { // false schedulers not running
			if time.Now().After(startTime) && time.Now().Before(endTime) {
				// schedulers starting
				userMap, err := utility.GetUserMap(api)
				if err != nil {
					log.Fatalln(err)
				}
				userList := utility.GenerateListFromMap(userMap)
				go scheduler(userList, api, schedulerChannel)

				go googleSheetScheduler(userList, srv, sheetschedulerChannel)

				go func() {
					time.Sleep(config.DaySchedulerDelay * time.Minute)
					daytempchannel <- true
				}()
			} else { // true schedulers are running
				go func() {
					time.Sleep(config.DaySchedulerDelay * time.Minute)
					daytempchannel <- false
				}()
			}
		} else {
			if time.Now().After(startTime) && time.Now().Before(endTime) {
				go func() {
					time.Sleep(config.DaySchedulerDelay * time.Minute)
					daytempchannel <- true
				}()
			} else {
				schedulerChannel <- "exit"
				sheetschedulerChannel <- "exit"
				go func() {
					time.Sleep(config.DaySchedulerDelay * time.Minute)
					daytempchannel <- false
				}()
			}
		}

	}
}

func googleSheetScheduler(userList *[]data.UserTimeData, srv *sheets.Service, mainChannel chan string) {

	tempChannel := make(chan uint64)

	go func() {
		tempChannel <- 1
	}()

	for {
		select {
		case mainEvent := <-mainChannel:
			if mainEvent == "exit" {
				return
			}
		case <-tempChannel:
			month := time.Now().Month()
			year := time.Now().Year()
			day := time.Now().Day()

			userrange := month.String() + " " + strconv.Itoa(year) + "!" + "A" + "1"

			writeRange := month.String() + " " + strconv.Itoa(year) + "!" + utility.GetColumnName(day*3+1) + "1"
			_, err := srv.Spreadsheets.Values.Get(config.SpreadsheetID, userrange).Do()
			if err != nil {
				batch := sheets.BatchUpdateSpreadsheetRequest{Requests: []*sheets.Request{&sheets.Request{AddSheet: &sheets.AddSheetRequest{Properties: &sheets.SheetProperties{Title: month.String() + " " + strconv.Itoa(year)}}}}}

				_, err := srv.Spreadsheets.BatchUpdate(config.SpreadsheetID, &batch).Do()
				if err != nil {
					log.Fatalf("Unable to retrieve data from sheet. %v", err)
				}
			}

			var ur sheets.ValueRange
			ur.Values = append(ur.Values, []interface{}{"ID", "Name"})

			var wr sheets.ValueRange
			wr.Values = append(wr.Values, []interface{}{"TimeSpent - Seconds", "Day " + strconv.Itoa(day) + " - Attendance"})

			var active string

			for idx := range *userList {
				ur.Values = append(ur.Values, []interface{}{(*userList)[idx].UserId, (*userList)[idx].Name})
				if (*userList)[idx].TotalDuration > config.PresentBreakPoint {
					active = "Present"
				} else {
					active = "Absent"
				}
				log.Println((*userList)[idx])
				wr.Values = append(wr.Values, []interface{}{strconv.Itoa(int((*userList)[idx].TotalDuration)), active})
			}

			_, err = srv.Spreadsheets.Values.Update(config.SpreadsheetID, userrange, &ur).ValueInputOption("RAW").Do()
			if err != nil {
				log.Fatalf("Unable to retrieve data from sheet. %v", err)
			}

			_, err = srv.Spreadsheets.Values.Update(config.SpreadsheetID, writeRange, &wr).ValueInputOption("RAW").Do()
			if err != nil {
				log.Fatalf("Unable to retrieve data from sheet. %v", err)
			}

			go func() {
				time.Sleep(config.GoogleSheetSchedulerDelay * time.Second)
				tempChannel <- 1
			}()
		}

	}

}

func scheduler(userList *[]data.UserTimeData, api *slack.Client, mainChannel chan string) {

	currentCount := 0

	start := time.Now().Unix()

	apiChannel := make(chan uint64)

	for idx := range *userList {
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
			if currentCount == config.PresenceApiLimitPerMinute {
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
							_, _, err := api.PostMessage(ev.Channel, slack.MsgOptionText("Ignore this noble being, and * ur self", false))
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
	// log.Println("Shit 1")
}
