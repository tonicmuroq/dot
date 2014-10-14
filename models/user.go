package models

type User struct {
	Id   int
	Name string
}

func (self *User) TableUnique() [][]string {
	return [][]string{
		[]string{"Name"},
	}
}
