package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/0x00f00bar/webcrawlerGo/queue"
)

// getMarkedURLS returns a slice of marked urls starting with '/'
func getMarkedURLS(mURLStr string) []string {
	// return empty slice when no marked urls
	markedURLs := seperateCmdArgs(mURLStr)

	// add leading '/' if not present
	for i, mUrl := range markedURLs {
		mUrl = strings.TrimSpace(mUrl)
		if mUrl[0] != '/' {
			mUrl = "/" + mUrl
		}
		markedURLs[i] = mUrl
	}

	return markedURLs
}

// seperateCmdArgs returns string slice of comma seperated cmd args
func seperateCmdArgs(args string) []string {
	argList := []string{}
	args = strings.TrimSpace(args)

	if args == "" {
		return argList
	}

	if strings.Contains(args, ",") {
		argList = strings.Split(args, ",")
	} else {
		argList = append(argList, args)
	}
	return argList
}

func listenForSignals(cancel context.CancelFunc, queue *queue.UniqueQueue, logger *log.Logger) {
	defer cancel()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	// wait for signal
	s := <-quit
	// clear queue
	queue.Clear()

	timeOut := 3 * time.Second

	logger.Println("=============== SHUTDOWN INITIATED ===============")
	logger.Printf("%s signal received", s.String())
	logger.Printf("Will shutdown in %s\n", timeOut.String())
	time.Sleep(timeOut)
}

func printBanner() {
	fmt.Printf(Cyan + "\nWho are we?      : " + Red + "web crawlers!" + Reset)
	fmt.Printf(Cyan + "\nWhat do we want? : " + Red + "To crawl the web!" + Reset)
	fmt.Println(Red + banner + Reset)
	fmt.Printf(Cyan+"v%s\n\n"+Reset, version)
}
