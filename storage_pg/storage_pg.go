package storage_pg

import(
	"context"
	"fmt"
	"os"
	"path/filepath"
	"bufio"
	"sync"
	"time"
	
	"telsrv/app"
	
	"github.com/labstack/gommon/log"
	"github.com/jackc/pgx/v5"
)

const TIME_LAYOUT = "2006-01-02T15:04:05.000-07"
const QUERY_FILE_NAME = "queries.sql"
const QUERY_FILE_EXEC_PAUSE_MIN = 5
const STORAGE_DESCR = "Postgresql storage"

type StoragePG struct {
	ConnStr string
	Logger *log.Logger
	FileLock sync.Mutex
	TelData chan *app.TelematicsData
	ConnMaxIdleTime int
	ConnMaxTime int
}

func (s *StoragePG) GetDescr() string {
	return STORAGE_DESCR
}

func (s *StoragePG) Init(connStr string, logger *log.Logger, processCount int) error {
	if processCount == 0 {
		processCount = 1
	}		
	s.ConnStr = connStr
	s.Logger = logger	
	s.TelData = make(chan *app.TelematicsData)
	
	for i:=0; i<processCount; i++ {
		go s.WaitForData(i)
	}
	//File		
	go (func(storage *StoragePG) {
		for {			
			storage.queryFromFile()
			time.Sleep(time.Duration(QUERY_FILE_EXEC_PAUSE_MIN) * time.Minute)
		}
	})(s)
	s.Logger.Infof("StoragePG: %s initialized. ProcessCount=%d, connMaxIdleTime=%d, connMaxTime=%d", s.GetDescr(), processCount, s.ConnMaxIdleTime, s.ConnMaxTime)
	
	return nil	
}

//never exists
func (s *StoragePG) WaitForData(procId int)  {	
	var data *app.TelematicsData
	var conn_dead_time time.Time
	var conn *pgx.Conn
	for {
		if s.ConnMaxIdleTime == 0 || conn == nil {
			//blocking
			data = <- s.TelData
			//s.processData(conn, data, &conn_dead_time, procId)
			
			s.Logger.Debugf("StoragePG WaitForData: Got query to execute, procId=%d", procId)			
			if conn == nil {
				var err error
				conn, err = pgx.Connect(context.Background(), s.ConnStr)
				if err == nil {
					if s.ConnMaxTime > 0 {
						conn_dead_time = time.Now().Add(time.Duration(s.ConnMaxTime) * time.Millisecond)
					}
					s.Logger.Debugf("StoragePG WaitForData: Acquired DB connection, procId=%d", procId)
				}else{
					s.Logger.Errorf("StoragePG WaitForData pgx.Connect():%v", err)			
				}
			}						
			query := getQuery(data)				
			if conn == nil {
				s.queryToFile(query)
				
			}else if _, err := conn.Exec(context.Background(), query); err != nil {
				s.Logger.Errorf("StoragePG WaitForData: %v",err)
				s.queryToFile(query)
				conn.Close(context.Background())
				conn = nil
			}
			
		}else{
			select {
			case data = <- s.TelData:
				//s.processData(conn, data, &conn_dead_time, procId)
				s.Logger.Debugf("StoragePG WaitForData: Got query to execute, procId=%d", procId)			
				if conn == nil {
					var err error
					conn, err = pgx.Connect(context.Background(), s.ConnStr)
					if err == nil {
						if s.ConnMaxTime > 0 {
							conn_dead_time = time.Now().Add(time.Duration(s.ConnMaxTime) * time.Millisecond)
						}
						s.Logger.Debugf("StoragePG WaitForData: Acquired DB connection, procId=%d", procId)
					}else{
						s.Logger.Errorf("StoragePG WaitForData DbPool.Acquire():%v", err)			
					}
				}						
				query := getQuery(data)				
				if conn == nil {
					s.queryToFile(query)
					
				}else if _, err := conn.Exec(context.Background(), query); err != nil {
					s.Logger.Errorf("StoragePG WaitForData: %v",err)
					s.queryToFile(query)
					conn.Close(context.Background())
					conn = nil
				}
				
			case <-time.After(time.Millisecond * time.Duration(s.ConnMaxIdleTime)):
				if conn != nil {					
					conn.Close(context.Background())
					conn = nil
					s.Logger.Debugf("StoragePG WaitForData: conn killed on idle timeout, procId=%d", procId)
				}
			}
		}
		if s.ConnMaxTime > 0 && conn != nil && time.Now().After(conn_dead_time) {			
			conn.Close(context.Background())
			conn = nil		
			s.Logger.Debugf("StoragePG WaitForData: conn killed on max timeout, procId=%d", procId)
		}
		time.Sleep(time.Duration(50) * time.Millisecond)		
	}
	if conn != nil {
		conn.Close(context.Background())
	}
}

