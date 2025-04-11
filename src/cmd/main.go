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
)

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
	}
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
