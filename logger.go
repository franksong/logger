// logger with level
// better use globalLogger
package logger

import (
	"fmt"
	"jscfg"
	"log"
	"os"
	"path"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
	"timer"
)

const (
	CfgBaseDir = "cfg_dir/"
	LogBaseDir = "log_dir/"
)

//配置表
type stCfg struct {
	LogNumPreFile uint32 //最大条数
	LogLevel      int    //日志级别
	NewFile       int    // 创建新日志文件频率(小时)
}

var cfg stCfg

// base path（exec go program path）
var sBasePath = ""

//log索引
var iLogIndex uint64 = 0
var lCreateFile sync.Mutex

//记录到文件
func createLoggerFile(index uint64) {
	lCreateFile.Lock()
	if index < iLogIndex {
		lCreateFile.Unlock()
		return
	}
	iLogIndex = index + 1
	lCreateFile.Unlock()

	//文件夹被删除了？
	if err := os.MkdirAll(sBasePath, os.ModePerm); err != nil {
		return
	}

	stime := time.Now().Format("20060102150405")
	i := 0
	sname := ""
	for {
		sname = sBasePath + stime + "-" + strconv.Itoa(i) + ".log"
		f, err := os.OpenFile(sname, os.O_CREATE|os.O_EXCL|os.O_RDWR, os.ModePerm)
		if err != nil {
			i++
			continue
		}

		loggerTemp := New(f, "", log.LstdFlags|log.Lshortfile, cfg.LogLevel, iLogIndex)
		if globalLogger != nil {
			globalLogger.close()
		}
		globalLogger = loggerTemp

		//标准输出重定向
		os.Stdout = f
		os.Stderr = f

		break
	}

	//定时换
	timeNow := time.Now()
	cd := cfg.NewFile
	if cd < 1 {
		cd = 1
	}
	timeNext := time.Date(timeNow.Year(), timeNow.Month(), timeNow.Day(), timeNow.Hour()+cd, 0, 0, 0, time.Local)
	tm := timer.NewTimer(time.Second * time.Duration(timeNext.Unix()-timeNow.Unix()))
	tm.Start(func() {
		tm.Stop()
		createLoggerFile(iLogIndex)
	})
}

//定时读取配置表
func keepLoadCfg() error {
	spath, err := os.Getwd()
	if err != nil {
		return err
	}

	var cfgTemp stCfg
	//读取配置表
	if err := jscfg.ReadJson(path.Join(spath, CfgBaseDir+"logger.json"), &cfgTemp); err != nil {
		return err
	}

	if cfg.LogLevel != cfgTemp.LogLevel && globalLogger != nil {
		globalLogger.SetLevel(cfgTemp.LogLevel)
	}
	cfg = cfgTemp

	return nil
}

func init() {
	_, sfile := path.Split(os.Args[0])

	spath, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	//读取配置表
	if err := jscfg.ReadJson(path.Join(spath, CfgBaseDir+"logger.json"), &cfg); err != nil {
		panic(err)
	}
	sBasePath = path.Join(spath, LogBaseDir, sfile) + "/"

	//定时读取配置表，开关日志及数量
	tm := timer.NewTimer(time.Second * 5)
	tm.Start(func() {
		keepLoadCfg()
	})

	createLoggerFile(iLogIndex)
}

var globalLogger *Logger

const (
	DEBUG = iota
	INFO
	WARNING
	ERROR
	FATAL
	NONE
)

var levelNames = []string{
	"DEBUG: ",
	"INFO: ",
	"WARNING: ",
	"ERROR: ",
	"FATAL: ",
	"NONE: ",
}

func init() {
	// levelPrefixes = make([]string, len(levelNames))
	// for i, name := range levelNames {
	// 	levelPrefixes[i] = name + ": "
	// }
}

func Debug(format string, args ...interface{}) {
	globalLogger.Output(DEBUG, format, args...)
}

func Info(format string, args ...interface{}) {
	globalLogger.Output(INFO, format, args...)
}

func Warning(format string, args ...interface{}) {
	globalLogger.Output(WARNING, format, args...)
}

func Error(format string, args ...interface{}) {
	globalLogger.Output(ERROR, format, args...)
}

func Fatal(format string, args ...interface{}) {
	globalLogger.Output(FATAL, format, args...)
	debug.PrintStack()
	panic(fmt.Sprintf(format, args...))
}

type Logger struct {
	file    *os.File
	logger  *log.Logger
	level   int
	index   uint64 //索引，防止多个log同时请求创建新的文件
	numbers uint32 //log数量，超出开新文件
}

func New(f *os.File, prefix string, flag, level int, index uint64) *Logger {
	return &Logger{
		file:    f,
		logger:  log.New(f, prefix, flag),
		level:   level,
		index:   index,
		numbers: uint32(0),
	}
}

func (self *Logger) Debug(format string, args ...interface{}) {
	self.Output(DEBUG, format, args...)
}

func (self *Logger) Info(format string, args ...interface{}) {
	self.Output(INFO, format, args...)
}

func (self *Logger) Warning(format string, args ...interface{}) {
	self.Output(WARNING, format, args...)
}

func (self *Logger) Error(format string, args ...interface{}) {
	self.Output(ERROR, format, args...)
}

func (self *Logger) Fatal(format string, args ...interface{}) {
	self.Output(FATAL, format, args...)
	debug.PrintStack()
	panic(fmt.Sprintf(format, args...))
}

//关闭，只调用一次，给30秒的缓冲，肯定都已经写完了
func (self *Logger) close() {
	go func() {
		time.Sleep(time.Second * 30)
		self.file.Close()
	}()
}

// 如果对象包含需要加密的信息（例如密码），请实现Redactor接口
type Redactor interface {
	// 返回一个去处掉敏感信息的示例
	Redacted() interface{}
}

// Redact 返回跟字符串等长的“＊”。
func Redact(s string) string {
	return strings.Repeat("*", len(s))
}

func (self *Logger) Output(level int, format string, args ...interface{}) {
	if self.level > level {
		return
	}
	redactedArgs := make([]interface{}, len(args))
	for i, arg := range args {
		if redactor, ok := arg.(Redactor); ok {
			redactedArgs[i] = redactor.Redacted()
		} else {
			redactedArgs[i] = arg
		}
	}
	self.logger.Output(3, levelNames[level]+fmt.Sprintf(format, redactedArgs...))
}

func SetLogger(logger *Logger) {
	globalLogger = logger
}

func (self *Logger) SetFlags(flag int) {
	self.logger.SetFlags(flag)
}

func (self *Logger) SetPrefix(prefix string) {
	self.logger.SetPrefix(prefix)
}

func (self *Logger) SetLevel(level int) {
	self.level = level
}

func LogNameToLogLevel(name string) int {
	s := strings.ToUpper(name)
	for i, level := range levelNames {
		if level == s {
			return i
		}
	}
	panic(fmt.Errorf("no log level: %v", name))
}
