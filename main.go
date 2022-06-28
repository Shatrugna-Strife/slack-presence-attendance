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

	userMap, err := utility.GetUserMap(api)
	if err != nil {
		log.Fatalln(err)
	}
	userList := utility.GenerateListFromMap(userMap)

	schedulerChannel := make(chan string)

	ctx := context.Background()
	conf, err := google.JWTConfigFromJSON([]byte(config.GoogleServiceJsonKey), sheets.SpreadsheetsScope)
	checkError(err)

	clientSheet := conf.Client(context.TODO())
	srv, err := sheets.NewService(ctx, option.WithHTTPClient(clientSheet))
	checkError(err)

	go scheduler(userList, api, schedulerChannel)

	go googleSheetScheduler(userList, srv)

	client.Run()
}

func checkError(err error) {
	if err != nil {
		panic(err.Error())
	}
}

func googleSheetScheduler(userList *[]data.UserTimeData, srv *sheets.Service) {

	for {
		month := time.Now().Month()
		year := time.Now().Year()

		userrange := month.String() + " " + strconv.Itoa(year) + "!" + "A" + "1"

		writeRange := month.String() + " " + strconv.Itoa(year) + "!" + utility.GetColumnName(time.Now().Day()*3+1) + "1"
		_, err := srv.Spreadsheets.Values.Get(config.SpreadsheetID, userrange).Do()
		if err != nil {
			batch := sheets.BatchUpdateSpreadsheetRequest{Requests: []*sheets.Request{&sheets.Request{AddSheet: &sheets.AddSheetRequest{Properties: &sheets.SheetProperties{Title: month.String() + " " + strconv.Itoa(year)}}}}}

			_, err := srv.Spreadsheets.BatchUpdate(config.SpreadsheetID, &batch).Do()
			if err != nil {
				log.Fatalf("Unable to retrieve data from sheet. %v", err)
			}
			// _, err = srv.Spreadsheets.Values.Get(config.SpreadsheetID, writeRange).Do()
			// if err != nil {
			// 	log.Fatalf("Unable to retrieve data from sheet. %v", err)
			// }
		}

		var ur sheets.ValueRange
		ur.Values = append(ur.Values, []interface{}{"ID", "Name"})

		var wr sheets.ValueRange
		wr.Values = append(wr.Values, []interface{}{"TimeSpent - Seconds", "Day " + strconv.Itoa(time.Now().Day()) + " - Attendance"})

		var active string

		for idx := range *userList {
			ur.Values = append(ur.Values, []interface{}{(*userList)[idx].UserId, (*userList)[idx].Name})
			if (*userList)[idx].TotalDuration > 100 {
				active = "Present"
			} else {
				active = "Absent"
			}
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

		time.Sleep(10 * time.Second)

	}

	// spreadsheetID := "1s0fqIe1k5uRHz6TV9NflCZl0IDu78BJcS1_NWEjmgtQ"
	// readRange := "IntUnsecured62167!A2:C"
	// resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	// checkError(err)

	// if len(resp.Values) == 0 {
	// 	fmt.Println("No data found.")
	// } else {
	// 	fmt.Println("Name, Major:")
	// 	for _, row := range resp.Values {
	// 		fmt.Printf("%s, %s\n", row[0], row[1])
	// 	}
	// }

	// writeRange := "March 2022!A1"

	// var vr sheets.ValueRange

	// myval := []interface{}{"One", "Two", "Three"}
	// vr.Values = append(vr.Values, myval)

	// batchUpdate.Do()

	// _, err = srv.Spreadsheets.Values.Update(spreadsheetID, writeRange, )
	// srv.Spreadsheets.Create()
	// _, err = srv.Spreadsheets.Values.Update(spreadsheetID, writeRange, &vr).ValueInputOption("RAW").Do()
	// if err != nil {
	// 	log.Fatalf("Unable to retrieve data from sheet. %v", err)
	// }
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
