package main

import (
	"container/list"
	"time"
)

type Levi struct {
	conn   *Connection
	host   string
	size   int
	queue  *list.List
	finish bool
}

func NewLevi(conn *Connection, size int) *Levi {
	return &Levi{conn, conn.host, size, list.New(), false}
}

func (self *Levi) AddTask(task *Task) {
	self.queue.PushBack(task)
	logger.Debug(self.queue.Len())
	if self.queue.Len() >= self.size {
		logger.Debug("full check")
		self.SendTasks()
	}
}

func (self *Levi) SendTasks() {
}

func (self *Levi) Close() {
	self.conn.CloseConnection()
	self.finish = true
}

func (self *Levi) Run() {
	go func() {
		for !self.finish {
			if self.queue.Len() > 0 {
				logger.Debug("period check")
				self.SendTasks()
				logger.Debug("period check done")
			} else {
				logger.Debug("empty queue")
			}
			time.Sleep(time.Duration(config.Task.Dispatch) * time.Second)
		}
	}()
}
