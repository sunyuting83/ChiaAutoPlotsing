package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Pool      bool   `yaml:"Pool"`
	NumPlots  string `yaml:"NumPlots"`
	KSize     string `yaml:"KSize"`
	Buffer    string `yaml:"Buffer"`
	Threads   string `yaml:"Threads"`
	Buckets   string `yaml:"Buckets"`
	TempPath  string `yaml:"TempPath"`
	FinalPath string `yaml:"FinalPath"`
	Total     int    `yaml:"Total"`
	Sleep     int    `yaml:"Sleep"`
	RunPath   string `yaml:"RunPath"`
	FarmerKey string `yaml:"FarmerKey"`
	PoolKey   string `yaml:"PoolKey"`
}

func main() {
	CurrentPath, _ := GetCurrentPath()
	LinkPathStr := "/"
	if runtime.GOOS == "windows" {
		LinkPathStr = "\\"
	}
	ConfigFile := strings.Join([]string{CurrentPath, "config.yaml"}, LinkPathStr)

	confYaml := new(Config)
	yamlFile, err := ioutil.ReadFile(ConfigFile)
	if err != nil {
		fmt.Println("读取配置文件出错")
		os.Exit(0)
	}
	err = yaml.Unmarshal(yamlFile, confYaml)
	// err = yaml.Unmarshal(yamlFile, &resultMap)
	if err != nil {
		fmt.Println("读取配置文件出错")
		os.Exit(0)
	}
	var (
		ChiaAppPath string = confYaml.RunPath
		rootPath    string = confYaml.RunPath
		appName     string = "chia"
		farmKey     string = confYaml.FarmerKey
		poolKey     string = confYaml.PoolKey
	)
	if runtime.GOOS == "windows" {
		appName = "chia.exe"
	}
	if !IsDir(rootPath) {
		fmt.Println("获取Chia运行目录失败")
		time.Sleep(time.Duration(10) * time.Second)
		os.Exit(0)
	}
	if !IsDir(confYaml.FinalPath) {
		fmt.Println(strings.Join([]string{"获取缓存目录", confYaml.FinalPath, "失败，请检查配置文件"}, " "))
		time.Sleep(time.Duration(10) * time.Second)
		os.Exit(0)
	}
	if !IsDir(confYaml.TempPath) {
		fmt.Println("获取缓存目录失败，请检查配置文件")
		time.Sleep(time.Duration(10) * time.Second)
		os.Exit(0)
	}
	ChiaExec := GetChieExec(ChiaAppPath)
	if len(confYaml.FarmerKey) <= 0 && len(confYaml.PoolKey) <= 0 {
		fmt.Println("农田公钥和矿池公钥不能为空")
		time.Sleep(time.Duration(10) * time.Second)
		os.Exit(0)
	}

	StartPlots(ChiaExec, farmKey, poolKey, *confYaml)
	fmt.Println(time.Now().Format("2006-01-02 15:04:05"))

	task := func() {
		status := isProcessExist(appName)
		if !status {
			fmt.Println(time.Now().Format("2006-01-02 15:04:05"))
			fmt.Println("P盘任务结束，10秒后自动关闭本窗口")
			time.Sleep(time.Duration(10) * time.Second)
			os.Exit(0)
		}
	}
	var ch chan int
	ticker := time.NewTicker(time.Second * 3)
	go func() {
		for range ticker.C {
			task()
		}
		ch <- 1
	}()
	<-ch
}

func RunExec(ChiaExec string) {
	if runtime.GOOS == "windows" {
		WinCmd := strings.Join([]string{`start `, ChiaExec}, "")
		cmd := exec.Command("cmd", "/C", WinCmd)
		fmt.Println(cmd.Args)
		cmd.Start()
	} else {
		LinCmd := strings.Join([]string{`nohup `, ChiaExec, " 2>&1"}, "")
		cmd := exec.Command("/bin/bash", "-c", LinCmd)
		fmt.Println(cmd.Args)
		cmd.Start()
	}
}
func StartPlots(ChiaExec, farmKey, poolKey string, confYaml Config) {
	var pool string = "-c"
	if !confYaml.Pool {
		pool = "-p"
	}
	ChiaCmd := strings.Join([]string{ChiaExec, "plots", "create", "-n", confYaml.NumPlots, "-k", confYaml.KSize, "-b", confYaml.Buffer, "-r", confYaml.Threads, "-f", farmKey, pool, poolKey, "-t", confYaml.TempPath, "-d", confYaml.FinalPath}, " ")
	for i := 0; i < confYaml.Total; i++ {
		go RunExec(ChiaCmd)
		time.Sleep(time.Duration(confYaml.Sleep) * time.Second)
	}
}

