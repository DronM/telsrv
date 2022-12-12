package reportsyst

import(
	"net"
	"time"
	"math"
	"fmt"
	"strconv"
	"io"
	"sync"
	"encoding/hex"
	
	"telsrv/app"
)

const(
	DATA_PACKAGE_LEN = 59
	YEAR_START = 2000
	
	BACK_REPORT_CADR = 6
	
	COORD_STATUS_NE	= 5
	COORD_STATUS_SE = 6
	COORD_STATUS_NW	= 7
	COORD_STATUS_SW	= 8

	DIR_N = "n"
	DIR_S = "s"
	DIR_E = "e"
	DIR_W = "w"
	
	VALID_LAT_LEN = 9
	VALID_LON_LEN = 10
		
)

type ReportSysClientSocket struct {
	IMEI string
	Conn net.Conn
	mx sync.RWMutex
	LastActivity time.Time
	StartTime time.Time
	DownloadedBytes uint64
	UploadedBytes uint64
	Handshakes uint64	
	App *app.Application
}

func (sock *ReportSysClientSocket) SetConn(conn net.Conn) {
	sock.Conn = conn
}

func (sock *ReportSysClientSocket) SetApp(ap *app.Application) {
	sock.App = ap
}

func (sock *ReportSysClientSocket) SetStartTime() {
	sock.mx.Lock()
	sock.StartTime = time.Now()
	sock.mx.Unlock()
}

func (sock *ReportSysClientSocket) IncDownloadedBytes(bt uint64) {
	sock.mx.Lock()
	sock.DownloadedBytes =+ bt
	sock.mx.Unlock()
	//total bytes
	sock.App.IncDownloadedBytes(bt)
}

func (sock *ReportSysClientSocket) IncUploadedBytes(bt uint64) {
	sock.mx.Lock()
	sock.UploadedBytes =+ bt
	sock.mx.Unlock()
	//total bytes
	sock.App.IncUploadedBytes(bt)
}

func (sock *ReportSysClientSocket) IncHandshakes() {
	sock.mx.Lock()
	sock.Handshakes++
	sock.mx.Unlock()
	sock.App.IncHandshakes()
}

func (sock *ReportSysClientSocket) GetRunTime() uint64 {
	sock.mx.Lock()
	dif := uint64(time.Now().Sub(sock.StartTime).Seconds())
	sock.mx.Unlock()
	
	return dif
}

func (sock *ReportSysClientSocket) GetDownloadedBytes() uint64 {
	sock.mx.Lock()
	bt := sock.DownloadedBytes
	sock.mx.Unlock()
	return bt
}

func (sock *ReportSysClientSocket) GetUploadedBytes() uint64 {
	sock.mx.Lock()
	bt := sock.UploadedBytes
	sock.mx.Unlock()
	return bt
}

func (sock *ReportSysClientSocket) GetHandshakes() uint64 {
	sock.mx.Lock()
	bt := sock.Handshakes
	sock.mx.Unlock()
	return bt
}

func (sock *ReportSysClientSocket) Write(resp []byte) {
	sock.Conn.Write(resp)
}

func (sock *ReportSysClientSocket) GetIMEI() string {
	sock.mx.Lock()
	imei := sock.IMEI
	sock.mx.Unlock()
	return imei
}

func (sock *ReportSysClientSocket) WriteServCommand(payload []byte) error{	
	str := hex.EncodeToString(payload)
	sock.App.Logger.Debugf("ID:%s, server command:%s", sock.IMEI, str)
	
	_, err := sock.Conn.Write(payload)
	if err != nil {
		sock.App.Logger.Errorf("sock.Conn.Write %v", err)
	}
	
	sock.IncUploadedBytes(uint64(len(payload)))	
	return nil
}

