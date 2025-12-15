package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	. "github.com/usace-cloud-compute/cloudcompute"
	"github.com/usace-cloud-compute/cloudcompute-cli/internal/utils"
	. "github.com/usace-cloud-compute/cloudcompute/providers/awsbatch"
	. "github.com/usace-cloud-compute/cloudcompute/providers/docker"
)

type TerminationLevel string

const (
	arrayGenType         string = "array"
	arrayGenType2        string = "array2"
	streamGenType        string = "stream"
	realzBlockGenType    string = "realzblock"
	concurrentSubmission int    = 10
)

type CmdCompute struct {
	//options        CmdOpts
	provider           ComputeProvider
	computeConfig      *CmdComputeConfig
	computeFileDir     string
	computeQueue       string
	compute            CloudCompute
	providerType       string
	jobStore           CcJobStore
	eventPostProcessor EventProcessor
}

func (c *CmdCompute) GetLog(jobId string, token *string) (JobLogOutput, error) {
	output, err := c.provider.JobLog(jobId, token)
	if err != nil {
		return JobLogOutput{}, err
	}
	return output, nil
}

func (c *CmdCompute) Terminate(terminationLevel string, terminationIdentifier string, terminationMessage string) error {

	tji := TerminateJobInput{
		Reason:   terminationMessage,
		JobQueue: c.computeQueue,
		Query: JobsSummaryQuery{
			QueryLevel: strings.ToUpper(terminationLevel),
			QueryValue: JobNameParts{
				Compute: terminationIdentifier,
			},
		},
	}

	return c.provider.TerminateJobs(tji)
}

// Register reads and registers all plugins specified in the compute configuration.
// It iterates over each plugin path provided in the configuration, reads the plugin manifest,
// and then attempts to register it with the provider. If any step fails, it logs a fatal error
// and exits the program.  If it succeeds it prints the compute provider registration output
// data for each plugin registered
func (c *CmdCompute) Register() {
	plugins := make([]*Plugin, len(c.computeConfig.Plugins))

	for i, v := range c.computeConfig.Plugins {
		pluginpath := fmt.Sprintf("%s/%s", c.computeFileDir, v)
		plugin, err := utils.ReadJson[Plugin](pluginpath)
		if err != nil {
			log.Fatalf("Unable to read the plugin manifest: %s\n", err)
		}
		plugins[i] = plugin
	}

	for _, plugin := range plugins {
		registrationOutput, err := c.provider.RegisterPlugin(plugin)
		if err != nil {
			log.Fatalf("Failed to register plugin: %s: %s\n", plugin.Name, err)
		}
		data, err := json.Marshal(registrationOutput)
		if err != nil {
			log.Printf("invalid registration return value: %s\n", err)
		}
		fmt.Println(string(data))
	}

}

// Register reads a single user specified plugin and registers it with the compute configuration.
// If it succeeds it prints the compute provider registration output data.
// If it fails, it logs a fatal error and exits the program.
func (c *CmdCompute) RegisterManifest(pluginManifestFile string) {

	plugin, err := utils.ReadJson[Plugin](pluginManifestFile)
	if err != nil {
		log.Fatalf("Unable to read the plugin manifest %s (%s)\n", pluginManifestFile, err)
	}

	registrationOutput, err := c.provider.RegisterPlugin(plugin)
	if err != nil {
		log.Fatalf("Failed to register plugin: %s: %s\n", plugin.Name, err)
	}
	data, err := json.Marshal(registrationOutput)
	if err != nil {
		log.Printf("invalid registration return value: %s\n", err)
	}
	fmt.Println(string(data))

}

// Run executes the compute command by processing manifests, generating events,
// and initiating a CloudCompute instance.
//
// It performs the following steps:
// 1. Retrieves the list of manifest files from the compute configuration.
// 2. Reads each manifest file into a CmdLineManifest structure.
// 3. Assigns a unique ID to each manifest and stores them in a slice.
// 4. Converts the slice of CmdLineManifests to ComputeManifests.
// 5. Creates an EventList containing the computed manifests.
// 6. Initializes a CloudCompute instance with the necessary parameters.
// 7. Executes the CloudCompute instance and handles any errors that occur.
func (c *CmdCompute) Run() {
	manifestList, ok := c.computeConfig.Event["compute-manifests"]
	if !ok {
		log.Fatalf("No manifests")
	}

	ml := manifestList.([]any)

	manifests := make(CmdLineManifests, len(ml))
	for i, manifestFile := range ml {
		manifestPath := fmt.Sprintf("%s/%s", c.computeFileDir, manifestFile)
		manifest, err := utils.ReadJson[CmdLineManifest](manifestPath)
		if err != nil {
			log.Fatalf("Error reading manifest %s: %s\n", manifestFile, err)
		}
		manifest.ManifestID = uuid.New()
		manifest.FileName = manifestFile.(string) //@TODO type inference can result in a panic.  Revisit later after finalizing file formats
		manifests[i] = *manifest
	}

	computeManifests := manifests.ToComputeManifests()

	eventGenerator, err := buildEventGenerator(computeManifests, c.computeConfig)
	if err != nil {
		log.Fatalln(err)
	}

	computeID := uuid.New()
	fmt.Printf("Compute Identifier: %s\n", computeID.String())

	ccCompute := CloudCompute{
		Name:            "AGGREGATOR_TEST",
		ID:              computeID,
		JobQueue:        c.computeQueue,
		Events:          eventGenerator,
		ComputeProvider: c.provider,
		JobStore:        c.jobStore,
		EventProcessor:  c.eventPostProcessor,
	}

	//err = ccCompute.Run()
	err = ccCompute.RunParallel(concurrentSubmission)
	if err != nil {
		log.Fatalln(err)
	}
	c.compute = ccCompute
}

