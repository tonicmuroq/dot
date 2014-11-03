package models

import . "../utils"

type User struct {
	Id   int
	Name string
}

func (self *User) TableUnique() [][]string {
	return [][]string{
		[]string{"Name"},
	}
}

func NewUser(username string) *User {
	user := User{Name: username}
	if _, id, err := db.ReadOrCreate(&user, "Name"); err == nil {
		user.Id = int(id)
		return &user
	} else {
		Logger.Info("Create User error: ", err)
		return nil
	}
}
