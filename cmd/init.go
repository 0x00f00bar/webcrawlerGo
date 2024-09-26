package main

import "os"

var logFolderName = "logs"

func init() {

	// make a folder to store logs
	if _, err := os.Stat(logFolderName); os.IsNotExist(err) {
		err = os.Mkdir(logFolderName, 0755)
		if err != nil {
			panic(err)
		}
	}
}
