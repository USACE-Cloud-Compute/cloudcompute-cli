package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/spf13/cobra"
	. "github.com/usace/cloudcompute"
	"github.com/usace/manifestor/internal/utils"
)

const (
	DOCKER_PROVIDER          = "docker"
	AWSBATCH_PROVIDER        = "awsbatch"
	SECRET_MANAGER_IN_MEMORY = "in-memory"
	SECRET_MANAGER_ENV       = "env"
)

// global flags
var (
	computeFile string
	envFile     string
	jobStore    string
	//pluginManifest string //used for register as an alternative to reading manifests from the DAG in the compute file
)

// commands
var (
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run the compute job",
		RunE: func(cmd *cobra.Command, args []string) error {
			compute, err := initCompute()
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error running compute:", err)
				return err
			}

			if jobStore != "" {
				jobStoreWriter, err := utils.NewCsvJobStore(jobStore)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error creating Job Store file:", err)
				}
				compute.jobStore = jobStoreWriter
			}

			if compute.providerType == DOCKER_PROVIDER {
				compute.Register()
			}
			compute.Run()
			if compute.providerType == DOCKER_PROVIDER {
				compute.WaitForJobs()
			}
			return nil
		},
	}

	registerCmd = &cobra.Command{
		Use:   "register [plugin manifest file]",
		Short: "Register plugins with a compute provider.",
		Long:  "By default, all plugins referenced in the compute file DAG will be registered.  Optionally the command can include a file path to a plugin manifest. If this option is provided, then the register function will only register the given plugin manifest.",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {

			compute, err := initCompute()
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error running compute:", err)
				return err
			}

			//if there is an argument, it is a pluginManifestFile path
			if len(args) > 0 {
				log.Printf("Registering a single plugin manifest: %s", args[0])
				compute.RegisterManifest(args[0])
				return nil
			} else {
				compute.Register()
				return nil
			}
		},
	}

	terminateCmd = &cobra.Command{
		Use:   "terminate [level] [id] [message]",
		Short: "Terminate job(s) on a compute provider",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			compute, err := initCompute()
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error running compute:", err)
				return err
			}
			level, id, message := args[0], args[1], args[2]
			compute.Terminate(level, id, message)
			return nil
		},
	}

	logCmd = &cobra.Command{
		Use:   "log [jobID]",
		Short: "Get job logs from a compute provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			compute, err := initCompute()
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error getting log:", err)
				return err
			}

			jobID := args[0]
			var token *string = aws.String("")
			for {
				logs, err := compute.GetLog(jobID, token)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error getting log:", err)
					return err
				}
				if logs.Token == nil || *token == *logs.Token {
					break
				}
				token = logs.Token
				for _, logrow := range logs.Logs {
					fmt.Println(logrow)
				}
			}
			return nil
		},
	}

	rootCmd = &cobra.Command{
		Use:   "manifestor",
		Short: "Manifestor CLI to run, register, terminate, and fetch logs for cloud compute.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if envFile != "" {
				return setEnv(envFile)
			}
			return nil
		},
	}
)

func initializeCommands() {
	//root
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentFlags().StringVarP(&computeFile, "computeFile", "c", "", "Path to compute file (required)")
	rootCmd.PersistentFlags().StringVarP(&envFile, "envFile", "e", "", "Path to env file")
	rootCmd.MarkPersistentFlagRequired("computeFile")

	//run
	runCmd.Flags().StringVarP(&jobStore, "jobStore", "j", "", "Optional local csv file for writing the list of jobs submitted to the compute provider")

	//register
	//registerCmd.Flags().StringVarP(&pluginManifest, "pluginManifest", "p", "", "Path to plugin manifest file (for registration only)")
}

func main() {
	initializeCommands()
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(registerCmd)
	rootCmd.AddCommand(terminateCmd)
	rootCmd.AddCommand(logCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "unable to run manifestor:", err)
		os.Exit(1)
	}
}

func initCompute() (*CmdCompute, error) {
	if computeFile == "" {
		return nil, fmt.Errorf("compute file not provided")
	}

	computefiledir := path.Dir(computeFile)
	computeConfig, err := utils.ReadJson[CmdComputeConfig](computeFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Unable to read the compute file:")
		return nil, err
	}

	providerQueue, ok := computeConfig.Provider["queue"].(string)
	if !ok {
		return nil, fmt.Errorf("no queue was specified for the compute provider")
	}
	providerType, ok := computeConfig.Provider["type"]
	if !ok {
		log.Fatalln("No compute provider defined in the compute.json file.")
	}

	var computeProvider ComputeProvider
	switch providerType {
	case DOCKER_PROVIDER:
		fmt.Println("Using the docker compute provider")
		computeProvider, err = dockerCompute(computeConfig)
	case AWSBATCH_PROVIDER:
		fmt.Println("Using the AWS Batch compute provider")
		computeProvider, err = awsCompute(computeConfig)
	default:
		log.Fatalf("Invalid compute provider: %s\n", providerType)
	}
	if err != nil {
		return nil, err
	}

	return &CmdCompute{
		provider:       computeProvider,
		computeConfig:  computeConfig,
		computeFileDir: computefiledir,
		computeQueue:   providerQueue,
		providerType:   providerType.(string),
	}, nil
}

func setEnv(envfile string) error {
	vars, err := readLines(envfile)
	if err != nil {
		return err
	}
	for _, v := range vars {
		vals := strings.SplitN(v, "=", 2)
		if len(vals) != 2 {
			return fmt.Errorf("invalid environment variable: %s", v)
		}
		if err := os.Setenv(vals[0], vals[1]); err != nil {
			log.Printf("Unable to set environment variable %s", v)
			return err
		}
	}
	return nil
}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

/*
example code for defining custom help output
// Override the HelpFunc to exclude [flags] when there are no flags
    exampleSubCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
        if cmd.HasAvailableFlags() {
            cmd.PrintDefaultHelp()
        } else {
            fmt.Fprintf(cmd.OutOrStderr(), "%s\n\n", cmd.Short)
            fmt.Fprintf(cmd.OutOrStderr(), "Usage:\n  %s\n\n", cmd.UseLine())
            if len(cmd.Long) > 0 {
                fmt.Fprintf(cmd.OutOrStderr(), "%s\n\n", cmd.Long)
            }
        }
    })
*/
