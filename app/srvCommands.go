package app

import(
	"fmt"	
	"time"
)

const (
	CMD_CLIENT_CNT byte = 0x01
	CMD_RUN_TIME byte = 0x02
	CMD_CLIENT_MAX_CNT byte = 0x03
	CMD_DOWNLOADED_BYTES byte = 0x04
	CMD_UPLOADED_BYTES byte = 0x05
	CMD_LIST byte = 0x06
	CMD_HANDSHAKES byte = 0x07
	CMD_STATUS byte = 0xFF
	
	CMD_DEV_RUN_TIME byte = 0x82
	CMD_DEV_DOWNLOADED_BYTES byte = 0x84	
	CMD_DEV_UPLOADED_BYTES byte = 0x85
	CMD_DEV_HANDSHAKES byte = 0x87
	CMD_DEV_STATUS byte = 0xFE
)

func (app *Application) SrvCMDError(errStr string) string {
	return app.SrvCMDResponse(errStr, "")
}

func (app *Application) SrvCMDResponse(errStr string, objects string) string {
	var objects_s string
	if objects != "" {
		objects_s = ","+objects
	}
	return fmt.Sprintf(`{"err":"%s"%s}`, errStr, objects_s)
}	

func (app *Application) SrvCMDClientCount() string {
	return fmt.Sprintf(`"clientCount":%d`, (app.ClientSockets.Len() - 1))
}

func (app *Application) SrvCMDRunTime() string {
	diff := uint64(time.Now().Sub(app.GetStartTime()).Seconds())
	return fmt.Sprintf(`"runTime":%d`, diff)
}

func (app *Application) SrvCMDClientMaxCount() string {
	return fmt.Sprintf(`"maxClientCount":%d`, app.GetMaxClientCount())
}

func (app *Application) SrvCMDDownloadedBytes() string {
	return fmt.Sprintf(`"downloadedBytes":%d`, app.GetDownloadedBytes())
}

func (app *Application) SrvCMDUploadedBytes() string {
	return fmt.Sprintf(`"uploadedBytes":%d`, app.GetUploadedBytes())
}

func (app *Application) SrvCMDHandshakes() string {
	return fmt.Sprintf(`"handshakes":%d`, app.GetHandshakes())
}

//returns json string
func (app *Application) SrvCMDRunServerCommand(cmd byte, imei string, sock ClientSocketer) string {
	switch cmd {
		case CMD_CLIENT_CNT:
			return app.SrvCMDResponse("",app.SrvCMDClientCount())
			
		case CMD_RUN_TIME:
			return app.SrvCMDResponse("",app.SrvCMDRunTime())
		
		case CMD_CLIENT_MAX_CNT:
			return app.SrvCMDResponse("",app.SrvCMDClientMaxCount())

		case CMD_DOWNLOADED_BYTES:
			return app.SrvCMDResponse("",app.SrvCMDDownloadedBytes())

		case CMD_UPLOADED_BYTES:
			return app.SrvCMDResponse("",app.SrvCMDUploadedBytes())

		case CMD_HANDSHAKES:
			return app.SrvCMDResponse("",app.SrvCMDHandshakes())
			
		case CMD_LIST:
			list_s := ""
			list := app.GetDeviceList()
			ind := 0
			for _,imei := range list{
				if ind > 0 {
					list_s += ","
				}
				list_s += fmt.Sprintf(`"%s"`,imei)
				ind++
			}
			return app.SrvCMDResponse("",fmt.Sprintf(`"list":[%s]`, list_s))

		case CMD_STATUS:
			status := fmt.Sprintf(`{"status":{%s,%s,%s,%s,%s,%s}}`, app.SrvCMDClientCount(),
				app.SrvCMDRunTime(), app.SrvCMDClientMaxCount(), app.SrvCMDDownloadedBytes(), app.SrvCMDUploadedBytes(),
				app.SrvCMDHandshakes())
			return app.SrvCMDResponse("", status)
		
		case CMD_DEV_RUN_TIME:
			return app.SrvCMDResponse("", fmt.Sprintf(`"imei":"%s","runTime":%d`,imei,sock.GetRunTime()))

		case CMD_DEV_DOWNLOADED_BYTES:
			return app.SrvCMDResponse("", fmt.Sprintf(`"imei":"%s","downloadedBytes":%d`,imei,sock.GetDownloadedBytes()))

		case CMD_DEV_UPLOADED_BYTES:
			return app.SrvCMDResponse("", fmt.Sprintf(`"imei":"%s","uploadedBytes":%d`,imei,sock.GetUploadedBytes()))
		
		case CMD_DEV_HANDSHAKES:
			return app.SrvCMDResponse("", fmt.Sprintf(`"imei":"%s","handshakes":%d`,imei,sock.GetHandshakes()))
			
		case CMD_DEV_STATUS:
			return app.SrvCMDResponse("", fmt.Sprintf(`"imei":"%s",status:{"runTime":%d,"downloadedBytes":%d,"uploadedBytes":%d}`,
				imei, sock.GetRunTime(), sock.GetDownloadedBytes(), sock.GetUploadedBytes()))
		
		
		default:
			t := fmt.Sprintf("Server command not found %d", cmd)
			app.Logger.Error(t)
			return app.SrvCMDResponse(t, "")
	}
}
