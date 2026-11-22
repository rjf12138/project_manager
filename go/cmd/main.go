package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rjf12138/project_manager/core"
	"github.com/rjf12138/project_manager/tui"
)

var solution *core.Solution
var initialized bool

func ensureInit() {
	if !initialized {
		solution = core.NewSolution()
		if err := solution.Init(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		initialized = true
	}
}

func printHelp() {
	fmt.Println("pm - Project Manager")
	fmt.Println()
	fmt.Println("Project management:")
	fmt.Println("  pm init <name> [parent-path]              Create a new project")
	fmt.Println("  pm load <path>                            Load/register an existing project")
	fmt.Println("  pm list                                   List registered projects")
	fmt.Println("  pm remove <name|path>                     Unregister a project (files kept)")
	fmt.Println()
	fmt.Println("Build & run:")
	fmt.Println("  pm build [name] [-t debug|release] [-r]   Build a project (-r = clean first)")
	fmt.Println("  pm clean [name]                           Remove build artifacts")
	fmt.Println("  pm run [name] [-- args...]                Run the built executable")
	fmt.Println()
	fmt.Println("Dependencies (project level):")
	fmt.Println("  pm deps install [name]                    Install all declared dependencies")
	fmt.Println("  pm deps add <pkg>[@ver] [-p project]      Add and install a dependency")
	fmt.Println("  pm deps remove <pkg> [-p project]         Remove a dependency + its installed files")
	fmt.Println("                     [--keep-files]        Only drop the declaration, keep files")
	fmt.Println("  pm deps list [name]                       List dependencies")
	fmt.Println()
	fmt.Println("Package repository:")
	fmt.Println("  pm pkg create <dir> [-o out.pm] [-f]      Build a .pm package from a directory")
	fmt.Println("  pm pkg install <pkg>[@ver] [-p project]   Install a package into a project")
	fmt.Println("  pm pkg remove <pkg>[@ver]                 Remove a package from the repository")
	fmt.Println("  pm pkg list                               List packages in the repository")
	fmt.Println("  pm pkg search [keyword]                   Search packages")
	fmt.Println("  pm pkg dir                                Print the repository directory")
	fmt.Println()
	fmt.Println("Interactive (TUI):")
	fmt.Println("  pm tui                                    Launch full TUI")
	fmt.Println("  pm -l                                     Select a project and open it in VS Code (TUI)")
	fmt.Println("  pm -cfg                                   Configure active project (TUI)")
	fmt.Println()
	fmt.Println("Shortcuts (active project):")
	fmt.Println("  pm -r                                     Incremental build (no clean)")
	fmt.Println("  pm -c                                     Clean active project artifacts")
	fmt.Println("  pm -cr                                    Clean & rebuild active project")
	fmt.Println()
	fmt.Println("  pm -h, --help                             Print this help")
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		printHelp()
		return
	}

	cmd := args[0]
	rest := args[1:]

	var err error

	switch cmd {
	case "-h", "--help", "help":
		printHelp()
		return

	case "tui":
		ensureInit()
		err = tui.RunTUI(solution)

	case "-l":
		ensureInit()
		err = tui.RunProjectList(solution)

	case "-cfg":
		ensureInit()
		err = tui.RunConfig(solution)

	case "-r":
		ensureInit()
		err = tui.RunBuild(solution)
		if err == nil {
			fmt.Println("Build completed.")
		}

	case "-c":
		ensureInit()
		err = tui.RunClean(solution)
		if err == nil {
			fmt.Println("Clean completed.")
		}

	case "-cr":
		ensureInit()
		err = tui.RunCleanRebuild(solution)
		if err == nil {
			fmt.Println("Clean rebuild completed.")
		}

	case "init":
		err = cmdInit(rest)
	case "load":
		err = cmdLoad(rest)
	case "list":
		err = cmdList(rest)
	case "remove":
		err = cmdRemove(rest)

	case "build":
		err = cmdBuild(rest)
	case "clean":
		err = cmdClean(rest)
	case "run":
		err = cmdRun(rest)

	case "deps":
		err = cmdDeps(rest)
	case "pkg":
		err = cmdPkg(rest)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printHelp()
		os.Exit(2)
	}

	if err == flag.ErrHelp {
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
