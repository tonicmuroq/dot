package utils

import (
	"log"
)

var Logger *DotLogger = &DotLogger{}

type DotLogger struct {
	Mode bool
}

func (self *DotLogger) Info(v ...interface{}) {
	log.Println(v...)
}

func (self *DotLogger) Debug(v ...interface{}) {
	if self.Mode {
		log.Println(v...)
	}
}

func (self *DotLogger) Assert(err error, context string) {
	if err != nil {
		log.Fatal(context+": ", err)
	}
}
