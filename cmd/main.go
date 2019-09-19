package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"net/http"
	"io/ioutil"
	"encoding/json"

	"github.com/policygenius/monday/pkg/config"
	"github.com/policygenius/monday/pkg/forwarder"
	"github.com/policygenius/monday/pkg/hostfile"
	"github.com/policygenius/monday/pkg/proxy"
	"github.com/policygenius/monday/pkg/runner"
	"github.com/policygenius/monday/pkg/ui"
	"github.com/policygenius/monday/pkg/watcher"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"github.com/jroimartin/gocui"
)

const (
	name = "bifrost"
)

var (
	Version string

	proxyComponent     *proxy.Proxy
	forwarderComponent *forwarder.Forwarder
	runnerComponent    *runner.Runner
	watcherComponent   *watcher.Watcher

	openerCommand string

	uiEnabled = len(os.Getenv("BIFROST_ENABLE_UI")) > 0
)

func main() {
	checkVersion()
	initRuntimeEnvironment()

	rootCmd := &cobra.Command{
		Run: func(cmd *cobra.Command, args []string) {
			if !uiEnabled {
				uiEnabled, _ = strconv.ParseBool(cmd.Flag("ui").Value.String())
			}

			conf, err := config.Load()
			if err != nil {
				fmt.Printf("❌  %v", err)
				return
			}

			choice := selectProject(conf)
			run(conf, choice)

			handleExitSignal()
		},
	}

	// UI-enable flag (for both root and run commands)
	runCmd.Flags().Bool("ui", false, "Enable the terminal UI")
	rootCmd.Flags().Bool("ui", false, "Enable the terminal UI")

	rootCmd.AddCommand(completionCmd)
	rootCmd.AddCommand(editCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(runCmd)
	//rootCmd.AddCommand(upgradeCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(rebuildCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("❌  An error has occured during 'edit' command: %v\n", err)
		os.Exit(1)
	}
}

func initRuntimeEnvironment() {
	switch runtime.GOOS {
	case "darwin":
		openerCommand = "open"

	default:
		openerCommand = "vim"
	}
}

func selectProject(conf *config.Config) string {
	prompt := promptui.Select{
		Label: "Which project do you want to work on?",
		Items: conf.GetProjectNames(),
		Size:  20,
	}

	_, choice, err := prompt.Run()
	if err != nil {
		panic(fmt.Sprintf("Prompt failed:\n%v", err))
	}

	fmt.Print("\n")

	return choice
}

func run(conf *config.Config, choice string) {
	layout := ui.NewLayout(uiEnabled)
	layout.Init()

	// Retrieve selected project configuration by its name
	project, err := conf.GetProjectByName(choice)
	if err != nil {
		panic(err)
	}

	// Initializes hosts file manager
	hostfile, err := hostfile.NewClient()
	if err != nil {
		panic(err)
	}

	proxyComponent = proxy.NewProxy(layout.GetProxyView(), hostfile)
	runnerComponent = runner.NewRunner(layout.GetLogsView(), proxyComponent, project)
	forwarderComponent = forwarder.NewForwarder(layout.GetForwardsView(), proxyComponent, project)

	watcherComponent = watcher.NewWatcher(runnerComponent, forwarderComponent, conf.Watcher, project)
	watcherComponent.Watch()

	if uiEnabled {
		defer layout.GetGui().Close()

		if err := layout.GetGui().SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
			panic(err)
		}

		layout.GetStatusView().Writef(" ⇢  %s | Commands: ←/→: select view | ↑/↓: scroll up/down | a: toggle autoscroll | f: toggle fullscreen", choice)

		if err := layout.GetGui().MainLoop(); err != nil && err != gocui.ErrQuit {
			fmt.Println(err)
			stopAll()
		}
	}
}

func checkVersion(){
	var vers string
	vers = "https://api.github.com/repos/policygenius/monday/releases/latest"

	resp, err := http.Get(vers)
	if err != nil{
		fmt.Printf("Internal error on startup, could not check version: %v\n", err)
		return
	}
	dat, _ := ioutil.ReadAll(resp.Body)
	githubResp := &GithubAPIResponse{}
	json.Unmarshal(dat, githubResp)
	if githubResp.TagName != Version {
		fmt.Printf("🐢 There is a new version of bifrost available\n🐢 run ./install --upgrade to get newest version \n")
	}
	return

}
// Handle for an exit signal in order to quit application on a proper way (shutting down connections and servers).
func handleExitSignal() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop

	stopAll()
}

func stopAll() {
	fmt.Println("\n👋  Bye, closing your local applications and remote connections now")

	forwarderComponent.Stop()
	proxyComponent.Stop()
	runnerComponent.Stop()
	watcherComponent.Stop()

	os.Exit(0)
}

func quit(g *gocui.Gui, v *gocui.View) error {
	g.Close()
	stopAll()
	return gocui.ErrQuit
}

