package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"crypto/md5"
	"bytes"
)

const VERSION = "1.0"
const AUTHOR = "Alexander Weigl <weigl@kit.edu>"

const BOLD_SEPARATOR = "\n================================================================================\n";

const NORMAL_SEPARATOR = "\n--------------------------------------------------------------------------------\n";

type Leaderboard []Record

type LeaderboardService struct {
	BoardFileName       string
	CurrentBoard        Leaderboard
	Script              string
	Title               string
	Description         string
	TemplateFile        string
	template            string
	Endpoint            string
	SubmissionFilename  string
	SubmissionFolder    string
	ReevaluationAllowed bool
}

func (this *LeaderboardService) load() {

	dat, err := ioutil.ReadFile(this.BoardFileName)
	if err == nil {
		err := json.Unmarshal(dat, &this.CurrentBoard)
		if err != nil {
			log.Printf("Could not parse current board: %s, %s\n",
				this.BoardFileName, err)
		} else {
			log.Printf("Board loaded: %s with %d enries.\n", this.BoardFileName, this.CurrentBoard.Len())
			sort.Sort(this.CurrentBoard)
		}
	} else {
		log.Printf("Could not read current board: %s\n", this.BoardFileName)
		log.Println(err)
	}

	a, err := ioutil.ReadFile(this.TemplateFile)
	if err != nil {
		log.Printf("Could not read template file %s. Create it.\n", this.TemplateFile)
		ioutil.WriteFile(this.TemplateFile, []byte{}, os.ModePerm)
	} else {
		this.template = string(a[:])
	}

	err = os.MkdirAll(this.SubmissionFolder, os.ModePerm)
	if err != nil {
		log.Printf("Could not create submission folder: %s", this.SubmissionFolder)
	}
}

func (this *LeaderboardService) add(r Record) int {
	this.CurrentBoard = append(this.CurrentBoard, r)
	sort.Sort(this.CurrentBoard)

	data, err := json.Marshal(this.CurrentBoard)
	if err != nil {
		log.Print(err)
	} else {
		ioutil.WriteFile(this.BoardFileName, data, os.ModePerm)
	}

	for i, v := range this.CurrentBoard {
		if v.Id == r.Id {
			return i + 1
		}
	}
	return 0
}

func (this *LeaderboardService) exists(h [16]byte) bool {
	for _, e := range this.CurrentBoard {
		if bytes.Equal(e.Hash[:], h[:]) {
			return true
		}
	}
	return false
}

func (this *LeaderboardService) submit(w http.ResponseWriter, r *http.Request) {
	entry := Record{}
	entry.Id = strconv.FormatUint(rand.Uint64(), 16)
	folder, err := ioutil.TempDir(this.SubmissionFolder, entry.Id)

	log, _ := os.Create(path.Join("/tmp", entry.Id+".log"))
	defer log.Close()

	sink := io.MultiWriter(w, log)

	fmt.Fprintf(sink, "Your submission id is %s\n", entry.Id)

	if err != nil {
		fmt.Fprintf(sink, "%s\n", err)
		return
	}

	folder, err = filepath.Abs(folder)
	if err != nil {
		fmt.Fprintf(sink, "%s\n", err)
		return
	}

	content, err := ioutil.ReadAll(r.Body)
	entry.Hash = md5.Sum(content)
	if err != nil {
		fmt.Fprintf(sink, "%s\n", err)
		return
	}

	if (this.exists(entry.Hash)) {
		fmt.Fprintf(sink, "*** A submission already exists with hash: %s.", entry.Hash)
		if (!this.ReevaluationAllowed) {
			fmt.Fprintf(sink, "*** Abort, re-evaluation is forbidden.", entry.Hash)
			return
		}
	}

	target := path.Clean(path.Join(folder, this.SubmissionFilename))
	ioutil.WriteFile(target, content, os.ModePerm)

	//fmt.Fprintf(sink, "Symlinking %s to %s\n", this.Script, path.Join(folder, this.Script))
	//os.Symlink(this.Script, path.Join(folder, this.Script))

	fmt.Fprintf(sink, "Environment is set up.\n")

	script, _ := filepath.Abs(this.Script)
	cmd := exec.Command("sh", "-c", script)
	//"/usr/bin/time", "-f", "'[%U,%S,%e]'", script)
	cmd.Dir, _ = filepath.Abs(folder)

	start := time.Now()
	output, err := cmd.CombinedOutput()
	elapsed := time.Since(start)

	fmt.Fprint(sink, "**************************************************************************\n")
	fmt.Fprint(sink, "**************************   stdout/stderr   *****************************\n")
	fmt.Fprint(sink, string(output[:]))
	fmt.Fprint(sink, "\n**************************************************************************\n")

	if err != nil {
		fmt.Fprintln(sink, err)
		return
	}

	exitStatus := cmd.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()

	fmt.Fprintf(sink, "Process exited with %d\n", exitStatus)
	fmt.Fprintf(sink, "Runtime: user/system/real = %5.2f / %5.2f / %5.2f\n",
		cmd.ProcessState.UserTime().Seconds(),
		cmd.ProcessState.SystemTime().Seconds(),
		elapsed.Seconds())

	if err != nil {
		fmt.Fprintf(sink, "Error occured during process. %s\n", err)
		fmt.Fprintf(sink, "*** Submission rejected ***\n")
		return
	}

	entry.Name = r.FormValue("alias")
	entry.Date = time.Now().Format(time.RFC3339)
	entry.Runtime = cmd.ProcessState.UserTime().Seconds()
	entry.Score = extractScore(output)

	pos := this.add(entry)
	fmt.Fprintf(sink, "Your position is %d.\n", pos)
}

