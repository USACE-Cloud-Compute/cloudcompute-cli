package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

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
	Global *CmdOpts
}

func (rc *RunCommand) Execute(args []string) error {
	compute, err := initCompute(*rc.Global)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error running compute:")
		return err
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

type Options struct {
	CmdOpts
	Run       RunCommand       `command:"run" description:"Run the service"`
	Register  RegisterCommand  `command:"register" description:"Register the service"`
	Terminate TerminateCommand `command:"terminate" description:"Terminate the service"`
}

func main() {
	var opts Options

	// Inject global options into subcommands
	opts.Run.Global = &opts.CmdOpts
	opts.Register.Global = &opts.CmdOpts
	opts.Terminate.Global = &opts.CmdOpts

	parser := flags.NewParser(&opts, flags.Default)

	if _, err := parser.Parse(); err != nil {
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

/*
type CmdOpts struct {
	EnvFile string `short:"e" long:"envFile" description:"Environment file for the compute run"`
	Args    struct {
		Bin         string
		Command     string `description:"The command to run {run/register}. Running on local docker will auto register the plugin."`
		ComputeFile string `description:"The absolute or relative path to the compute file"`
	} `positional-args:"yes"`
}

func main() {

	options := CmdOpts{}

	_, err := flags.ParseArgs(&options, os.Args)
	if err != nil {
		log.Fatalln("Exiting")
	}

	doCompute(options)
}

func doCompute(options CmdOpts) {

	if options.EnvFile != "" {
		err := setEnv(options.EnvFile)
		if err != nil {
			log.Fatalln("Failed to set environment")
		}
	}

	computefiledir := path.Dir(options.Args.ComputeFile)

	computeConfig, err := utils.ReadJson[CmdComputeConfig](options.Args.ComputeFile)
	if err != nil {
		log.Fatalf("Unable to read the compute file: %s\n", err)
	}

	///////////////////
	var computeProvider ComputeProvider
	providerQueue, ok := computeConfig.Provider["queue"].(string)
	if !ok {
		log.Fatalln("No queue was specified for the compute provider")
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
			log.Fatalln(err)
		}
	case AWSBATCH_PROVIDER:
		fmt.Println("Using the AWS Batch compute provider")
		computeProvider, err = awsCompute(computeConfig)
		if err != nil {
			log.Fatalln(err)
		}
	default:
		log.Fatalf("Invalid compute provider: %s\n", providerType)
	}

	compute := CmdCompute{
		provider:       computeProvider,
		computeConfig:  computeConfig,
		computeFileDir: computefiledir,
		computeQueue:   providerQueue,
	}

	switch options.Args.Command {
	case CMD_REGISTER:
		compute.Register()
	case CMD_RUN:
		if providerType == DOCKER_PROVIDER {
			compute.Register() //always register the plugins for a local docker run
		}
		compute.Run()
		if providerType == DOCKER_PROVIDER {
			compute.WaitForJobs() //don't let the main goroutine run out until all of the jobs have completed
		}
	case CMD_TERMINATE:
		log.Println("Terminate")
		compute.Terminate(SUMMARY_COMPUTE, "cfbf5619-9abd-45f8-a0aa-f01bc67abffe", "I screwed up!")
	}
}
*/

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
