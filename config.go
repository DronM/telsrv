package main

import (
	"encoding/json"
	"io/ioutil"
	"bytes"	
	
	"github.com/labstack/gommon/log"
)

type SrvConfig struct {
	Host string `json:"host"`
	Port int `json:"port"`
	ConLiveSec int `json:"conLiveSec"`	
}

type AppConfig struct {
	ArnaviSrv SrvConfig `json:"arnavi"`
	ReportSystSrv SrvConfig `json:"reportsyst"`
	StorageConnection string `json:"storageConnection"`
	LogLevel string `json:"logLevel"`
	CommandKey string `json:"commandKey"`
	DbProcessCount int `json:"dbProcessCount"`
	ConnMaxIdleTime int `json:"connMaxIdleTime"`
	ConnMaxTime int `json:"connMaxTime"`
}

func (c *AppConfig) ReadConf(fileName string) error{
	file, err := ioutil.ReadFile(fileName)
	if err == nil {
		file = bytes.TrimPrefix(file, []byte("\xef\xbb\xbf"))
		err = json.Unmarshal([]byte(file), c)		
	}
	return err
}

func (c *AppConfig) WriteConf(fileName string) error{
	cont_b, err := json.Marshal(c)
	if err == nil {
		err = ioutil.WriteFile(fileName, cont_b, 0644)
	}
	return err
}

func (c AppConfig) getLogLevel() log.Lvl {
	var lvl log.Lvl

	switch c.LogLevel {
	case "debug":
		lvl = log.DEBUG
		break
	case "info":
		lvl = log.INFO
		break
	case "warn":
		lvl = log.WARN
		break
	case "error":
		lvl = log.ERROR
		break
	default:
		lvl = log.INFO
	}
	return lvl

}
