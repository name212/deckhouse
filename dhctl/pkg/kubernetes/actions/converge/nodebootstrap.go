// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package converge

import (
	"bufio"
	"bytes"
	"fmt"
	"sync"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

func NodeName(cfg *config.MetaConfig, nodeGroupName string, index int) string {
	return fmt.Sprintf("%s-%s-%v", cfg.ClusterPrefix, nodeGroupName, index)
}

func BootstrapAdditionalNode(kubeCl *client.KubernetesClient, cfg *config.MetaConfig, index int, step, nodeGroupName, cloudConfig string, isConverge bool, terraformContext *terraform.TerraformContext) error {
	nodeName := NodeName(cfg, nodeGroupName, index)

	if isConverge {
		nodeExists, err := IsNodeExistsInCluster(kubeCl, nodeName, log.GetDefaultLogger())
		if err != nil {
			return err
		} else if nodeExists {
			return fmt.Errorf("node with name %s exists in cluster", nodeName)
		}
	}

	nodeGroupSettings := cfg.FindTerraNodeGroup(nodeGroupName)

	// TODO pass cache as argument or better refact func
	runner := terraformContext.GetBootstrapNodeRunner(cfg, cache.Global(), terraform.BootstrapNodeRunnerOptions{
		AutoApprove:     true,
		NodeName:        nodeName,
		NodeGroupStep:   step,
		NodeGroupName:   nodeGroupName,
		NodeIndex:       index,
		NodeCloudConfig: cloudConfig,
		AdditionalStateSaverDestinations: []terraform.SaverDestination{
			NewNodeStateSaver(kubeCl, nodeName, nodeGroupName, nodeGroupSettings),
		},
		RunnerLogger: log.GetDefaultLogger(),
	})

	outputs, err := terraform.ApplyPipeline(runner, nodeName, terraform.OnlyState)
	if err != nil {
		return err
	}

	if tomb.IsInterrupted() {
		return ErrConvergeInterrupted
	}

	err = SaveNodeTerraformState(kubeCl, nodeName, nodeGroupName, outputs.TerraformState, nodeGroupSettings, log.GetDefaultLogger())
	if err != nil {
		return err
	}

	return nil
}

func BootstrapAdditionalNodeForParallelRun(kubeCl *client.KubernetesClient, cfg *config.MetaConfig, index int, step, nodeGroupName, cloudConfig string, isConverge bool, terraformContext *terraform.TerraformContext, runnerLogger log.Logger) error {
	nodeName := NodeName(cfg, nodeGroupName, index)
	nodeGroupSettings := cfg.FindTerraNodeGroup(nodeGroupName)
	// TODO pass cache as argument or better refact func
	runner := terraformContext.GetBootstrapNodeRunner(cfg, cache.Global(), terraform.BootstrapNodeRunnerOptions{
		AutoApprove:     true,
		NodeName:        nodeName,
		NodeGroupStep:   step,
		NodeGroupName:   nodeGroupName,
		NodeIndex:       index,
		NodeCloudConfig: cloudConfig,
		AdditionalStateSaverDestinations: []terraform.SaverDestination{
			NewNodeStateSaver(kubeCl, nodeName, nodeGroupName, nodeGroupSettings),
		},
		RunnerLogger: runnerLogger,
	})

	outputs, err := terraform.ApplyPipeline(runner, nodeName, terraform.OnlyState)
	if err != nil {
		return err
	}

	if tomb.IsInterrupted() {
		return ErrConvergeInterrupted
	}

	err = SaveNodeTerraformState(kubeCl, nodeName, nodeGroupName, outputs.TerraformState, nodeGroupSettings, runnerLogger)
	if err != nil {
		return err
	}

	return nil
}

func ParallelBootstrapAdditionalNodes(kubeCl *client.KubernetesClient, cfg *config.MetaConfig, nodesIndexToCreate []int, step, nodeGroupName, cloudConfig string, isConverge bool, terraformContext *terraform.TerraformContext, ngLogger log.Logger, saveLogToBuffer bool) ([]string, error) {

	var (
		nodesToWait []string
		wg          sync.WaitGroup
		mu          sync.Mutex
	)

	type checkResult struct {
		name        string
		buffNodeLog *bytes.Buffer
		err         error
	}

	for _, indexCandidate := range nodesIndexToCreate {
		candidateName := fmt.Sprintf("%s-%s-%v", cfg.ClusterPrefix, nodeGroupName, indexCandidate)
		nodeExists, err := IsNodeExistsInCluster(kubeCl, candidateName, ngLogger)
		if err != nil {
			return nil, err
		} else if nodeExists {
			return nil, fmt.Errorf("node with name %s exists in cluster", candidateName)
		}
	}

	if len(nodesIndexToCreate) > 1 && !saveLogToBuffer {
		ngLogger.LogWarnF("Many pipelines will run in parallel, terraform output for nodes %s-%v will be displayed after main execution.\n\n", nodeGroupName, nodesIndexToCreate[1:])
	}

	resultsChan := make(chan checkResult, len(nodesIndexToCreate))
	for i, indexCandidate := range nodesIndexToCreate {
		candidateName := fmt.Sprintf("%s-%s-%v", cfg.ClusterPrefix, nodeGroupName, indexCandidate)
		wg.Add(1)
		go func(i, indexCandidate int, candidateName string, logger log.Logger, saveLogToBuffer bool) {
			defer wg.Done()
			var buffNodeLog bytes.Buffer
			var nodeLogger log.Logger

			nodeLogger = ngLogger.CreateBufferLogger(&buffNodeLog)
			if i == 0 && !saveLogToBuffer {
				nodeLogger = ngLogger
			}
			err := BootstrapAdditionalNodeForParallelRun(kubeCl, cfg, indexCandidate, step, nodeGroupName, cloudConfig, true, terraformContext, nodeLogger)

			resultsChan <- checkResult{
				name:        candidateName,
				buffNodeLog: &buffNodeLog,
				err:         err,
			}
			mu.Lock()
			nodesToWait = append(nodesToWait, candidateName)
			mu.Unlock()
		}(i, indexCandidate, candidateName, ngLogger, saveLogToBuffer)
	}

	wg.Wait()
	close(resultsChan)

	for candidate := range resultsChan {
		if candidate.err != nil {
			return nodesToWait, candidate.err
		}
		if candidate.buffNodeLog.Len() == 0 {
			continue
		}

		scanner := bufio.NewScanner(candidate.buffNodeLog)
		for scanner.Scan() {
			ngLogger.LogInfoLn((scanner.Text()))
		}
	}
	return nodesToWait, nil
}

func ParallelCreateNodeGroup(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, terraNodeGroups []config.TerraNodeGroupSpec, terraformContext *terraform.TerraformContext) error {
	msg := "Create NodeGroups "
	for _, group := range terraNodeGroups {
		msg += fmt.Sprintf("%s (replicas: %v)️; ", group.Name, group.Replicas)
	}

	return log.Process("converge", msg, func() error {
		var (
			mu sync.Mutex
			wg sync.WaitGroup
		)
		type checkResult struct {
			name    string
			buffLog *bytes.Buffer
			err     error
		}
		currentLogger := log.GetDefaultLogger()

		ngWaitMap := make(map[string]int)
		resultsChan := make(chan checkResult, len(terraNodeGroups))
		for i, group := range terraNodeGroups {
			wg.Add(1)
			go func(i int, group config.TerraNodeGroupSpec) {
				defer wg.Done()

				var (
					buffNGLog       bytes.Buffer
					ngLogger        log.Logger
					saveLogToBuffer bool
				)

				if i == 0 {
					saveLogToBuffer = false
					ngLogger = currentLogger
				} else {
					saveLogToBuffer = true
					ngLogger = currentLogger.CreateBufferLogger(&buffNGLog)
				}

				err := CreateNodeGroup(kubeCl, group.Name, ngLogger, metaConfig.NodeGroupManifest(group))
				if err != nil {
					resultsChan <- checkResult{
						name:    group.Name,
						buffLog: &buffNGLog,
						err:     err,
					}
					return
				}

				nodeCloudConfig, err := GetCloudConfig(kubeCl, group.Name, ShowDeckhouseLogs, ngLogger)
				if err != nil {
					resultsChan <- checkResult{
						name:    group.Name,
						buffLog: &buffNGLog,
						err:     err,
					}
					return
				}

				var nodesIndexToCreate []int
				for i := 0; i < group.Replicas; i++ {
					nodesIndexToCreate = append(nodesIndexToCreate, i)
				}

				_, err = ParallelBootstrapAdditionalNodes(kubeCl, metaConfig, nodesIndexToCreate, "static-node", group.Name, nodeCloudConfig, true, terraformContext, ngLogger, saveLogToBuffer)

				resultsChan <- checkResult{
					name:    group.Name,
					buffLog: &buffNGLog,
					err:     err,
				}
				mu.Lock()
				ngWaitMap[group.Name] = group.Replicas
				mu.Unlock()
			}(i, group)
		}

		wg.Wait()
		close(resultsChan)

		for ng := range resultsChan {
			if ng.err != nil {
				return ng.err
			}
			if ng.buffLog.Len() == 0 {
				continue
			}
			currentPLogger := log.GetProcessLogger()
			currentPLogger.LogProcessStart(fmt.Sprintf("Output NG [%s] log", ng.name))
			scanner := bufio.NewScanner(ng.buffLog)
			for scanner.Scan() {
				log.InfoLn(scanner.Text())
			}
			currentPLogger.LogProcessEnd()
		}

		return WaitForNodesBecomeReady(kubeCl, ngWaitMap)
	})
}

func BootstrapAdditionalMasterNode(kubeCl *client.KubernetesClient, cfg *config.MetaConfig, index int, cloudConfig string, isConverge bool, terraformContext *terraform.TerraformContext) (*terraform.PipelineOutputs, error) {
	nodeName := NodeName(cfg, MasterNodeGroupName, index)

	if isConverge {
		nodeExists, existsErr := IsNodeExistsInCluster(kubeCl, nodeName, log.GetDefaultLogger())
		if existsErr != nil {
			return nil, existsErr
		} else if nodeExists {
			return nil, fmt.Errorf("node with name %s exists in cluster", nodeName)
		}
	}

	// TODO pass cache as argument or better refact func
	runner := terraformContext.GetBootstrapNodeRunner(cfg, cache.Global(), terraform.BootstrapNodeRunnerOptions{
		AutoApprove:     true,
		NodeName:        nodeName,
		NodeGroupStep:   "master-node",
		NodeGroupName:   MasterNodeGroupName,
		NodeIndex:       index,
		NodeCloudConfig: cloudConfig,
		AdditionalStateSaverDestinations: []terraform.SaverDestination{
			NewNodeStateSaver(kubeCl, nodeName, MasterNodeGroupName, nil),
		},
		RunnerLogger: log.GetDefaultLogger(),
	})

	outputs, err := terraform.ApplyPipeline(runner, nodeName, terraform.GetMasterNodeResult)
	if err != nil {
		return nil, err
	}

	if tomb.IsInterrupted() {
		return nil, ErrConvergeInterrupted
	}

	err = SaveMasterNodeTerraformState(kubeCl, nodeName, outputs.TerraformState, []byte(outputs.KubeDataDevicePath))
	if err != nil {
		return outputs, err
	}

	return outputs, err
}