func (sock *ReportSysClientSocket) HandleConnection(connLiveSec int) {
	
	defer sock.Conn.Close()
	
	package_buf := make([]byte, 1180)//10 packets max
	
	for {
		package_len, err := sock.Conn.Read(package_buf)			
		
		sock.LastActivity = time.Now()
		sock.Conn.SetReadDeadline(time.Now().Add( time.Duration(connLiveSec) * time.Second))
		
		switch err {
		case nil:
			sock.IncDownloadedBytes(uint64(package_len))
			
			sock.App.Logger.Debugf("ID:%s, Package %d bytes", sock.GetDescr(), package_len)
						
			if sock.App.IsSysPackage(package_buf, package_len, sock) {
				continue
			
			}else {
				packet_count := package_len / DATA_PACKAGE_LEN 
				for packet_ind := 0; packet_ind < packet_count; packet_ind++ {
					packet_offset := DATA_PACKAGE_LEN * packet_ind
					
					if package_buf[0+packet_offset] != 0xAF || package_buf[1+packet_offset] != 0x84 || package_buf[DATA_PACKAGE_LEN-2+packet_offset] != 0x0D || package_buf[DATA_PACKAGE_LEN-1+packet_offset] != 0x0A {
						sock.App.Logger.Debugf("ID:%s, wrong package structrure", sock.GetDescr())
						sock.App.Logger.Debugf("[0]=%d", package_buf[0+packet_offset])
						sock.App.Logger.Debugf("[1]=%d", package_buf[1+packet_offset])
						sock.App.Logger.Debugf("[DATA_PACKAGE_LEN-2]=%d", package_buf[DATA_PACKAGE_LEN-2+packet_offset])
						sock.App.Logger.Debugf("[DATA_PACKAGE_LEN-1]=%d", package_buf[DATA_PACKAGE_LEN-1+packet_offset])									
						continue
					}
				
					sock.IMEI = strconv.FormatUint(uint64(package_buf[24+packet_offset] - 0x20) * 100000000 + uint64(package_buf[25+packet_offset] - 0x20) * 1000000 + uint64(package_buf[26+packet_offset] - 0x20) * 10000 + uint64(package_buf[27+packet_offset] - 0x20) * 100 + uint64(package_buf[28+packet_offset] - 0x20), 10)
					tracker_time := time.Date(int(package_buf[9+packet_offset] - 0x20) + YEAR_START, time.Month(package_buf[8+packet_offset] - 0x20), int(package_buf[7+packet_offset] - 0x20), int(package_buf[4+packet_offset] - 0x20), int(package_buf[5+packet_offset] - 0x20), int(package_buf[6+packet_offset] - 0x20), 0, time.UTC)
					//!!! временно !!!
					tracker_time = tracker_time.Add(time.Hour * time.Duration(1))
					
					
					/*ns := ""
					ew := ""
					switch package_buf[10] - 0x20 {
					case COORD_STATUS_NE:
						ns = DIR_N
						ew = DIR_E
					case COORD_STATUS_SE:
						ns = DIR_S
						ew = DIR_E
					case COORD_STATUS_NW:
						ns = DIR_N
						ew = DIR_W
					case COORD_STATUS_SW:
						ns = DIR_S
						ew = DIR_W
					default :	
						ns = DIR_S
						ew = DIR_W
					}
					*/
					
					lat_deg := int(package_buf[11+packet_offset] - 0x20)
					lat_min := int(package_buf[12+packet_offset] - 0x20)
					lat_min_dec := int(package_buf[13+packet_offset] - 0x20) * 100 + int(package_buf[14+packet_offset] - 0x20)
					lat_s := fmt.Sprintf("%d%02d.%d", lat_deg, lat_min, lat_min_dec)				
					
					lon_deg := int(package_buf[15+packet_offset] - 0x20) * 100 + int(package_buf[16+packet_offset] - 0x20)
					lon_min := int(package_buf[17+packet_offset] - 0x20)
					lon_min_dec := int(package_buf[18+packet_offset] - 0x20) * 100 + int(package_buf[19+packet_offset] - 0x20)
					lon_s := fmt.Sprintf("%03d%02d.%d", lon_deg, lon_min, lon_min_dec)				

					tel_data := app.TelematicsData{ID: sock.IMEI,
							GPSTime: tracker_time,
							GPSValid: false,
							ReceivedTime: time.Now(),
							Lon_s: lon_s,
							Lon: convertDegreeToFloat(lon_deg, lon_min, lon_min_dec),
							Lat_s: lat_s,
							Lat: convertDegreeToFloat(lat_deg, lat_min, lat_min_dec),
							
							Speed: (int(package_buf[20+packet_offset] - 0x20) * 100 + int(package_buf[21+packet_offset] - 0x20) ) / 10,
							Heading: int(package_buf[22+packet_offset] - 0x20) * 100 + int(package_buf[23+packet_offset] - 0x20),
							SattlliteNum: 0,
							Height: 0,
							VoltExt: int16(package_buf[47+packet_offset] - 0x20) + int16(package_buf[48+packet_offset] - 0x20) /100,
							VoltInt: 0,
							SignalLevel: 0,
							Odom: uint32(package_buf[49+packet_offset] - 0x20) * 1000000 + uint32(package_buf[50+packet_offset] - 0x20) * 10000 + uint32(package_buf[51] - 0x20) * 100 + uint32(package_buf[52] - 0x20),
							FromMemory: (package_buf[3+packet_offset] - 0x20) > 0,						
					}
					if len(lat_s) == VALID_LAT_LEN && len(lon_s) == VALID_LON_LEN && tel_data.Lon > 0 && tel_data.Lat > 0 {
						tel_data.GPSValid = true
					}	
					/*sock.App.Logger.Debugf(`Writing data id=%s, tracker_time=%v, Lon=%f, Lat=%f, Speed=%d, Heading=%d, Odom=%d, VoltExt=%d,
							lat_deg=%d,lat_min=%d,lat_min_dec=%d, lat=%f, lat_s=%s
							lon_deg=%d,lon_min=%d,lon_min_dec=%d, lon=%f, lon_s=%s`,
						tel_data.ID,
						tracker_time,
						tel_data.Lon,
						tel_data.Lat,
						tel_data.Speed,
						tel_data.Heading,
						tel_data.Odom,
						tel_data.VoltExt,
						lat_deg,lat_min,lat_min_dec,tel_data.Lat,tel_data.Lat_s,
						lon_deg,lon_min,lon_min_dec,tel_data.Lon,tel_data.Lon_s,
						)
					*/
					//go sock.App.Storage.Write(&tel_data)
					sock.App.Storage.Write(&tel_data)
					sock.App.Logger.Debugf("ID=%s, packet decoded %v+",sock.IMEI, tel_data)
					
					if (package_buf[2+packet_offset] - 0x20) == BACK_REPORT_CADR {
						sock.writeServResponse()
					}			
				}
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

func (sock *ReportSysClientSocket) writeServResponse() error{
	resp := []byte{0x54, 0x53, 0x41, 0x0D, 0x0A}
	_, err := sock.Conn.Write(resp)
	if err != nil {
		sock.App.Logger.Errorf("sock.Conn.Write %v", err)
	}
	
	sock.IncUploadedBytes(uint64(len(resp)))
	return nil
}

func (sock *ReportSysClientSocket) GetDescr() string{
	var descr string
	if sock.IMEI != "" {
		descr = sock.IMEI
	}else{
		descr = sock.Conn.RemoteAddr().String()
	}
	return descr
}

func convertDegreeToFloat(degree int, min int, minDec int) float32 {	
	pw := math.Pow(10, float64(len(strconv.Itoa(minDec))))
	return float32(degree) + (float32(min) + float32(float64(minDec)/pw) )/60.0
}
