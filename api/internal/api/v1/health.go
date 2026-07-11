package v1

import (
	"bufio"
	"context"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/artur-oliveira/ctech-dfe/api/internal/awsclient"
	"github.com/artur-oliveira/ctech-dfe/api/internal/cache"
	"github.com/artur-oliveira/ctech-dfe/api/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/gofiber/fiber/v3"
)

var startTime = time.Now()

const checkTimeout = 2 * time.Second

type healthEntry struct {
	ComponentName   string  `json:"componentName"`
	MeasurementName string  `json:"measurementName"`
	ComponentType   string  `json:"componentType"`
	ObservedValue   float64 `json:"observedValue"`
	ObservedUnit    string  `json:"observedUnit"`
	Status          string  `json:"status"`
	Time            string  `json:"time"`
}

type healthResponse struct {
	Status      string                 `json:"status"`
	Version     string                 `json:"version"`
	ReleaseID   string                 `json:"releaseId"`
	ServiceID   string                 `json:"serviceId"`
	Description string                 `json:"description"`
	Checks      map[string]healthEntry `json:"checks"`
}

func RegisterHealth(router fiber.Router, cacheBackend cache.Backend, clients *awsclient.Clients, cfg *config.Config) {
	router.Get("/health-check", func(c fiber.Ctx) error {
		now := time.Now().UTC()
		nowStr := now.Format(time.RFC3339Nano)

		ctx, cancel := context.WithTimeout(c.Context(), checkTimeout)
		defer cancel()

		dynamo := checkDynamoDB(ctx, clients.DynamoDB, nowStr)
		s3c := checkS3(ctx, clients.S3, cfg.S3BucketDocuments, nowStr)
		snsc := checkSNS(ctx, clients.SNS, cfg.WorkerTopicARN, nowStr)
		sqsResults := checkSQS(ctx, clients.SQS, "sqs_results", cfg.ResultsQueueURL, nowStr)
		sqsDist := checkSQS(ctx, clients.SQS, "sqs_distribution", cfg.DistributionQueueURL, nowStr)
		cachec := checkCache(ctx, cacheBackend, nowStr)
		cpu := checkCPU(nowStr)
		mem := checkMemory(nowStr)

		uptime := healthEntry{
			ComponentName:   "server",
			MeasurementName: "uptime",
			ComponentType:   "system",
			ObservedValue:   time.Since(startTime).Seconds(),
			ObservedUnit:    "second",
			Status:          "pass",
			Time:            nowStr,
		}

		loadBearing := []string{dynamo.Status, s3c.Status, snsc.Status, sqsResults.Status, sqsDist.Status, cpu.Status, mem.Status}
		overall := "pass"
		statusCode := fiber.StatusOK
		for _, s := range loadBearing {
			if s == "fail" {
				overall = "fail"
				statusCode = fiber.StatusServiceUnavailable
				break
			}
		}
		if overall != "fail" {
			for _, s := range append(loadBearing, cachec.Status) {
				if s == "warn" {
					overall = "warn"
					statusCode = 207
					break
				}
			}
		}

		return c.Status(statusCode).JSON(healthResponse{
			Status:      overall,
			Version:     "/v1.0",
			ReleaseID:   cfg.AppVersion,
			ServiceID:   "CTech DF-e",
			Description: "Health check details for CTech DF-e API",
			Checks: map[string]healthEntry{
				"uptime":           uptime,
				"dynamodb":         dynamo,
				"s3":               s3c,
				"sns":              snsc,
				"sqs_results":      sqsResults,
				"sqs_distribution": sqsDist,
				"cpu":              cpu,
				"memory":           mem,
				"cache":            cachec,
			},
		})
	})
}

func checkDynamoDB(ctx context.Context, db *dynamodb.Client, nowStr string) healthEntry {
	t0 := time.Now()
	_, err := db.ListTables(ctx, &dynamodb.ListTablesInput{Limit: aws.Int32(1)})
	ms := float64(time.Since(t0).Milliseconds())
	st := "pass"
	if err != nil {
		st = "fail"
	}
	return healthEntry{"dynamodb", "responseTime", "datastore:database", ms, "ms", st, nowStr}
}

