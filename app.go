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

	"golang.org/x/text/encoding/simplifiedchinese"
	"gopkg.in/yaml.v2"
)

var (
	wg sync.WaitGroup
)

type Config struct {
	NumPlots  string `yaml:"NumPlots"`
	KSize     string `yaml:"KSize"`
	Buffer    string `yaml:"Buffer"`
	Threads   string `yaml:"Threads"`
	Buckets   string `yaml:"Buckets"`
	TempPath  string `yaml:"TempPath"`
	FinalPath string `yaml:"FinalPath"`
	Total     int    `yaml:"Total"`
	Sleep     int    `yaml:"Sleep"`
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

	homedir, err := GetUserInfo()
	if err != nil {
		fmt.Println("获取用户目录失败")
		os.Exit(0)
	}
	var ChiaAppPath string
	rootPath := strings.Join([]string{homedir, `AppData\Local\chia-blockchain`}, `\`)
	if !IsDir(rootPath) {
		fmt.Println("获取Chia运行目录失败")
		os.Exit(0)
	}
	if !IsDir(confYaml.TempPath) {
		fmt.Println("获取缓存目录失败，请检查配置文件")
		os.Exit(0)
	}
	if !IsDir(confYaml.FinalPath) {
		fmt.Println("获取数据目录失败，请检查配置文件")
		os.Exit(0)
	}
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
	farmKey, poolKey := GetPublicKey(ChiaAppPath)

	ChiaExec := strings.Join([]string{ChiaAppPath, "chia.exe"}, `\`)
	// params := []string{"/C", "start", ChiaExec, "plots", "create", "-k", "32", "-b", "4000", "-r", "2", "-f", farmKey, "-p", poolKey, "-t", "d:", "-d", "d:", ">>", Path}
	ChiaCmd := strings.Join([]string{"start", ChiaExec, "plots", "create", "-n", confYaml.NumPlots, "-k", confYaml.KSize, "-b", confYaml.Buffer, "-r", confYaml.Threads, "-f", farmKey, "-p", poolKey, "-t", confYaml.TempPath, "-d", confYaml.FinalPath}, " ")
	for i := 0; i < confYaml.Total; i++ {
		go RunExec(ChiaCmd)
		time.Sleep(time.Duration(confYaml.Sleep) * time.Second)
	}
	startTime := time.Now().Format("2006-01-02 15:04:05")
	appName := "chia.exe"
	task := func() {
		status, _, _, _ := isProcessExist(appName)
		if !status {
			// content := strings.Join([]string{mailYaml.Content, time.Now().Format("2006-01-02 15:04:05")}, " 完成于 ")
			// title := strings.Join([]string{mailYaml.Title, startTime}, " 开始于 ")
			wg.Add(1)
			fmt.Println(startTime + "done")
			wg.Wait()
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
	cmd := exec.Command("cmd", "/C", ChiaExec)
	fmt.Println("CmdAndChangeDirToFile", cmd.Args)
	cmd.Start()
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
func GetPublicKey(ChiaAppPath string) (farmKey, poolKey string) {
	ChiaExec := strings.Join([]string{ChiaAppPath, "chia.exe"}, `\`)
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

func ConvertByte2String(byte []byte, charset string) string {
	var str string
	switch charset {
	case "GB18030":
		var decodeBytes, _ = simplifiedchinese.GB18030.NewDecoder().Bytes(byte)
		str = string(decodeBytes)
	case "UTF8":
		fallthrough
	default:
		str = string(byte)
	}
	return str
}

func isProcessExist(appName string) (bool, string, int, int) {
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
		err := errors.New(`Can't find "/" or "\".`)
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
