package utility

import (
	"log"

	"slack-user-attendence-app/data"

	"github.com/slack-go/slack"
)

func GetUserMap(api *slack.Client) (*map[string]data.UserTimeData, error) {

	// defer func(){
	// 	if(recover()){
	// 		return nil, errors.New("fucked");
	// 	}
	// }()

	Users, err := api.GetUsers()
	if err != nil {
		log.Fatalln(err)
	}
	// usertimelist := make([]data.UserTimeData, 0, 100)
	usermapData := make(map[string]data.UserTimeData)
	for _, user := range Users {
		if !user.IsBot && !user.IsAppUser && user.ID != "USLACKBOT" {
			temp := data.UserTimeData{
				UserId:        user.ID,
				PresenceState: data.Away,
				Name:          user.Name,
				TotalDuration: 0,
			}
			// usertimelist = append(usertimelist, temp)
			usermapData[user.ID] = temp
		}

	}
	log.Println(usermapData)
	return &usermapData, nil
}

func GenerateListFromMap(usermap *map[string]data.UserTimeData) *[]data.UserTimeData {
	userList := make([]data.UserTimeData, 0, 100)

	for key := range *usermap {
		userList = append(userList, (*usermap)[key])
	}
	return &userList
}
