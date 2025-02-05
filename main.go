package main

import (
	"bytes"
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

type MyCustomClaims struct {
	WorkDir string `json:"workDir"`
	jwt.RegisteredClaims
}

var Secret string

func createToken(workDir string) *string {
	if len(Secret) < 50 {
		fmt.Println("Either key is missing or length is below 50")
		return nil
	}
	claims := MyCustomClaims{
		WorkDir: workDir,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	o, err := token.SignedString([]byte(Secret))

	if err != nil {
		fmt.Printf("Unable to create token: %s\n", err.Error())
		return nil
	}
	return &o
}

func parseToken(tokenString string) (*MyCustomClaims, error) {
	if len(Secret) < 50 {
		fmt.Println("Either key is missing or length is below 50")
		return nil, fmt.Errorf("Either key is missing or length is below 50")
	}

	token, err := jwt.ParseWithClaims(tokenString, &MyCustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(Secret), nil
	})

	if err != nil {
		fmt.Printf("token is invalid : %s\n", err.Error())
		return nil, err
	}

	if claims, ok := token.Claims.(*MyCustomClaims); ok {
		return claims, nil
	} else {
		return nil, errors.New("claim cannot be verified")
	}
}

func sendError(w http.ResponseWriter, message string) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(message))
}

func sendSuccess(w http.ResponseWriter, message string) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(message))
}

func hasDirectoryInArgs(args []string) bool {
	hasDir := false
	for _, arg := range args {
		_, err := os.Stat(arg)
		if err == nil {
			hasDir = true
			break
		}
	}
	return hasDir
}

func hasUnSafeCommand(binary string) bool {
	unsafe := false
	list := []string{"bash", "sh", "csh", "cat", "dd", ">", "wget", "find", "sudo"}
	for _, l := range list {
		if l == binary {
			unsafe = true
			break
		}
	}
	return unsafe
}

func main() {
	args := os.Args
	if slices.Contains(args, "--createtoken") {
		commandIndex := slices.Index(args, "--createtoken")
		workDir := ""
		if len(args) < commandIndex+1 {
			println("\n Sorry you should provide a workdir ex <binary> --createtoken /var/www/html/yourworkdir \n")
			return
		}

		workDir = args[commandIndex+1]
		if !strings.HasPrefix(workDir, "/var/www/html") {
			println("\n Sorry you should provide a workdir ex <binary> --token:generate /var/www/html/yourworkdir \n")
			return
		}

		println("Please copy the following secret carefully and keep it safe\n")
		token := createToken(workDir)
		println(*token)
		println("")
		return
	}

	http.HandleFunc("/{cmd}", func(w http.ResponseWriter, r *http.Request) {
		errMul := r.ParseMultipartForm(300 << 20) // 10MB
		err := r.ParseForm()

		if err != nil || errMul != nil {
			sendError(w, "Unable to parse form body")
			return
		}

		if strings.ToUpper(r.Method) != "POST" {
			sendError(w, "Method not supported")
			return
		}
		token := strings.Trim(r.Header.Get("X-Token"), " ")

		claims, err := parseToken(token)

		if claims == nil {
			sendError(w, "Unauthorized - claims")
			return
		}

		if len(token) < 5 || err != nil {
			sendError(w, "Unauthorized"+err.Error())
			return
		}

		binary := r.PathValue("cmd")
		args := strings.Trim(r.FormValue("args"), " ")

		if hasDirectoryInArgs(strings.Split(args, " ")) {
			sendError(w, "you cannot use directories directly {WORKDIR} var to replace it with working directory ex. args = {WORKDIR}/folder")
			return
		}

		args = strings.ReplaceAll(args, "{WORKDIR}", claims.WorkDir)

		file, handler, fileUploadErr := r.FormFile("file")

		if fileUploadErr == nil {
			defer file.Close()
			tmpLocation := filepath.Join(claims.WorkDir, handler.Filename)
			dst, createError := os.Create(tmpLocation)
			if createError != nil {
				sendError(w, createError.Error())
				return
			}
			defer dst.Close()
			// Copy the file content to the destination
			_, err = io.Copy(dst, file)
			if err != nil {
				sendError(w, err.Error())
				return
			}
			sendSuccess(w, "Uploaded successfully")
			return
		}

		if len(binary) < 1 {
			sendError(w, "No binary supplied")
			return
		}

		if hasUnSafeCommand(binary) {
			sendError(w, "Dangerous op not allowed.")
			return
		}

		if !strings.HasPrefix(claims.WorkDir, "/var/www/html") {
			sendError(w, "Workdir is incorrect, can't use service")
			return
		}

		var stdout bytes.Buffer
		var stderr bytes.Buffer

		fmt.Printf("%s %s \n", binary, args)

		binary = "/usr/bin/" + binary

		var cmd *exec.Cmd
		if binary == "/usr/bin/systemctl" {
			args += binary
			cmd = exec.Command("sudo", strings.Split(args, " ")...)
		} else {
			cmd = exec.Command(binary, strings.Split(args, " ")...)
		}
		cmd.Dir = claims.WorkDir
		cmd.Stderr = &stdout
		cmd.Stdout = &stdout

		cmdError := cmd.Run()

		if cmdError != nil {
			sendError(w, "\n"+stdout.String()+" \n"+stderr.String()+"\n"+cmdError.Error())
			return
		}

		sendSuccess(w, "\n"+stdout.String()+" \n"+stderr.String())
	})
	// Start the server
	server := &http.Server{
		Addr:         ":21000",
		ReadTimeout:  5 * time.Minute,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  5 * time.Minute,
	}
	port := ":21000"
	fmt.Printf("Starting server on %s...\n", port)
	if err := server.ListenAndServe(); err != nil {
		fmt.Printf("Failed to start server: %s\n", err)
	}
}
