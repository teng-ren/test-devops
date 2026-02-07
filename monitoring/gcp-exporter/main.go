package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	// GKE Node metrics
	gkeNodeCPUUtilization = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gke_node_cpu_utilization",
			Help: "GKE node CPU utilization percentage",
		},
		[]string{"node_name", "cluster_name"},
	)

	gkeNodeMemoryUtilization = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gke_node_memory_utilization",
			Help: "GKE node memory utilization percentage",
		},
		[]string{"node_name", "cluster_name"},
	)

	gkeNodeDiskUtilization = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gke_node_disk_utilization",
			Help: "GKE node disk utilization percentage",
		},
		[]string{"node_name", "cluster_name"},
	)

	// GKE Container metrics
	gkeContainerCPUUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gke_container_cpu_usage_seconds",
			Help: "GKE container CPU usage in seconds",
		},
		[]string{"container_name", "pod_name", "namespace"},
	)

	gkeContainerMemoryUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gke_container_memory_usage_bytes",
			Help: "GKE container memory usage in bytes",
		},
		[]string{"container_name", "pod_name", "namespace"},
	)

	// GKE Pod metrics
	gkePodNetworkReceivedBytes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gke_pod_network_received_bytes_total",
			Help: "GKE pod network received bytes",
		},
		[]string{"pod_name", "namespace"},
	)

	gkePodNetworkSentBytes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gke_pod_network_sent_bytes_total",
			Help: "GKE pod network sent bytes",
		},
		[]string{"pod_name", "namespace"},
	)

	// Cluster-level metrics
	gkeClusterNodeCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gke_cluster_node_count",
			Help: "Number of nodes in the GKE cluster",
		},
		[]string{"cluster_name"},
	)

	gkeClusterPodCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gke_cluster_pod_count",
			Help: "Number of pods in the GKE cluster",
		},
		[]string{"cluster_name", "namespace"},
	)

	// Request metrics from Cloud Monitoring
	httpRequestCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gcp_http_request_count",
			Help: "HTTP request count from GCP Load Balancer",
		},
		[]string{"response_code_class"},
	)

	httpRequestLatency = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gcp_http_request_latency_ms",
			Help: "HTTP request latency in milliseconds",
		},
		[]string{"percentile"},
	)
)

func init() {
	prometheus.MustRegister(gkeNodeCPUUtilization)
	prometheus.MustRegister(gkeNodeMemoryUtilization)
	prometheus.MustRegister(gkeNodeDiskUtilization)
	prometheus.MustRegister(gkeContainerCPUUsage)
	prometheus.MustRegister(gkeContainerMemoryUsage)
	prometheus.MustRegister(gkePodNetworkReceivedBytes)
	prometheus.MustRegister(gkePodNetworkSentBytes)
	prometheus.MustRegister(gkeClusterNodeCount)
	prometheus.MustRegister(gkeClusterPodCount)
	prometheus.MustRegister(httpRequestCount)
	prometheus.MustRegister(httpRequestLatency)
}

type GCPExporter struct {
	client      *monitoring.MetricClient
	projectID   string
	clusterName string
}

func NewGCPExporter(projectID, clusterName string) (*GCPExporter, error) {
	ctx := context.Background()
	client, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create monitoring client: %v", err)
	}

	return &GCPExporter{
		client:      client,
		projectID:   projectID,
		clusterName: clusterName,
	}, nil
}

func (e *GCPExporter) Close() error {
	return e.client.Close()
}

func (e *GCPExporter) collectMetrics(ctx context.Context) error {
	now := time.Now()
	startTime := now.Add(-5 * time.Minute)

	// Collect node CPU utilization
	if err := e.collectNodeCPU(ctx, startTime, now); err != nil {
		log.Printf("Error collecting node CPU: %v", err)
	}

	// Collect node memory utilization
	if err := e.collectNodeMemory(ctx, startTime, now); err != nil {
		log.Printf("Error collecting node memory: %v", err)
	}

	// Collect container CPU usage
	if err := e.collectContainerCPU(ctx, startTime, now); err != nil {
		log.Printf("Error collecting container CPU: %v", err)
	}

	// Collect container memory usage
	if err := e.collectContainerMemory(ctx, startTime, now); err != nil {
		log.Printf("Error collecting container memory: %v", err)
	}

	// Collect network metrics
	if err := e.collectPodNetwork(ctx, startTime, now); err != nil {
		log.Printf("Error collecting pod network: %v", err)
	}

	// Collect cluster-level metrics (disabled - metrics not available)
	// if err := e.collectClusterMetrics(ctx, startTime, now); err != nil {
	// 	log.Printf("Error collecting cluster metrics: %v", err)
	// }

	return nil
}