func buildEventGenerator(computeManifests []ComputeManifest, config *CmdComputeConfig) (EventGenerator, error) {

	//must have a generator config and a generator type, or we fall back to a
	//list generator with a single event
	if config.Generator != nil {

		event := Event{
			ID:              uuid.New(),
			EventIdentifier: "1",
			Manifests:       computeManifests,
		}

		//process per event loop structures first
		var pelmap []map[string]string
		var err error
		if pel, pelok := config.Generator["perEventLoop"]; pelok {
			pelData := pel.([]any)
			pelmap, err = utils.PelSliceToMap(pelData)
			if err != nil {
				return nil, err
			}
		}

		//create the event generator from the config
		if genType, ok := config.Generator["type"]; ok {
			switch genType {
			case arrayGenType:
				var start int64 = -1
				var end int64 = -1
				if st, stok := config.Generator["start"]; stok {
					start = int64(st.(float64))
				}
				if en, enok := config.Generator["end"]; enok {
					end = int64(en.(float64))
				}
				if start < 0 || end < 0 {
					return nil, fmt.Errorf("invalid Array Event generator start or end")
				}

				return NewArrayEventGenerator(event, pelmap, start, end)

			case streamGenType:
				if filepath, ok := config.Generator["file"]; ok {
					if delimiter, ok := config.Generator["delimiter"]; ok {
						file, err := os.Open(filepath.(string))
						if err == nil {
							return NewStreamingEventGeneratorForReader(event, pelmap, file, delimiter.(string))
						} else {
							wd, _ := os.Getwd()
							return nil, fmt.Errorf("failed to open stream generator file %s from working directory %s: %s", filepath, wd, err)
						}
					}
				}
				return nil, fmt.Errorf("invalid Stream Event generator")
			case realzBlockGenType:
				var startRealz int64 = -1
				var endRealz int64 = -1
				var startBlock int64 = -1
				var endBlock int64 = -1
				if st, stok := config.Generator["startRealz"]; stok {
					startRealz = int64(st.(float64))
				}
				if en, enok := config.Generator["endRealz"]; enok {
					endRealz = int64(en.(float64))
				}
				if st, stok := config.Generator["startBlock"]; stok {
					startBlock = int64(st.(float64))
				}
				if en, enok := config.Generator["endBlock"]; enok {
					endBlock = int64(en.(float64))
				}

				if startRealz > -1 && endRealz > -1 && startBlock > -1 && endBlock > -1 {
					return utils.NewRealzBlockEventGenerator(event, startRealz, endRealz, startBlock, endBlock)
				}
				return nil, fmt.Errorf("realz block event generator")

			case arrayGenType2:
				var start int64 = -1
				var end int64 = -1
				if st, stok := config.Generator["start"]; stok {
					start = int64(st.(float64))
				}
				if en, enok := config.Generator["end"]; enok {
					end = int64(en.(float64))
				}
				if start < 0 || end < 0 {
					return nil, fmt.Errorf("invalid Array Event generator start or end")
				}

				var ae map[string]any
				var aeok bool
				if addEnv, addok := config.Generator["addEnv"]; addok {
					ae, aeok = addEnv.(map[string]any)
					if !aeok {
						return nil, fmt.Errorf("invalid additional env vars.  Must be a map<string,string>")
					}
				}

				return utils.NewArrayEventGenerator2(utils.ArrayEventGeneratorInput{
					Event:                  event,
					PerEventLoopData:       pelmap,
					StartIndex:             start,
					EndIndex:               end,
					AdditionalEnvResources: ae,
				})
			}
		}
	}

	//default to a list event generator with a single event
	return NewEventList([]Event{
		{
			ID:              uuid.New(),
			EventIdentifier: "1",
			Manifests:       computeManifests,
		},
	}), nil

}

