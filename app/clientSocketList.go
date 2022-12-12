package app

import(
	"sync"
	"net"
)

//Interface for client sockets
type ClientSocketer interface {
	HandleConnection(int)
	WriteServCommand(payload []byte) error
	SetStartTime()
	Write([]byte)
	GetRunTime() uint64
	GetDownloadedBytes() uint64
	GetUploadedBytes() uint64
	GetHandshakes() uint64
	GetIMEI() string
	SetConn(net.Conn)
	SetApp(*Application)
}

type ClientSocketItem struct {
	ID string
	Socket ClientSocketer
}

//Structure for managing client sockets
type ClientSocketList struct {
	mx sync.RWMutex
	m map[string]ClientSocketer //client connections		
}

func (l *ClientSocketList) Append(socket ClientSocketer, id string) int{
	l.mx.Lock()
	defer l.mx.Unlock()
	
	l.m[id] = socket
	socket.SetStartTime()
	return len(l.m)	
}
func (l *ClientSocketList) Remove(id string){
	l.mx.Lock()
	if _,ok := l.m[id]; ok {
		delete(l.m,id) 
	}
	l.mx.Unlock()
}

func (l *ClientSocketList) Get(id string) ClientSocketer {
	l.mx.Lock()
	defer l.mx.Unlock()
	
	if sock,ok := l.m[id]; ok {
		return sock
	}
	return nil
}

func (l *ClientSocketList) GetByIMEI(imei string) ClientSocketer {
	l.mx.Lock()
	defer l.mx.Unlock()
	
	for _, sock := range l.m {
		if sock.GetIMEI() == imei {
			return sock
		}
	}
	return nil
}


func (l ClientSocketList) Len() int{
	l.mx.Lock()
	defer l.mx.Unlock()
	return len(l.m)
}

// Iterates over the events in the concurrent slice
func (l *ClientSocketList) Iter() <-chan ClientSocketItem {
	c := make(chan ClientSocketItem)

	f := func() {
		l.mx.Lock()
		defer l.mx.Unlock()
		for k, v := range l.m {
			c <- ClientSocketItem{k, v}
		}
		close(c)
	}
	go f()

	return c
}

