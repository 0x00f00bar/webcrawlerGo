package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

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

	// handle incorrect cmd args like
	// ,, ,,, ,,value, ,,    ,, value  ,
	if strings.Contains(args, ",") {
		splitList := strings.Split(args, ",")
		for _, str := range splitList {
			if str != "" {
				str = strings.TrimSpace(str)
				argList = append(argList, str)
			}
		}
	} else {
		argList = append(argList, args)
	}
	return argList
}

// listenForSignals will listen on sigChan for [os.Signal] and quit on SIGINT and SIGTERM
// after calling cancel func.
func listenForSignals(
	cancel context.CancelFunc,
	sigChan chan os.Signal,
	queue *queue.UniqueQueue,
	loggers *loggers,
) {
	defer cancel()
	// quit := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	// wait for signal
	s := <-sigChan
	// clear queue
	queue.Clear()

	loggers.fileLogger.Println("=============== SHUTDOWN INITIATED ===============")
	loggers.fileLogger.Printf("%s signal received", s.String())
	loggers.fileLogger.Println("Waiting for crawlers to quit...")
}

func printBanner() {
	fmt.Println("")
	fmt.Println(cyanStyle.Render("Who are we?      : ") + redStyle.Render("web crawlers!"))
	fmt.Println(cyanStyle.Render("What do we want? : ") + redStyle.Render("To crawl the web!"))
	fmt.Println(redStyle.Margin(0, 0, 0, 2).Render(banner))
	fmt.Println(grayStyle.Render("v" + version))
}