// WaitForJobs continuously monitors the status of jobs associated with a compute instance.
// It periodically checks if any jobs are still running. If no jobs are found in the "RUNNING" state,
// it sets the `jobsRunning` flag to false and prints "Shutting Down".
func (c *CmdCompute) WaitForJobs() {
	jobsRunning := true
	for jobsRunning {
		time.Sleep(1 * time.Second)
		c.compute.Status(JobsSummaryQuery{
			QueryLevel: "COMPUTE",
			QueryValue: JobNameParts{
				Compute: c.compute.ID.String(),
			},
			JobSummaryFunction: func(summaries []JobSummary) {
				count := 0
				for _, summary := range summaries {
					if summary.Status == "RUNNING" ||
						summary.Status == "SUBMITTED" ||
						summary.Status == "PENDING" ||
						summary.Status == "STARTING" ||
						summary.Status == "RUNNABLE" {
						count++
					}
				}
				if count == 0 {
					jobsRunning = false
					fmt.Println("Shutting Down")
				}
			},
		})
	}
}

func awsCompute(compute *CmdComputeConfig) (ComputeProvider, error) {
	er := compute.Provider["execution-role"].(string)
	region := compute.Provider["region"].(string)
	profile := ""
	if pprofile, ok := compute.Provider["profile"].(string); ok {
		profile = pprofile
	}
	//profile := compute.Provider["profile"].(string)
	cpi := NewAwsBatchProviderInput(er, region, profile)
	return NewAwsBatchProvider(cpi)
}

func dockerCompute(compute *CmdComputeConfig) (ComputeProvider, error) {
	concurrency := 1
	if c, ok := compute.Provider["concurrency"]; ok {
		if cJson, okt := c.(float64); okt {
			concurrency = int(cJson)
		}
	}
	dockerComputeProviderConfig := DockerComputeProviderConfig{
		Concurrency:               concurrency,
		DockerPullProgressFactory: &utils.CliDockerPullProgressFactory{},
	}

	if compute.SecretsManager != nil {
		sm := compute.SecretsManager
		switch sm.SmType {
		case "env":
			esm := NewEnvironmentSecretsManager()
			for k, v := range sm.Secrets {
				esm.AddSecret(k, v)
			}
			dockerComputeProviderConfig.SecretsManager = esm
		default:
			return nil, fmt.Errorf("invalid secrets manager type.  type: %s is unsupported", sm.SmType)
		}
	}

	computeProvider := NewDockerComputeProvider(dockerComputeProviderConfig)

	return computeProvider, nil
}

type CmdComputeConfig struct {
	Name           string                     `json:"name"`
	Provider       map[string]any             `json:"provider"`
	Plugins        []string                   `json:"plugins"`
	Event          map[string]any             `json:"event"`
	SecretsManager *CommandLineSecretsManager `json:"secrets-manager"`
	Generator      map[string]any             `json:"generator"`
}

type CommandLineSecretsManager struct {
	SmType  string            `json:"type"`
	Secrets map[string]string `json:"secrets"`
}

type CmdLineManifests []CmdLineManifest

// ToComputeManifests converts CmdLineManifests into a slice of ComputeManifest.
// It assigns unique UUIDs to each manifest and resolves dependencies by mapping
// them to their respective ManifestIDs. If a dependency cannot be found, it logs
// a fatal error.
func (clms CmdLineManifests) ToComputeManifests() []ComputeManifest {
	//set ids
	for i := range clms {
		clms[i].ManifestID = uuid.New()
	}

	//create compute manifests and update deps
	computeManifests := make([]ComputeManifest, len(clms))
	for i := range clms {
		m := clms[i]
		depcount := len(m.DependsOn)
		if depcount > 0 {
			dependencies := make([]uuid.UUID, depcount)
			for i, dep := range m.DependsOn {
				dm, err := clms.GetManifest(dep)
				if err != nil {
					log.Fatalf("manifest error: %s\n", err)
				}
				dependencies[i] = dm.ManifestID
			}
			clms[i].Dependencies = dependencies
		}
		computeManifests[i] = clms[i].ComputeManifest
	}
	return computeManifests
}

func (clms CmdLineManifests) GetManifest(filename string) (CmdLineManifest, error) {
	for _, clm := range clms {
		if clm.FileName == filename {
			return clm, nil
		}
	}
	return CmdLineManifest{}, fmt.Errorf("invalid manifest: %s not found", filename)
}

type CmdLineManifest struct {
	DependsOn []string `json:"depends-on"`
	FileName  string
	ComputeManifest
}
