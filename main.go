package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var Secret string

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

func createRandomKey() string {
	buff := make([]byte, 200)
	rand.Read(buff)
	return hex.EncodeToString(buff)
}

func createToken(params []string) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"params":    strings.Join(params, "|"),
		"createdAt": time.Now(),
	})
	o, err := token.SignedString([]byte(Secret))

	if err != nil {
		fmt.Printf("Unable to create token: %s\n", err.Error())
		return ""
	}
	return o
}

func parseToken(tokenString string) (*jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(Secret), nil
	})
	if err != nil {
		fmt.Printf("token is invalid : %s\n", err.Error())
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		return &claims, nil
	} else {
		return nil, errors.New("claim cannot be verified")
	}
}

func main() {
	args := os.Args
	println(strings.Join(args, " "))
	if slices.Contains(args, "--token:generate") {
		println("Please copy the following secret carefully and keep it safe\n")
		println(createToken(args[slices.Index(args, "--token:generate"):]))
		println("")
		return
	}

	if slices.Contains(args, "--key:generate") {
		print(createRandomKey())
		return
	}

	http.HandleFunc("/{cmd}", func(w http.ResponseWriter, r *http.Request) {
		token := strings.Trim(r.Header.Get("X-Token"), " ")
		_, err := parseToken(token)
		if len(token) > 5 && err != nil {
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
