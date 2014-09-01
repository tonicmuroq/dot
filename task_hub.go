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
	mutex *sync.Mutex
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

func (self *TaskHub) FinishOneTask() {
	self.wg.Done()
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
	self.mutex.Lock()
	count := len(self.queue)
	for i := 0; i < count; i = i + 1 {
		d := self.GetTask()
		self.wg.Add(1)
		log.Println(string(*d))
	}
	self.wg.Wait()
	self.mutex.Unlock()
	log.Println("finish, restart nginx")
}

func init() {
	// TODO size shall be in arguments
	taskhub = &TaskHub{queue: make(chan *[]byte, 5), size: 5, wg: &sync.WaitGroup{}, mutex: &sync.Mutex{}}
}
