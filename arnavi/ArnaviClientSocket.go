package arnavi

import(
	"net"
	"encoding/binary"
	"time"
	"math"
	"strconv"
	"io"
	"sync"
	"encoding/hex"
	"fmt"
	
	"telsrv/app"
)

const (
	INIT_PACKAGE_LEN = 18
	DATA_PACKAGE_LEN = 512
	
	INIT_PACKAGE_PREF = 0xFF	
	HEADER_PROT1 = 0x22
	HEADER_PROT2 = 0x23
	HEADER_PROT3 = 0x24
	
	DATA_PACKAGE_PREF = 0x5B
	DATA_PACKAGE_POSTF = 0x5D
	DATA_PACKAGE_TYPE_ANSWER = 0xFD
	DATA_PACKAGE_MAX_PARCEL = 0xFB
	
	PACKET_PING = 0x00
	
	PACKET_TAGS = 0x01
	PACKET_TAGS_LEN = 5
	TAG_VAR_VOLT = 1
	TAG_VAR_ID = 2
	TAG_VAR_LAT = 3
	TAG_VAR_LON = 4
	TAG_VAR_ATTRS = 5 //speed (high), satellites, height, course (least significant byte)
	TAG_VAR_PIN = 6
	TAG_VAR_SIM1_ATTRS = 7 //local area code(2), cell ID (2)
	TAG_VAR_SIM1_ATTRS2 = 8
	TAG_VAR_DEVICE_STAT = 9
	
	PACKET_TEXT = 0x03
	PACKET_FILE = 0x04
	PACKET_BINARY = 0x06
	PACKET_CONFIRM = 0x08
	PACKET_CONFIRM_BY_TOKEN = 0x09
	
	SERV_RESP_PREF = 0x7B
	SERV_RESP_POSTF = 0x7D
	HEADER_PARCEL = 0x00
	
	SERV_CMD_PARCEL = 0xFF	
)


func Float32frombytes(bytes []byte) float32 {
    bits := binary.LittleEndian.Uint32(bytes)
    float := math.Float32frombits(bits)
    return float
}

func calcCheckSum(bf []byte) (sm byte) {
	for i:=0; i<len(bf); i++{
		sm += bf[i]; 	
	}
	return sm
}

type ArnaviClientSocket struct {
	IMEI string
	Conn net.Conn
	mx sync.RWMutex
	LastActivity time.Time
	//LastSysError time.Time
	//SysErrorCount int
	StartTime time.Time
	DownloadedBytes uint64
	UploadedBytes uint64
	Handshakes uint64	
	App *app.Application
}

func (sock *ArnaviClientSocket) SetConn(conn net.Conn) {
	sock.Conn = conn
}

func (sock *ArnaviClientSocket) SetApp(ap *app.Application) {
	sock.App = ap
}

func (sock *ArnaviClientSocket) SetStartTime() {
	sock.mx.Lock()
	sock.StartTime = time.Now()
	sock.mx.Unlock()
}

func (sock *ArnaviClientSocket) IncDownloadedBytes(bt uint64) {
	sock.mx.Lock()
	sock.DownloadedBytes =+ bt
	sock.mx.Unlock()
	//total bytes
	sock.App.IncDownloadedBytes(bt)
}

func (sock *ArnaviClientSocket) IncUploadedBytes(bt uint64) {
	sock.mx.Lock()
	sock.UploadedBytes =+ bt
	sock.mx.Unlock()
	//total bytes
	sock.App.IncUploadedBytes(bt)
}

func (sock *ArnaviClientSocket) IncHandshakes() {
	sock.mx.Lock()
	sock.Handshakes++
	sock.mx.Unlock()
	sock.App.IncHandshakes()
}

//direct command
func (sock *ArnaviClientSocket) Write(resp []byte) {
	sock.Conn.Write(resp)
}

func (sock *ArnaviClientSocket) GetRunTime() uint64 {
	sock.mx.Lock()
	dif := uint64(time.Now().Sub(sock.StartTime).Seconds())
	sock.mx.Unlock()
	
	return dif
}

func (sock *ArnaviClientSocket) GetDownloadedBytes() uint64 {
	sock.mx.Lock()
	bt := sock.DownloadedBytes
	sock.mx.Unlock()
	return bt
}

func (sock *ArnaviClientSocket) GetUploadedBytes() uint64 {
	sock.mx.Lock()
	bt := sock.UploadedBytes
	sock.mx.Unlock()
	return bt
}

