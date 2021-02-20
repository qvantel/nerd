package testcontainers

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"gopkg.in/yaml.v2"
)

const (
	envProjectName = "COMPOSE_PROJECT_NAME"
	envComposeFile = "COMPOSE_FILE"
)

// DockerCompose defines the contract for running Docker Compose
type DockerCompose interface {
	Down() ExecError
	Invoke() ExecError
	WithCommand([]string) DockerCompose
	WithEnv(map[string]string) DockerCompose
}

// LocalDockerCompose represents a Docker Compose execution using local binary
// docker-compose or docker-compose.exe, depending on the underlying platform
type LocalDockerCompose struct {
	Executable          string
	ComposeFilePaths    []string
	absComposeFilePaths []string
	Identifier          string
	Cmd                 []string
	Env                 map[string]string
	Services            map[string]interface{}
}

// NewLocalDockerCompose returns an instance of the local Docker Compose, using an
// array of Docker Compose file paths and an identifier for the Compose execution.
//
// It will iterate through the array adding '-f compose-file-path' flags to the local
// Docker Compose execution. The identifier represents the name of the execution,
// which will define the name of the underlying Docker network and the name of the
// running Compose services.
func NewLocalDockerCompose(filePaths []string, identifier string) *LocalDockerCompose {
	dc := &LocalDockerCompose{}

	dc.Executable = "docker-compose"
	if runtime.GOOS == "windows" {
		dc.Executable = "docker-compose.exe"
	}

	dc.ComposeFilePaths = filePaths

	dc.absComposeFilePaths = make([]string, len(filePaths))
	for i, cfp := range dc.ComposeFilePaths {
		abs, _ := filepath.Abs(cfp)
		dc.absComposeFilePaths[i] = abs
	}

	dc.validate()

	dc.Identifier = strings.ToLower(identifier)

	return dc
}

// Down executes docker-compose down
func (dc *LocalDockerCompose) Down() ExecError {
	return executeCompose(dc, []string{"down"})
}

func (dc *LocalDockerCompose) getDockerComposeEnvironment() map[string]string {
	environment := map[string]string{}

	composeFileEnvVariableValue := ""
	for _, abs := range dc.absComposeFilePaths {
		composeFileEnvVariableValue += abs + string(os.PathListSeparator)
	}

	environment[envProjectName] = dc.Identifier
	environment[envComposeFile] = composeFileEnvVariableValue

	return environment
}

// Invoke invokes the docker compose
func (dc *LocalDockerCompose) Invoke() ExecError {
	return executeCompose(dc, dc.Cmd)
}

// WithCommand assigns the command
func (dc *LocalDockerCompose) WithCommand(cmd []string) DockerCompose {
	dc.Cmd = cmd
	return dc
}

// WithEnv assigns the environment
func (dc *LocalDockerCompose) WithEnv(env map[string]string) DockerCompose {
	dc.Env = env
	return dc
}

// validate checks if the files to be run in the compose are valid YAML files, setting up
// references to all services in them
func (dc *LocalDockerCompose) validate() error {
	type compose struct {
		Services map[string]interface{}
	}

	for _, abs := range dc.absComposeFilePaths {
		c := compose{}

		yamlFile, err := ioutil.ReadFile(abs)
		if err != nil {
			return err
		}
		err = yaml.Unmarshal(yamlFile, &c)
		if err != nil {
			return err
		}

		dc.Services = c.Services
	}

	return nil
}

// ExecError is super struct that holds any information about an execution error, so the client code
// can handle the result
type ExecError struct {
	Command []string
	Error   error
	Stdout  error
	Stderr  error
}

// execute executes a program with arguments and environment variables inside a specific directory
func execute(
	dirContext string, environment map[string]string, binary string, args []string) ExecError {

	var errStdout, errStderr error

	cmd := exec.Command(binary, args...)
	cmd.Dir = dirContext
	cmd.Env = os.Environ()

	for key, value := range environment {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	stdoutIn, _ := cmd.StdoutPipe()
	stderrIn, _ := cmd.StderrPipe()

	stdout := newCapturingPassThroughWriter(os.Stdout)
	stderr := newCapturingPassThroughWriter(os.Stderr)

	err := cmd.Start()
	if err != nil {
		execCmd := []string{"Starting command", dirContext, binary}
		execCmd = append(execCmd, args...)

		return ExecError{
			// add information about the CMD and arguments used
			Command: execCmd,
			Error:   err,
			Stderr:  errStderr,
			Stdout:  errStdout,
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		_, errStdout = io.Copy(stdout, stdoutIn)
		wg.Done()
	}()

	_, errStderr = io.Copy(stderr, stderrIn)
	wg.Wait()

	err = cmd.Wait()

	execCmd := []string{"Reading std", dirContext, binary}
	execCmd = append(execCmd, args...)

	return ExecError{
		Command: execCmd,
		Error:   err,
		Stderr:  errStderr,
		Stdout:  errStdout,
	}
}

func executeCompose(dc *LocalDockerCompose, args []string) ExecError {
	if which(dc.Executable) != nil {
		return ExecError{
			Command: []string{dc.Executable},
			Error:   fmt.Errorf("Local Docker Compose not found. Is %s on the PATH?", dc.Executable),
		}
	}

	environment := dc.getDockerComposeEnvironment()
	for k, v := range dc.Env {
		environment[k] = v
	}

	cmds := []string{}
	pwd := "."
	if len(dc.absComposeFilePaths) > 0 {
		pwd, _ = filepath.Split(dc.absComposeFilePaths[0])

		for _, abs := range dc.absComposeFilePaths {
			cmds = append(cmds, "-f", abs)
		}
	} else {
		cmds = append(cmds, "-f", "docker-compose.yml")
	}
	cmds = append(cmds, args...)

	execErr := execute(pwd, environment, dc.Executable, cmds)
	err := execErr.Error
	if err != nil {
		args := strings.Join(dc.Cmd, " ")
		return ExecError{
			Command: []string{dc.Executable},
			Error:   fmt.Errorf("Local Docker compose exited abnormally whilst running %s: [%v]. %s", dc.Executable, args, err.Error()),
		}
	}

	return execErr
}

// capturingPassThroughWriter is a writer that remembers
// data written to it and passes it to w
type capturingPassThroughWriter struct {
	buf bytes.Buffer
	w   io.Writer
}

// newCapturingPassThroughWriter creates new capturingPassThroughWriter
func newCapturingPassThroughWriter(w io.Writer) *capturingPassThroughWriter {
	return &capturingPassThroughWriter{
		w: w,
	}
}

func (w *capturingPassThroughWriter) Write(d []byte) (int, error) {
	w.buf.Write(d)
	return w.w.Write(d)
}

// Bytes returns bytes written to the writer
func (w *capturingPassThroughWriter) Bytes() []byte {
	return w.buf.Bytes()
}

// Which checks if a binary is present in PATH
func which(binary string) error {
	_, err := exec.LookPath(binary)

	return err
}