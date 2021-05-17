package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"gopkg.in/yaml.v2"
)

var (
	wg sync.WaitGroup
)

type Config struct {
	NumPlots  string   `yaml:"NumPlots"`
	KSize     string   `yaml:"KSize"`
	Buffer    string   `yaml:"Buffer"`
	Threads   string   `yaml:"Threads"`
	Buckets   string   `yaml:"Buckets"`
	TempPath  string   `yaml:"TempPath"`
	FinalPath []string `yaml:"FinalPath"`
	Total     int      `yaml:"Total"`
	Sleep     int      `yaml:"Sleep"`
	RunPath   string   `yaml:"RunPath"`
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
		LogPath     string
	)
	if runtime.GOOS == "windows" {
		homedir, err := GetUserInfo()
		if err != nil {
			fmt.Println("获取用户目录失败")
			os.Exit(0)
		}
		rootPath = strings.Join([]string{homedir, `AppData\Local\chia-blockchain`}, `\`)
		appName = "chia.exe"
	}
	if !IsDir(rootPath) {
		fmt.Println("获取Chia运行目录失败")
		os.Exit(0)
	}
	if len(confYaml.FinalPath) > 0 {
		for _, item := range confYaml.FinalPath {
			if !IsDir(item) {
				fmt.Println(strings.Join([]string{"获取缓存目录", item, "失败，请检查配置文件"}, " "))
				os.Exit(0)
			}
		}
	} else {
		fmt.Println("获取数据目录失败，请检查配置文件")
		os.Exit(0)
	}
	LogPath = strings.Join([]string{CurrentPath, "log"}, "/")
	if !IsDir(LogPath) {
		err := os.Mkdir(LogPath, os.ModePerm)
		if err != nil {
			fmt.Println("创建日志目录失败，请重试")
			os.Exit(0)
		}
	}
	if !IsDir(confYaml.TempPath) {
		fmt.Println("获取缓存目录失败，请检查配置文件")
		os.Exit(0)
	}
	if runtime.GOOS == "windows" {
		files, _ := ioutil.ReadDir(rootPath)
		var versionNumber []string
		for _, f := range files {
			if strings.Contains(f.Name(), "app-") {
				versionNumber = append(versionNumber, string(f.Name()))
			}
		}
		ChiaAppPath = strings.Join([]string{rootPath, versionNumber[0], `resources\app.asar.unpacked\daemon`}, `\`)
		if len(versionNumber) > 1 {
			n := len(versionNumber) - 1
			ChiaAppPath = strings.Join([]string{rootPath, versionNumber[n], `resources\app.asar.unpacked\daemon`}, `\`)
		}
	}
	ChiaExec := GetChieExec(ChiaAppPath)
	farmKey, poolKey := GetPublicKey(ChiaAppPath, ChiaExec)

	StartPlots(LogPath, CurrentPath, ChiaExec, farmKey, poolKey, *confYaml)

	task := func() {
		status, _, _, _ := isProcessExist(appName)
		if !status {
			// content := strings.Join([]string{mailYaml.Content, time.Now().Format("2006-01-02 15:04:05")}, " 完成于 ")
			// title := strings.Join([]string{mailYaml.Title, startTime}, " 开始于 ")
			time.Sleep(time.Duration(180) * time.Second)
			current := GetCurrentNumber(CurrentPath, len(confYaml.FinalPath)-1)
			if current <= 0 {
				wg.Add(1)
				fmt.Println("done")
				wg.Wait()
				os.Exit(0)
			}
			StartPlots(LogPath, CurrentPath, ChiaExec, farmKey, poolKey, *confYaml)
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

func RunExec(ChiaExec, LogPath string) {
	cmd := exec.Command("nohup", ChiaExec, ">", LogPath, "2>&1")
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", "start", ChiaExec)
	}
	fmt.Println(cmd.Args)
	cmd.Start()
}
func StartPlots(LogPath, CurrentPath, ChiaExec, farmKey, poolKey string, confYaml Config) {
	current := GetCurrentNumber(CurrentPath, len(confYaml.FinalPath)-1)

	ChiaCmd := strings.Join([]string{ChiaExec, "plots", "create", "-n", confYaml.NumPlots, "-k", confYaml.KSize, "-b", confYaml.Buffer, "-r", confYaml.Threads, "-f", farmKey, "-p", poolKey, "-t", confYaml.TempPath, "-d", confYaml.FinalPath[current]}, " ")
	for i := 0; i < confYaml.Total; i++ {
		startTime := time.Now().Format("20060102")
		LogFileName := strings.Join([]string{startTime, "_", strconv.Itoa(i), ".log"}, "")
		logPath := strings.Join([]string{LogPath, LogFileName}, "/")
		go RunExec(ChiaCmd, logPath)
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
func GetPublicKey(ChiaAppPath, ChiaExec string) (farmKey, poolKey string) {
	c := exec.Command(ChiaExec, "keys", "show")
	pwdOutput, _ := c.Output()
	pwdLine := strings.Split(string(pwdOutput), "\r\n")
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
	LineString := `\`
	if runtime.GOOS == "windows" {
		ChiaExe = "chia.exe"
		LineString = "/"
	}
	ChiaExec = strings.Join([]string{ChiaAppPath, ChiaExe}, LineString)
	return
}

func isProcessExist(appName string) (bool, string, int, int) {
	// 做win的判断  这里没做完
	appary := make(map[string]int)
	cmd := exec.Command("cmd", "/C", "tasklist")
	output, _ := cmd.Output()
	n := strings.Index(string(output), "System")
	if n == -1 {
		fmt.Println("no find")
		os.Exit(1)
	}
	data := string(output)[n:]
	fields := strings.Fields(data)
	lange := []int{}
	for k, v := range fields {
		if v == appName {
			appary[appName], _ = strconv.Atoi(fields[k+1])
			lange = append(lange, appary[appName])
		}
	}
	if len(lange) > 0 {
		return true, appName, appary[appName], len(lange)
	}
	return false, appName, -1, 0
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

func GetCurrentNumber(CurrentPath string, current int) (n int) {
	NumberData := strings.Join([]string{CurrentPath, "nb"}, "/")
	if IsExist(NumberData) {
		number, err := ioutil.ReadFile(NumberData)
		if err != nil {
			return 0
		}
		return Byte2Int(number)
	} else {
		os.Create(NumberData)
		number := Int2Byte(current)
		ioutil.WriteFile(NumberData, number, 0644)
		return current
	}
}
