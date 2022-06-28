package utility

import (
	"log"

	"slack-user-attendence-app/data"

	"github.com/slack-go/slack"
)

func GetUserList(api *slack.Client) (*[]data.UserTimeData, error) {

	// defer func(){
	// 	if(recover()){
	// 		return nil, errors.New("fucked");
	// 	}
	// }()

	Users, err := api.GetUsers()
	if err != nil {
		log.Fatalln(err)
	}
	usertimelist := make([]data.UserTimeData, 0, 100)
	for _, user := range Users {
		if !user.IsBot && !user.IsAppUser && user.ID != "USLACKBOT" {
			temp := data.UserTimeData{
				UserId:        user.ID,
				PresenceState: data.Away,
				Name:          user.Name,
				TotalDuration: 0,
			}
			usertimelist = append(usertimelist, temp)
		}

	}
	log.Println(usertimelist)
	return &usertimelist, nil
}