func (sock *ArnaviClientSocket) GetHandshakes() uint64 {
	sock.mx.Lock()
	bt := sock.Handshakes
	sock.mx.Unlock()
	return bt
}

func (sock *ArnaviClientSocket) GetIMEI() string {
	sock.mx.Lock()
	imei := sock.IMEI
	sock.mx.Unlock()
	return imei
}

func (sock *ArnaviClientSocket) writeServResponse(payload []byte, parcelNum byte) error{
	//Prefix(1),Length(1),parcelNum(1),checkSum(1),payload(n),Potfix(1) 4 bytes with no payload	
	var payload_n byte
	var payload_inc byte;
	if payload != nil {
		payload_n = byte(len(payload))		
		payload_inc = 5 //check sum!
	}else{
		payload_inc = 4
	}
	resp := make([]byte, payload_n + payload_inc)
	resp[0] = SERV_RESP_PREF
	resp[1] = payload_n
	resp[2] = parcelNum
	
	if payload != nil {
		resp[3] = calcCheckSum(payload)
		var i byte
		for i = 0; i < payload_n; i++{
			resp[i + 4] = payload[i]
		}
	}
	
	resp[payload_n + payload_inc - 1] = SERV_RESP_POSTF
	_, err := sock.Conn.Write(resp)
	if err != nil {
		sock.App.Logger.Errorf("sock.Conn.Write %v", err)
	}
	
	sock.IncUploadedBytes(uint64(len(resp)))
	
	return err
}

func (sock *ArnaviClientSocket) WriteServCommand(payload []byte) error{	
	str := hex.EncodeToString(payload)
	sock.App.Logger.Debugf("ID:%s, server command:%s", sock.IMEI, str)
	return sock.writeServResponse(payload, SERV_CMD_PARCEL)
}

