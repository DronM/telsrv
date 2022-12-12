package main

import (
	"os"
	"fmt"
	"net"
	"time"
	"context"
	"net/textproto"
	"bufio"
)

const (
	PAR_HOST = 1
	PAR_KEY = 2
	PAR_CMD = 3
	PAR_IMEI = 4
	
	PREF_LEN = 3
)

type Command struct {
	NeedIMEI bool
	Seq []byte
	Direct byte
}
type CommandList map[string]Command

//./client 192.168.1.3:52053 eg419rh4t14mn4s54tgr7g1 clientCount
//./client 192.168.1.3:52053 eg419rh4t14mn4s54tgr7g1 transmitCoords 888888888888888
//./client 192.168.1.77:52053 eg419rh4t14mn4s54tgr7g1 status

func main() {

	//1) obligatory argument IP:port
	if len(os.Args)<PAR_HOST+1 {
		panic("IP:port is missing")
	}

	//2) obligatory argument key
	if len(os.Args)<PAR_KEY+1 {
		panic("key is missing")
	}
	
	//3) command argument
	if len(os.Args)<PAR_CMD+1 {
		panic("Command param is missing")
	}

	//all commands
	commands := make(CommandList)
	commands["clientCount"] = Command{NeedIMEI: false, Seq: []byte{0x01}}
	commands["runTime"] = Command{NeedIMEI: false, Seq: []byte{0x02}}
	commands["clientMaxCount"] = Command{NeedIMEI: false, Seq: []byte{0x03}}
	commands["downloadedBytes"] = Command{NeedIMEI: false, Seq: []byte{0x04}}
	commands["uploadedBytes"] = Command{NeedIMEI: false, Seq: []byte{0x05}}
	commands["list"] = Command{NeedIMEI: false, Seq: []byte{0x06}}
	commands["handshakes"] = Command{NeedIMEI: false, Seq: []byte{0x07}}
	commands["status"] = Command{NeedIMEI: false, Seq: []byte{0xFF}}
	
	//specific, arnavi
	commands["transmitCoords"] = Command{NeedIMEI: true, Seq: []byte{0x01,0x01}, Direct:1}
	commands["updateSoftwareForce"] = Command{NeedIMEI: true, Seq: []byte{0x01,0x04}, Direct:1}
	commands["updateSoftware"] = Command{NeedIMEI: true, Seq: []byte{0x01,0x05}, Direct:1}
	commands["reset"] = Command{NeedIMEI: true, Seq: []byte{0x01,0x07}, Direct:1}
	commands["downloadSettingsFromWebConf"] = Command{NeedIMEI: true, Seq: []byte{0x01,0x08}, Direct:1}
	commands["sendSettingsToWebConf"] = Command{NeedIMEI: true, Seq: []byte{0x01,0x09}, Direct:1}
	
	//Specific, status
	commands["imeiRunTime"] = Command{NeedIMEI: true, Seq: []byte{0x82}, Direct:0}
	commands["imeiDownloadedBytes"] = Command{NeedIMEI: true, Seq: []byte{0x84}, Direct:0}
	commands["imeiUploadedBytes"] = Command{NeedIMEI: true, Seq: []byte{0x85}, Direct:0}
	commands["imeiHandshakes"] = Command{NeedIMEI: false, Seq: []byte{0x87}}
	commands["imeiStatus"] = Command{NeedIMEI: true, Seq: []byte{0xFE}, Direct:0}
	
	cmd_found := false
	var cur_cmd *Command
	for nm, cmd := range commands {
		if nm == os.Args[PAR_CMD] {
			cmd_found = true
			cur_cmd = &cmd
			break
		}
	}
	if !cmd_found {
		panic("Command not found!")
	}
	
	var imei string
	if cur_cmd.NeedIMEI && len(os.Args)<PAR_IMEI+1 {
		panic("Command needs IMEI")
		
	}else if cur_cmd.NeedIMEI {
		imei = os.Args[PAR_IMEI]
	}
	
	var d net.Dialer
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	
	conn, err := d.DialContext(ctx, "tcp", os.Args[PAR_HOST])
	if err != nil {
		panic(fmt.Sprintf("Failed to dial: %v", err))
	}
	defer conn.Close()

	buf := make([]byte, 3)
	if imei != "" {
		buf[0] = 0xFF
		buf[1] = 0xFF
		buf[2] = 0xFF 
	}else{
		buf[0] = 0xFE
		buf[1] = 0xFE
		buf[2] = 0xFE
	}
	buf = append(buf, []byte(os.Args[PAR_KEY])...)
	if imei != "" {
		buf = append(buf, byte(len(imei)))
		buf = append(buf, []byte(imei)...)
	}
	buf = append(buf, byte(len(cur_cmd.Seq)))
	buf = append(buf, cur_cmd.Seq...)
	buf = append(buf, cur_cmd.Direct)
	buf = append(buf, 0x0A)
	
	fmt.Printf("Sending command %s\n",os.Args[PAR_CMD])	
	if _, err := conn.Write(buf); err != nil {
		panic(err)
	}
	
	reader := bufio.NewReader(conn)
	tp := textproto.NewReader(reader)
	
	line, err := tp.ReadLine()
	if err != nil {
		panic(err)
	}
	fmt.Println(line)
}
