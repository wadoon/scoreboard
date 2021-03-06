package main

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"github.com/plimble/ace"
	"io/ioutil"
	"log"
	"math/rand"
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
)

const VERSION = "1.0"
const AUTHOR = "Alexander Weigl <weigl@kit.edu>"
const BoldSeparator = "\n================================================================================\n"
const NormalSeparator = "\n--------------------------------------------------------------------------------\n"
const CONFIG = "config.json"
const OutputLog = "out.log"

func main() {
	channel := make(chan Submission, 25)

	var config []ServiceData
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

		ace := ace.New()
		for _, data := range config {
			New(&data, channel, ace)
			log.Printf("Register %s at %s. Board goes to: %s\n",
				data.Title, data.Endpoint, data.BoardFileName)
		}
		log.Printf("Start worker.\n")
		go StartWorker(channel)
		ace.Run(":8080")
	}
}

/////Datatypes
type ScoreBoard []Entry

type ServiceData struct {
	BoardFileName       string
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

type Service struct {
	data              *ServiceData
	currentBoard      ScoreBoard
	currentUnfinished ScoreBoard
	toWorker          chan Submission
	evalFinished      chan Entry
}

func New(data *ServiceData, worker chan Submission, ace *ace.Ace) Service {
	finished := make(chan Entry)
	s := Service{
		data:         data,
		toWorker:     worker,
		evalFinished: finished,
	}

	dat, err := ioutil.ReadFile(data.BoardFileName)
	if err == nil {
		err := json.Unmarshal(dat, &s.currentBoard)
		if err != nil {
			log.Printf("Could not parse current board: %s, %s\n",
				data.BoardFileName, err)
		} else {
			log.Printf("Board loaded: %s with %d enries.\n", data.BoardFileName, s.currentBoard.Len())
			sort.Sort(s.currentBoard)
		}
	} else {
		log.Printf("Could not read current board: %s\n", data.BoardFileName)
		log.Println(err)
	}

	a, err := ioutil.ReadFile(data.TemplateFile)
	if err != nil {
		log.Printf("Could not read template file %s. Create it.\n", data.TemplateFile)
		ioutil.WriteFile(data.TemplateFile, []byte{}, os.ModePerm)
	} else {
		log.Printf("Template file '%s' readed.\n", data.TemplateFile)
		data.template = string(a[:])
	}

	err = os.MkdirAll(data.SubmissionFolder, os.ModePerm)
	if err != nil {
		log.Printf("Could not create submission folder: %s", data.SubmissionFolder)
	}

	g := ace.Group(data.Endpoint)
	g.GET("/", s.show)
	g.GET("/submission/:id", s.jobStatus)
	g.POST("/submission", s.submit)

	//start a worker for finished jobs
	go func() {
		for {
			r := <-s.evalFinished
			s.currentBoard = append(s.currentBoard, r)
			data, err := json.Marshal(s.currentBoard)
			if err != nil {
				log.Println("Error marshaling board.", err)
			} else {
				err = ioutil.WriteFile(s.data.BoardFileName, data, os.ModePerm)
				if err != nil {
					log.Println("Error writing board.", err)
				}
			}
		}
	}()

	return s
}

func (s *Service) exists(h [16]byte) bool {
	for _, e := range s.currentBoard {
		if bytes.Equal(e.Hash[:], h[:]) {
			return true
		}
	}
	return false
}

func (s *Service) submit(c *ace.C) {
	//w := c.Writer
	r := c.Request

	entry := Entry{
		Id:       strconv.FormatUint(rand.Uint64(), 16),
		NickName: r.URL.Query().Get("nickname"),
		Score:    -1,
	}

	log.Printf("Incoming submission: nickname => %s", entry.NickName)

	folder, err := ioutil.TempDir(s.data.SubmissionFolder, entry.Id)
	//submissionLog, _ := os.Create(path.Join("/tmp", entry.Id+".submissionLog"))
	//defer submissionLog.Close()
	sink := c.Writer //io.MultiWriter(w, submissionLog)

	fmt.Fprintf(sink, "\n***************************************************\n")
	fmt.Fprintf(sink, "*** Your submission id is %-21s ***\n", entry.Id)
	fmt.Fprintf(sink, "***************************************************\n\n")
	if err != nil {
		fmt.Fprintf(sink, "%s\n", err)
		return
	}

	folder, err = filepath.Abs(folder)
	if err != nil {
		fmt.Fprintf(sink, "%s\n", err)
		return
	}
	entry.Folder = folder

	content, err := ioutil.ReadAll(r.Body)
	entry.Hash = md5.Sum(content)
	if err != nil {
		fmt.Fprintf(sink, "%s\n", err)
		return
	}

	if s.exists(entry.Hash) {
		fmt.Fprintf(sink, "!!! A submission exists already with the same content !!!\n")
		if !s.data.ReevaluationAllowed {
			fmt.Fprintln(sink, "!!!      Abort, re-evaluation is forbidden!    !!!\n")
			return
		}
	}

	target := path.Clean(path.Join(folder, s.data.SubmissionFilename))
	ioutil.WriteFile(target, content, os.ModePerm)

	//fmt.Fprintf(sink, "Symlinking %s to %s\n", s.Script, path.Join(folder, s.Script))
	//os.Symlink(s.Script, path.Join(folder, s.Script))

	entry.Script, _ = filepath.Abs(s.data.Script)

	fmt.Fprintf(sink, "\n\nSubmission successfully retrieved.\n\n")
	s.currentUnfinished = append(s.currentUnfinished, entry)
	pos := len(s.toWorker) + 1
	s.toWorker <- Submission{entry: entry, result: s.evalFinished}

	fmt.Fprintf(sink, "*** Your submission is enqueued at position: %d ***\n", pos)

	fmt.Fprintf(sink, "********************************************************************************\n")
	fmt.Fprintf(sink, "*** Please note down your submission id. You will need it later to retrieve  ***\n")
	fmt.Fprintf(sink, "*** its output, status and scoreboard rank.                                  ***\n")
	fmt.Fprintf(sink, "***                                                                          ***\n")
	fmt.Fprintf(sink, "*** Please use:    ./scoreboard.sh status                                    ***\n")
	fmt.Fprintf(sink, "********************************************************************************\n")
}

func (s *Service) show(c *ace.C) {
	w := c.Writer
	r := c.Request

	sort.Sort(s.currentBoard)

	fmt.Fprintf(w, s.data.template)
	fmt.Fprintf(w, "\n #  %-20s %4s %6s %-30s", "NAME", "PTS", "TIME", "DATE")
	fmt.Fprintf(w, NormalSeparator)
	for i, e := range s.currentBoard {
		fmt.Fprintf(w, "%3d %-20s %4d %6.3f %-30s\n",
			i+1, e.NickName, e.Score, e.Runtime, e.Date)
		if i >= 24 { // only showing top 25.
			break
		}
	}
	fmt.Fprintf(w, NormalSeparator)
	r.ParseForm()
	v := r.FormValue("ids")
	if v != "" {
		yourIds := strings.Split(v, ",")
	outer:
		for _, yourId := range yourIds {
			for rank, entry := range s.currentBoard {
				if strings.EqualFold(entry.Id, yourId) {
					fmt.Fprintf(w, "Your submission %s is on rank %d!\n", yourId, 1+rank)
					continue outer
				}
			}
			fmt.Fprintf(w, "Submission %s not found!\n", yourId)
		}
	} else {
		fmt.Fprint(w, "No submission id are given via ?ids=...")

	}

	fmt.Fprint(w, BoldSeparator)
	fmt.Fprintf(w,
		"Server version: %s                        Server time: %s\n"+
			"https://github.com/wadoon/scoreboard              %s\n",
		VERSION, time.Now().Format(time.RFC3339), AUTHOR)
}

func (service *Service) jobStatus(c *ace.C) {
	w := c.Writer
	id := c.Param("id")
	pos := service.GetSubmission(id)
	if pos < 0 {
		fmt.Fprintf(w, "The submission '%s' not found.\n", id)
		c.Abort()
	} else {
		entry := service.currentBoard[pos]
		if entry.IsRun() {
			fmt.Fprintln(w, "The submission was executed.")
			fmt.Fprint(w, BoldSeparator)
			fmt.Fprintln(w, entry.Output())
			fmt.Fprint(w, BoldSeparator)
			fmt.Fprintf(w, "Its rank is %d.\n", 1+pos)
			if pos == 0 {
				fmt.Fprintf(w, "!!! Congratulation, you are leading the scoreboard !!!")
			}
		} else if entry.isFailure() {
			fmt.Fprintln(w, "The submission was executed with an error.\nPlease inspect the output carefully.")
			fmt.Fprint(w, BoldSeparator)
			fmt.Fprintln(w, entry.Output())
			fmt.Fprint(w, BoldSeparator)
		} else {
			fmt.Fprintln(w, "The submission was not executed.")
			//fmt.Fprintln(w, "Position in queue: %d.", )
		}
	}
	fmt.Fprintln(w, "")
}
func (service *Service) GetSubmission(id string) int {
	sort.Sort(service.currentBoard)

	for i, e := range service.currentBoard {
		if e.Id == id {
			return i
		}
	}
	return -1
}

// Submission Worker
//*****************************************************************************
type Submission struct {
	entry  Entry
	result chan Entry
}

func StartWorker(input chan Submission) {
	for {
		submission := <-input
		entry := submission.entry

		logFile := path.Join(entry.Folder, OutputLog)
		out, _ := os.Create(logFile)

		log.Printf("Execute submission: %s. Output goes to %s\n", entry.Id, logFile)

		cmd := exec.Command(entry.Script, entry.Id)
		log.Printf("Run '%s %s'\n", entry.Script, entry.Id)
		//"/usr/bin/time", "-f", "'[%U,%S,%e]'", script)
		cmd.Dir, _ = filepath.Abs(entry.Folder)

		start := time.Now()
		output, err := cmd.CombinedOutput()
		elapsed := time.Since(start)

		out.Write(output)

		/*if err != nil {
			log.Println(err)
			fmt.Fprintln(out, err)
			continue
		}*/

		exitStatus := cmd.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()

		fmt.Fprintf(out, "Process exited with %d\n", exitStatus)
		fmt.Fprintf(out, "Runtime: user/system/real = %5.2f / %5.2f / %5.2f\n",
			cmd.ProcessState.UserTime().Seconds(),
			cmd.ProcessState.SystemTime().Seconds(),
			elapsed.Seconds())

		entry.Date = time.Now().Format(time.RFC3339)
		entry.Runtime = cmd.ProcessState.UserTime().Seconds()

		if err != nil {
			fmt.Fprintf(out, "Error occured during process. %s\n", err)
			fmt.Fprintf(out, "*** Submission rejected ***\n")
			entry.Score = -2
		} else {
			//entry.NickName = r.FormValue("alias")
			entry.Score = extractScore(entry.Id, output)
		}
		submission.result <- entry
		out.Close()
	}
}

//
func extractScore(salt string, bytes []byte) int {
	re, err := regexp.Compile(`score[ ]*` + salt + `=[ ]*(\d+)`)
	if err != nil {
		log.Fatal(err)
	}
	score := re.FindSubmatch(bytes)
	if score != nil {
		s, _ := strconv.Atoi(string(score[1][:]))
		return s
	} else {
		return 0
	}
}

type Entry struct {
	Id       string
	NickName string
	Score    int
	Runtime  float64
	Date     string
	Hash     [16]byte
	Folder   string
	Script   string
}

func (e *Entry) Output() string {
	out, err := ioutil.ReadFile(filepath.Join(e.Folder, OutputLog));
	if err != nil {
		log.Println("Error in reading output", err)
	}
	return string(out[:])
}

func (e *Entry) IsRun() bool {
	return e.Score >= 0
}
func (e *Entry) isFailure() bool {
	return e.Score == -2
}

func (a ScoreBoard) Len() int      { return len(a) }
func (a ScoreBoard) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ScoreBoard) Less(i, j int) bool {
	x := a[i]
	y := a[j]

	if x.IsRun() && !y.IsRun() {
		return true
	}
	if !x.IsRun() && y.IsRun() {
		return false
	}

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