func extractScore(bytes []byte) int {
	re, err := regexp.Compile(`score[ ]*=[ ]*(\d+)`)
	if err != nil {
		log.Fatal(err)
	}
	score := re.FindSubmatch(bytes)
	if score != nil {
		return int(score[0][1])
	} else {
		return 0
	}
}

func (this *LeaderboardService) show(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, this.template, this.Title, this.Description)
	fmt.Fprintf(w, " #  %-20s %4s %6s %-30s", "NAME", "PTS", "TIME", "DATE")
	fmt.Fprintf(w, NORMAL_SEPARATOR)
	for i, e := range this.CurrentBoard {
		fmt.Fprintf(w, "%03d %-20s %4d %6.3f %-30s\n",
			i+1, e.Id, e.Score, e.Runtime, e.Date)
		if i > 25 { // only showing top 25.
			break
		}
	}
	fmt.Fprintf(w, "\n"+NORMAL_SEPARATOR)
	r.ParseForm()
	yourIds := strings.Split(r.FormValue("ids"), ",")
outer:
	for _, yourId := range yourIds {
		for rank, entry := range this.CurrentBoard {
			if (strings.EqualFold(entry.Id, yourId)) {
				fmt.Fprintf(w, "Your submission %s on rank %d!", yourId, rank);
				continue outer
			}
		}
		fmt.Fprintf(w, "Submission %s not found!", yourId);
	}

	fmt.Fprintln(w, BOLD_SEPARATOR);
	fmt.Fprintf(w, "Server version: %s\tServer time: %s\n", VERSION,
		time.Now().Format(time.RFC3339));
}

func (this *LeaderboardService) handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain")
	if r.Method == "POST" {
		this.submit(w, r)
	} else {
		this.show(w, r)
	}
}

const CONFIG = "config.json"

func main() {
	var config []LeaderboardService
	jsonCfg, err := ioutil.ReadFile(CONFIG)
	if err != nil {
		log.Printf("Could not load config file: %s", CONFIG)
		log.Fatal(err)
	} else {
		err := json.Unmarshal(jsonCfg, &config)

		if err != nil {
			log.Printf("Could not interpret config file: %s", CONFIG)
			log.Fatal(err)
		}

		for _, serv := range config {
			serv.load()
			log.Printf("Register %s at %s. Board goes to: %s\n",
				serv.Title, serv.Endpoint, serv.BoardFileName)
			http.HandleFunc(serv.Endpoint, serv.handler)
		}
		log.Fatal(http.ListenAndServe(":8080", nil))
	}
}

type Record struct {
	Id      string
	Name    string
	Score   int
	Runtime float64
	Date    string
	Hash    [16]byte
}

func (a Leaderboard) Len() int      { return len(a) }
func (a Leaderboard) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a Leaderboard) Less(i, j int) bool {
	x := a[i]
	y := a[j]

	keys := []int{
		-(x.Score - y.Score), // reverse
		int((x.Runtime - y.Runtime) * 10000),
		strings.Compare(y.Date, x.Date),
	}

	for _, k := range keys {
		if k != 0 {
			return k < 0
		}
	}
	return false
}
