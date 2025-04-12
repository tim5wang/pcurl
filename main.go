package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/antlabs/pcurl"
	"github.com/urfave/cli/v2"
	"golang.org/x/time/rate"
)

var live bool
var data string
var curl string
var limitRate int
var thread int
var display int
var dataSeparator string

var counter int64
var dataParsed []map[string]string

const indexFiled = "_INDEX"

func main() {
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:        "data",
			Value:       "",
			Aliases:     []string{"d"},
			Usage:       "data for pressure, first line is head line, -d ./data/path.csv",
			Destination: &data,
		},
		&cli.StringFlag{
			Name:        "curl",
			Value:       "",
			Usage:       "curl template, -c 'curl google.com'",
			Aliases:     []string{"c"},
			Destination: &curl,
		},
		&cli.IntFlag{
			Name:        "limit_rate",
			Value:       10,
			Aliases:     []string{"r"},
			Usage:       "rate to pressure",
			Destination: &limitRate,
		},
		&cli.IntFlag{
			Name:        "thread_num",
			Value:       10,
			Usage:       "thread to send msg",
			Aliases:     []string{"n"},
			Destination: &thread,
		},
		&cli.IntFlag{
			Name:        "show",
			Value:       -1,
			Usage:       "display result per [show] request",
			Aliases:     []string{"s"},
			Destination: &display,
		},
		&cli.StringFlag{
			Name:        "data_separator",
			Value:       ",",
			Usage:       "--data_separator '|', default is ,",
			Destination: &dataSeparator,
		},
	}
	app.Commands = []*cli.Command{
		{
			Name:    "press",
			Aliases: []string{"p"},
			Usage:   "",
			Action: func(c *cli.Context) error {
				if file, err := os.OpenFile(curl, os.O_RDWR, 0666); err == nil {
					body, e := io.ReadAll(file)
					if e != nil {
						e = fmt.Errorf("%v is can not parse as curl comand nor read as file, read err:%v", curl, e)
						return e
					}
					_, e = pcurl.ParseAndRequest(string(body))
					if e != nil {
						e = fmt.Errorf("parse curl from %v error: %w", curl, e)
						return e
					}
					curl = string(body)
				}

				live = true
				go func() {
					pressure(data, curl, limitRate, thread)
				}()
				sigterm := make(chan os.Signal, 1)
				signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)
				select {
				case <-sigterm:
					live = false
					log.Printf("\n=== Pressure Test Summary ===")
					log.Printf("Total Requests: %d", pressureStats.totalRequests)
					log.Printf("Success: %d", pressureStats.successCount)
					log.Printf("Failure: %d", pressureStats.failureCount)
					log.Printf("Success Rate: %.2f%%", float64(pressureStats.successCount)/float64(pressureStats.totalRequests)*100)
					log.Printf("Duration: %v", time.Since(pressureStats.startTime))
					log.Printf("Requests Per Second: %.2f", float64(pressureStats.totalRequests)/time.Since(pressureStats.startTime).Seconds())
				}
				return nil
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Println(err)
	}
	//
	// app.Run([]string{
	// 	"", "-c", `./requests/curl`, "-d", "./requests/data.txt", "-s", "20", "p",
	// })

}

type stats struct {
	totalRequests int64
	successCount  int64
	failureCount  int64
	startTime     time.Time
}

var pressureStats stats

func pressure(filepath, curl string, r int, thread int) {
	pressureStats = stats{startTime: time.Now()}
	err := parseData(filepath)
	if err != nil {
		log.Printf("parse data error %v", err)
		return
	}
	limit := rate.NewLimiter(rate.Limit(r), 1)

	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < thread; i++ {
		wg.Add(1)
		go func() {
			for live {
				limit.Wait(ctx)
				c := atomic.AddInt64(&counter, 1)
				reqCurl := replaceCurl(curl, c)
				req, e := pcurl.ParseAndRequest(reqCurl)
				if e != nil {
					fmt.Printf("err:%s\n", e)
					continue
				}
				resp, e := http.DefaultClient.Do(req)
				atomic.AddInt64(&pressureStats.totalRequests, 1)
				if e != nil {
					atomic.AddInt64(&pressureStats.failureCount, 1)
					fmt.Printf("err:%s\n", e)
					continue
				}
				atomic.AddInt64(&pressureStats.successCount, 1)
				defer resp.Body.Close()
				if display > 0 && c%int64(display) == 0 {
					result, _ := io.ReadAll(resp.Body)
					var caseNum int64
					if len(dataParsed) > 0 {
						caseNum = c % int64(len(dataParsed))
					}
					log.Printf("case=%v,code=%v,req=%v,rsp=%v", caseNum, resp.Status, reqCurl, string(result))
					io.Copy(os.Stdout, resp.Body)
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
	duration := time.Since(pressureStats.startTime)
	log.Printf("\n=== Pressure Test Summary ===")
	log.Printf("Total Requests: %d", pressureStats.totalRequests)
	log.Printf("Success: %d", pressureStats.successCount)
	log.Printf("Failure: %d", pressureStats.failureCount)
	log.Printf("Success Rate: %.2f%%", float64(pressureStats.successCount)/float64(pressureStats.totalRequests)*100)
	log.Printf("Duration: %v", duration)
	log.Printf("Requests Per Second: %.2f", float64(pressureStats.totalRequests)/duration.Seconds())
}

func parseData(filepath string) error {
	if filepath == "" {
		return nil
	}
	file, e := os.OpenFile(filepath, os.O_RDWR, 0666)
	if e != nil {
		e = fmt.Errorf("open %v error: %w", filepath, e)
		return e
	}
	var ln int
	var heads = []string{indexFiled}
	buf := bufio.NewReader(file)
	for {
		line, err := buf.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return err
			}
		}
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		ln++
		dataLine := strings.Split(line, dataSeparator)
		if ln == 1 {
			for _, h := range dataLine {
				h = strings.TrimSpace(h)
				h = strings.Replace(h, "\uFEFF", "", 1)
				heads = append(heads, h)
			}
			continue
		}
		row := make(map[string]string)
		for i, v := range dataLine {
			if i >= len(heads)-1 {
				continue
			}
			row[indexFiled] = fmt.Sprintf("%v", ln)
			row[heads[i+1]] = v
		}
		dataParsed = append(dataParsed, row)
	}
	log.Printf("%v data parsed, head is: %v", ln, heads)
	return nil
}

func replaceCurl(c string, counter int64) string {
	if len(dataParsed) == 0 {
		return c
	}
	i := counter % int64(len(dataParsed))
	row := dataParsed[i]
	for k, v := range row {
		kk := fmt.Sprintf(`{{%v}}`, k)
		c = strings.ReplaceAll(c, kk, v)
	}
	return c
}