func checkS3(ctx context.Context, s3c *s3.Client, bucket, nowStr string) healthEntry {
	t0 := time.Now()
	_, err := s3c.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)})
	ms := float64(time.Since(t0).Milliseconds())
	st := "pass"
	if err != nil {
		st = "warn"
	}
	return healthEntry{"s3", "responseTime", "datastore:storage", ms, "ms", st, nowStr}
}

func checkSNS(ctx context.Context, snsc *sns.Client, topicARN, nowStr string) healthEntry {
	t0 := time.Now()
	st := "pass"
	if topicARN == "" {
		return healthEntry{"sns", "responseTime", "component:topic", -1, "ms", "warn", nowStr}
	}
	_, err := snsc.GetTopicAttributes(ctx, &sns.GetTopicAttributesInput{
		TopicArn: aws.String(topicARN),
	})
	ms := float64(time.Since(t0).Milliseconds())
	if err != nil {
		st = "warn"
	}
	return healthEntry{"sns", "responseTime", "component:topic", ms, "ms", st, nowStr}
}

func checkSQS(ctx context.Context, sqsc *sqs.Client, name, queueURL, nowStr string) healthEntry {
	t0 := time.Now()
	st := "pass"
	if queueURL == "" {
		return healthEntry{name, "responseTime", "component:queue", -1, "ms", "warn", nowStr}
	}
	_, err := sqsc.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl: aws.String(queueURL),
	})
	ms := float64(time.Since(t0).Milliseconds())
	if err != nil {
		st = "warn"
	}
	return healthEntry{name, "responseTime", "component:queue", ms, "ms", st, nowStr}
}

func checkCache(ctx context.Context, cb cache.Backend, nowStr string) healthEntry {
	if cb == nil {
		return healthEntry{"cache", "responseTime", "datastore:cache", -1, "ms", "warn", nowStr}
	}
	t0 := time.Now()
	err := cb.Ping(ctx)
	ms := float64(time.Since(t0).Milliseconds())
	st := "pass"
	if err != nil {
		st = "warn"
	}
	return healthEntry{"cache", "responseTime", "datastore:cache", ms, "ms", st, nowStr}
}

func checkCPU(nowStr string) healthEntry {
	pct := cpuPercent()
	st := "pass"
	if pct < 0 || pct > 90 {
		st = "warn"
	}
	return healthEntry{"cpu", "utilization", "system", pct, "percent", st, nowStr}
}

func checkMemory(nowStr string) healthEntry {
	pct := memoryPercent()
	st := "pass"
	if pct < 0 || pct > 90 {
		st = "warn"
	}
	return healthEntry{"memory", "utilization", "system", pct, "percent", st, nowStr}
}

func cpuPercent() float64 {
	if runtime.GOOS != "linux" {
		return -1
	}
	f, err := os.Open("/proc/stat")
	if err != nil {
		return -1
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return -1
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return -1
	}
	var vals []int64
	for _, s := range fields[1:] {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			break
		}
		vals = append(vals, v)
	}
	if len(vals) < 4 {
		return -1
	}
	idle := vals[3]
	total := int64(0)
	for _, v := range vals {
		total += v
	}
	if total == 0 {
		return -1
	}
	return roundOne(100.0 * float64(total-idle) / float64(total))
}

func memoryPercent() float64 {
	if runtime.GOOS != "linux" {
		return -1
	}
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return -1
	}
	defer f.Close()
	info := map[string]int64{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		valStr := strings.Fields(strings.TrimSpace(parts[1]))
		if len(valStr) == 0 {
			continue
		}
		v, err := strconv.ParseInt(valStr[0], 10, 64)
		if err == nil {
			info[key] = v
		}
	}
	total, ok1 := info["MemTotal"]
	available, ok2 := info["MemAvailable"]
	if !ok1 || !ok2 || total == 0 {
		return -1
	}
	return roundOne(100.0 * float64(total-available) / float64(total))
}

func roundOne(v float64) float64 {
	return float64(int64(v*10+0.5)) / 10
}
