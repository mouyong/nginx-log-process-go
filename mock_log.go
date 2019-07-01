package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"
)

func main() {
	var filePath string
	var verbose bool

	flag.StringVar(&filePath, "file", "./access.log", "-file access.log")
	flag.BoolVar(&verbose, "v", false, "show verbose info")
	flag.Parse()

	var file *os.File
	var err error
	var fileLock sync.RWMutex

	fileInfo, err := os.Stat(filePath)
	if err != nil{
		fmt.Println("os.Stat error: ", err.Error())

		file, err = os.Create(filePath)

		if err != nil {
			fmt.Println("create file error: ", err.Error())
		} else {
			fmt.Println("create file success: ", filePath)
		}
	} else {
		fmt.Println("fileInfo", fileInfo)

		time.After(3 * time.Second)

		file, err = os.OpenFile(filePath, os.O_APPEND, 0666)
		if err != nil {
			fmt.Println("open file error: ", err.Error())
			os.Exit(-1)
		}
	}
	defer file.Close()

	var ipStr []string
	for i:=0; i<4 ; i++ {
		intNum, _ := rand.Int(rand.Reader, big.NewInt(255))
		ipStr = append(ipStr, intNum.String())
	}

	for {

		ipS := strings.Join(ipStr, ".")

		status, _ := rand.Int(rand.Reader, big.NewInt(599))
		bodyBytesSend, _ := rand.Int(rand.Reader, big.NewInt(2000))

		timeStr := time.Now().Format("02/Jan/2006:15:04:05 +0800")
		nginxLogStr := ipS +` - - [` + timeStr + `] "GET /api/child_star/query?classify=2&page=1&page_size=18 HTTP/1.1" `+ status.String() +` `+ bodyBytesSend.String() +` "-" "okhttp/3.10.0"`
		nginxLogStr = fmt.Sprintln(nginxLogStr)

		for i:=0;i<4 ; i++ {
			fileLock.Lock()
			_, err := file.WriteString(nginxLogStr);
			fileLock.Unlock()

			if verbose == true {
				fmt.Print(nginxLogStr)
			}

			if err != nil {
				fmt.Println("write log error: %s", err.Error())
				os.Exit(-1)
			}
		}

		time.Sleep(1 * time.Second)
	}
}
