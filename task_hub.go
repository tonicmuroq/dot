package main

import (
	"log"
	"sync"
	"time"
)

type TaskHub struct {
	queue chan *[]byte
	size  int
	wg    *sync.WaitGroup
}

var taskhub *TaskHub

func (self *TaskHub) GetTask() *[]byte {
	return <-self.queue
}

func (self *TaskHub) AddTask(task *[]byte) {
	self.queue <- task
	log.Println(len(self.queue))
	if len(self.queue) >= self.size {
		log.Println("full, do dispatch")
		self.Dispatch()
	}
}

func (self *TaskHub) Run() {
	for {
		count := len(self.queue)
		if count > 0 {
			log.Println("period check")
			self.Dispatch()
			log.Println("period check done")
		} else {
			log.Println("empty")
		}
		time.Sleep(30 * time.Second)
	}
}

func (self *TaskHub) Dispatch() {
	count := len(self.queue)
	for i := 0; i < count; i = i + 1 {
		d := self.GetTask()
		log.Println(string(*d))
	}
}

func init() {
	// TODO size shall be in arguments
	taskhub = &TaskHub{queue: make(chan *[]byte, 5), size: 5, wg: &sync.WaitGroup{}}
}
