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
				EmailID:       user.Profile.Email,
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

func GetColumnName(col int) string {
	name := make([]byte, 0, 3) // max 16,384 columns (2022)
	const aLen = 'Z' - 'A' + 1 // alphabet length
	for ; col > 0; col /= aLen + 1 {
		name = append(name, byte('A'+(col-1)%aLen))
	}
	for i, j := 0, len(name)-1; i < j; i, j = i+1, j-1 {
		name[i], name[j] = name[j], name[i]
	}
	return string(name)
}