func (e *GCPExporter) collectNodeCPU(ctx context.Context, startTime, endTime time.Time) error {
	req := &monitoringpb.ListTimeSeriesRequest{
		Name:   "projects/" + e.projectID,
		Filter: fmt.Sprintf(`metric.type="kubernetes.io/node/cpu/allocatable_utilization" AND resource.label.cluster_name="%s"`, e.clusterName),
		Interval: &monitoringpb.TimeInterval{
			EndTime:   timestamppb.New(endTime),
			StartTime: timestamppb.New(startTime),
		},
	}

	it := e.client.ListTimeSeries(ctx, req)
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		nodeName := resp.Resource.Labels["node_name"]
		clusterName := resp.Resource.Labels["cluster_name"]

		if len(resp.Points) > 0 {
			value := resp.Points[0].Value.GetDoubleValue()
			gkeNodeCPUUtilization.WithLabelValues(nodeName, clusterName).Set(value * 100)
		}
	}

	return nil
}

func (e *GCPExporter) collectNodeMemory(ctx context.Context, startTime, endTime time.Time) error {
	req := &monitoringpb.ListTimeSeriesRequest{
		Name:   "projects/" + e.projectID,
		Filter: fmt.Sprintf(`metric.type="kubernetes.io/node/memory/allocatable_utilization" AND resource.label.cluster_name="%s"`, e.clusterName),
		Interval: &monitoringpb.TimeInterval{
			EndTime:   timestamppb.New(endTime),
			StartTime: timestamppb.New(startTime),
		},
	}

	it := e.client.ListTimeSeries(ctx, req)
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		nodeName := resp.Resource.Labels["node_name"]
		clusterName := resp.Resource.Labels["cluster_name"]

		if len(resp.Points) > 0 {
			value := resp.Points[0].Value.GetDoubleValue()
			gkeNodeMemoryUtilization.WithLabelValues(nodeName, clusterName).Set(value * 100)
		}
	}

	return nil
}

func (e *GCPExporter) collectContainerCPU(ctx context.Context, startTime, endTime time.Time) error {
	req := &monitoringpb.ListTimeSeriesRequest{
		Name:   "projects/" + e.projectID,
		Filter: fmt.Sprintf(`metric.type="kubernetes.io/container/cpu/core_usage_time" AND resource.label.cluster_name="%s"`, e.clusterName),
		Interval: &monitoringpb.TimeInterval{
			EndTime:   timestamppb.New(endTime),
			StartTime: timestamppb.New(startTime),
		},
	}

	it := e.client.ListTimeSeries(ctx, req)
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		containerName := resp.Resource.Labels["container_name"]
		podName := resp.Resource.Labels["pod_name"]
		namespace := resp.Resource.Labels["namespace_name"]

		if len(resp.Points) > 0 {
			value := resp.Points[0].Value.GetDoubleValue()
			gkeContainerCPUUsage.WithLabelValues(containerName, podName, namespace).Set(value)
		}
	}

	return nil
}

func (e *GCPExporter) collectContainerMemory(ctx context.Context, startTime, endTime time.Time) error {
	req := &monitoringpb.ListTimeSeriesRequest{
		Name:   "projects/" + e.projectID,
		Filter: fmt.Sprintf(`metric.type="kubernetes.io/container/memory/used_bytes" AND resource.label.cluster_name="%s"`, e.clusterName),
		Interval: &monitoringpb.TimeInterval{
			EndTime:   timestamppb.New(endTime),
			StartTime: timestamppb.New(startTime),
		},
	}

	it := e.client.ListTimeSeries(ctx, req)
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		containerName := resp.Resource.Labels["container_name"]
		podName := resp.Resource.Labels["pod_name"]
		namespace := resp.Resource.Labels["namespace_name"]

		if len(resp.Points) > 0 {
			value := resp.Points[0].Value.GetInt64Value()
			gkeContainerMemoryUsage.WithLabelValues(containerName, podName, namespace).Set(float64(value))
		}
	}

	return nil
}

