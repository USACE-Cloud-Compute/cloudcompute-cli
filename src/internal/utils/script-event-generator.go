package utils

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/buffer"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/require"
	"github.com/usace/cloudcompute"
)

type ScriptEventGenerator struct {
	vm     *goja.Runtime
	script string
	mu     sync.Mutex // Mutex to ensure thread safety
}

func NewScriptEventGenerator(scriptPath string) (*ScriptEventGenerator, error) {
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

	seg := ScriptEventGenerator{vm: vm, script: scriptPath}

	err = seg.loadScript()

	return &seg, err
}

func (seg *ScriptEventGenerator) loadScript() error {
	script, err := os.ReadFile(seg.script)
	if err != nil {
		return fmt.Errorf("failed to read script '%s': %s", seg.script, err)
	}

	_, err = seg.vm.RunScript("EventGenerator", string(script))
	if err != nil {
		return fmt.Errorf("failed to run script '%s': %s", seg.script, err)
	}
	//call init function it it exists
	fnValue := seg.vm.Get("init")
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

func (seg *ScriptEventGenerator) HasNextEvent() bool {
	return true
}

func (seg *ScriptEventGenerator) NextEvent() (cloudcompute.Event, bool, error) {
	return cloudcompute.Event{}, true, nil
}
