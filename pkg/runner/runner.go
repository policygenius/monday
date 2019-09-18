package runner

import (
	"fmt"
	"os"
	"io"
	"log"
	"os/exec"
	"strings"
	"bytes"
	"sync"
	"syscall"

	"github.com/policygenius/monday/pkg/config"
	"github.com/policygenius/monday/pkg/proxy"
	"github.com/policygenius/monday/pkg/ui"
)

var (
	execCommand = exec.Command
	hasSetup    = false
	debugMode = len(os.Getenv("BIFROST_ENABLE_DEBUG")) > 0
)

type RunnerInterface interface {
	RunAll()
	SetupAll()
	Run(application *config.Application)
	Restart(application *config.Application)
	Stop() error
}

// Runner is the struct that manage running local applications
type Runner struct {
	proxy        proxy.ProxyInterface
	projectName  string
	applications []*config.Application
	cmds         map[string]*exec.Cmd
	view         ui.ViewInterface
}

type interactiveWriter struct {
	buf bytes.Buffer
	w io.Writer
}

func NewinteractiveWriter(w io.Writer) *interactiveWriter {
	return &interactiveWriter{
		w: w,
	}
}

func (w *interactiveWriter) Write(d []byte) (int, error) {
	w.buf.Write(d)
	return w.w.Write(d)
}

func (w *interactiveWriter) Bytes() []byte {
	return w.buf.Bytes()
}

// NewRunner instancites a Runner struct from configuration data
func NewRunner(view ui.ViewInterface, proxy proxy.ProxyInterface, project *config.Project) *Runner {
	return &Runner{
		proxy:        proxy,
		projectName:  project.Name,
		applications: project.Applications,
		cmds:         make(map[string]*exec.Cmd, 0),
		view:         view,
	}
}

// RunAll runs all local applications in separated goroutines
func (r *Runner) RunAll() {
	for _, application := range r.applications {
		go r.Run(application)

		if application.Hostname != "" {
			proxyForward := proxy.NewProxyForward(application.Name, application.Hostname, "", "", "")
			r.proxy.AddProxyForward(application.Name, proxyForward)
		}
	}
}

// SetupAll runs setup commands for all applications in case their directory does not already exists
func (r *Runner) SetupAll() {
	var wg sync.WaitGroup

	for _, application := range r.applications {
		wg.Add(1)
		r.setup(application, &wg)
	}

	wg.Wait()

	if hasSetup {
		r.view.Write("\n✅  Setup complete!\n\n")
	}
}

// Run launches the application
func (r *Runner) Run(application *config.Application) {

	if debugMode {

		if err := r.checkApplicationExecutableEnvironment(application); err != nil {
			r.view.Writef("❌  %s\n", err.Error())
			return
		}

		r.view.Writef("⚙️   Running local app '%s' (%s)...\n", application.Name, application.Path)

		applicationPath := application.GetPath()

		stdoutStream := NewLogstreamer(StdOut, application.Name, r.view)
		stderrStream := NewLogstreamer(StdErr, application.Name, r.view)

		cmd := exec.Command(application.Executable, application.Args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: false}
		cmd.Dir = applicationPath
		cmd.Stdout = stdoutStream
		cmd.Stderr = stderrStream
		cmd.Stdin = os.Stdin

		cmd.Env = os.Environ()

		// Add environment variables
		for key, value := range application.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
		}

		r.cmds[application.Name] = cmd

		if err := cmd.Run(); err != nil {
			r.view.Writef("❌  Cannot run the following application: %s: %v\n", applicationPath, err)
			return
		}
		cmd.Stdin = os.Stdin
	}else {

		cmd := exec.Command(application.Executable, application.Args...)

		cmd.Dir = application.GetPath()

		var errStdout, errStderr error
		stdoutIn, _ := cmd.StdoutPipe()
		stderrIn, _ := cmd.StderrPipe()
		stdout := NewinteractiveWriter(os.Stdout)
		stderr := NewinteractiveWriter(os.Stderr)
		//cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Stdin = os.Stdin
		err := cmd.Start()
		if err != nil {
			log.Fatalf("Unable to run command: %s", err)
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
		if err != nil {
			log.Fatalf("Unable to run command")
		}
		if errStdout != nil || errStderr != nil {
			log.Fatalf("internal failure with std pipe")
		}
	}
	
}

// Restart kills the current application launch (if it exists) and launch a new one
func (r *Runner) Restart(application *config.Application) {
	if cmd, ok := r.cmds[application.Name]; ok {
		pgid, err := syscall.Getpgid(cmd.Process.Pid)
		if err == nil {
			syscall.Kill(-pgid, 15)
		}
	}

	go r.Run(application)
}

// Stop stops all the currently active local applications
func (r *Runner) Stop() error {
	for _, application := range r.applications {
		// Kill process
		if cmd, ok := r.cmds[application.Name]; ok {
			pgid, err := syscall.Getpgid(cmd.Process.Pid)
			if err == nil {
				syscall.Kill(-pgid, 15)
			}
		}

		// In case we have stop command, run it
		if application.StopExecutable != "" {
			err := exec.Command(application.StopExecutable, application.StopArgs...).Start()
			if err != nil {
				r.view.Writef("❌  Cannot run stop command for application '%s': %v\n", application.Name, err)
			}
		}
	}

	return nil
}

func (r *Runner) checkApplicationExecutableEnvironment(application *config.Application) error {
	applicationPath := application.GetPath()

	// Check application path exists
	if _, err := os.Stat(applicationPath); os.IsNotExist(err) {
		return fmt.Errorf("Unable to find application path: %s", applicationPath)
	}

	return nil
}

// Setup runs setup commands for a specified application
func (r *Runner) setup(application *config.Application, wg *sync.WaitGroup) error {
	defer wg.Done()

	if err := r.checkApplicationExecutableEnvironment(application); err == nil {
		return nil
	}

	if len(application.Setup) == 0 {
		return nil
	}

	hasSetup = true

	r.view.Writef("⚙️  Please wait while setup of application '%s'...\n", application.Name)

	stdoutStream := NewLogstreamer(StdOut, application.Name, r.view)
	stderrStream := NewLogstreamer(StdErr, application.Name, r.view)

	var setup = strings.Join(application.Setup, "; ")

	setup = strings.Replace(setup, "~", "$HOME", -1)
	setup = os.ExpandEnv(setup)

	commands := strings.Join(application.Setup, "\n")
	r.view.Writef("👉  Running commands:\n%s\n\n", commands)

	cmd := exec.Command("/bin/sh", "-c", setup)
	cmd.Stdout = stdoutStream
	cmd.Stderr = stderrStream
	cmd.Env = os.Environ()

	setup = os.ExpandEnv(setup)

	cmd.Run()

	return nil
}
