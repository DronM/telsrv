package app

import(
	"net"
	"time"
	"fmt"
	"crypto/rand"
	"sync"
	"encoding/hex"
	
	"github.com/labstack/gommon/log"
)

const (
	SYS_PKG_PREF_LEN = 3
)

type NewSocketFunc = func() ClientSocketer

type TelematicsData struct {
	ID string
	GPSTime time.Time
	ReceivedTime time.Time
	Lon float32
	Lon_s string
	Lat float32
	Lat_s string
	Speed int
	Heading int
	SattlliteNum byte
	Height int
	VoltExt int16
	VoltInt int16
	SignalLevel byte
	Odom uint32
	FromMemory bool
	GPSValid bool
	//MobileCountryCode
	//MobileNeworkCode byte //01 -MTS, 2- Begafon, 07- Smarts, 99 -Beeline
}

//Interface for storages
type Storager interface {
	Init(string, *log.Logger, int) error
	Write(*TelematicsData)
	GetDescr() string
}

//

type Application struct {
	CommandKey string
	Logger *log.Logger
	Storage Storager
	ClientSockets *ClientSocketList
	SysPackageMinLen int
	StartTime time.Time
	mx sync.RWMutex
	MaxClientCount int
	DownloadedBytes uint64
	UploadedBytes uint64
	Handshakes uint64	
}

//, store Connector
func (a *Application) RunServer(ID string, host string, port int, connLiveSec int, newSocket NewSocketFunc) {

	srv_addr := fmt.Sprintf("%s:%d",host, port)

	l, err := net.Listen("tcp", srv_addr)
	if err != nil {
		a.Logger.Fatalf("net.Listen: %v", err)
	}
	defer l.Close()

	a.ClientSockets = &ClientSocketList{m: make(map[string]ClientSocketer)}	
	a.SysPackageMinLen = 3 + len(a.CommandKey) + 1 //see IsSysPackage func for package structure
	a.StartTime = time.Now()
	
	a.Logger.Infof("%s TCP server started: %s", ID, srv_addr)
	for {
		conn, err := l.Accept()
		if err != nil {
			a.Logger.Errorf("l.Accept: %v", err)
		} else {						
			go a.HandleConnection(conn, newSocket, connLiveSec)
		}
	}
}

func (a *Application) HandleConnection(conn net.Conn, newSocket NewSocketFunc, connLiveSec int) {
	id, err := genID()
	if err != nil {
		a.Logger.Errorf("genID: %v", err)
		return
	}
	socket := newSocket()
	socket.SetConn(conn)
	socket.SetApp(a)	
	cnt := a.ClientSockets.Append(socket, id)
	a.mx.Lock()
	if cnt > a.MaxClientCount {
		a.MaxClientCount = cnt
	}	
	a.mx.Unlock()
	socket.HandleConnection(connLiveSec)
	a.ClientSockets.Remove(id)
}

/**
 * Sys package
 * prefix 3 bytes specific device command 0xFF 0xFF 0xFF OR all devices command 0xEF 0xEF 0xEF
 * a.CommandKey
 *
 *	Server command
 *OR
 * 	IMEI string - 15 bytes
 * 	Command bytes
 */
func (a *Application) IsSysPackage(buffer []byte, packageLen int, senderSocket ClientSocketer) bool{	
	
	if packageLen >= a.SysPackageMinLen && buffer[0] == 0xFF && buffer[1] == 0xFF && buffer[2] == 0xFF &&
	a.CommandKey == string(buffer[SYS_PKG_PREF_LEN : SYS_PKG_PREF_LEN+len(a.CommandKey)]){
		//specific device command
		
		imei_ind := SYS_PKG_PREF_LEN + byte(len(a.CommandKey))
		imei_len := buffer[imei_ind : imei_ind + 2][0]
		imei := string(buffer[imei_ind+1 : imei_ind + 1 + imei_len])
		
		cmd_len := buffer[imei_ind+1+imei_len : imei_ind+1+imei_len+2][0]
		cmd := buffer[imei_ind+1+imei_len+1 : imei_ind+1+imei_len+1 + cmd_len]
		direct := buffer[imei_ind+1+imei_len+1 + cmd_len : imei_ind+1+imei_len+1 + cmd_len+1][0]
		
		socket := a.ClientSockets.GetByIMEI(imei)
		if socket != nil && direct == 1 {			
			//direct device command
			socket.WriteServCommand(cmd)
			senderSocket.Write([]byte("OK"+"\n"))

		}else if socket != nil && direct != 1 {			
			//indirct device command
			resp:= a.SrvCMDRunServerCommand(cmd[0], imei, senderSocket)
			senderSocket.Write([]byte(resp+"\n"))
			
		}else{
			cmd_s := hex.EncodeToString(cmd)
			err_s := fmt.Sprintf("IMEI %s, not connected, command=%s", imei, cmd_s)
			a.Logger.Error(err_s)
			resp := a.SrvCMDResponse(err_s,"")
			senderSocket.Write([]byte(resp+"\n"))
		}
		
		return true

	}else if packageLen >= a.SysPackageMinLen && buffer[0] == 0xFE && buffer[1] == 0xFE && buffer[2] == 0xFE &&
	a.CommandKey == string(buffer[SYS_PKG_PREF_LEN : SYS_PKG_PREF_LEN+len(a.CommandKey)]){
		//Command for all devices
		cmd_ind := SYS_PKG_PREF_LEN + byte(len(a.CommandKey))		
		cmd_len := buffer[cmd_ind : cmd_ind+2][0]
		cmd := buffer[cmd_ind+1 : cmd_ind+1+cmd_len]		
		resp:= a.SrvCMDRunServerCommand(cmd[0], "", nil)
		senderSocket.Write([]byte(resp+"\n"))
		return true
	}
	
	return false
}

func (a *Application) IncDownloadedBytes(bt uint64){
	a.mx.Lock()
	a.DownloadedBytes += bt
	a.mx.Unlock()
}

func (a *Application) IncUploadedBytes(bt uint64){
	a.mx.Lock()
	a.UploadedBytes += bt
	a.mx.Unlock()
}

func (a *Application) IncHandshakes(){
	a.mx.Lock()
	a.Handshakes ++
	a.mx.Unlock()
}

func (a *Application) GetStartTime() time.Time{
	a.mx.Lock()
	tm := a.StartTime
	a.mx.Unlock()
	return tm
}

func (a *Application) GetMaxClientCount() int{
	a.mx.Lock()
	cnt := a.MaxClientCount
	a.mx.Unlock()
	return cnt
}

func (a *Application) GetDownloadedBytes() uint64{
	a.mx.Lock()
	bt := a.DownloadedBytes
	a.mx.Unlock()
	return bt
}

func (a *Application) GetUploadedBytes() uint64{
	a.mx.Lock()
	bt := a.UploadedBytes
	a.mx.Unlock()
	return bt
}

func (a *Application) GetHandshakes() uint64{
	a.mx.Lock()
	bt := a.Handshakes
	a.mx.Unlock()
	return bt
}

func (a *Application) GetDeviceList() []string {
	var list []string
	for it := range a.ClientSockets.Iter() {
		list = append(list, it.Socket.GetIMEI())
	}
	return list
}

// generates unique ID
func genID() (string,error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
	    return "",err
	}
	uuid := fmt.Sprintf("%x-%x-%x-%x-%x",b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
	return uuid,nil
}

