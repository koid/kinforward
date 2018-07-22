package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/fluent/fluent-logger-golang/fluent"
	"github.com/twinj/uuid"
	"github.com/twitchscience/kinsumer"

	"github.com/koid/kinforward/dogstatsd"
)

var (
	kinesisStreamName     string
	checkpointTablePrefix string
	tagKey                string

	fluentTagPrefix string
	fluentSocket    string
	fluentHost      = "localhost"
	fluentPort      = 24224

	dogStatsdAddr string
	dogStatsdTags []string

	clientName string
)

func init() {
	kinesisStreamName = os.Getenv("KINESIS_STREAM_NAME")
	if len(kinesisStreamName) == 0 {
		log.Fatalln("env KINESIS_STREAM_NAME is required")
	}
	checkpointTablePrefix = os.Getenv("CHECKPOINT_TABLE_PREFIX")
	if len(checkpointTablePrefix) == 0 {
		log.Fatalln("env CHECKPOINT_TABLE_PREFIX is required")
	}

	tagKey = os.Getenv("TAG_KEY")

	fluentTagPrefix = os.Getenv("FLUENT_TAG_PREFIX")
	fluentSocket = os.Getenv("FLUENT_SOCKET")
	if len(os.Getenv("FLUENT_HOST")) != 0 {
		fluentHost = os.Getenv("FLUENT_HOST")
	}
	if len(os.Getenv("FLUENT_PORT")) != 0 {
		fluentPort, _ = strconv.Atoi(os.Getenv("FLUENT_PORT"))
	}

	dogStatsdAddr = os.Getenv("DOGSTATSD_ADDR")
	dogStatsdTags = strings.Split(os.Getenv("DOGSTATSD_TAGS"), ",")

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}

	pid := os.Getpid()
	clientName = fmt.Sprintf("%s:%d:%s", hostname, pid, uuid.NewV4().String())
}

func initFluentLogger(retry int) *fluent.Fluent {
	var cfg fluent.Config
	if len(fluentSocket) > 0 {
		cfg = fluent.Config{
			MarshalAsJSON:    true,
			TagPrefix:        fluentTagPrefix,
			FluentNetwork:    "unix",
			FluentSocketPath: fluentSocket,
		}
	} else {
		cfg = fluent.Config{
			MarshalAsJSON: true,
			TagPrefix:     fluentTagPrefix,
			FluentHost:    fluentHost,
			FluentPort:    fluentPort,
		}
	}


	_l, err := fluent.New(cfg)
	if err != nil {
		if retry == 0 {
			log.Fatalf("fluent.New returned error: %v", err)
		}
		log.Println("Waiting for fluentd...")
		time.Sleep(3 * time.Second)
		return initFluentLogger(retry-1)
	}
	return _l
}

func initDogStatsd() *dogstatsd.Statsd {
	dogStatsdTags = append(dogStatsdTags, []string{
		fmt.Sprintf("streamname:%s", kinesisStreamName),
		fmt.Sprintf("app:%s", checkpointTablePrefix),
		fmt.Sprintf("client:%s", clientName),
	}...)
	_s, err := dogstatsd.New(dogStatsdAddr, dogStatsdTags)
	if err != nil {
		log.Fatalf("dogstatsd.New returned error: %v", err)
	}
	return _s
}

var (
	records chan []byte
	wg      sync.WaitGroup
	k       *kinsumer.Kinsumer
	l       *fluent.Fluent
)

func main() {
	var (
		stats kinsumer.StatReceiver = &kinsumer.NoopStatReceiver{}
		err   error
	)

	// fluent
	retry := 5
	l = initFluentLogger(retry)
	defer l.Close()

	// statsd
	if len(dogStatsdAddr) > 0 {
		stats = initDogStatsd()
	}

	// kinsumer
	sess := session.Must(session.NewSession(aws.NewConfig()))
	k, err := kinsumer.NewWithSession(
		sess,
		kinesisStreamName,
		checkpointTablePrefix,
		clientName,
		kinsumer.NewConfig().WithStats(stats),
	)
	if err != nil {
		log.Fatalf("kinsumer.NewWithSession returned error: %v", err)
	}

	err = k.Run()
	if err != nil {
		log.Fatalf("kinsumer.Kinsumer.Run returned error: %v", err)
	}

	// start consume
	records = make(chan []byte)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			record, err := k.Next()
			if err != nil {
				log.Fatalf("k.Next returned error: %v", err)
			}
			if record != nil {
				records <- record
			} else {
				return
			}
		}
	}()

	var totalConsumed int64
	t := time.NewTicker(3 * time.Second)
	defer t.Stop()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT)

	defer func() {
		log.Println("Total records consumed", totalConsumed)
	}()

	for {
		select {
		case <-sigc:
			k.Stop()
			return
		case data, ok := <-records:
			if !ok {
				log.Println("Failed to receive record.")
				continue
			}
			var message map[string]interface{}
			if err := json.Unmarshal(data, &message); err != nil {
				log.Printf("Failed to unmarshal: %v", err)
				continue
			}
			var tag string
			if tagKey != "" {
				if val, ok := message[tagKey].(string); ok {
					tag = val
				} else {
					tag = "unknown"
				}
			} else {
				tag = "default"
			}
			if err := l.Post(tag, message); err != nil {
				log.Fatalf("Failed to post: %v", err)
			}
			totalConsumed++
		case <-t.C:
			log.Printf("Consumed %v", totalConsumed)
		}
	}

	wg.Wait()
}
