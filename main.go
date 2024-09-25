package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

const TOKEN string = "123456"

func executeShellCommand(binary string, r *http.Request, args ...string) ([]byte, error) {
	allowed_cmds := []string{"ls", "ll", "rmdir", "systemctl", "rm", "csync"}
	if !slices.Contains(allowed_cmds, binary) {
		return nil, errors.New("commandh invalid or not allowed")
	}
	// trim args
	for index, param := range args {
		args[index] = strings.Trim(param, "")
	}

	switch binary {
	case "csync":
		target := args[0]
		if !strings.HasPrefix(target, "/var/www/html/") {
			return nil, errors.New("no target has defined")
		}
		r.ParseMultipartForm(50 << 20) // 50 mb
		file, handler, err := r.FormFile("file")
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve the file: %s", err.Error())
		}
		defer file.Close()

		tmpLocationZip := filepath.Join("/tmp", handler.Filename)
		dst, err := os.Create(tmpLocationZip)
		if err != nil {
			return nil, fmt.Errorf("failed to upload file on server: %s", err.Error())
		}
		defer dst.Close()

		// Copy the file content to the destination
		_, err = io.Copy(dst, file)
		if err != nil {
			return nil, fmt.Errorf("failed to upload file on server: %s", err.Error())
		}

		cmd := exec.Command("unzip", tmpLocationZip, "-d", target)
		return cmd.Output()

	case "rmdir":
		// verify if is a deletable directory
		dir := args[0]
		if !strings.HasPrefix(dir, "/var/www/html") && dir != "/var/www/html" && dir != "/var/www/html/default" {
			return nil, errors.New("invalid directory path or not allowed")
		}
		cmd := exec.Command("rm", "-rf", dir)
		return cmd.Output()
	case "rm":
		file := args[0]
		if !strings.HasPrefix(file, "/var/www/html") && !strings.HasPrefix(file, "/var/www/html/default") {
			return nil, errors.New("files cannot be deleted in given directory")
		}
		cmd := exec.Command("rm", file)
		return cmd.Output()
	case "systemctl":
		action := args[0]
		if !slices.Contains([]string{"restart", "start", "stop"}, action) {
			return nil, errors.New("only restart|start|stop actions are allowed")
		}
		cmd := exec.Command(binary, args...)
		return cmd.Output()
	case "ls":
		cmd := exec.Command(binary, "-a")
		return cmd.Output()
	case "ll":
		cmd := exec.Command(binary, "-a")
		return cmd.Output()
	default:
		cmd := exec.Command(binary)
		return cmd.Output()
	}
}

func main() {
	http.HandleFunc("/{cmd}", func(w http.ResponseWriter, r *http.Request) {
		token := strings.Trim(r.Header.Get("X-Token"), " ")
		if token != TOKEN {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Unauthorized\n"))
			return
		}
		binary := r.PathValue("cmd")
		params := strings.Split(r.FormValue("args"), "|")

		fmt.Printf("%s %s \n", binary, strings.Join(params, " "))
		out, err := executeShellCommand(binary, r, params...)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		w.Write(out)
	})
	// Start the server
	port := ":21000"
	fmt.Printf("Starting server on %s...\n", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		fmt.Printf("Failed to start server: %s\n", err)
	}
}
