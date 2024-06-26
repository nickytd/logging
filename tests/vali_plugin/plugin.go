// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllertest"

	"github.com/gardener/logging/pkg/config"
	"github.com/gardener/logging/pkg/valiplugin"
	plugintestclient "github.com/gardener/logging/tests/vali_plugin/plugintest/client"
	plugintestcluster "github.com/gardener/logging/tests/vali_plugin/plugintest/cluster"
	plugintestconfig "github.com/gardener/logging/tests/vali_plugin/plugintest/config"
	"github.com/gardener/logging/tests/vali_plugin/plugintest/input"
	"github.com/gardener/logging/tests/vali_plugin/plugintest/matcher"
)

const (
	numberOfClusters              = 100
	simulatesShootNamespacePrefix = "shoot--logging--test-"
)

var (
	valiPluginConfiguration config.Config
	testClient              *plugintestclient.BlackBoxTestingValiClient
	fakeInformer            *controllertest.FakeInformer
	clusters                []plugintestcluster.Cluster
	plugin                  valiplugin.Vali
)

func main() {
	var err error

	valiPluginConfiguration, err = plugintestconfig.NewConfiguration()
	if err != nil {
		panic(err)
	}
	fakeInformer = &controllertest.FakeInformer{}
	logger := plugintestconfig.NewLogger()

	testClient = plugintestclient.NewBlackBoxTestingValiClient()
	valiPluginConfiguration.ClientConfig.TestingClient = testClient
	go testClient.Run()

	fakeInformer.Synced = true
	fmt.Println("Creating new plugin")
	plugin, err = valiplugin.NewPlugin(fakeInformer, &valiPluginConfiguration, logger)
	if err != nil {
		panic(err)
	}

	fmt.Println("Creating Cluster resources")
	clusters = plugintestcluster.CreateNClusters(numberOfClusters)
	for i := 0; i < numberOfClusters; i++ {
		fakeInformer.Add(clusters[i].GetCluster())
	}

	loggerController := input.NewLoggerController(plugin, input.LoggerControllerConfig{
		NumberOfClusters:        numberOfClusters,
		NumberOfOperatorLoggers: 1,
		NumberOfUserLoggers:     1,
		NumberOfLogs:            10000,
	})
	loggerController.Run()
	fmt.Println("Waiting for pods to finish logging")
	loggerController.Wait()
	fmt.Println("Waiting for thwo more minutes")
	time.Sleep(5 * time.Minute)

	matcher := matcher.NewMatcher()

	fmt.Println("Matching")
	pods := loggerController.GetPods()
	for _, pod := range pods {
		if !matcher.Match(pod, testClient) {
			fmt.Println("Not all logs found for ", pod.GetOutput().GetLabelSet())
		}
	}

	// for _, entry := range testClient.GetEntries() {
	// 	fmt.Println(entry)
	// }

	fmt.Println("Closing Vali plugin")
	plugin.Close()
	fmt.Println("Test ends")
}