func IsDir(path string) bool {
	s, err := os.Stat(path)
	if err != nil {
		return false
	}
	return s.IsDir()
}
func GetUserInfo() (homedir string, err error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	return u.HomeDir, nil
}
func GetPublicKey(ChiaExec string) (farmKey, poolKey string) {
	c := exec.Command(ChiaExec, "keys", "show")
	pwdOutput, _ := c.Output()
	pwdLine := strings.Split(string(pwdOutput), "\n")
	if runtime.GOOS == "windows" {
		pwdLine = strings.Split(string(pwdOutput), "\r\n")
	}
	for _, keys := range pwdLine {
		if len(keys) > 0 {
			if strings.Contains(keys, "Farmer") {
				farmKey = strings.Split(keys, ": ")[1]
			}
			if strings.Contains(keys, "Pool") {
				poolKey = strings.Split(keys, ": ")[1]
			}
		}
	}
	return
}

func GetChieExec(ChiaAppPath string) (ChiaExec string) {
	ChiaExe := "chia"
	LineString := `/`
	if runtime.GOOS == "windows" {
		ChiaExe = "chia.exe"
		LineString = `\`
	}
	ChiaExec = strings.Join([]string{ChiaAppPath, ChiaExe}, LineString)
	return
}

func isProcessExist(appName string) bool {
	OS := runtime.GOOS
	if OS == "windows" {
		command := `ps -ef|grep "chia plots cerate"| grep -v grep`
		c, _ := RunCommand(OS, command)
		if len(c) > 0 {
			return len(c) > 0
		}
		return false
	}
	command := `wmic process where name="chia.exe" get commandline 2>nul | find "create" 1>nul 2>nul && echo 1 || echo 0`
	c := GetPublicWinCommandLine(OS, command)
	if c == "1" {
		return c == "1"
	}
	return false
}

func GetPublicWinCommandLine(OS, command string) (s string) {
	p, _ := RunCommand(OS, command)
	p = CompressStr(p)
	pList := strings.Split(p, "\r\n")
	for _, v := range pList {
		if len(v) > 0 {
			if strings.Contains(v, "=") {
				s = strings.Split(v, "=")[1]
				break
			}
		}
	}
	return
}
func CmdAndChangeDirToFile(commandName string, params []string) {
	cmd := exec.Command(commandName, params...)
	fmt.Println(cmd.Args)
	cmd.Start()
	cmd.Wait()
}

func GetCurrentPath() (string, error) {
	file, err := exec.LookPath(os.Args[0])
	if err != nil {
		return "", err
	}
	path, err := filepath.Abs(file)
	if err != nil {
		return "", err
	}
	//fmt.Println("path111:", path)
	if runtime.GOOS == "windows" {
		path = strings.Replace(path, "\\", "/", -1)
	}
	//fmt.Println("path222:", path)
	i := strings.LastIndex(path, "/")
	if i < 0 {
		err := errors.New("can't find file")
		return "", err
	}
	return string(path[0 : i+1]), nil
}

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func IsExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}

func Int2Byte(data int) (ret []byte) {
	// 数字转Byte
	var len uintptr = unsafe.Sizeof(data)
	ret = make([]byte, len)
	var tmp int = 0xff
	var index uint = 0
	for index = 0; index < uint(len); index++ {
		ret[index] = byte((tmp << (index * 8) & data) >> (index * 8))
	}
	return ret
}

func Byte2Int(data []byte) int {
	// Byte转数字
	var ret int = 0
	var len int = len(data)
	var i uint = 0
	for i = 0; i < uint(len); i++ {
		ret = ret | (int(data[i]) << (i * 8))
	}
	return ret
}

// RunCommand run command
func RunCommand(OS, command string) (k string, err error) {
	var cmd *exec.Cmd
	if OS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
	} else {
		cmd = exec.Command("/bin/sh", "-c", command)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	if err := cmd.Start(); err != nil {
		return "", err
	}

	bytesErr, err := ioutil.ReadAll(stderr)
	if err != nil {
		return "", err
	}

	if len(bytesErr) != 0 {
		return "", errors.New("0")

	}

	bytes, err := ioutil.ReadAll(stdout)
	if err != nil {
		return "", err
	}

	if err := cmd.Wait(); err != nil {
		return "", err
	}
	return string(bytes), nil
}

// CompressStr
func CompressStr(str string) string {
	if str == "" {
		return ""
	}
	reg := regexp.MustCompile("\\s+")
	return reg.ReplaceAllString(str, "")
}
