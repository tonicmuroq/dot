package main

import (
	"log"
	"time"
)

type TaskQueue struct {
	queue chan *[]byte
	size  int
}

var taskqueue *TaskQueue

func (self *TaskQueue) GetTask() *[]byte {
	return <-self.queue
}

func (self *TaskQueue) AddTask(task *[]byte) {
	self.queue <- task
	log.Println(len(self.queue))
	if len(self.queue) >= self.size {
		// do deploy
		log.Println("full, do deploy")
		self.DoDeploy()
	}
}

func (self *TaskQueue) Run() {
	for {
		count := len(self.queue)
		if count > 0 {
			// do deploy
			log.Println("period check")
			self.DoDeploy()
			log.Println("period check done")
		} else {
			log.Println("empty")
		}
		time.Sleep(30 * time.Second)
	}
}

func (self *TaskQueue) DoDeploy() {
	count := len(self.queue)
	for i := 0; i < count; i = i + 1 {
		d := self.GetTask()
		log.Println(string(*d))
	}
}

func init() {
	// TODO size shall be in arguments
	taskqueue = &TaskQueue{queue: make(chan *[]byte, 5), size: 5}
}
