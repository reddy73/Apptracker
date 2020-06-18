package apptracker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

//HWND is a type for window handle
type HWND uintptr

//RECT is a type for window dimension
type RECT struct {
	Left, Top, Right, Bottom int32
}

const (
	PROCESS_ALL_ACCESS = 0x0010
	PROCESS_QUERY_INFO = 0x0400
	LineBreak          = "\r\n"
	PROCESS_STATS_LOG  = "\\AppData\\Local\\WFH\\processStats.txt"
)

// var handle uintptr
var (
	Flag           bool
	kernel32, _    = syscall.LoadLibrary("kernel32.dll")
	user32, _      = syscall.LoadLibrary("user32.dll")
	findWindowW, _ = syscall.GetProcAddress(user32, "FindWindowW")
)

//GetWindowInformation gets processname from process Id
func GetWindowInformation(hwnd uintptr) string {
	psapi := syscall.MustLoadDLL("psapi.dll")
	prc := psapi.MustFindProc("GetProcessImageFileNameW")
	var prcName [512]byte
	res, _, err := prc.Call(hwnd, uintptr(unsafe.Pointer(&prcName)), 512)
	log.Println(res, "Message: ", err)
	str := string(prcName[:])
	return str
}

//GetOpenProcess gets handle to the active process
func GetOpenProcess(pid uintptr) HWND {
	kernel32 := syscall.MustLoadDLL("kernel32.dll")
	proc := kernel32.MustFindProc("OpenProcess")
	res, _, err := proc.Call(ptr(PROCESS_ALL_ACCESS|PROCESS_QUERY_INFO), ptr(true), pid)
	log.Println("OpenProcess: result:", res, " Message:", err)
	return HWND(res)
}

//GetWindowThreadProcessID gets process id from handle
func GetWindowThreadProcessID(hwnd uintptr) uintptr {
	var prcsID uintptr = 0
	us32 := syscall.MustLoadDLL("user32.dll")
	prc := us32.MustFindProc("GetWindowThreadProcessId")
	ret, _, err := prc.Call(hwnd, uintptr(unsafe.Pointer(&prcsID)))
	log.Println("ProcessId: ", prcsID, "ret", ret, " Message: ", err)
	return prcsID
}

//GetForegroundWindow gets current foreground windows process handle
func GetForegroundWindow() uintptr {
	us32 := syscall.MustLoadDLL("user32.dll")
	prc := us32.MustFindProc("GetForegroundWindow")
	ret, _, _ := prc.Call()
	return ret
}

//FindWindowByTitle get hadnle to process holding the window
func FindWindowByTitle(title string) uintptr {
	ret, _, _ := syscall.Syscall(
		findWindowW,
		2,
		0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(title))),
		0,
	)
	return ret
}

func getCurrentProcessId() string {
	hwnd := GetForegroundWindow()
	pid := GetWindowThreadProcessID(hwnd)
	handle := GetOpenProcess(pid)
	path := GetWindowInformation(uintptr(handle))
	return path
}

func getProcessName() string {
	s := strings.Split(getCurrentProcessId(), "\\")
	str := strings.Split(s[len(s)-1], ".")
	return removeSPaces(str[0])
}

func trackProcesses(processName string, timeMap map[string]int64) {
	_, ok := timeMap[processName]
	if !ok && len(processName) > 1 {
		timeMap[processName] = 1
	} else {
		timeMap[processName] = timeMap[processName] + 1
	}
}

func initTimeUnitMap() (timeUnitMap map[string]time.Duration) {
	timeUnitMap = make(map[string]time.Duration, 3)
	timeUnitMap["MINUTES"] = time.Minute
	timeUnitMap["SECONDS"] = time.Second
	timeUnitMap["HOURS"] = time.Hour
	return
}

func getTimeToStop(endtime int, timeUnit string) (timeToStop time.Time) {
	timeUnitMap := initTimeUnitMap()
	d := time.Duration(time.Duration(endtime) * timeUnitMap[timeUnit])
	timeToStop = time.Now().Add(d)
	return
}

func trackProcessUntilEndTime(endtime int, timeUnit string) {
	timeToStop := getTimeToStop(endtime, timeUnit)
	log.Println(timeToStop)
	timeMap := make(map[string]int64, 0)
	var counter int64 = 0
	// for Flag && timeToStop.After(time.Now()) {
	for Flag {
		log.Println("Flag is ", Flag)
		trackProcesses(getProcessName(), timeMap)
		time.Sleep(1 * time.Second)
		if counter == 300 {
			// postDataToAnalyticsService(timeMap)
			writeStatsToFile(timeMap)
			counter = 0
		}
		counter++
	}
}

func getHomePath() (filePath string) {
	home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
	filePath = home + PROCESS_STATS_LOG
	return
}

func StartTracking() {
	arguments := os.Args[1:]
	var endtime int = 60
	timeUnit := "SECONDS"
	if len(arguments) > 1 {
		endtime, _ = strconv.Atoi(arguments[1])
		timeUnit = strings.ToUpper(arguments[2])
	}
	trackProcessUntilEndTime(endtime, timeUnit)
}

func writeStatsToFile(timeMap map[string]int64) {
	fPath := getHomePath()
	f, err := os.Create(fPath)
	if err != nil {
		log.Println("Initial err: ", err.Error())
	}
	defer f.Close()
	for k, v := range timeMap {
		// v, unit := getTimeUnitFromTimeInSeconds(v)
		res := fmt.Sprintf("%s,%d", k, v)
		_, e := f.WriteString(res + LineBreak)
		if e != nil {
			log.Println(e.Error())
		} else {
			log.Println("FILE GENERATED AT: ", fPath)
		}
	}
}

func getTimeUnitFromTimeInSeconds(v int64) (int64, string) {
	var unit string
	if v > 60 {
		unit = "Minutes"
		v = v / 60
	} else {
		unit = "Seconds"
	}
	return v, unit
}

func removeSPaces(processName string) string {
	byteArr := []byte(processName)
	var name string
	for _, v := range byteArr {
		if v != 0 {
			name = name + string(v)
		}
	}
	return name
}

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

func postDataToAnalyticsService(timeMap map[string]int64) {
	url := "http://localhost:8080/processes/"
	var jsonStr, err = json.Marshal(timeMap)
	checkError(err)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	checkError(err)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	checkError(err)
	defer resp.Body.Close()
	fmt.Println("response Status:", resp.Status)
}

func ptr(val interface{}) uintptr {
	switch val.(type) {
	case string:
		return uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(val.(string))))
	case int:
		return uintptr(val.(int))
	default:
		return uintptr(0)
	}
}
