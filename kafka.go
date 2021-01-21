package kafka

import (
    "fmt"
	"context"
	"golang.org/x/time/rate"
	"time"
)

func (k *Kafka) Push( v interface{} ) {
	if k.close {
		time.Sleep(time.Second)
	}

	var data []byte
	switch msg := v.(type) {
	case string:
		data = []byte(msg)
	case []byte:
		data = msg
	case Message: //消耗挺大 不建议
		data = msg.Byte()
	default:
        data = []byte(fmt.Sprintf("%v" , msg))
	}

	k.buffer <- data
}

// 开始传输
func (k *Kafka) Start() error {

	k.thread = make([]Thread, k.C.thread)
	k.buffer = make(chan []byte , k.C.buffer)
	k.ctx , k.cancel = context.WithCancel(context.Background())
	k.limiter = &Limiter{}

	//设置限速
	if k.C.limit > 0 {
		k.limiter.limit = rate.NewLimiter(rate.Limit(k.C.limit) , k.C.limit * 2)
		k.limiter.ctx , _ = context.WithCancel(context.TODO())
	} else {
		k.limiter.limit = nil
	}

	//创建并启动程序
	for i := 0; i < k.C.thread; i++ {

		k.thread[i] = NewThread(i , k)
		k.thread[i].start() //启动线程
	}

	go k.Heartbeat()

	return nil
}

// 线程状态检测
func (k *Kafka) Status() bool {
	inactive := 0

	for _ , v := range k.thread {
		if v.status != OK {
			inactive++
		}
	}

	if inactive == k.C.thread {
		return false
	}

	return  true
}

func (k *Kafka) Ping() {

	for id, t := range k.thread {
		switch t.status {
		case OK:
			//Out.Info("%s kafka thread.id = %d up" , t.C.name , id)
			continue

		case CLOSE:
			Out.Info("%s kafka thread.id = %d close" , t.C.name , id)
			//Out.Err("%s kafka threads check: topic [%s], %d up, %d down", k.C.name , k.C.topic, k.count, k.C.thread-k.count)
		case ERROR:
			k.thread[id].start()
			//Out.Err("%s kafka thread.id = %d start" , k.C.name , id)
		}
	}

}

// 心跳检测
func (k *Kafka) Heartbeat() {
	tk := time.NewTicker( time.Second * time.Duration(k.C.heartbeat))
	defer tk.Stop()

	for {
		select {
		case <-k.ctx.Done():
			Out.Err("%s kafka heartbeat exit", k.C.name)
			return
		case <-tk.C:
			k.Ping()
		}
	}
}

// 关闭连接
func (k *Kafka) Close() {
	k.cancel()
	k.close = true
	time.Sleep(500 * time.Millisecond)
}

func (k *Kafka) Reload() {
	Out.Err("%s kafka reload to close ..." , k.C.name)
	k.Close()
	Out.Err("%s kafka reload  close end" , k.C.name)

	if err := k.Start() ; err != nil {
		Out.Err("%s kafka reload , start err: %v" , k.C.name , err)
	}
}
