package main

import (
	"bufio"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	. "fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type commitObject struct {
	hash    string
	author  string
	message string
}

func main() {
	var command string
	if len(os.Args) < 2 {
		command = "--help"
	} else {
		command = os.Args[1]
	}

	err := os.MkdirAll("./vcs", os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	os.MkdirAll("./vcs/commits", os.ModePerm)
	os.OpenFile("./vcs/log.txt", os.O_CREATE|os.O_RDWR, 0644)

	switch command {
	case "--help":
		Println("These are SVCS commands:")
		Println("config     Get and set a username.")
		Println("add        Add a file to the index.")
		Println("log        Show commit logs.")
		Println("commit     Save changes.")
		Println("checkout   Restore a file.")
	case "config":
		Println(config())
	case "add":
		addFile()
	case "log":
		showLogs()
	case "commit":
		commitToFile()
	case "checkout":
		checkoutFile()
	default:
		Printf("'%s' is not a SVCS command.\n", command)
	}
}

func config() string {
	file, err := os.OpenFile("./vcs/config.txt", os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	if len(os.Args) == 2 {
		b, errRd := os.ReadFile("./vcs/config.txt")
		if errRd != nil {
			log.Fatal(errRd)
		}

		if string(b) == "" {
			return "Please, tell me who you are."
		} else {
			return string(b)
		}
	}

	name := os.Args[2]
	result := Sprintf("The username is %s.", name)
	if _, err = file.WriteString(result); err != nil {
		log.Fatal(err)
	}
	return result
}

func addFile() {
	file, err := os.OpenFile("./vcs/index.txt", os.O_APPEND|os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	var (
		targetFile string
		found      bool
	)

	if len(os.Args) != 3 {
		data, errRd := os.ReadFile("./vcs/index.txt")
		if errRd != nil {
			log.Fatal(errRd)
		}

		if string(data) == "Tracked files:" || string(data) == "" {
			Println("Add a file to the index.")
			return
		} else {
			Println("Tracked files:")
			scanner := bufio.NewScanner(strings.NewReader(string(data)))
			for scanner.Scan() {
				line := scanner.Text()
				if line == "Tracked files:" {
					continue
				}
				Println(line)
			}
		}
		return
	} else {
		targetFile = os.Args[2]
	}

	_, err = file.WriteString("Tracked files:\n")
	if err != nil {
		log.Fatal(err)
	}

	err = filepath.Walk("./", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if path == targetFile {
			if _, err = Fprintln(file, targetFile); err != nil {
				log.Fatal(err)
			}
			found = true
		}
		// Println(path)
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	if !found {
		Printf("Can't find '%s'.\n", targetFile)
		return
	}

	Printf("The file '%s' is tracked.\n", targetFile)
}

func commitToFile() {
	var (
		message, currentDir string
	)
	notChanged := true

	if len(os.Args) != 3 {
		Println("Message was not passed.")
		return
	} else {
		message = strings.Trim(os.Args[2], "\"")
	}

	file, err := os.OpenFile("./vcs/index.txt", os.O_RDWR, 0644)
	fileStat, _ := file.Stat()
	if os.IsNotExist(err) || fileStat.Size() == 0 {
		Println("Nothing to commit.")
		return
	}
	defer file.Close()

	entries, _ := os.ReadDir("./vcs/commits")
	if len(entries) == 0 {
		injectCommit(file, message)
		return
	}

	currentDir = entries[len(entries)-1].Name()
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		pathFile := scanner.Text()
		if pathFile == "Tracked files:" {
			continue
		}

		err = filepath.Walk("./vcs/commits/"+currentDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if filepath.Base(path) == strings.TrimSpace(pathFile) {
				sha256Curr := sha256.New()
				sha256Commd := sha256.New()

				currFile, _ := os.Open("./" + pathFile)
				thisFile, _ := os.Open(path)

				io.Copy(sha256Curr, currFile)
				io.Copy(sha256Commd, thisFile)

				if hex.EncodeToString(sha256Curr.Sum(nil)) != hex.EncodeToString(sha256Commd.Sum(nil)) {
					notChanged = false
					return io.EOF
				}
				currFile.Close()
				thisFile.Close()
			}
			return nil
		})
		if !notChanged {
			break
		}
	}
	if !notChanged {
		file.Seek(0, 0)
		injectCommit(file, message)
		return
	}
	Println("Nothing to commit.")
	return
}

func injectCommit(file *os.File, message string) {
	var hash [20]byte
	hash = sha1.Sum([]byte(message))
	hashString := hex.EncodeToString(hash[:])

	err := os.Mkdir("./vcs/commits/"+hashString, os.ModePerm)
	if os.IsExist(err) {
		hash = sha1.Sum([]byte(hashString))
		hashString = hex.EncodeToString(hash[:])
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "Tracked files:" {
			continue
		}

		currFile, _ := os.ReadFile(line)
		err = os.WriteFile("./vcs/commits/"+hashString+"/"+line, currFile, 0644)
		if err != nil {
			log.Fatal(err)
		}

	}
	saveToLog(hashString, message)
	Println("Changes are committed.")
}

func saveToLog(hash, message string) {
	var name string
	file, err := os.OpenFile("./vcs/log.txt", os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	configFile, errConf := os.OpenFile("./vcs/config.txt", os.O_RDONLY, 0644)
	if errConf != nil {
		log.Fatal(errConf)
	}
	defer configFile.Close()

	scanner := bufio.NewScanner(configFile)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "The username is") {
			name = strings.Trim(line, "The username is ")
			name = strings.Trim(name, ".")
			break
		}
	}

	logMessage := Sprintf("%s %s %s", hash, name, message)
	if _, err = Fprintln(file, logMessage); err != nil {
		log.Fatal(err)
	}
}

func showLogs() {
	var commits []commitObject

	file, err := os.OpenFile("./vcs/log.txt", os.O_RDONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	entries, errGet := os.ReadDir("./vcs/commits")
	if errGet != nil {
		log.Fatal(errGet)
	}

	if len(entries) == 0 {
		Println("No commits yet.")
		return
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var commit commitObject
		line := scanner.Text()
		lineSlc := strings.Split(line, " ")

		commit.hash = lineSlc[0]
		commit.author = lineSlc[1]
		commit.message = strings.Join(lineSlc[2:], " ")

		commits = append(commits, commit)
	}

	slices.Reverse(commits)

	for _, commit := range commits {
		commitLog := Sprintf("commit %s\nAuthor: %s\n%s\n", commit.hash, commit.author, commit.message)
		Println(commitLog)
	}
}

func checkoutFile() {
	var idFound bool

	if len(os.Args) != 3 {
		Println("Commit id was not passed.")
		return
	}

	hashId := os.Args[2]
	entries, _ := os.ReadDir("./vcs/commits")
	for _, entry := range entries {
		if entry.Name() == hashId {
			idFound = true
			break
		}
	}

	if !idFound {
		Println("Commit does not exist.")
		return
	}

	entryFile, _ := os.ReadDir("./vcs/commits/" + hashId)
	for _, entry := range entryFile {
		dstFile, _ := os.OpenFile("./"+entry.Name(), os.O_RDWR, 0644)
		srcFile, _ := os.ReadFile("./vcs/commits/" + hashId + "/" + entry.Name())
		dstFile.Truncate(0)
		os.WriteFile("./"+entry.Name(), srcFile, 0644)
		dstFile.Close()
	}

	Printf("Switched to commit %s.\n", hashId)
}
