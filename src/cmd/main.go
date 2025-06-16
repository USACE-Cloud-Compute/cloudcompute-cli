package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/jessevdk/go-flags"
	. "github.com/usace/cloudcompute"
	"github.com/usace/manifestor/internal/utils"
)

const (
	DOCKER_PROVIDER          string = "docker"
	AWSBATCH_PROVIDER        string = "awsbatch"
	SECRET_MANAGER_IN_MEMORY string = "in-memory"
	SECRET_MANAGER_ENV       string = "env"
	CMD_REGISTER             string = "register"
	CMD_RUN                  string = "run"
	CMD_TERMINATE            string = "terminate"
)

type CmdOpts struct {
	ComputeFile string `short:"c" long:"computeFile" description:"absolute or relative path to the compute file (required)" required:"false"`
	EnvFile     string `short:"e" long:"envFile" description:"Environment file for the compute run"`
}

type RunCommand struct {
	Global       *CmdOpts
	JobStorePath string `short:"j" long:"jobStore" description:"Optional local csv file for writing the list of jobs submitted to the compute provider"`
}

func (rc *RunCommand) Execute(args []string) error {
	compute, err := initCompute(*rc.Global)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error running compute:")
		return err
	}

	if rc.JobStorePath != "" {
		jobStore, err := utils.NewCsvJobStore(rc.JobStorePath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error creating Job Store file:", err)
		}
		compute.jobStore = jobStore
	}

	if compute.providerType == DOCKER_PROVIDER {
		compute.Register()
	}
	compute.Run()
	if compute.providerType == DOCKER_PROVIDER {
		compute.WaitForJobs() //don't let the main goroutine run out until all of the jobs have completed
	}
	return nil
}

type RegisterCommand struct {
	Global *CmdOpts
}

func (rc *RegisterCommand) Execute(args []string) error {
	compute, err := initCompute(*rc.Global)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error running compute:")
		return err
	}
	compute.Register()
	return nil
}

type TerminateCommand struct {
	Global *CmdOpts
	Args   struct {
		TerminateLevel   string `positional-arg-name:"level" description:"Termination level: one of the following options: COMPUTE, EVENT, JOB"`
		TerminateId      string `positional-arg-name:"id" description:"Termination identifier: The guid identifier for the Compute, Event, or Job being terminated"`
		TerminateMessage string `positional-arg-name:"message" description:"Message for the compute provider describing why the jobs were terminated"`
	} `positional-args:"yes"`
}

func (tc *TerminateCommand) Execute(args []string) error {
	compute, err := initCompute(*tc.Global)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error running compute:")
		return err
	}
	compute.Terminate(tc.Args.TerminateLevel, tc.Args.TerminateId, tc.Args.TerminateMessage)
	return nil
}

type LogCommand struct {
	Global *CmdOpts
	Args   struct {
		JobId string `positional-arg-name:"id" description:"job identifier: The guid identifier for the compute provider job"`
	} `positional-args:"yes"`
}

func (lc *LogCommand) Execute(args []string) error {
	compute, err := initCompute(*lc.Global)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error getting log:", err)
		return err
	}

	var token *string = aws.String("")

	for {
		logs, err := compute.GetLog(lc.Args.JobId, token)
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
}

type Options struct {
	CmdOpts
	Run       RunCommand       `command:"run" description:"Run the service"`
	Register  RegisterCommand  `command:"register" description:"Register the service"`
	Terminate TerminateCommand `command:"terminate" description:"Terminate the service"`
	Logs      LogCommand       `command:"log" description:"Get job logs"`
}

func main() {
	var opts Options

	// Inject global options into subcommands
	opts.Run.Global = &opts.CmdOpts
	opts.Register.Global = &opts.CmdOpts
	opts.Terminate.Global = &opts.CmdOpts
	opts.Logs.Global = &opts.CmdOpts

	parser := flags.NewParser(&opts, flags.Default)

	if _, err := parser.Parse(); err != nil {
		fmt.Fprintln(os.Stderr, "unable to run manifestor:", err)
		os.Exit(1)
	}
}

func initCompute(options CmdOpts) (*CmdCompute, error) {
	if options.EnvFile != "" {
		err := setEnv(options.EnvFile)
		if err != nil {
			return nil, err
		}
	}

	computefiledir := path.Dir(options.ComputeFile)

	computeConfig, err := utils.ReadJson[CmdComputeConfig](options.ComputeFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Unable to read the compute file:")
		return nil, err
	}

	var computeProvider ComputeProvider
	providerQueue, ok := computeConfig.Provider["queue"].(string)
	if !ok {
		return nil, fmt.Errorf("no queue was specified for the compute provider")
	}
	providerType, ok := computeConfig.Provider["type"]
	if !ok {
		log.Fatalln("No compute provider defined in the compute.json file.")
	}
	switch providerType {
	case DOCKER_PROVIDER:
		fmt.Println("Using the docker compute provider")
		computeProvider, err = dockerCompute(computeConfig)
		if err != nil {
			return nil, err
		}
	case AWSBATCH_PROVIDER:
		fmt.Println("Using the AWS Batch compute provider")
		computeProvider, err = awsCompute(computeConfig)
		if err != nil {
			return nil, err
		}
	default:
		log.Fatalf("Invalid compute provider: %s\n", providerType)
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
		vals := strings.Split(v, "=")
		if len(vals) != 2 {
			return fmt.Errorf("invalid environment variable")
		}
		err := os.Setenv(vals[0], vals[1])
		if err != nil {
			log.Println("Unable to set environment variable")
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