func (sock *ArnaviClientSocket) HandleConnection(connLiveSec int) {
	
	defer sock.Conn.Close()
	
	package_buf := make([]byte, DATA_PACKAGE_LEN)
	
	for {
		package_len, err := sock.Conn.Read(package_buf)			
		
		sock.LastActivity = time.Now()
		sock.Conn.SetReadDeadline(time.Now().Add( time.Duration(connLiveSec) * time.Second))
		
		switch err {
		case nil:			
			//connTimer.Reset(config.Srv.getEmptyConnTTL())
			
			sock.IncDownloadedBytes(uint64(package_len))
			
			sock.App.Logger.Debugf("ID:%s, Package %d bytes", sock.GetDescr(), package_len)
			
			//str := hex.EncodeToString(package_buf[:package_len])
			//sock.App.Logger.Debugf("Data=%s", sock.IMEI, str)
			
			if sock.App.IsSysPackage(package_buf, package_len, sock) {
				sock.App.Logger.Debug("ID=%s syspackage, skeeped", sock.IMEI)
				continue
			
			//init package 0-Pref, 1-Protocol, 2-9 IMEI
			}else if package_len <= INIT_PACKAGE_LEN && package_buf[0] == INIT_PACKAGE_PREF && (package_buf[1]==HEADER_PROT1 || package_buf[1]==HEADER_PROT2) {
				
				//IMEI/ID 8 bytes uint64
				sock.IMEI = strconv.FormatUint(binary.LittleEndian.Uint64(package_buf[2:10]), 10)
				sock.App.Logger.Debugf("ID:%s, Init package", sock.IMEI)
				
				if package_buf[1] == HEADER_PROT1 {
					//empty payload
					if sock.writeServResponse(nil, 0) != nil {
						return
					}
					
				}else if package_buf[1] == HEADER_PROT2 {
					//payload = unix timestamp
					payload := make([]byte, 4)
					binary.LittleEndian.PutUint32(payload, uint32(time.Now().UTC().Unix()))
					if sock.writeServResponse(payload, HEADER_PARCEL) != nil {
						return
					}
				}
				
				sock.IncHandshakes()
				
			}else if sock.IMEI != "" && package_buf[0] == DATA_PACKAGE_PREF && package_buf[1] == DATA_PACKAGE_TYPE_ANSWER {	
				//answer to server command
				answer_code := package_buf[2]
				sock.App.Logger.Debugf("Data package, answer to command, code, %d", answer_code)
				
			}else if sock.IMEI != "" && package_buf[0] == DATA_PACKAGE_PREF && package_buf[1] <= DATA_PACKAGE_MAX_PARCEL {
				//&& package_buf[package_len-1] == DATA_PACKAGE_POSTF{	
				//data package, requires confirmation				
				
				packets := package_buf[2:package_len]
				
				//sock.App.Logger.Debugf("ID:%s, DATA package", sock.IMEI)
				
				for len(packets) > 0 && packets[0] != DATA_PACKAGE_POSTF {
					
					//encodedString := hex.EncodeToString(packets)
					//sock.App.Logger.Debug("Packet data:")
					//sock.App.Logger.Debug(encodedString)				
					
					if packets[0] == PACKET_PING {
						_ = sock.writeServResponse(nil, 1)
						break
						
					}else if packets[0] == PACKET_TAGS && len(packets) > 3 {
						data_len := binary.LittleEndian.Uint16(packets[1:3])
						if int(data_len)+7 >= len(packets) {							
							//sock.LastSysError = time.Now()
							//sock.SysErrorCount++
							sock.App.Logger.Errorf("ID=%s: data_len+7 >= len(packets)  %d<>%d", sock.IMEI, data_len, len(packets))	
							//Здесь происходит залипание, нада как-то перезагружать когда трекер одно и то же шлёт
							//RESET: []byte{0x01,0x07}
							//sock.WriteServCommand([]byte{0x01,0x07})
							break
						}
						unix_time := binary.LittleEndian.Uint32(packets[3:7])
						packet_time := time.Unix(int64(unix_time), 0)
						data := packets[7:7+data_len]
						check_sum := packets[7+data_len]
						calc_check_sum := calcCheckSum(packets[3 : 7+data_len])
						if calc_check_sum != check_sum {
							sock.App.Logger.Errorf("ID=%s: Data package TAGS checksum error %d<>%d", sock.IMEI, calc_check_sum, check_sum)	
							break
						}
						sock.App.Logger.Debugf("ID=%s: Data package TAGS, data_len=%d, packet_time=%v", sock.IMEI, data_len, packet_time)
						
						//tag decode, total PACKET_TAGS_LEN bytes
						tel_data := app.TelematicsData{ID: sock.IMEI,
								GPSTime: packet_time,
								ReceivedTime: time.Now(),
								GPSValid: true,
							}
						tag_n := len(data) / PACKET_TAGS_LEN
						var num_ind int
						for tag_i :=0 ;tag_i < tag_n; tag_i++ {
							num_ind = tag_i * PACKET_TAGS_LEN
							tag_var_num := data[num_ind]
							tag_var_val := data[num_ind + 1 : (tag_i+1) * PACKET_TAGS_LEN]
							switch tag_var_num {
							case TAG_VAR_VOLT:
								tel_data.VoltExt = int16(binary.LittleEndian.Uint16(tag_var_val[0:2]))
								tel_data.VoltInt = int16(binary.LittleEndian.Uint16(tag_var_val[2:4]))
								
							case TAG_VAR_ID:
								
								
							case TAG_VAR_LAT:
								tel_data.Lat = Float32frombytes(tag_var_val)
								
							case TAG_VAR_LON:
								tel_data.Lon = Float32frombytes(tag_var_val)
								
							case TAG_VAR_ATTRS:
								tel_data.Speed = int(float32(tag_var_val[3]) * 1.852)
								tel_data.SattlliteNum =  tag_var_val[2]
								tel_data.Height = int(tag_var_val[1]) * 10
								tel_data.Heading = int(tag_var_val[0]) * 2
							
							//case TAG_VAR_SIM1_ATTRS:
								
							case TAG_VAR_SIM1_ATTRS2:	
								tel_data.SignalLevel = tag_var_val[0]
								
							//case TAG_VAR_DEVICE_STAT: Tabel 2
								
							}
						}
						
						lat_deg, lat_min, lat_min_dec := convertFloatToDegree(tel_data.Lat)
						tel_data.Lat_s = fmt.Sprintf("%d%02d.%d", lat_deg, lat_min, lat_min_dec)
						
						lon_deg, lon_min, lon_min_dec := convertFloatToDegree(tel_data.Lon)
						tel_data.Lon_s = fmt.Sprintf("%03d%02d.%d", lon_deg, lon_min, lon_min_dec)
						
						sock.App.Storage.Write(&tel_data)						
						sock.App.Logger.Debugf("ID=%s, packet decoded %v+",sock.IMEI, tel_data)
						
						//next packet
						new_from := 7+data_len+1
						if int(new_from) < len(packets) {
							packets = packets[new_from:]
						}else{
							//???
							sock.App.Logger.Errorf("ID=%s: int(new_from) < len(packets) %d<>%d", sock.IMEI, new_from, len(packets))	
							break
						}

					}else if packets[0] == PACKET_TEXT {
						data_len := binary.LittleEndian.Uint16(package_buf[1:3])
						unix_time := binary.LittleEndian.Uint32(packets[3:7])
						packet_time := time.Unix(int64(unix_time), 0)
						data := package_buf[7:7+data_len]
						//check_sum = package_buf[7+data_len]
						sock.App.Logger.Debugf("ID=%s: Data package TEXT, data_len=%d, packet_time=%v, data=%s", sock.IMEI, data_len,packet_time,data)
						break

					}else if packets[0] == PACKET_FILE {
						data_len := binary.LittleEndian.Uint16(package_buf[1:3])
						unix_time := binary.LittleEndian.Uint32(packets[3:7])
						packet_time := time.Unix(int64(unix_time), 0)
						//file_offset := package_buf[7:11]				
						//data := package_buf[11:11+data_len]
						//check_sum = package_buf[11+data_len]
						sock.App.Logger.Debugf("ID=%s: Data package File, data_len=%d, packet_time=%v",sock.IMEI, data_len, packet_time)
						break
									
					}else if packets[0] == PACKET_BINARY {
						data_len := binary.LittleEndian.Uint16(package_buf[1:3])
						unix_time := binary.LittleEndian.Uint32(packets[3:7])
						packet_time := time.Unix(int64(unix_time), 0)
						//data := package_buf[7:7+data_len]
						//check_sum = package_buf[7+data_len]
						sock.App.Logger.Debugf("ID=%s: Data package BINARY, data_len=%d, packet_time=%v", sock.IMEI, data_len, packet_time)

					}else if packets[0] == PACKET_CONFIRM {
						data_len := binary.LittleEndian.Uint16(package_buf[1:3])
						unix_time := binary.LittleEndian.Uint32(packets[3:7])
						packet_time := time.Unix(int64(unix_time), 0)
						error_code := package_buf[7:8]
						//check_sum = package_buf[7+data_len]
						sock.App.Logger.Debugf("ID=%s: Data package confirm, data_len=%d, packet_time=%v, error_code=%d",sock.IMEI, data_len, packet_time, error_code)
						break
					
					}else if packets[0] == PACKET_CONFIRM_BY_TOKEN {
						sock.App.Logger.Debugf("ID=%s: Data package PACKET_CONFIRM_BY_TOKEN", sock.IMEI)
						break
						
					}else{						
						str := hex.EncodeToString(package_buf[:package_len])
						sock.App.Logger.Debugf("ID=%s: Data package unknown, Data=%s", sock.IMEI, str)
						break
					}
				}
				
				//confirmation
				if sock.writeServResponse(nil, 1) == nil {
					sock.App.Logger.Debugf("ID:%s: package confirmed", sock.IMEI)
				}
				//panic("Stopped!")				
				
			}else if sock.IMEI != "" {	
				//IMEI есть, данные непонятные
				sock.App.Logger.Debugf("ID:%s package_buf[0]=%d, package_buf[1]=%d, package_buf[package_len-1]=%d", sock.IMEI, package_buf[0], package_buf[1], package_buf[package_len-1])
			}
			
		case io.EOF:
			sock.App.Logger.Warnf("%s: Closed on timeout", sock.GetDescr())
			return
			
		default:
			sock.App.Logger.Warnf("%s, conn.Read: %v", sock.GetDescr(), err)
			return
		}
			
	}
}

func (sock *ArnaviClientSocket) GetDescr() string{
	var descr string
	if sock.IMEI != "" {
		descr = sock.IMEI
	}else{
		descr = sock.Conn.RemoteAddr().String()
	}
	return descr
}

//returns degree, minutes, minute decimzl part
func convertFloatToDegree(coord float32) (int, int, int) {	
	if coord == 0.0 {
		return 0,0,0
	}
	min, min_dec := math.Modf(float64(coord) * 60.0)
	min_dec_s := fmt.Sprintf("%.5f", min_dec)	
	if len(min_dec_s) >= 3 {
		//0. - skeep
		if i, err := strconv.Atoi(min_dec_s[2:]); err == nil {
			return int(coord), int(min), i
		}		
	}
	return 0,0,0
}
