package types

import . "utils"

type User struct {
	ID   int `orm:"column(id);auto;pk"`
	Name string
}

func NewUser(username string) *User {
	user := User{Name: username}
	_, id, err := db.ReadOrCreate(&user, "Name")
	if err != nil {
		Logger.Info("Create User error: ", err)
		return nil
	}
	user.ID = int(id)
	return &user
}
