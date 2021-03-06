package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/justinbarrick/hone/pkg/cache"
	"github.com/justinbarrick/hone/pkg/cache/s3"
	"github.com/justinbarrick/hone/pkg/executors/local"
	"github.com/justinbarrick/hone/pkg/logger"
)

func main() {
	s3 := s3cache.S3Cache{
		Bucket:    os.Getenv("S3_BUCKET"),
		Endpoint:  os.Getenv("S3_ENDPOINT"),
		AccessKey: os.Getenv("S3_ACCESS_KEY"),
		SecretKey: os.Getenv("S3_SECRET_KEY"),
	}

	logger.InitLogger(0, nil)

	if err := s3.Init(); err != nil {
		log.Fatal(err)
	}

	cacheKey := os.Getenv("CACHE_KEY")

	cacheManifest, err := s3.LoadCacheManifest("srcs_manifests", cacheKey)
	if err != nil {
		log.Fatal(err)
	}

	for _, entry := range cacheManifest {
		err := s3.Get("srcs", entry)
		if err != nil {
			log.Fatal(err)
		}
		err = entry.SyncAttrs()
		if err != nil {
			log.Fatal(err)
		}
		logger.Printf("Loaded %s from cache (%s).", entry.Filename, s3.Name())
	}

	outputs := []string{}
	err = json.Unmarshal([]byte(os.Getenv("OUTPUTS")), &outputs)
	if err != nil {
		log.Fatal(err)
	}

	os.Unsetenv("S3_BUCKET")
	os.Unsetenv("S3_ENDPOINT")
	os.Unsetenv("S3_ACCESS_KEY")
	os.Unsetenv("S3_SECRET_KEY")
	os.Unsetenv("CACHE_KEY")
	os.Unsetenv("OUTPUTS")

	if err = local.Exec(os.Args[1:], local.ParseEnv(os.Environ())); err != nil {
		log.Fatal(err)
	}

	if _, err = cache.DumpOutputs(cacheKey, &s3, outputs); err != nil {
		log.Fatal(err)
	}
	logger.Printf("Dumped outputs to cache (%s).", s3.Name())
}
