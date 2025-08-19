package utils

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/buffer"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/require"
	"github.com/usace/cloudcompute"
)

const (
	jsToObject   string = "computeEvent=JSON.parse(computeEventJson)"
	jsFromObject string = "computeEventJson=JSON.stringify(computeEvent)"
)

type JavascriptPostProcessor struct {
	vm     *goja.Runtime
	script string
	mu     sync.Mutex // Mutex to ensure thread safety
}

func NewJavascriptPostProcessor(scriptPath string) (*JavascriptPostProcessor, error) {
	registry := new(require.Registry)
	vm := goja.New()
	registry.Enable(vm)
	console.Enable(vm)
	buffer.Enable(vm)

	fs := vm.NewObject()

	readFileSync := func(call goja.FunctionCall) goja.Value {
		path := call.Argument(0).String()
		content, err := os.ReadFile(path)
		if err != nil {
			panic(fmt.Errorf("unable to read file %s: %s", path, err)) //panic on error when running in JS
		}
		return vm.ToValue(string(content))
	}

	err := fs.Set("readFileSync", readFileSync)
	if err != nil {
		return nil, fmt.Errorf("unable to set the readFileSync function on the fs object: %s", err)
	}

	readFileLinesSync := func(call goja.FunctionCall) goja.Value {
		path := call.Argument(0).String()
		file, err := os.Open(path)
		if err != nil {
			panic(fmt.Errorf("failed to open file: %s", err))
		}
		defer file.Close()

		lines := []string{}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			lines = append(lines, line)
		}
		return vm.ToValue(lines)
	}

	err = fs.Set("readFileLinesSync", readFileLinesSync)
	if err != nil {
		return nil, fmt.Errorf("unable to set the readFileLinesSync function on the fs object: %s", err)
	}

	err = vm.Set("fs", fs)
	if err != nil {
		return nil, fmt.Errorf("unable to set the fs object on the vm: %s", err)
	}
	jpp := JavascriptPostProcessor{vm: vm, script: scriptPath}

	err = jpp.loadScript()

	return &jpp, err
}

func (pp *JavascriptPostProcessor) loadScript() error {
	script, err := os.ReadFile(pp.script)
	if err != nil {
		return fmt.Errorf("failed to read script '%s': %s", pp.script, err)
	}

	_, err = pp.vm.RunScript("PostProcess", string(script))
	if err != nil {
		return fmt.Errorf("failed to run script '%s': %s", pp.script, err)
	}
	//call init function it it exists
	fnValue := pp.vm.Get("init")
	if fnValue != nil {
		log.Println("found js post process init function.  calling it now")
		fn, ok := goja.AssertFunction(fnValue)
		if !ok {
			panic("init is not a function")
		}

		_, err := fn(goja.Undefined())
		if err != nil {
			return err
		}
	}
	return nil
}

func (pp *JavascriptPostProcessor) Process(event cloudcompute.Event) (cloudcompute.Event, error) {

	//lock the method to ensure "thread" safety
	pp.mu.Lock()
	defer pp.mu.Unlock()

	startTime := time.Now()

	//Start by stringify'ing the golang event struct to json
	jsonevent, err := json.Marshal(event)
	if err != nil {
		panic(fmt.Errorf("error marshaling to json: %s", err))
	}

	//inject the json event into the vm
	pp.vm.Set("computeEventJson", string(jsonevent))

	//run js code to convert the json back into a native js object
	_, err = pp.vm.RunScript("JsonToObject", string(jsToObject))
	if err != nil {
		panic(fmt.Errorf("failed to parse json compute manifest': %s", err))
	}

	//call the onEvent js function
	fnValue := pp.vm.Get("onEvent")
	if fnValue != nil {
		fn, ok := goja.AssertFunction(fnValue)
		if !ok {
			panic("onEvent is not a function")
		}

		_, err := fn(goja.Undefined())
		if err != nil {
			return cloudcompute.Event{}, err
		}
	}

	//convert the js event object back into json
	_, err = pp.vm.RunScript("objectFromJson", string(jsFromObject))
	if err != nil {
		panic(fmt.Errorf("failed to stringify compute manifest': %s", err))
	}

	//export the json and unmarshall it back into a golang event struct
	val := []byte(pp.vm.Get("computeEventJson").Export().(string))
	computeEvent := cloudcompute.Event{}
	err = json.Unmarshal(val, &computeEvent)
	if err != nil {
		panic(fmt.Errorf("error unmarshaling from json: %s", err))
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)
	log.Printf("Duration was %d ms", duration/time.Millisecond)

	return computeEvent, nil
}
