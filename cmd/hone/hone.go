package main


import (
	"github.com/justinbarrick/hone/pkg/cache"
	"github.com/justinbarrick/hone/pkg/config"
	"github.com/justinbarrick/hone/pkg/executors"
	"github.com/justinbarrick/hone/pkg/executors/docker"
	"github.com/justinbarrick/hone/pkg/graph"
	"github.com/justinbarrick/hone/pkg/job"
	"github.com/justinbarrick/hone/pkg/events"
	"github.com/justinbarrick/hone/pkg/logger"
	"github.com/justinbarrick/hone/pkg/scm"
	"github.com/justinbarrick/hone/pkg/reporting"
	"fmt"
	"log"
	"io"
	"os"
	"path/filepath"
)


func main() {
	honePath := "Honefile"
	target := "all"

	if len(os.Args) == 2 {
		target = os.Args[1]
	} else if len(os.Args) == 3 {
		honePath = os.Args[1]
		target = os.Args[2]
	}

	logger.InitLogger(0, nil)

	config, err := config.Unmarshal(honePath)
	if err != nil {
		log.Fatal(err)
	}

	scms, err := scm.InitSCMs(config.SCM, config.Env)
	if err != nil {
		log.Fatal(err)
	}

	report, err := reporting.New(target, scms, config.Cache.S3)
	if err != nil {
		log.Fatal(err)
	}

	if err = scm.BuildStarted(scms); err != nil {
		logger.Errorf("Error initializing SCMs: %s", err)
		report.Exit(err)
	}

	g, err := graph.NewJobGraph(config.GetJobs())
	if err != nil {
		logger.Errorf("Error initializing job graph: %s", err)
		report.Exit(err)
	}

	longest, errs := g.LongestTarget(target)
	if len(errs) != 0 {
		report.Exit(errs...)
	}

	callback := func(j *job.Job) error {
		return executors.Run(config, j)
	}

	callback = events.EventCallback(config.Env, callback)

	fileCache := config.Cache.File
	if err = fileCache.Init(); err != nil {
		logger.Errorf("Error initializing file cache: %s", err)
		report.Exit(err)
	}

	var logWriter io.WriteCloser
	var logUrl string

	if config.Cache.S3 != nil && !config.Cache.S3.Disabled {
		if err = config.Cache.S3.Init(); err != nil {
			logger.Errorf("Error initializing S3: %s", err)
			report.Exit(err)
		}
		callback = cache.CacheJob(config.Cache.S3, callback)

		path := filepath.Join(report.GitCommit, fmt.Sprintf("%d.log", report.StartTime.Unix()))
		logWriter, logUrl, err = config.Cache.S3.Writer("logs", path)
		if err != nil {
			logger.Errorf("Error writing logs: %s", err)
			report.Exit(err)
		}
	}

	logger.InitLogger(longest, logWriter)

	callback = report.ReportJob(cache.CacheJob(fileCache, callback))

	config.DockerConfig = &docker.DockerConfig{}
	if err := config.DockerConfig.Init(); err != nil {
		logger.Errorf("Error initializing Docker: %s", err)
		report.Exit(err)
	}

	errs = g.ResolveTarget(target, logger.LogJob(callback))

	if logUrl != "" {
		logger.Printf("Logs available: %s", logUrl)
	}

	report.Final(errs...)

	if logUrl != "" {
		err = logWriter.Close()
		if err != nil {
			log.Printf("Error uploading logs: %s", err)
		}
	}

	config.DockerConfig.Cleanup()
	os.Exit(len(errs))
}
