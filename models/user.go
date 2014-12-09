package models

import . "../utils"

type User struct {
	ID   int `orm:"column(id);auto;pk"`
	Name string
}

func NewUser(username string) *User {
	user := User{Name: username}
	if _, id, err := db.ReadOrCreate(&user, "Name"); err == nil {
		user.ID = int(id)
		return &user
	} else {
		Logger.Info("Create User error: ", err)
		return nil
	}
}
