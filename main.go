package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const newline = "\n"
const appname = "pkgstate"

type Pkg_config struct {
	Update    string
	Install   string
	Remove    string
	Installed string
}

func run_cmd(cmd string) error {
	rslt := exec.Command("/bin/sh", "-c", cmd)
	rslt.Stdin = os.Stdin
	rslt.Stdout = os.Stdout
	rslt.Stderr = os.Stderr
	err := rslt.Run()
	return (err)
}

func update_all(pkg_config Pkg_config) {
	err := run_cmd(pkg_config.Update)
	if err != nil {
		panic(err)
	}
}

func pkg_conf_exists(pkg_manager string) bool {
	// Check in user config folder
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		panic(err)
	}

	path := filepath.Join(userConfigDir, appname, "pkgs_config", pkg_manager+".json")
	_, err = os.Stat(path)
	if err == nil {
		return true
	}

	// Check in same folder as executable
	path, err = filepath.Abs(pkg_manager + ".json")
	if err != nil {
		panic(err)
	}
	_, err = os.Stat(path)
	return err == nil
}

func parse_config(file_path string) map[string][]string {
	result := make(map[string][]string)
	file, err := os.Open(file_path)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	section := "DEFAULT"
	validLine := regexp.MustCompile(`^[a-zA-Z0-9]`)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line[1 : len(line)-1]
		} else {
			if validLine.MatchString(line) {
				result[section] = append(result[section], line)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
	return result
}

func get_config_path() string {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		panic(err)
	}

	config_path := filepath.Join(userConfigDir, appname, "pkgs.ini")
	_, err = os.Stat(config_path)
	if err != nil {
		config_path = filepath.Join(".", "pkgs.ini")
	}

	return config_path
}

func pprint(o any) {
	rslt, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s\n", rslt)
}

func get_pkg_config(pkg_manager string) Pkg_config {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		panic(err)
	}

	config_path := filepath.Join(userConfigDir, appname, pkg_manager+".json")
	_, err = os.Stat(config_path)
	if err != nil {
		config_path = filepath.Join("./", pkg_manager+".json")
	}
	content, err := os.ReadFile(config_path)
	if err != nil {
		panic(err)
	}

	var pkg_config Pkg_config
	json.Unmarshal(content, &pkg_config)
	return pkg_config
}

func is_pkg_installed(pkg string, pkg_config Pkg_config) bool {
	cmd := strings.ReplaceAll(pkg_config.Installed, "<pkg>", pkg)
	err := run_cmd(cmd)
	return err == nil
}

func pkgs_install(pkgs []string, pkg_config Pkg_config) {
	cmd := strings.ReplaceAll(pkg_config.Install, "<pkgs>", strings.Join(pkgs, " "))
	err := run_cmd(cmd)
	if err != nil {
		if len(pkgs) > 1 {
			for _, pkg := range pkgs {
				pkgs_install([]string{pkg}, pkg_config)
			}
		}
	}
}

func pkgs_remove(pkgs []string, pkg_config Pkg_config) {
	cmd := strings.ReplaceAll(pkg_config.Remove, "<pkgs>", strings.Join(pkgs, " "))
	err := run_cmd(cmd)
	if err != nil {
		if len(pkgs) > 1 {
			for _, pkg := range pkgs {
				pkgs_remove([]string{pkg}, pkg_config)
			}
		}
	}
}

func get_state_path(pkg_manager string) string {
	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		panic(err)
	}
	path := filepath.Join(userCacheDir, appname)
	err = os.MkdirAll(path, 0750)
	if err != nil {
		panic(err)
	}
	state_file_path := filepath.Join(path, pkg_manager+".state")
	return state_file_path
}

func get_state(pkg_manager string) []string {
	state_path := get_state_path(pkg_manager)
	_, err := os.Stat(state_path)
	if err != nil {
		_, err = os.Create(state_path)
		if err != nil {
			panic(err)
		}
	}
	rslt, err := os.ReadFile(state_path)
	if err != nil {
		panic(err)
	}
	return strings.Split(string(rslt), newline)
}

func save_state(pkg_manager string, pkgs []string) {
	data := strings.Join(pkgs, newline)
	state_file, err := os.Create(get_state_path(pkg_manager))
	if err != nil {
		panic(err)
	}
	defer state_file.Close()
	state_file.WriteString(data)
}

func sync_pkgs(pkg_manager string, pkgs []string) {
	pkg_config := get_pkg_config(pkg_manager)

	update_all(pkg_config)
	pkgs_to_install := pkgs[:0]
	for _, x := range pkgs {
		if !is_pkg_installed(x, pkg_config) {
			pkgs_to_install = append(pkgs_to_install, x)
		}
	}
	if len(pkgs_to_install) > 0 {
		pkgs_install(pkgs_to_install, pkg_config)
	}

	state := get_state(pkg_manager)
	var to_remove []string
	if len(state) > 0 {
		to_remove = state[:0]
		for _, pkg := range state {
			if !slices.Contains(pkgs, pkg) {
				to_remove = append(to_remove, pkg)
			}
		}
	}
	if len(pkgs) > 0 {
		save_state(pkg_manager, pkgs)
	}
	if len(to_remove) > 0 {
		pkgs_remove(to_remove, pkg_config)
	}
}

func main() {
	pkgs_config := parse_config(get_config_path())
	for pkg_manager, pkgs := range pkgs_config {
		if pkg_conf_exists(pkg_manager) {
			sync_pkgs(pkg_manager, pkgs)
		}
	}
}