func (s *StoragePG) Write(data *app.TelematicsData) {
	s.TelData <- data
}

func (s *StoragePG) queryToFile(str string) {
	f_name:= filepath.Dir(os.Args[0]) + "/" +QUERY_FILE_NAME
	s.Logger.Warnf("StoragePG: queryToFile %s", f_name)	
	s.FileLock.Lock()
	file, err := os.OpenFile(f_name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err == nil {
		file.WriteString(str+"\n\n") //empty string separator
		file.Close()
	}else{
		s.Logger.Errorf("StoragePG queryToFile: failed os.OpenFile() %v",err)
	}	
	s.FileLock.Unlock()
}

func (s *StoragePG) queryFromFile() {
	f_name:= filepath.Dir(os.Args[0]) + "/" +QUERY_FILE_NAME	
	s.Logger.Warnf("StoragePG: queryFromFile %s", f_name)	
	s.FileLock.Lock()
	defer s.FileLock.Unlock()
	
	file, err := os.Open(f_name)
	if err != nil {
		return
	}
	
	conn, err := pgx.Connect(context.Background(), s.ConnStr)
	if err != nil {
		s.Logger.Errorf("StoragePG queryFromFile: DbPool.Acquire() failed %v",err)
		return
	}
	defer conn.Close(context.Background())

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	query := ""
	for scanner.Scan() {
		str := scanner.Text()
		if str == "" && query != "" {
			if _, err = conn.Exec(context.Background(), query); err != nil {
				s.Logger.Errorf("StoragePG queryFromFile: failed conn.Exec() %v",err)
				return
			}
			query = ""
			s.Logger.Debug("StoragePG queryFromFile: query executed")		
		}else{
			query+= str
		}
	}
	 	
	file.Close()
	if err := os.Remove(f_name); err != nil {
		s.Logger.Debug("StoragePG queryFromFile: file remove error %v", err)			
	}else{
		s.Logger.Warn("StoragePG queryFromFile: file removed")			
	}
}


func getQuery(data *app.TelematicsData) string{
	from_mem := 0
	if data.FromMemory {
		from_mem = 1
	}
	gps_valid := 0
	if data.GPSValid {
		gps_valid = 1
	}
	return fmt.Sprintf(`INSERT INTO car_tracking
		(car_id, period,
		longitude, latitude,
		speed, ns, ew,
		magvar, heading, recieved_dt, gps_valid, from_memory, odometer,
		voltage, engine_on, lon, lat, sat_num)
		VALUES('%s',
		'%s' At time zone 'utc',
		'%s',
		'%s',
		%d,
		CASE WHEN %d >=90 AND %d <270 THEN 'n' ELSE 's' END,
		CASE WHEN %d >=180 THEN 'w' ELSE 'e' END,
		0,
		%d,
		now() At time zone 'utc',
		%d,
		%d,
		%d,
		%d,
		1,
		%f,
		%f,
		%d
		)
		ON CONFLICT (car_id, period) DO NOTHING`,
		data.ID,
		data.GPSTime.Format(TIME_LAYOUT),
		data.Lon_s,
		data.Lat_s,
		data.Speed,
		data.Heading,
		data.Heading,
		data.Heading,
		data.Heading,		
		gps_valid,
		from_mem,
		data.Odom,
		data.VoltExt,		
		data.Lon,
		data.Lat,
		data.SattlliteNum)
		//SET recieved_dt = now() At time zone 'utc'	
}