func (e *GCPExporter) collectPodNetwork(ctx context.Context, startTime, endTime time.Time) error {
	// Received bytes
	req := &monitoringpb.ListTimeSeriesRequest{
		Name:   "projects/" + e.projectID,
		Filter: fmt.Sprintf(`metric.type="kubernetes.io/pod/network/received_bytes_count" AND resource.label.cluster_name="%s"`, e.clusterName),
		Interval: &monitoringpb.TimeInterval{
			EndTime:   timestamppb.New(endTime),
			StartTime: timestamppb.New(startTime),
		},
	}

	it := e.client.ListTimeSeries(ctx, req)
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		podName := resp.Resource.Labels["pod_name"]
		namespace := resp.Resource.Labels["namespace_name"]

		if len(resp.Points) > 0 {
			value := resp.Points[0].Value.GetInt64Value()
			gkePodNetworkReceivedBytes.WithLabelValues(podName, namespace).Set(float64(value))
		}
	}

	// Sent bytes
	req.Filter = fmt.Sprintf(`metric.type="kubernetes.io/pod/network/sent_bytes_count" AND resource.label.cluster_name="%s"`, e.clusterName)
	it = e.client.ListTimeSeries(ctx, req)
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		podName := resp.Resource.Labels["pod_name"]
		namespace := resp.Resource.Labels["namespace_name"]

		if len(resp.Points) > 0 {
			value := resp.Points[0].Value.GetInt64Value()
			gkePodNetworkSentBytes.WithLabelValues(podName, namespace).Set(float64(value))
		}
	}

	return nil
}

func (e *GCPExporter) collectClusterMetrics(ctx context.Context, startTime, endTime time.Time) error {
	// Node count
	req := &monitoringpb.ListTimeSeriesRequest{
		Name:   "projects/" + e.projectID,
		Filter: fmt.Sprintf(`metric.type="kubernetes.io/node_count" AND resource.label.cluster_name="%s"`, e.clusterName),
		Interval: &monitoringpb.TimeInterval{
			EndTime:   timestamppb.New(endTime),
			StartTime: timestamppb.New(startTime),
		},
	}

	it := e.client.ListTimeSeries(ctx, req)
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		clusterName := resp.Resource.Labels["cluster_name"]

		if len(resp.Points) > 0 {
			value := resp.Points[0].Value.GetInt64Value()
			gkeClusterNodeCount.WithLabelValues(clusterName).Set(float64(value))
		}
	}

	// Pod count
	req.Filter = fmt.Sprintf(`metric.type="kubernetes.io/container/uptime" AND resource.cluster_name="%s"`, e.clusterName)
	it = e.client.ListTimeSeries(ctx, req)
	podCounts := make(map[string]map[string]int)

	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		namespace := resp.Resource.Labels["namespace_name"]
		podName := resp.Resource.Labels["pod_name"]

		if podCounts[e.clusterName] == nil {
			podCounts[e.clusterName] = make(map[string]int)
		}
		podKey := namespace + "/" + podName
		if _, exists := podCounts[e.clusterName][podKey]; !exists {
			podCounts[e.clusterName][podKey] = 1
		}
	}

	for cluster, pods := range podCounts {
		namespaceCounts := make(map[string]int)
		for podKey := range pods {
			// Extract namespace from "namespace/pod" format
			for ns := range namespaceCounts {
				if len(podKey) > len(ns) && podKey[:len(ns)] == ns {
					namespaceCounts[ns]++
					break
				}
			}
		}
		total := len(pods)
		gkeClusterPodCount.WithLabelValues(cluster, "all").Set(float64(total))
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	projectID := getEnv("GCP_PROJECT_ID", "dop-assignment-team1")
	clusterName := getEnv("GKE_CLUSTER_NAME", "ai-model-tester-2")
	port := getEnv("PORT", "8888")

	exporter, err := NewGCPExporter(projectID, clusterName)
	if err != nil {
		log.Fatalf("Failed to create GCP exporter: %v", err)
	}
	defer exporter.Close()

	// Start metrics collection in background
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		log.Printf("Metrics collection loop started")
		defer ticker.Stop()

		for {
			ctx := context.Background()
			if err := exporter.collectMetrics(ctx); err != nil {
				log.Printf("Error collecting metrics: %v", err)
			}
			<-ticker.C
		}
	}()

	// Expose metrics endpoint
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Printf("Starting GCP monitoring exporter on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
