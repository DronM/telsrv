package main

import(
	"os"
	"fmt"
	
	"telsrv/app"
	"telsrv/reportsyst"
	"telsrv/arnavi"
	"telsrv/storage_pg"
	
	"github.com/labstack/gommon/log"
)

//https://kodazm.ru/articles/go/tcp-server-clockwork/

//var App *Application

func main() {

	App := &app.Application{}
	
	//ini_file := string(bytes.Replace([]byte(os.Args[0]), []byte(".exe"), []byte(".json"), -1))
	var ini_file string
	if len(os.Args) >= 2 {
		ini_file = os.Args[1]
	}else{
		ini_file = os.Args[0]+".json"
	}
	
	
	config := AppConfig{}
	err := config.ReadConf(ini_file)
	if err != nil {
		panic(fmt.Sprintf("ReadConf: %v",err))
	}

	App.Logger = log.New("-")
	App.Logger.SetHeader("${time_rfc3339_nano} ${short_file}:${line} ${level} -${message}")
	App.Logger.SetLevel(config.getLogLevel()) //log.Lvl(1)

	App.CommandKey = config.CommandKey	
		
	App.Storage = &storage_pg.StoragePG{ConnMaxIdleTime: config.ConnMaxIdleTime, ConnMaxTime: config.ConnMaxTime}
	err = App.Storage.Init(config.StorageConnection, App.Logger, config.DbProcessCount)
	if err != nil {
		App.Logger.Fatalf("App.Storage.Init %v",err)
	}
	
	//ReportSystems server
	reportsyst_new_socket := func() app.ClientSocketer{
		return &reportsyst.ReportSysClientSocket{}
	}
	go App.RunServer("ReportSystems", config.ReportSystSrv.Host, config.ReportSystSrv.Port, config.ReportSystSrv.ConLiveSec, reportsyst_new_socket)
	
	//Arnavi server
	arnavi_new_sock := func() app.ClientSocketer{
		return &arnavi.ArnaviClientSocket{}
	}
	App.RunServer("Arnavi", config.ArnaviSrv.Host, config.ArnaviSrv.Port, config.ArnaviSrv.ConLiveSec, arnavi_new_sock)
}

