package apiqueue

import (
	"net/http"
	"time"
)

// RequestPriority описывает приоритет запроса
type RequestPriority int

const (
	Low RequestPriority = iota
	High
)

// RequestTask описывает задачу запроса к API
type RequestTask struct {
	Req      *http.Request
	Priority RequestPriority
	Response chan *http.Response
	Error    chan error
}

// ApiQueue — очередь с приоритетом
type ApiQueue struct {
	highTasks chan *RequestTask
	lowTasks  chan *RequestTask
	interval  time.Duration
}

// глобальная очередь
var Queue *ApiQueue

// InitPriorityQueue инициализирует глобальную очередь с приоритетами
func InitPriorityQueue(highSize, lowSize int, interval time.Duration) {
	if Queue == nil {
		Queue = &ApiQueue{
			highTasks: make(chan *RequestTask, highSize),
			lowTasks:  make(chan *RequestTask, lowSize),
			interval:  interval,
		}
		go Queue.startWorker()
	}
}

// startWorker выполняет задачи с приоритетом
func (q *ApiQueue) startWorker() {
	client := &http.Client{}
	ticker := time.NewTicker(q.interval)
	defer ticker.Stop()

	for {
		var task *RequestTask
		select {
		case task = <-q.highTasks: // сначала high priority
		default:
			select {
			case task = <-q.highTasks:
			case task = <-q.lowTasks:
			}
		}

		if task == nil {
			continue
		}

		<-ticker.C

		resp, err := client.Do(task.Req)
		if err != nil {
			task.Error <- err
		} else {
			task.Response <- resp
		}
		close(task.Response)
		close(task.Error)
	}
}

// Enqueue добавляет запрос в очередь с указанным приоритетом
func (q *ApiQueue) Enqueue(req *http.Request, priority RequestPriority) (*http.Response, error) {
	task := &RequestTask{
		Req:      req,
		Priority: priority,
		Response: make(chan *http.Response),
		Error:    make(chan error),
	}

	if priority == High {
		q.highTasks <- task
	} else {
		q.lowTasks <- task
	}

	resp := <-task.Response
	err := <-task.Error
	return resp, err
}

// Close закрывает очередь
func (q *ApiQueue) Close() {
	close(q.highTasks)
	close(q.lowTasks)
}
