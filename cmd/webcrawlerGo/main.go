package main

import (
	"context"
	"os"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	"github.com/0x00f00bar/webcrawlerGo/internal"
	"github.com/0x00f00bar/webcrawlerGo/models"
	"github.com/0x00f00bar/webcrawlerGo/models/psql"
	"github.com/0x00f00bar/webcrawlerGo/models/sqlite"
	"github.com/0x00f00bar/webcrawlerGo/queue"
)

var (
	version = "0.9.0"
	banner  = `
                __                             __          ______    
 _      _____  / /_  ______________ __      __/ /__  _____/ ____/___ 
| | /| / / _ \/ __ \/ ___/ ___/ __ '/ | /| / / / _ \/ ___/ / __/ __ \
| |/ |/ /  __/ /_/ / /__/ /  / /_/ /| |/ |/ / /  __/ /  / /_/ / /_/ /
|__/|__/\___/_.___/\___/_/   \__,_/ |__/|__/_/\___/_/   \____/\____/ 
                                                                     `

	exitCode           int
	currentLogFileName string
)

func main() {
	printBanner()

	defer func() {
		os.Exit(exitCode)
	}()

	v := internal.NewValidator()

	// parse cmd flags; exit if flags invalid
	cmdArgs := parseCmdFlags(v)

	// init file and os.Stdout logger
	f, loggers := initialiseLoggers(cmdArgs.verbose)
	defer f.Close()
	f.Write([]byte(banner + "\n" + "v" + version + "\n\n"))

	if !cmdArgs.runserver {
		logCmdArgs(cmdArgs, f)
	}

	// init and test db
	driverName, dbConns, err := getDBConnections(*cmdArgs.dbDSN, loggers)
	if err != nil {
		exitCode = 1
		loggers.fileLogger.Println(err)
		return
	}
	loggers.multiLogger.Println("DB connection OK.")
	defer dbConns.Close()

	// if the driver used is sqlite3 consolidate the WAL journal to db
	// before closing connection
	defer sqlite.ExecWALCheckpoint(driverName, dbConns.writer)

	// create cancel context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	// init models
	var m models.Models

	// get postgres models and initialise database tables
	if driverName == psql.DriverNamePgSQL {
		psqlModels := psql.NewPsqlDB(dbConns.writer)
		err = psqlModels.InitDatabase(ctx, dbConns.writer)
		if err != nil {
			exitCode = 1
			loggers.multiLogger.Println(err)
			return
		}
		m.URLs = psqlModels.URLModel
		m.Pages = psqlModels.PageModel
	}
	// get sqlite3 models and initialise database tables
	if driverName == sqlite.DriverNameSQLite {
		sqliteModels := sqlite.NewSQLiteDB(dbConns.reader, dbConns.writer)
		err = sqliteModels.InitDatabase(ctx, dbConns.writer)
		if err != nil {
			exitCode = 1
			loggers.multiLogger.Println(err)
			return
		}
		m.URLs = sqliteModels.URLModel
		m.Pages = sqliteModels.PageModel
	}

	// init queue & push base url
	q := queue.NewQueue()

	// chanel to get os signals
	quit := make(chan os.Signal, 1)

	go listenForSignals(cancel, quit, q, loggers)

	if cmdArgs.runserver {
		app := webapp{
			Models:       &m,
			Loggers:      loggers,
			OSSigCtx:     ctx,
			CrawlerQueue: q,
		}

		err = app.serve(ctx, quit)
		if err != nil {
			exitCode = 3
			app.Loggers.multiLogger.Println(err)
		}
		return
	}

	if cmdArgs.dbToDisk {
		err = saveDbContentToDisk(
			ctx,
			m.Pages,
			cmdArgs.baseURL,
			cmdArgs.savePath,
			cmdArgs.cutOffDate,
			cmdArgs.markedURLs,
			loggers,
		)
		if err != nil {
			exitCode = 1
			loggers.multiLogger.Printf("Error while saving to disk: %v\n", err)
		} else {
			loggers.multiLogger.Println("Transfer completed")
		}
		return
	}

	err = beginCrawl(ctx, cmdArgs, quit, q, &m, loggers)
	if err != nil {
		loggers.multiLogger.Println(err)
		return
	}
}
